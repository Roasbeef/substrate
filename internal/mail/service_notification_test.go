package mail

import (
	"context"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/stretchr/testify/require"
)

// shutdownSystem is a test helper to gracefully shutdown an actor system.
func shutdownSystem(t testing.TB, system *actor.ActorSystem) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = system.Shutdown(ctx)
}

// TestServiceNotifiesRecipients tests that sending a message triggers
// notifications to all recipients via the notification hub actor.
func TestServiceNotifiesRecipients(t *testing.T) {
	t.Parallel()

	// Set up test storage.
	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create sender and recipients.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Sender",
	})
	require.NoError(t, err)

	recipient1, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient1",
	})
	require.NoError(t, err)

	recipient2, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient2",
	})
	require.NoError(t, err)

	// Create an actor system.
	system := actor.NewActorSystem()
	defer shutdownSystem(t, system)

	// Create and spawn the notification hub.
	notifHub := NewNotificationHub()
	notifHubRef := NotificationHubKey.Spawn(system, "test-notif-hub", notifHub)

	// Subscribe to notifications for both recipients.
	deliveryChan1 := make(chan InboxMessage, 10)
	deliveryChan2 := make(chan InboxMessage, 10)

	subResp1 := notifHubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      recipient1.ID,
		SubscriberID: "test-sub-1",
		DeliveryChan: deliveryChan1,
	})
	result1 := subResp1.Await(ctx)
	_, err = result1.Unpack()
	require.NoError(t, err)

	subResp2 := notifHubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      recipient2.ID,
		SubscriberID: "test-sub-2",
		DeliveryChan: deliveryChan2,
	})
	result2 := subResp2.Await(ctx)
	_, err = result2.Unpack()
	require.NoError(t, err)

	// Create the mail service with the notification hub.
	svc := NewService(ServiceConfig{
		Store:           storage,
		NotificationHub: notifHubRef,
	})

	// Send a message to both recipients.
	resp, err := svc.Send(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Recipient1", "Recipient2"},
		Subject:        "Test Message",
		Body:           "Hello, this is a test!",
		Priority:       PriorityNormal,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageID)

	// Wait for notifications to be delivered.
	timeout := time.After(2 * time.Second)

	// Check recipient 1 received the notification.
	select {
	case msg := <-deliveryChan1:
		require.Equal(t, resp.MessageID, msg.ID)
		require.Equal(t, sender.ID, msg.SenderID)
		require.Equal(t, "Sender", msg.SenderName)
		require.Equal(t, "Test Message", msg.Subject)
		require.Equal(t, "Hello, this is a test!", msg.Body)
		require.Equal(t, StateUnreadStr.String(), msg.State)
	case <-timeout:
		t.Fatal("timeout waiting for notification to recipient 1")
	}

	// Check recipient 2 received the notification.
	select {
	case msg := <-deliveryChan2:
		require.Equal(t, resp.MessageID, msg.ID)
		require.Equal(t, sender.ID, msg.SenderID)
		require.Equal(t, "Sender", msg.SenderName)
		require.Equal(t, "Test Message", msg.Subject)
	case <-timeout:
		t.Fatal("timeout waiting for notification to recipient 2")
	}
}

// TestServiceNoNotificationWithoutHub tests that sending a message works
// without a notification hub (notifications are simply skipped).
func TestServiceNoNotificationWithoutHub(t *testing.T) {
	t.Parallel()

	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create agents.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Sender",
	})
	require.NoError(t, err)

	_, err = storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient",
	})
	require.NoError(t, err)

	// Create mail service without notification hub.
	svc := NewServiceWithStore(storage)

	// Send a message. Should succeed without errors.
	resp, err := svc.Send(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Recipient"},
		Subject:        "No Notification Test",
		Body:           "This won't trigger notifications",
		Priority:       PriorityNormal,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageID)
}

// TestServiceNotificationPriority tests that message priority is correctly
// passed through to the notification.
func TestServiceNotificationPriority(t *testing.T) {
	t.Parallel()

	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Sender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient",
	})
	require.NoError(t, err)

	system := actor.NewActorSystem()
	defer shutdownSystem(t, system)

	notifHub := NewNotificationHub()
	notifHubRef := NotificationHubKey.Spawn(system, "test-notif-hub", notifHub)

	deliveryChan := make(chan InboxMessage, 10)
	subResp := notifHubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      recipient.ID,
		SubscriberID: "test-sub",
		DeliveryChan: deliveryChan,
	})
	result := subResp.Await(ctx)
	_, err = result.Unpack()
	require.NoError(t, err)

	svc := NewService(ServiceConfig{
		Store:           storage,
		NotificationHub: notifHubRef,
	})

	// Send an urgent message.
	_, err = svc.Send(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Recipient"},
		Subject:        "Urgent!",
		Body:           "This is urgent",
		Priority:       PriorityUrgent,
	})
	require.NoError(t, err)

	// Verify the notification has the correct priority.
	select {
	case msg := <-deliveryChan:
		require.Equal(t, PriorityUrgent, msg.Priority)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

// TestMailActorWithNotificationHub tests the full actor-based integration where
// both the mail service and notification hub are actors.
func TestMailActorWithNotificationHub(t *testing.T) {
	t.Parallel()

	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create agents.
	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Sender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient",
	})
	require.NoError(t, err)

	// Create actor system.
	system := actor.NewActorSystem()
	defer shutdownSystem(t, system)

	// Spawn notification hub actor.
	notifHub := NewNotificationHub()
	notifHubRef := NotificationHubKey.Spawn(system, "notif-hub", notifHub)

	// Subscribe to notifications.
	deliveryChan := make(chan InboxMessage, 10)
	subResp := notifHubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      recipient.ID,
		SubscriberID: "test-subscriber",
		DeliveryChan: deliveryChan,
	})
	result := subResp.Await(ctx)
	_, err = result.Unpack()
	require.NoError(t, err)

	// Create mail actor with notification hub reference.
	mailActorRef := StartMailActor(ActorConfig{
		ID:              "mail-actor",
		Store:           storage,
		NotificationHub: notifHubRef,
	})

	// Send message via the mail actor using Ask.
	sendResp := mailActorRef.Ask(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Recipient"},
		Subject:        "Actor Test",
		Body:           "Message from actor",
		Priority:       PriorityUrgent,
	})

	sendResult := sendResp.Await(ctx)
	sendRaw, err := sendResult.Unpack()
	require.NoError(t, err)

	sendResponse, ok := sendRaw.(SendMailResponse)
	require.True(t, ok)
	require.NoError(t, sendResponse.Error)
	require.NotZero(t, sendResponse.MessageID)

	// Verify notification was received.
	select {
	case msg := <-deliveryChan:
		require.Equal(t, sendResponse.MessageID, msg.ID)
		require.Equal(t, "Actor Test", msg.Subject)
		require.Equal(t, PriorityUrgent, msg.Priority)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

// TestServiceSetNotificationHubDeferred tests setting the notification hub
// after service creation (deferred initialization).
func TestServiceSetNotificationHubDeferred(t *testing.T) {
	t.Parallel()

	storage, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	sender, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Sender",
	})
	require.NoError(t, err)

	recipient, err := storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: "Recipient",
	})
	require.NoError(t, err)

	system := actor.NewActorSystem()
	defer shutdownSystem(t, system)

	// Create service without notification hub initially.
	svc := NewServiceWithStore(storage)

	// Now create and set the notification hub.
	notifHub := NewNotificationHub()
	notifHubRef := NotificationHubKey.Spawn(system, "notif-hub", notifHub)

	deliveryChan := make(chan InboxMessage, 10)
	subResp := notifHubRef.Ask(ctx, SubscribeAgentMsg{
		AgentID:      recipient.ID,
		SubscriberID: "test-sub",
		DeliveryChan: deliveryChan,
	})
	result := subResp.Await(ctx)
	_, err = result.Unpack()
	require.NoError(t, err)

	// Set the hub after creation.
	svc.SetNotificationHub(notifHubRef)

	// Send a message - should now trigger notifications.
	_, err = svc.Send(ctx, SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Recipient"},
		Subject:        "Deferred Hub Test",
		Body:           "Testing deferred hub setup",
		Priority:       PriorityNormal,
	})
	require.NoError(t, err)

	// Verify notification was received.
	select {
	case msg := <-deliveryChan:
		require.Equal(t, "Deferred Hub Test", msg.Subject)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification after deferred hub setup")
	}
}
