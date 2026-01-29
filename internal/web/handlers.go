package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

	// MessagesEndpoint is the HTMX endpoint for loading messages.
	MessagesEndpoint string

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
	ProjectKey     string // Full project path.
	GitBranch      string // Current git branch.
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

	// Find first non-User agent for default stats.
	var agentID int64
	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		for _, a := range agents {
			if a.Name != UserAgentName {
				agentID = a.ID
				break
			}
		}
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
		Title:            "Inbox",
		ActiveNav:        "inbox",
		UnreadCount:      int(unreadCount),
		Category:         category,
		MessagesEndpoint: "/inbox/messages",
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

// handleStarred renders the starred messages page.
func (s *Server) handleStarred(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get first agent as default.
	var agentID int64
	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		agentID = agents[0].ID
	}

	// Get starred count.
	var starredCount int64
	if agentID > 0 {
		starredCount, _ = s.store.Queries().CountStarredByAgent(ctx, agentID)
	}

	data := PageData{
		Title:            "Starred",
		ActiveNav:        "starred",
		UnreadCount:      int(starredCount),
		MessagesEndpoint: "/starred/messages",
		Stats: &InboxStats{
			Starred: int(starredCount),
		},
	}

	s.render(w, "inbox.html", data)
}

// handleSnoozed renders the snoozed messages page.
func (s *Server) handleSnoozed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get first agent as default.
	var agentID int64
	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		agentID = agents[0].ID
	}

	// Get snoozed count.
	var snoozedCount int64
	if agentID > 0 {
		snoozedCount, _ = s.store.Queries().CountSnoozedByAgent(ctx, agentID)
	}

	data := PageData{
		Title:            "Snoozed",
		ActiveNav:        "snoozed",
		MessagesEndpoint: "/snoozed/messages",
		UnreadCount:      int(snoozedCount),
		Stats: &InboxStats{
			Snoozed: int(snoozedCount),
		},
	}

	s.render(w, "inbox.html", data)
}

// handleSent renders the sent messages page.
func (s *Server) handleSent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Use User agent for sent messages (human-sent messages from UI).
	agentID := s.getUserAgentID(ctx)

	// Get sent count.
	var sentCount int64
	if agentID > 0 {
		sentCount, _ = s.store.Queries().CountSentByAgent(ctx, agentID)
	}

	data := PageData{
		Title:            "Sent",
		ActiveNav:        "sent",
		MessagesEndpoint: "/sent/messages",
		UnreadCount:      int(sentCount),
		Stats: &InboxStats{
			PrimaryCount: int(sentCount),
		},
	}

	s.render(w, "inbox.html", data)
}

// handleArchive renders the archived messages page.
func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get first agent as default.
	var agentID int64
	agents, err := s.store.Queries().ListAgents(ctx)
	if err == nil && len(agents) > 0 {
		agentID = agents[0].ID
	}

	// Get archived count.
	var archivedCount int64
	if agentID > 0 {
		archivedCount, _ = s.store.Queries().CountArchivedByAgent(ctx, agentID)
	}

	data := PageData{
		Title:            "Archive",
		ActiveNav:        "archive",
		MessagesEndpoint: "/archive/messages",
		UnreadCount:      int(archivedCount),
		Stats: &InboxStats{
			PrimaryCount: int(archivedCount),
		},
	}

	s.render(w, "inbox.html", data)
}

// handleInboxMessages returns the message list partial.
func (s *Server) handleInboxMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query parameter, or find first non-User agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		// Find first agent that isn't the User agent.
		agents, err := s.store.Queries().ListAgents(ctx)
		if err == nil && len(agents) > 0 {
			for _, a := range agents {
				if a.Name != UserAgentName {
					agentID = a.ID
					break
				}
			}
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

// handleStarredMessages returns the starred message list partial.
func (s *Server) handleStarredMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query or use first agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		agents, err := s.store.Queries().ListAgents(ctx)
		if err == nil && len(agents) > 0 {
			agentID = agents[0].ID
		}
	}

	if agentID == 0 {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	// Fetch starred messages.
	dbMessages, err := s.store.Queries().GetStarredMessages(ctx, sqlc.GetStarredMessagesParams{
		AgentID: agentID,
		Limit:   50,
	})
	if err != nil {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	messages := s.convertStarredToView(ctx, dbMessages)
	s.renderPartial(w, "message-list", MessagesListData{Messages: messages})
}

// handleSnoozedMessages returns the snoozed message list partial.
func (s *Server) handleSnoozedMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query or use first agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		agents, err := s.store.Queries().ListAgents(ctx)
		if err == nil && len(agents) > 0 {
			agentID = agents[0].ID
		}
	}

	if agentID == 0 {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	// Fetch snoozed messages.
	dbMessages, err := s.store.Queries().GetSnoozedMessages(ctx, sqlc.GetSnoozedMessagesParams{
		AgentID: agentID,
		Limit:   50,
	})
	if err != nil {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	messages := s.convertSnoozedToView(ctx, dbMessages)
	s.renderPartial(w, "message-list", MessagesListData{Messages: messages})
}

// handleSentMessages returns the sent message list partial.
func (s *Server) handleSentMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query, or default to User agent for human-sent msgs.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		// Default to User agent for the Sent page.
		agentID = s.getUserAgentID(ctx)
	}

	if agentID == 0 {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	// Fetch sent messages.
	dbMessages, err := s.store.Queries().GetSentMessages(ctx, sqlc.GetSentMessagesParams{
		SenderID: agentID,
		Limit:    50,
	})
	if err != nil {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	messages := s.convertSentMessagesToView(ctx, dbMessages)
	s.renderPartial(w, "message-list", MessagesListData{Messages: messages})
}

// handleArchivedMessages returns the archived message list partial.
func (s *Server) handleArchivedMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent ID from query or use first agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64

	if agentIDStr != "" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	} else {
		agents, err := s.store.Queries().ListAgents(ctx)
		if err == nil && len(agents) > 0 {
			agentID = agents[0].ID
		}
	}

	if agentID == 0 {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	// Fetch archived messages.
	dbMessages, err := s.store.Queries().GetArchivedMessages(ctx, sqlc.GetArchivedMessagesParams{
		AgentID: agentID,
		Limit:   50,
	})
	if err != nil {
		s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
		return
	}

	messages := s.convertArchivedToView(ctx, dbMessages)
	s.renderPartial(w, "message-list", MessagesListData{Messages: messages})
}

// convertStarredToView converts starred messages to view models.
func (s *Server) convertStarredToView(
	ctx context.Context, dbMessages []sqlc.GetStarredMessagesRow,
) []MessageView {
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: getInitials(senderName),
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          m.State,
			IsStarred:      true,
			IsImportant:    m.Priority == "urgent",
			IsAgent:        true,
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}
	return messages
}

// convertSnoozedToView converts snoozed messages to view models.
func (s *Server) convertSnoozedToView(
	ctx context.Context, dbMessages []sqlc.GetSnoozedMessagesRow,
) []MessageView {
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: getInitials(senderName),
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          m.State,
			IsStarred:      false,
			IsImportant:    m.Priority == "urgent",
			IsAgent:        true,
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}
	return messages
}

// convertArchivedToView converts archived messages to view models.
func (s *Server) convertArchivedToView(
	ctx context.Context, dbMessages []sqlc.GetArchivedMessagesRow,
) []MessageView {
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: getInitials(senderName),
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          m.State,
			IsStarred:      false,
			IsImportant:    m.Priority == "urgent",
			IsAgent:        true,
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}
	return messages
}

// convertSentMessagesToView converts sent messages to view models.
func (s *Server) convertSentMessagesToView(
	ctx context.Context, dbMessages []sqlc.Message,
) []MessageView {
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: getInitials(senderName),
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          "sent",
			IsAgent:        true,
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}
	return messages
}

// handleCompose returns the compose modal partial.
func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	s.renderPartial(w, "compose-modal", nil)
}

// TopicViewData holds data for the topic view page.
type TopicViewData struct {
	TopicName   string
	TopicType   string
	Messages    []MessageView
	Subscribers []AgentView
}

// handleTopicView renders the topic view page showing messages in a topic.
func (s *Server) handleTopicView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topicName := strings.TrimPrefix(r.URL.Path, "/topic/")

	// Get topic info.
	topic, err := s.store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		http.Error(w, "Topic not found: "+topicName, http.StatusNotFound)
		return
	}

	// Get messages in this topic.
	dbMessages, err := s.store.Queries().GetMessagesByTopic(ctx, topic.ID)
	if err != nil {
		dbMessages = nil
	}

	// Convert to message views.
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		senderName := fmt.Sprintf("Agent#%d", m.SenderID)
		sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
		if err == nil {
			senderName = sender.Name
		}

		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     senderName,
			SenderInitials: getInitials(senderName),
			Subject:        m.Subject,
			Body:           m.BodyMd,
			State:          "read",
			IsImportant:    m.Priority == "urgent",
			CreatedAt:      time.Unix(m.CreatedAt, 0),
		}
	}

	// Get subscribers.
	subs, _ := s.store.Queries().ListSubscriptionsByTopic(ctx, topic.ID)
	subscribers := make([]AgentView, len(subs))
	for i, sub := range subs {
		subscribers[i] = AgentView{
			ID:   fmt.Sprintf("%d", sub.ID),
			Name: sub.Name,
		}
	}

	_ = TopicViewData{
		TopicName:   topicName,
		TopicType:   topic.TopicType,
		Messages:    messages,
		Subscribers: subscribers,
	}

	// Render inline HTML for now (could create a template later).
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>%s - Substrate</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-50">
    <div class="max-w-4xl mx-auto p-6">
        <div class="mb-6">
            <a href="/inbox" class="text-blue-600 hover:underline">← Back to Inbox</a>
        </div>
        <div class="bg-white rounded-lg shadow p-6">
            <div class="flex items-center justify-between mb-4">
                <div>
                    <h1 class="text-2xl font-bold">%s</h1>
                    <p class="text-gray-500">Type: %s</p>
                </div>
                <span class="px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-sm">%d messages</span>
            </div>
            <div class="border-t pt-4">
                <h2 class="font-semibold mb-2">Subscribers (%d)</h2>
                <div class="flex flex-wrap gap-2 mb-4">`, topicName, topicName, topic.TopicType, len(messages), len(subscribers))

	for _, sub := range subscribers {
		fmt.Fprintf(w, `<span class="px-2 py-1 bg-gray-100 rounded text-sm">%s</span>`, sub.Name)
	}

	fmt.Fprintf(w, `</div>
            </div>
            <div class="border-t pt-4">
                <h2 class="font-semibold mb-2">Messages</h2>
                <div class="space-y-3">`)

	if len(messages) == 0 {
		fmt.Fprintf(w, `<p class="text-gray-500">No messages in this topic yet.</p>`)
	} else {
		for _, msg := range messages {
			fmt.Fprintf(w, `<div class="p-3 border rounded hover:bg-gray-50">
                    <div class="flex justify-between">
                        <span class="font-medium">%s</span>
                        <span class="text-gray-500 text-sm">%s</span>
                    </div>
                    <div class="font-medium">%s</div>
                    <div class="text-gray-600 text-sm truncate">%s</div>
                </div>`, msg.SenderName, msg.CreatedAt.Format("Jan 2, 3:04 PM"), msg.Subject, truncate(msg.Body, 100))
		}
	}

	fmt.Fprintf(w, `</div>
            </div>
        </div>
    </div>
</body>
</html>`)
}

// handleSearch handles search queries and returns search results as HTML.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")

	w.Header().Set("Content-Type", "text/html")

	// If no query, return empty content (div stays hidden).
	if query == "" {
		fmt.Fprint(w, ``)
		return
	}

	// Search messages.
	results, err := s.store.SearchMessages(ctx, query, 20)
	if err != nil {
		fmt.Fprintf(w, `<div class="p-4 text-red-600">Search error: %s</div>
			<script>document.getElementById('search-results').classList.remove('hidden')</script>`)
		return
	}

	// Render results as dropdown.
	if len(results) == 0 {
		fmt.Fprintf(w, `<div class="p-4 text-gray-500">No results found for "%s"</div>
			<script>document.getElementById('search-results').classList.remove('hidden')</script>`, query)
		return
	}

	fmt.Fprint(w, `<div class="max-h-96 overflow-y-auto">`)
	for _, result := range results {
		// Get sender name.
		senderName := fmt.Sprintf("Agent#%d", result.SenderID)
		if sender, err := s.store.Queries().GetAgent(ctx, result.SenderID); err == nil {
			senderName = sender.Name
		}

		// Format time.
		createdAt := time.Unix(result.CreatedAt, 0)
		timeStr := formatTimeAgo(createdAt)

		// Priority badge.
		priorityBadge := ""
		if result.Priority == "urgent" {
			priorityBadge = `<span class="px-1.5 py-0.5 bg-red-100 text-red-700 text-xs rounded">URGENT</span>`
		}

		fmt.Fprintf(w, `
			<a href="/thread/%s"
			   class="block p-3 hover:bg-gray-50 border-b border-gray-100 last:border-0"
			   hx-get="/thread/%s"
			   hx-target="#content-area"
			   hx-swap="innerHTML"
			   onclick="document.getElementById('search-results').classList.add('hidden')">
				<div class="flex items-center justify-between mb-1">
					<span class="font-medium text-gray-900 text-sm">%s</span>
					<span class="text-xs text-gray-500">%s</span>
				</div>
				<div class="flex items-center gap-2">
					<span class="text-sm text-gray-700">%s</span>
					%s
				</div>
				<div class="text-xs text-gray-500 truncate mt-1">%s</div>
			</a>`,
			result.ThreadID, result.ThreadID,
			senderName, timeStr,
			result.Subject, priorityBadge,
			truncate(result.BodyMd, 80))
	}
	fmt.Fprint(w, `</div>`)

	// Show count footer and unhide the results div.
	fmt.Fprintf(w, `<div class="p-2 bg-gray-50 border-t text-xs text-gray-500 text-center">%d result(s) for "%s"</div>
		<script>document.getElementById('search-results').classList.remove('hidden')</script>`, len(results), query)
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
			ProjectKey:     aws.Agent.ProjectKey.String,
			GitBranch:      aws.Agent.GitBranch.String,
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

// handleThreadAction handles thread actions like reply, archive, trash, unread.
// Routes: POST /api/threads/{thread_id}/{action}
func (s *Server) handleThreadAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /api/threads/{thread_id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/threads/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	threadID := parts[0]
	action := parts[1]

	ctx := context.Background()

	switch action {
	case "reply":
		s.handleThreadReply(ctx, w, r, threadID)
	case "archive":
		s.handleThreadArchive(ctx, w, r, threadID)
	case "trash":
		s.handleThreadTrash(ctx, w, r, threadID)
	case "unread":
		s.handleThreadUnread(ctx, w, r, threadID)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleThreadReply handles replying to a thread.
func (s *Server) handleThreadReply(
	ctx context.Context, w http.ResponseWriter, r *http.Request, threadID string,
) {
	// Parse form data.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "Reply body required", http.StatusBadRequest)
		return
	}

	// Get the original thread to find the topic and participants.
	messages, err := s.store.Queries().GetMessagesByThread(ctx, threadID)
	if err != nil || len(messages) == 0 {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	// Get the first message to find the topic.
	firstMsg := messages[0]

	// Determine sender: if provided in form, use that; otherwise use the
	// User agent (for human-sent messages from UI).
	senderName := r.FormValue("sender")
	var senderID int64
	if senderName != "" {
		sender, err := s.store.Queries().GetAgentByName(ctx, senderName)
		if err == nil {
			senderID = sender.ID
		}
	}
	// Fallback: use the User agent for UI-sent messages.
	if senderID == 0 {
		senderID = s.getUserAgentID(ctx)
	}
	if senderID == 0 {
		http.Error(w, "Could not determine reply sender", http.StatusInternalServerError)
		return
	}

	// Get the next log offset for this topic.
	maxOffsetResult, err := s.store.Queries().GetMaxLogOffset(ctx, firstMsg.TopicID)
	var nextOffset int64 = 0
	if err == nil && maxOffsetResult != nil {
		if offset, ok := maxOffsetResult.(int64); ok {
			nextOffset = offset + 1
		}
	}

	// Create the reply message.
	_, err = s.store.Queries().CreateMessage(ctx, sqlc.CreateMessageParams{
		ThreadID:  threadID,
		TopicID:   firstMsg.TopicID,
		LogOffset: nextOffset,
		SenderID:  senderID,
		Subject:   "Re: " + firstMsg.Subject,
		BodyMd:    body,
		Priority:  "normal",
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		http.Error(w, "Failed to send reply: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response for HTMX - refresh the thread view.
	w.Header().Set("HX-Trigger", "threadUpdated")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-3 bg-green-50 text-green-700 rounded-lg text-sm">Reply sent successfully!</div>`))
}

// handleThreadArchive handles archiving a thread.
func (s *Server) handleThreadArchive(
	ctx context.Context, w http.ResponseWriter, r *http.Request, threadID string,
) {
	// TODO: Implement archive functionality (update message state).
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-2 text-sm text-gray-500">Thread archived</div>`))
}

// handleThreadTrash handles trashing a thread.
func (s *Server) handleThreadTrash(
	ctx context.Context, w http.ResponseWriter, r *http.Request, threadID string,
) {
	// TODO: Implement trash functionality (update message state).
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-2 text-sm text-gray-500">Thread moved to trash</div>`))
}

// handleThreadUnread handles marking a thread as unread.
func (s *Server) handleThreadUnread(
	ctx context.Context, w http.ResponseWriter, r *http.Request, threadID string,
) {
	// TODO: Implement unread functionality (update recipient state).
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-2 text-sm text-gray-500">Thread marked as unread</div>`))
}

// handleMessageSend handles sending a new message from the compose form.
// Route: POST /api/messages/send
func (s *Server) handleMessageSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Accept both "to" and "recipients" field names.
	to := r.FormValue("to")
	if to == "" {
		to = r.FormValue("recipients")
	}
	subject := r.FormValue("subject")
	body := r.FormValue("body")
	priority := r.FormValue("priority")

	if to == "" || subject == "" || body == "" {
		http.Error(w, "Recipients, subject, and body are required", http.StatusBadRequest)
		return
	}

	if priority == "" {
		priority = "normal"
	}

	// Find the recipient agent.
	recipient, err := s.store.Queries().GetAgentByName(ctx, to)
	if err != nil {
		http.Error(w, "Recipient agent not found: "+to, http.StatusBadRequest)
		return
	}

	// Use User agent as sender.
	senderID := s.getUserAgentID(ctx)
	if senderID == 0 {
		http.Error(w, "Could not determine sender", http.StatusInternalServerError)
		return
	}

	// Get or create the topic for the recipient's inbox.
	topicName := "agent/" + to + "/inbox"
	topic, err := s.store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		// Create the topic if it doesn't exist.
		topic, err = s.store.Queries().CreateTopic(ctx, sqlc.CreateTopicParams{
			Name:             topicName,
			TopicType:        "direct",
			RetentionSeconds: sql.NullInt64{Int64: 604800, Valid: true},
			CreatedAt:        time.Now().Unix(),
		})
		if err != nil {
			http.Error(w, "Failed to create topic: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Generate thread ID for new message.
	threadID := fmt.Sprintf("thread-%d-%d", time.Now().UnixNano(), senderID)

	// Get next log offset.
	maxOffsetResult, err := s.store.Queries().GetMaxLogOffset(ctx, topic.ID)
	var nextOffset int64 = 1
	if err == nil && maxOffsetResult != nil {
		if offset, ok := maxOffsetResult.(int64); ok {
			nextOffset = offset + 1
		}
	}

	// Create the message.
	msg, err := s.store.Queries().CreateMessage(ctx, sqlc.CreateMessageParams{
		ThreadID:  threadID,
		TopicID:   topic.ID,
		LogOffset: nextOffset,
		SenderID:  senderID,
		Subject:   subject,
		BodyMd:    body,
		Priority:  priority,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		http.Error(w, "Failed to create message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create recipient entry.
	err = s.store.Queries().CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: msg.ID,
		AgentID:   recipient.ID,
	})
	if err != nil {
		http.Error(w, "Failed to add recipient: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success with HX-Trigger to close modal and refresh.
	w.Header().Set("HX-Trigger", "messageSent")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-4 bg-green-50 text-green-700 rounded-lg text-center">
		<p class="font-medium">Message sent successfully!</p>
		<p class="text-sm mt-1">Your message to ` + to + ` has been delivered.</p>
	</div>`))
}

// handleMessageAction handles message actions like star, archive, snooze, trash.
// Routes: POST /api/messages/{message_id}/{action}
func (s *Server) handleMessageAction(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/messages/{message_id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	messageIDStr := parts[0]
	action := parts[1]

	var messageID int64
	if _, err := fmt.Sscanf(messageIDStr, "%d", &messageID); err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the first agent as the default recipient (for demo purposes).
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil || len(agents) == 0 {
		http.Error(w, "No agents found", http.StatusInternalServerError)
		return
	}
	agentID := agents[0].ID

	switch action {
	case "star":
		s.handleMessageStar(ctx, w, r, messageID, agentID)
	case "archive":
		s.handleMessageArchive(ctx, w, r, messageID, agentID)
	case "snooze-menu":
		s.handleMessageSnoozeMenu(ctx, w, r, messageID)
	case "snooze":
		s.handleMessageSnooze(ctx, w, r, messageID, agentID)
	case "trash":
		s.handleMessageTrash(ctx, w, r, messageID, agentID)
	case "ack":
		s.handleMessageAck(ctx, w, r, messageID, agentID)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
	}
}

// handleMessageStar toggles the star status of a message.
func (s *Server) handleMessageStar(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current state.
	recipient, err := s.store.Queries().GetMessageRecipient(ctx, sqlc.GetMessageRecipientParams{
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Toggle star state.
	newState := "starred"
	if recipient.State == "starred" {
		newState = "read"
	}

	err = s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     newState,
		Column2:   newState,
		ReadAt:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		http.Error(w, "Failed to update state", http.StatusInternalServerError)
		return
	}

	// Return updated star button.
	isStarred := newState == "starred"
	starClass := "text-gray-400"
	if isStarred {
		starClass = "text-yellow-400"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<button class="star-btn p-1 rounded hover:bg-gray-100 %s"
		hx-post="/api/messages/%d/star"
		hx-swap="outerHTML">
		<svg class="w-5 h-5" fill="%s" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
				d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/>
		</svg>
	</button>`, starClass, messageID, map[bool]string{true: "currentColor", false: "none"}[isStarred])
}

// handleMessageArchive archives a message.
func (s *Server) handleMessageArchive(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     "archived",
		Column2:   "archived",
		ReadAt:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		http.Error(w, "Failed to archive message", http.StatusInternalServerError)
		return
	}

	// Return empty response to remove the message from the list.
	w.Header().Set("HX-Trigger", "messageArchived")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(""))
}

// handleMessageSnoozeMenu returns the snooze options menu as a modal overlay.
func (s *Server) handleMessageSnoozeMenu(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID int64,
) {
	w.Header().Set("Content-Type", "text/html")
	// Render a modal overlay with the snooze menu.
	// The backdrop click closes the modal. Snooze buttons use htmx to post and then
	// trigger a custom event to close the modal after the request completes.
	fmt.Fprintf(w, `<div id="snooze-modal" class="fixed inset-0 z-50 flex items-center justify-center pointer-events-auto" onclick="if(event.target === this) this.remove()">
	<!-- Backdrop -->
	<div class="absolute inset-0 bg-black/20"></div>
	<!-- Menu -->
	<div class="relative bg-white rounded-lg shadow-xl border border-gray-200 py-2 min-w-[200px]">
		<div class="px-4 py-2 border-b border-gray-100">
			<h3 class="text-sm font-medium text-gray-900">Snooze until...</h3>
		</div>
		<button class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-3"
			hx-post="/api/messages/%d/snooze?duration=1h"
			hx-target="#message-%d"
			hx-swap="outerHTML"
			hx-on::after-request="document.getElementById('snooze-modal')?.remove()">
			<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
			</svg>
			1 hour
		</button>
		<button class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-3"
			hx-post="/api/messages/%d/snooze?duration=4h"
			hx-target="#message-%d"
			hx-swap="outerHTML"
			hx-on::after-request="document.getElementById('snooze-modal')?.remove()">
			<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
			</svg>
			4 hours
		</button>
		<button class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-3"
			hx-post="/api/messages/%d/snooze?duration=24h"
			hx-target="#message-%d"
			hx-swap="outerHTML"
			hx-on::after-request="document.getElementById('snooze-modal')?.remove()">
			<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/>
			</svg>
			Tomorrow
		</button>
		<button class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-3"
			hx-post="/api/messages/%d/snooze?duration=168h"
			hx-target="#message-%d"
			hx-swap="outerHTML"
			hx-on::after-request="document.getElementById('snooze-modal')?.remove()">
			<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
			</svg>
			Next week
		</button>
	</div>
</div>`, messageID, messageID, messageID, messageID, messageID, messageID, messageID, messageID)
}

// handleMessageSnooze snoozes a message for the specified duration.
func (s *Server) handleMessageSnooze(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	durationStr := r.URL.Query().Get("duration")
	if durationStr == "" {
		durationStr = "1h"
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		http.Error(w, "Invalid duration", http.StatusBadRequest)
		return
	}

	snoozedUntil := time.Now().Add(duration).Unix()

	err = s.store.Queries().UpdateRecipientSnoozed(ctx, sqlc.UpdateRecipientSnoozedParams{
		SnoozedUntil: sql.NullInt64{Int64: snoozedUntil, Valid: true},
		MessageID:    messageID,
		AgentID:      agentID,
	})
	if err != nil {
		http.Error(w, "Failed to snooze message", http.StatusInternalServerError)
		return
	}

	// Return empty response to remove the message from the list.
	w.Header().Set("HX-Trigger", "messageSnoozed")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(""))
}

// handleMessageTrash moves a message to trash.
func (s *Server) handleMessageTrash(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     "trash",
		Column2:   "trash",
		ReadAt:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		http.Error(w, "Failed to trash message", http.StatusInternalServerError)
		return
	}

	// Return empty response to remove the message from the list.
	w.Header().Set("HX-Trigger", "messageTrashed")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(""))
}

// handleMessageAck acknowledges a message.
func (s *Server) handleMessageAck(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.store.Queries().UpdateRecipientAcked(ctx, sqlc.UpdateRecipientAckedParams{
		AckedAt:   sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		http.Error(w, "Failed to acknowledge message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<span class="text-green-600 text-sm">Acknowledged</span>`))
}

// handleAgentAction routes agent action requests to the appropriate handler.
// Handles: /agents/{id}, /agents/{id}/message, /agents/{id}/session.
func (s *Server) handleAgentAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse path: /agents/{id} or /agents/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/agents/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent ID", http.StatusBadRequest)
		return
	}

	// Determine action (default is view details).
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		s.handleAgentDetails(ctx, w, r, agentID)
	case "message":
		s.handleAgentMessage(ctx, w, r, agentID)
	case "session":
		s.handleAgentSession(ctx, w, r, agentID)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
	}
}

// handleAgentDetails shows the agent details modal.
func (s *Server) handleAgentDetails(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	agentID int64,
) {
	// Get agent from database.
	agent, err := s.store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Render agent details modal.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="fixed inset-0 z-50 flex items-center justify-center pointer-events-auto" onclick="if(event.target === this) this.remove()">
	<div class="absolute inset-0 bg-black/50"></div>
	<div class="relative bg-white rounded-xl shadow-2xl w-full max-w-lg mx-4 overflow-hidden">
		<div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 bg-gradient-to-r from-blue-50 to-indigo-50">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 bg-blue-100 rounded-full flex items-center justify-center">
					<span class="text-blue-600 font-semibold">%s</span>
				</div>
				<div>
					<h2 class="text-lg font-semibold text-gray-900">%s</h2>
					<p class="text-sm text-gray-500">Agent Details</p>
				</div>
			</div>
			<button onclick="this.closest('.fixed').remove()" class="p-1 text-gray-400 hover:text-gray-600 rounded">
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
				</svg>
			</button>
		</div>
		<div class="px-6 py-4 space-y-4">
			<div>
				<label class="text-xs font-medium text-gray-500 uppercase">Name</label>
				<p class="text-sm text-gray-900">%s</p>
			</div>
			<div>
				<label class="text-xs font-medium text-gray-500 uppercase">Project</label>
				<p class="text-sm text-gray-900 font-mono">%s</p>
			</div>
			<div>
				<label class="text-xs font-medium text-gray-500 uppercase">Created</label>
				<p class="text-sm text-gray-900">%s</p>
			</div>
			<div>
				<label class="text-xs font-medium text-gray-500 uppercase">Last Active</label>
				<p class="text-sm text-gray-900">%s</p>
			</div>
		</div>
		<div class="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50 border-t border-gray-200">
			<button onclick="this.closest('.fixed').remove()" class="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg">
				Close
			</button>
			<button class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
				hx-get="/agents/%d/message"
				hx-target="#modal-container"
				hx-swap="innerHTML"
				onclick="this.closest('.fixed').remove()">
				Send Message
			</button>
		</div>
	</div>
</div>`,
		string(agent.Name[0]),
		agent.Name,
		agent.Name,
		agent.ProjectKey.String,
		time.Unix(agent.CreatedAt, 0).Format("Jan 2, 2006 3:04 PM"),
		time.Unix(agent.LastActiveAt, 0).Format("Jan 2, 2006 3:04 PM"),
		agentID,
	)
}

// handleAgentMessage shows the message agent modal.
func (s *Server) handleAgentMessage(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	agentID int64,
) {
	// Get agent from database.
	agent, err := s.store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Render message modal.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="fixed inset-0 z-50 flex items-center justify-center pointer-events-auto" onclick="if(event.target === this) this.remove()">
	<div class="absolute inset-0 bg-black/50"></div>
	<div class="relative bg-white rounded-xl shadow-2xl w-full max-w-2xl mx-4 overflow-hidden">
		<div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 bg-gradient-to-r from-blue-50 to-indigo-50">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 bg-blue-100 rounded-full flex items-center justify-center">
					<svg class="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z"/>
					</svg>
				</div>
				<div>
					<h2 class="text-lg font-semibold text-gray-900">Message %s</h2>
					<p class="text-sm text-gray-500">Send a direct message to this agent</p>
				</div>
			</div>
			<button onclick="this.closest('.fixed').remove()" class="p-1 text-gray-400 hover:text-gray-600 rounded">
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
				</svg>
			</button>
		</div>
		<form hx-post="/api/messages/send" hx-target="#modal-container" hx-swap="innerHTML">
			<input type="hidden" name="to_agent_id" value="%d">
			<div class="px-6 py-4 space-y-4">
				<div>
					<label for="subject" class="block text-sm font-medium text-gray-700 mb-1">Subject</label>
					<input type="text" id="subject" name="subject" required
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
						placeholder="What's this about?">
				</div>
				<div>
					<label for="body" class="block text-sm font-medium text-gray-700 mb-1">Message</label>
					<textarea id="body" name="body" required rows="6"
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-none"
						placeholder="Write your message..."></textarea>
				</div>
				<div>
					<label for="priority" class="block text-sm font-medium text-gray-700 mb-1">Priority</label>
					<select id="priority" name="priority"
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none">
						<option value="normal">Normal</option>
						<option value="urgent">Urgent</option>
						<option value="low">Low</option>
					</select>
				</div>
			</div>
			<div class="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50 border-t border-gray-200">
				<button type="button" onclick="this.closest('.fixed').remove()" class="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg">
					Cancel
				</button>
				<button type="submit" class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 flex items-center gap-2">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"/>
					</svg>
					Send Message
				</button>
			</div>
		</form>
	</div>
</div>`, agent.Name, agentID)
}

// handleAgentSession shows the start session modal for an agent.
func (s *Server) handleAgentSession(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	agentID int64,
) {
	// Get agent from database.
	agent, err := s.store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Render session modal.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="fixed inset-0 z-50 flex items-center justify-center pointer-events-auto" onclick="if(event.target === this) this.remove()">
	<div class="absolute inset-0 bg-black/50"></div>
	<div class="relative bg-white rounded-xl shadow-2xl w-full max-w-2xl mx-4 overflow-hidden">
		<div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 bg-gradient-to-r from-green-50 to-blue-50">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 bg-green-100 rounded-full flex items-center justify-center">
					<svg class="w-5 h-5 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"/>
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
					</svg>
				</div>
				<div>
					<h2 class="text-lg font-semibold text-gray-900">Start Session with %s</h2>
					<p class="text-sm text-gray-500">Kick off a new work session</p>
				</div>
			</div>
			<button onclick="this.closest('.fixed').remove()" class="p-1 text-gray-400 hover:text-gray-600 rounded">
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
				</svg>
			</button>
		</div>
		<form hx-post="/api/sessions" hx-target="#modal-container" hx-swap="innerHTML">
			<input type="hidden" name="agent_id" value="%d">
			<div class="px-6 py-4 space-y-4">
				<div>
					<label for="goal" class="block text-sm font-medium text-gray-700 mb-1">Goal / Task Description</label>
					<textarea id="goal" name="goal" required rows="3"
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-green-500 outline-none resize-none"
						placeholder="Describe what you want the agent to accomplish..."></textarea>
				</div>
				<div>
					<label for="work_dir" class="block text-sm font-medium text-gray-700 mb-1">
						Working Directory <span class="text-gray-400">(optional)</span>
					</label>
					<input type="text" id="work_dir" name="work_dir"
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-green-500 outline-none font-mono text-sm"
						placeholder="%s"
						value="%s">
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="git_branch" class="block text-sm font-medium text-gray-700 mb-1">
							Git Branch <span class="text-gray-400">(optional)</span>
						</label>
						<input type="text" id="git_branch" name="git_branch"
							class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-green-500 outline-none font-mono text-sm"
							placeholder="feature/my-feature">
					</div>
					<div>
						<label for="priority" class="block text-sm font-medium text-gray-700 mb-1">Priority</label>
						<select id="priority" name="priority"
							class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-green-500 outline-none">
							<option value="normal">Normal</option>
							<option value="high">High</option>
							<option value="urgent">Urgent</option>
							<option value="low">Low</option>
						</select>
					</div>
				</div>
			</div>
			<div class="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50 border-t border-gray-200">
				<button type="button" onclick="this.closest('.fixed').remove()" class="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg">
					Cancel
				</button>
				<button type="submit" class="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 flex items-center gap-2">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"/>
					</svg>
					Start Session
				</button>
			</div>
		</form>
	</div>
</div>`, agent.Name, agentID, agent.ProjectKey.String, agent.ProjectKey.String)
}
