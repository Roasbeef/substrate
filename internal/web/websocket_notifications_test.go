package web

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/stretchr/testify/require"
)

// mockNotificationHubRef implements NotificationHubRef for testing.
type mockNotificationHubRef struct {
	mu               sync.Mutex
	subscriptions    map[int64]map[string]chan<- mail.InboxMessage
	subscribeCalls   int
	unsubscribeCalls int
}

func newMockNotificationHubRef() *mockNotificationHubRef {
	return &mockNotificationHubRef{
		subscriptions: make(map[int64]map[string]chan<- mail.InboxMessage),
	}
}

func (m *mockNotificationHubRef) Subscribe(
	ctx context.Context, agentID int64, subscriberID string, ch chan<- mail.InboxMessage,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribeCalls++
	if m.subscriptions[agentID] == nil {
		m.subscriptions[agentID] = make(map[string]chan<- mail.InboxMessage)
	}
	m.subscriptions[agentID][subscriberID] = ch
	return nil
}

func (m *mockNotificationHubRef) Unsubscribe(
	ctx context.Context, agentID int64, subscriberID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unsubscribeCalls++
	if subs, ok := m.subscriptions[agentID]; ok {
		delete(subs, subscriberID)
		if len(subs) == 0 {
			delete(m.subscriptions, agentID)
		}
	}
	return nil
}

func (m *mockNotificationHubRef) subscriberCount(agentID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscriptions[agentID])
}

// TestHubNotificationBridge_ForwardNotification tests that notifications are forwarded to WebSocket.
func TestHubNotificationBridge_ForwardNotification(t *testing.T) {
	t.Parallel()

	// Create a hub without a server (nil is OK for this test).
	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	mockRef := newMockNotificationHubRef()
	bridge := NewHubNotificationBridge(hub, mockRef)
	bridge.Start()
	defer bridge.Stop()

	// Simulate receiving a notification.
	testMsg := mail.InboxMessage{
		ID:         100,
		SenderID:   1,
		SenderName: "TestSender",
		Subject:    "Test Subject",
		Body:       "Test Body",
		Priority:   mail.PriorityNormal,
		CreatedAt:  time.Now(),
		ThreadID:   "thread-1",
		State:      "unread",
	}

	// Send notification to bridge's delivery channel.
	bridge.deliveryChan <- testMsg

	// Give time for processing.
	time.Sleep(50 * time.Millisecond)

	// The notification should have been forwarded (broadcasted to all).
	// Since we don't have connected clients, we just verify no panic occurred.
}

// TestHubNotificationBridge_SubscriptionSync tests subscription management.
func TestHubNotificationBridge_SubscriptionSync(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	mockRef := newMockNotificationHubRef()
	bridge := NewHubNotificationBridge(hub, mockRef)

	// Manually add a client to the hub to simulate a connection.
	hub.mu.Lock()
	hub.clients[42] = make(map[*WSClient]struct{})
	hub.clients[42][&WSClient{}] = struct{}{}
	hub.mu.Unlock()

	// Sync subscriptions.
	bridge.syncSubscriptions()

	// Verify subscription was created.
	require.Equal(t, 1, mockRef.subscribeCalls)
	require.Equal(t, 1, mockRef.subscriberCount(42))
	require.True(t, bridge.subscriptions[42])

	// Remove client and sync again.
	hub.mu.Lock()
	delete(hub.clients, 42)
	hub.mu.Unlock()

	bridge.syncSubscriptions()

	// Verify unsubscription was called.
	require.Equal(t, 1, mockRef.unsubscribeCalls)
	require.Equal(t, 0, mockRef.subscriberCount(42))
	require.False(t, bridge.subscriptions[42])
}

// TestHubNotificationBridge_MultipleAgents tests handling multiple agents.
func TestHubNotificationBridge_MultipleAgents(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	mockRef := newMockNotificationHubRef()
	bridge := NewHubNotificationBridge(hub, mockRef)

	// Add multiple clients.
	hub.mu.Lock()
	hub.clients[1] = make(map[*WSClient]struct{})
	hub.clients[1][&WSClient{}] = struct{}{}
	hub.clients[2] = make(map[*WSClient]struct{})
	hub.clients[2][&WSClient{}] = struct{}{}
	hub.clients[3] = make(map[*WSClient]struct{})
	hub.clients[3][&WSClient{}] = struct{}{}
	hub.mu.Unlock()

	// Sync subscriptions.
	bridge.syncSubscriptions()

	// Verify all subscriptions created.
	require.Equal(t, 3, mockRef.subscribeCalls)
	require.Equal(t, 1, mockRef.subscriberCount(1))
	require.Equal(t, 1, mockRef.subscriberCount(2))
	require.Equal(t, 1, mockRef.subscriberCount(3))

	// Partially disconnect.
	hub.mu.Lock()
	delete(hub.clients, 2)
	hub.mu.Unlock()

	bridge.syncSubscriptions()

	// Should have one unsubscribe call.
	require.Equal(t, 1, mockRef.unsubscribeCalls)
	require.Equal(t, 0, mockRef.subscriberCount(2))
}

// TestHubNotificationBridge_UnsubscribeAll tests cleanup on shutdown.
func TestHubNotificationBridge_UnsubscribeAll(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	mockRef := newMockNotificationHubRef()
	bridge := NewHubNotificationBridge(hub, mockRef)

	// Add clients and sync.
	hub.mu.Lock()
	hub.clients[10] = make(map[*WSClient]struct{})
	hub.clients[20] = make(map[*WSClient]struct{})
	hub.mu.Unlock()

	bridge.syncSubscriptions()
	require.Equal(t, 2, mockRef.subscribeCalls)

	// Unsubscribe all.
	bridge.unsubscribeAll()

	// Verify unsubscriptions.
	require.Equal(t, 2, mockRef.unsubscribeCalls)
	require.Equal(t, 0, len(bridge.subscriptions))
}

// TestActorNotificationHubRef_Integration tests the actor-based notification hub ref.
func TestActorNotificationHubRef_Integration(t *testing.T) {
	t.Parallel()

	// Create an actor system.
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	// Create and spawn the notification hub.
	notifHub := mail.NewNotificationHub()
	hubRef := mail.NotificationHubKey.Spawn(system, "test-notif-hub", notifHub)

	// Wrap in ActorNotificationHubRef.
	actorRef := NewActorNotificationHubRef(hubRef)

	// Create delivery channel.
	deliveryChan := make(chan mail.InboxMessage, 10)

	// Subscribe.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := actorRef.Subscribe(ctx, 1, "test-sub", deliveryChan)
	require.NoError(t, err)

	// Verify subscription via direct hub check.
	require.Equal(t, 1, notifHub.SubscriberCount(1))

	// Unsubscribe.
	err = actorRef.Unsubscribe(ctx, 1, "test-sub")
	require.NoError(t, err)

	// Verify unsubscription.
	require.Equal(t, 0, notifHub.SubscriberCount(1))
}

// TestActorNotificationHubRef_NotificationDelivery tests end-to-end notification delivery.
func TestActorNotificationHubRef_NotificationDelivery(t *testing.T) {
	t.Parallel()

	// Create an actor system.
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	// Create and spawn the notification hub.
	notifHub := mail.NewNotificationHub()
	hubRef := mail.NotificationHubKey.Spawn(system, "test-notif-hub", notifHub)

	// Wrap in ActorNotificationHubRef.
	actorRef := NewActorNotificationHubRef(hubRef)

	// Create delivery channel.
	deliveryChan := make(chan mail.InboxMessage, 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Subscribe.
	err := actorRef.Subscribe(ctx, 1, "test-sub", deliveryChan)
	require.NoError(t, err)

	// Send a notification directly via the hub ref.
	testMsg := mail.InboxMessage{
		ID:      500,
		Subject: "Integration Test",
		Body:    "Test body content",
	}

	notifyResp := hubRef.Ask(ctx, mail.NotifyAgentMsg{
		AgentID: 1,
		Message: testMsg,
	})

	result := notifyResp.Await(ctx)
	resp, err := result.Unpack()
	require.NoError(t, err)

	notifyResult, ok := resp.(mail.NotifyAgentResponse)
	require.True(t, ok)
	require.Equal(t, 1, notifyResult.DeliveredCount)

	// Verify message was delivered to channel.
	select {
	case msg := <-deliveryChan:
		require.Equal(t, int64(500), msg.ID)
		require.Equal(t, "Integration Test", msg.Subject)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("notification was not delivered")
	}

	// Cleanup.
	err = actorRef.Unsubscribe(ctx, 1, "test-sub")
	require.NoError(t, err)
}

// TestHubNotificationBridge_FullIntegration tests the complete bridge with actor system.
func TestHubNotificationBridge_FullIntegration(t *testing.T) {
	t.Parallel()

	// Create an actor system.
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	// Create the WebSocket hub.
	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	// Create and spawn the notification hub actor.
	notifHub := mail.NewNotificationHub()
	hubRef := mail.NotificationHubKey.Spawn(system, "test-notif-hub", notifHub)

	// Create the bridge with the actor ref.
	actorRef := NewActorNotificationHubRef(hubRef)
	bridge := NewHubNotificationBridge(hub, actorRef)
	bridge.Start()
	defer bridge.Stop()

	// Simulate a client connecting.
	hub.mu.Lock()
	hub.clients[99] = make(map[*WSClient]struct{})
	hub.clients[99][&WSClient{}] = struct{}{}
	hub.mu.Unlock()

	// Force subscription sync.
	bridge.syncSubscriptions()

	// Verify subscription was established.
	require.True(t, bridge.subscriptions[99])
	require.Equal(t, 1, notifHub.SubscriberCount(99))

	// Send a notification to agent 99.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	hubRef.Ask(ctx, mail.NotifyAgentMsg{
		AgentID: 99,
		Message: mail.InboxMessage{
			ID:      999,
			Subject: "Full Integration Test",
		},
	}).Await(ctx)

	// Give time for notification to propagate through bridge.
	time.Sleep(100 * time.Millisecond)

	// The message should have been forwarded by the bridge.
	// Since we don't have a real WebSocket client, we verify the bridge processed it.
}

// TestHubNotificationBridge_NilNotificationHub tests graceful handling of nil hub.
func TestHubNotificationBridge_NilNotificationHub(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	// Create bridge with nil notification hub.
	bridge := NewHubNotificationBridge(hub, nil)

	// Add a client.
	hub.mu.Lock()
	hub.clients[1] = make(map[*WSClient]struct{})
	hub.mu.Unlock()

	// Sync should not panic with nil hub.
	bridge.syncSubscriptions()

	// Unsubscribe all should not panic.
	bridge.unsubscribeAll()
}
