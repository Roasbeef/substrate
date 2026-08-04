package plangen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"

	"github.com/roasbeef/subtrate/internal/readingdiff"
)

// DefaultModel is the model used when a config leaves it unset. Abridging is a
// judgment task over a whole diff rather than a generation task, and Sonnet
// handles it at a fraction of the cost of a larger model.
const DefaultModel = "claude-sonnet-5"

// DefaultTimeout bounds one plan generation. A stuck subprocess must not hold
// a request open indefinitely.
const DefaultTimeout = 4 * time.Minute

// Config configures the Claude-backed generator.
type Config struct {
	// Model is the model ID to run. Defaults to DefaultModel.
	Model string

	// Timeout bounds a single Generate call. Defaults to DefaultTimeout.
	Timeout time.Duration

	// CLIPath overrides the claude binary location when non-empty.
	CLIPath string

	// IsolateConfigDir runs the agent against a throwaway config directory.
	//
	// This is off by default because relocating the config directory also
	// relocates where the CLI looks for its credentials, and the agent then
	// fails with "Not logged in" on a machine whose login lives in the system
	// keychain. The isolation that actually matters here comes from disabling
	// setting sources and skills, which is unconditional: those are what stop
	// the operator's own hooks from running inside a read-only analysis.
	IsolateConfigDir bool

	// Logf, when set, receives diagnostic lines.
	Logf func(format string, args ...any)
}

// Generator produces edit plans by running a Claude agent over a numbered
// diff. It satisfies readingdiff.Generator.
type Generator struct {
	cfg Config
}

// New returns a generator using the supplied config, filling in defaults.
func New(cfg Config) *Generator {
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}

	return &Generator{cfg: cfg}
}

// Generate asks the agent for a plan covering the request's diff.
//
// The agent runs with the repository as its working directory and read-only
// tool access, so it can inspect surrounding source to judge whether a row is
// load-bearing, but cannot modify the tree it is describing.
func (g *Generator) Generate(ctx context.Context, req readingdiff.Request,
	feedback string) (readingdiff.Plan, error) {

	var zero readingdiff.Plan

	ctx, cancel := context.WithTimeout(ctx, g.cfg.Timeout)
	defer cancel()

	opts, cleanup, err := g.clientOptions(req)
	if err != nil {
		return zero, err
	}
	defer cleanup()

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return zero, fmt.Errorf("create claude client: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			g.cfg.Logf("plangen: closing client: %v", cerr)
		}
	}()

	prompt := buildPrompt(req, feedback)
	start := time.Now()

	g.cfg.Logf("plangen: start model=%s patch=%dB prompt=%dB retry=%v",
		g.cfg.Model, len(req.UnifiedDiff), len(prompt), feedback != "")

	var (
		lastText  string
		resultErr error
		parseErr  error
		stats     = newStreamStats()
	)
	for msg := range client.Query(ctx, prompt) {
		stats.messages++

		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			stats.noteToolUses(m)

			if text := m.ContentText(); text != "" {
				stats.assistantText++
				lastText = text

				// The plan is the whole deliverable, so stop as soon as one
				// parses rather than waiting for the subprocess to wind down
				// through its stop hooks.
				plan, perr := parsePlan(text)
				if perr == nil {
					g.cfg.Logf(
						"plangen: plan accepted after %s (%s, "+
							"remove=%d fold=%d replace=%d)",
						time.Since(start).Round(time.Millisecond),
						stats.summary(), len(plan.Remove), len(plan.Fold),
						len(plan.Replace))

					return plan, nil
				}

				// Record why a candidate was rejected. Every message is
				// examined, so most of these are the model narrating rather
				// than a real failure; keeping the last one means the final
				// error can say what was actually wrong with the output.
				parseErr = perr
				g.cfg.Logf(
					"plangen: message %d (%dB) is not a plan yet: %v",
					stats.messages, len(text), perr)
			}

		case claudeagent.SystemMessage:
			stats.systems[m.Subtype]++

			// The init banner is the one cheap answer to "is this subprocess
			// even wired up": it names the model actually selected, where the
			// credential came from, and which tools the model believes it has.
			// An empty APIKeySource here means the run is doomed before the
			// first token, which is otherwise indistinguishable from a slow
			// abridgement.
			if m.Subtype == "" || m.Subtype == "init" {
				g.cfg.Logf("plangen: init model=%s auth=%q cwd=%s "+
					"mode=%s tools=%d",
					m.Model, m.APIKeySource, m.Cwd, m.PermissionMode,
					len(m.Tools))
			}

		case claudeagent.APIRetryMessage:
			// Retries are how an unreachable model presents itself. They are
			// counted rather than logged individually, because a wedged
			// credential produces dozens and the useful signal is the total
			// plus the status code.
			stats.retries++
			stats.lastRetry = describeRetry(m)
			if stats.retries == 1 {
				g.cfg.Logf("plangen: api retry %s", stats.lastRetry)
			}

		case claudeagent.PermissionDeniedMessage:
			// A denial loop is the other way a run burns minutes without
			// producing text: the model retries a tool the policy will never
			// allow. Counting them distinguishes that from an unreachable API.
			stats.denials++
			stats.lastDenial = m.ToolName
			if stats.denials == 1 {
				g.cfg.Logf("plangen: tool denied tool=%s reason=%q",
					m.ToolName, truncate(m.Message, 200))
			}

		case claudeagent.HookStartedMessage:
			// Hooks must never run here. This generator disables setting
			// sources, skills, and hooks precisely so the operator's own
			// automation cannot fire inside a read-only analysis, and the
			// project's Stop hook blocks for minutes by design. Seeing one at
			// all means an isolation option stopped working, so it is counted
			// and named rather than silently tolerated.
			stats.hooks++
			stats.lastHook = m.HookEvent
			if stats.hooks == 1 {
				g.cfg.Logf("plangen: unexpected hook fired event=%s name=%s",
					m.HookEvent, m.HookName)
			}

		case claudeagent.HookResponseMessage:
			stats.hooks++
			stats.lastHook = m.HookEvent

		case claudeagent.ResultMessage:
			if m.Result != "" {
				lastText = m.Result
			}
			g.cfg.Logf("plangen: result cost=%v duration=%dms error=%v "+
				"result=%q errors=%v",
				m.TotalCostUSD, m.DurationMs, m.IsError,
				truncate(m.Result, 400), m.Errors)

			if m.IsError {
				resultErr = fmt.Errorf("agent reported an error: %s",
					resultDetail(m))
			}
		}
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	// Log the whole stream once, whatever the outcome. Without this a run that
	// produced no text left nothing behind to distinguish "the model was never
	// reached" from "the model refused" from "the model is still thinking".
	g.cfg.Logf("plangen: stream finished in %s (%s)", elapsed, stats.summary())

	// A run that never got a word out of the model is not a planning failure.
	// Reporting it as one sends the reader to the prompt and the parser, which
	// are the two things working correctly, so name the actual cause instead.
	// This is checked before the result error because the result's own message
	// for this case is the CLI's internal diagnostic, which explains nothing.
	if stats.assistantText == 0 {
		g.cfg.Logf("plangen: model never spoke after %s: %s",
			elapsed, stats.diagnose())

		return zero, fmt.Errorf("%w after %s: %s (%s)", ErrModelSilent,
			elapsed, stats.diagnose(), stats.summary())
	}

	// An agent that failed outright is reported as such. Falling through to
	// "no json plan found" would blame the parser for an auth or subprocess
	// failure and send the caller looking in the wrong place.
	if resultErr != nil {
		g.cfg.Logf("plangen: failed after %s (%s): %v",
			elapsed, stats.summary(), resultErr)

		return zero, resultErr
	}

	if err := ctx.Err(); err != nil {
		g.cfg.Logf("plangen: timed out after %s (%s, last text %dB)",
			elapsed, stats.summary(), len(lastText))

		return zero, fmt.Errorf(
			"plan generation timed out after %s: %w", elapsed, err)
	}

	plan, err := parsePlan(lastText)
	if err != nil {
		if parseErr != nil {
			err = parseErr
		}

		// Log a slice of what the agent actually said. Without it this error
		// names a symptom and gives no way to tell a refusal from a malformed
		// plan from a model that ignored the format entirely.
		g.cfg.Logf("plangen: unusable output after %s (%s): %v\n"+
			"---- agent said ----\n%s\n--------------------",
			elapsed, stats.summary(), err, truncate(lastText, 2000))

		return zero, fmt.Errorf(
			"agent output was not a usable plan after %s: %w; it said: %q",
			elapsed, err, truncate(strings.TrimSpace(lastText), 300))
	}

	g.cfg.Logf("plangen: plan accepted from final text after %s (%s)",
		elapsed, stats.summary())

	return plan, nil
}

// TextOnlyMaxTurns bounds a generation that works from the diff alone. One
// turn is all a self-contained judgment needs.
const TextOnlyMaxTurns = 1

// RepoReadMaxTurns bounds a generation allowed to inspect the repository. It
// leaves room to look up a handful of call sites without permitting an
// open-ended exploration.
const RepoReadMaxTurns = 12

// maxTurns returns the turn ceiling appropriate to a request.
func (g *Generator) maxTurns(req readingdiff.Request) int {
	if req.RepoRoot != "" {
		return RepoReadMaxTurns
	}

	return TextOnlyMaxTurns
}

// strPtr returns a pointer to s, for the SDK's optional-argument map.
func strPtr(s string) *string { return &s }

// clientOptions assembles the SDK options, returning a cleanup that removes
// any temporary config directory.
//
// The isolation flags matter: without them the agent loads the user's own
// Claude Code hooks and skills, which would let a Stop hook keep the
// subprocess alive for minutes after the plan is ready, and could run
// arbitrary project automation inside what should be a read-only analysis.
func (g *Generator) clientOptions(req readingdiff.Request) ([]claudeagent.Option,
	func(), error) {

	cleanup := func() {}

	opts := []claudeagent.Option{
		claudeagent.WithModel(g.cfg.Model),
		claudeagent.WithSystemPrompt(systemPrompt),
		claudeagent.WithCanUseTool(readOnlyPolicy(req.RepoRoot)),
		claudeagent.WithNoSessionPersistence(),
		claudeagent.WithSettingSources(nil),
		claudeagent.WithSkillsDisabled(),

		claudeagent.WithStderr(func(data string) {
			g.cfg.Logf("plangen: cli stderr: %s", data)
		}),
	}

	// Bound the conversation and, with no repository to read, withhold the
	// read-only tools outright.
	//
	// These go through ExtraArgs rather than the SDK's WithMaxTurns and
	// WithDisallowedTools, which are write-only in v1.1.0: both set a field on
	// Options that the transport never reads when building the command line,
	// so calling them looks like a bound and enforces nothing. The CLI itself
	// accepts --max-turns and --disallowed-tools, and ExtraArgs is the path
	// that reaches it.
	//
	// The bound matters because abridging is a single judgment over text that
	// is entirely in the prompt: the useful answer arrives on the first turn,
	// and anything past it is the agent wandering. One unbounded request
	// accumulated 79 messages over 97 seconds and produced nothing.
	extra := map[string]*string{
		"max-turns": strPtr(strconv.Itoa(g.maxTurns(req))),
	}
	if req.RepoRoot == "" {
		extra["disallowed-tools"] = strPtr(strings.Join(toolNames(), ","))
	}
	opts = append(opts, claudeagent.WithExtraArgs(extra))

	// Forward credentials explicitly. A daemon started from a
	// non-interactive shell may otherwise hand the subprocess an empty
	// environment, which surfaces as an opaque invalid-API-key failure.
	authEnv := make(map[string]string)
	for _, key := range []string{
		"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY",
	} {
		if val := os.Getenv(key); val != "" {
			authEnv[key] = val
		}
	}
	if len(authEnv) > 0 {
		opts = append(opts, claudeagent.WithEnv(authEnv))
	}

	if g.cfg.IsolateConfigDir {
		tmp, err := os.MkdirTemp("", "readingdiff-*")
		if err != nil {
			return nil, cleanup, fmt.Errorf("create temp config dir: %w", err)
		}
		cleanup = func() {
			if rerr := os.RemoveAll(tmp); rerr != nil {
				g.cfg.Logf("plangen: removing temp dir: %v", rerr)
			}
		}

		configDir := filepath.Join(tmp, ".claude")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			cleanup()

			return nil, func() {}, fmt.Errorf("create config dir: %w", err)
		}
		opts = append(opts, claudeagent.WithConfigDir(configDir))
	}

	if g.cfg.CLIPath != "" && g.cfg.CLIPath != "claude" {
		opts = append(opts, claudeagent.WithCLIPath(g.cfg.CLIPath))
	}
	if req.RepoRoot != "" {
		opts = append(opts, claudeagent.WithCwd(req.RepoRoot))
	}

	return opts, cleanup, nil
}

// buildPrompt renders the user turn: the instruction, any compiler feedback
// from a rejected attempt, and the numbered diff.
func buildPrompt(req readingdiff.Request, feedback string) string {
	var b strings.Builder

	b.WriteString("Abridge the following unified diff into a reading diff " +
		"by returning a complete remove/fold/replace plan against the " +
		"numbered original lines. Coordinates are 1-based and always refer " +
		"to the original numbering. The `N|` gutter is display only and is " +
		"not part of a line's source text.\n\n")

	if req.RepoRoot != "" {
		b.WriteString("You may read files in the repository to judge " +
			"whether a row is load-bearing, or whether a file is " +
			"generated. Do not over-investigate: most rows can be judged " +
			"from the diff alone.\n\n")
	} else {
		b.WriteString("Judge from the diff text alone.\n\n")
	}

	if feedback != "" {
		b.WriteString(feedback)
		b.WriteString("\n\n")
	}

	b.WriteString("```diff\n")
	b.WriteString(readingdiff.Numbered(req.UnifiedDiff))
	b.WriteString("```\n")

	return b.String()
}

// jsonBlockRE captures the contents of a fenced json block.
var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)```")

// parsePlan extracts the edit plan from an agent's reply.
//
// The agent is asked for a single fenced json block, but models routinely wrap
// it in commentary or emit several blocks while thinking aloud. Scanning
// candidates from the last backwards, and requiring each to carry a summary and
// at least one edit key, tolerates that without accepting a draft or an echoed
// copy of the example in the system prompt.
func parsePlan(text string) (readingdiff.Plan, error) {
	var zero readingdiff.Plan

	matches := jsonBlockRE.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		// Fall back to a bare object when the agent omitted the fence.
		if start := strings.Index(text, "{"); start >= 0 {
			if end := strings.LastIndex(text, "}"); end > start {
				matches = [][]string{{"", text[start : end+1]}}
			}
		}
	}
	if len(matches) == 0 {
		return zero, fmt.Errorf("no json plan found in agent output")
	}

	var lastErr error
	for i := len(matches) - 1; i >= 0; i-- {
		plan, err := decodePlan(matches[i][1])
		if err != nil {
			lastErr = err

			continue
		}

		return plan, nil
	}

	return zero, fmt.Errorf("no valid json plan in agent output: %w", lastErr)
}

// decodePlan strictly decodes one candidate block into a plan.
//
// A candidate must actually look like a plan, not merely be valid JSON. The
// loop above parses every assistant message and takes the first block that
// decodes, and the system prompt itself contains a worked example with real
// coordinates in it. Without a presence check, a model restating the output
// format would have its example accepted as the plan, and a bare "{}" would
// compile into a deliberate-looking "keep everything" — the one failure the
// reader cannot distinguish from a considered judgment.
//
// Absent arrays are still normalized to empty, so a plan that legitimately has
// no edits of one kind is not rejected for a missing key.
func decodePlan(raw string) (readingdiff.Plan, error) {
	var plan readingdiff.Plan

	var present map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &present); err != nil {
		return plan, fmt.Errorf("decode plan envelope: %w", err)
	}
	if _, ok := present["summary"]; !ok {
		return plan, fmt.Errorf(
			"candidate has no summary key, so it is not a plan")
	}
	hasEdits := false
	for _, key := range []string{"remove", "fold", "replace"} {
		if _, ok := present[key]; ok {
			hasEdits = true
		}
	}
	if !hasEdits {
		return plan, fmt.Errorf(
			"candidate has no remove, fold, or replace key, so it is not " +
				"a plan")
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return plan, fmt.Errorf("decode plan: %w", err)
	}

	if plan.Remove == nil {
		plan.Remove = []readingdiff.Range{}
	}
	if plan.Fold == nil {
		plan.Fold = []readingdiff.Range{}
	}
	if plan.Replace == nil {
		plan.Replace = []readingdiff.Replacement{}
	}

	return plan, nil
}

// truncate shortens s for logging, marking that it was cut.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}
