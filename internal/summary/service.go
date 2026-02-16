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

	// OnSummaryGenerated is an optional callback invoked after a new
	// summary is generated and persisted. Used to broadcast WebSocket
	// updates.
	OnSummaryGenerated func(agentID int64, summary, delta string)
}

// NewService creates a new summary service.
func NewService(
	cfg Config, storage store.Storage, log *slog.Logger,
) *Service {
	if log == nil {
		log = slog.Default()
	}

	return &Service{
		cfg:   cfg,
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

// triggerRefresh launches a background summary generation. Uses a
// detached context so the goroutine survives after the originating
// HTTP request completes.
func (s *Service) triggerRefresh(_ context.Context, agentID int64) {
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

		// Use a background context so this goroutine is not
		// canceled when the originating HTTP request ends.
		bgCtx := context.Background()

		// Look up agent info to get project key and session ID.
		agent, err := s.store.GetAgent(bgCtx, agentID)
		if err != nil {
			s.log.Warn("Failed to get agent for summary",
				"agent_id", agentID, "error", err,
			)
			return
		}

		// Skip agents without a project key — can't locate
		// transcripts without it.
		if agent.ProjectKey == "" {
			return
		}

		// If the agent has no session ID persisted, try to
		// discover the most recently modified session file
		// from the transcript directory.
		sessID := agent.CurrentSessionID
		if sessID == "" {
			discovered, discErr := s.reader.FindActiveSession(
				agent.ProjectKey,
			)
			if discErr != nil {
				s.log.Warn("No session found for agent",
					"agent_id", agentID,
					"project_key", agent.ProjectKey,
					"error", discErr,
				)
				return
			}
			sessID = discovered
			s.log.Info("Discovered session for agent",
				"agent_id", agentID,
				"session_id", sessID,
			)
		}

		err = s.generateSummary(
			bgCtx, agentID, agent.ProjectKey, sessID,
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

	s.log.Info("Generating summary",
		"agent_id", agentID,
		"project_key", projectKey,
		"session_id", sessionID,
	)

	// Read the transcript.
	transcript, err := s.reader.ReadRecentTranscript(
		projectKey, sessionID,
	)
	if err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}

	s.log.Info("Transcript read",
		"agent_id", agentID,
		"hash", transcript.Hash[:12],
		"content_len", len(transcript.Content),
	)

	// Check if content changed since last summary.
	s.mu.RLock()
	cached := s.cache[agentID]
	s.mu.RUnlock()

	if cached != nil &&
		cached.transcriptHash == transcript.Hash &&
		cached.isValid(s.cfg.CacheTTL) {

		s.log.Info("Summary cache hit, skipping",
			"agent_id", agentID,
		)
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

	s.log.Info("Calling Haiku for summary",
		"agent_id", agentID,
	)

	// Call Haiku via Agent SDK.
	summaryText, deltaText, err := s.callHaiku(
		ctx, transcript.Content, prevSummary,
	)
	if err != nil {
		return fmt.Errorf("haiku summarization: %w", err)
	}

	// Check if the new summary is meaningfully different from the
	// previous one. If Haiku reports no change or the text is nearly
	// identical, update the cache (to prevent re-generation) but
	// skip persisting to DB and broadcasting — this keeps the
	// timeline free of duplicate "standing by" entries.
	isDuplicate := isSummaryDuplicate(
		summaryText, deltaText, prevSummary,
	)

	s.log.Info("Summary generated",
		"agent_id", agentID,
		"summary_len", len(summaryText),
		"delta_len", len(deltaText),
		"duplicate", isDuplicate,
	)

	now := time.Now()
	result := &SummaryResult{
		AgentID:        agentID,
		Summary:        summaryText,
		Delta:          deltaText,
		TranscriptHash: transcript.Hash,
		GeneratedAt:    now,
	}

	// Update cache in-place rather than replacing the entry. This
	// ensures the generating flag set by triggerRefresh is preserved
	// and the defer in triggerRefresh clears it on the same object.
	s.mu.Lock()
	entry := s.cache[agentID]
	if entry == nil {
		entry = &cachedSummary{}
		s.cache[agentID] = entry
		s.evictOldestLocked()
	}
	entry.result = result
	entry.transcriptHash = transcript.Hash
	entry.cachedAt = now
	s.mu.Unlock()

	// Skip DB persist and WS broadcast for duplicate summaries.
	if isDuplicate {
		return nil
	}

	// Persist to DB.
	_, dbErr := s.store.CreateSummary(ctx, store.CreateSummaryParams{
		AgentID:        agentID,
		Summary:        summaryText,
		Delta:          deltaText,
		TranscriptHash: transcript.Hash,
	})
	if dbErr != nil {
		s.log.Error("Failed to persist summary",
			"agent_id", agentID, "error", dbErr,
		)
	}

	// Notify listeners (e.g., WebSocket broadcast) of the new
	// summary regardless of DB persistence outcome. The cache is
	// already updated, so clients will see the new data.
	if s.OnSummaryGenerated != nil {
		s.OnSummaryGenerated(agentID, summaryText, deltaText)
	}

	if dbErr != nil {
		return fmt.Errorf("persist summary: %w", dbErr)
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

	// Capture subprocess stderr for debugging.
	var stderrBuf strings.Builder
	stderrCb := func(data string) {
		stderrBuf.WriteString(data)
	}

	opts := []claudeagent.Option{
		claudeagent.WithModel(s.cfg.Model),
		claudeagent.WithSystemPrompt(summarizerSystemPrompt),
		claudeagent.WithMaxTurns(1),
		claudeagent.WithConfigDir(configDir),
		// Don't load user/project filesystem settings (which
		// include hooks that interfere with subprocess lifecycle).
		claudeagent.WithSettingSources(nil),
		// Disable skills to prevent --setting-sources from being
		// passed to the CLI (the default SkillsConfig sends
		// --setting-sources user,project which loads project
		// hooks from .claude/settings.json).
		claudeagent.WithSkillsDisabled(),
		claudeagent.WithNoSessionPersistence(),
		claudeagent.WithCanUseTool(denyAllToolPolicy),
		claudeagent.WithStderr(stderrCb),
		// Pass empty hooks map to override any shell hooks the
		// CLI subprocess might discover. Without this, the
		// subprocess may pick up substrate hooks that try to
		// call EnsureIdentity RPC on session start.
		claudeagent.WithHooks(
			map[claudeagent.HookType][]claudeagent.HookConfig{},
		),
	}

	// Explicitly forward authentication tokens to the subprocess.
	// Without these, the Haiku subprocess inherits an empty env
	// and fails with auth errors. Matches the isolation pattern
	// used by the reviewer sub-actor.
	authEnv := make(map[string]string)
	for _, key := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN",
		"ANTHROPIC_API_KEY",
	} {
		if val := os.Getenv(key); val != "" {
			authEnv[key] = val
		}
	}
	if len(authEnv) > 0 {
		opts = append(opts, claudeagent.WithEnv(authEnv))
	}

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return "", "", fmt.Errorf("create client: %w", err)
	}
	defer client.Close()

	prompt := buildSummaryPrompt(transcript, previousSummary)

	// Use a tight timeout — Haiku should respond in seconds.
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Stream the response via single message channel.
	var responseText strings.Builder

	for msg := range client.Query(queryCtx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			text := m.ContentText()
			if text != "" {
				responseText.WriteString(text)
			}

		case claudeagent.ResultMessage:
			// Log result details for debugging.
			s.log.Info("Haiku result",
				"status", m.Status,
				"subtype", m.Subtype,
				"is_error", m.IsError,
				"result_len", len(m.Result),
				"errors", m.Errors,
				"cost_usd", m.TotalCostUSD,
				"turns", m.NumTurns,
			)

			// Use Result field if assistant stream was empty.
			if responseText.Len() == 0 && m.Result != "" {
				responseText.WriteString(m.Result)
			}

		default:
			// UserMessage or other types — skip.
		}
	}

	raw := responseText.String()
	stderr := stderrBuf.String()

	s.log.Info("Haiku raw response",
		"response_len", len(raw),
		"response_preview", truncate(raw, 200),
	)
	if stderr != "" {
		s.log.Warn("Haiku stderr output",
			"stderr_len", len(stderr),
			"stderr_preview", truncate(stderr, 500),
		)
	}

	return parseSummaryResponse(raw)
}

// parseSummaryResponse extracts summary and delta from the Haiku
// response text.
func parseSummaryResponse(
	text string,
) (summary string, delta string, err error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if val, ok := strings.CutPrefix(line, "SUMMARY:"); ok {
			summary = strings.TrimSpace(val)
		} else if val, ok := strings.CutPrefix(line, "DELTA:"); ok {
			delta = strings.TrimSpace(val)
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

// isSummaryDuplicate returns true when the new summary is not
// meaningfully different from the previous one. This prevents the
// timeline from filling up with near-identical "standing by" entries.
func isSummaryDuplicate(
	newSummary, delta, prevSummary string,
) bool {
	if prevSummary == "" {
		return false
	}

	deltaLower := strings.ToLower(delta)

	// If the delta explicitly says nothing changed, it's a duplicate.
	if strings.Contains(deltaLower, "no change") ||
		strings.Contains(deltaLower, "no new") ||
		strings.Contains(deltaLower, "unchanged") ||
		strings.Contains(deltaLower, "remains idle") ||
		strings.Contains(deltaLower, "still idle") ||
		strings.Contains(deltaLower, "same as") ||
		strings.Contains(deltaLower, "no update") {

		return true
	}

	// If the summary text is identical to the previous, skip.
	if strings.EqualFold(
		strings.TrimSpace(newSummary),
		strings.TrimSpace(prevSummary),
	) {
		return true
	}

	return false
}

// evictOldestLocked removes the oldest cache entry when the cache
// exceeds MaxCacheEntries. Must be called with s.mu held.
func (s *Service) evictOldestLocked() {
	if len(s.cache) <= DefaultMaxCacheEntries {
		return
	}

	var (
		oldestID   int64
		oldestTime time.Time
		found      bool
	)
	for id, c := range s.cache {
		// Don't evict entries that are currently generating.
		if c.generating {
			continue
		}
		if !found || c.cachedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = c.cachedAt
			found = true
		}
	}
	if found {
		delete(s.cache, oldestID)
	}
}

// truncate returns the first n characters of s, appending "..." if
// truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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

	var refreshCount int
	for _, agent := range agents {
		// Skip agents without a project key — can't locate
		// transcripts. Session ID is optional here because
		// triggerRefresh will auto-discover the session file.
		if agent.ProjectKey == "" {
			continue
		}

		// Only refresh agents active in the last 30 minutes.
		if time.Since(agent.LastActiveAt) > 30*time.Minute {
			continue
		}

		refreshCount++
		s.triggerRefresh(ctx, agent.ID)
	}

	s.log.Info("Background refresh cycle",
		"total_agents", len(agents),
		"eligible", refreshCount,
	)
}
