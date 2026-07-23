package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// sampleTranscript covers the entry shapes the flow parser must
// handle: prompts, assistant text/thinking/tool blocks, meta noise,
// tool results, and malformed lines.
const sampleTranscript = `
{"type":"user","timestamp":"2026-07-19T10:00:00Z","message":{"role":"user","content":"Fix the expiry race in rfq"}}
{"type":"assistant","timestamp":"2026-07-19T10:00:05Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"The race is in AcceptHtlc; clock captured too early."},{"type":"text","text":"I'll start by reading the negotiator."}]}}
{"type":"assistant","timestamp":"2026-07-19T10:00:10Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"rfq/negotiator.go"}}]}}
{"type":"user","timestamp":"2026-07-19T10:00:12Z","message":{"role":"user","content":[{"type":"tool_result","content":"...file..."}]}}
{"type":"user","timestamp":"2026-07-19T10:00:14Z","isMeta":true,"message":{"role":"user","content":"<system-reminder>noise</system-reminder>"}}
{"type":"summary","summary":"irrelevant"}
not json at all
{"type":"assistant","timestamp":"2026-07-19T10:00:20Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./rfq/..."}}]}}
`

// TestParseFlowEvents verifies transcript JSONL parsing, ordering,
// and noise filtering.
func TestParseFlowEvents(t *testing.T) {
	events := parseFlowEvents(sampleTranscript, 60)

	// prompt, thinking, text, tool, tool — newest first.
	require.Len(t, events, 5)
	require.Equal(t, flowKindTool, events[0].Kind)
	require.Equal(t, "Bash", events[0].Label)
	require.Equal(t, "go test ./rfq/...", events[0].Detail)

	require.Equal(t, flowKindTool, events[1].Kind)
	require.Equal(t, "Read", events[1].Label)
	require.Equal(t, "rfq/negotiator.go", events[1].Detail)

	require.Equal(t, flowKindText, events[2].Kind)
	require.Equal(t, flowKindThinking, events[3].Kind)

	require.Equal(t, flowKindPrompt, events[4].Kind)
	require.Equal(
		t, "Fix the expiry race in rfq", events[4].Label,
	)
}

// TestParseFlowEventsLimit verifies the newest entries win when the
// limit truncates.
func TestParseFlowEventsLimit(t *testing.T) {
	events := parseFlowEvents(sampleTranscript, 2)
	require.Len(t, events, 2)
	require.Equal(t, "Bash", events[0].Label)
	require.Equal(t, "Read", events[1].Label)
}

// TestTruncateFlow verifies whitespace collapsing and capping.
func TestTruncateFlow(t *testing.T) {
	require.Equal(t, "a b c", truncateFlow("a\n  b\t c", 100))
	long := truncateFlow(
		"word word word word word word word word", 20,
	)
	require.Len(t, []rune(long), 20)
}
