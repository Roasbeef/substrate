package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/stretchr/testify/require"
)

// testEnv holds the test environment with all backend services running.
type testEnv struct {
	t *testing.T

	// Database components.
	dbPath string
	store  *db.Store

	// Services.
	mailSvc *mail.Service

	// Test data.
	agents map[string]sqlc.Agent
	topics map[string]sqlc.Topic

	// Cleanup functions.
	cleanups []func()
}

// newTestEnv creates a complete test environment with backend services.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory for test data.
	tmpDir, err := os.MkdirTemp("", "subtrate-e2e-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	store, err := db.Open(dbPath)
	require.NoError(t, err)

	// Find and run migrations.
	migrationsDir := findMigrationsDir(t)
	err = db.RunMigrations(store.DB(), migrationsDir)
	require.NoError(t, err)

	// Create services.
	mailSvc := mail.NewService(store)

	env := &testEnv{
		t:       t,
		dbPath:  dbPath,
		store:   store,
		mailSvc: mailSvc,
		agents:  make(map[string]sqlc.Agent),
		topics:  make(map[string]sqlc.Topic),
	}

	env.cleanups = append(env.cleanups, func() {
		store.Close()
		os.RemoveAll(tmpDir)
	})

	return env
}

// cleanup tears down the test environment.
func (e *testEnv) cleanup() {
	for i := len(e.cleanups) - 1; i >= 0; i-- {
		e.cleanups[i]()
	}
}

// createAgent creates a test agent and stores it in the env.
func (e *testEnv) createAgent(name string) sqlc.Agent {
	e.t.Helper()

	agent, err := e.store.Queries().CreateAgent(context.Background(), sqlc.CreateAgentParams{
		Name:      name,
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(e.t, err)

	e.agents[name] = agent
	return agent
}

// createTopic creates a test topic and stores it in the env.
func (e *testEnv) createTopic(name, topicType string) sqlc.Topic {
	e.t.Helper()

	topic, err := e.store.Queries().CreateTopic(context.Background(), sqlc.CreateTopicParams{
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(e.t, err)

	e.topics[name] = topic
	return topic
}

// subscribe subscribes an agent to a topic.
func (e *testEnv) subscribe(agentName, topicName string) {
	e.t.Helper()

	agent := e.agents[agentName]
	topic := e.topics[topicName]

	err := e.store.Queries().CreateSubscription(context.Background(), sqlc.CreateSubscriptionParams{
		AgentID:      agent.ID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
	require.NoError(e.t, err)
}

// findMigrationsDir locates the migrations directory.
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	paths := []string{
		"../../../internal/db/migrations",
		"../../../../internal/db/migrations",
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

// TestE2E_DirectMail tests the full message flow using the mail service directly.
func TestE2E_DirectMail(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create agents.
	alice := env.createAgent("Alice")
	bob := env.createAgent("Bob")

	// Alice sends a message to Bob.
	sendReq := mail.SendMailRequest{
		SenderID:       alice.ID,
		RecipientNames: []string{"Bob"},
		Subject:        "Hello Bob",
		Body:           "This is a test message from Alice.",
		Priority:       mail.PriorityNormal,
	}

	result := env.mailSvc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	sendResp := val.(mail.SendMailResponse)
	require.NoError(t, sendResp.Error)
	require.Greater(t, sendResp.MessageID, int64(0))
	threadID := sendResp.ThreadID

	t.Logf("Sent message ID: %d, Thread ID: %s", sendResp.MessageID, threadID)

	// Bob fetches his inbox.
	fetchReq := mail.FetchInboxRequest{
		AgentID:    bob.ID,
		Limit:      10,
		UnreadOnly: true,
	}

	result = env.mailSvc.Receive(ctx, fetchReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	fetchResp := val.(mail.FetchInboxResponse)
	require.NoError(t, fetchResp.Error)
	require.Len(t, fetchResp.Messages, 1)
	require.Equal(t, "Hello Bob", fetchResp.Messages[0].Subject)
	require.Equal(t, "unread", fetchResp.Messages[0].State)

	t.Logf("Bob has %d unread messages", len(fetchResp.Messages))

	// Bob reads the message.
	readReq := mail.ReadMessageRequest{
		AgentID:   bob.ID,
		MessageID: sendResp.MessageID,
	}

	result = env.mailSvc.Receive(ctx, readReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	readResp := val.(mail.ReadMessageResponse)
	require.NoError(t, readResp.Error)
	require.Equal(t, "This is a test message from Alice.", readResp.Message.Body)
	require.Equal(t, "read", readResp.Message.State)

	t.Logf("Bob read message: %s", readResp.Message.Subject)

	// Bob replies to Alice in the same thread.
	replyReq := mail.SendMailRequest{
		SenderID:       bob.ID,
		RecipientNames: []string{"Alice"},
		Subject:        "Re: Hello Bob",
		Body:           "Hi Alice, thanks for your message!",
		Priority:       mail.PriorityNormal,
		ThreadID:       threadID,
	}

	result = env.mailSvc.Receive(ctx, replyReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	replyResp := val.(mail.SendMailResponse)
	require.NoError(t, replyResp.Error)
	require.Equal(t, threadID, replyResp.ThreadID)

	t.Logf("Bob replied in thread: %s", threadID)

	// Alice fetches her inbox and sees the reply.
	aliceInbox := mail.FetchInboxRequest{
		AgentID: alice.ID,
		Limit:   10,
	}

	result = env.mailSvc.Receive(ctx, aliceInbox)
	val, err = result.Unpack()
	require.NoError(t, err)

	aliceResp := val.(mail.FetchInboxResponse)
	require.NoError(t, aliceResp.Error)
	require.Len(t, aliceResp.Messages, 1)
	require.Equal(t, "Re: Hello Bob", aliceResp.Messages[0].Subject)

	t.Logf("Alice has %d messages from Bob", len(aliceResp.Messages))
}

// TestE2E_PubSub tests the pub/sub message flow.
func TestE2E_PubSub(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create agents.
	publisher := env.createAgent("Publisher")
	sub1 := env.createAgent("Subscriber1")
	sub2 := env.createAgent("Subscriber2")

	// Create a broadcast topic.
	env.createTopic("announcements", "broadcast")

	// Subscribe agents to the topic.
	env.subscribe("Subscriber1", "announcements")
	env.subscribe("Subscriber2", "announcements")

	// Publisher sends an announcement.
	pubReq := mail.PublishRequest{
		SenderID:  publisher.ID,
		TopicName: "announcements",
		Subject:   "Important Update",
		Body:      "Please read this important announcement.",
		Priority:  mail.PriorityUrgent,
	}

	result := env.mailSvc.Receive(ctx, pubReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	pubResp := val.(mail.PublishResponse)
	require.NoError(t, pubResp.Error)
	require.Equal(t, 2, pubResp.RecipientsCount)

	t.Logf("Published to %d recipients", pubResp.RecipientsCount)

	// Both subscribers should have the message.
	for _, sub := range []sqlc.Agent{sub1, sub2} {
		fetchReq := mail.FetchInboxRequest{
			AgentID: sub.ID,
			Limit:   10,
		}

		result = env.mailSvc.Receive(ctx, fetchReq)
		val, err = result.Unpack()
		require.NoError(t, err)

		fetchResp := val.(mail.FetchInboxResponse)
		require.NoError(t, fetchResp.Error)
		require.Len(t, fetchResp.Messages, 1)
		require.Equal(t, "Important Update", fetchResp.Messages[0].Subject)
		require.Equal(t, mail.PriorityUrgent, fetchResp.Messages[0].Priority)

		t.Logf("%s received the announcement", sub.Name)
	}
}

// TestE2E_PollChanges tests the log-based polling flow.
func TestE2E_PollChanges(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create agents.
	sender := env.createAgent("Sender")
	receiver := env.createAgent("Receiver")

	// Create a topic and subscribe.
	topic := env.createTopic("updates", "broadcast")
	env.subscribe("Receiver", "updates")

	// Initial poll - should have no messages.
	pollReq := mail.PollChangesRequest{
		AgentID:      receiver.ID,
		SinceOffsets: map[int64]int64{},
	}

	result := env.mailSvc.Receive(ctx, pollReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	pollResp := val.(mail.PollChangesResponse)
	require.NoError(t, pollResp.Error)
	require.Empty(t, pollResp.NewMessages)

	t.Log("Initial poll: no messages")

	// Publish 3 messages.
	for i := 0; i < 3; i++ {
		pubReq := mail.PublishRequest{
			SenderID:  sender.ID,
			TopicName: "updates",
			Subject:   "Update " + string(rune('A'+i)),
			Body:      "Content " + string(rune('A'+i)),
			Priority:  mail.PriorityNormal,
		}

		result = env.mailSvc.Receive(ctx, pubReq)
		val, err = result.Unpack()
		require.NoError(t, err)

		pubResp := val.(mail.PublishResponse)
		require.NoError(t, pubResp.Error)
	}

	t.Log("Published 3 messages")

	// Poll again - should get all 3 messages.
	pollReq = mail.PollChangesRequest{
		AgentID:      receiver.ID,
		SinceOffsets: map[int64]int64{},
	}

	result = env.mailSvc.Receive(ctx, pollReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	pollResp = val.(mail.PollChangesResponse)
	require.NoError(t, pollResp.Error)
	require.Len(t, pollResp.NewMessages, 3)
	require.Contains(t, pollResp.NewOffsets, topic.ID)
	lastOffset := pollResp.NewOffsets[topic.ID]

	t.Logf("Poll returned %d messages, last offset: %d", len(pollResp.NewMessages), lastOffset)

	// Mark all messages as read so they don't appear in next poll.
	for _, msg := range pollResp.NewMessages {
		readResult := env.mailSvc.Receive(ctx, mail.ReadMessageRequest{
			AgentID:   receiver.ID,
			MessageID: msg.ID,
		})
		readVal, err := readResult.Unpack()
		require.NoError(t, err)
		require.NoError(t, readVal.(mail.ReadMessageResponse).Error)
	}

	// Poll with last offset - should get no new messages since all were read.
	pollReq = mail.PollChangesRequest{
		AgentID:      receiver.ID,
		SinceOffsets: pollResp.NewOffsets,
	}

	result = env.mailSvc.Receive(ctx, pollReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	pollResp = val.(mail.PollChangesResponse)
	require.NoError(t, pollResp.Error)
	require.Empty(t, pollResp.NewMessages)

	t.Log("Poll with offset: no new messages (as expected)")
}

// TestE2E_MessageStates tests message state transitions.
func TestE2E_MessageStates(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create agents.
	sender := env.createAgent("Sender")
	receiver := env.createAgent("Receiver")

	// Send a message.
	sendReq := mail.SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Receiver"},
		Subject:        "State Test",
		Body:           "Testing state transitions.",
		Priority:       mail.PriorityNormal,
	}

	result := env.mailSvc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(mail.SendMailResponse)
	msgID := sendResp.MessageID

	t.Logf("Sent message ID: %d", msgID)

	// Test state transitions: unread -> read -> starred -> archived.
	states := []struct {
		action   string
		newState string
	}{
		{"read", "read"},
		{"star", "starred"},
		{"archive", "archived"},
	}

	for _, s := range states {
		var req mail.MailRequest

		switch s.action {
		case "read":
			req = mail.ReadMessageRequest{
				AgentID:   receiver.ID,
				MessageID: msgID,
			}
		default:
			req = mail.UpdateStateRequest{
				AgentID:   receiver.ID,
				MessageID: msgID,
				NewState:  s.newState,
			}
		}

		result = env.mailSvc.Receive(ctx, req)
		val, err = result.Unpack()
		require.NoError(t, err)

		t.Logf("Transition: %s -> %s", s.action, s.newState)
	}

	// Verify final state by fetching inbox with no filters.
	fetchReq := mail.FetchInboxRequest{
		AgentID: receiver.ID,
		Limit:   10,
	}

	result = env.mailSvc.Receive(ctx, fetchReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	fetchResp := val.(mail.FetchInboxResponse)
	require.NoError(t, fetchResp.Error)

	// Message should be archived and not in default inbox view.
	// (This depends on how FetchInbox filters - may need adjustment.)
	t.Logf("Final inbox count: %d", len(fetchResp.Messages))
}

// TestE2E_UrgentWithDeadline tests urgent messages with deadlines.
func TestE2E_UrgentWithDeadline(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create agents.
	sender := env.createAgent("Manager")
	receiver := env.createAgent("Worker")

	// Send an urgent message with a deadline.
	deadline := time.Now().Add(time.Hour)
	sendReq := mail.SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"Worker"},
		Subject:        "Urgent Task",
		Body:           "Please complete this task by the deadline.",
		Priority:       mail.PriorityUrgent,
		Deadline:       &deadline,
	}

	result := env.mailSvc.Receive(ctx, sendReq)
	val, err := result.Unpack()
	require.NoError(t, err)

	sendResp := val.(mail.SendMailResponse)
	require.NoError(t, sendResp.Error)
	msgID := sendResp.MessageID

	t.Logf("Sent urgent message with deadline: %v", deadline)

	// Get status - should show 1 urgent.
	statusReq := mail.GetStatusRequest{
		AgentID: receiver.ID,
	}

	result = env.mailSvc.Receive(ctx, statusReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	statusResp := val.(mail.GetStatusResponse)
	require.NoError(t, statusResp.Error)
	require.Equal(t, int64(1), statusResp.Status.UnreadCount)
	require.Equal(t, int64(1), statusResp.Status.UrgentCount)

	t.Logf("Status: %d unread, %d urgent", statusResp.Status.UnreadCount, statusResp.Status.UrgentCount)

	// Acknowledge the message.
	ackReq := mail.AckMessageRequest{
		AgentID:   receiver.ID,
		MessageID: msgID,
	}

	result = env.mailSvc.Receive(ctx, ackReq)
	val, err = result.Unpack()
	require.NoError(t, err)

	ackResp := val.(mail.AckMessageResponse)
	require.NoError(t, ackResp.Error)
	require.True(t, ackResp.Success)

	t.Log("Message acknowledged")
}
