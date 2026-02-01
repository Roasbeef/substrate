// Package web provides the HTTP server and handlers for the Subtrate web UI.
package web

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
)

// NotificationHubRef is the interface for interacting with the notification hub.
// This abstracts the actor reference to allow for testing with mocks.
type NotificationHubRef interface {
	// Subscribe registers for notifications for a specific agent.
	Subscribe(ctx context.Context, agentID int64, subscriberID string, ch chan<- mail.InboxMessage) error

	// Unsubscribe removes a notification subscription.
	Unsubscribe(ctx context.Context, agentID int64, subscriberID string) error
}

// HubNotificationBridge connects the WebSocket hub to the actor notification system.
// It subscribes to notifications for connected agents and forwards them to WebSocket clients.
type HubNotificationBridge struct {
	hub           *Hub
	notifHub      NotificationHubRef
	deliveryChan  chan mail.InboxMessage
	subscriptions map[int64]bool // Track which agents we're subscribed to.
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewHubNotificationBridge creates a new bridge between the WebSocket hub and notifications.
func NewHubNotificationBridge(hub *Hub, notifHub NotificationHubRef) *HubNotificationBridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &HubNotificationBridge{
		hub:           hub,
		notifHub:      notifHub,
		deliveryChan:  make(chan mail.InboxMessage, 256),
		subscriptions: make(map[int64]bool),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the notification bridge, listening for notifications and forwarding them.
func (b *HubNotificationBridge) Start() {
	go b.runNotificationLoop()
	go b.runSubscriptionManager()
}

// Stop shuts down the notification bridge.
func (b *HubNotificationBridge) Stop() {
	b.cancel()
}

// runNotificationLoop receives notifications and forwards them to WebSocket clients.
func (b *HubNotificationBridge) runNotificationLoop() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case msg := <-b.deliveryChan:
			b.forwardNotification(msg)
		}
	}
}

// forwardNotification converts a mail notification to a WebSocket message and broadcasts it.
func (b *HubNotificationBridge) forwardNotification(msg mail.InboxMessage) {
	wsMsg := &WSMessage{
		Type: WSMsgTypeNewMessage,
		Payload: map[string]any{
			"id":          msg.ID,
			"sender_id":   msg.SenderID,
			"sender_name": msg.SenderName,
			"subject":     msg.Subject,
			"body":        msg.Body,
			"priority":    string(msg.Priority),
			"created_at":  msg.CreatedAt.UTC().Format(time.RFC3339),
			"thread_id":   msg.ThreadID,
			"state":       msg.State,
		},
	}

	// Broadcast to all recipients. In a real implementation, we'd track which
	// agent received which message, but for simplicity we broadcast based on
	// sender for now (the mail actor handles recipient filtering).
	// The SenderID is who sent it, we need to broadcast to recipients.
	// For now, broadcast to all connected clients and let them filter.
	b.hub.BroadcastToAll(wsMsg)
}

// runSubscriptionManager periodically updates subscriptions based on connected clients.
func (b *HubNotificationBridge) runSubscriptionManager() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			// Unsubscribe from all on shutdown.
			b.unsubscribeAll()
			return
		case <-ticker.C:
			b.syncSubscriptions()
		}
	}
}

// syncSubscriptions updates subscriptions to match connected clients.
func (b *HubNotificationBridge) syncSubscriptions() {
	if b.notifHub == nil {
		return
	}

	b.hub.mu.RLock()
	connectedAgents := make(map[int64]bool)
	for agentID := range b.hub.clients {
		connectedAgents[agentID] = true
	}
	b.hub.mu.RUnlock()

	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	// Subscribe to new agents.
	for agentID := range connectedAgents {
		if !b.subscriptions[agentID] {
			subscriberID := fmt.Sprintf("ws-hub-%d", agentID)
			err := b.notifHub.Subscribe(ctx, agentID, subscriberID, b.deliveryChan)
			if err != nil {
				log.Printf("WebSocket: Failed to subscribe for agent %d: %v", agentID, err)
				continue
			}
			b.subscriptions[agentID] = true
			log.Printf("WebSocket: Subscribed to notifications for agent %d", agentID)
		}
	}

	// Unsubscribe from disconnected agents.
	for agentID := range b.subscriptions {
		if !connectedAgents[agentID] {
			subscriberID := fmt.Sprintf("ws-hub-%d", agentID)
			err := b.notifHub.Unsubscribe(ctx, agentID, subscriberID)
			if err != nil {
				log.Printf("WebSocket: Failed to unsubscribe for agent %d: %v", agentID, err)
			}
			delete(b.subscriptions, agentID)
			log.Printf("WebSocket: Unsubscribed from notifications for agent %d", agentID)
		}
	}
}

// unsubscribeAll removes all subscriptions during shutdown.
func (b *HubNotificationBridge) unsubscribeAll() {
	if b.notifHub == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for agentID := range b.subscriptions {
		subscriberID := fmt.Sprintf("ws-hub-%d", agentID)
		_ = b.notifHub.Unsubscribe(ctx, agentID, subscriberID)
	}
	b.subscriptions = make(map[int64]bool)
}

// ActorNotificationHubRef wraps an actor reference to implement NotificationHubRef.
type ActorNotificationHubRef struct {
	ref mail.NotificationActorRef
}

// NewActorNotificationHubRef creates a NotificationHubRef from an actor reference.
func NewActorNotificationHubRef(ref mail.NotificationActorRef) *ActorNotificationHubRef {
	return &ActorNotificationHubRef{ref: ref}
}

// Subscribe implements NotificationHubRef.
func (a *ActorNotificationHubRef) Subscribe(
	ctx context.Context, agentID int64, subscriberID string, ch chan<- mail.InboxMessage,
) error {
	resp := a.ref.Ask(ctx, mail.SubscribeAgentMsg{
		AgentID:      agentID,
		SubscriberID: subscriberID,
		DeliveryChan: ch,
	})

	result := resp.Await(ctx)
	_, err := result.Unpack()
	return err
}

// Unsubscribe implements NotificationHubRef.
func (a *ActorNotificationHubRef) Unsubscribe(
	ctx context.Context, agentID int64, subscriberID string,
) error {
	resp := a.ref.Ask(ctx, mail.UnsubscribeAgentMsg{
		AgentID:      agentID,
		SubscriberID: subscriberID,
	})

	result := resp.Await(ctx)
	_, err := result.Unpack()
	return err
}
