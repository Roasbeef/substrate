package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/summary"
)

// FlowEvent is one high-granularity entry parsed from an agent's
// Claude Code session transcript: a prompt, assistant text, a tool
// invocation, or a thinking snippet.
type FlowEvent struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
}

// Flow event kinds.
const (
	flowKindPrompt   = "prompt"
	flowKindText     = "text"
	flowKindTool     = "tool"
	flowKindThinking = "thinking"
)

// flowMaxLines bounds how much transcript tail is parsed per request.
const flowMaxLines = 400

// AgentFlowResponse is the payload for GET /api/v1/command/flow/{id}.
type AgentFlowResponse struct {
	AgentID   int64       `json:"agent_id"`
	SessionID string      `json:"session_id,omitempty"`
	Events    []FlowEvent `json:"events"`
}

// registerCommandFlowRoutes registers the agent flow endpoint.
func (s *Server) registerCommandFlowRoutes() {
	s.mux.HandleFunc("/api/v1/command/flow/", s.handleAgentFlow)
}

// handleAgentFlow returns recent transcript flow events for an agent,
// giving the canvas its high-granularity timeline tier.
func (s *Server) handleAgentFlow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w, "Method not allowed", http.StatusMethodNotAllowed,
		)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/command/flow/")
	agentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	// Transcripts are keyed by the agent's project; without one there
	// is no flow to show, which is a benign empty response.
	if agent.ProjectKey == "" {
		writeJSON(w, http.StatusOK, AgentFlowResponse{
			AgentID: agentID, Events: []FlowEvent{},
		})
		return
	}

	reader := summary.NewTranscriptReader("", flowMaxLines)

	sessionID := agent.CurrentSessionID
	if sessionID == "" {
		sessionID, err = reader.FindActiveSession(agent.ProjectKey)
		if err != nil {
			writeJSON(w, http.StatusOK, AgentFlowResponse{
				AgentID: agentID, Events: []FlowEvent{},
			})
			return
		}
	}

	data, err := reader.ReadRecentTranscript(
		agent.ProjectKey, sessionID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, AgentFlowResponse{
			AgentID: agentID, SessionID: sessionID,
			Events: []FlowEvent{},
		})
		return
	}

	limit := 60
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, pErr := strconv.Atoi(l); pErr == nil && n > 0 {
			limit = n
		}
	}

	writeJSON(w, http.StatusOK, AgentFlowResponse{
		AgentID:   agentID,
		SessionID: sessionID,
		Events:    parseFlowEvents(data.Content, limit),
	})
}

// transcriptEntry mirrors the subset of a Claude Code session JSONL
// line the flow parser cares about.
type transcriptEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	IsMeta    bool   `json:"isMeta"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// transcriptBlock is one content block inside a transcript message.
type transcriptBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
}

// parseFlowEvents converts raw transcript JSONL into typed flow
// events, newest first, capped at limit. Malformed lines and noise
// (meta entries, tool results, summaries) are skipped.
func parseFlowEvents(content string, limit int) []FlowEvent {
	lines := strings.Split(content, "\n")
	events := make([]FlowEvent, 0, limit)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.IsMeta {
			continue
		}

		ts := normalizeFlowTimestamp(entry.Timestamp)

		switch entry.Type {
		case "user":
			ev, ok := parseUserEntry(entry, ts)
			if ok {
				events = append(events, ev)
			}

		case "assistant":
			events = append(
				events, parseAssistantBlocks(entry, ts)...,
			)
		}
	}

	// Newest first, keeping the most recent entries when over limit.
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	return events
}

// parseUserEntry extracts a prompt event from a user transcript line.
// Tool results (array content) are skipped as pure noise.
func parseUserEntry(entry transcriptEntry, ts string) (FlowEvent, bool) {
	var text string
	if err := json.Unmarshal(
		entry.Message.Content, &text,
	); err != nil || strings.TrimSpace(text) == "" {
		return FlowEvent{}, false
	}

	// Hook/system injections start with markers users never type.
	if strings.HasPrefix(text, "<") {
		return FlowEvent{}, false
	}

	return FlowEvent{
		Timestamp: ts,
		Kind:      flowKindPrompt,
		Label:     truncateFlow(text, 120),
	}, true
}

// parseAssistantBlocks extracts text, thinking, and tool events from
// an assistant transcript line's content blocks.
func parseAssistantBlocks(entry transcriptEntry, ts string) []FlowEvent {
	var blocks []transcriptBlock
	if err := json.Unmarshal(
		entry.Message.Content, &blocks,
	); err != nil {
		return nil
	}

	events := make([]FlowEvent, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			events = append(events, FlowEvent{
				Timestamp: ts,
				Kind:      flowKindText,
				Label:     truncateFlow(b.Text, 140),
				Detail:    truncateFlow(b.Text, 600),
			})

		case "thinking":
			if strings.TrimSpace(b.Thinking) == "" {
				continue
			}
			events = append(events, FlowEvent{
				Timestamp: ts,
				Kind:      flowKindThinking,
				Label:     truncateFlow(b.Thinking, 140),
			})

		case "tool_use":
			events = append(events, FlowEvent{
				Timestamp: ts,
				Kind:      flowKindTool,
				Label:     b.Name,
				Detail:    toolInputHint(b.Input),
			})
		}
	}

	return events
}

// toolInputHint pulls the most human-meaningful field out of a tool
// input for a one-line hint.
func toolInputHint(input json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	for _, key := range []string{
		"command", "file_path", "path", "pattern", "description",
		"prompt", "url", "subject",
	} {
		if v, ok := m[key].(string); ok && v != "" {
			return truncateFlow(v, 120)
		}
	}

	return ""
}

// normalizeFlowTimestamp best-effort normalizes transcript timestamps
// to RFC3339, passing through values it cannot parse.
func normalizeFlowTimestamp(ts string) string {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return ts
}

// truncateFlow trims whitespace and caps a string at n runes for
// display.
func truncateFlow(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
