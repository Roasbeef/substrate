package plangen

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// ErrModelSilent reports that a generation ran to completion without the model
// ever producing text.
//
// This is worth a distinct error because its causes are entirely disjoint from
// a planning failure: an unreachable API, a hook that ate the turn, or a tool
// the policy will never allow. Reporting it as "no json plan found" sends a
// reader to the prompt and the parser, which are the two things that are
// working.
var ErrModelSilent = errors.New("the model produced no text")

// streamStats accumulates what a generation's message stream contained.
//
// A failed run used to leave behind one opaque number — 79 messages — which
// could not distinguish a model that was never reached from one that talked
// itself out of a plan. Counting by kind turns that into a diagnosis.
type streamStats struct {
	messages      int
	assistantText int

	// toolUses counts calls per tool name, which is how an exploration that
	// ran away from the actual task becomes visible.
	toolUses map[string]int

	// systems counts system messages by subtype. Hook events land here, and a
	// generation full of them is a generation whose subprocess loaded the
	// operator's own automation.
	systems map[string]int

	retries    int
	lastRetry  string
	denials    int
	lastDenial string
	hooks      int
	lastHook   string

	// turnErrors counts assistant turns that carried an upstream API error
	// code, which is the model reporting its own failure rather than the
	// transport reporting a retry.
	turnErrors    int
	lastTurnError string
}

// newStreamStats returns an empty stats accumulator.
func newStreamStats() *streamStats {
	return &streamStats{
		toolUses: make(map[string]int),
		systems:  make(map[string]int),
	}
}

// noteToolUses records the tools an assistant message invoked, and any
// upstream error the turn itself reported.
func (s *streamStats) noteToolUses(m claudeagent.AssistantMessage) {
	for _, block := range m.Message.Content {
		if block.Type == "tool_use" {
			s.toolUses[block.Name]++
		}
	}

	// An assistant turn can carry its own API error code. It is the most
	// direct statement available of why a turn produced nothing, so it is
	// recorded even though the turn looks otherwise unremarkable.
	if m.Error != "" {
		s.turnErrors++
		s.lastTurnError = string(m.Error)
	}
}

// summary renders the accumulated counts as one compact log field set.
//
// Zero-valued categories are omitted so the common case stays short and an
// unusual one stands out rather than hiding among zeros.
func (s *streamStats) summary() string {
	parts := []string{
		fmt.Sprintf("messages=%d", s.messages),
		fmt.Sprintf("text=%d", s.assistantText),
	}

	if len(s.toolUses) > 0 {
		parts = append(parts, "tools="+joinCounts(s.toolUses))
	}
	if s.retries > 0 {
		parts = append(parts, fmt.Sprintf("retries=%d", s.retries))
	}
	if s.denials > 0 {
		parts = append(parts, fmt.Sprintf("denials=%d(%s)", s.denials,
			s.lastDenial))
	}
	if s.hooks > 0 {
		parts = append(parts, fmt.Sprintf("hooks=%d(%s)", s.hooks, s.lastHook))
	}
	if s.turnErrors > 0 {
		parts = append(parts, fmt.Sprintf("turn_errors=%d(%s)", s.turnErrors,
			s.lastTurnError))
	}
	if len(s.systems) > 0 {
		parts = append(parts, "system="+joinCounts(s.systems))
	}

	return strings.Join(parts, " ")
}

// diagnose explains a silent run in terms a reader can act on.
//
// The categories are ordered by how far upstream they sit: a model that was
// never reached explains everything downstream of it, so it is reported first
// and the rest are not mentioned.
func (s *streamStats) diagnose() string {
	switch {
	case s.retries > 0:
		return fmt.Sprintf("the API was never reached: %d retries, last %s",
			s.retries, s.lastRetry)

	case s.turnErrors > 0:
		return fmt.Sprintf("every assistant turn failed upstream: %d turns, "+
			"last error %q", s.turnErrors, s.lastTurnError)

	case s.hooks > 0:
		return fmt.Sprintf(
			"the subprocess ran %d hook events (last %s), which suggests it "+
				"loaded external automation and the turn was consumed before "+
				"the model answered", s.hooks, s.lastHook)

	case s.denials > 0:
		return fmt.Sprintf(
			"%d tool calls were denied (last %s), so the model spent the run "+
				"retrying tools it is not allowed to use", s.denials,
			s.lastDenial)

	case len(s.toolUses) > 0:
		return fmt.Sprintf(
			"the model made %s and never wrote a plan",
			joinCounts(s.toolUses))

	default:
		return "the stream carried no assistant text, tool calls, retries, " +
			"or hook activity"
	}
}

// joinCounts renders a name/count map deterministically, so two log lines for
// the same stream are comparable.
func joinCounts(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s:%d", name, counts[name]))
	}

	return strings.Join(parts, ",")
}

// describeRetry renders an API retry compactly, keeping the status code that
// says whether the cause was auth, rate limiting, or the network.
func describeRetry(m claudeagent.APIRetryMessage) string {
	status := "no status"
	if m.ErrorStatus != nil {
		status = fmt.Sprintf("status %d", *m.ErrorStatus)
	}

	return fmt.Sprintf("attempt %d/%d, %s, error %v", m.Attempt, m.MaxRetries,
		status, m.Error)
}

// resultDetail extracts something quotable from a failed result message.
//
// The CLI reports some failures with an empty Result and the real explanation
// in Errors, which produced the useless "agent reported an error: " with
// nothing after the colon. Preferring whichever field is populated keeps the
// error from being empty.
func resultDetail(m claudeagent.ResultMessage) string {
	if text := strings.TrimSpace(m.Result); text != "" {
		return truncate(text, 400)
	}
	if len(m.Errors) > 0 {
		return truncate(strings.Join(m.Errors, "; "), 400)
	}
	if m.Subtype != "" {
		return "subtype " + m.Subtype
	}

	return "no detail reported"
}
