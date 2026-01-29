package mail

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

// testDB creates a temporary test database with migrations applied.
func testDB(t *testing.T) (*db.Store, func()) {
	t.Helper()

	// Create temp directory for test database.
	tmpDir, err := os.MkdirTemp("", "subtrate-mail-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	store, err := db.Open(dbPath)
	require.NoError(t, err)

	// Find migrations directory.
	migrationsDir := findMigrationsDir(t)

	// Run migrations.
	err = db.RunMigrations(store.DB(), migrationsDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

// findMigrationsDir locates the migrations directory relative to the test.
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	paths := []string{
		"../db/migrations",
		"../../internal/db/migrations",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		p := filepath.Join(gopath, "src/github.com/roasbeef/subtrate/internal/db/migrations")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatal("Could not find migrations directory")
	return ""
}

// createTestAgent creates an agent for testing.
func createTestAgent(t *testing.T, store *db.Store, name string) sqlc.Agent {
	t.Helper()

	agent, err := store.Queries().CreateAgent(context.Background(), sqlc.CreateAgentParams{
		Name:      name,
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	return agent
}

// createTestTopic creates a topic for testing.
func createTestTopic(t *testing.T, store *db.Store, name, topicType string) sqlc.Topic {
	t.Helper()

	topic, err := store.Queries().CreateTopic(context.Background(), sqlc.CreateTopicParams{
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	return topic
}

func TestService_SendMail(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	// Create sender and recipient agents.
	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	req := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test Subject",
		Body:           "Test body content",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(SendMailResponse)
	require.NoError(t, resp.Error)
	require.Greater(t, resp.MessageID, int64(0))
	require.NotEmpty(t, resp.ThreadID)
}

func TestService_SendMail_WithThread(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send first message.
	req1 := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "First Message",
		Body:           "First body",
		Priority:       PriorityNormal,
	}

	result1 := svc.Receive(ctx, req1)
	val1, err := result1.Unpack()
	require.NoError(t, err)
	resp1 := val1.(SendMailResponse)
	require.NoError(t, resp1.Error)

	// Send reply in same thread.
	req2 := SendMailRequest{
		SenderID:       recipient.ID,
		RecipientNames: []string{sender.Name},
		Subject:        "Re: First Message",
		Body:           "Reply body",
		Priority:       PriorityNormal,
		ThreadID:       resp1.ThreadID,
	}

	result2 := svc.Receive(ctx, req2)
	val2, err := result2.Unpack()
	require.NoError(t, err)
	resp2 := val2.(SendMailResponse)
	require.NoError(t, resp2.Error)
	require.Equal(t, resp1.ThreadID, resp2.ThreadID)
}

func TestService_FetchInbox(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a few messages.
	for i := 0; i < 3; i++ {
		req := SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: []string{recipient.Name},
			Subject:        "Message " + string(rune('A'+i)),
			Body:           "Body " + string(rune('A'+i)),
			Priority:       PriorityNormal,
		}

		result := svc.Receive(ctx, req)
		val, err := result.Unpack()
		require.NoError(t, err)
		resp := val.(SendMailResponse)
		require.NoError(t, resp.Error)
	}

	// Fetch inbox.
	fetchReq := FetchInboxRequest{
		AgentID: recipient.ID,
		Limit:   10,
	}

	result := svc.Receive(ctx, fetchReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(FetchInboxResponse)
	require.NoError(t, resp.Error)
	require.Len(t, resp.Messages, 3)
}

func TestService_FetchInbox_UnreadOnly(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send messages.
	var messageIDs []int64
	for i := 0; i < 3; i++ {
		req := SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: []string{recipient.Name},
			Subject:        "Message " + string(rune('A'+i)),
			Body:           "Body",
			Priority:       PriorityNormal,
		}

		result := svc.Receive(ctx, req)
		val, err := result.Unpack()
		require.NoError(t, err)
		resp := val.(SendMailResponse)
		require.NoError(t, resp.Error)
		messageIDs = append(messageIDs, resp.MessageID)
	}

	// Read one message.
	readReq := ReadMessageRequest{
		AgentID:   recipient.ID,
		MessageID: messageIDs[0],
	}
	result := svc.Receive(ctx, readReq)
	_, err := result.Unpack()
	require.NoError(t, err)

	// Fetch only unread.
	fetchReq := FetchInboxRequest{
		AgentID:    recipient.ID,
		Limit:      10,
		UnreadOnly: true,
	}

	result = svc.Receive(ctx, fetchReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(FetchInboxResponse)
	require.NoError(t, resp.Error)
	require.Len(t, resp.Messages, 2)
}

func TestService_ReadMessage(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test Subject",
		Body:           "Test body",
		Priority:       PriorityUrgent,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)
	require.NoError(t, sendResp.Error)

	// Read the message.
	readReq := ReadMessageRequest{
		AgentID:   recipient.ID,
		MessageID: sendResp.MessageID,
	}

	result = svc.Receive(ctx, readReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	readResp := val.(ReadMessageResponse)
	require.NoError(t, readResp.Error)
	require.NotNil(t, readResp.Message)
	require.Equal(t, "Test Subject", readResp.Message.Subject)
	require.Equal(t, "Test body", readResp.Message.Body)
	require.Equal(t, PriorityUrgent, readResp.Message.Priority)
	require.Equal(t, "read", readResp.Message.State)
}

func TestService_UpdateState(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test",
		Body:           "Body",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)

	// Star the message.
	starReq := UpdateStateRequest{
		AgentID:   recipient.ID,
		MessageID: sendResp.MessageID,
		NewState:  "starred",
	}

	result = svc.Receive(ctx, starReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	starResp := val.(UpdateStateResponse)
	require.NoError(t, starResp.Error)
	require.True(t, starResp.Success)
}

func TestService_UpdateState_Snooze(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test",
		Body:           "Body",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)

	// Snooze the message.
	snoozeUntil := time.Now().Add(time.Hour)
	snoozeReq := UpdateStateRequest{
		AgentID:      recipient.ID,
		MessageID:    sendResp.MessageID,
		NewState:     "snoozed",
		SnoozedUntil: &snoozeUntil,
	}

	result = svc.Receive(ctx, snoozeReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	snoozeResp := val.(UpdateStateResponse)
	require.NoError(t, snoozeResp.Error)
	require.True(t, snoozeResp.Success)
}

func TestService_AckMessage(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message with deadline.
	deadline := time.Now().Add(time.Hour)
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Urgent Task",
		Body:           "Please complete",
		Priority:       PriorityUrgent,
		Deadline:       &deadline,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)

	// Acknowledge the message.
	ackReq := AckMessageRequest{
		AgentID:   recipient.ID,
		MessageID: sendResp.MessageID,
	}

	result = svc.Receive(ctx, ackReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	ackResp := val.(AckMessageResponse)
	require.NoError(t, ackResp.Error)
	require.True(t, ackResp.Success)
}

func TestService_GetStatus(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send messages with different priorities.
	for _, priority := range []Priority{PriorityUrgent, PriorityNormal, PriorityLow} {
		req := SendMailRequest{
			SenderID:       sender.ID,
			RecipientNames: []string{recipient.Name},
			Subject:        "Test " + string(priority),
			Body:           "Body",
			Priority:       priority,
		}

		result := svc.Receive(ctx, req)
		val, err := result.Unpack()
		require.NoError(t, err)
		resp := val.(SendMailResponse)
		require.NoError(t, resp.Error)
	}

	// Get status.
	statusReq := GetStatusRequest{
		AgentID: recipient.ID,
	}

	result := svc.Receive(ctx, statusReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	statusResp := val.(GetStatusResponse)
	require.NoError(t, statusResp.Error)
	require.Equal(t, recipient.ID, statusResp.Status.AgentID)
	require.Equal(t, recipient.Name, statusResp.Status.AgentName)
	require.Equal(t, int64(3), statusResp.Status.UnreadCount)
	require.Equal(t, int64(1), statusResp.Status.UrgentCount)
}

func TestService_Publish(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	// Create publisher and subscribers.
	publisher := createTestAgent(t, store, "Publisher")
	sub1 := createTestAgent(t, store, "Subscriber1")
	sub2 := createTestAgent(t, store, "Subscriber2")

	// Create a topic.
	topic := createTestTopic(t, store, "announcements", "broadcast")

	// Subscribe agents to topic.
	err := store.Queries().CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      sub1.ID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	err = store.Queries().CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      sub2.ID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Publish a message.
	pubReq := PublishRequest{
		SenderID:  publisher.ID,
		TopicName: topic.Name,
		Subject:   "Announcement",
		Body:      "Important update!",
		Priority:  PriorityNormal,
	}

	result := svc.Receive(ctx, pubReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	pubResp := val.(PublishResponse)
	require.NoError(t, pubResp.Error)
	require.Greater(t, pubResp.MessageID, int64(0))
	require.Equal(t, 2, pubResp.RecipientsCount)
}

func TestService_PollChanges(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	// Create agents.
	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Create a topic and subscribe.
	topic := createTestTopic(t, store, "notifications", "broadcast")
	err := store.Queries().CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      recipient.ID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Publish some messages.
	for i := 0; i < 3; i++ {
		pubReq := PublishRequest{
			SenderID:  sender.ID,
			TopicName: topic.Name,
			Subject:   "Update " + string(rune('A'+i)),
			Body:      "Content",
			Priority:  PriorityNormal,
		}

		result := svc.Receive(ctx, pubReq)
		val, err := result.Unpack()
		require.NoError(t, err)
		pubResp := val.(PublishResponse)
		require.NoError(t, pubResp.Error)
	}

	// Poll for changes from offset 0.
	pollReq := PollChangesRequest{
		AgentID:      recipient.ID,
		SinceOffsets: map[int64]int64{},
	}

	result := svc.Receive(ctx, pollReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	pollResp := val.(PollChangesResponse)
	require.NoError(t, pollResp.Error)
	require.Len(t, pollResp.NewMessages, 3)
	require.Contains(t, pollResp.NewOffsets, topic.ID)
	require.Equal(t, int64(3), pollResp.NewOffsets[topic.ID])

	// Poll again from the last offset - should get no new messages.
	pollReq2 := PollChangesRequest{
		AgentID:      recipient.ID,
		SinceOffsets: pollResp.NewOffsets,
	}

	result = svc.Receive(ctx, pollReq2)
	val, err = result.Unpack()
	require.NoError(t, err)

	pollResp2 := val.(PollChangesResponse)
	require.NoError(t, pollResp2.Error)
	require.Empty(t, pollResp2.NewMessages)
}

func TestService_UnknownMessageType(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	// Create a fake message type that implements MailRequest.
	type fakeRequest struct {
		SendMailRequest
	}

	result := svc.Receive(ctx, fakeRequest{})
	require.True(t, result.IsErr())
}

func TestService_ReadMessage_NotFound(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	recipient := createTestAgent(t, store, "Recipient")

	// Try to read non-existent message.
	readReq := ReadMessageRequest{
		AgentID:   recipient.ID,
		MessageID: 9999,
	}

	result := svc.Receive(ctx, readReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	readResp := val.(ReadMessageResponse)
	require.Error(t, readResp.Error)
}

func TestService_UpdateState_NonExistentMessage(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	agent := createTestAgent(t, store, "Agent")

	// SQLite UPDATE succeeds even when no rows match (no rows affected).
	// The service reports success because the query completed without error.
	updateReq := UpdateStateRequest{
		AgentID:   agent.ID,
		MessageID: 9999,
		NewState:  "starred",
	}

	result := svc.Receive(ctx, updateReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	updateResp := val.(UpdateStateResponse)
	// UPDATE succeeds even when no rows match - this is expected SQLite behavior.
	require.NoError(t, updateResp.Error)
	require.True(t, updateResp.Success)
}

func TestService_AckMessage_NonExistentMessage(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	agent := createTestAgent(t, store, "Agent")

	// SQLite UPDATE succeeds even when no rows match (no rows affected).
	// The service reports success because the query completed without error.
	ackReq := AckMessageRequest{
		AgentID:   agent.ID,
		MessageID: 9999,
	}

	result := svc.Receive(ctx, ackReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	ackResp := val.(AckMessageResponse)
	// UPDATE succeeds even when no rows match - this is expected SQLite behavior.
	require.NoError(t, ackResp.Error)
	require.True(t, ackResp.Success)
}

func TestService_Publish_TopicNotFound(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	publisher := createTestAgent(t, store, "Publisher")

	// Try to publish to non-existent topic.
	pubReq := PublishRequest{
		SenderID:  publisher.ID,
		TopicName: "non-existent-topic",
		Subject:   "Test",
		Body:      "Body",
		Priority:  PriorityNormal,
	}

	result := svc.Receive(ctx, pubReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	pubResp := val.(PublishResponse)
	require.Error(t, pubResp.Error)
}

func TestService_SendMail_RecipientNotFound(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")

	// Try to send to non-existent recipient.
	req := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"NonExistentAgent"},
		Subject:        "Test",
		Body:           "Body",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	require.NoError(t, err)

	resp := val.(SendMailResponse)
	require.Error(t, resp.Error)
	require.Contains(t, resp.Error.Error(), "not found")
}

func TestService_GetStatus_NoMessages(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	agent := createTestAgent(t, store, "Agent")

	// Get status with no messages.
	statusReq := GetStatusRequest{
		AgentID: agent.ID,
	}

	result := svc.Receive(ctx, statusReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	statusResp := val.(GetStatusResponse)
	require.NoError(t, statusResp.Error)
	require.Equal(t, int64(0), statusResp.Status.UnreadCount)
	require.Equal(t, int64(0), statusResp.Status.UrgentCount)
}

func TestService_UpdateState_Archive(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test",
		Body:           "Body",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)

	// Archive the message.
	archiveReq := UpdateStateRequest{
		AgentID:   recipient.ID,
		MessageID: sendResp.MessageID,
		NewState:  "archived",
	}

	result = svc.Receive(ctx, archiveReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	archiveResp := val.(UpdateStateResponse)
	require.NoError(t, archiveResp.Error)
	require.True(t, archiveResp.Success)
}

func TestService_UpdateState_Trash(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	svc := NewService(store)
	ctx := context.Background()

	sender := createTestAgent(t, store, "Sender")
	recipient := createTestAgent(t, store, "Recipient")

	// Send a message.
	sendReq := SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipient.Name},
		Subject:        "Test",
		Body:           "Body",
		Priority:       PriorityNormal,
	}

	result := svc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(SendMailResponse)

	// Trash the message.
	trashReq := UpdateStateRequest{
		AgentID:   recipient.ID,
		MessageID: sendResp.MessageID,
		NewState:  "trash",
	}

	result = svc.Receive(ctx, trashReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	trashResp := val.(UpdateStateResponse)
	require.NoError(t, trashResp.Error)
	require.True(t, trashResp.Success)
}
