// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket message types for real-time updates.
const (
	WSMsgTypeUnreadCount  = "unread_count"
	WSMsgTypeNewMessage   = "new_message"
	WSMsgTypeAgentUpdate  = "agent_update"
	WSMsgTypeActivity     = "activity"
	WSMsgTypePong         = "pong"
	WSMsgTypeConnected    = "connected"
	WSMsgTypeError        = "error"
)

// WSMessage represents a WebSocket message sent to clients.
type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Hub maintains the set of active WebSocket clients and broadcasts messages.
type Hub struct {
	// Registered clients keyed by agent ID.
	clients map[int64]map[*WSClient]struct{}

	// All clients regardless of agent ID.
	allClients map[*WSClient]struct{}

	// Register requests from clients.
	register chan *WSClient

	// Unregister requests from clients.
	unregister chan *WSClient

	// Broadcast messages to specific agent's clients.
	broadcast chan *agentBroadcast

	// Broadcast messages to all clients.
	broadcastAll chan *WSMessage

	// Server reference for data fetching.
	server *Server

	// Mutex for thread-safe access.
	mu sync.RWMutex

	// Context for shutdown.
	ctx    context.Context
	cancel context.CancelFunc
}

// agentBroadcast holds a message targeted at a specific agent.
type agentBroadcast struct {
	agentID int64
	message *WSMessage
}

// NewHub creates a new WebSocket hub.
func NewHub(server *Server) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:      make(map[int64]map[*WSClient]struct{}),
		allClients:   make(map[*WSClient]struct{}),
		register:     make(chan *WSClient),
		unregister:   make(chan *WSClient),
		broadcast:    make(chan *agentBroadcast, 256),
		broadcastAll: make(chan *WSMessage, 256),
		server:       server,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	// Start background ticker for periodic updates.
	go h.runPeriodicUpdates()

	for {
		select {
		case <-h.ctx.Done():
			// Clean up all clients on shutdown.
			h.mu.Lock()
			for client := range h.allClients {
				client.Close()
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			agentID := client.AgentID()
			h.mu.Lock()
			h.allClients[client] = struct{}{}
			if agentID > 0 {
				if h.clients[agentID] == nil {
					h.clients[agentID] = make(map[*WSClient]struct{})
				}
				h.clients[agentID][client] = struct{}{}
			}
			h.mu.Unlock()
			log.Printf("WebSocket: Client registered (agent_id=%d, total=%d)",
				agentID, len(h.allClients))

		case client := <-h.unregister:
			agentID := client.AgentID()
			h.mu.Lock()
			if _, ok := h.allClients[client]; ok {
				delete(h.allClients, client)
				if agentID > 0 {
					delete(h.clients[agentID], client)
					if len(h.clients[agentID]) == 0 {
						delete(h.clients, agentID)
					}
				}
				client.Close()
			}
			h.mu.Unlock()
			log.Printf("WebSocket: Client unregistered (agent_id=%d, total=%d)",
				agentID, len(h.allClients))

		case msg := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.clients[msg.agentID]; ok {
				for client := range clients {
					client.Send(msg.message)
				}
			}
			h.mu.RUnlock()

		case msg := <-h.broadcastAll:
			h.mu.RLock()
			for client := range h.allClients {
				client.Send(msg)
			}
			h.mu.RUnlock()
		}
	}
}

// runPeriodicUpdates sends periodic updates to all connected clients.
func (h *Hub) runPeriodicUpdates() {
	agentTicker := time.NewTicker(15 * time.Second)
	activityTicker := time.NewTicker(10 * time.Second)
	inboxTicker := time.NewTicker(5 * time.Second)

	defer agentTicker.Stop()
	defer activityTicker.Stop()
	defer inboxTicker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-agentTicker.C:
			h.broadcastAgentStatus()
		case <-activityTicker.C:
			h.broadcastActivity()
		case <-inboxTicker.C:
			h.broadcastUnreadCounts()
		}
	}
}

// broadcastAgentStatus sends agent status updates to all clients.
func (h *Hub) broadcastAgentStatus() {
	if h.server == nil {
		return
	}

	ctx := context.Background()
	agents, err := h.server.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		return
	}

	agentList := make([]map[string]any, 0, len(agents))
	for _, aws := range agents {
		agentList = append(agentList, map[string]any{
			"id":                      aws.Agent.ID,
			"name":                    aws.Agent.Name,
			"status":                  string(aws.Status),
			"last_active_at":          aws.LastActive.UTC().Format(time.RFC3339),
			"seconds_since_heartbeat": int(time.Since(aws.LastActive).Seconds()),
		})
	}

	counts, _ := h.server.heartbeatMgr.GetStatusCounts(ctx)

	h.BroadcastToAll(&WSMessage{
		Type: WSMsgTypeAgentUpdate,
		Payload: map[string]any{
			"agents": agentList,
			"counts": countsToMap(counts),
		},
	})
}

// broadcastActivity sends activity updates to all clients.
func (h *Hub) broadcastActivity() {
	if h.server == nil {
		return
	}

	ctx := context.Background()
	activities, err := h.server.store.Queries().ListRecentActivities(ctx, 10)
	if err != nil {
		return
	}

	activityList := make([]map[string]any, 0, len(activities))
	for _, a := range activities {
		// Get agent name.
		agentName := ""
		if agent, err := h.server.store.Queries().GetAgent(ctx, a.AgentID); err == nil {
			agentName = agent.Name
		}

		activityList = append(activityList, map[string]any{
			"id":          a.ID,
			"agent_id":    a.AgentID,
			"agent_name":  agentName,
			"type":        a.ActivityType,
			"description": a.Description,
			"created_at":  time.Unix(a.CreatedAt, 0).UTC().Format(time.RFC3339),
		})
	}

	h.BroadcastToAll(&WSMessage{
		Type:    WSMsgTypeActivity,
		Payload: activityList,
	})
}

// broadcastUnreadCounts sends unread counts to each connected agent.
func (h *Hub) broadcastUnreadCounts() {
	if h.server == nil {
		return
	}

	h.mu.RLock()
	agentIDs := make([]int64, 0, len(h.clients))
	for agentID := range h.clients {
		agentIDs = append(agentIDs, agentID)
	}
	h.mu.RUnlock()

	ctx := context.Background()
	for _, agentID := range agentIDs {
		count, err := h.server.store.Queries().CountUnreadByAgent(ctx, agentID)
		if err != nil {
			continue
		}

		h.BroadcastToAgent(agentID, &WSMessage{
			Type: WSMsgTypeUnreadCount,
			Payload: map[string]any{
				"count": count,
			},
		})
	}
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	h.cancel()
}

// BroadcastToAgent sends a message to all clients for a specific agent.
func (h *Hub) BroadcastToAgent(agentID int64, msg *WSMessage) {
	select {
	case h.broadcast <- &agentBroadcast{agentID: agentID, message: msg}:
	default:
		log.Printf("WebSocket: Broadcast buffer full, dropping message for agent %d", agentID)
	}
}

// BroadcastToAll sends a message to all connected clients.
func (h *Hub) BroadcastToAll(msg *WSMessage) {
	select {
	case h.broadcastAll <- msg:
	default:
		log.Printf("WebSocket: Broadcast buffer full, dropping message")
	}
}

// BroadcastNewMessage notifies clients of a new message.
func (h *Hub) BroadcastNewMessage(recipientID int64, msg map[string]any) {
	h.BroadcastToAgent(recipientID, &WSMessage{
		Type:    WSMsgTypeNewMessage,
		Payload: msg,
	})
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.allClients)
}

// upgrader specifies parameters for upgrading an HTTP connection to WebSocket.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Check origin to prevent CSRF attacks.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Allow if no origin header (same-origin requests).
		if origin == "" {
			return true
		}
		// Allow Vite dev server in development.
		if origin == "http://localhost:5174" {
			return true
		}
		// Allow same-origin requests.
		host := r.Host
		return origin == "http://"+host || origin == "https://"+host
	},
}

// handleWebSocket handles WebSocket connections at /ws.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		http.Error(w, "WebSocket not available", http.StatusServiceUnavailable)
		return
	}

	// Parse agent_id from query params.
	agentIDStr := r.URL.Query().Get("agent_id")
	var agentID int64
	if agentIDStr != "" {
		var err error
		agentID, err = strconv.ParseInt(agentIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent_id", http.StatusBadRequest)
			return
		}
	}

	// Upgrade HTTP connection to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Create new client.
	client := NewWSClient(s.hub, conn, agentID)

	// Register client with hub.
	s.hub.register <- client

	// Send connection confirmation.
	client.Send(&WSMessage{
		Type: WSMsgTypeConnected,
		Payload: map[string]any{
			"agent_id": agentID,
			"time":     time.Now().UTC().Format(time.RFC3339),
		},
	})

	// Start read and write pumps.
	go client.writePump()
	go client.readPump()
}

// handleIncomingMessage processes messages received from WebSocket clients.
func (h *Hub) handleIncomingMessage(client *WSClient, messageType int, data []byte) {
	if messageType != websocket.TextMessage {
		return
	}

	var msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		client.Send(&WSMessage{
			Type: WSMsgTypeError,
			Payload: map[string]any{
				"message": "Invalid message format",
			},
		})
		return
	}

	switch msg.Type {
	case "ping":
		// Respond to ping with pong.
		client.Send(&WSMessage{
			Type: WSMsgTypePong,
			Payload: map[string]any{
				"time": time.Now().UTC().Format(time.RFC3339),
			},
		})

	case "subscribe":
		// Handle subscription requests (agent_id change).
		var sub struct {
			AgentID int64 `json:"agent_id"`
		}
		if err := json.Unmarshal(msg.Data, &sub); err == nil && sub.AgentID > 0 {
			oldAgentID := client.AgentID()
			h.mu.Lock()
			// Remove from old agent.
			if oldAgentID > 0 {
				delete(h.clients[oldAgentID], client)
			}
			// Add to new agent.
			client.SetAgentID(sub.AgentID)
			if h.clients[sub.AgentID] == nil {
				h.clients[sub.AgentID] = make(map[*WSClient]struct{})
			}
			h.clients[sub.AgentID][client] = struct{}{}
			h.mu.Unlock()

			client.Send(&WSMessage{
				Type: "subscribed",
				Payload: map[string]any{
					"agent_id": sub.AgentID,
				},
			})
		}

	default:
		// Unknown message type.
		client.Send(&WSMessage{
			Type: WSMsgTypeError,
			Payload: map[string]any{
				"message": "Unknown message type: " + msg.Type,
			},
		})
	}
}
