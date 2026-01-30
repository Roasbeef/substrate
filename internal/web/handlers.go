package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
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

	// Agent context for inbox filtering.
	CurrentAgentID   int64  // 0 = global view (all agents).
	CurrentAgentName string // "Global" or agent name.
	AllAgents        []AgentSwitcherItem

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

// AgentSwitcherItem represents an agent in the switcher dropdown.
type AgentSwitcherItem struct {
	ID          int64
	Name        string
	DisplayName string // "CodeName@repo.branch" format.
	IsActive    bool
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
	RecipientName    string // For global view, shows which agent received the message.
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
	DisplayName    string // "CodeName@repo.branch" format for at-a-glance ID.
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
	ShowSuccess   bool // Show success message after reply.
}

// ThreadMessage represents a message within a thread view.
type ThreadMessage struct {
	ID            string
	SenderName    string
	SenderAvatar  string
	RecipientName string
	Body          string
	State         string // "unread", "read", "starred"
	TopicName     string
	Deadline      time.Time
	AckedAt       time.Time
	CreatedAt     time.Time
	IsStarred     bool
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

// buildAgentSwitcherItems creates the agent switcher dropdown items.
// currentAgentID is the currently selected agent (0 for Global view).
func (s *Server) buildAgentSwitcherItems(
	ctx context.Context, currentAgentID int64,
) []AgentSwitcherItem {
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil {
		agents = nil
	}

	items := make([]AgentSwitcherItem, 0, len(agents)+1)
	// Add "Global" option first.
	items = append(items, AgentSwitcherItem{
		ID:          0,
		Name:        "Global",
		DisplayName: "Global",
		IsActive:    currentAgentID == 0,
	})
	for _, a := range agents {
		displayName := formatAgentDisplayName(
			a.Name, a.ProjectKey.String, a.GitBranch.String,
		)
		items = append(items, AgentSwitcherItem{
			ID:          a.ID,
			Name:        a.Name,
			DisplayName: displayName,
			IsActive:    a.ID == currentAgentID,
		})
	}
	return items
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

	// Get all agents for the switcher dropdown.
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil {
		agents = nil
	}

	// Determine which agent's inbox to show.
	// Priority: query param > User agent > first non-User agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var currentAgentID int64
	var currentAgentName string

	if agentIDStr == "all" || agentIDStr == "0" {
		// Global view - show all messages.
		currentAgentID = 0
		currentAgentName = "Global"
	} else if agentIDStr != "" {
		// Specific agent requested.
		fmt.Sscanf(agentIDStr, "%d", &currentAgentID)
		for _, a := range agents {
			if a.ID == currentAgentID {
				currentAgentName = formatAgentDisplayName(
					a.Name, a.ProjectKey.String, a.GitBranch.String,
				)
				break
			}
		}
	} else {
		// Default: find User agent, or fall back to first non-User agent.
		for _, a := range agents {
			if a.Name == UserAgentName {
				currentAgentID = a.ID
				currentAgentName = formatAgentDisplayName(
					a.Name, a.ProjectKey.String, a.GitBranch.String,
				)
				break
			}
		}
		// If no User agent found, use first non-User agent.
		if currentAgentID == 0 && len(agents) > 0 {
			for _, a := range agents {
				if a.Name != UserAgentName {
					currentAgentID = a.ID
					currentAgentName = formatAgentDisplayName(
						a.Name, a.ProjectKey.String, a.GitBranch.String,
					)
					break
				}
			}
		}
	}

	// Build agent switcher items.
	agentSwitcherItems := make([]AgentSwitcherItem, 0, len(agents)+1)
	// Add "Global" option first.
	agentSwitcherItems = append(agentSwitcherItems, AgentSwitcherItem{
		ID:          0,
		Name:        "Global",
		DisplayName: "Global",
		IsActive:    currentAgentID == 0,
	})
	for _, a := range agents {
		displayName := formatAgentDisplayName(
			a.Name, a.ProjectKey.String, a.GitBranch.String,
		)
		agentSwitcherItems = append(agentSwitcherItems, AgentSwitcherItem{
			ID:          a.ID,
			Name:        a.Name,
			DisplayName: displayName,
			IsActive:    a.ID == currentAgentID,
		})
	}

	// Get real counts from store.
	var unreadCount, urgentCount int64
	if currentAgentID > 0 {
		unreadCount, _ = s.store.Queries().CountUnreadByAgent(ctx, currentAgentID)
		urgentCount, _ = s.store.Queries().CountUnreadUrgentByAgent(ctx, currentAgentID)
	} else {
		// Global view - sum across all agents.
		for _, a := range agents {
			cnt, _ := s.store.Queries().CountUnreadByAgent(ctx, a.ID)
			unreadCount += cnt
			urg, _ := s.store.Queries().CountUnreadUrgentByAgent(ctx, a.ID)
			urgentCount += urg
		}
	}

	// Count topics.
	topics, _ := s.store.Queries().ListTopics(ctx)
	topicsCount := len(topics)

	// Count agents.
	agentsCount := len(agents)

	totalCount := int(unreadCount)
	page := 1
	pageSize := 50

	// Build messages endpoint with agent filter.
	messagesEndpoint := "/inbox/messages"
	if currentAgentID == 0 {
		messagesEndpoint = "/inbox/messages?agent_id=all"
	} else {
		messagesEndpoint = fmt.Sprintf("/inbox/messages?agent_id=%d", currentAgentID)
	}

	data := PageData{
		Title:            "Inbox",
		ActiveNav:        "inbox",
		UnreadCount:      int(unreadCount),
		Category:         category,
		MessagesEndpoint: messagesEndpoint,
		CurrentAgentID:   currentAgentID,
		CurrentAgentName: currentAgentName,
		AllAgents:        agentSwitcherItems,
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

	// Build agent switcher items.
	agentSwitcherItems := s.buildAgentSwitcherItems(ctx, 0)

	data := PageData{
		Title:     "Agent Dashboard",
		ActiveNav: "agents",
		AllAgents: agentSwitcherItems,
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

	// Get all agents for the switcher dropdown.
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil {
		agents = nil
	}

	// Determine which agent's sent messages to show.
	agentIDStr := r.URL.Query().Get("agent_id")
	var currentAgentID int64
	var currentAgentName string

	if agentIDStr == "all" || agentIDStr == "0" {
		// Global view - show all sent messages.
		currentAgentID = 0
		currentAgentName = "Global"
	} else if agentIDStr != "" {
		// Specific agent requested.
		fmt.Sscanf(agentIDStr, "%d", &currentAgentID)
		for _, a := range agents {
			if a.ID == currentAgentID {
				currentAgentName = formatAgentDisplayName(
					a.Name, a.ProjectKey.String, a.GitBranch.String,
				)
				break
			}
		}
	} else {
		// Default: show all sent (Global view).
		currentAgentID = 0
		currentAgentName = "Global"
	}

	// Build agent switcher items.
	agentSwitcherItems := make([]AgentSwitcherItem, 0, len(agents)+1)
	agentSwitcherItems = append(agentSwitcherItems, AgentSwitcherItem{
		ID:          0,
		Name:        "Global",
		DisplayName: "Global",
		IsActive:    currentAgentID == 0,
	})
	for _, a := range agents {
		displayName := formatAgentDisplayName(
			a.Name, a.ProjectKey.String, a.GitBranch.String,
		)
		agentSwitcherItems = append(agentSwitcherItems, AgentSwitcherItem{
			ID:          a.ID,
			Name:        a.Name,
			DisplayName: displayName,
			IsActive:    a.ID == currentAgentID,
		})
	}

	// Get sent count.
	var sentCount int64
	if currentAgentID > 0 {
		sentCount, _ = s.store.Queries().CountSentByAgent(ctx, currentAgentID)
	} else {
		// Global view - sum across all agents.
		for _, a := range agents {
			cnt, _ := s.store.Queries().CountSentByAgent(ctx, a.ID)
			sentCount += cnt
		}
	}

	// Build endpoint with agent_id parameter.
	endpoint := "/sent/messages"
	if currentAgentID > 0 {
		endpoint = fmt.Sprintf("/sent/messages?agent_id=%d", currentAgentID)
	} else {
		endpoint = "/sent/messages?agent_id=0"
	}

	data := PageData{
		Title:            "Sent",
		ActiveNav:        "sent",
		MessagesEndpoint: endpoint,
		UnreadCount:      int(sentCount),
		CurrentAgentID:   currentAgentID,
		CurrentAgentName: currentAgentName,
		AllAgents:        agentSwitcherItems,
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

	// Check if global view is requested.
	agentIDStr := r.URL.Query().Get("agent_id")
	isGlobal := agentIDStr == "all" || agentIDStr == "0"

	var messages []MessageView

	if isGlobal {
		// Global view: get all messages.
		dbMessages, err := s.store.Queries().GetAllInboxMessages(ctx, 50)
		if err != nil {
			s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
			return
		}

		messages = make([]MessageView, len(dbMessages))
		for i, m := range dbMessages {
			// Get sender name with display format.
			senderName := fmt.Sprintf("Agent#%d", m.SenderID)
			sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
			if err == nil {
				senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
			}

			// Get recipient name for global view context.
			recipientName := ""
			if recipient, err := s.store.Queries().GetAgent(ctx, m.RecipientAgentID); err == nil {
				recipientName = formatAgentDisplayName(
					recipient.Name, recipient.ProjectKey.String, recipient.GitBranch.String,
				)
			}

			initials := getInitials(senderName)

			messages[i] = MessageView{
				ID:             fmt.Sprintf("%d", m.ID),
				ThreadID:       m.ThreadID,
				SenderName:     senderName,
				SenderInitials: initials,
				RecipientName:  recipientName,
				Subject:        m.Subject,
				Body:           m.BodyMd,
				State:          m.State,
				IsStarred:      m.State == "starred",
				IsImportant:    m.Priority == "urgent",
				IsAgent:        true,
				CreatedAt:      time.Unix(m.CreatedAt, 0),
			}
		}
	} else {
		// Specific agent view.
		var agentID int64
		if agentIDStr != "" {
			fmt.Sscanf(agentIDStr, "%d", &agentID)
		} else {
			// Default: find User agent, or fall back to first non-User agent.
			agents, err := s.store.Queries().ListAgents(ctx)
			if err == nil && len(agents) > 0 {
				for _, a := range agents {
					if a.Name == UserAgentName {
						agentID = a.ID
						break
					}
				}
				if agentID == 0 {
					for _, a := range agents {
						if a.Name != UserAgentName {
							agentID = a.ID
							break
						}
					}
				}
			}
		}

		if agentID == 0 {
			s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
			return
		}

		dbMessages, err := s.store.Queries().GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
			AgentID: agentID,
			Limit:   50,
		})
		if err != nil {
			s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
			return
		}

		messages = make([]MessageView, len(dbMessages))
		for i, m := range dbMessages {
			senderName := fmt.Sprintf("Agent#%d", m.SenderID)
			sender, err := s.store.Queries().GetAgent(ctx, m.SenderID)
			if err == nil {
				senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
			}

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

// formatAgentDisplayName creates "CodeName@repo.branch" format for at-a-glance
// agent identification. Falls back to just the codename if project/branch are
// not available.
func formatAgentDisplayName(name, projectKey, branch string) string {
	if projectKey == "" && branch == "" {
		return name
	}
	repo := filepath.Base(projectKey)
	if repo == "" || repo == "." {
		return name // Just codename if no repo.
	}
	if branch == "" {
		branch = "main"
	}
	return fmt.Sprintf("%s@%s.%s", name, repo, branch)
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

	// Check if global view is requested.
	agentIDStr := r.URL.Query().Get("agent_id")
	isGlobal := agentIDStr == "all" || agentIDStr == "0"

	var messages []MessageView

	if isGlobal {
		// Global view: get all sent messages across all agents.
		dbMessages, err := s.store.Queries().GetAllSentMessages(ctx, 50)
		if err != nil {
			s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
			return
		}
		messages = s.convertAllSentMessagesToView(dbMessages)
	} else {
		// Agent-specific view.
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

		dbMessages, err := s.store.Queries().GetSentMessages(
			ctx, sqlc.GetSentMessagesParams{
				SenderID: agentID,
				Limit:    50,
			},
		)
		if err != nil {
			s.renderPartial(w, "message-list", MessagesListData{Messages: nil})
			return
		}
		messages = s.convertSentMessagesToView(ctx, dbMessages)
	}

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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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

// convertAllSentMessagesToView converts global sent messages to view models.
// The sender name is already included in the query result.
func (s *Server) convertAllSentMessagesToView(
	dbMessages []sqlc.GetAllSentMessagesRow,
) []MessageView {
	messages := make([]MessageView, len(dbMessages))
	for i, m := range dbMessages {
		messages[i] = MessageView{
			ID:             fmt.Sprintf("%d", m.ID),
			ThreadID:       m.ThreadID,
			SenderName:     m.SenderName,
			SenderInitials: getInitials(m.SenderName),
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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
    <main id="main-content" class="ml-64 flex-1 min-h-screen">
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
    </main>
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
			<script>document.getElementById('search-results').classList.remove('hidden')</script>`, err.Error())
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
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

// handleSettings renders the settings page.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current agent list for default agent selection.
	agents, _ := s.store.Queries().ListAgents(ctx)

	// Get topics for subscription management.
	topics, _ := s.store.Queries().ListTopics(ctx)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Settings - Substrate</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <script src="/static/js/vendor/htmx.min.js"></script>
</head>
<body class="bg-gray-50">
    <div class="max-w-4xl mx-auto p-6">
        <div class="mb-6">
            <a href="/inbox" class="text-blue-600 hover:underline">← Back to Inbox</a>
        </div>

        <h1 class="text-2xl font-bold mb-6">Settings</h1>

        <!-- Agent Settings -->
        <div class="bg-white rounded-lg shadow p-6 mb-6">
            <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
                <svg class="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
                </svg>
                Agent Configuration
            </h2>
            <div class="space-y-4">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">Default Agent</label>
                    <select class="w-full md:w-1/2 px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none">
                        <option value="">Select default agent...</option>`)

	for _, agent := range agents {
		if agent.Name != UserAgentName {
			fmt.Fprintf(w, `<option value="%d">%s</option>`, agent.ID, agent.Name)
		}
	}

	fmt.Fprint(w, `
                    </select>
                    <p class="text-sm text-gray-500 mt-1">The agent used for viewing your inbox by default.</p>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">Registered Agents</label>
                    <div class="bg-gray-50 rounded-lg p-4">`)

	if len(agents) == 0 {
		fmt.Fprint(w, `<p class="text-gray-500">No agents registered yet.</p>`)
	} else {
		fmt.Fprint(w, `<div class="space-y-2">`)
		for _, agent := range agents {
			if agent.Name != UserAgentName {
				projectStr := "No project"
				if agent.ProjectKey.Valid {
					projectStr = shortProject(agent.ProjectKey.String)
				}
				fmt.Fprintf(w, `
                            <div class="flex items-center justify-between p-2 bg-white rounded border">
                                <div>
                                    <span class="font-medium">%s</span>
                                    <span class="text-sm text-gray-500 ml-2">%s</span>
                                </div>
                                <button class="text-red-600 hover:text-red-700 text-sm">Remove</button>
                            </div>`, agent.Name, projectStr)
			}
		}
		fmt.Fprint(w, `</div>`)
	}

	fmt.Fprint(w, `
                    </div>
                </div>
            </div>
        </div>

        <!-- Topic Subscriptions -->
        <div class="bg-white rounded-lg shadow p-6 mb-6">
            <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
                <svg class="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z"/>
                </svg>
                Topic Subscriptions
            </h2>
            <div class="space-y-2">`)

	if len(topics) == 0 {
		fmt.Fprint(w, `<p class="text-gray-500">No topics available.</p>`)
	} else {
		for _, topic := range topics {
			fmt.Fprintf(w, `
                <div class="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                    <div class="flex items-center gap-3">
                        <input type="checkbox" id="topic-%d" class="w-4 h-4 text-blue-600 rounded">
                        <label for="topic-%d" class="cursor-pointer">
                            <span class="font-medium">%s</span>
                            <span class="text-sm text-gray-500 ml-2">(%s)</span>
                        </label>
                    </div>
                </div>`, topic.ID, topic.ID, topic.Name, topic.TopicType)
		}
	}

	fmt.Fprintf(w, `
            </div>
        </div>

        <!-- Notification Settings -->
        <div class="bg-white rounded-lg shadow p-6 mb-6">
            <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
                <svg class="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
                </svg>
                Notifications
            </h2>
            <div class="space-y-3">
                <div class="flex items-center justify-between">
                    <div>
                        <span class="font-medium">Email Notifications</span>
                        <p class="text-sm text-gray-500">Receive email for urgent messages</p>
                    </div>
                    <label class="relative inline-flex items-center cursor-pointer">
                        <input type="checkbox" class="sr-only peer">
                        <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                    </label>
                </div>
                <div class="flex items-center justify-between">
                    <div>
                        <span class="font-medium">Desktop Notifications</span>
                        <p class="text-sm text-gray-500">Show browser notifications</p>
                    </div>
                    <div class="flex items-center gap-3">
                        <button onclick="testNotification()" class="px-3 py-1 text-sm bg-blue-100 text-blue-700 rounded hover:bg-blue-200">
                            Test
                        </button>
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" id="desktop-notifications-toggle" class="sr-only peer" onchange="toggleDesktopNotifications(this)">
                            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                        </label>
                    </div>
                </div>
                <div class="flex items-center justify-between">
                    <div>
                        <span class="font-medium">Sound Alerts</span>
                        <p class="text-sm text-gray-500">Play sound for new messages</p>
                    </div>
                    <label class="relative inline-flex items-center cursor-pointer">
                        <input type="checkbox" class="sr-only peer">
                        <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                    </label>
                </div>
            </div>
        </div>

        <!-- Database Info -->
        <div class="bg-white rounded-lg shadow p-6">
            <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
                <svg class="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4"/>
                </svg>
                System Information
            </h2>
            <div class="space-y-2 text-sm">
                <div class="flex justify-between py-2 border-b">
                    <span class="text-gray-600">Agents</span>
                    <span class="font-medium">%d</span>
                </div>
                <div class="flex justify-between py-2 border-b">
                    <span class="text-gray-600">Topics</span>
                    <span class="font-medium">%d</span>
                </div>
                <div class="flex justify-between py-2">
                    <span class="text-gray-600">Database</span>
                    <span class="font-medium text-green-600">Connected</span>
                </div>
            </div>
        </div>

        <div class="mt-6 flex justify-end">
            <button class="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
                Save Settings
            </button>
        </div>
    </div>

    <script src="/static/js/main.js"></script>
</body>
</html>`, len(agents)-1, len(topics))
}

// handleThread returns the thread view partial.
func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	threadID := strings.TrimPrefix(r.URL.Path, "/thread/")
	s.renderThreadView(ctx, w, threadID, false)
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
		// Skip the User agent from card display.
		if agent.Name == UserAgentName {
			continue
		}
		s.partials.ExecuteTemplate(&html, "agent-card", agent)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html.String()))
}

// handleAPIActivity returns the activity feed partial.
func (s *Server) handleAPIActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch recent activities from database.
	dbActivities, err := s.store.Queries().ListRecentActivities(ctx, 20)
	if err != nil {
		dbActivities = nil
	}

	// Convert to view models.
	activities := make([]ActivityView, len(dbActivities))
	for i, act := range dbActivities {
		// Get agent name.
		agentName := fmt.Sprintf("Agent#%d", act.AgentID)
		if agent, err := s.store.Queries().GetAgent(ctx, act.AgentID); err == nil {
			agentName = agent.Name
		}

		activities[i] = ActivityView{
			ID:          fmt.Sprintf("%d", act.ID),
			AgentID:     fmt.Sprintf("%d", act.AgentID),
			AgentName:   agentName,
			Type:        act.ActivityType,
			Description: act.Description,
			Timestamp:   time.Unix(act.CreatedAt, 0),
		}
	}

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

// handleAPISessionCreate creates a new session for an agent.
func (s *Server) handleAPISessionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	agentIDStr := r.FormValue("agent_id")
	goal := r.FormValue("goal")
	workDir := r.FormValue("work_dir")
	gitBranch := r.FormValue("git_branch")
	priority := r.FormValue("priority")

	if agentIDStr == "" || goal == "" {
		http.Error(w, "Agent ID and goal are required", http.StatusBadRequest)
		return
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid agent ID", http.StatusBadRequest)
		return
	}

	// Get agent.
	ctx := r.Context()
	agent, err := s.store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Use agent's project key as work dir if not specified.
	if workDir == "" && agent.ProjectKey.Valid {
		workDir = agent.ProjectKey.String
	}

	// Generate session ID.
	sessionID := fmt.Sprintf("session-%d-%d", agentID, time.Now().Unix())

	// Log the session start activity.
	_, err = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      agentID,
		ActivityType: "session_start",
		Description:  fmt.Sprintf("Started session: %s", truncateString(goal, 50)),
		Metadata:     sql.NullString{},
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		// Log error but continue.
		log.Printf("Failed to log session activity: %v", err)
	}

	// For now, we don't actually spawn the agent (that would require the
	// Claude Agent SDK). Just show a success message and close the modal.
	// In a real implementation, we would use the spawner to start the agent.
	_ = gitBranch
	_ = priority
	_ = sessionID

	// Return success HTML that closes the modal and shows a toast.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Session started for `+agent.Name+`", "type": "success"}}`)
	fmt.Fprintf(w, `<script>
		document.querySelector('#modal-container > .fixed')?.remove();
		if (window.showToast) showToast('Session started for %s', 'success');
	</script>`, agent.Name)
}

// truncateString truncates a string to the given length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

	// Disable write deadline for SSE (long-lived connection).
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("SSE: Failed to disable write deadline: %v", err)
	}

	ctx := r.Context()
	// Use 15s interval to reduce update frequency and prevent flashing.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Send initial agent status.
	s.sendAgentStatusEvent(w, flusher)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendAgentStatusEvent(w, flusher)
		}
	}
}

// sendAgentStatusEvent sends an SSE event with current agent statuses.
func (s *Server) sendAgentStatusEvent(w http.ResponseWriter, flusher http.Flusher) {
	// Get agents with full details for rendering.
	agents := s.getAgentsFromHeartbeat()

	// Send compact sidebar items.
	var sidebarHTML strings.Builder
	for _, agent := range agents {
		if agent.Name == UserAgentName {
			continue
		}
		if err := s.partials.ExecuteTemplate(&sidebarHTML, "agent-sidebar-item", agent); err != nil {
			continue
		}
	}

	// Send full cards for dashboard.
	var cardsHTML strings.Builder
	for _, agent := range agents {
		if agent.Name == UserAgentName {
			continue
		}
		if err := s.partials.ExecuteTemplate(&cardsHTML, "agent-card", agent); err != nil {
			continue
		}
	}

	// Send sidebar update event.
	sidebarData := strings.ReplaceAll(sidebarHTML.String(), "\n", "")
	if len(sidebarData) > 0 {
		fmt.Fprintf(w, "event: agent-sidebar-update\ndata: %s\n\n", sidebarData)
	}

	// Send cards update event for dashboard.
	cardsData := strings.ReplaceAll(cardsHTML.String(), "\n", "")
	if len(cardsData) > 0 {
		fmt.Fprintf(w, "event: agent-update\ndata: %s\n\n", cardsData)
	}

	flusher.Flush()
}

// statusColor returns the CSS class for an agent status color.
func statusColor(status string) string {
	switch status {
	case "active":
		return "bg-green-500"
	case "busy":
		return "bg-yellow-500"
	case "idle":
		return "bg-gray-400"
	default:
		return "bg-red-500"
	}
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

	// Disable write deadline for SSE (long-lived connection).
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("SSE: Failed to disable write deadline: %v", err)
	}

	ctx := r.Context()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastActivityID int64

	// Send initial activities.
	lastActivityID = s.sendActivityEvent(w, flusher, lastActivityID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastActivityID = s.sendActivityEvent(w, flusher, lastActivityID)
		}
	}
}

// sendActivityEvent sends an SSE event with new activities since lastID.
func (s *Server) sendActivityEvent(w http.ResponseWriter, flusher http.Flusher, lastID int64) int64 {
	ctx := context.Background()
	activities, err := s.store.Queries().ListRecentActivities(ctx, 10)
	if err != nil || len(activities) == 0 {
		return lastID
	}

	// Find new activities.
	var newActivities []sqlc.Activity
	for _, act := range activities {
		if act.ID > lastID {
			newActivities = append(newActivities, act)
		}
	}

	if len(newActivities) == 0 {
		return lastID
	}

	// Build HTML for new activities.
	var html strings.Builder
	for _, act := range newActivities {
		agentName := fmt.Sprintf("Agent#%d", act.AgentID)
		if agent, err := s.store.Queries().GetAgent(ctx, act.AgentID); err == nil {
			agentName = agent.Name
		}

		icon := activityIcon(act.ActivityType)
		timeAgo := formatTimeAgo(time.Unix(act.CreatedAt, 0))

		html.WriteString(fmt.Sprintf(
			`<div class="flex items-start gap-3 p-3 border-b border-gray-100">
				<div class="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center text-blue-600">%s</div>
				<div class="flex-1">
					<p class="text-sm"><strong>%s</strong> %s</p>
					<p class="text-xs text-gray-500">%s</p>
				</div>
			</div>`,
			icon, agentName, act.Description, timeAgo,
		))
	}

	fmt.Fprintf(w, "event: activity-update\ndata: %s\n\n", strings.ReplaceAll(html.String(), "\n", ""))
	flusher.Flush()

	return activities[0].ID // Return the highest ID.
}

// activityIcon returns an icon for an activity type.
func activityIcon(activityType string) string {
	switch activityType {
	case "commit":
		return "📝"
	case "message":
		return "✉️"
	case "session_start":
		return "🚀"
	case "session_complete":
		return "✅"
	case "decision":
		return "⚖️"
	case "error":
		return "❌"
	case "blocker":
		return "🚧"
	default:
		return "📌"
	}
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

	// Disable write deadline for SSE (long-lived connection).
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("SSE: Failed to disable write deadline: %v", err)
	}

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second) // Fast polling for snappy real-time feel.
	defer ticker.Stop()

	// Get agent ID from query param or use first available agent.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64
	if agentIDStr != "" {
		agentID, _ = strconv.ParseInt(agentIDStr, 10, 64)
	} else {
		// Use first non-User agent.
		agents, _ := s.store.Queries().ListAgents(ctx)
		for _, a := range agents {
			if a.Name != UserAgentName {
				agentID = a.ID
				break
			}
		}
	}

	// Track last message ID to detect new messages.
	var lastMessageID int64
	messages, err := s.store.Queries().GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   1,
	})
	if err == nil && len(messages) > 0 {
		lastMessageID = messages[0].ID
	}
	log.Printf("SSE: Connection started for agent %d, lastMessageID=%d", agentID, lastMessageID)

	// Send initial unread count.
	s.sendInboxCountEvent(w, flusher, agentID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for new messages and send notification event (JSON, not HTML).
			// JavaScript handles this by triggering a morph refresh.
			lastMessageID = s.sendNewMessagesEvent(w, flusher, agentID, lastMessageID)
			s.sendInboxCountEvent(w, flusher, agentID)
		}
	}
}

// sendInboxCountEvent sends an SSE event with unread message count.
func (s *Server) sendInboxCountEvent(w http.ResponseWriter, flusher http.Flusher, agentID int64) {
	ctx := context.Background()
	count, err := s.store.Queries().CountUnreadByAgent(ctx, agentID)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "event: unread-count\ndata: %d\n\n", count)
	flusher.Flush()
}

// newMessageEvent represents a new message notification for SSE.
type newMessageEvent struct {
	ID       int64  `json:"id"`
	ThreadID string `json:"thread_id"`
	Sender   string `json:"sender"`
	Project  string `json:"project"`
	Subject  string `json:"subject"`
	Preview  string `json:"preview"`
	Priority string `json:"priority"`
	State    string `json:"state"`
}

// sendNewMessagesEvent sends an SSE event with new message metadata as JSON.
// The client uses this to: (1) check for duplicates, (2) trigger list refresh, (3) show notifications.
func (s *Server) sendNewMessagesEvent(w http.ResponseWriter, flusher http.Flusher, agentID int64, lastID int64) int64 {
	ctx := context.Background()
	messages, err := s.store.Queries().GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   10,
	})
	if err != nil || len(messages) == 0 {
		return lastID
	}

	// Find new messages.
	var newMsgs []newMessageEvent
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.ID > lastID {
			// Get sender name and project.
			senderName := "Unknown"
			senderProject := ""
			if sender, err := s.store.Queries().GetAgent(ctx, m.SenderID); err == nil {
				senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
				if sender.ProjectKey.Valid {
					senderProject = shortProject(sender.ProjectKey.String)
				}
			}

			// Truncate body preview.
			preview := m.BodyMd
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}

			newMsgs = append(newMsgs, newMessageEvent{
				ID:       m.ID,
				ThreadID: m.ThreadID,
				Sender:   senderName,
				Project:  senderProject,
				Subject:  m.Subject,
				Preview:  preview,
				Priority: m.Priority,
				State:    m.State,
			})
		}
	}

	if len(newMsgs) == 0 {
		return lastID
	}

	// Send as JSON SSE event - client will handle deduplication and refresh.
	jsonData, err := json.Marshal(newMsgs)
	if err != nil {
		return lastID
	}

	log.Printf("SSE: Sending new-message event with %d messages (lastID was %d, now %d)",
		len(newMsgs), lastID, messages[0].ID)
	fmt.Fprintf(w, "event: new-message\ndata: %s\n\n", string(jsonData))
	flusher.Flush()

	return messages[0].ID
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

	// If session ID provided, track it and record activity.
	if req.SessionID != "" {
		agentData, err := s.registry.GetAgentByName(ctx, req.AgentName)
		if err == nil {
			// Check if this is a new session.
			existingSession := s.heartbeatMgr.GetActiveSessionID(agentData.ID)
			if existingSession != req.SessionID {
				// New session started.
				_, _ = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
					AgentID:      agentData.ID,
					ActivityType: "session_start",
					Description:  fmt.Sprintf("Started session %s", req.SessionID),
					CreatedAt:    time.Now().Unix(),
				})
			}
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
		displayName := formatAgentDisplayName(
			aws.Agent.Name,
			aws.Agent.ProjectKey.String,
			aws.Agent.GitBranch.String,
		)
		result[i] = AgentView{
			ID:             fmt.Sprintf("%d", aws.Agent.ID),
			Name:           aws.Agent.Name,
			DisplayName:    displayName,
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

	// Re-render the thread view with the new message included.
	s.renderThreadView(ctx, w, threadID, true)
}

// renderThreadView renders the thread-view partial with optional success toast.
func (s *Server) renderThreadView(ctx context.Context, w http.ResponseWriter, threadID string, showSuccess bool) {
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
			senderName = formatAgentDisplayName(
					sender.Name, sender.ProjectKey.String, sender.GitBranch.String,
				)
		}

		// Get avatar initial.
		avatar := "?"
		if len(senderName) > 0 {
			avatar = strings.ToUpper(string(senderName[0]))
		}

		// Get recipient name(s) for this message.
		recipientName := ""
		recipients, err := s.store.Queries().GetMessageRecipients(ctx, m.ID)
		if err == nil && len(recipients) > 0 {
			// Get the first recipient's name.
			recipient, err := s.store.Queries().GetAgent(ctx, recipients[0].AgentID)
			if err == nil {
				recipientName = formatAgentDisplayName(
					recipient.Name, recipient.ProjectKey.String, recipient.GitBranch.String,
				)
			}
		}

		threadMessages[i] = ThreadMessage{
			ID:            fmt.Sprintf("%d", m.ID),
			SenderName:    senderName,
			SenderAvatar:  avatar,
			RecipientName: recipientName,
			Body:          m.BodyMd,
			State:         "read", // TODO: Get actual state from recipient.
			CreatedAt:     time.Unix(m.CreatedAt, 0),
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
		ShowSuccess:   showSuccess,
	}

	// Set HX-Trigger to show success toast if applicable.
	if showSuccess {
		w.Header().Set("HX-Trigger", `{"showToast": "Reply sent successfully!"}`)
	}

	s.renderPartial(w, "thread-view", thread)
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

	// Record activity for the message.
	_, _ = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      senderID,
		ActivityType: "message",
		Description:  fmt.Sprintf("Sent message \"%s\" to %s", subject, to),
		CreatedAt:    time.Now().Unix(),
	})

	// Return success with HX-Trigger to close modal and refresh.
	w.Header().Set("HX-Trigger", "messageSent")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="p-4 bg-green-50 text-green-700 rounded-lg text-center">
		<p class="font-medium">Message sent successfully!</p>
		<p class="text-sm mt-1">Your message to ` + to + ` has been delivered.</p>
	</div>`))
}

// handleMessageAction handles message actions like star, archive, snooze, trash.
// Routes: POST /api/messages/{message_id}/{action}?agent_id={agent_id}
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

	// Get agent ID from query parameter, form data, or fall back to message
	// recipient lookup.
	var agentID int64
	agentIDStr := r.URL.Query().Get("agent_id")
	if agentIDStr == "" {
		agentIDStr = r.FormValue("agent_id")
	}

	if agentIDStr != "" && agentIDStr != "all" && agentIDStr != "0" {
		fmt.Sscanf(agentIDStr, "%d", &agentID)
	}

	// If no agent ID specified, look up the recipient of this message.
	if agentID == 0 {
		recipients, err := s.store.Queries().GetMessageRecipients(
			ctx, messageID,
		)
		if err == nil && len(recipients) > 0 {
			agentID = recipients[0].AgentID
		}
	}

	// Final fallback: use first agent.
	if agentID == 0 {
		agents, err := s.store.Queries().ListAgents(ctx)
		if err != nil || len(agents) == 0 {
			http.Error(w, "No agents found", http.StatusInternalServerError)
			return
		}
		agentID = agents[0].ID
	}

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

	_, err = s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
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

	_, err := s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
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
// For sent messages (where the agent is the sender), marks deleted_by_sender.
// For received messages (where the agent is a recipient), updates recipient state.
func (s *Server) handleMessageTrash(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	messageID, agentID int64,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the message to check sender.
	msg, err := s.store.Queries().GetMessage(ctx, messageID)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Check the Referer to see if we're on the Sent page.
	referer := r.Header.Get("Referer")
	onSentPage := strings.Contains(referer, "/sent")

	// If on Sent page OR if the agentID matches the sender, delete from sender's perspective.
	if onSentPage || msg.SenderID == agentID {
		err = s.store.Queries().MarkMessageDeletedBySender(ctx, sqlc.MarkMessageDeletedBySenderParams{
			ID:       messageID,
			SenderID: msg.SenderID,
		})
		if err != nil {
			http.Error(w, "Failed to delete sent message", http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Trigger", "messageTrashed")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	// Otherwise, update recipient state.
	_, err = s.store.Queries().UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
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
