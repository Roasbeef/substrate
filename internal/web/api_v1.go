// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// APIResponse wraps API responses with data and optional metadata.
type APIResponse struct {
	Data any      `json:"data"`
	Meta *APIMeta `json:"meta,omitempty"`
}

// APIMeta contains pagination metadata.
type APIMeta struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// APIError represents an API error response.
type APIError struct {
	Error APIErrorDetail `json:"error"`
}

// APIErrorDetail contains error details.
type APIErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// registerAPIV1Routes registers all /api/v1/ routes.
func (s *Server) registerAPIV1Routes() {
	// CORS middleware for API routes.
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from Vite dev server.
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next(w, r)
		}
	}

	// JSON middleware for API routes.
	jsonMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next(w, r)
		}
	}

	// Combine middlewares.
	api := func(handler http.HandlerFunc) http.HandlerFunc {
		return corsMiddleware(jsonMiddleware(handler))
	}

	// Health check.
	s.mux.HandleFunc("/api/v1/health", api(s.handleAPIV1Health))

	// Agents.
	s.mux.HandleFunc("/api/v1/agents", api(s.handleAPIV1Agents))
	s.mux.HandleFunc("/api/v1/agents/status", api(s.handleAPIV1AgentsStatusJSON))
	s.mux.HandleFunc("/api/v1/agents/heartbeat", api(s.handleAPIV1AgentHeartbeat))
	s.mux.HandleFunc("/api/v1/agents/", api(s.handleAPIV1AgentByID))

	// Stats.
	s.mux.HandleFunc("/api/v1/stats/dashboard", api(s.handleAPIV1DashboardStats))

	// Messages.
	s.mux.HandleFunc("/api/v1/messages", api(s.handleAPIV1Messages))
	s.mux.HandleFunc("/api/v1/messages/", api(s.handleAPIV1MessageByID))

	// Heartbeat.
	s.mux.HandleFunc("/api/v1/heartbeat", api(s.handleAPIV1Heartbeat))

	// Autocomplete.
	s.mux.HandleFunc("/api/v1/autocomplete/recipients", api(s.handleAPIV1AutocompleteRecipients))

	// Topics.
	s.mux.HandleFunc("/api/v1/topics", api(s.handleAPIV1Topics))
	s.mux.HandleFunc("/api/v1/topics/", api(s.handleAPIV1TopicByID))

	// Sessions.
	s.mux.HandleFunc("/api/v1/sessions", api(s.handleAPIV1Sessions))
	s.mux.HandleFunc("/api/v1/sessions/active", api(s.handleAPIV1ActiveSessions))
	s.mux.HandleFunc("/api/v1/sessions/", api(s.handleAPIV1SessionByID))

	// Activities.
	s.mux.HandleFunc("/api/v1/activities", api(s.handleAPIV1Activities))

	// Search.
	s.mux.HandleFunc("/api/v1/search", api(s.handleAPIV1Search))
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIError{
		Error: APIErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// handleAPIV1Health handles GET /api/v1/health.
func (s *Server) handleAPIV1Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// handleAPIV1AgentsStatusJSON handles GET /api/v1/agents/status (JSON version).
func (s *Server) handleAPIV1AgentsStatusJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch agent status")
		return
	}

	counts, err := s.heartbeatMgr.GetStatusCounts(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to get status counts")
		return
	}

	response := make([]map[string]any, 0, len(agents))
	for _, aws := range agents {
		response = append(response, map[string]any{
			"id":                      aws.Agent.ID,
			"name":                    aws.Agent.Name,
			"status":                  string(aws.Status),
			"last_active_at":          aws.LastActive.UTC().Format(time.RFC3339),
			"session_id":              aws.ActiveSessionID,
			"seconds_since_heartbeat": int(time.Since(aws.LastActive).Seconds()),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agents": response,
		"counts": countsToMap(counts),
	})
}

// countsToMap converts StatusCounts to a map.
func countsToMap(counts *agent.StatusCounts) map[string]int {
	if counts == nil {
		return map[string]int{}
	}
	return map[string]int{
		"active":  counts.Active,
		"busy":    counts.Busy,
		"idle":    counts.Idle,
		"offline": counts.Offline,
	}
}

// APIV1Agent represents an agent in the JSON API.
type APIV1Agent struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at,omitempty"`
}

// handleAPIV1Agents handles GET/POST /api/v1/agents.
func (s *Server) handleAPIV1Agents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// List all agents.
		agents, err := s.store.ListAgents(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch agents")
			return
		}

		result := make([]APIV1Agent, 0, len(agents))
		for _, a := range agents {
			result = append(result, APIV1Agent{
				ID:        a.ID,
				Name:      a.Name,
				CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
			})
		}

		writeJSON(w, http.StatusOK, APIResponse{
			Data: result,
			Meta: &APIMeta{
				Total:    len(result),
				Page:     1,
				PageSize: len(result),
			},
		})

	case http.MethodPost:
		// Register a new agent.
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
			return
		}

		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "Agent name is required")
			return
		}

		// Create the agent.
		ag, err := s.store.CreateAgent(ctx, store.CreateAgentParams{
			Name: req.Name,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to create agent")
			return
		}

		writeJSON(w, http.StatusCreated, APIV1Agent{
			ID:        ag.ID,
			Name:      ag.Name,
			CreatedAt: ag.CreatedAt.UTC().Format(time.RFC3339),
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAPIV1AgentByID handles GET/PATCH /api/v1/agents/{id}.
func (s *Server) handleAPIV1AgentByID(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from path.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "Agent ID required")
		return
	}

	// Handle status and heartbeat routes separately.
	if parts[0] == "status" || parts[0] == "heartbeat" {
		return
	}

	agentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid agent ID")
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Get agent details.
		ag, err := s.store.GetAgent(ctx, agentID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}

		writeJSON(w, http.StatusOK, APIV1Agent{
			ID:        ag.ID,
			Name:      ag.Name,
			CreatedAt: ag.CreatedAt.UTC().Format(time.RFC3339),
		})

	case http.MethodPatch:
		// Update agent.
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
			return
		}

		// Verify agent exists.
		_, err := s.store.GetAgent(ctx, agentID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Agent not found")
			return
		}

		// Update the agent name.
		if req.Name != "" {
			err = s.store.UpdateAgentName(ctx, agentID, req.Name)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", "Failed to update agent")
				return
			}
		}

		// Fetch updated agent.
		ag, _ := s.store.GetAgent(ctx, agentID)

		writeJSON(w, http.StatusOK, APIV1Agent{
			ID:        ag.ID,
			Name:      ag.Name,
			CreatedAt: ag.CreatedAt.UTC().Format(time.RFC3339),
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAPIV1AgentHeartbeat handles POST /api/v1/agents/heartbeat.
func (s *Server) handleAPIV1AgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	var req struct {
		AgentName string `json:"agent_name"`
		SessionID string `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.AgentName == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Agent name is required")
		return
	}

	ctx := r.Context()

	// Find or create agent by name.
	ag, err := s.store.GetAgentByName(ctx, req.AgentName)
	if err != nil {
		// Agent doesn't exist, create it.
		ag, err = s.store.CreateAgent(ctx, store.CreateAgentParams{
			Name: req.AgentName,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to create agent")
			return
		}
	}
	agentID := ag.ID

	// Record heartbeat.
	_ = s.heartbeatMgr.RecordHeartbeat(ctx, agentID)
	if req.SessionID != "" {
		s.heartbeatMgr.StartSession(agentID, req.SessionID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAPIV1DashboardStats handles GET /api/v1/stats/dashboard.
func (s *Server) handleAPIV1DashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	counts, _ := s.heartbeatMgr.GetStatusCounts(ctx)

	activeAgents := 0
	if counts != nil {
		activeAgents = counts.Active + counts.Busy
	}

	writeJSON(w, http.StatusOK, APIResponse{Data: map[string]int{
		"active_agents":    activeAgents,
		"running_sessions": 0,
		"pending_messages": 0,
		"completed_today":  0,
	}})
}

// APIV1Message represents a message in the JSON API.
type APIV1Message struct {
	ID         int64            `json:"id"`
	SenderID   int64            `json:"sender_id"`
	SenderName string           `json:"sender_name"`
	Subject    string           `json:"subject"`
	Body       string           `json:"body"`
	Priority   string           `json:"priority"`
	CreatedAt  string           `json:"created_at"`
	ThreadID   string           `json:"thread_id,omitempty"`
	Recipients []APIV1Recipient `json:"recipients,omitempty"`
}

// APIV1Recipient represents a message recipient in the JSON API.
type APIV1Recipient struct {
	MessageID    int64  `json:"message_id"`
	AgentID      int64  `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	State        string `json:"state"`
	SnoozedUntil string `json:"snoozed_until,omitempty"`
	ReadAt       string `json:"read_at,omitempty"`
	AckedAt      string `json:"acknowledged_at,omitempty"`
}

// handleAPIV1Messages handles GET/POST /api/v1/messages.
func (s *Server) handleAPIV1Messages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()

	if r.Method == http.MethodPost {
		// Handle send message.
		var req struct {
			To       []int64 `json:"to"`
			Subject  string  `json:"subject"`
			Body     string  `json:"body"`
			Priority string  `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
			return
		}

		priority := req.Priority
		if priority == "" {
			priority = "normal"
		}

		// Convert recipient IDs to names for actor system.
		recipientNames := make([]string, 0, len(req.To))
		for _, recipientID := range req.To {
			ag, err := s.store.GetAgent(ctx, recipientID)
			if err == nil {
				recipientNames = append(recipientNames, ag.Name)
			}
		}

		// Use actor system to send message.
		if s.actorRefs.HasMailActor() {
			resp, err := s.sendMail(ctx, mail.SendMailRequest{
				SenderID:       s.getUserAgentID(ctx),
				RecipientNames: recipientNames,
				Subject:        req.Subject,
				Body:           req.Body,
				Priority:       mail.Priority(priority),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "mail_error", "Failed to send message")
				return
			}

			writeJSON(w, http.StatusCreated, APIV1Message{
				ID:        resp.MessageID,
				SenderID:  s.getUserAgentID(ctx),
				Subject:   req.Subject,
				Body:      req.Body,
				Priority:  priority,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
				ThreadID:  resp.ThreadID,
			})
			return
		}

		// Fallback to direct store access.
		msg, err := s.store.CreateMessage(ctx, store.CreateMessageParams{
			SenderID: s.getUserAgentID(ctx),
			Subject:  req.Subject,
			Body:     req.Body,
			Priority: priority,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to create message")
			return
		}

		// Add recipients.
		for _, recipientID := range req.To {
			_ = s.store.CreateMessageRecipient(ctx, msg.ID, recipientID)
		}

		writeJSON(w, http.StatusCreated, APIV1Message{
			ID:        msg.ID,
			SenderID:  msg.SenderID,
			Subject:   msg.Subject,
			Body:      msg.Body,
			Priority:  msg.Priority,
			CreatedAt: msg.CreatedAt.UTC().Format(time.RFC3339),
		})
		return
	}

	// GET messages - fetch inbox messages.
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	messages, err := s.store.GetAllInboxMessages(ctx, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch messages")
		return
	}

	// Collect message IDs for bulk recipient fetch.
	messageIDs := make([]int64, 0, len(messages))
	for _, m := range messages {
		messageIDs = append(messageIDs, m.ID)
	}

	// Bulk fetch all recipients for all messages (single query instead of N+1).
	recipientsByMsg, _ := s.store.GetMessageRecipientsBulk(ctx, messageIDs)

	result := make([]APIV1Message, 0, len(messages))
	for _, m := range messages {
		apiMsg := APIV1Message{
			ID:         m.ID,
			SenderID:   m.SenderID,
			SenderName: m.SenderName,
			Subject:    m.Subject,
			Body:       m.Body,
			Priority:   m.Priority,
			CreatedAt:  m.CreatedAt.UTC().Format(time.RFC3339),
			ThreadID:   m.ThreadID,
		}

		// Use pre-fetched recipients from bulk query.
		for _, rec := range recipientsByMsg[m.ID] {
			apiRec := APIV1Recipient{
				MessageID: rec.MessageID,
				AgentID:   rec.AgentID,
				AgentName: rec.AgentName,
				State:     rec.State,
			}
			if rec.SnoozedUntil != nil {
				apiRec.SnoozedUntil = rec.SnoozedUntil.UTC().Format(time.RFC3339)
			}
			if rec.ReadAt != nil {
				apiRec.ReadAt = rec.ReadAt.UTC().Format(time.RFC3339)
			}
			if rec.AckedAt != nil {
				apiRec.AckedAt = rec.AckedAt.UTC().Format(time.RFC3339)
			}
			apiMsg.Recipients = append(apiMsg.Recipients, apiRec)
		}

		result = append(result, apiMsg)
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Data: result,
		Meta: &APIMeta{
			Total:    len(result),
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// handleAPIV1MessageByID handles GET/POST /api/v1/messages/{id} and message actions.
func (s *Server) handleAPIV1MessageByID(w http.ResponseWriter, r *http.Request) {
	// Extract message ID and optional action from path.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/messages/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "Message ID required")
		return
	}

	msgID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid message ID")
		return
	}

	// Check for action in path (e.g., /api/v1/messages/123/star).
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	ctx := r.Context()

	// Handle message actions via POST.
	if r.Method == http.MethodPost && action != "" {
		s.handleMessageAction(w, r, msgID, action)
		return
	}

	if r.Method == http.MethodGet {
		// Use actor system to read message (marks as read).
		if s.actorRefs.HasMailActor() {
			agentID := s.getUserAgentID(ctx)
			resp, err := s.readMessage(ctx, agentID, msgID)
			if err != nil || resp.Error != nil {
				writeError(w, http.StatusNotFound, "not_found", "Message not found")
				return
			}

			writeJSON(w, http.StatusOK, APIV1Message{
				ID:         resp.Message.ID,
				SenderID:   resp.Message.SenderID,
				SenderName: resp.Message.SenderName,
				Subject:    resp.Message.Subject,
				Body:       resp.Message.Body,
				Priority:   string(resp.Message.Priority),
				CreatedAt:  resp.Message.CreatedAt.UTC().Format(time.RFC3339),
				ThreadID:   resp.Message.ThreadID,
			})
			return
		}

		// Fallback to direct store access.
		msg, err := s.store.GetMessage(ctx, msgID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "Message not found")
			return
		}

		sender, _ := s.store.GetAgent(ctx, msg.SenderID)

		apiMsg := APIV1Message{
			ID:         msg.ID,
			SenderID:   msg.SenderID,
			SenderName: sender.Name,
			Subject:    msg.Subject,
			Body:       msg.Body,
			Priority:   msg.Priority,
			CreatedAt:  msg.CreatedAt.UTC().Format(time.RFC3339),
			ThreadID:   msg.ThreadID,
		}

		// Fetch recipients.
		recipients, _ := s.store.GetMessageRecipients(ctx, msg.ID)
		for _, rec := range recipients {
			apiRec := APIV1Recipient{
				MessageID: rec.MessageID,
				AgentID:   rec.AgentID,
				AgentName: rec.AgentName,
				State:     rec.State,
			}
			apiMsg.Recipients = append(apiMsg.Recipients, apiRec)
		}

		writeJSON(w, http.StatusOK, apiMsg)
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
}

// handleMessageAction handles POST /api/v1/messages/{id}/{action}.
func (s *Server) handleMessageAction(w http.ResponseWriter, r *http.Request, msgID int64, action string) {
	ctx := r.Context()
	agentID := s.getUserAgentID(ctx)

	// Parse optional request body for snooze duration.
	var req struct {
		SnoozedUntil string `json:"snoozed_until,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	switch action {
	case "star":
		if s.actorRefs.HasMailActor() {
			_, err := s.updateMessageState(ctx, agentID, msgID, "starred", nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to star message")
				return
			}
		} else {
			err := s.store.UpdateRecipientState(ctx, msgID, agentID, "starred")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to star message")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "starred"})

	case "archive":
		if s.actorRefs.HasMailActor() {
			_, err := s.updateMessageState(ctx, agentID, msgID, "archived", nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to archive message")
				return
			}
		} else {
			err := s.store.UpdateRecipientState(ctx, msgID, agentID, "archived")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to archive message")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})

	case "snooze":
		var snoozedUntil *time.Time
		if req.SnoozedUntil != "" {
			t, err := time.Parse(time.RFC3339, req.SnoozedUntil)
			if err == nil {
				snoozedUntil = &t
			}
		}
		if snoozedUntil == nil {
			// Default snooze for 1 hour.
			t := time.Now().Add(time.Hour)
			snoozedUntil = &t
		}

		if s.actorRefs.HasMailActor() {
			_, err := s.updateMessageState(ctx, agentID, msgID, "snoozed", snoozedUntil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to snooze message")
				return
			}
		} else {
			_ = s.store.SnoozeMessage(ctx, msgID, agentID, *snoozedUntil)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "snoozed",
			"snoozed_until": snoozedUntil.UTC().Format(time.RFC3339),
		})

	case "ack":
		if s.actorRefs.HasMailActor() {
			_, err := s.ackMessage(ctx, agentID, msgID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to ack message")
				return
			}
		} else {
			_ = s.store.AckMessage(ctx, msgID, agentID)
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})

	case "read":
		if s.actorRefs.HasMailActor() {
			_, err := s.readMessage(ctx, agentID, msgID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to mark message as read")
				return
			}
		} else {
			_ = s.store.MarkMessageRead(ctx, msgID, agentID)
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "read"})

	case "unread":
		// Mark as unread (reset state).
		if s.actorRefs.HasMailActor() {
			_, err := s.updateMessageState(ctx, agentID, msgID, "unread", nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "action_failed", "Failed to mark message as unread")
				return
			}
		} else {
			_ = s.store.UpdateRecipientState(ctx, msgID, agentID, "unread")
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "unread"})

	default:
		writeError(w, http.StatusBadRequest, "invalid_action", "Unknown action: "+action)
	}
}

// handleAPIV1Heartbeat handles POST /api/v1/heartbeat.
func (s *Server) handleAPIV1Heartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	var req struct {
		AgentID   int64  `json:"agent_id"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body for simple heartbeat.
		req.AgentID = s.getUserAgentID(r.Context())
	}

	// Record heartbeat.
	if req.AgentID > 0 {
		_ = s.heartbeatMgr.RecordHeartbeat(r.Context(), req.AgentID)
		if req.SessionID != "" {
			s.heartbeatMgr.StartSession(req.AgentID, req.SessionID)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAPIV1AutocompleteRecipients handles GET /api/v1/autocomplete/recipients.
func (s *Server) handleAPIV1AutocompleteRecipients(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	query := r.URL.Query().Get("q")
	ctx := r.Context()

	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch agents")
		return
	}

	result := make([]map[string]any, 0)
	queryLower := strings.ToLower(query)
	for _, a := range agents {
		if query == "" || strings.Contains(strings.ToLower(a.Name), queryLower) {
			result = append(result, map[string]any{
				"id":   a.ID,
				"name": a.Name,
			})
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// APIV1Topic represents a topic in the JSON API.
type APIV1Topic struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	MessageCount int    `json:"message_count"`
	CreatedAt    string `json:"created_at"`
}

// handleAPIV1Topics handles GET /api/v1/topics.
func (s *Server) handleAPIV1Topics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	topics, err := s.store.ListTopics(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch topics")
		return
	}

	result := make([]APIV1Topic, 0, len(topics))
	for _, t := range topics {
		result = append(result, APIV1Topic{
			ID:           t.ID,
			Name:         t.Name,
			MessageCount: 0,
			CreatedAt:    t.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Data: result,
		Meta: &APIMeta{
			Total:    len(result),
			Page:     1,
			PageSize: len(result),
		},
	})
}

// handleAPIV1TopicByID handles GET /api/v1/topics/{id}/messages.
func (s *Server) handleAPIV1TopicByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	// Extract topic ID from path.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/topics/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "Topic ID required")
		return
	}

	topicID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid topic ID")
		return
	}

	ctx := r.Context()

	// Get topic.
	topic, err := s.store.GetTopic(ctx, topicID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Topic not found")
		return
	}

	// Check for /messages subpath.
	if len(parts) > 1 && parts[1] == "messages" {
		// Return messages in topic using existing query.
		messages, _ := s.store.GetMessagesByTopic(ctx, topicID)
		result := make([]APIV1Message, 0, len(messages))
		for _, m := range messages {
			sender, _ := s.store.GetAgent(ctx, m.SenderID)
			result = append(result, APIV1Message{
				ID:         m.ID,
				SenderID:   m.SenderID,
				SenderName: sender.Name,
				Subject:    m.Subject,
				Body:       m.Body,
				Priority:   m.Priority,
				CreatedAt:  m.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		writeJSON(w, http.StatusOK, APIResponse{Data: result})
		return
	}

	// Return topic details.
	writeJSON(w, http.StatusOK, APIV1Topic{
		ID:        topic.ID,
		Name:      topic.Name,
		CreatedAt: topic.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// APIV1Session represents a session in the JSON API.
// Note: The database uses session_identities table which maps Claude sessions to agents.
// This API provides a simplified view of active agent sessions.
type APIV1Session struct {
	ID          int64  `json:"id"`
	AgentID     int64  `json:"agent_id"`
	AgentName   string `json:"agent_name,omitempty"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// handleAPIV1Sessions handles GET/POST /api/v1/sessions.
func (s *Server) handleAPIV1Sessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return sessions based on heartbeat manager's active sessions.
		ctx := r.Context()
		agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch sessions")
			return
		}

		result := make([]APIV1Session, 0)
		for _, aws := range agents {
			if aws.ActiveSessionID != "" {
				result = append(result, APIV1Session{
					ID:        aws.Agent.ID,
					AgentID:   aws.Agent.ID,
					AgentName: aws.Agent.Name,
					Status:    "active",
					StartedAt: aws.LastActive.UTC().Format(time.RFC3339),
				})
			}
		}

		writeJSON(w, http.StatusOK, APIResponse{
			Data: result,
			Meta: &APIMeta{
				Total:    len(result),
				Page:     1,
				PageSize: len(result),
			},
		})

	case http.MethodPost:
		// Start a new session via heartbeat manager.
		var req struct {
			AgentID   int64  `json:"agent_id"`
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
			return
		}

		if req.SessionID == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "Session ID is required")
			return
		}

		s.heartbeatMgr.StartSession(req.AgentID, req.SessionID)

		writeJSON(w, http.StatusCreated, map[string]any{
			"session_id": req.SessionID,
			"agent_id":   req.AgentID,
			"status":     "active",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAPIV1ActiveSessions handles GET /api/v1/sessions/active.
func (s *Server) handleAPIV1ActiveSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()
	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch sessions")
		return
	}

	result := make([]APIV1Session, 0)
	for _, aws := range agents {
		if aws.ActiveSessionID != "" && (aws.Status == "active" || aws.Status == "busy") {
			result = append(result, APIV1Session{
				ID:        aws.Agent.ID,
				AgentID:   aws.Agent.ID,
				AgentName: aws.Agent.Name,
				Status:    string(aws.Status),
				StartedAt: aws.LastActive.UTC().Format(time.RFC3339),
			})
		}
	}

	writeJSON(w, http.StatusOK, APIResponse{Data: result})
}

// handleAPIV1SessionByID handles GET/POST /api/v1/sessions/{id}.
func (s *Server) handleAPIV1SessionByID(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "Session ID required")
		return
	}

	// Handle /active route separately.
	if parts[0] == "active" {
		return
	}

	sessionID := parts[0]

	ctx := r.Context()

	// Check for /complete subpath.
	if len(parts) > 1 && parts[1] == "complete" && r.Method == http.MethodPost {
		// Find agent by session ID and end the session.
		agents, _ := s.heartbeatMgr.ListAgentsWithStatus(ctx)
		for _, aws := range agents {
			if aws.ActiveSessionID == sessionID {
				s.heartbeatMgr.EndSession(aws.Agent.ID)
				break
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodGet {
		// Look up session in heartbeat manager.
		agents, _ := s.heartbeatMgr.ListAgentsWithStatus(ctx)

		for _, aws := range agents {
			if aws.ActiveSessionID == sessionID {
				writeJSON(w, http.StatusOK, APIV1Session{
					ID:        aws.Agent.ID,
					AgentID:   aws.Agent.ID,
					AgentName: aws.Agent.Name,
					Status:    string(aws.Status),
					StartedAt: aws.LastActive.UTC().Format(time.RFC3339),
				})
				return
			}
		}

		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
}

// APIV1Activity represents an activity in the JSON API.
type APIV1Activity struct {
	ID          int64  `json:"id"`
	AgentID     int64  `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// handleAPIV1Activities handles GET /api/v1/activities.
func (s *Server) handleAPIV1Activities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	ctx := r.Context()

	// Get pagination params.
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Use actor system for activities if available.
	if s.actorRefs.HasActivityActor() {
		activityItems, err := s.listRecentActivities(ctx, pageSize)
		if err == nil {
			result := make([]APIV1Activity, 0, len(activityItems))
			for _, a := range activityItems {
				result = append(result, APIV1Activity{
					ID:          a.ID,
					AgentID:     a.AgentID,
					AgentName:   a.AgentName,
					Type:        a.ActivityType,
					Description: a.Description,
					CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
				})
			}
			writeJSON(w, http.StatusOK, APIResponse{
				Data: result,
				Meta: &APIMeta{
					Total:    len(result),
					Page:     page,
					PageSize: pageSize,
				},
			})
			return
		}
		// Fall through to direct store access on error.
	}

	// Fallback to direct store access.
	activities, err := s.store.ListRecentActivities(ctx, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to fetch activities")
		return
	}

	result := make([]APIV1Activity, 0, len(activities))
	for _, a := range activities {
		agentName := ""
		if ag, err := s.store.GetAgent(ctx, a.AgentID); err == nil {
			agentName = ag.Name
		}
		result = append(result, APIV1Activity{
			ID:          a.ID,
			AgentID:     a.AgentID,
			AgentName:   agentName,
			Type:        a.ActivityType,
			Description: a.Description,
			CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Data: result,
		Meta: &APIMeta{
			Total:    len(result),
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// handleAPIV1Search handles GET /api/v1/search?q=.
func (s *Server) handleAPIV1Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, APIResponse{Data: []any{}})
		return
	}

	ctx := r.Context()

	// Search messages using the store interface.
	messages, err := s.store.SearchMessages(ctx, query, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "Failed to search")
		return
	}

	result := make([]APIV1Message, 0, len(messages))
	for _, m := range messages {
		result = append(result, APIV1Message{
			ID:         m.ID,
			SenderID:   m.SenderID,
			SenderName: m.SenderName,
			Subject:    m.Subject,
			Body:       m.Body,
			Priority:   m.Priority,
			CreatedAt:  m.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Data: result,
		Meta: &APIMeta{
			Total:    len(result),
			Page:     1,
			PageSize: len(result),
		},
	})
}
