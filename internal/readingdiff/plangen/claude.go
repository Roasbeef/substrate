package plangen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		msgCount  int
	)
	for msg := range client.Query(ctx, prompt) {
		msgCount++

		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			if text := m.ContentText(); text != "" {
				lastText = text

				// The plan is the whole deliverable, so stop as soon as one
				// parses rather than waiting for the subprocess to wind down
				// through its stop hooks.
				plan, perr := parsePlan(text)
				if perr == nil {
					g.cfg.Logf(
						"plangen: plan accepted after %s (%d messages, "+
							"remove=%d fold=%d replace=%d)",
						time.Since(start).Round(time.Millisecond), msgCount,
						len(plan.Remove), len(plan.Fold), len(plan.Replace))

					return plan, nil
				}

				// Record why a candidate was rejected. Every message is
				// examined, so most of these are the model narrating rather
				// than a real failure; keeping the last one means the final
				// error can say what was actually wrong with the output.
				parseErr = perr
				g.cfg.Logf(
					"plangen: message %d (%dB) is not a plan yet: %v",
					msgCount, len(text), perr)
			}

		case claudeagent.ResultMessage:
			if m.Result != "" {
				lastText = m.Result
			}
			g.cfg.Logf("plangen: result cost=%v duration=%dms error=%v "+
				"result=%q errors=%v",
				m.TotalCostUSD, m.DurationMs, m.IsError,
				truncate(m.Result, 400), m.Errors)

			if m.IsError {
				resultErr = fmt.Errorf(
					"agent reported an error: %s",
					strings.TrimSpace(truncate(m.Result, 400)))
			}
		}
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	// An agent that failed outright is reported as such. Falling through to
	// "no json plan found" would blame the parser for an auth or subprocess
	// failure and send the caller looking in the wrong place.
	if resultErr != nil {
		g.cfg.Logf("plangen: failed after %s (%d messages): %v",
			elapsed, msgCount, resultErr)

		return zero, resultErr
	}

	if err := ctx.Err(); err != nil {
		g.cfg.Logf("plangen: timed out after %s (%d messages, last text %dB)",
			elapsed, msgCount, len(lastText))

		return zero, fmt.Errorf(
			"plan generation timed out after %s: %w", elapsed, err)
	}
	if lastText == "" {
		g.cfg.Logf("plangen: no output after %s (%d messages)",
			elapsed, msgCount)

		return zero, fmt.Errorf(
			"agent produced no output in %s across %d messages",
			elapsed, msgCount)
	}

	plan, err := parsePlan(lastText)
	if err != nil {
		if parseErr != nil {
			err = parseErr
		}

		// Log a slice of what the agent actually said. Without it this error
		// names a symptom and gives no way to tell a refusal from a malformed
		// plan from a model that ignored the format entirely.
		g.cfg.Logf("plangen: unusable output after %s (%d messages): %v\n"+
			"---- agent said ----\n%s\n--------------------",
			elapsed, msgCount, err, truncate(lastText, 2000))

		return zero, fmt.Errorf(
			"agent output was not a usable plan after %s: %w; it said: %q",
			elapsed, err, truncate(strings.TrimSpace(lastText), 300))
	}

	g.cfg.Logf("plangen: plan accepted from final text after %s (%d messages)",
		elapsed, msgCount)

	return plan, nil
}

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
