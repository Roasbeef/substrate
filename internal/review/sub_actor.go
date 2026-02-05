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

	// NoSessionPersistence disables session saving.
	NoSessionPersistence bool

	// ConfigDir overrides ~/.claude for isolation.
	ConfigDir string
}

// DefaultSubActorSpawnConfig returns the default spawn config for reviewers.
func DefaultSubActorSpawnConfig() *SpawnConfig {
	return &SpawnConfig{
		CLIPath:  "claude",
		MaxTurns: 20,
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

	// isTerminalDecision returns true for review decisions that end the
	// review lifecycle (no further back-and-forth expected).
	isTerminalDecision := func(decision string) bool {
		return decision == "approve" || decision == "reject"
	}

	var (
		lastText    string
		response    claudeagent.ResultMessage
		gotResult   bool
		earlyResult bool
		msgCount    int
	)

	for msg := range client.Query(ctx, prompt) {
		msgCount++

		log.InfoS(ctx, "Reviewer received message from CLI",
			"review_id", r.reviewID,
			"msg_num", msgCount,
			"msg_type", fmt.Sprintf("%T", msg),
		)

		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			text := m.ContentText()
			if text != "" {
				lastText = text
				log.InfoS(ctx, "Reviewer assistant text",
					"review_id", r.reviewID,
					"msg_num", msgCount,
					"text_len", len(text),
					"text_preview",
					truncateStr(text, 500),
				)
			}

			// Log each content block for debugging,
			// including tool arguments for tool_use blocks.
			if m.Message.Content != nil {
				for i, block := range m.Message.Content {
					args := ""
					if block.Type == "tool_use" &&
						len(block.Input) > 0 {

						args = truncateStr(
							string(block.Input), 500,
						)
					}
					log.InfoS(ctx,
						"Reviewer content block",
						"review_id", r.reviewID,
						"msg_num", msgCount,
						"block_idx", i,
						"block_type", block.Type,
						"block_name", block.Name,
						"tool_args", args,
					)
				}
			}
			result.SessionID = m.SessionID

			// Early detection: if the assistant text
			// contains a valid YAML review result, persist
			// it and invoke the callback immediately so the
			// FSM transitions without waiting for the
			// subprocess to exit (the stop hook may keep it
			// alive for follow-up messages). We continue
			// reading the stream for potential back-and-
			// forth conversation.
			if text != "" && !earlyResult {
				if parsed, pErr := ParseReviewerResponse(
					text,
				); pErr == nil && parsed.Decision != "" {

					log.InfoS(ctx,
						"Reviewer detected YAML "+
							"result in assistant "+
							"text, persisting now",
						"review_id", r.reviewID,
						"decision", parsed.Decision,
						"issues",
						len(parsed.Issues),
					)

					result.Result = parsed
					result.Duration = time.Since(
						startTime,
					)
					earlyResult = true

					// Persist and notify the FSM
					// immediately.
					r.persistResults(
						ctx, result, startTime,
					)
					r.callback(ctx, result)

					// For terminal decisions (approve/
					// reject), kill the Claude subprocess
					// immediately. The stop hook would
					// otherwise keep it alive for ~9m30s
					// polling for mail. Close() sends
					// stdin EOF → 5s grace → SIGKILL.
					// For request_changes, we keep the
					// process alive so the stop hook can
					// facilitate back-and-forth.
					if isTerminalDecision(
						parsed.Decision,
					) {
						log.InfoS(ctx,
							"Reviewer killing "+
								"subprocess "+
								"after terminal "+
								"decision",
							"review_id",
							r.reviewID,
							"decision",
							parsed.Decision,
						)

						client.Close()
					}
				}
			}

		case claudeagent.ResultMessage:
			response = m
			gotResult = true
			result.SessionID = m.SessionID
			result.CostUSD = m.TotalCostUSD
			result.Duration = time.Duration(
				m.DurationMs,
			) * time.Millisecond

			log.InfoS(ctx, "Reviewer received ResultMessage",
				"review_id", r.reviewID,
				"session_id", m.SessionID,
				"is_error", m.IsError,
				"cost_usd", m.TotalCostUSD,
				"duration_ms", m.DurationMs,
				"result_len", len(m.Result),
				"num_errors", len(m.Errors),
			)

		case claudeagent.UserMessage:
			// Log tool result content for diagnostics.
			// The UserMessage following a tool_use contains
			// the Bash output, which is critical for debugging
			// substrate CLI failures.
			var parentToolID string
			if m.ParentToolUseID != nil {
				parentToolID = *m.ParentToolUseID
			}

			// Log each content block with its type so we
			// can see what the CLI actually returned (the
			// Text field may be empty for non-text types).
			for i, cb := range m.Message.Content {
				log.InfoS(ctx,
					"Reviewer tool result block",
					"review_id", r.reviewID,
					"msg_num", msgCount,
					"block_idx", i,
					"block_type", cb.Type,
					"text_len", len(cb.Text),
					"text_preview",
					truncateStr(cb.Text, 500),
				)
			}

			// Also log the raw ToolUseResult field which
			// may contain the tool output when the content
			// blocks are empty.
			if m.ToolUseResult != nil {
				raw, _ := json.Marshal(m.ToolUseResult)
				log.InfoS(ctx,
					"Reviewer tool use result",
					"review_id", r.reviewID,
					"msg_num", msgCount,
					"parent_tool_use_id",
					parentToolID,
					"raw_result",
					truncateStr(string(raw), 1000),
				)
			}

			log.InfoS(ctx, "Reviewer tool result",
				"review_id", r.reviewID,
				"msg_num", msgCount,
				"parent_tool_use_id", parentToolID,
				"content_blocks",
				len(m.Message.Content),
			)

		default:
			log.InfoS(ctx, "Reviewer received other message type",
				"review_id", r.reviewID,
				"msg_num", msgCount,
				"type", fmt.Sprintf("%T", msg),
			)
		}
	}

	// Log why the loop exited.
	log.InfoS(ctx, "Reviewer query loop exited",
		"review_id", r.reviewID,
		"total_messages", msgCount,
		"got_result", gotResult,
		"early_result", earlyResult,
		"ctx_err", fmt.Sprintf("%v", ctx.Err()),
		"elapsed", time.Since(startTime).String(),
	)

	// If we already detected and persisted the YAML result during the
	// message loop, nothing more to do.
	if earlyResult {
		return
	}

	if !gotResult {
		result.Error = fmt.Errorf("no result message received")
		result.Duration = time.Since(startTime)
		log.ErrorS(ctx, "Reviewer received no result message",
			result.Error,
			"review_id", r.reviewID,
			"total_messages", msgCount,
			"last_text_preview",
			truncateStr(lastText, 300),
			"duration", result.Duration.String(),
		)
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
		log.ErrorS(ctx, "Reviewer returned error result",
			result.Error,
			"review_id", r.reviewID,
			"duration", result.Duration.String(),
		)
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
		log.ErrorS(ctx, "Reviewer failed to parse response YAML",
			err,
			"review_id", r.reviewID,
			"duration", result.Duration.String(),
		)
		r.callback(ctx, result)
		return
	}

	result.Result = parsed

	log.InfoS(ctx, "Reviewer completed review",
		"review_id", r.reviewID,
		"decision", parsed.Decision,
		"issues_found", len(parsed.Issues),
		"files_reviewed", parsed.FilesReviewed,
		"lines_analyzed", parsed.LinesAnalyzed,
		"session_id", result.SessionID,
		"cost_usd", result.CostUSD,
		"duration", result.Duration.String(),
	)

	// Persist the iteration and issues to the database.
	r.persistResults(ctx, result, startTime)

	// Invoke the callback so the service can update the FSM.
	r.callback(ctx, result)
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

	if r.spawnCfg.MaxTurns > 0 {
		opts = append(
			opts,
			claudeagent.WithMaxTurns(r.spawnCfg.MaxTurns),
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

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"You are %s, a code reviewer.\n\n",
		r.config.Name,
	))

	// Role section — varies by review type.
	sb.WriteString("## Your Role\n")
	sb.WriteString(
		"Review the code changes on the current branch " +
			"compared to the base branch. ",
	)

	switch r.config.Name {
	case "SecurityReviewer":
		sb.WriteString(
			"You are an elite security code reviewer. " +
				"Your primary mission is identifying " +
				"and preventing vulnerabilities before " +
				"they reach production.\n\n",
		)
	case "PerformanceReviewer":
		sb.WriteString(
			"You are a performance engineering " +
				"specialist. Focus on efficiency across " +
				"algorithmic, data access, and resource " +
				"management layers.\n\n",
		)
	case "ArchitectureReviewer":
		sb.WriteString(
			"You are an architecture reviewer. Focus " +
				"on design patterns, interface contracts, " +
				"separation of concerns, and long-term " +
				"maintainability.\n\n",
		)
	default:
		sb.WriteString(
			"Identify bugs, security issues, logic " +
				"errors, and CLAUDE.md violations.\n\n",
		)
	}

	// Per-type detailed guidance.
	r.writeReviewTypeGuidance(&sb)

	// Explicit exclusion list — critical for signal-to-noise.
	sb.WriteString("## What NOT to Flag\n")
	sb.WriteString(
		"Do NOT report any of the following. These are " +
			"explicitly excluded to keep the review " +
			"high-signal:\n\n",
	)
	sb.WriteString(
		"- **Style preferences** — formatting, naming " +
			"conventions, comment style (unless violating " +
			"explicit CLAUDE.md rules)\n",
	)
	sb.WriteString(
		"- **Linter-catchable issues** — unused imports, " +
			"unreachable code, simple type errors (these " +
			"are caught by `make lint`)\n",
	)
	sb.WriteString(
		"- **Pre-existing issues** — problems in code that " +
			"was NOT modified in this diff\n",
	)
	sb.WriteString(
		"- **Subjective suggestions** — \"consider using X " +
			"instead\" without a concrete bug or violation\n",
	)
	sb.WriteString(
		"- **Pedantic nitpicks** — minor wording in " +
			"comments, import ordering preferences\n",
	)
	sb.WriteString(
		"- **Hypothetical issues** — problems that require " +
			"specific unusual inputs to trigger and are not " +
			"realistic in the codebase's context\n\n",
	)

	// Signal criteria.
	sb.WriteString("## High-Signal Findings (What TO Flag)\n")
	sb.WriteString(
		"Focus on findings that are **definitively " +
			"wrong**, not just potentially suboptimal:\n\n",
	)
	sb.WriteString(
		"- **Compilation/parse failures** — syntax errors, " +
			"type errors, missing imports, unresolved " +
			"references\n",
	)
	sb.WriteString(
		"- **Definitive logic failures** — code that will " +
			"produce incorrect results for normal inputs\n",
	)
	sb.WriteString(
		"- **Security vulnerabilities** — injection, auth " +
			"bypass, data exposure, race conditions\n",
	)
	sb.WriteString(
		"- **Resource leaks** — unclosed connections, " +
			"goroutine leaks, missing defers\n",
	)
	sb.WriteString(
		"- **CLAUDE.md violations** — explicit, quotable " +
			"rule violations (include the rule text in " +
			"claude_md_ref)\n",
	)
	sb.WriteString(
		"- **Missing error handling** — errors silently " +
			"discarded or not propagated at boundaries\n\n",
	)

	// Focus areas from config.
	if len(r.config.FocusAreas) > 0 {
		sb.WriteString("## Additional Focus Areas\n")
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

	// Review process.
	sb.WriteString("## Review Process\n")
	sb.WriteString(
		"1. **Read the diff** using the provided git " +
			"command\n",
	)
	sb.WriteString(
		"2. **Produce a change summary** — briefly " +
			"describe what the diff modifies (2-3 " +
			"sentences)\n",
	)
	sb.WriteString(
		"3. **Detailed analysis** — examine each file " +
			"for issues from the high-signal list\n",
	)
	sb.WriteString(
		"4. **Positive observations** — note what was " +
			"done well (good patterns, thorough tests, " +
			"clean interfaces)\n",
	)
	sb.WriteString(
		"5. **Prioritize by impact-to-effort ratio** — " +
			"recommend high-ROI fixes first\n",
	)
	sb.WriteString(
		"6. **Emit YAML result** with your decision " +
			"and issues\n\n",
	)

	// Output format.
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
	sb.WriteString("    type: bug | security | logic_error | ")
	sb.WriteString("performance | style | documentation | ")
	sb.WriteString("claude_md_violation | other\n")
	sb.WriteString("    severity: critical | high | medium | low\n")
	sb.WriteString("    file: \"path/to/file.go\"\n")
	sb.WriteString("    line_start: 42\n")
	sb.WriteString("    line_end: 50\n")
	sb.WriteString("    description: \"Detailed description\"\n")
	sb.WriteString("    code_snippet: \"the problematic code\"\n")
	sb.WriteString("    suggestion: \"Suggested fix or code example\"\n")
	sb.WriteString("    claude_md_ref: \"Quoted CLAUDE.md rule, " +
		"if applicable\"\n")
	sb.WriteString("```\n\n")
	sb.WriteString(
		"**Decision guidelines:**\n",
	)
	sb.WriteString(
		"- `approve` — No issues, or only minor/suggestion " +
			"level findings\n",
	)
	sb.WriteString(
		"- `request_changes` — Issues found that should be " +
			"fixed before merging\n",
	)
	sb.WriteString(
		"- `reject` — Critical issues or fundamental " +
			"design problems\n\n",
	)
	sb.WriteString(
		"If the code looks good and you approve, set " +
			"decision to 'approve' with an empty issues list.\n",
	)

	// Add substrate messaging instructions so the agent can
	// communicate its review results via the substrate system.
	sb.WriteString("## Substrate Messaging\n")
	sb.WriteString(
		"You are a substrate agent. After completing your " +
			"review, send a **detailed** review mail to the " +
			"requesting agent with your full findings.\n\n",
	)
	sb.WriteString("**Your agent name**: " +
		r.reviewerAgentName() + "\n\n",
	)

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

	sb.WriteString(fmt.Sprintf(
		"**Requester agent**: %s\n\n", requesterName,
	))

	agentName := r.reviewerAgentName()

	// Explain the two-step send process: write body to a file in
	// /tmp/substrate_reviews/, then send with --body-file. This
	// avoids shell quoting issues with multi-line markdown.
	sb.WriteString("### How to Send Your Review\n\n")
	sb.WriteString(
		"Use a two-step process to send rich review content:\n\n",
	)

	shortID := r.reviewID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	bodyFile := "/tmp/substrate_reviews/review-" + shortID + ".md"

	sb.WriteString(
		"1. First, create the reviews directory: " +
			"`mkdir -p /tmp/substrate_reviews`\n\n",
	)

	sb.WriteString(
		"2. **Write** your full review to `" + bodyFile +
			"` using the Write tool. " +
			"Use full markdown with headers, code blocks, " +
			"etc.\n\n",
	)

	sb.WriteString(
		"3. **Send** the review using `substrate send` " +
			"with `--body-file`:\n",
	)
	sb.WriteString("```bash\n")
	sb.WriteString(
		"substrate send" +
			" --agent " + agentName +
			" --to " + requesterName +
			" --thread " + r.threadID +
			" --subject \"Review: <decision>\"" +
			" --body-file " + bodyFile + "\n",
	)
	sb.WriteString("```\n\n")

	// Instruct the agent on what to include in the mail body.
	sb.WriteString("### Mail Body Format\n")
	sb.WriteString(
		"The review file must contain a **full, rich " +
			"review** in markdown format (similar to a " +
			"GitHub PR review). Include ALL of:\n\n",
	)
	sb.WriteString(
		"1. **Decision** (approve/request_changes/reject) " +
			"and a brief summary paragraph\n",
	)
	sb.WriteString(
		"2. **Per-issue details** for every issue found:\n",
	)
	sb.WriteString(
		"   - Severity (critical/high/medium/low)\n",
	)
	sb.WriteString(
		"   - File path and line numbers\n",
	)
	sb.WriteString(
		"   - Description of the problem\n",
	)
	sb.WriteString(
		"   - Code snippet showing the problematic code\n",
	)
	sb.WriteString(
		"   - Suggested fix with code example\n",
	)
	sb.WriteString(
		"3. **Positive observations** — what was done well\n",
	)
	sb.WriteString(
		"4. **Stats** — files reviewed, lines analyzed\n\n",
	)
	sb.WriteString(
		"Use markdown headers, code blocks, and formatting " +
			"for readability.\n\n",
	)

	sb.WriteString(
		"If you receive messages (injected by the stop hook), " +
			"process them and respond. For re-review requests, " +
			"run the diff command again and provide an updated " +
			"review.\n\n",
	)

	// Append project CLAUDE.md if available, giving the reviewer
	// project-specific style guidelines and conventions.
	if claudeMD := r.loadProjectCLAUDEMD(); claudeMD != "" {
		sb.WriteString("\n## Project Guidelines (from CLAUDE.md)\n")
		sb.WriteString(
			"The following are the project's coding guidelines. " +
				"Use these to inform your review:\n\n",
		)
		sb.WriteString(claudeMD)
		sb.WriteString("\n")
	}

	return sb.String()
}

// writeReviewTypeGuidance appends per-review-type detailed guidance to the
// system prompt. These match the specialized reviewer personas from Anthropic's
// code-review agents (security, performance, architecture).
func (r *reviewSubActor) writeReviewTypeGuidance(sb *strings.Builder) {
	switch r.config.Name {
	case "SecurityReviewer":
		sb.WriteString("## Security Review Checklist\n\n")
		sb.WriteString(
			"### Vulnerability Assessment\n",
		)
		sb.WriteString(
			"- OWASP Top 10: injection, broken auth, " +
				"sensitive data exposure, XXE, broken " +
				"access control, misconfiguration, XSS, " +
				"insecure deserialization\n",
		)
		sb.WriteString(
			"- SQL/NoSQL/command injection vectors\n",
		)
		sb.WriteString(
			"- Race conditions and TOCTOU " +
				"vulnerabilities\n",
		)
		sb.WriteString(
			"- Cryptographic implementation issues\n\n",
		)
		sb.WriteString(
			"### Input Validation\n",
		)
		sb.WriteString(
			"- User input validated against expected " +
				"formats/ranges\n",
		)
		sb.WriteString(
			"- Server-side validation as primary " +
				"control\n",
		)
		sb.WriteString(
			"- File upload restrictions (type, size, " +
				"content)\n",
		)
		sb.WriteString(
			"- Path traversal and directory escape " +
				"risks\n\n",
		)
		sb.WriteString(
			"### Auth and Authorization\n",
		)
		sb.WriteString(
			"- Session management correctness\n",
		)
		sb.WriteString(
			"- Privilege escalation vectors\n",
		)
		sb.WriteString(
			"- IDOR (insecure direct object " +
				"references)\n",
		)
		sb.WriteString(
			"- Access control at every protected " +
				"endpoint\n\n",
		)

	case "PerformanceReviewer":
		sb.WriteString("## Performance Review Checklist\n\n")
		sb.WriteString(
			"### Algorithmic Efficiency\n",
		)
		sb.WriteString(
			"- O(n^2) or worse operations on " +
				"potentially large inputs\n",
		)
		sb.WriteString(
			"- Redundant calculations that could be " +
				"cached\n",
		)
		sb.WriteString(
			"- Blocking calls that should be async\n",
		)
		sb.WriteString(
			"- Nested loops over the same data\n\n",
		)
		sb.WriteString(
			"### Database and Network\n",
		)
		sb.WriteString(
			"- N+1 query patterns and missing " +
				"indexes\n",
		)
		sb.WriteString(
			"- API batching opportunities and " +
				"unnecessary round-trips\n",
		)
		sb.WriteString(
			"- Missing pagination, filtering, or " +
				"projection\n",
		)
		sb.WriteString(
			"- Connection pooling patterns\n\n",
		)
		sb.WriteString(
			"### Resource Management\n",
		)
		sb.WriteString(
			"- Memory leaks from unclosed connections " +
				"or listeners\n",
		)
		sb.WriteString(
			"- Excessive allocations in loops\n",
		)
		sb.WriteString(
			"- Missing cleanup in defers or finally " +
				"blocks\n",
		)
		sb.WriteString(
			"- Goroutine leaks from unjoined " +
				"goroutines\n\n",
		)
		sb.WriteString(
			"For each finding, estimate performance " +
				"impact with complexity notation and " +
				"prioritize by impact-to-effort ratio.\n\n",
		)

	case "ArchitectureReviewer":
		sb.WriteString("## Architecture Review Checklist\n\n")
		sb.WriteString(
			"- Separation of concerns between layers\n",
		)
		sb.WriteString(
			"- Interface contracts and abstraction " +
				"boundaries\n",
		)
		sb.WriteString(
			"- Dependency direction (inward, not " +
				"outward)\n",
		)
		sb.WriteString(
			"- Testability (can components be tested " +
				"in isolation?)\n",
		)
		sb.WriteString(
			"- SOLID principle adherence\n",
		)
		sb.WriteString(
			"- Appropriate design pattern usage\n\n",
		)

	default:
		// Default (full) review — general guidance.
		sb.WriteString("## Review Dimensions\n\n")
		sb.WriteString(
			"### Code Correctness\n",
		)
		sb.WriteString(
			"- Logic errors producing wrong results\n",
		)
		sb.WriteString(
			"- Missing error handling at failure " +
				"points\n",
		)
		sb.WriteString(
			"- Edge cases and boundary conditions\n",
		)
		sb.WriteString(
			"- Null/nil handling and zero-value " +
				"assumptions\n\n",
		)
		sb.WriteString(
			"### Security\n",
		)
		sb.WriteString(
			"- Input validation at system boundaries\n",
		)
		sb.WriteString(
			"- Authorization checks on protected " +
				"operations\n",
		)
		sb.WriteString(
			"- Sensitive data in logs or error " +
				"messages\n\n",
		)
		sb.WriteString(
			"### CLAUDE.md Compliance\n",
		)
		sb.WriteString(
			"- Check changes against project " +
				"CLAUDE.md rules (appended below)\n",
		)
		sb.WriteString(
			"- When flagging a violation, quote the " +
				"specific rule in the claude_md_ref " +
				"field\n\n",
		)
	}
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
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"Review the code changes for review ID: %s\n\n",
		r.reviewID,
	))

	// Template the diff command based on available branch info.
	diffCmd := r.buildDiffCommand()

	sb.WriteString(fmt.Sprintf(
		"Please examine the diff using:\n```\n%s\n```\n\n",
		diffCmd,
	))
	sb.WriteString(
		"Review each modified file for bugs, security issues, " +
			"logic errors, and style violations.\n\n",
	)
	sb.WriteString(
		"After your analysis, include the YAML frontmatter " +
			"block with your decision and any issues found.\n",
	)

	return sb.String()
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
	return map[claudeagent.HookType][]claudeagent.HookConfig{
		claudeagent.HookTypeSessionStart: {{
			Matcher:  "*",
			Callback: r.hookSessionStart,
		}},
		claudeagent.HookTypeStop: {{
			Matcher:  "*",
			Callback: r.hookStop,
		}},
	}
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

	// Poll for unread messages with a timeout.
	deadline := time.Now().Add(stopPollTimeout)
	for time.Now().Before(deadline) {
		// Send heartbeat to keep agent active.
		if err := r.store.UpdateLastActive(
			ctx, r.agentID, time.Now(),
		); err != nil {
			log.WarnS(ctx,
				"Reviewer substrate hook: heartbeat "+
					"failed during poll",
				err,
				"agent_name", agentName,
			)
		}

		// Check for unread messages.
		msgs, err := r.store.GetUnreadMessages(
			ctx, r.agentID, 10,
		)
		if err != nil {
			log.ErrorS(ctx,
				"Reviewer substrate hook: mail check "+
					"failed",
				err,
				"agent_name", agentName,
			)

			return claudeagent.HookResult{
				Decision: "approve",
			}, nil
		}

		if len(msgs) > 0 {
			// Format messages as a prompt for the reviewer.
			reason := formatMailAsPrompt(msgs)

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
					"You have %d unread message(s) "+
						"from the substrate "+
						"messaging system. Process "+
						"these messages and respond "+
						"appropriately.",
					len(msgs),
				),
			}, nil
		}

		// No messages, wait before next poll.
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

// formatMailAsPrompt converts unread inbox messages into a text prompt
// that can be injected into the reviewer's conversation when the Stop
// hook blocks exit.
func formatMailAsPrompt(msgs []store.InboxMessage) string {
	var sb strings.Builder

	sb.WriteString("You have new messages:\n\n")

	for i, msg := range msgs {
		sb.WriteString(fmt.Sprintf(
			"--- Message %d ---\n", i+1,
		))
		sb.WriteString(fmt.Sprintf(
			"From: %s\n", msg.SenderName,
		))
		sb.WriteString(fmt.Sprintf(
			"Subject: %s\n", msg.Subject,
		))
		sb.WriteString(fmt.Sprintf("Body:\n%s\n\n", msg.Body))
	}

	sb.WriteString(
		"Please process these messages and respond " +
			"appropriately. If asked to re-review, run " +
			"the diff command again and provide an updated " +
			"review.\n",
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
