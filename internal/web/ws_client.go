// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096

	// Size of the client send buffer.
	sendBufferSize = 256
)

// WSClient represents a single WebSocket connection.
type WSClient struct {
	// The WebSocket hub.
	hub *Hub

	// The WebSocket connection.
	conn *websocket.Conn

	// Agent ID associated with this connection.
	agentID int64

	// Buffered channel of outbound messages.
	send chan *WSMessage

	// Mutex for thread-safe connection operations.
	mu sync.Mutex

	// Closed flag.
	closed bool
}

// NewWSClient creates a new WebSocket client.
func NewWSClient(hub *Hub, conn *websocket.Conn, agentID int64) *WSClient {
	return &WSClient{
		hub:     hub,
		conn:    conn,
		agentID: agentID,
		send:    make(chan *WSMessage, sendBufferSize),
	}
}

// AgentID returns the agent ID associated with this client (thread-safe).
func (c *WSClient) AgentID() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.agentID
}

// SetAgentID sets the agent ID for this client (thread-safe).
func (c *WSClient) SetAgentID(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentID = id
}

// Send queues a message to be sent to the client.
func (c *WSClient) Send(msg *WSMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	select {
	case c.send <- msg:
	default:
		// Buffer full, drop message.
		log.Printf("WebSocket: Send buffer full for agent %d, dropping message", c.agentID)
	}
}

// Close closes the client connection.
func (c *WSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.send)
	c.conn.Close()
}

// readPump pumps messages from the WebSocket connection to the hub.
// It runs in a separate goroutine for each client.
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket: Read error for agent %d: %v", c.AgentID(), err)
			}
			return
		}

		// Process incoming message.
		c.hub.handleIncomingMessage(c, messageType, data)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
// It runs in a separate goroutine for each client.
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Marshal and send the message.
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("WebSocket: Marshal error: %v", err)
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("WebSocket: Write error for agent %d: %v", c.AgentID(), err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
