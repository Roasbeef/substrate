package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// PageData holds common data for page templates.
type PageData struct {
	Title       string
	ActiveNav   string
	UnreadCount int
	Category    string

	// Page-specific stats (use the one that applies).
	Stats      *InboxStats
	AgentStats *DashboardStats

	Messages []MessageView
	Agents   []AgentView
	Sessions []SessionView

	// Pagination fields.
	TotalCount int
	Page       int
	PrevPage   int
	NextPage   int
	HasPrev    bool
	HasNext    bool
}

// DashboardStats holds stats for the agents dashboard.
type DashboardStats struct {
	ActiveAgents    int
	RunningSessions int
	PendingMessages int
	CompletedToday  int
}

// InboxStats holds stats for the inbox page.
type InboxStats struct {
	Unread       int
	Starred      int
	Snoozed      int
	Urgent       int
	PrimaryCount int
	AgentsCount  int
	TopicsCount  int
}

// MessageView represents a message for display.
type MessageView struct {
	ID               string
	ThreadID         string
	SenderName       string
	SenderInitials   string
	Subject          string
	Body             string
	State            string
	IsStarred        bool
	IsImportant      bool
	IsAgent          bool
	IsTopic          bool
	HasAttachments   bool
	Labels           []string
	TopicName        string
	ParticipantCount int
	CreatedAt        time.Time
}

// AgentView represents an agent for display.
type AgentView struct {
	ID             string
	Name           string
	Type           string
	Status         string // "active", "busy", "idle", "offline"
	Description    string
	Tags           []string
	LastActivityAt time.Time
	LastAction     string
	CurrentSession *SessionView
}

// SessionView represents a session for display.
type SessionView struct {
	ID             string
	AgentID        string
	AgentName      string
	Goal           string
	Status         string // "active", "paused", "completed"
	CurrentStep    string
	CompletedSteps int
	TotalSteps     int
	ProgressPct    int
	StartedAt      time.Time
}

// ActivityView represents an activity item for display.
type ActivityView struct {
	ID          string
	AgentID     string
	AgentName   string
	SessionID   string
	Type        string // "commit", "message", "session_start", "session_complete", "decision", "error", "blocker"
	Description string
	File        string
	Line        int
	Timestamp   time.Time
}

// ThreadView represents a thread for display.
type ThreadView struct {
	ID            string
	ThreadID      string // Alias for ID, used in templates.
	Subject       string
	Labels        []string
	TopicName     string
	Messages      []ThreadMessage
	MessageIndex  int
	TotalMessages int
	HasPrev       bool
	HasNext       bool
}

// ThreadMessage represents a message within a thread view.
type ThreadMessage struct {
	ID           string
	SenderName   string
	SenderAvatar string
	Body         string
	State        string // "unread", "read", "starred"
	TopicName    string
	Deadline     time.Time
	AckedAt      time.Time
	CreatedAt    time.Time
	IsStarred    bool
}

// StatusView represents status data for the sidebar.
type StatusView struct {
	AgentName   string
	UnreadCount int
	UrgentCount int
}

// TopicView represents a topic for display.
type TopicView struct {
	ID          string
	Name        string
	TopicType   string
	UnreadCount int
}

// TopicsListData holds data for the topics list partial.
type TopicsListData struct {
	Topics      []TopicView
	ActiveTopic string
}

// AgentsListData holds data for the agents list partial.
type AgentsListData struct {
	Agents []AgentView
}

// MessagesListData holds data for the messages list partial.
type MessagesListData struct {
	Messages []MessageView
}

// handleIndex redirects to the inbox.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/inbox", http.StatusTemporaryRedirect)
}

// handleInbox renders the inbox page.
func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "primary"
	}

	// Try to get first agent for default stats.
	var agentID int64
	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		agentID = agents[0].ID
	}

	// Get real counts from store.
	var unreadCount, urgentCount int64
	if agentID > 0 {
		unreadCount, _ = s.store.Queries().CountUnreadByAgent(ctx, agentID)
		urgentCount, _ = s.store.Queries().CountUnreadUrgentByAgent(ctx, agentID)
	}

	// Count topics.
	topics, _ := s.store.Queries().ListTopics(ctx)
	topicsCount := len(topics)

	// Count agents.
	agentsCount := len(agents)

	totalCount := int(unreadCount)
	page := 1
	pageSize := 50

	data := PageData{
		Title:       "Inbox",
		ActiveNav:   "inbox",
		UnreadCount: int(unreadCount),
		Category:    category,
		Stats: &InboxStats{
			Unread:       int(unreadCount),
			Starred:      0, // TODO: Add starred count query.
			Snoozed:      0, // TODO: Add snoozed count query.
			Urgent:       int(urgentCount),
			PrimaryCount: int(unreadCount),
			AgentsCount:  agentsCount,
			TopicsCount:  topicsCount,
		},
		TotalCount: totalCount,
		Page:       page,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 1,
		HasNext:    totalCount > page*pageSize,
	}

	s.render(w, "inbox.html", data)
}

// handleAgentsDashboard renders the agents dashboard page.
func (s *Server) handleAgentsDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get real stats from heartbeat manager.
	counts, err := s.heartbeatMgr.GetStatusCounts(ctx)
	activeAgents := 0
	if err == nil {
		activeAgents = counts.Active + counts.Busy
	}

	data := PageData{
		Title:     "Agent Dashboard",
		ActiveNav: "agents",
		AgentStats: &DashboardStats{
			ActiveAgents:    activeAgents,
			RunningSessions: 0, // TODO: Track sessions in database.
			PendingMessages: 0, // TODO: Aggregate from all agent inboxes.
			CompletedToday:  0, // TODO: Track completed sessions.
		},
	}

	s.render(w, "agents-dashboard.html", data)
}

// handleSessions renders the sessions page.
// TODO: Create dedicated sessions.html template.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	// Redirect to agents dashboard which shows active sessions.
	http.Redirect(w, r, "/agents", http.StatusTemporaryRedirect)
}

// handleInboxMessages returns the message list partial.
func (s *Server) handleInboxMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query parameter, or try first registered agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		// Try to get first agent as default for demo purposes.
		agents, err := s.store.Queries().ListAgents(ctx)
		if err == nil && len(agents) > 0 {
			agentID = agents[0].ID
		}
	}

	// If no agent ID, return empty list.
	if agentID == 0 {
		s.renderPartial(w, "message-list", MessagesListData{
			Messages: nil,
		})
		return
	}

	// Fetch inbox messages from database.
	dbMessages, err := s.store.Queries().GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   50,
	})
	if err != nil {
		// Return empty list on error.
		s.renderPartial(w, "message-list", MessagesListData{
			Messages: nil,
		})
		return
	}

	// Convert to view models.
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		// Get sender name.
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		// Generate initials from sender name.
		initials := getInitials(senderName)

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: initials,
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          m.State,
			IsStarred:      m.State == "starred",
			IsImportant:    m.Priority == "urgent",
			IsAgent:        true,
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}

	s.renderPartial(w, "message-list", MessagesListData{
		Messages: messages,
	})
}

// getInitials returns the initials from a name (e.g., "backend-agent" -> "BA").
func getInitials(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		if len(parts[0]) >= 2 {
			return strings.ToUpper(parts[0][:2])
		}
		return strings.ToUpper(parts[0])
	}
	return strings.ToUpper(string(parts[0][0]) + string(parts[1][0]))
}

// handleCompose returns the compose modal partial.
func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "compose-modal", nil)
}

// handleThread returns the thread view partial.
func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	threadID := strings.TrimPrefix(r.URL.Path, "/thread/")

	// Fetch messages in this thread from database.
	dbMessages, err := s.store.Queries().GetMessagesByThread(ctx, threadID)
	if err != nil || len(dbMessages) == 0 {
		// Return empty thread view if not found.
		s.renderPartial(w, "thread-view", ThreadView{
			ID:       threadID,
			ThreadID: threadID,
			Subject:  "Thread not found",
			Messages: nil,
		})
		return
	}

	// Convert to thread messages.
	threadMessages := make([]ThreadMessage, len(dbMessages))
	subject := ""
	for i, m := range dbMessages {
		// Use first message subject as thread subject.
		if i == 0 {
			subject = m.Subject
		}

		// Get sender name.
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		// Get avatar initial.
		avatar := "?"
		if len(senderName) > 0 {
			avatar = strings.ToUpper(string(senderName[0]))
		}

		threadMessages[i] = ThreadMessage{
			ID:           fmt.Sprintf("%d", m.ID),
			SenderName:   senderName,
			SenderAvatar: avatar,
			Body:         m.BodyMd,
			State:        "read", // TODO: Get actual state from recipient.
			CreatedAt:    time.Unix(m.CreatedAt, 0),
		}

		// Check for deadline.
		if m.DeadlineAt.Valid {
			threadMessages[i].Deadline = time.Unix(m.DeadlineAt.Int64, 0)
		}
	}

	thread := ThreadView{
		ID:            threadID,
		ThreadID:      threadID,
		Subject:       subject,
		Messages:      threadMessages,
		MessageIndex:  1,
		TotalMessages: len(threadMessages),
		HasPrev:       false,
		HasNext:       len(threadMessages) > 1,
	}

	s.renderPartial(w, "thread-view", thread)
}

// handleNewAgentModal returns the new agent modal partial.
func (s *Server) handleNewAgentModal(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "new-agent-modal", nil)
}

// handleNewSessionModal returns the new session modal partial.
func (s *Server) handleNewSessionModal(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "start-session-modal", nil)
}

// handleAPIStatus returns the status partial.
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get first agent for status.
	var agentName string = "no agents"
	var unreadCount, urgentCount int64

	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		agentName = agents[0].Name
		unreadCount, _ = s.store.Queries().CountUnreadByAgent(ctx, agents[0].ID)
		urgentCount, _ = s.store.Queries().CountUnreadUrgentByAgent(ctx, agents[0].ID)
	}

	status := StatusView{
		AgentName:   agentName,
		UnreadCount: int(unreadCount),
		UrgentCount: int(urgentCount),
	}
	s.renderPartial(w, "status", status)
}

// handleAPITopics returns the topics list partial.
func (s *Server) handleAPITopics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch topics from database.
	dbTopics, err := s.store.Queries().ListTopics(ctx)
	if err != nil {
		// Return empty list on error.
		s.renderPartial(w, "topics-list", TopicsListData{Topics: nil})
		return
	}

	topics := make([]TopicView, len(dbTopics))
	for i, t := range dbTopics {
		topics[i] = TopicView{
			ID:          fmt.Sprintf("%d", t.ID),
			Name:        t.Name,
			TopicType:   t.TopicType,
			UnreadCount: 0, // TODO: Calculate unread count per topic.
		}
	}

	s.renderPartial(w, "topics-list", TopicsListData{Topics: topics})
}

// handleAPIAgents returns the agents list partial.
func (s *Server) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.getAgentsFromHeartbeat()
	s.renderPartial(w, "agents-list", AgentsListData{Agents: agents})
}

// handleAPIAgentsSidebar returns the sidebar agents partial.
func (s *Server) handleAPIAgentsSidebar(w http.ResponseWriter, r *http.Request) {
	agents := s.getAgentsFromHeartbeat()

	var html strings.Builder
	for _, agent := range agents {
		s.partials.ExecuteTemplate(&html, "agent-sidebar-item", agent)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleAPIAgentsCards returns the agent cards partial.
func (s *Server) handleAPIAgentsCards(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	agents := s.getAgentsFromHeartbeat()

	// Filter agents if requested.
	if filter != "" && filter != "all" {
		var filtered []AgentView
		for _, a := range agents {
			if filter == "active" && (a.Status == "active" || a.Status == "busy") {
				filtered = append(filtered, a)
			} else if filter == "idle" && a.Status == "idle" {
				filtered = append(filtered, a)
			}
		}
		agents = filtered
	}

	var html strings.Builder
	for _, agent := range agents {
		s.partials.ExecuteTemplate(&html, "agent-card", agent)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleAPIActivity returns the activity feed partial.
func (s *Server) handleAPIActivity(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement activity tracking in database.
	// For now, return empty activity feed.
	var activities []ActivityView

	var html strings.Builder
	for _, activity := range activities {
		s.partials.ExecuteTemplate(&html, "activity-item", activity)
	}

	// If no activities, return a placeholder message.
	if len(activities) == 0 {
		html.WriteString(`<div class="p-4 text-gray-500 text-sm">No recent activity</div>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleAPIActiveSessions returns the active sessions partial.
func (s *Server) handleAPIActiveSessions(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement session tracking in database.
	// For now, return empty sessions list.
	var sessions []SessionView

	var html strings.Builder
	for _, session := range sessions {
		s.partials.ExecuteTemplate(&html, "session-row", session)
	}

	// If no sessions, return a placeholder message.
	if len(sessions) == 0 {
		html.WriteString(`<tr><td colspan="4" class="p-4 text-gray-500 text-sm text-center">No active sessions</td></tr>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleSSEAgents streams agent updates via SSE.
func (s *Server) handleSSEAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Keep connection alive until client disconnects.
	<-r.Context().Done()
	_ = flusher // Use flusher when we have real events.
}

// handleSSEActivity streams activity updates via SSE.
func (s *Server) handleSSEActivity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Keep connection alive until client disconnects.
	<-r.Context().Done()
	_ = flusher // Use flusher when we have real events.
}

// handleSSEInbox streams inbox updates via SSE.
func (s *Server) handleSSEInbox(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Keep connection alive until client disconnects.
	// TODO: Subscribe to mail service notifications and emit:
	// - "new-message" events with message HTML
	// - "unread-count" events with updated count
	<-r.Context().Done()
	_ = flusher // Use flusher when we have real events.
}

// render renders a full page template.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderPartial renders a partial template (for HTMX requests).
func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.partials.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HeartbeatRequest represents a heartbeat request from an agent.
type HeartbeatRequest struct {
	AgentName string `json:"agent_name"`
	SessionID string `json:"session_id,omitempty"`
}

// HeartbeatResponse represents the response to a heartbeat request.
type HeartbeatResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// handleAPIHeartbeat handles heartbeat requests from agents.
func (s *Server) handleAPIHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentName == "" {
		http.Error(w, "agent_name is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Record the heartbeat.
	if err := s.heartbeatMgr.RecordHeartbeatByName(ctx, req.AgentName); err != nil {
		// If agent doesn't exist, return 404.
		http.Error(
			w, fmt.Sprintf("Agent not found: %s", req.AgentName),
			http.StatusNotFound,
		)
		return
	}

	// If session ID provided, track it.
	if req.SessionID != "" {
		agentData, err := s.registry.GetAgentByName(ctx, req.AgentName)
		if err == nil {
			s.heartbeatMgr.StartSession(agentData.ID, req.SessionID)
		}
	}

	// Return success response.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HeartbeatResponse{
		Status:  "ok",
		Message: "Heartbeat recorded",
	})
}

// AgentStatusResponse represents an agent's current status.
type AgentStatusResponse struct {
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	LastActiveAt   time.Time `json:"last_active_at"`
	SessionID      string    `json:"session_id,omitempty"`
	SecondsSinceHB int       `json:"seconds_since_heartbeat"`
}

// AgentStatusListResponse represents the list of agent statuses.
type AgentStatusListResponse struct {
	Agents []AgentStatusResponse `json:"agents"`
	Counts agent.StatusCounts    `json:"counts"`
}

// handleAPIAgentsStatus returns the current status of all agents (JSON).
func (s *Server) handleAPIAgentsStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	counts, err := s.heartbeatMgr.GetStatusCounts(ctx)
	if err != nil {
		http.Error(w, "Failed to get status counts", http.StatusInternalServerError)
		return
	}

	resp := AgentStatusListResponse{
		Agents: make([]AgentStatusResponse, len(agents)),
		Counts: *counts,
	}

	for i, aws := range agents {
		resp.Agents[i] = AgentStatusResponse{
			Name:           aws.Agent.Name,
			Status:         string(aws.Status),
			LastActiveAt:   aws.LastActive,
			SessionID:      aws.ActiveSessionID,
			SecondsSinceHB: int(time.Since(aws.LastActive).Seconds()),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getAgentsFromHeartbeat returns agents with real status data from the
// heartbeat manager. Returns empty list if no agents exist.
func (s *Server) getAgentsFromHeartbeat() []AgentView {
	ctx := context.Background()

	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		return nil
	}

	result := make([]AgentView, len(agents))
	for i, aws := range agents {
		result[i] = AgentView{
			ID:             fmt.Sprintf("agent-%d", aws.Agent.ID),
			Name:           aws.Agent.Name,
			Type:           "claude-code",
			Status:         string(aws.Status),
			LastActivityAt: aws.LastActive,
		}

		// Add session info if available.
		if aws.ActiveSessionID != "" {
			result[i].CurrentSession = &SessionView{
				ID: aws.ActiveSessionID,
			}
		}
	}

	return result
}
