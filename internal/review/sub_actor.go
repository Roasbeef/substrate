package review

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/roasbeef/subtrate/internal/store"
	"gopkg.in/yaml.v3"
)

// ReviewerResult is the structured result parsed from YAML frontmatter in a
// reviewer agent's response.
type ReviewerResult struct {
	// Decision is the review decision: "approve", "request_changes", or
	// "reject".
	Decision string `yaml:"decision"`

	// Summary is a human-readable summary of the review.
	Summary string `yaml:"summary"`

	// Issues is the list of issues found during the review.
	Issues []ReviewerIssue `yaml:"issues"`

	// FilesReviewed is the number of files analyzed.
	FilesReviewed int `yaml:"files_reviewed"`

	// LinesAnalyzed is the approximate number of lines analyzed.
	LinesAnalyzed int `yaml:"lines_analyzed"`
}

// ReviewerIssue is a single issue found during code review.
type ReviewerIssue struct {
	Title       string `yaml:"title"`
	IssueType   string `yaml:"type"`
	Severity    string `yaml:"severity"`
	FilePath    string `yaml:"file"`
	LineStart   int    `yaml:"line_start"`
	LineEnd     int    `yaml:"line_end"`
	Description string `yaml:"description"`
	CodeSnippet string `yaml:"code_snippet"`
	Suggestion  string `yaml:"suggestion"`
	ClaudeMDRef string `yaml:"claude_md_ref"`
}

// SubActorConfig holds configuration for the reviewer sub-actor.
type SubActorConfig struct {
	// Store is the storage backend for persisting review data.
	Store store.Storage

	// SpawnConfig overrides the default spawn configuration. If nil, a
	// default config is used.
	SpawnConfig *SpawnConfig
}

// SpawnConfig holds the Claude Agent SDK spawning parameters for a reviewer.
type SpawnConfig struct {
	// CLIPath is the path to the claude CLI binary.
	CLIPath string

	// MaxTurns limits the number of conversation turns.
	MaxTurns int

	// AllowDangerouslySkipPermissions enables bypassing permissions.
	AllowDangerouslySkipPermissions bool

	// NoSessionPersistence disables session saving.
	NoSessionPersistence bool

	// ConfigDir overrides ~/.claude for isolation.
	ConfigDir string
}

// DefaultSubActorSpawnConfig returns the default spawn config for reviewers.
func DefaultSubActorSpawnConfig() *SpawnConfig {
	return &SpawnConfig{
		CLIPath:                         "claude",
		MaxTurns:                        20,
		AllowDangerouslySkipPermissions: true,
	}
}

// reviewSubActor manages a single reviewer agent instance. It is spawned as a
// goroutine per review, creates a Claude Agent SDK client, sends the review
// prompt, processes the response, and feeds events back to the service.
type reviewSubActor struct {
	reviewID  string
	threadID  string
	repoPath  string
	requester int64
	config    *ReviewerConfig
	store     store.Storage

	spawnCfg *SpawnConfig

	// callback is called when the reviewer produces a result. The service
	// uses this to feed FSM events.
	callback func(ctx context.Context, result *SubActorResult)

	// cancel stops the sub-actor's context.
	cancel context.CancelFunc
}

// SubActorResult contains the outcome of a reviewer sub-actor run.
type SubActorResult struct {
	ReviewID  string
	ThreadID  string
	SessionID string
	Result    *ReviewerResult
	CostUSD   float64
	Duration  time.Duration
	Error     error
}

// newReviewSubActor creates a new reviewer sub-actor for the given review.
func newReviewSubActor(
	reviewID, threadID, repoPath string,
	requester int64,
	config *ReviewerConfig,
	st store.Storage,
	spawnCfg *SpawnConfig,
	callback func(ctx context.Context, result *SubActorResult),
) *reviewSubActor {
	if spawnCfg == nil {
		spawnCfg = DefaultSubActorSpawnConfig()
	}

	return &reviewSubActor{
		reviewID:  reviewID,
		threadID:  threadID,
		repoPath:  repoPath,
		requester: requester,
		config:    config,
		store:     st,
		spawnCfg:  spawnCfg,
		callback:  callback,
	}
}

// Run starts the reviewer sub-actor. It creates a Claude Agent SDK client,
// sends the review prompt, parses the response, persists the iteration and
// issues, and invokes the callback with the result. This method blocks until
// the review completes or the context is cancelled.
func (r *reviewSubActor) Run(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, r.config.Timeout)
	r.cancel = cancel
	defer cancel()

	startTime := time.Now()

	result := &SubActorResult{
		ReviewID: r.reviewID,
		ThreadID: r.threadID,
	}

	// Build the client options from the reviewer config.
	opts := r.buildClientOptions()

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		result.Error = fmt.Errorf("create claude client: %w", err)
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)
		return
	}
	defer client.Close()

	// Connect to the CLI subprocess.
	if err := client.Connect(ctx); err != nil {
		result.Error = fmt.Errorf("connect to claude CLI: %w", err)
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)
		return
	}

	// Build and send the review prompt.
	prompt := r.buildReviewPrompt()

	var (
		lastText  string
		response  claudeagent.ResultMessage
		gotResult bool
	)

	for msg := range client.Query(ctx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			text := m.ContentText()
			if text != "" {
				lastText = text
			}
			result.SessionID = m.SessionID

		case claudeagent.ResultMessage:
			response = m
			gotResult = true
			result.SessionID = m.SessionID
			result.CostUSD = m.TotalCostUSD
			result.Duration = time.Duration(
				m.DurationMs,
			) * time.Millisecond
		}
	}

	if !gotResult {
		result.Error = fmt.Errorf("no result message received")
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)
		return
	}

	if response.IsError {
		errMsg := "review failed"
		if len(response.Errors) > 0 {
			errMsg = response.Errors[0]
		}
		result.Error = fmt.Errorf("reviewer error: %s", errMsg)
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)
		return
	}

	// Parse the review result from the response text.
	responseText := response.Result
	if responseText == "" {
		responseText = lastText
	}

	parsed, err := ParseReviewerResponse(responseText)
	if err != nil {
		result.Error = fmt.Errorf("parse reviewer response: %w", err)
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)
		return
	}

	result.Result = parsed

	// Persist the iteration and issues to the database.
	r.persistResults(ctx, result, startTime)

	// Invoke the callback so the service can update the FSM.
	r.callback(ctx, result)
}

// Stop cancels the sub-actor's context.
func (r *reviewSubActor) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

// buildClientOptions constructs SDK options from the reviewer and spawn
// configs.
func (r *reviewSubActor) buildClientOptions() []claudeagent.Option {
	opts := []claudeagent.Option{
		claudeagent.WithModel(r.config.Model),
	}

	if r.spawnCfg.CLIPath != "" &&
		r.spawnCfg.CLIPath != "claude" {

		opts = append(
			opts,
			claudeagent.WithCLIPath(r.spawnCfg.CLIPath),
		)
	}

	if r.repoPath != "" {
		opts = append(opts, claudeagent.WithCwd(r.repoPath))
	}

	systemPrompt := r.buildSystemPrompt()
	if systemPrompt != "" {
		opts = append(
			opts, claudeagent.WithSystemPrompt(systemPrompt),
		)
	}

	if r.spawnCfg.MaxTurns > 0 {
		opts = append(
			opts,
			claudeagent.WithMaxTurns(r.spawnCfg.MaxTurns),
		)
	}

	if r.spawnCfg.AllowDangerouslySkipPermissions {
		opts = append(
			opts,
			claudeagent.WithAllowDangerouslySkipPermissions(
				true,
			),
		)
	}

	if r.spawnCfg.NoSessionPersistence {
		opts = append(
			opts, claudeagent.WithNoSessionPersistence(),
		)
	}

	if r.spawnCfg.ConfigDir != "" {
		opts = append(
			opts,
			claudeagent.WithConfigDir(r.spawnCfg.ConfigDir),
		)
	}

	return opts
}

// buildSystemPrompt constructs the system prompt for the reviewer agent based
// on the ReviewerConfig persona.
func (r *reviewSubActor) buildSystemPrompt() string {
	if r.config.SystemPrompt != "" {
		return r.config.SystemPrompt
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"You are %s, a code reviewer for the Subtrate project.\n\n",
		r.config.Name,
	))

	sb.WriteString("## Your Role\n")
	sb.WriteString(
		"Review the code changes on the current branch " +
			"compared to the base branch. ",
	)
	sb.WriteString(
		"Identify bugs, security issues, logic errors, " +
			"and style violations.\n\n",
	)

	if len(r.config.FocusAreas) > 0 {
		sb.WriteString("## Focus Areas\n")
		for _, area := range r.config.FocusAreas {
			sb.WriteString(fmt.Sprintf("- %s\n", area))
		}
		sb.WriteString("\n")
	}

	if len(r.config.IgnorePatterns) > 0 {
		sb.WriteString("## Ignore Patterns\n")
		sb.WriteString(
			"Skip the following files/patterns:\n",
		)
		for _, pat := range r.config.IgnorePatterns {
			sb.WriteString(fmt.Sprintf("- %s\n", pat))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Output Format\n")
	sb.WriteString(
		"You MUST include a YAML frontmatter block at the " +
			"END of your response.\n",
	)
	sb.WriteString(
		"The block must be delimited by ```yaml and ``` " +
			"markers.\n",
	)
	sb.WriteString("Use this exact schema:\n\n")
	sb.WriteString("```yaml\n")
	sb.WriteString("decision: approve | request_changes | reject\n")
	sb.WriteString("summary: \"Brief summary of findings\"\n")
	sb.WriteString("files_reviewed: 5\n")
	sb.WriteString("lines_analyzed: 500\n")
	sb.WriteString("issues:\n")
	sb.WriteString("  - title: \"Issue title\"\n")
	sb.WriteString("    type: bug | security | performance | ")
	sb.WriteString("style | logic | architecture\n")
	sb.WriteString("    severity: critical | high | medium | low\n")
	sb.WriteString("    file: \"path/to/file.go\"\n")
	sb.WriteString("    line_start: 42\n")
	sb.WriteString("    line_end: 50\n")
	sb.WriteString("    description: \"Detailed description\"\n")
	sb.WriteString("    suggestion: \"Suggested fix\"\n")
	sb.WriteString("```\n\n")
	sb.WriteString(
		"If the code looks good and you approve, set " +
			"decision to 'approve' with an empty issues list.\n",
	)

	return sb.String()
}

// buildReviewPrompt constructs the initial user prompt sent to the reviewer.
func (r *reviewSubActor) buildReviewPrompt() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"Review the code changes for review ID: %s\n\n",
		r.reviewID,
	))
	sb.WriteString(
		"Please examine the diff between the current branch " +
			"and the base branch.\n",
	)
	sb.WriteString(
		"Use `git diff main...HEAD` to see the changes, then " +
			"review each modified file.\n\n",
	)
	sb.WriteString(
		"After your analysis, include the YAML frontmatter " +
			"block with your decision and any issues found.\n",
	)

	return sb.String()
}

// persistResults saves the iteration and issue records to the database.
func (r *reviewSubActor) persistResults(
	ctx context.Context, result *SubActorResult, startTime time.Time,
) {
	if result.Result == nil {
		return
	}

	parsed := result.Result
	now := time.Now()

	// Determine iteration number by querying existing iterations.
	iters, err := r.store.GetReviewIterations(ctx, r.reviewID)
	if err != nil {
		log.Printf(
			"review sub-actor: get iterations for %s: %v",
			r.reviewID, err,
		)
		return
	}
	iterNum := len(iters) + 1

	// Create the iteration record.
	_, err = r.store.CreateReviewIteration(
		ctx, store.CreateReviewIterationParams{
			ReviewID:          r.reviewID,
			IterationNum:      iterNum,
			ReviewerID:        r.config.Name,
			ReviewerSessionID: result.SessionID,
			Decision:          parsed.Decision,
			Summary:           parsed.Summary,
			FilesReviewed:     parsed.FilesReviewed,
			LinesAnalyzed:     parsed.LinesAnalyzed,
			DurationMS:        result.Duration.Milliseconds(),
			CostUSD:           result.CostUSD,
			StartedAt:         startTime,
			CompletedAt:       &now,
		},
	)
	if err != nil {
		log.Printf(
			"review sub-actor: create iteration for %s: %v",
			r.reviewID, err,
		)
		return
	}

	// Create issue records for each issue found.
	for _, issue := range parsed.Issues {
		_, err := r.store.CreateReviewIssue(
			ctx, store.CreateReviewIssueParams{
				ReviewID:     r.reviewID,
				IterationNum: iterNum,
				IssueType:    issue.IssueType,
				Severity:     issue.Severity,
				FilePath:     issue.FilePath,
				LineStart:    issue.LineStart,
				LineEnd:      issue.LineEnd,
				Title:        issue.Title,
				Description:  issue.Description,
				CodeSnippet:  issue.CodeSnippet,
				Suggestion:   issue.Suggestion,
				ClaudeMDRef:  issue.ClaudeMDRef,
			},
		)
		if err != nil {
			log.Printf(
				"review sub-actor: create issue for %s: %v",
				r.reviewID, err,
			)
		}
	}
}

// ParseReviewerResponse extracts the YAML frontmatter block from a reviewer's
// response text. The YAML block is expected to be delimited by ```yaml and
// ``` markers at the end of the response.
func ParseReviewerResponse(text string) (*ReviewerResult, error) {
	yamlBlock := extractYAMLBlock(text)
	if yamlBlock == "" {
		return nil, fmt.Errorf(
			"no YAML frontmatter block found in response",
		)
	}

	var result ReviewerResult
	if err := yaml.Unmarshal([]byte(yamlBlock), &result); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter: %w", err)
	}

	// Validate required fields.
	if result.Decision == "" {
		return nil, fmt.Errorf("missing required field: decision")
	}

	validDecisions := map[string]bool{
		"approve":         true,
		"request_changes": true,
		"reject":          true,
	}
	if !validDecisions[result.Decision] {
		return nil, fmt.Errorf(
			"invalid decision: %q (expected approve, "+
				"request_changes, or reject)",
			result.Decision,
		)
	}

	return &result, nil
}

// extractYAMLBlock finds the last ```yaml ... ``` block in the text.
func extractYAMLBlock(text string) string {
	// Find the last occurrence of ```yaml marker.
	lastIdx := strings.LastIndex(text, "```yaml")
	if lastIdx == -1 {
		// Try alternative marker.
		lastIdx = strings.LastIndex(text, "```yml")
	}
	if lastIdx == -1 {
		return ""
	}

	// Find the start of the YAML content (after the marker line).
	contentStart := strings.Index(text[lastIdx:], "\n")
	if contentStart == -1 {
		return ""
	}
	contentStart += lastIdx + 1

	// Find the closing ``` marker.
	remaining := text[contentStart:]
	closingIdx := strings.Index(remaining, "```")
	if closingIdx == -1 {
		return ""
	}

	return strings.TrimSpace(remaining[:closingIdx])
}

// SubActorManager manages active reviewer sub-actors. It tracks running
// goroutines and provides cleanup on shutdown.
type SubActorManager struct {
	mu       sync.Mutex
	actors   map[string]*reviewSubActor
	store    store.Storage
	spawnCfg *SpawnConfig
}

// NewSubActorManager creates a new sub-actor manager.
func NewSubActorManager(
	st store.Storage, spawnCfg *SpawnConfig,
) *SubActorManager {
	if spawnCfg == nil {
		spawnCfg = DefaultSubActorSpawnConfig()
	}

	return &SubActorManager{
		actors:   make(map[string]*reviewSubActor),
		store:    st,
		spawnCfg: spawnCfg,
	}
}

// SpawnReviewer creates and starts a reviewer sub-actor for the given review.
// The callback is invoked when the reviewer completes.
func (m *SubActorManager) SpawnReviewer(
	ctx context.Context,
	reviewID, threadID, repoPath string,
	requester int64,
	config *ReviewerConfig,
	callback func(ctx context.Context, result *SubActorResult),
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Avoid duplicate spawns for the same review.
	if _, exists := m.actors[reviewID]; exists {
		log.Printf(
			"sub-actor manager: reviewer already active for %s",
			reviewID,
		)
		return
	}

	actor := newReviewSubActor(
		reviewID, threadID, repoPath, requester,
		config, m.store, m.spawnCfg,
		func(ctx context.Context, result *SubActorResult) {
			// Remove from active actors when done.
			m.mu.Lock()
			delete(m.actors, reviewID)
			m.mu.Unlock()

			// Forward to the service callback.
			callback(ctx, result)
		},
	)

	m.actors[reviewID] = actor

	go actor.Run(ctx)
}

// StopReviewer stops a running reviewer sub-actor.
func (m *SubActorManager) StopReviewer(reviewID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if actor, ok := m.actors[reviewID]; ok {
		actor.Stop()
		delete(m.actors, reviewID)
	}
}

// StopAll stops all active reviewer sub-actors.
func (m *SubActorManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, actor := range m.actors {
		actor.Stop()
		delete(m.actors, id)
	}
}

// ActiveCount returns the number of active reviewer sub-actors.
func (m *SubActorManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.actors)
}
