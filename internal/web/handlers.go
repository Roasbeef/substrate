package web

import (
	"net/http"
	"strings"
	"time"
)

// PageData holds common data for page templates.
type PageData struct {
	Title       string
	ActiveNav   string
	UnreadCount int
	Category    string
	Stats       *DashboardStats
	Messages    []MessageView
	Agents      []AgentView
	Sessions    []SessionView

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

// MessageView represents a message for display.
type MessageView struct {
	ID               string
	ThreadID         string
	SenderName       string
	Subject          string
	Body             string
	State            string
	IsStarred        bool
	IsImportant      bool
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
		TotalCount:  totalCount,
		Page:        page,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		HasPrev:     page > 1,
		HasNext:     totalCount > page*pageSize,
	}

	s.render(w, "inbox.html", data)
}

// handleAgentsDashboard renders the agents dashboard page.
func (s *Server) handleAgentsDashboard(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:     "Agent Dashboard",
		ActiveNav: "agents",
		Stats: &DashboardStats{
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
			ID:          "1",
			ThreadID:    "t1",
			SenderName:  "backend-agent",
			Subject:     "Implemented user authentication",
			Body:        "I've completed the JWT authentication implementation. The changes include login, logout, and token refresh endpoints.",
			State:       "unread",
			IsStarred:   true,
			IsImportant: true,
			CreatedAt:   time.Now().Add(-30 * time.Minute),
		},
		{
			ID:          "2",
			ThreadID:    "t2",
			SenderName:  "test-writer",
			Subject:     "Added unit tests for auth module",
			Body:        "Test coverage is now at 87% for the authentication module. All edge cases covered.",
			State:       "unread",
			CreatedAt:   time.Now().Add(-2 * time.Hour),
		},
		{
			ID:          "3",
			ThreadID:    "t3",
			SenderName:  "code-reviewer",
			Subject:     "Review: PR #42 - Database optimization",
			Body:        "Found a few issues with the query optimization. See detailed comments below.",
			State:       "read",
			Labels:      []string{"review"},
			CreatedAt:   time.Now().Add(-5 * time.Hour),
		},
		{
			ID:          "4",
			ThreadID:    "t4",
			SenderName:  "security-auditor",
			Subject:     "Security scan completed",
			Body:        "No critical vulnerabilities found. Two medium-severity issues flagged for review.",
			State:       "read",
			IsImportant: true,
			Labels:      []string{"security"},
			CreatedAt:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:               "5",
			ThreadID:         "t5",
			SenderName:       "refactor-agent, me",
			Subject:          "Code cleanup proposal",
			Body:             "Proposing to consolidate duplicate helper functions across packages.",
			State:            "read",
			ParticipantCount: 2,
			CreatedAt:        time.Now().Add(-48 * time.Hour),
		},
	}

	s.renderPartial(w, "message-list", map[string]interface{}{
		"Messages": messages,
	})
}

// handleCompose returns the compose modal partial.
func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "compose-modal", nil)
}

// handleThread returns the thread view partial.
func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	threadID := strings.TrimPrefix(r.URL.Path, "/thread/")
	_ = threadID // TODO: Use to fetch thread.

	// Mock thread data.
	thread := map[string]interface{}{
		"ID":      threadID,
		"Subject": "Implemented user authentication",
		"Labels":  []string{"feature"},
		"Messages": []map[string]interface{}{
			{
				"ID":         "1",
				"SenderName": "backend-agent",
				"SenderAvatar": "B",
				"Body":       "I've completed the JWT authentication implementation.\n\nThe changes include:\n- Login endpoint at POST /api/auth/login\n- Logout endpoint at POST /api/auth/logout\n- Token refresh at POST /api/auth/refresh\n\nAll endpoints are tested and documented.",
				"CreatedAt":  time.Now().Add(-30 * time.Minute),
				"IsStarred":  true,
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
	status := map[string]interface{}{
		"AgentName":   "primary",
		"UnreadCount": 5,
		"UrgentCount": 1,
	}
	s.renderPartial(w, "status", status)
}

// handleAPITopics returns the topics list partial.
func (s *Server) handleAPITopics(w http.ResponseWriter, r *http.Request) {
	topics := []map[string]interface{}{
		{"ID": "1", "Name": "builds", "Type": "broadcast", "UnreadCount": 3},
		{"ID": "2", "Name": "deployments", "Type": "broadcast", "UnreadCount": 0},
		{"ID": "3", "Name": "alerts", "Type": "broadcast", "UnreadCount": 1},
	}
	s.renderPartial(w, "topics-list", map[string]interface{}{"Topics": topics})
}

// handleAPIAgents returns the agents list partial.
func (s *Server) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.getMockAgents()
	s.renderPartial(w, "agents-list", map[string]interface{}{"Agents": agents})
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
