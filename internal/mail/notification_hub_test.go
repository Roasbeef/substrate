package mail

import (
	"context"
	"testing"
	"time"

	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/stretchr/testify/require"
)

// TestNotificationHubSubscribeUnsubscribe tests subscribing and unsubscribing.
func TestNotificationHubSubscribeUnsubscribe(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	// Create a delivery channel.
	deliveryChan := make(chan InboxMessage, 10)

	// Subscribe.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subResp := hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan,
	})

	resp := subResp.Await(ctx)
	result, err := resp.Unpack()
	require.NoError(t, err)

	subResult, ok := result.(SubscribeAgentResponse)
	require.True(t, ok)
	require.True(t, subResult.Success)

	// Verify subscriber count.
	require.Equal(t, 1, hub.SubscriberCount(1))

	// Unsubscribe.
	unsubResp := hubRef.Ask(ctx, UnsubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
	})

	resp2 := unsubResp.Await(ctx)
	result2, err := resp2.Unpack()
	require.NoError(t, err)

	unsubResult, ok := result2.(UnsubscribeAgentResponse)
	require.True(t, ok)
	require.True(t, unsubResult.Success)

	// Verify subscriber removed.
	require.Equal(t, 0, hub.SubscriberCount(1))
}

// TestNotificationHubNotifyAgent tests sending notifications to subscribers.
func TestNotificationHubNotifyAgent(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	// Create delivery channels for two subscribers.
	deliveryChan1 := make(chan InboxMessage, 10)
	deliveryChan2 := make(chan InboxMessage, 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Subscribe both.
	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan1,
	}).Await(ctx)

	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-2",
		DeliveryChan: deliveryChan2,
	}).Await(ctx)

	require.Equal(t, 2, hub.SubscriberCount(1))

	// Send a notification.
	testMsg := InboxMessage{
		ID:      100,
		Subject: "Test Subject",
		Body:    "Test Body",
	}

	notifyResp := hubRef.Ask(ctx, NotifyAgentMsg{
		AgentID: 1,
		Message: testMsg,
	})

	resp := notifyResp.Await(ctx)
	result, err := resp.Unpack()
	require.NoError(t, err)

	notifyResult, ok := result.(NotifyAgentResponse)
	require.True(t, ok)
	require.Equal(t, 2, notifyResult.DeliveredCount)

	// Verify both subscribers received the message.
	select {
	case msg := <-deliveryChan1:
		require.Equal(t, int64(100), msg.ID)
		require.Equal(t, "Test Subject", msg.Subject)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 1 did not receive message")
	}

	select {
	case msg := <-deliveryChan2:
		require.Equal(t, int64(100), msg.ID)
		require.Equal(t, "Test Subject", msg.Subject)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 2 did not receive message")
	}
}

// TestNotificationHubNotifyTopic tests sending topic notifications.
func TestNotificationHubNotifyTopic(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	// Create delivery channels for subscribers on different agents.
	deliveryChan1 := make(chan InboxMessage, 10)
	deliveryChan2 := make(chan InboxMessage, 10)
	deliveryChan3 := make(chan InboxMessage, 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Subscribe to different agents.
	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan1,
	}).Await(ctx)

	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      2,
		SubscriberID: "sub-2",
		DeliveryChan: deliveryChan2,
	}).Await(ctx)

	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      3,
		SubscriberID: "sub-3",
		DeliveryChan: deliveryChan3,
	}).Await(ctx)

	// Send a topic notification to agents 1 and 2 (not 3).
	testMsg := InboxMessage{
		ID:      200,
		Subject: "Topic Message",
	}

	notifyResp := hubRef.Ask(ctx, NotifyTopicMsg{
		TopicID:  10,
		AgentIDs: []int64{1, 2},
		Message:  testMsg,
	})

	resp := notifyResp.Await(ctx)
	result, err := resp.Unpack()
	require.NoError(t, err)

	notifyResult, ok := result.(NotifyTopicResponse)
	require.True(t, ok)
	require.Equal(t, 2, notifyResult.DeliveredCount)

	// Verify agents 1 and 2 received, agent 3 did not.
	select {
	case msg := <-deliveryChan1:
		require.Equal(t, int64(200), msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 1 did not receive message")
	}

	select {
	case msg := <-deliveryChan2:
		require.Equal(t, int64(200), msg.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber 2 did not receive message")
	}

	// Agent 3 should NOT have received a message.
	select {
	case <-deliveryChan3:
		t.Fatal("subscriber 3 should not have received message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message.
	}
}

// TestNotificationHubNonBlockingSend tests that notifications don't block.
func TestNotificationHubNonBlockingSend(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	// Create a small buffer channel that will fill up.
	deliveryChan := make(chan InboxMessage, 1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan,
	}).Await(ctx)

	// Send multiple notifications - should not block even with full channel.
	for i := 0; i < 5; i++ {
		notifyResp := hubRef.Ask(ctx, NotifyAgentMsg{
			AgentID: 1,
			Message: InboxMessage{ID: int64(i)},
		})

		resp := notifyResp.Await(ctx)
		_, err := resp.Unpack()
		require.NoError(t, err)
	}

	// Only one message should be in the channel (first one).
	select {
	case msg := <-deliveryChan:
		require.Equal(t, int64(0), msg.ID)
	default:
		t.Fatal("expected at least one message")
	}
}

// TestNotificationHubIdempotentUnsubscribe tests unsubscribing is idempotent.
func TestNotificationHubIdempotentUnsubscribe(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Unsubscribe from non-existent subscription - should succeed.
	unsubResp := hubRef.Ask(ctx, UnsubscribeAgentMsg{
		AgentID:      999,
		SubscriberID: "non-existent",
	})

	resp := unsubResp.Await(ctx)
	result, err := resp.Unpack()
	require.NoError(t, err)

	unsubResult, ok := result.(UnsubscribeAgentResponse)
	require.True(t, ok)
	require.True(t, unsubResult.Success)
}

// TestNotificationHubDuplicateSubscribe tests subscribing twice is idempotent.
func TestNotificationHubDuplicateSubscribe(t *testing.T) {
	system := actor.NewActorSystem()
	defer system.Shutdown(context.Background())

	hub := NewNotificationHub()
	hubRef := NotificationHubKey.Spawn(system, "test-hub", hub)

	deliveryChan := make(chan InboxMessage, 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Subscribe twice with same ID.
	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan,
	}).Await(ctx)

	hubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      1,
		SubscriberID: "sub-1",
		DeliveryChan: deliveryChan,
	}).Await(ctx)

	// Should only have one subscriber.
	require.Equal(t, 1, hub.SubscriberCount(1))
}
