package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lightningnetwork/lnd/fn/v2"
	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
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
	Confidence  string `yaml:"confidence"`
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

	// NoSessionPersistence disables session saving.
	NoSessionPersistence bool

	// ConfigDir overrides ~/.claude for isolation.
	ConfigDir string
}

// DefaultSubActorSpawnConfig returns the default spawn config for reviewers.
func DefaultSubActorSpawnConfig() *SpawnConfig {
	return &SpawnConfig{
		CLIPath: "claude",
	}
}

// reviewSubActor manages a single reviewer agent instance. It is spawned as a
// goroutine per review, creates a Claude Agent SDK client, sends the review
// prompt, processes the response, and feeds events back to the service.
type reviewSubActor struct {
	reviewID   string
	threadID   string
	repoPath   string
	requester  int64
	branch     string
	baseBranch string
	commitSHA  string
	config     *ReviewerConfig
	store      store.Storage

	spawnCfg *SpawnConfig

	// isMultiReview indicates this reviewer is a coordinator that
	// delegates to specialized sub-agents via WithAgents(). When
	// true, the coordinator prompt template is used instead of the
	// standard single-reviewer template, and the SDK client is
	// configured with sub-agent definitions.
	isMultiReview bool

	// agentID is the database ID of this reviewer's agent identity,
	// set during session start when the agent is registered with
	// the substrate system.
	agentID int64

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

// ReviewerActorKey is the service key for reviewer sub-actors. Each reviewer
// is registered with the actor system under this key when spawned.
var ReviewerActorKey = actor.NewServiceKey[ReviewerRequest, any](
	"reviewer-actor",
)

// Ensure reviewSubActor implements ActorBehavior and Stoppable at compile time.
var (
	_ actor.ActorBehavior[ReviewerRequest, any] = (*reviewSubActor)(nil)
	_ actor.Stoppable                           = (*reviewSubActor)(nil)
)

// Receive implements actor.ActorBehavior. It dispatches to the review
// execution logic based on the message type.
func (r *reviewSubActor) Receive(
	ctx context.Context, msg ReviewerRequest,
) fn.Result[any] {
	switch m := msg.(type) {
	case RunReviewMsg:
		log.InfoS(ctx, "Reviewer actor received RunReviewMsg",
			"review_id", r.reviewID,
			"reviewer", r.config.Name,
			"repo_path", r.repoPath,
		)
		r.Run(ctx)
		return fn.Ok[any](nil)

	case ResumeReviewMsg:
		log.InfoS(ctx, "Reviewer actor received ResumeReviewMsg",
			"review_id", r.reviewID,
			"reviewer", r.config.Name,
			"commit_sha", m.CommitSHA,
		)
		r.Run(ctx)
		return fn.Ok[any](nil)

	default:
		log.WarnS(ctx, "Reviewer actor received unknown message",
			nil,
			"review_id", r.reviewID,
			"msg_type", fmt.Sprintf("%T", msg),
		)
		return fn.Err[any](fmt.Errorf(
			"unknown reviewer message: %T", msg,
		))
	}
}

// OnStop implements actor.Stoppable. Called during actor shutdown to cancel
// the running review and trigger Claude CLI subprocess cleanup.
func (r *reviewSubActor) OnStop(ctx context.Context) error {
	log.InfoS(ctx, "Reviewer actor OnStop called, cancelling review",
		"review_id", r.reviewID,
		"reviewer", r.config.Name,
	)
	r.Stop()
	return nil
}

// newReviewSubActor creates a new reviewer sub-actor for the given review.
// The branch, baseBranch, and commitSHA are used to template the git diff
// command in the reviewer's prompt.
func newReviewSubActor(
	reviewID, threadID, repoPath string,
	requester int64,
	branch, baseBranch, commitSHA string,
	config *ReviewerConfig,
	st store.Storage,
	spawnCfg *SpawnConfig,
	callback func(ctx context.Context, result *SubActorResult),
) *reviewSubActor {
	if spawnCfg == nil {
		spawnCfg = DefaultSubActorSpawnConfig()
	}

	return &reviewSubActor{
		reviewID:   reviewID,
		threadID:   threadID,
		repoPath:   repoPath,
		requester:  requester,
		branch:     branch,
		baseBranch: baseBranch,
		commitSHA:  commitSHA,
		config:     config,
		store:      st,
		spawnCfg:   spawnCfg,
		callback:   callback,
	}
}

// Run starts the reviewer sub-actor. It creates a Claude Agent SDK client,
// sends the review prompt, parses the response, persists the iteration and
// issues, and invokes the callback with the result. This method blocks until
// the review completes or the context is cancelled.
//
// The parentCtx is the actor's context (derived from context.Background in
// the actor system). Lifecycle management (shutdown, cleanup) is handled by
// the actor system's OnStop → Stop() → cancel() path rather than a timeout.
func (r *reviewSubActor) Run(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	r.cancel = cancel
	defer cancel()

	startTime := time.Now()

	log.InfoS(ctx, "Reviewer sub-actor starting review",
		"review_id", r.reviewID,
		"thread_id", r.threadID,
		"reviewer", r.config.Name,
		"model", r.config.Model,
		"repo_path", r.repoPath,
	)

	result := &SubActorResult{
		ReviewID: r.reviewID,
		ThreadID: r.threadID,
	}

	// Build the client options from the reviewer config.
	opts, configDir := r.buildClientOptions()

	// Clean up the temp config dir when done.
	if configDir != "" {
		defer func() {
			if err := os.RemoveAll(
				filepath.Dir(configDir),
			); err != nil {
				log.Warnf("Failed to clean up reviewer "+
					"config dir %s: %v",
					configDir, err,
				)
			}
		}()
	}

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		result.Error = fmt.Errorf("create claude client: %w", err)
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer failed to create Claude client",
			err,
			"review_id", r.reviewID,
			"duration", result.Duration.String(),
		)
		r.callback(ctx, result)
		return
	}
	defer client.Close()

	log.InfoS(ctx, "Reviewer connecting to Claude CLI subprocess",
		"review_id", r.reviewID,
		"cli_path", r.spawnCfg.CLIPath,
	)

	// Connect to the CLI subprocess.
	if err := client.Connect(ctx); err != nil {
		result.Error = fmt.Errorf("connect to claude CLI: %w", err)
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer failed to connect to Claude CLI",
			err,
			"review_id", r.reviewID,
			"duration", result.Duration.String(),
		)
		r.callback(ctx, result)
		return
	}

	// Register the reviewer agent identity eagerly after connect.
	// The SessionStart hook may not fire for SDK-spawned sessions,
	// so we register the agent directly here to ensure the agent
	// identity is available for substrate CLI commands and the
	// Stop hook's mail polling.
	r.registerAgentIdentity(ctx)

	log.InfoS(ctx, "Reviewer connected, sending review query",
		"review_id", r.reviewID,
	)

	// Build and send the review prompt.
	prompt := r.buildReviewPrompt()

	log.InfoS(ctx, "Reviewer review prompt",
		"review_id", r.reviewID,
		"prompt_len", len(prompt),
		"prompt", truncateStr(prompt, 500),
	)

	state := &messageLoopState{}

	for msg := range client.Query(ctx, prompt) {
		state.msgCount++

		log.InfoS(ctx, "Reviewer received message from CLI",
			"review_id", r.reviewID,
			"msg_num", state.msgCount,
			"msg_type", fmt.Sprintf("%T", msg),
		)

		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			r.handleAssistantMsg(
				ctx, m, state, result,
				startTime, client,
			)

		case claudeagent.ResultMessage:
			r.handleResultMsg(ctx, m, state, result)

		case claudeagent.UserMessage:
			r.handleToolResultMsg(ctx, m, state)

		default:
			log.InfoS(ctx,
				"Reviewer received other message type",
				"review_id", r.reviewID,
				"msg_num", state.msgCount,
				"type", fmt.Sprintf("%T", msg),
			)
		}
	}

	// Process the final result from the loop, unless it was already
	// handled eagerly from an AssistantMessage YAML block.
	r.processPostLoop(ctx, state, result, startTime)
}

// Stop cancels the sub-actor's context, triggering Claude CLI subprocess
// cleanup (stdin close → 5s grace period → SIGKILL).
func (r *reviewSubActor) Stop() {
	if r.cancel != nil {
		log.Infof("Reviewer sub-actor stopping, cancelling "+
			"context: review_id=%s", r.reviewID,
		)
		r.cancel()
	}
}

// messageLoopState tracks mutable state accumulated while processing the
// stream of messages from the Claude Agent SDK query loop.
type messageLoopState struct {
	// lastText is the most recent non-empty assistant text, used as a
	// fallback when the ResultMessage has an empty Result field.
	lastText string

	// response is the final ResultMessage from the CLI subprocess.
	response claudeagent.ResultMessage

	// gotResult is true once a ResultMessage has been received.
	gotResult bool

	// earlyResult is true if the review YAML was already parsed and
	// the callback fired during the message loop (eager path). When
	// set, the post-loop processing is skipped to avoid double-
	// persisting and double-calling the callback.
	earlyResult bool

	// msgCount is the total number of messages received so far.
	msgCount int

	// authErr is set when an authentication error is detected in
	// assistant text. Auth errors take priority over all other
	// processing since the reviewer cannot proceed without valid
	// credentials.
	authErr error
}

// isTerminalDecision returns true for review decisions that end the
// review lifecycle (no further back-and-forth expected).
func isTerminalDecision(decision string) bool {
	return decision == "approve" || decision == "reject"
}

// handleAssistantMsg processes an AssistantMessage from the CLI. It logs
// the text content, detects auth errors, eagerly parses YAML review
// results, and logs individual content blocks for diagnostics.
func (r *reviewSubActor) handleAssistantMsg(
	ctx context.Context, m claudeagent.AssistantMessage,
	state *messageLoopState, result *SubActorResult,
	startTime time.Time, client *claudeagent.Client,
) {
	text := m.ContentText()
	if text != "" {
		state.lastText = text
		log.InfoS(ctx, "Reviewer assistant text",
			"review_id", r.reviewID, "msg_num", state.msgCount,
			"text_len", len(text), "text_preview", truncateStr(text, 500),
		)

		// Detect auth errors returned as assistant text. The
		// CLI surfaces auth failures as plain text messages
		// rather than error results.
		if isAuthError(text) {
			state.authErr = fmt.Errorf(
				"auth error: %s", strings.TrimSpace(text),
			)
			log.ErrorS(ctx,
				"Reviewer detected auth error, aborting",
				state.authErr, "review_id", r.reviewID,
			)
		}
	}

	// Log each content block for debugging.
	r.logContentBlocks(ctx, m, state.msgCount)
	result.SessionID = m.SessionID

	// Eagerly parse YAML review results from assistant text. The
	// stop hook keeps the CLI alive for follow-up messages, so the
	// ResultMessage may not arrive for minutes.
	if text != "" {
		r.tryParseReviewYAML(
			ctx, text, state, result, startTime, client,
		)
	}
}

// tryParseReviewYAML attempts to parse a YAML review result from
// assistant text. If a valid YAML block is found (and we haven't
// already processed a result), the result is persisted to the database
// and the callback is fired immediately. For terminal decisions
// (approve/reject), the subprocess is killed to avoid the 9m30s stop
// hook timeout. For request_changes, the process stays alive so the
// stop hook can facilitate back-and-forth conversation.
func (r *reviewSubActor) tryParseReviewYAML(
	ctx context.Context, text string, state *messageLoopState,
	result *SubActorResult, startTime time.Time,
	client *claudeagent.Client,
) {
	if state.earlyResult {
		return
	}

	parsed, err := ParseReviewerResponse(text)
	if err != nil || parsed.Decision == "" {
		return
	}

	log.InfoS(ctx, "Reviewer detected YAML result, persisting",
		"review_id", r.reviewID, "decision", parsed.Decision,
		"issues", len(parsed.Issues),
	)

	result.Result = parsed
	result.Duration = time.Since(startTime)
	state.earlyResult = true

	// Persist and notify the FSM immediately.
	r.persistResults(ctx, result, startTime)
	r.callback(ctx, result)

	// For terminal decisions, kill the Claude subprocess
	// immediately. The stop hook would otherwise keep it alive
	// for ~9m30s polling for mail. Close() sends stdin EOF → 5s
	// grace → SIGKILL. For request_changes, we keep the process
	// alive so the stop hook can facilitate back-and-forth.
	if isTerminalDecision(parsed.Decision) {
		log.InfoS(ctx, "Reviewer killing subprocess",
			"review_id", r.reviewID, "decision", parsed.Decision,
		)
		client.Close()
	} else {
		// For non-terminal decisions (request_changes), reset
		// earlyResult so the reviewer can output a new YAML
		// result after processing follow-up messages during the
		// back-and-forth conversation facilitated by the stop
		// hook.
		state.earlyResult = false

		log.InfoS(ctx, "Reviewer allowing subsequent YAML parsing",
			"review_id", r.reviewID, "decision", parsed.Decision,
		)
	}
}

// logContentBlocks logs each content block in an AssistantMessage for
// debugging. Tool use blocks include a truncated preview of their
// arguments.
func (r *reviewSubActor) logContentBlocks(
	ctx context.Context, m claudeagent.AssistantMessage, msgNum int,
) {
	if m.Message.Content == nil {
		return
	}

	for i, block := range m.Message.Content {
		args := ""
		if block.Type == "tool_use" && len(block.Input) > 0 {
			args = truncateStr(string(block.Input), 500)
		}
		log.InfoS(ctx, "Reviewer content block",
			"review_id", r.reviewID, "msg_num", msgNum,
			"block_idx", i, "block_type", block.Type,
			"block_name", block.Name, "tool_args", args,
		)
	}
}

// handleResultMsg processes the final ResultMessage from the CLI. This
// message contains cost, duration, and optionally the final response
// text. It may arrive long after the actual review output if the stop
// hook keeps the CLI alive for follow-up messages.
func (r *reviewSubActor) handleResultMsg(
	ctx context.Context, m claudeagent.ResultMessage,
	state *messageLoopState, result *SubActorResult,
) {
	state.response = m
	state.gotResult = true
	result.SessionID = m.SessionID
	result.CostUSD = m.TotalCostUSD
	result.Duration = time.Duration(
		m.DurationMs,
	) * time.Millisecond

	log.InfoS(ctx, "Reviewer received ResultMessage",
		"review_id", r.reviewID, "session_id", m.SessionID,
		"is_error", m.IsError, "cost_usd", m.TotalCostUSD,
		"duration_ms", m.DurationMs, "result_len", len(m.Result),
		"num_errors", len(m.Errors),
	)
}

// handleToolResultMsg processes a UserMessage (tool result) from the
// CLI. These messages follow tool_use blocks and contain the tool output
// (e.g., Bash stdout). They are logged for diagnostics but do not
// affect the review result.
func (r *reviewSubActor) handleToolResultMsg(
	ctx context.Context, m claudeagent.UserMessage,
	state *messageLoopState,
) {
	var parentToolID string
	if m.ParentToolUseID != nil {
		parentToolID = *m.ParentToolUseID
	}

	// Log each content block with its type so we can see what
	// the CLI actually returned.
	for i, cb := range m.Message.Content {
		log.InfoS(ctx, "Reviewer tool result block",
			"review_id", r.reviewID, "msg_num", state.msgCount,
			"block_idx", i, "block_type", cb.Type,
			"text_len", len(cb.Text), "text_preview", truncateStr(cb.Text, 500),
		)
	}

	// Log the raw ToolUseResult field which may contain the tool
	// output when the content blocks are empty.
	if m.ToolUseResult != nil {
		raw, _ := json.Marshal(m.ToolUseResult)
		log.InfoS(ctx, "Reviewer tool use result",
			"review_id", r.reviewID, "msg_num", state.msgCount,
			"parent_tool_use_id", parentToolID, "raw_result", truncateStr(string(raw), 1000),
		)
	}

	log.InfoS(ctx, "Reviewer tool result",
		"review_id", r.reviewID, "msg_num", state.msgCount,
		"parent_tool_use_id", parentToolID, "content_blocks", len(m.Message.Content),
	)
}

// processPostLoop handles the final result processing after the query
// loop exits. If the YAML result was already parsed eagerly from an
// AssistantMessage (earlyResult == true), this only updates cost
// metadata from the ResultMessage without re-persisting or re-calling
// the callback. Otherwise it performs the standard post-loop processing:
// auth error reporting, ResultMessage parsing, and callback invocation.
func (r *reviewSubActor) processPostLoop(
	ctx context.Context, state *messageLoopState,
	result *SubActorResult, startTime time.Time,
) {
	log.InfoS(ctx, "Reviewer query loop exited",
		"review_id", r.reviewID, "total_messages", state.msgCount,
		"got_result", state.gotResult, "early_result", state.earlyResult,
		"auth_error", state.authErr != nil, "ctx_err", fmt.Sprintf("%v", ctx.Err()),
		"elapsed", time.Since(startTime).String(),
	)

	// Auth errors take priority — the review cannot proceed.
	if state.authErr != nil {
		result.Error = state.authErr
		result.Duration = time.Since(startTime)
		r.callback(ctx, result)

		return
	}

	// If we already detected and persisted the YAML result during
	// the message loop, update cost metadata from the ResultMessage
	// (if we got one) and return.
	if state.earlyResult {
		if state.gotResult {
			result.CostUSD = state.response.TotalCostUSD
			result.Duration = time.Duration(
				state.response.DurationMs,
			) * time.Millisecond
		}

		return
	}

	if !state.gotResult {
		result.Error = fmt.Errorf("no result message received")
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer received no result message",
			result.Error, "review_id", r.reviewID,
			"total_messages", state.msgCount,
			"last_text_preview", truncateStr(state.lastText, 300),
			"duration", result.Duration.String(),
		)
		r.callback(ctx, result)

		return
	}

	if state.response.IsError {
		errMsg := "review failed"
		if len(state.response.Errors) > 0 {
			errMsg = state.response.Errors[0]
		}
		result.Error = fmt.Errorf("reviewer error: %s", errMsg)
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer returned error result",
			result.Error, "review_id", r.reviewID, "duration", result.Duration.String(),
		)
		r.callback(ctx, result)

		return
	}

	// Parse the review result from the response text.
	responseText := state.response.Result
	if responseText == "" {
		responseText = state.lastText
	}

	parsed, err := ParseReviewerResponse(responseText)
	if err != nil {
		result.Error = fmt.Errorf(
			"parse reviewer response: %w", err,
		)
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer failed to parse response YAML",
			err, "review_id", r.reviewID, "duration", result.Duration.String(),
		)
		r.callback(ctx, result)

		return
	}

	result.Result = parsed

	log.InfoS(ctx, "Reviewer completed review",
		"review_id", r.reviewID, "decision", parsed.Decision,
		"issues_found", len(parsed.Issues), "files_reviewed", parsed.FilesReviewed,
		"lines_analyzed", parsed.LinesAnalyzed, "session_id", result.SessionID,
		"cost_usd", result.CostUSD, "duration", result.Duration.String(),
	)

	// Persist the iteration and issues to the database.
	r.persistResults(ctx, result, startTime)

	// Invoke the callback so the service can update the FSM.
	r.callback(ctx, result)
}

// buildClientOptions constructs SDK options from the reviewer and spawn
// configs. The reviewer is fully isolated from user hooks and settings
// via a temporary config directory. This prevents the user's substrate
// Stop hook (which blocks exit indefinitely) from interfering with the
// one-shot review lifecycle. Returns the options and the config dir path
// (empty string if no temp dir was created).
func (r *reviewSubActor) buildClientOptions() ([]claudeagent.Option, string) {
	// Create an isolated config directory so the CLI subprocess does
	// not load the user's hooks, settings, or sessions from ~/.claude.
	// This matches the isolation pattern used by the Claude Agent SDK
	// integration tests (isolatedClientOptions).
	configDir := r.spawnCfg.ConfigDir
	if configDir == "" {
		dir, err := os.MkdirTemp("", "subtrate-reviewer-*")
		if err != nil {
			log.Errorf("Failed to create temp config dir, "+
				"falling back to default: %v", err,
			)
		} else {
			configDir = filepath.Join(dir, ".claude")
			if mkErr := os.MkdirAll(configDir, 0o755); mkErr != nil {
				log.Errorf("Failed to create config dir %s: %v",
					configDir, mkErr,
				)
				configDir = ""
			}
		}
	}

	opts := []claudeagent.Option{
		claudeagent.WithModel(r.config.Model),
		// Route all permission decisions through our read-only
		// policy callback. This provides fine-grained control
		// over which tools are allowed (reads yes, writes only
		// to /tmp/claude/, dangerous bash commands denied).
		claudeagent.WithCanUseTool(
			makeReviewerPermissionPolicy(r.repoPath),
		),
		// NOTE: We intentionally do NOT use
		// WithAllowDangerouslySkipPermissions here. Our
		// CanUseTool callback provides fine-grained control
		// over which operations are allowed.
		// Don't save sessions — reviewers are one-shot tasks.
		claudeagent.WithNoSessionPersistence(),
		// Don't load user/project filesystem settings (which
		// include hooks that interfere with subprocess lifecycle).
		claudeagent.WithSettingSources(nil),
		// Disable skills to prevent --setting-sources from being
		// passed to the CLI (the default SkillsConfig sends
		// --setting-sources user,project which loads project
		// hooks from .claude/settings.json).
		claudeagent.WithSkillsDisabled(),
		// Forward CLI stderr to the reviewer's structured logger
		// for diagnostic visibility into subprocess errors.
		claudeagent.WithStderr(func(data string) {
			log.Infof("Reviewer CLI stderr [%s]: %s",
				r.reviewID, data,
			)
		}),
		// Register substrate hooks for agent messaging. These
		// are Go-native equivalents of the substrate CLI hook
		// scripts, enabling the reviewer to participate in the
		// substrate messaging system directly.
		claudeagent.WithHooks(r.buildSubstrateHooks()),
	}

	// Explicitly forward authentication tokens to the subprocess.
	// When substrated is started without these in its environment
	// (e.g., launched from a non-interactive shell), the reviewer
	// subprocess inherits an empty env and fails with "Invalid API
	// key". Reading from the current environment ensures the token
	// is always available if set anywhere in the process tree.
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

	// Isolate from user config if we have a temp dir.
	if configDir != "" {
		opts = append(
			opts, claudeagent.WithConfigDir(configDir),
		)
		log.Infof("Reviewer using isolated config dir: "+
			"review_id=%s, config_dir=%s",
			r.reviewID, configDir,
		)
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

	// For multi-review mode, register the specialized sub-reviewer
	// agent definitions. The coordinator agent will spawn these via
	// the SDK Task tool to perform parallel domain-specific reviews.
	if r.isMultiReview {
		opts = append(
			opts,
			claudeagent.WithAgents(FullReviewSubAgents()),
		)

		log.Infof("Coordinator configured with %d sub-reviewer "+
			"agents: review_id=%s",
			len(FullReviewSubAgents()), r.reviewID,
		)
	}

	return opts, configDir
}

// buildSystemPrompt constructs the system prompt for the reviewer agent based
// on the ReviewerConfig persona. The prompt follows patterns from Anthropic's
// code-review plugin: explicit exclusion lists, signal-to-noise criteria,
// per-review-type focus guidance, and severity-ordered output.
func (r *reviewSubActor) buildSystemPrompt() string {
	if r.config.SystemPrompt != "" {
		return r.config.SystemPrompt
	}

	// Look up requester name so we can template it into the
	// substrate send command (avoids the agent guessing).
	requesterName := "User"
	if r.requester > 0 {
		requester, err := r.store.GetAgent(
			context.Background(), r.requester,
		)
		if err == nil {
			requesterName = requester.Name
		}
	}

	shortID := r.reviewID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	data := systemPromptData{
		Name:           r.config.Name,
		ReviewerType:   r.config.Name,
		IsMultiReview:  r.isMultiReview,
		FocusAreas:     r.config.FocusAreas,
		IgnorePatterns: r.config.IgnorePatterns,
		AgentName:      r.reviewerAgentName(),
		RequesterName:  requesterName,
		ThreadID:       r.threadID,
		BodyFile: "/tmp/substrate_reviews/review-" +
			shortID + ".md",
		Branch:     r.branch,
		BaseBranch: r.baseBranch,
		ClaudeMD:   r.loadProjectCLAUDEMD(),
	}

	// Use the coordinator prompt for multi-sub-reviewer mode.
	if r.isMultiReview {
		return renderCoordinatorPrompt(
			context.Background(), data,
		)
	}

	return renderSystemPrompt(context.Background(), data)
}

// loadProjectCLAUDEMD reads the CLAUDE.md file from the repo root. Returns
// an empty string if the file doesn't exist or can't be read.
func (r *reviewSubActor) loadProjectCLAUDEMD() string {
	if r.repoPath == "" {
		return ""
	}

	path := filepath.Join(r.repoPath, "CLAUDE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(data)
}

// buildReviewPrompt constructs the initial user prompt sent to the reviewer.
// The prompt templates the git diff command from the review's branch and
// base branch so the reviewer examines exactly the right commit range.
func (r *reviewSubActor) buildReviewPrompt() string {
	data := reviewPromptData{
		ReviewID:   r.reviewID,
		DiffCmd:    r.buildDiffCommand(),
		Branch:     r.branch,
		BaseBranch: r.baseBranch,
	}

	// Use the coordinator review prompt for multi-sub-reviewer mode.
	if r.isMultiReview {
		return renderCoordinatorReviewPrompt(
			context.Background(), data,
		)
	}

	return renderReviewPrompt(context.Background(), data)
}

// buildDiffCommand constructs the appropriate git diff command based on the
// review's branch configuration. It uses the three-dot diff syntax to show
// only the changes unique to the feature branch relative to the base branch.
func (r *reviewSubActor) buildDiffCommand() string {
	// If we have both base and feature branches, use three-dot diff.
	// This shows the diff of changes on the feature branch since it
	// diverged from the base branch, which is what PR reviews need.
	if r.baseBranch != "" && r.branch != "" {
		return fmt.Sprintf(
			"git diff %s...%s", r.baseBranch, r.branch,
		)
	}

	// If we only have a base branch, diff against HEAD.
	if r.baseBranch != "" {
		return fmt.Sprintf("git diff %s...HEAD", r.baseBranch)
	}

	// If we have a specific commit SHA, show that commit's changes.
	if r.commitSHA != "" {
		return fmt.Sprintf("git show %s", r.commitSHA)
	}

	// Fallback: diff the last commit.
	return "git diff HEAD~1"
}

// reviewerAgentName returns the substrate agent name for this reviewer.
// Each reviewer instance gets a unique name based on the reviewer config
// name and a short review ID suffix. Review aggregation happens via the
// CodeReviewer topic, not the agent name.
func (r *reviewSubActor) reviewerAgentName() string {
	// Use branch name for agent identity so reviewers are grouped by
	// what they're reviewing rather than by review ID. This allows
	// the UI to aggregate all reviewer-* agents under "CodeReviewer".
	branch := r.branch
	if branch == "" {
		branch = "unknown"
	}

	// Sanitize branch name: replace slashes with dashes for valid
	// agent naming.
	branch = strings.ReplaceAll(branch, "/", "-")

	return "reviewer-" + branch
}

// buildSubstrateHooks constructs the SDK hook callbacks for substrate
// messaging integration. These hooks mirror the behavior of the substrate
// CLI hook scripts but run as Go callbacks in the daemon process space,
// giving them direct access to the store and mail service.
func (r *reviewSubActor) buildSubstrateHooks() map[claudeagent.HookType][]claudeagent.HookConfig {
	hooks := map[claudeagent.HookType][]claudeagent.HookConfig{
		claudeagent.HookTypeSessionStart: {{
			Matcher:  "*",
			Callback: r.hookSessionStart,
		}},
		claudeagent.HookTypeStop: {{
			Matcher:  "*",
			Callback: r.hookStop,
		}},
	}

	// For multi-review mode, add subagent lifecycle hooks to track
	// when the coordinator spawns and completes sub-reviewer agents.
	if r.isMultiReview {
		hooks[claudeagent.HookTypeSubagentStart] = []claudeagent.HookConfig{{
			Matcher: "*",
			Callback: func(ctx context.Context,
				input claudeagent.HookInput) (
				claudeagent.HookResult, error) {

				start := input.(claudeagent.SubagentStartInput)
				log.InfoS(ctx,
					"Sub-reviewer agent spawned",
					"review_id", r.reviewID,
					"agent_type", start.AgentType,
					"agent_id", start.AgentID,
				)
				return claudeagent.HookResult{
					Continue: true,
				}, nil
			},
		}}
		hooks[claudeagent.HookTypeSubagentStop] = []claudeagent.HookConfig{{
			Matcher: "*",
			Callback: func(ctx context.Context,
				input claudeagent.HookInput) (
				claudeagent.HookResult, error) {

				stop := input.(claudeagent.SubagentStopInput)
				log.InfoS(ctx,
					"Sub-reviewer agent completed",
					"review_id", r.reviewID,
					"agent_name", stop.AgentName,
					"status", stop.Status,
				)
				return claudeagent.HookResult{
					Continue: true,
				}, nil
			},
		}}
	}

	return hooks
}

// registerAgentIdentity creates or retrieves the reviewer's agent record
// in the store and sets r.agentID. This is called eagerly after Connect()
// because the SessionStart hook may not fire for SDK-spawned sessions.
func (r *reviewSubActor) registerAgentIdentity(ctx context.Context) {
	agentName := r.reviewerAgentName()

	log.InfoS(ctx, "Reviewer registering agent identity",
		"review_id", r.reviewID,
		"agent_name", agentName,
	)

	agent, err := r.store.GetAgentByName(ctx, agentName)
	if err != nil {
		// Agent doesn't exist yet, create it.
		agent, err = r.store.CreateAgent(
			ctx, store.CreateAgentParams{
				Name:       agentName,
				ProjectKey: "subtrate",
				GitBranch:  r.branch,
			},
		)
		if err != nil {
			log.ErrorS(ctx,
				"Reviewer failed to register agent identity",
				err,
				"agent_name", agentName,
			)
			return
		}
	}

	r.agentID = agent.ID

	// Send initial heartbeat.
	if err := r.store.UpdateLastActive(
		ctx, agent.ID, time.Now(),
	); err != nil {
		log.WarnS(ctx, "Reviewer initial heartbeat failed",
			err,
			"agent_name", agentName,
		)
	}

	log.InfoS(ctx, "Reviewer agent identity registered",
		"review_id", r.reviewID,
		"agent_name", agentName,
		"agent_id", agent.ID,
	)
}

// hookSessionStart registers the reviewer agent identity with the substrate
// system and sends an initial heartbeat. This is the Go equivalent of the
// substrate session_start.sh hook script. Note: this may not fire for
// SDK-spawned sessions, so registerAgentIdentity is also called eagerly
// after Connect().
func (r *reviewSubActor) hookSessionStart(
	ctx context.Context, input claudeagent.HookInput,
) (claudeagent.HookResult, error) {
	log.InfoS(ctx, "Reviewer substrate hook: session start",
		"review_id", r.reviewID,
		"agent_name", r.reviewerAgentName(),
	)

	// Register identity if not already done by the eager call.
	if r.agentID == 0 {
		r.registerAgentIdentity(ctx)
	}

	return claudeagent.HookResult{Continue: true}, nil
}

// stopPollInterval is how long the Stop hook waits between mail checks.
const stopPollInterval = 10 * time.Second

// stopPollTimeout is the maximum time the Stop hook polls for messages
// before allowing the reviewer to exit. This matches the substrate CLI
// stop hook's 9m30s timeout (under the 10m hook timeout limit).
const stopPollTimeout = 9*time.Minute + 30*time.Second

// hookStop checks for unread messages from the reviewee. If messages exist,
// it blocks exit and injects the message content as a new prompt so the
// reviewer can handle follow-up requests (e.g., "please re-review after
// fixes"). This is the Go equivalent of the substrate stop.sh hook script.
func (r *reviewSubActor) hookStop(
	ctx context.Context, input claudeagent.HookInput,
) (claudeagent.HookResult, error) {
	agentName := r.reviewerAgentName()

	log.InfoS(ctx, "Reviewer substrate hook: stop, checking mail",
		"review_id", r.reviewID,
		"agent_name", agentName,
		"agent_id", r.agentID,
	)

	// If we don't have an agent ID, we can't check mail.
	if r.agentID == 0 {
		log.WarnS(ctx,
			"Reviewer substrate hook: no agent ID, "+
				"allowing exit",
			nil,
			"review_id", r.reviewID,
		)

		return claudeagent.HookResult{
			Decision: "approve",
		}, nil
	}

	// Use a dedicated context for store operations. The SDK
	// context (ctx) may be canceled during shutdown while this
	// hook is still polling in the messagePump goroutine,
	// causing every store call to fail with "context canceled".
	storeCtx, storeCancel := context.WithTimeout(
		context.Background(), stopPollTimeout,
	)
	defer storeCancel()

	// Poll for unread messages with a timeout.
	deadline := time.Now().Add(stopPollTimeout)
	for time.Now().Before(deadline) {
		// Send heartbeat to keep agent active.
		if err := r.store.UpdateLastActive(
			storeCtx, r.agentID, time.Now(),
		); err != nil {
			log.WarnS(ctx,
				"Reviewer substrate hook: heartbeat "+
					"failed during poll",
				err,
				"agent_name", agentName,
			)
		}

		// Check for unread messages. On transient DB errors, log
		// and retry on the next poll iteration rather than
		// immediately approving exit (which could cut a review
		// short if the database is momentarily busy).
		msgs, err := r.store.GetUnreadMessages(
			storeCtx, r.agentID, 10,
		)
		if err != nil {
			log.ErrorS(ctx,
				"Reviewer substrate hook: mail check "+
					"failed, will retry",
				err,
				"agent_name", agentName,
			)

			time.Sleep(stopPollInterval)

			continue
		}

		if len(msgs) > 0 {
			// Mark messages as read so they are not
			// re-injected on subsequent stop hook
			// invocations.
			for _, msg := range msgs {
				if err := r.store.MarkMessageRead(
					storeCtx, msg.ID, r.agentID,
				); err != nil {
					log.WarnS(ctx,
						"Reviewer substrate hook: "+
							"mark read failed",
						err,
						"msg_id", msg.ID,
					)
				}
			}

			// Format messages as a prompt including the
			// diff command for re-review context.
			diffCmd := r.buildDiffCommand()
			reason := formatMailAsReReviewPrompt(
				msgs, diffCmd,
			)

			log.InfoS(ctx,
				"Reviewer substrate hook: blocking "+
					"exit, injecting mail",
				"review_id", r.reviewID,
				"message_count", len(msgs),
			)

			return claudeagent.HookResult{
				Decision: "block",
				Reason:   reason,
				SystemMessage: fmt.Sprintf(
					"You have %d unread "+
						"message(s) with "+
						"feedback on your "+
						"review. Re-read the "+
						"diff, address the "+
						"feedback, and emit an "+
						"updated YAML review.",
					len(msgs),
				),
			}, nil
		}

		// No messages, wait before next poll. Exit early if
		// the SDK context is canceled (shutdown requested).
		select {
		case <-ctx.Done():
			return claudeagent.HookResult{
				Decision: "approve",
			}, nil
		case <-time.After(stopPollInterval):
			continue
		}
	}

	log.InfoS(ctx,
		"Reviewer substrate hook: no messages after polling, "+
			"allowing exit",
		"review_id", r.reviewID,
		"poll_duration", stopPollTimeout.String(),
	)

	return claudeagent.HookResult{Decision: "approve"}, nil
}

// formatMailAsReReviewPrompt converts unread inbox messages into a
// structured prompt for the reviewer using the re-review template. It
// includes the feedback messages and the diff command so Claude can
// re-review the code and emit an updated YAML review block.
func formatMailAsReReviewPrompt(
	msgs []store.InboxMessage, diffCmd string,
) string {

	tmplMsgs := make([]reReviewMessage, len(msgs))
	for i, msg := range msgs {
		tmplMsgs[i] = reReviewMessage{
			Index:      i + 1,
			SenderName: msg.SenderName,
			Subject:    msg.Subject,
			Body:       msg.Body,
		}
	}

	return renderReReviewPrompt(reReviewPromptData{
		Messages: tmplMsgs,
		DiffCmd:  diffCmd,
	})
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
		log.ErrorS(ctx, "Reviewer failed to get iterations for persist",
			err,
			"review_id", r.reviewID,
		)
		return
	}
	iterNum := len(iters) + 1

	log.InfoS(ctx, "Reviewer persisting iteration results",
		"review_id", r.reviewID,
		"iteration_num", iterNum,
		"decision", parsed.Decision,
		"issues_count", len(parsed.Issues),
	)

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
		log.ErrorS(ctx, "Reviewer failed to create iteration record",
			err,
			"review_id", r.reviewID,
			"iteration_num", iterNum,
		)
		return
	}

	// Create issue records for each issue found.
	for i, issue := range parsed.Issues {
		// Normalize the issue type to match the DB CHECK
		// constraint. The reviewer agent may use shorthand
		// names that need mapping to the canonical types.
		issueType := normalizeIssueType(issue.IssueType)

		_, err := r.store.CreateReviewIssue(
			ctx, store.CreateReviewIssueParams{
				ReviewID:     r.reviewID,
				IterationNum: iterNum,
				IssueType:    issueType,
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
			log.ErrorS(ctx, "Reviewer failed to create issue record",
				err,
				"review_id", r.reviewID,
				"issue_index", i,
				"issue_title", issue.Title,
			)
		}
	}

	log.InfoS(ctx, "Reviewer persisted results successfully",
		"review_id", r.reviewID,
		"iteration_num", iterNum,
		"issues_persisted", len(parsed.Issues),
	)
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

// validIssueTypes is the set of issue types accepted by the review_issues
// CHECK constraint in the database schema.
var validIssueTypes = map[string]bool{
	"bug":                 true,
	"security":            true,
	"claude_md_violation": true,
	"logic_error":         true,
	"performance":         true,
	"style":               true,
	"documentation":       true,
	"other":               true,
}

// issueTypeAliases maps common shorthand or alternate names to their
// canonical DB-compatible issue type.
var issueTypeAliases = map[string]string{
	"logic":        "logic_error",
	"architecture": "other",
	"arch":         "other",
	"refactor":     "other",
	"testing":      "other",
	"test":         "other",
	"docs":         "documentation",
	"doc":          "documentation",
	"claude_md":    "claude_md_violation",
}

// normalizeIssueType maps a reviewer-provided issue type to a valid DB
// value. If the type is already valid, it is returned as-is. If it matches
// a known alias, the canonical form is returned. Otherwise "other" is used.
func normalizeIssueType(issueType string) string {
	lower := strings.ToLower(strings.TrimSpace(issueType))

	if validIssueTypes[lower] {
		return lower
	}

	if canonical, ok := issueTypeAliases[lower]; ok {
		return canonical
	}

	return "other"
}

// authErrorPatterns are substrings in CLI assistant text that indicate an
// authentication failure. When detected, the review should be cancelled
// immediately since the reviewer cannot proceed without valid credentials.
var authErrorPatterns = []string{
	"Invalid API key",
	"Please run /login",
	"authentication failed",
	"unauthorized",
	"invalid_api_key",
	"expired token",
}

// isAuthError returns true if the text contains a known authentication
// error pattern. The CLI surfaces auth failures as plain assistant text
// rather than structured error results.
func isAuthError(text string) bool {
	lower := strings.ToLower(text)
	for _, pattern := range authErrorPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// truncateStr truncates a string to maxLen characters, appending "..." if
// the string was shortened.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// readOnlyTools is the set of Claude Code tool names that are safe for
// read-only reviewer agents. These tools cannot modify the filesystem.
var readOnlyTools = map[string]bool{
	"Read":         true,
	"Glob":         true,
	"Grep":         true,
	"LS":           true,
	"WebFetch":     true,
	"WebSearch":    true,
	"NotebookRead": true,
	"Bash":         true, // Bash is allowed but command-filtered below.
}

// writeTools is the set of tools that can modify the filesystem or spawn
// subprocesses. These are always denied for reviewer agents.
var writeTools = map[string]bool{
	"Edit":         true,
	"MultiEdit":    true,
	"NotebookEdit": true,
	"Task":         true,
	"TodoWrite":    true,
}

// bashDangerousPrefixes lists command prefixes that indicate destructive
// Bash operations. Commands starting with any of these are denied.
var bashDangerousPrefixes = []string{
	"rm ", "rm\t", "rmdir ",
	"mv ", "cp ",
	"chmod ", "chown ",
	"git push", "git checkout", "git reset",
	"git rebase", "git merge", "git commit",
	"git add", "git stash",
	"curl ", "wget ",
	"pip ", "npm ", "go install",
	"make ",
	// Block environment variable enumeration to prevent leaking
	// forwarded auth tokens (CLAUDE_CODE_OAUTH_TOKEN, etc.).
	"env ", "env\t",
	"printenv ", "printenv\t",
	"export ", "export\t",
	"set ", "set\t",
}

// bashDangerousExact lists commands that should be blocked even when
// invoked with no arguments (bare command). For example, "env" with no
// args dumps the full environment including auth tokens.
var bashDangerousExact = map[string]bool{
	"env":      true,
	"printenv": true,
	"set":      true,
	"export":   true,
}

// reviewWriteDir is the directory where reviewers are allowed to write
// review body files. Using /tmp avoids cluttering the project directory.
const reviewWriteDir = "/tmp/substrate_reviews"

// makeReviewerPermissionPolicy creates a CanUseTool callback for
// reviewer agents. It enforces a read-only policy: read tools are
// allowed, write tools are denied, and Bash commands are filtered to
// prevent filesystem mutations. The Write tool is allowed only for
// /tmp/substrate_reviews/ paths so the reviewer can write review body
// content for substrate send --body-file.
func makeReviewerPermissionPolicy(
	_ string,
) claudeagent.CanUseToolFunc {
	// Resolve the canonical write prefix, handling macOS /tmp →
	// /private/tmp symlink. We accept both the symlink and the
	// resolved path so writes work regardless of whether
	// filepath.Clean resolves the symlink or not.
	canonicalDir := reviewWriteDir
	if resolved, err := filepath.EvalSymlinks(
		reviewWriteDir,
	); err == nil {
		canonicalDir = resolved
	}

	prefixes := []string{
		reviewWriteDir + string(filepath.Separator),
		canonicalDir + string(filepath.Separator),
	}

	return func(
		_ context.Context, req claudeagent.ToolPermissionRequest,
	) claudeagent.PermissionResult {
		return reviewerPermissionPolicy(req, prefixes)
	}
}

// reviewerPermissionPolicy is the core permission logic for reviewer
// agents. The allowedWritePrefixes parameter lists the absolute path
// prefixes where the Write tool is allowed (e.g.,
// ["/tmp/substrate_reviews/", "/private/tmp/substrate_reviews/"]).
func reviewerPermissionPolicy(
	req claudeagent.ToolPermissionRequest,
	allowedWritePrefixes []string,
) claudeagent.PermissionResult {
	toolName := req.ToolName

	// Log every permission request for diagnostics.
	argPreview := truncateStr(string(req.Arguments), 200)
	log.Infof("Reviewer permission check: tool=%s args=%s",
		toolName, argPreview,
	)

	var result claudeagent.PermissionResult

	// Allow Write tool only to .reviews/ paths for review body
	// files. The CLI sandbox allows writes to the CWD (project
	// dir), so writing to {repoPath}/.reviews/ works through
	// both our callback and the CLI sandbox.
	if toolName == "Write" {
		result = checkWritePath(req.Arguments, allowedWritePrefixes)
	} else if writeTools[toolName] {
		// Deny known write tools immediately.
		result = claudeagent.PermissionDeny{
			Reason: fmt.Sprintf(
				"tool %q is not allowed in read-only "+
					"review mode", toolName,
			),
		}
	} else if toolName == "Bash" {
		// For Bash, inspect the command to block destructive
		// operations.
		result = checkBashCommand(req.Arguments)
	} else if readOnlyTools[toolName] {
		// Allow known read-only tools.
		result = claudeagent.PermissionAllow{}
	} else {
		// Default: deny unknown tools for safety.
		result = claudeagent.PermissionDeny{
			Reason: fmt.Sprintf(
				"unknown tool %q is not allowed in "+
					"read-only review mode", toolName,
			),
		}
	}

	// Log the decision.
	if result.IsAllow() {
		log.Infof("Reviewer permission ALLOW: tool=%s",
			toolName,
		)
	} else {
		deny, _ := result.(claudeagent.PermissionDeny)
		log.Infof("Reviewer permission DENY: tool=%s reason=%s",
			toolName, deny.Reason,
		)
	}

	return result
}

// writeArgs is the JSON structure of Write tool arguments.
type writeArgs struct {
	FilePath string `json:"file_path"`
}

// checkWritePath allows the Write tool only when the target path is
// under one of the allowed prefixes. This lets the reviewer write
// review body markdown to /tmp/substrate_reviews/ without granting
// general filesystem write access.
func checkWritePath(
	arguments json.RawMessage, allowedPrefixes []string,
) claudeagent.PermissionResult {
	var args writeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		log.Infof("Reviewer Write permission: failed to "+
			"parse args: %v (raw: %s)", err,
			truncateStr(string(arguments), 200),
		)

		return claudeagent.PermissionDeny{
			Reason: "failed to parse Write arguments",
		}
	}

	path := filepath.Clean(args.FilePath)

	// Check if the path is under any allowed prefix. Multiple
	// prefixes handle macOS /tmp → /private/tmp symlink resolution.
	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			allowed = true
			break
		}
	}

	log.Infof("Reviewer Write permission check: "+
		"raw_path=%q cleaned_path=%q prefixes=%v allowed=%v",
		args.FilePath, path, allowedPrefixes, allowed,
	)

	if !allowed {
		log.Infof("Reviewer Write DENIED: path=%q does not "+
			"match any allowed prefix",
			path,
		)

		return claudeagent.PermissionDeny{
			Reason: fmt.Sprintf(
				"Write to %q denied: reviewer can only "+
					"write to %s/",
				path, reviewWriteDir,
			),
		}
	}

	return claudeagent.PermissionAllow{}
}

// bashArgs is the JSON structure of Bash tool arguments from Claude Code.
type bashArgs struct {
	Command string `json:"command"`
}

// bashChainOperators are shell metacharacters that allow chaining
// multiple commands. We split on these to check each sub-command
// independently, preventing bypass via "safe; dangerous".
var bashChainOperators = []string{
	"&&", "||", ";", "|", "\n",
}

// bashSubshellPatterns match shell constructs that can embed
// arbitrary commands: $(...), `...`, and process substitution.
var bashSubshellPatterns = []string{
	"$(", "`",
	"<(", ">(",
}

// checkBashCommand inspects the Bash command and denies destructive
// operations while allowing read-only commands like git diff, git log,
// etc. It splits chained commands on shell operators (;, &&, ||, |) and
// checks each sub-command independently. It also rejects subshell
// constructs like $() and backticks that could embed dangerous commands.
func checkBashCommand(
	arguments json.RawMessage,
) claudeagent.PermissionResult {
	var args bashArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return claudeagent.PermissionDeny{
			Reason: "failed to parse Bash arguments",
		}
	}

	cmd := strings.TrimSpace(args.Command)

	// Reject subshell constructs that can embed arbitrary commands.
	// These bypass prefix-based filtering because the dangerous
	// command is nested inside an otherwise safe-looking command.
	for _, pattern := range bashSubshellPatterns {
		if strings.Contains(cmd, pattern) {
			return claudeagent.PermissionDeny{
				Reason: fmt.Sprintf(
					"subshell construct %q is not "+
						"allowed in read-only "+
						"review mode",
					pattern,
				),
			}
		}
	}

	// Split on chain operators and check each sub-command. This
	// prevents bypass via "git log; rm -rf /" where only the first
	// token is checked.
	subCmds := splitOnChainOperators(cmd)
	for _, sub := range subCmds {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}

		// Check for exact bare-command matches (e.g., "env" with
		// no arguments) that would dump sensitive info.
		if bashDangerousExact[sub] {
			return claudeagent.PermissionDeny{
				Reason: fmt.Sprintf(
					"bash command %q is not "+
						"allowed in read-only"+
						" review mode",
					sub,
				),
			}
		}

		// Check against dangerous command prefixes.
		for _, prefix := range bashDangerousPrefixes {
			if strings.HasPrefix(sub, prefix) {
				return claudeagent.PermissionDeny{
					Reason: fmt.Sprintf(
						"bash command %q is not "+
							"allowed in read-only"+
							" review mode",
						truncateStr(sub, 80),
					),
				}
			}
		}
	}

	// Deny output redirects that create/overwrite files. The
	// reviewer should use the Write tool for file creation (writing
	// to .reviews/), not Bash redirects. Allow stderr redirects
	// (2>&1) since those are diagnostic.
	if strings.Contains(cmd, ">") && !isOnlyStderrRedirect(cmd) {
		log.Infof("Reviewer Bash redirect DENIED: cmd=%s",
			truncateStr(cmd, 200),
		)

		return claudeagent.PermissionDeny{
			Reason: "output redirection is not allowed " +
				"in read-only review mode; use the " +
				"Write tool to write files",
		}
	}

	return claudeagent.PermissionAllow{}
}

// splitOnChainOperators splits a command string on shell chain
// operators (;, &&, ||, |, newline) to extract individual sub-commands.
// This is a best-effort split for security filtering — it does not
// handle quoted strings or escape sequences.
func splitOnChainOperators(cmd string) []string {
	// Replace multi-char operators with a sentinel, then split on
	// the sentinel and single-char operators.
	const sentinel = "\x00"

	result := cmd
	for _, op := range bashChainOperators {
		result = strings.ReplaceAll(result, op, sentinel)
	}

	return strings.Split(result, sentinel)
}

// isOnlyStderrRedirect returns true when the command's only redirect
// is 2>&1, which is a safe diagnostic pattern.
func isOnlyStderrRedirect(cmd string) bool {
	// Remove all occurrences of 2>&1 and check if any > remains.
	stripped := strings.ReplaceAll(cmd, "2>&1", "")

	return !strings.Contains(stripped, ">")
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

// SubActorManager manages active reviewer sub-actors by registering them
// with the actor system. Each reviewer is a proper actor with lifecycle
// management, WaitGroup tracking, and OnStop cleanup hooks.
type SubActorManager struct {
	mu          sync.Mutex
	actorIDs    map[string]string // reviewID → actor system ID.
	store       store.Storage
	spawnCfg    *SpawnConfig
	actorSystem *actor.ActorSystem
}

// NewSubActorManager creates a new sub-actor manager that registers reviewer
// actors with the given actor system.
func NewSubActorManager(
	as *actor.ActorSystem, st store.Storage, spawnCfg *SpawnConfig,
) *SubActorManager {
	if spawnCfg == nil {
		spawnCfg = DefaultSubActorSpawnConfig()
	}

	return &SubActorManager{
		actorIDs:    make(map[string]string),
		store:       st,
		spawnCfg:    spawnCfg,
		actorSystem: as,
	}
}

// SpawnReviewer creates and starts a reviewer sub-actor for the given review.
// The sub-actor is registered with the actor system as a proper actor, giving
// it automatic lifecycle management and graceful shutdown support. The branch,
// baseBranch, and commitSHA are used to template the git diff command in the
// reviewer's prompt. The callback is invoked when the reviewer completes.
func (m *SubActorManager) SpawnReviewer(
	ctx context.Context,
	reviewID, threadID, repoPath string,
	requester int64,
	branch, baseBranch, commitSHA string,
	config *ReviewerConfig,
	callback func(ctx context.Context, result *SubActorResult),
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Avoid duplicate spawns for the same review.
	if _, exists := m.actorIDs[reviewID]; exists {
		log.WarnS(ctx, "Sub-actor manager: reviewer already active, "+
			"skipping duplicate spawn",
			nil,
			"review_id", reviewID,
		)
		return
	}

	log.InfoS(ctx, "Sub-actor manager spawning reviewer",
		"review_id", reviewID,
		"thread_id", threadID,
		"reviewer", config.Name,
		"repo_path", repoPath,
		"requester_id", requester,
	)

	// Create the sub-actor which implements ActorBehavior.
	sub := newReviewSubActor(
		reviewID, threadID, repoPath, requester,
		branch, baseBranch, commitSHA,
		config, m.store, m.spawnCfg,
		func(ctx context.Context, result *SubActorResult) {
			// Remove from tracking when done.
			m.mu.Lock()
			delete(m.actorIDs, reviewID)
			m.mu.Unlock()

			log.InfoS(ctx, "Sub-actor manager: reviewer completed, "+
				"removed from tracking",
				"review_id", reviewID,
				"active_count", m.ActiveCount(),
			)

			// Forward to the service callback.
			callback(ctx, result)
		},
	)

	// Enable multi-sub-reviewer mode for the coordinator config.
	// The coordinator delegates to specialized sub-agents via the
	// SDK's WithAgents() mechanism.
	if config.Name == "CoordinatorReviewer" {
		sub.isMultiReview = true

		log.InfoS(ctx, "Sub-actor manager: multi-sub-reviewer "+
			"mode enabled",
			"review_id", reviewID,
			"config", config.Name,
		)
	}

	// Register as a proper actor in the system with extended cleanup
	// timeout for subprocess shutdown (SDK uses 5s grace + SIGKILL).
	actorID := fmt.Sprintf("reviewer-%s", reviewID)
	ref := actor.RegisterWithSystem(
		m.actorSystem, actorID, ReviewerActorKey, sub,
		actor.WithCleanupTimeout(15*time.Second),
	)

	m.actorIDs[reviewID] = actorID

	log.InfoS(ctx, "Sub-actor manager registered reviewer with actor system",
		"review_id", reviewID,
		"actor_id", actorID,
		"active_count", len(m.actorIDs),
	)

	// Send the run message to kick off the review.
	ref.Tell(ctx, RunReviewMsg{})
}

// StopReviewer stops a running reviewer sub-actor by removing it from the
// actor system. The system calls OnStop which cancels the review context.
func (m *SubActorManager) StopReviewer(reviewID string) {
	m.mu.Lock()
	actorID, ok := m.actorIDs[reviewID]
	if ok {
		delete(m.actorIDs, reviewID)
	}
	m.mu.Unlock()

	if ok {
		log.Infof("Sub-actor manager stopping reviewer: "+
			"review_id=%s, actor_id=%s", reviewID, actorID,
		)
		m.actorSystem.StopAndRemoveActor(actorID)
	}
}

// StopAll stops all active reviewer sub-actors. Called during graceful
// shutdown as a safety net (the actor system also stops them directly).
func (m *SubActorManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.actorIDs)
	if count > 0 {
		log.Infof("Sub-actor manager stopping all reviewers: "+
			"active_count=%d", count,
		)
	}

	for reviewID, actorID := range m.actorIDs {
		log.Infof("Sub-actor manager stopping reviewer: "+
			"review_id=%s, actor_id=%s", reviewID, actorID,
		)
		m.actorSystem.StopAndRemoveActor(actorID)
		delete(m.actorIDs, reviewID)
	}

	if count > 0 {
		log.Infof("Sub-actor manager stopped all reviewers: "+
			"stopped_count=%d", count,
		)
	}
}

// ActiveCount returns the number of active reviewer sub-actors.
func (m *SubActorManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.actorIDs)
}

// IsActive returns true if a reviewer sub-actor is currently running for
// the given review ID. This is used by the resubmit flow to determine
// whether to send mail to the existing reviewer (whose stop hook is
// polling) or to spawn a fresh reviewer.
func (m *SubActorManager) IsActive(reviewID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.actorIDs[reviewID]
	return ok
}
