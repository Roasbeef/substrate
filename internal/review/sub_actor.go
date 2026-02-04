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

	var (
		lastText  string
		response  claudeagent.ResultMessage
		gotResult bool
		msgCount  int
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
			var preview string
			if len(m.Message.Content) > 0 {
				preview = truncateStr(
					m.Message.Content[0].Text, 500,
				)
			}
			var parentToolID string
			if m.ParentToolUseID != nil {
				parentToolID = *m.ParentToolUseID
			}
			log.InfoS(ctx, "Reviewer tool result",
				"review_id", r.reviewID,
				"msg_num", msgCount,
				"parent_tool_use_id", parentToolID,
				"content_blocks",
				len(m.Message.Content),
				"preview", preview,
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
		"ctx_err", fmt.Sprintf("%v", ctx.Err()),
		"elapsed", time.Since(startTime).String(),
	)

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
		// policy callback. This replaces the blanket
		// --dangerously-skip-permissions with a fine-grained
		// policy that allows read operations and denies writes.
		claudeagent.WithCanUseTool(reviewerPermissionPolicy),
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
	sb.WriteString("    type: bug | security | logic_error | ")
	sb.WriteString("performance | style | documentation | ")
	sb.WriteString("claude_md_violation | other\n")
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

	// Look up requester name if possible.
	if r.requester > 0 {
		requester, err := r.store.GetAgent(
			context.Background(), r.requester,
		)
		if err == nil {
			sb.WriteString(fmt.Sprintf(
				"**Requester agent**: %s\n\n",
				requester.Name,
			))
		}
	}

	agentName := r.reviewerAgentName()

	sb.WriteString("Use the substrate CLI to send messages. ")
	sb.WriteString("Always pass `--agent " + agentName + "` ")
	sb.WriteString("to identify yourself, and `--thread " +
		r.threadID + "` to reply in the review thread:\n")
	sb.WriteString("```bash\n")
	sb.WriteString(
		"substrate send --agent " + agentName +
			" --to <requester-agent>" +
			" --thread " + r.threadID +
			" --subject \"Review: <decision>\"" +
			" --body \"$REVIEW_BODY\"\n",
	)
	sb.WriteString("```\n\n")

	// Instruct the agent on what to include in the mail body.
	sb.WriteString("### Mail Body Format\n")
	sb.WriteString(
		"The `--body` must contain a **full, rich review** " +
			"in markdown format (similar to a GitHub PR " +
			"review). Include ALL of the following:\n\n",
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
			"for readability. The body supports full markdown " +
			"so use ``` for code snippets.\n\n",
	)
	sb.WriteString(
		"**TIP**: Build the body string in a variable " +
			"before calling substrate send, since the body " +
			"can be long. Use a heredoc or variable:\n",
	)
	sb.WriteString("```bash\n")
	sb.WriteString(
		"REVIEW_BODY=$(cat <<'REVIEW_EOF'\n" +
			"# Code Review: <decision>\n\n" +
			"## Summary\n" +
			"<overview paragraph>\n\n" +
			"## Issues\n\n" +
			"### 1. [critical] <title> (`file.go:42`)\n" +
			"<description>\n" +
			"```go\n" +
			"// problematic code\n" +
			"```\n" +
			"**Suggestion**:\n" +
			"```go\n" +
			"// fixed code\n" +
			"```\n\n" +
			"## What Looks Good\n" +
			"- <positive observations>\n\n" +
			"## Stats\n" +
			"- Files reviewed: N\n" +
			"- Lines analyzed: ~N\n" +
			"REVIEW_EOF\n" +
			")\n",
	)
	sb.WriteString("```\n\n")

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
// The name includes the reviewer config name and a short review ID suffix
// for uniqueness across concurrent reviews.
func (r *reviewSubActor) reviewerAgentName() string {
	shortID := r.reviewID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	return fmt.Sprintf("reviewer-%s-%s", r.config.Name, shortID)
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
	"Write":        true,
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

// reviewerPermissionPolicy is the CanUseTool callback for reviewer agents.
// It enforces a read-only policy: read tools are allowed, write tools are
// denied, and Bash commands are filtered to prevent filesystem mutations.
func reviewerPermissionPolicy(
	_ context.Context, req claudeagent.ToolPermissionRequest,
) claudeagent.PermissionResult {
	toolName := req.ToolName

	// Deny known write tools immediately.
	if writeTools[toolName] {
		return claudeagent.PermissionDeny{
			Reason: fmt.Sprintf(
				"tool %q is not allowed in read-only "+
					"review mode", toolName,
			),
		}
	}

	// For Bash, inspect the command to block destructive operations.
	if toolName == "Bash" {
		return checkBashCommand(req.Arguments)
	}

	// Allow known read-only tools.
	if readOnlyTools[toolName] {
		return claudeagent.PermissionAllow{}
	}

	// Default: deny unknown tools for safety.
	return claudeagent.PermissionDeny{
		Reason: fmt.Sprintf(
			"unknown tool %q is not allowed in read-only "+
				"review mode", toolName,
		),
	}
}

// bashArgs is the JSON structure of Bash tool arguments from Claude Code.
type bashArgs struct {
	Command string `json:"command"`
}

// checkBashCommand inspects the Bash command and denies destructive
// operations while allowing read-only commands like git diff, git log, etc.
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

	// Check against dangerous command prefixes.
	for _, prefix := range bashDangerousPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return claudeagent.PermissionDeny{
				Reason: fmt.Sprintf(
					"bash command %q is not allowed "+
						"in read-only review mode",
					truncateStr(cmd, 80),
				),
			}
		}
	}

	// Also deny piped writes and redirects that create/overwrite files.
	if strings.Contains(cmd, ">") && !strings.Contains(cmd, "2>&1") {
		return claudeagent.PermissionDeny{
			Reason: "output redirection is not allowed " +
				"in read-only review mode",
		}
	}

	return claudeagent.PermissionAllow{}
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
