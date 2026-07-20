package web

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

// diffMarker is the marker used by `substrate send-diff` to embed raw
// patches inside a message body. The command feed uses it to classify
// messages as diffs without shipping extra metadata.
const diffMarker = "<!-- substrate:diff -->"

// waitingForPrefix marks the trailing "Waiting for:" line that
// `substrate status-update` appends to status message bodies.
const waitingForPrefix = "Waiting for:"

// Event kinds understood by the command center feed. Kinds drive both
// visual treatment and the actionable/informational split client-side.
const (
	eventKindPlan     = "plan"
	eventKindDiff     = "diff"
	eventKindQuestion = "question"
	eventKindStatus   = "status"
	eventKindReview   = "review"
	eventKindMessage  = "message"
	eventKindSteer    = "steer"
)

// Attention item kinds surfaced in the "needs you" queue.
const (
	attentionKindPlan     = "plan"
	attentionKindQuestion = "question"
	attentionKindUrgent   = "urgent"
	attentionKindBlocked  = "blocked"
)

// CommandEvent is a single feed entry inside an agent lane. Events are
// classified message traffic from the agent to the human operator.
type CommandEvent struct {
	MessageID    int64  `json:"message_id"`
	ThreadID     string `json:"thread_id"`
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Body         string `json:"body"`
	Priority     string `json:"priority"`
	State        string `json:"state"`
	CreatedAt    string `json:"created_at"`
	NeedsAction  bool   `json:"needs_action"`
	WaitingFor   string `json:"waiting_for,omitempty"`
	PlanReviewID string `json:"plan_review_id,omitempty"`
	PlanState    string `json:"plan_state,omitempty"`
	HasDiff      bool   `json:"has_diff"`
	// Direction is "in" for agent→operator traffic and "out" for
	// operator→agent steers, so the timeline shows both sides.
	Direction string `json:"direction"`
}

// CommandLaneAgent is the agent header info for a lane.
type CommandLaneAgent struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProjectKey  string `json:"project_key"`
	GitBranch   string `json:"git_branch"`
	Purpose     string `json:"purpose"`
	WorkingDir  string `json:"working_dir"`
	Status      string `json:"status"`
	LastActive  string `json:"last_active_at"`
	SecondsIdle int    `json:"seconds_since_heartbeat"`
	SessionID   string `json:"session_id,omitempty"`
}

// CommandLane groups one agent's live state and recent events.
type CommandLane struct {
	Agent       CommandLaneAgent `json:"agent"`
	Events      []CommandEvent   `json:"events"`
	UnreadCount int              `json:"unread_count"`
	NeedsAction int              `json:"needs_action_count"`
	WaitingFor  string           `json:"waiting_for,omitempty"`
	LastEventAt string           `json:"last_event_at,omitempty"`
}

// AttentionItem is one actionable entry in the cross-agent "needs you"
// queue, ordered most-urgent first.
type AttentionItem struct {
	Kind         string `json:"kind"`
	AgentID      int64  `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	Title        string `json:"title"`
	Detail       string `json:"detail,omitempty"`
	ThreadID     string `json:"thread_id,omitempty"`
	MessageID    int64  `json:"message_id,omitempty"`
	PlanReviewID string `json:"plan_review_id,omitempty"`
	Priority     string `json:"priority,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// CommandFeedResponse is the payload for GET /api/v1/command/feed.
type CommandFeedResponse struct {
	GeneratedAt string          `json:"generated_at"`
	Lanes       []CommandLane   `json:"lanes"`
	Attention   []AttentionItem `json:"attention"`
}

// registerCommandRoutes registers the command center REST endpoints.
func (s *Server) registerCommandRoutes() {
	s.mux.HandleFunc("/api/v1/command/feed", s.handleCommandFeed)
}

// classifyEvent derives the feed kind for a message based on subject
// and body conventions used by the substrate CLI and hooks. Plan and
// diff markers win over question heuristics, which win over status.
func classifyEvent(subject, body, priority string, isPlan bool) string {
	switch {
	case isPlan || strings.HasPrefix(subject, "[PLAN]"):
		return eventKindPlan

	case strings.Contains(body, diffMarker) ||
		strings.HasPrefix(subject, "[Diff]"):

		return eventKindDiff

	case strings.HasPrefix(subject, "[Review]") ||
		strings.HasPrefix(subject, "Review:"):

		return eventKindReview

	case isQuestion(subject, body, priority):
		return eventKindQuestion

	case strings.HasPrefix(subject, "[Status]") ||
		strings.HasPrefix(subject, "[Idle]") ||
		strings.HasPrefix(subject, "[Notification]") ||
		strings.HasPrefix(subject, "[Permission]"):

		return eventKindStatus

	default:
		return eventKindMessage
	}
}

// isQuestion reports whether a message looks like it needs a human
// answer: urgent priority, an interrogative subject, or an explicit
// decision request.
func isQuestion(subject, body, priority string) bool {
	if priority == "urgent" {
		return true
	}

	lowerSubject := strings.ToLower(subject)
	if strings.HasSuffix(strings.TrimSpace(subject), "?") {
		return true
	}
	for _, marker := range []string{
		"decision needed", "input needed", "approval needed",
		"question:", "needs decision", "which do you",
	} {
		if strings.Contains(lowerSubject, marker) {
			return true
		}
	}

	return false
}

// parseWaitingFor extracts the "Waiting for: X" trailer that
// status-update messages append, returning the waiting reason and the
// body with the trailer stripped.
func parseWaitingFor(body string) (string, string) {
	idx := strings.LastIndex(body, waitingForPrefix)
	if idx == -1 {
		return "", body
	}

	// Only treat it as a trailer when it starts a line.
	if idx > 0 && body[idx-1] != '\n' {
		return "", body
	}

	waiting := strings.TrimSpace(body[idx+len(waitingForPrefix):])
	trimmed := strings.TrimRight(body[:idx], "\n ")

	// Multi-line remainders are not trailers; keep the body intact.
	if strings.ContainsAny(waiting, "\n") {
		return "", body
	}

	return waiting, trimmed
}

// waitingNeedsHuman reports whether a "waiting for" reason describes
// something the operator must act on, filtering out self-referential
// values like "nothing" that agents emit while working.
func waitingNeedsHuman(waiting string) bool {
	if waiting == "" {
		return false
	}

	lower := strings.ToLower(waiting)
	for _, benign := range []string{
		"nothing", "none", "n/a", "no blockers", "next instructions",
	} {
		if strings.HasPrefix(lower, benign) {
			return false
		}
	}

	return true
}

// eventNeedsAction reports whether an event should surface in the
// attention queue: pending plans always, and unread or un-acked
// questions/urgent messages.
func eventNeedsAction(kind, state, priority, planState string) bool {
	switch kind {
	case eventKindPlan:
		return planState == "pending"

	case eventKindQuestion:
		return state == "unread" || state == "read"
	}

	return priority == "urgent" && state == "unread"
}

// handleCommandFeed aggregates agents, classified message traffic, and
// pending approvals into a single response so the command center pane
// loads in one round trip.
func (s *Server) handleCommandFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w, "Method not allowed", http.StatusMethodNotAllowed,
		)
		return
	}

	ctx := r.Context()

	userAgent, err := s.store.GetAgentByName(ctx, UserAgentName)
	if err != nil {
		// No User agent yet means an empty feed, not an error.
		writeJSON(w, http.StatusOK, CommandFeedResponse{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Lanes:       []CommandLane{},
			Attention:   []AttentionItem{},
		})
		return
	}

	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list agents",
		})
		return
	}

	inbox, err := s.store.GetInboxMessages(ctx, userAgent.ID, 200, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load inbox",
		})
		return
	}

	// Recent plan reviews keyed by thread let us link plan messages to
	// their review record and approval state.
	plans, err := s.store.ListPlanReviews(ctx, 100, 0)
	if err != nil {
		plans = nil
	}
	plansByThread := make(map[string]*storePlanRef, len(plans))
	for i := range plans {
		p := &plans[i]
		plansByThread[p.ThreadID] = &storePlanRef{
			id: p.PlanReviewID, state: p.State, title: p.PlanTitle,
		}
	}

	// Group classified events by sender.
	eventsByAgent := make(map[int64][]CommandEvent)
	for _, msg := range inbox {
		planRef := plansByThread[msg.ThreadID]

		event := buildEvent(msg.Subject, msg.Body, msg.Priority,
			msg.State, msg.ThreadID, msg.ID, msg.CreatedAt, planRef,
		)
		eventsByAgent[msg.SenderID] = append(
			eventsByAgent[msg.SenderID], event,
		)
	}

	// Merge the operator's outbound steers into recipient lanes so
	// each card reads as a two-way conversation.
	mergeOutboundEvents(ctx, s, userAgent.ID, eventsByAgent)

	// Both sources are merged, so restore newest-first order per
	// lane; buildLane depends on it for waiting-for and recency.
	for id := range eventsByAgent {
		evs := eventsByAgent[id]
		sort.SliceStable(evs, func(i, j int) bool {
			return evs[i].CreatedAt > evs[j].CreatedAt
		})
		eventsByAgent[id] = evs
	}

	lanes := make([]CommandLane, 0, len(agents))
	attention := make([]AttentionItem, 0, 8)

	for _, aws := range agents {
		if aws.Agent.Name == UserAgentName {
			continue
		}

		lane := buildLane(aws.Agent.ID, aws.Agent.Name,
			aws.Agent.ProjectKey.String, aws.Agent.GitBranch.String,
			aws.Agent.Purpose.String, string(aws.Status),
			aws.ActiveSessionID, aws.LastActive,
			eventsByAgent[aws.Agent.ID],
		)
		lane.Agent.WorkingDir = aws.Agent.WorkingDir.String
		lanes = append(lanes, lane)

		attention = append(
			attention, laneAttention(lane)...,
		)
	}

	sortLanes(lanes)
	sortAttention(attention)

	writeJSON(w, http.StatusOK, CommandFeedResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Lanes:       lanes,
		Attention:   attention,
	})
}

// mergeOutboundEvents appends the operator's recent sent messages to
// their recipient agents' event lists as "steer" events.
func mergeOutboundEvents(ctx context.Context, s *Server,
	userID int64, eventsByAgent map[int64][]CommandEvent,
) {
	sent, err := s.store.GetSentMessages(ctx, userID, 100)
	if err != nil || len(sent) == 0 {
		return
	}

	ids := make([]int64, 0, len(sent))
	for _, m := range sent {
		ids = append(ids, m.ID)
	}
	recipients, err := s.store.GetMessageRecipientsBulk(ctx, ids)
	if err != nil {
		return
	}

	for _, msg := range sent {
		for _, rcpt := range recipients[msg.ID] {
			eventsByAgent[rcpt.AgentID] = append(
				eventsByAgent[rcpt.AgentID],
				CommandEvent{
					MessageID: msg.ID,
					ThreadID:  msg.ThreadID,
					Kind:      eventKindSteer,
					Subject:   msg.Subject,
					Body:      msg.Body,
					Priority:  msg.Priority,
					State:     "sent",
					CreatedAt: msg.CreatedAt.UTC().
						Format(time.RFC3339),
					Direction: "out",
				},
			)
		}
	}
}

// storePlanRef is a minimal plan review reference used during event
// construction.
type storePlanRef struct {
	id    string
	state string
	title string
}

// buildEvent classifies one inbox message into a feed event.
func buildEvent(subject, body, priority, state, threadID string,
	messageID int64, createdAt time.Time, plan *storePlanRef,
) CommandEvent {

	kind := classifyEvent(subject, body, priority, plan != nil)

	waiting := ""
	eventBody := body
	if kind == eventKindStatus || kind == eventKindQuestion ||
		kind == eventKindMessage {

		waiting, eventBody = parseWaitingFor(body)
	}

	planID, planState := "", ""
	if plan != nil {
		planID, planState = plan.id, plan.state
	}

	return CommandEvent{
		MessageID:    messageID,
		ThreadID:     threadID,
		Kind:         kind,
		Subject:      subject,
		Body:         eventBody,
		Priority:     priority,
		State:        state,
		CreatedAt:    createdAt.UTC().Format(time.RFC3339),
		NeedsAction:  eventNeedsAction(kind, state, priority, planState),
		WaitingFor:   waiting,
		PlanReviewID: planID,
		PlanState:    planState,
		HasDiff:      strings.Contains(body, diffMarker),
		Direction:    "in",
	}
}

// buildLane assembles a lane from agent metadata plus its classified
// events, deriving unread counts and the latest waiting-for reason.
func buildLane(id int64, name, projectKey, gitBranch, purpose,
	status, sessionID string, lastActive time.Time,
	events []CommandEvent,
) CommandLane {

	if events == nil {
		events = []CommandEvent{}
	}

	unread, needsAction := 0, 0
	waitingFor := ""
	lastEventAt := ""

	for i := range events {
		if events[i].State == "unread" {
			unread++
		}
		if events[i].NeedsAction {
			needsAction++
		}
	}

	// Events arrive newest-first from the inbox query; take the
	// waiting-for reason from the most recent status-bearing event,
	// skipping benign values like "nothing yet" that need no human.
	for i := range events {
		if events[i].WaitingFor == "" {
			continue
		}
		if waitingNeedsHuman(events[i].WaitingFor) {
			waitingFor = events[i].WaitingFor
		}
		break
	}

	if len(events) > 0 {
		lastEventAt = events[0].CreatedAt
	}

	return CommandLane{
		Agent: CommandLaneAgent{
			ID:          id,
			Name:        name,
			ProjectKey:  projectKey,
			GitBranch:   gitBranch,
			Purpose:     purpose,
			Status:      status,
			LastActive:  lastActive.UTC().Format(time.RFC3339),
			SecondsIdle: int(time.Since(lastActive).Seconds()),
			SessionID:   sessionID,
		},
		Events:      events,
		UnreadCount: unread,
		NeedsAction: needsAction,
		WaitingFor:  waitingFor,
		LastEventAt: lastEventAt,
	}
}

// laneAttention extracts attention queue items from a lane: actionable
// events plus a blocked marker when the agent reports waiting on a
// human.
func laneAttention(lane CommandLane) []AttentionItem {
	items := make([]AttentionItem, 0, 2)

	for _, ev := range lane.Events {
		if !ev.NeedsAction {
			continue
		}

		var kind string
		title := ev.Subject
		switch ev.Kind {
		case eventKindPlan:
			kind = attentionKindPlan
			title = strings.TrimSpace(
				strings.TrimPrefix(ev.Subject, "[PLAN]"),
			)
		case eventKindQuestion:
			kind = attentionKindQuestion
		default:
			kind = attentionKindUrgent
		}

		items = append(items, AttentionItem{
			Kind:         kind,
			AgentID:      lane.Agent.ID,
			AgentName:    lane.Agent.Name,
			Title:        title,
			ThreadID:     ev.ThreadID,
			MessageID:    ev.MessageID,
			PlanReviewID: ev.PlanReviewID,
			Priority:     ev.Priority,
			CreatedAt:    ev.CreatedAt,
		})
	}

	if lane.WaitingFor != "" && len(items) == 0 {
		items = append(items, AttentionItem{
			Kind:      attentionKindBlocked,
			AgentID:   lane.Agent.ID,
			AgentName: lane.Agent.Name,
			Title:     "Waiting on you",
			Detail:    lane.WaitingFor,
			CreatedAt: lane.LastEventAt,
		})
	}

	return items
}

// laneSortRank orders lanes: agents needing action first, then by
// liveness (busy/active before idle before offline), then most recent
// event first.
func laneSortRank(lane CommandLane) int {
	rank := 0
	if lane.NeedsAction > 0 {
		rank -= 100
	}

	switch lane.Agent.Status {
	case "busy":
		rank -= 30
	case "active":
		rank -= 20
	case "idle":
		rank -= 10
	}

	return rank
}

// sortLanes orders lanes for display using laneSortRank with recency
// as the tie-breaker.
func sortLanes(lanes []CommandLane) {
	sort.SliceStable(lanes, func(i, j int) bool {
		ri, rj := laneSortRank(lanes[i]), laneSortRank(lanes[j])
		if ri != rj {
			return ri < rj
		}
		return lanes[i].LastEventAt > lanes[j].LastEventAt
	})
}

// attentionSortRank orders the attention queue: plans, then questions,
// then urgent messages, then blocked markers.
func attentionSortRank(kind string) int {
	switch kind {
	case attentionKindPlan:
		return 0
	case attentionKindQuestion:
		return 1
	case attentionKindUrgent:
		return 2
	default:
		return 3
	}
}

// sortAttention orders attention items by rank then recency.
func sortAttention(items []AttentionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		ri := attentionSortRank(items[i].Kind)
		rj := attentionSortRank(items[j].Kind)
		if ri != rj {
			return ri < rj
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
}
