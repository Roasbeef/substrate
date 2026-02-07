package summary

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"

	"github.com/roasbeef/subtrate/internal/store"
)

// Service provides agent activity summarization using Claude Haiku
// via the Go Agent SDK.
type Service struct {
	cfg    Config
	store  store.Storage
	reader *TranscriptReader
	log    *slog.Logger

	mu    sync.RWMutex
	cache map[int64]*cachedSummary

	// semaphore limits concurrent Haiku calls.
	sem chan struct{}
}

// NewService creates a new summary service.
func NewService(
	cfg Config, storage store.Storage, log *slog.Logger,
) *Service {
	if log == nil {
		log = slog.Default()
	}

	return &Service{
		cfg: cfg,
		store: storage,
		reader: NewTranscriptReader(
			cfg.TranscriptBasePath, cfg.MaxTranscriptLines,
		),
		log:   log.With("component", "summary"),
		cache: make(map[int64]*cachedSummary),
		sem:   make(chan struct{}, cfg.MaxConcurrent),
	}
}

// GetSummary returns the current summary for an agent. It returns a
// cached value if available, otherwise triggers generation.
func (s *Service) GetSummary(
	ctx context.Context, agentID int64,
) (*SummaryResult, error) {
	if !s.cfg.Enabled {
		return nil, fmt.Errorf("summary service disabled")
	}

	s.mu.RLock()
	cached := s.cache[agentID]
	s.mu.RUnlock()

	if cached != nil && cached.isValid(s.cfg.CacheTTL) {
		return cached.result, nil
	}

	// Try to return a stale cached value while triggering a
	// background refresh.
	if cached != nil && cached.result != nil && !cached.generating {
		s.triggerRefresh(ctx, agentID)

		stale := *cached.result
		stale.IsStale = true
		return &stale, nil
	}

	// No cache at all — try DB.
	dbSummary, err := s.store.GetLatestSummary(ctx, agentID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get latest summary: %w", err)
	}

	if err == nil {
		result := &SummaryResult{
			AgentID:        dbSummary.AgentID,
			Summary:        dbSummary.Summary,
			Delta:          dbSummary.Delta,
			TranscriptHash: dbSummary.TranscriptHash,
			GeneratedAt:    dbSummary.CreatedAt,
			CostUSD:        dbSummary.CostUSD,
			IsStale:        true,
		}

		s.mu.Lock()
		s.cache[agentID] = &cachedSummary{
			result:         result,
			transcriptHash: dbSummary.TranscriptHash,
			cachedAt:       dbSummary.CreatedAt,
		}
		s.mu.Unlock()

		s.triggerRefresh(ctx, agentID)
		return result, nil
	}

	// No summary exists at all — trigger generation.
	s.triggerRefresh(ctx, agentID)
	return nil, nil
}

// GetAllSummaries returns summaries for the given agent IDs.
func (s *Service) GetAllSummaries(
	ctx context.Context, agentIDs []int64,
) ([]*SummaryResult, error) {
	if !s.cfg.Enabled {
		return nil, fmt.Errorf("summary service disabled")
	}

	results := make([]*SummaryResult, 0, len(agentIDs))
	for _, id := range agentIDs {
		result, err := s.GetSummary(ctx, id)
		if err != nil {
			s.log.Warn("Failed to get summary",
				"agent_id", id, "error", err,
			)
			continue
		}
		if result != nil {
			results = append(results, result)
		}
	}
	return results, nil
}

// GetSummaryHistory returns recent summaries for an agent.
func (s *Service) GetSummaryHistory(
	ctx context.Context, agentID int64, limit int,
) ([]store.AgentSummary, error) {
	if limit <= 0 {
		limit = DefaultHistoryLimit
	}
	return s.store.GetSummaryHistory(ctx, agentID, limit)
}

// RefreshAgent generates a fresh summary for an agent synchronously.
func (s *Service) RefreshAgent(
	ctx context.Context, agentID int64,
	projectKey, sessionID string,
) error {
	return s.generateSummary(ctx, agentID, projectKey, sessionID)
}

// triggerRefresh launches a background summary generation.
func (s *Service) triggerRefresh(ctx context.Context, agentID int64) {
	s.mu.Lock()
	if c, ok := s.cache[agentID]; ok && c.generating {
		s.mu.Unlock()
		return
	}
	if s.cache[agentID] == nil {
		s.cache[agentID] = &cachedSummary{}
	}
	s.cache[agentID].generating = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			if c, ok := s.cache[agentID]; ok {
				c.generating = false
			}
			s.mu.Unlock()
		}()

		// Look up agent info to get project key and session ID.
		agent, err := s.store.GetAgent(ctx, agentID)
		if err != nil {
			s.log.Warn("Failed to get agent for summary",
				"agent_id", agentID, "error", err,
			)
			return
		}

		err = s.generateSummary(
			ctx, agentID, agent.ProjectKey,
			agent.CurrentSessionID,
		)
		if err != nil {
			s.log.Warn("Failed to generate summary",
				"agent_id", agentID, "error", err,
			)
		}
	}()
}

// generateSummary reads the transcript, calls Haiku, and caches the
// result.
func (s *Service) generateSummary(
	ctx context.Context, agentID int64,
	projectKey, sessionID string,
) error {
	if projectKey == "" || sessionID == "" {
		return fmt.Errorf(
			"missing project_key or session_id for agent %d",
			agentID,
		)
	}

	// Read the transcript.
	transcript, err := s.reader.ReadRecentTranscript(
		projectKey, sessionID,
	)
	if err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}

	// Check if content changed since last summary.
	s.mu.RLock()
	cached := s.cache[agentID]
	s.mu.RUnlock()

	if cached != nil &&
		cached.transcriptHash == transcript.Hash &&
		cached.isValid(s.cfg.CacheTTL) {

		return nil
	}

	// Get previous summary for delta tracking.
	var prevSummary string
	if cached != nil && cached.result != nil {
		prevSummary = cached.result.Summary
	}

	// Acquire semaphore.
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Call Haiku via Agent SDK.
	summaryText, deltaText, err := s.callHaiku(
		ctx, transcript.Content, prevSummary,
	)
	if err != nil {
		return fmt.Errorf("haiku summarization: %w", err)
	}

	now := time.Now()
	result := &SummaryResult{
		AgentID:        agentID,
		Summary:        summaryText,
		Delta:          deltaText,
		TranscriptHash: transcript.Hash,
		GeneratedAt:    now,
	}

	// Update cache.
	s.mu.Lock()
	s.cache[agentID] = &cachedSummary{
		result:         result,
		transcriptHash: transcript.Hash,
		cachedAt:       now,
	}
	s.mu.Unlock()

	// Persist to DB.
	_, dbErr := s.store.CreateSummary(ctx, store.CreateSummaryParams{
		AgentID:        agentID,
		Summary:        summaryText,
		Delta:          deltaText,
		TranscriptHash: transcript.Hash,
	})
	if dbErr != nil {
		s.log.Warn("Failed to persist summary",
			"agent_id", agentID, "error", dbErr,
		)
	}

	return nil
}

// callHaiku spawns a Claude Haiku agent to summarize the transcript.
func (s *Service) callHaiku(
	ctx context.Context, transcript, previousSummary string,
) (summary string, delta string, err error) {
	// Create a temp config dir for isolation.
	tmpDir, err := os.MkdirTemp("", "substrate-summary-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create config dir: %w", err)
	}

	opts := []claudeagent.Option{
		claudeagent.WithModel(s.cfg.Model),
		claudeagent.WithSystemPrompt(summarizerSystemPrompt),
		claudeagent.WithMaxTurns(1),
		claudeagent.WithConfigDir(configDir),
		claudeagent.WithSettingSources(nil),
		claudeagent.WithSkillsDisabled(),
		claudeagent.WithNoSessionPersistence(),
		claudeagent.WithCanUseTool(denyAllToolPolicy),
	}

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return "", "", fmt.Errorf("create client: %w", err)
	}
	defer client.Close()

	prompt := buildSummaryPrompt(transcript, previousSummary)

	// Stream the response via single message channel.
	var responseText strings.Builder

	for msg := range client.Query(ctx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			text := m.ContentText()
			if text != "" {
				responseText.WriteString(text)
			}

		case claudeagent.ResultMessage:
			// Terminal message — break out of loop.

		default:
			// UserMessage or other types — skip.
		}
	}

	return parseSummaryResponse(responseText.String())
}

// parseSummaryResponse extracts summary and delta from the Haiku
// response text.
func parseSummaryResponse(
	text string,
) (summary string, delta string, err error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "SUMMARY:") {
			summary = strings.TrimSpace(
				strings.TrimPrefix(line, "SUMMARY:"),
			)
		} else if strings.HasPrefix(line, "DELTA:") {
			delta = strings.TrimSpace(
				strings.TrimPrefix(line, "DELTA:"),
			)
		}
	}

	if summary == "" {
		// Fall back to using the entire response as summary.
		summary = strings.TrimSpace(text)
		if summary == "" {
			summary = "Agent idle"
		}
		if delta == "" {
			delta = "Initial summary"
		}
	}

	return summary, delta, nil
}

// denyAllToolPolicy denies all tool access — summarization is pure
// text-in/text-out with no tools needed.
func denyAllToolPolicy(
	_ context.Context, _ claudeagent.ToolPermissionRequest,
) claudeagent.PermissionResult {
	return claudeagent.PermissionDeny{
		Reason: "Tool use not permitted for summarization",
	}
}

// RunBackgroundRefresh starts a background loop that periodically
// refreshes summaries for active agents. It blocks until ctx is
// cancelled.
func (s *Service) RunBackgroundRefresh(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}

	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshActiveAgents(ctx)
		}
	}
}

// refreshActiveAgents refreshes summaries for all active/busy agents.
func (s *Service) refreshActiveAgents(ctx context.Context) {
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		s.log.Warn("Failed to list agents for refresh",
			"error", err,
		)
		return
	}

	for _, agent := range agents {
		if agent.CurrentSessionID == "" || agent.ProjectKey == "" {
			continue
		}

		// Only refresh agents active in the last 30 minutes.
		if time.Since(agent.LastActiveAt) > 30*time.Minute {
			continue
		}

		s.triggerRefresh(ctx, agent.ID)
	}
}
