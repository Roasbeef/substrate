package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
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
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "primary"
	}

	// TODO: Get actual counts from store.
	totalCount := 5
	page := 1
	pageSize := 50

	data := PageData{
		Title:       "Inbox",
		ActiveNav:   "inbox",
		UnreadCount: 5, // TODO: Get from store.
		Category:    category,
		Stats: &InboxStats{
			Unread:       5,
			Starred:      2,
			Snoozed:      1,
			Urgent:       1,
			PrimaryCount: 5,
			AgentsCount:  3,
			TopicsCount:  2,
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
	data := PageData{
		Title:     "Agent Dashboard",
		ActiveNav: "agents",
		AgentStats: &DashboardStats{
			ActiveAgents:    3,
			RunningSessions: 2,
			PendingMessages: 7,
			CompletedToday:  5,
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
	// Mock data for testing.
	messages := []MessageView{
		{
			ID:             "1",
			ThreadID:       "t1",
			SenderName:     "backend-agent",
			SenderInitials: "BA",
			Subject:        "Implemented user authentication",
			Body:           "I've completed the JWT authentication implementation. The changes include login, logout, and token refresh endpoints.",
			State:          "unread",
			IsStarred:      true,
			IsImportant:    true,
			IsAgent:        true,
			CreatedAt:      time.Now().Add(-30 * time.Minute),
		},
		{
			ID:             "2",
			ThreadID:       "t2",
			SenderName:     "test-writer",
			SenderInitials: "TW",
			Subject:        "Added unit tests for auth module",
			Body:           "Test coverage is now at 87% for the authentication module. All edge cases covered.",
			State:          "unread",
			IsAgent:        true,
			CreatedAt:      time.Now().Add(-2 * time.Hour),
		},
		{
			ID:             "3",
			ThreadID:       "t3",
			SenderName:     "code-reviewer",
			SenderInitials: "CR",
			Subject:        "Review: PR #42 - Database optimization",
			Body:           "Found a few issues with the query optimization. See detailed comments below.",
			State:          "read",
			IsAgent:        true,
			Labels:         []string{"review"},
			CreatedAt:      time.Now().Add(-5 * time.Hour),
		},
		{
			ID:             "4",
			ThreadID:       "t4",
			SenderName:     "security-auditor",
			SenderInitials: "SA",
			Subject:        "Security scan completed",
			Body:           "No critical vulnerabilities found. Two medium-severity issues flagged for review.",
			State:          "read",
			IsImportant:    true,
			IsAgent:        true,
			Labels:         []string{"security"},
			CreatedAt:      time.Now().Add(-24 * time.Hour),
		},
		{
			ID:               "5",
			ThreadID:         "t5",
			SenderName:       "refactor-agent",
			SenderInitials:   "RA",
			Subject:          "Code cleanup proposal",
			Body:             "Proposing to consolidate duplicate helper functions across packages.",
			State:            "read",
			IsAgent:          true,
			ParticipantCount: 2,
			CreatedAt:        time.Now().Add(-48 * time.Hour),
		},
	}

	s.renderPartial(w, "message-list", MessagesListData{
		Messages: messages,
	})
}

// handleCompose returns the compose modal partial.
func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "compose-modal", nil)
}

// handleThread returns the thread view partial.
func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	threadID := strings.TrimPrefix(r.URL.Path, "/thread/")

	// Mock thread data.
	thread := ThreadView{
		ID:            threadID,
		ThreadID:      threadID, // Alias for templates.
		Subject:       "Implemented user authentication",
		Labels:        []string{"feature"},
		TopicName:     "",
		MessageIndex:  1,
		TotalMessages: 1,
		HasPrev:       false,
		HasNext:       true,
		Messages: []ThreadMessage{
			{
				ID:           "1",
				SenderName:   "backend-agent",
				SenderAvatar: "B",
				Body:         "I've completed the JWT authentication implementation.\n\nThe changes include:\n- Login endpoint at POST /api/auth/login\n- Logout endpoint at POST /api/auth/logout\n- Token refresh at POST /api/auth/refresh\n\nAll endpoints are tested and documented.",
				State:        "read",
				CreatedAt:    time.Now().Add(-30 * time.Minute),
				IsStarred:    true,
			},
		},
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
	status := StatusView{
		AgentName:   "primary",
		UnreadCount: 5,
		UrgentCount: 1,
	}
	s.renderPartial(w, "status", status)
}

// handleAPITopics returns the topics list partial.
func (s *Server) handleAPITopics(w http.ResponseWriter, r *http.Request) {
	topics := []TopicView{
		{ID: "1", Name: "builds", TopicType: "broadcast", UnreadCount: 3},
		{ID: "2", Name: "deployments", TopicType: "broadcast", UnreadCount: 0},
		{ID: "3", Name: "alerts", TopicType: "broadcast", UnreadCount: 1},
	}
	s.renderPartial(w, "topics-list", TopicsListData{Topics: topics})
}

// handleAPIAgents returns the agents list partial.
func (s *Server) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.getMockAgents()
	s.renderPartial(w, "agents-list", AgentsListData{Agents: agents})
}

// handleAPIAgentsSidebar returns the sidebar agents partial.
func (s *Server) handleAPIAgentsSidebar(w http.ResponseWriter, r *http.Request) {
	agents := s.getMockAgents()

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
	agents := s.getMockAgents()

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
	activities := []ActivityView{
		{
			ID:          "a1",
			AgentName:   "backend-agent",
			SessionID:   "s1",
			Type:        "commit",
			Description: "Added JWT token validation middleware",
			File:        "internal/auth/middleware.go",
			Line:        45,
			Timestamp:   time.Now().Add(-5 * time.Minute),
		},
		{
			ID:          "a2",
			AgentName:   "test-writer",
			SessionID:   "s2",
			Type:        "session_start",
			Description: "Started session: Write unit tests for auth module",
			Timestamp:   time.Now().Add(-15 * time.Minute),
		},
		{
			ID:          "a3",
			AgentName:   "backend-agent",
			SessionID:   "s1",
			Type:        "decision",
			Description: "Using RS256 for JWT signing (more secure than HS256)",
			Timestamp:   time.Now().Add(-25 * time.Minute),
		},
		{
			ID:          "a4",
			AgentName:   "code-reviewer",
			Type:        "message",
			Description: "Posted review comments on PR #42",
			Timestamp:   time.Now().Add(-1 * time.Hour),
		},
		{
			ID:          "a5",
			AgentName:   "security-auditor",
			Type:        "session_complete",
			Description: "Completed security scan - no critical issues",
			Timestamp:   time.Now().Add(-2 * time.Hour),
		},
	}

	var html strings.Builder
	for _, activity := range activities {
		s.partials.ExecuteTemplate(&html, "activity-item", activity)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleAPIActiveSessions returns the active sessions partial.
func (s *Server) handleAPIActiveSessions(w http.ResponseWriter, r *http.Request) {
	sessions := []SessionView{
		{
			ID:             "s1",
			AgentName:      "backend-agent",
			Goal:           "Implement user authentication with JWT",
			Status:         "active",
			CurrentStep:    "Writing documentation",
			CompletedSteps: 4,
			TotalSteps:     5,
			ProgressPct:    80,
			StartedAt:      time.Now().Add(-45 * time.Minute),
		},
		{
			ID:             "s2",
			AgentName:      "test-writer",
			Goal:           "Write unit tests for auth module",
			Status:         "active",
			CurrentStep:    "Testing edge cases",
			CompletedSteps: 2,
			TotalSteps:     4,
			ProgressPct:    50,
			StartedAt:      time.Now().Add(-15 * time.Minute),
		},
	}

	var html strings.Builder
	for _, session := range sessions {
		s.partials.ExecuteTemplate(&html, "session-row", session)
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

// getMockAgents returns mock agent data for testing.
func (s *Server) getMockAgents() []AgentView {
	return []AgentView{
		{
			ID:             "agent-1",
			Name:           "backend-agent",
			Type:           "claude-code",
			Status:         "busy",
			Description:    "General-purpose backend development",
			Tags:           []string{"go", "backend", "api"},
			LastActivityAt: time.Now().Add(-5 * time.Minute),
			LastAction:     "Committed auth middleware",
			CurrentSession: &SessionView{
				ID:          "s1",
				Goal:        "Implement JWT authentication",
				ProgressPct: 80,
			},
		},
		{
			ID:             "agent-2",
			Name:           "test-writer",
			Type:           "test-writer",
			Status:         "active",
			Description:    "Writes comprehensive test suites",
			Tags:           []string{"testing", "go"},
			LastActivityAt: time.Now().Add(-2 * time.Minute),
			LastAction:     "Added edge case tests",
			CurrentSession: &SessionView{
				ID:          "s2",
				Goal:        "Write unit tests for auth",
				ProgressPct: 50,
			},
		},
		{
			ID:             "agent-3",
			Name:           "code-reviewer",
			Type:           "code-review",
			Status:         "idle",
			Description:    "Reviews PRs and suggests improvements",
			Tags:           []string{"review", "quality"},
			LastActivityAt: time.Now().Add(-1 * time.Hour),
			LastAction:     "Reviewed PR #42",
		},
		{
			ID:             "agent-4",
			Name:           "security-auditor",
			Type:           "security",
			Status:         "idle",
			Description:    "Security vulnerability scanning",
			Tags:           []string{"security", "audit"},
			LastActivityAt: time.Now().Add(-2 * time.Hour),
			LastAction:     "Completed security scan",
		},
	}
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

// getAgentsFromHeartbeat returns agents with real status data when available,
// otherwise falls back to mock data.
func (s *Server) getAgentsFromHeartbeat() []AgentView {
	ctx := context.Background()

	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil || len(agents) == 0 {
		// Fall back to mock data if no real agents exist.
		return s.getMockAgents()
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
