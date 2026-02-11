package subtraterpc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// testHarness holds all the components needed for gRPC integration tests.
type testHarness struct {
	t *testing.T

	// Server components.
	storage     store.Storage
	mailSvc     *mail.Service
	agentReg    *agent.Registry
	identityMgr *agent.IdentityManager
	server      *Server
	actorSystem *actor.ActorSystem

	// Client connections.
	conn        *grpc.ClientConn
	mailClient  MailClient
	agentClient AgentClient

	// Cleanup function.
	cleanup func()
}

// newTestHarness creates a new test harness with an embedded gRPC server.
func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	// Create temp directory for test database.
	tmpDir, err := os.MkdirTemp("", "subtrate-grpc-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database with migrations.
	logger := slog.Default()
	sqliteStore, err := db.NewSqliteStore(&db.SqliteConfig{
		DatabaseFileName: dbPath,
	}, logger)
	require.NoError(t, err)

	// Create storage from the sql.DB.
	storage := store.FromDB(sqliteStore.DB())

	// Create services.
	mailSvc := mail.NewServiceWithStore(storage)
	agentReg := agent.NewRegistry(sqliteStore.Store)

	// Use temp directory for identity storage to avoid writing to
	// ~/.subtrate/identities which may not be writable in sandbox.
	identityDir := filepath.Join(tmpDir, "identities")
	identityMgr, err := agent.NewIdentityManager(
		sqliteStore.Store, agentReg,
		agent.WithIdentityDir(identityDir),
	)
	require.NoError(t, err)

	// Create actor system and actors.
	actorSystem := actor.NewActorSystem()

	mailRef := actor.RegisterWithSystem(
		actorSystem,
		"mail-service",
		mail.MailServiceKey,
		mailSvc,
	)

	activitySvc := activity.NewService(activity.ServiceConfig{
		Store: storage,
	})
	activityRef := actor.RegisterWithSystem(
		actorSystem,
		"activity-service",
		activity.ActivityServiceKey,
		activitySvc,
	)

	// Create server config with a random port.
	cfg := ServerConfig{
		ListenAddr:                   "localhost:0", // Random port.
		ServerPingTime:               5 * time.Minute,
		ServerPingTimeout:            1 * time.Minute,
		ClientPingMinWait:            5 * time.Second,
		ClientAllowPingWithoutStream: true,
		MailRef:                      mailRef,
		ActivityRef:                  activityRef,
	}

	// Create heartbeat manager.
	heartbeatMgr := agent.NewHeartbeatManager(agentReg, nil)

	// Create and start server.
	server := NewServer(cfg, sqliteStore.Store, mailSvc, agentReg, identityMgr, heartbeatMgr, nil)
	err = server.Start()
	require.NoError(t, err)

	// Get the actual address the server is listening on.
	addr := server.listener.Addr().String()

	// Create client connection.
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	// Create service clients.
	mailClient := NewMailClient(conn)
	agentClient := NewAgentClient(conn)

	// Create the default "User" agent that the system uses for global inbox.
	ctx := context.Background()
	_, err = agentClient.RegisterAgent(ctx, &RegisterAgentRequest{Name: "User"})
	require.NoError(t, err)

	h := &testHarness{
		t:           t,
		storage:     storage,
		mailSvc:     mailSvc,
		agentReg:    agentReg,
		identityMgr: identityMgr,
		server:      server,
		actorSystem: actorSystem,
		conn:        conn,
		mailClient:  mailClient,
		agentClient: agentClient,
	}

	h.cleanup = func() {
		conn.Close()
		server.Stop()
		actorSystem.Shutdown(context.Background())
		sqliteStore.Close()
		os.RemoveAll(tmpDir)
	}

	return h
}

// Close cleans up the test harness.
func (h *testHarness) Close() {
	h.cleanup()
}

// createTestAgent creates a test agent and returns its ID.
func (h *testHarness) createTestAgent(name string) int64 {
	h.t.Helper()

	ctx := context.Background()
	resp, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: name,
	})
	require.NoError(h.t, err)
	return resp.AgentId
}

// ============================================================================
// Agent Service Tests
// ============================================================================

func TestAgentService_RegisterAgent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Test successful registration.
	resp, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: "TestAgent",
	})
	require.NoError(t, err)
	require.NotZero(t, resp.AgentId)
	require.Equal(t, "TestAgent", resp.Name)

	// Test registration with project key.
	resp, err = h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name:       "ProjectAgent",
		ProjectKey: "test-project",
	})
	require.NoError(t, err)
	require.NotZero(t, resp.AgentId)
}

func TestAgentService_RegisterAgent_EmptyName(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Test empty name error.
	_, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestAgentService_GetAgent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create an agent first.
	regResp, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: "LookupAgent",
	})
	require.NoError(t, err)

	// Test lookup by ID.
	resp, err := h.agentClient.GetAgent(ctx, &GetAgentRequest{
		AgentId: regResp.AgentId,
	})
	require.NoError(t, err)
	require.Equal(t, regResp.AgentId, resp.Id)
	require.Equal(t, "LookupAgent", resp.Name)

	// Test lookup by name.
	resp, err = h.agentClient.GetAgent(ctx, &GetAgentRequest{
		Name: "LookupAgent",
	})
	require.NoError(t, err)
	require.Equal(t, regResp.AgentId, resp.Id)
}

func TestAgentService_GetAgent_NotFound(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Test non-existent ID.
	_, err := h.agentClient.GetAgent(ctx, &GetAgentRequest{
		AgentId: 999999,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	// Test non-existent name.
	_, err = h.agentClient.GetAgent(ctx, &GetAgentRequest{
		Name: "NonExistentAgent",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestAgentService_ListAgents(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create a few agents.
	_, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{Name: "Agent1"})
	require.NoError(t, err)
	_, err = h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{Name: "Agent2"})
	require.NoError(t, err)
	_, err = h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{Name: "Agent3"})
	require.NoError(t, err)

	// List all agents (includes the default "User" agent created by harness).
	resp, err := h.agentClient.ListAgents(ctx, &ListAgentsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 4) // User + Agent1 + Agent2 + Agent3

	// Verify agent names.
	names := make(map[string]bool)
	for _, a := range resp.Agents {
		names[a.Name] = true
	}
	require.True(t, names["User"])
	require.True(t, names["Agent1"])
	require.True(t, names["Agent2"])
	require.True(t, names["Agent3"])
}

func TestAgentService_EnsureIdentity(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Ensure identity for a new session.
	resp, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "test-session-123",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)
	require.NotZero(t, resp.AgentId)
	require.NotEmpty(t, resp.AgentName)

	// Ensure same identity returns same agent.
	resp2, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "test-session-123",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)
	require.Equal(t, resp.AgentId, resp2.AgentId)
	require.Equal(t, resp.AgentName, resp2.AgentName)
}

func TestAgentService_SaveIdentity(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// First ensure an identity exists.
	ensureResp, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "save-test-session",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)

	// Save identity with consumer offsets.
	resp, err := h.agentClient.SaveIdentity(ctx, &SaveIdentityRequest{
		SessionId:       "save-test-session",
		AgentId:         ensureResp.AgentId,
		ConsumerOffsets: map[int64]int64{},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
}

func TestAgentService_RegisterAgent_Duplicate(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Register an agent.
	resp1, err := h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: "DuplicateAgent",
	})
	require.NoError(t, err)
	require.NotZero(t, resp1.AgentId)

	// Register same name again - should error (not idempotent).
	_, err = h.agentClient.RegisterAgent(ctx, &RegisterAgentRequest{
		Name: "DuplicateAgent",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE constraint failed")
}

func TestAgentService_ListAgents_OnlyDefaultUser(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// List agents - only the default "User" agent exists.
	resp, err := h.agentClient.ListAgents(ctx, &ListAgentsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	require.Equal(t, "User", resp.Agents[0].Name)
}

func TestAgentService_EnsureIdentity_DifferentSessions(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Different sessions for same project share the same agent.
	resp1, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "session-alpha",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)
	require.NotZero(t, resp1.AgentId)

	resp2, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "session-beta",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)
	require.NotZero(t, resp2.AgentId)

	// Sessions share the same agent for a project.
	require.Equal(t, resp1.AgentId, resp2.AgentId, "sessions share agent for same project")
}

func TestAgentService_GetAgent_InvalidRequest(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Get agent without specifying ID or name.
	_, err := h.agentClient.GetAgent(ctx, &GetAgentRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent_id or name is required")
}

func TestAgentService_EnsureIdentity_PreservesAcrossSessions(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// First session.
	resp1, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "session-v1",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)

	// Save the identity.
	_, err = h.agentClient.SaveIdentity(ctx, &SaveIdentityRequest{
		SessionId: "session-v1",
		AgentId:   resp1.AgentId,
	})
	require.NoError(t, err)

	// New session for same project should get same agent.
	resp2, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "session-v2",
		ProjectDir: "/test/project",
	})
	require.NoError(t, err)
	require.Equal(t, resp1.AgentId, resp2.AgentId, "same project should reuse agent")
}

// ============================================================================
// Mail Service Tests
// ============================================================================

func TestMailService_SendMail(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create sender and recipient agents.
	senderID := h.createTestAgent("Sender")
	h.createTestAgent("Recipient")

	// Send a message.
	resp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"Recipient"},
		Subject:        "Test Subject",
		Body:           "Test body content",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageId)
	require.NotEmpty(t, resp.ThreadId)
}

func TestMailService_SendMail_Urgent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("UrgentSender")
	h.createTestAgent("UrgentRecipient")

	// Send urgent message with deadline.
	deadline := timestamppb.New(time.Now().Add(1 * time.Hour))
	resp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"UrgentRecipient"},
		Subject:        "Urgent Matter",
		Body:           "This is urgent!",
		Priority:       Priority_PRIORITY_URGENT,
		DeadlineAt:     deadline,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageId)
}

func TestMailService_SendMail_MultipleRecipients(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create sender and multiple recipients.
	senderID := h.createTestAgent("MultiSender")
	recipient1ID := h.createTestAgent("MultiRecipient1")
	recipient2ID := h.createTestAgent("MultiRecipient2")
	recipient3ID := h.createTestAgent("MultiRecipient3")

	// Send message to multiple recipients.
	resp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"MultiRecipient1", "MultiRecipient2", "MultiRecipient3"},
		Subject:        "Multi-Recipient Test",
		Body:           "Message for multiple recipients",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageId)
	require.NotEmpty(t, resp.ThreadId)

	// Verify all recipients received the message.
	for _, recipientID := range []int64{recipient1ID, recipient2ID, recipient3ID} {
		inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
			AgentId: recipientID,
		})
		require.NoError(t, err)
		require.Len(t, inboxResp.Messages, 1)
		require.Equal(t, "Multi-Recipient Test", inboxResp.Messages[0].Subject)
	}
}

func TestMailService_SendMail_ThreadReply(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create two agents.
	agent1ID := h.createTestAgent("ThreadAgent1")
	agent2ID := h.createTestAgent("ThreadAgent2")

	// Agent1 sends initial message to Agent2.
	initialResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       agent1ID,
		RecipientNames: []string{"ThreadAgent2"},
		Subject:        "Thread Conversation",
		Body:           "Initial message in thread",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	threadID := initialResp.ThreadId

	// Agent2 replies within the same thread.
	replyResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       agent2ID,
		RecipientNames: []string{"ThreadAgent1"},
		Subject:        "Re: Thread Conversation",
		Body:           "Reply in thread",
		Priority:       Priority_PRIORITY_NORMAL,
		ThreadId:       threadID,
	})
	require.NoError(t, err)
	require.NotZero(t, replyResp.MessageId)
	require.Equal(t, threadID, replyResp.ThreadId, "reply should use same thread ID")

	// Verify thread contains both messages.
	threadResp, err := h.mailClient.ReadThread(ctx, &ReadThreadRequest{
		ThreadId: threadID,
		AgentId:  agent1ID,
	})
	require.NoError(t, err)
	require.Len(t, threadResp.Messages, 2, "thread should contain 2 messages")

	// Verify message order (oldest first).
	require.Equal(t, "Initial message in thread", threadResp.Messages[0].Body)
	require.Equal(t, "Reply in thread", threadResp.Messages[1].Body)
}

func TestMailService_SendMail_NonExistentRecipient(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("NonExistSender")

	// Send to non-existent recipient.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"NonExistentRecipient"},
		Subject:        "Test",
		Body:           "Message to non-existent recipient",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMailService_SendMail_EmptySubject(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("EmptySubjectSender")
	h.createTestAgent("EmptySubjectRecipient")

	// Send message with empty subject - should fail (subject is required).
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"EmptySubjectRecipient"},
		Subject:        "",
		Body:           "Message with no subject",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "subject is required")
}

func TestMailService_SendMail_EmptyBody(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("EmptyBodySender")
	h.createTestAgent("EmptyBodyRecipient")

	// Send message with empty body - should succeed (body can be empty).
	resp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"EmptyBodyRecipient"},
		Subject:        "Empty Body Test",
		Body:           "",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageId)
}

func TestMailService_SendMail_AllPriorities(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("PrioritySender")
	recipientID := h.createTestAgent("PriorityRecipient")

	priorities := []Priority{
		Priority_PRIORITY_LOW,
		Priority_PRIORITY_NORMAL,
		Priority_PRIORITY_URGENT,
	}

	// Send message at each priority level.
	for _, priority := range priorities {
		resp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
			SenderId:       senderID,
			RecipientNames: []string{"PriorityRecipient"},
			Subject:        "Priority Test",
			Body:           "Testing priority",
			Priority:       priority,
		})
		require.NoError(t, err, "priority %v should succeed", priority)
		require.NotZero(t, resp.MessageId)
	}

	// Verify all messages received.
	inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, inboxResp.Messages, 3)
}

func TestMailService_FetchInbox(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents.
	senderID := h.createTestAgent("InboxSender")
	recipientID := h.createTestAgent("InboxRecipient")

	// Send a message.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"InboxRecipient"},
		Subject:        "Inbox Test",
		Body:           "Test message for inbox",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Fetch inbox.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
	require.Equal(t, "Inbox Test", resp.Messages[0].Subject)
}

func TestMailService_FetchInbox_WithFilters(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents and send messages.
	senderID := h.createTestAgent("FilterSender")
	recipientID := h.createTestAgent("FilterRecipient")

	// Send normal message.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"FilterRecipient"},
		Subject:        "Normal Message",
		Body:           "Normal priority",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Send urgent message.
	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"FilterRecipient"},
		Subject:        "Urgent Message",
		Body:           "Urgent priority",
		Priority:       Priority_PRIORITY_URGENT,
	})
	require.NoError(t, err)

	// Fetch all messages.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 2)

	// Fetch with limit.
	resp, err = h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
		Limit:   1,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
}

func TestMailService_FetchInbox_Empty(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agent with no messages.
	agentID := h.createTestAgent("EmptyInboxAgent")

	// Fetch empty inbox.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: agentID,
	})
	require.NoError(t, err)
	require.Empty(t, resp.Messages)
}

func TestMailService_FetchInbox_UnreadOnly(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents.
	senderID := h.createTestAgent("UnreadSender")
	recipientID := h.createTestAgent("UnreadRecipient")

	// Send two messages.
	msg1, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"UnreadRecipient"},
		Subject:        "Message 1",
		Body:           "First message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"UnreadRecipient"},
		Subject:        "Message 2",
		Body:           "Second message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Both should be unread.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId:    recipientID,
		UnreadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 2)

	// Read the first message.
	_, err = h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: msg1.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)

	// Now only one unread.
	resp, err = h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId:    recipientID,
		UnreadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
	require.Equal(t, "Message 2", resp.Messages[0].Subject)
}

func TestMailService_FetchInbox_WithStateUpdates(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents.
	senderID := h.createTestAgent("StateUpdateSender")
	recipientID := h.createTestAgent("StateUpdateRecipient")

	// Send three messages.
	msg1, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"StateUpdateRecipient"},
		Subject:        "To Archive",
		Body:           "Will be archived",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	msg2, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"StateUpdateRecipient"},
		Subject:        "To Star",
		Body:           "Will be starred",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"StateUpdateRecipient"},
		Subject:        "Normal",
		Body:           "Stay in inbox",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Archive first message.
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: msg1.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_ARCHIVED,
	})
	require.NoError(t, err)

	// Star second message.
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: msg2.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_STARRED,
	})
	require.NoError(t, err)

	// Fetch inbox - archived messages are excluded by default.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 2, "archived messages should be excluded from inbox")

	// Verify states are correctly updated.
	stateMap := make(map[string]MessageState)
	for _, msg := range resp.Messages {
		stateMap[msg.Subject] = msg.State
	}
	require.Equal(t, MessageState_STATE_STARRED, stateMap["To Star"])
	require.Equal(t, MessageState_STATE_UNREAD, stateMap["Normal"])
}

func TestMailService_FetchInbox_MultipleSenders(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create multiple senders and one recipient.
	sender1ID := h.createTestAgent("MultiSender1")
	sender2ID := h.createTestAgent("MultiSender2")
	sender3ID := h.createTestAgent("MultiSender3")
	recipientID := h.createTestAgent("MultiSenderRecipient")

	// Each sender sends a message.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender1ID,
		RecipientNames: []string{"MultiSenderRecipient"},
		Subject:        "From Sender 1",
		Body:           "Message from sender 1",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender2ID,
		RecipientNames: []string{"MultiSenderRecipient"},
		Subject:        "From Sender 2",
		Body:           "Message from sender 2",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender3ID,
		RecipientNames: []string{"MultiSenderRecipient"},
		Subject:        "From Sender 3",
		Body:           "Message from sender 3",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Fetch inbox should have all 3 messages.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 3)

	// Verify different senders.
	senders := make(map[string]bool)
	for _, msg := range resp.Messages {
		senders[msg.SenderName] = true
	}
	require.True(t, senders["MultiSender1"])
	require.True(t, senders["MultiSender2"])
	require.True(t, senders["MultiSender3"])
}

func TestMailService_FetchInbox_DefaultUserAgent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Fetch with agent_id=0 uses the default "User" agent.
	resp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Messages) // User's inbox should be empty.
}

func TestMailService_ReadThread(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents and send message.
	senderID := h.createTestAgent("ThreadSender")
	recipientID := h.createTestAgent("ThreadRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"ThreadRecipient"},
		Subject:        "Thread Test",
		Body:           "Initial message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Read thread.
	resp, err := h.mailClient.ReadThread(ctx, &ReadThreadRequest{
		ThreadId: sendResp.ThreadId,
		AgentId:  recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
	require.Equal(t, "Initial message", resp.Messages[0].Body)
}

func TestMailService_ReadMessage(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents and send message.
	senderID := h.createTestAgent("ReadSender")
	recipientID := h.createTestAgent("ReadRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"ReadRecipient"},
		Subject:        "Read Test",
		Body:           "Mark as read",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Read message - this should mark it as read.
	resp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, "Read Test", resp.Message.Subject)
}

func TestMailService_ReadMessage_MarksAsRead(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("ReadMarkSender")
	recipientID := h.createTestAgent("ReadMarkRecipient")

	// Send message.
	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"ReadMarkRecipient"},
		Subject:        "Mark Read Test",
		Body:           "Test message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Verify message is initially unread.
	inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId:    recipientID,
		UnreadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, inboxResp.Messages, 1)

	// Read the message.
	readResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.NotNil(t, readResp.Message)
	require.NotZero(t, readResp.Message.ReadAt, "ReadAt timestamp should be set")

	// Verify message is no longer in unread list.
	inboxResp, err = h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId:    recipientID,
		UnreadOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, inboxResp.Messages, 0, "message should no longer be unread")
}

func TestMailService_ReadMessage_Idempotent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("IdempotentSender")
	recipientID := h.createTestAgent("IdempotentRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"IdempotentRecipient"},
		Subject:        "Idempotent Test",
		Body:           "Read multiple times",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Read the message multiple times - should not error.
	for i := 0; i < 3; i++ {
		resp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
			MessageId: sendResp.MessageId,
			AgentId:   recipientID,
		})
		require.NoError(t, err, "read %d should succeed", i+1)
		require.Equal(t, "Idempotent Test", resp.Message.Subject)
	}
}

func TestMailService_ReadMessage_InvalidMessageId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("InvalidMsgRecipient")

	// Read non-existent message.
	_, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: 999999,
		AgentId:   recipientID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMailService_ReadMessage_InvalidAgentId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Read with invalid agent ID.
	_, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: 1,
		AgentId:   0,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent_id is required")
}

func TestMailService_UpdateState_Archive(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents and send message.
	senderID := h.createTestAgent("ArchiveSender")
	recipientID := h.createTestAgent("ArchiveRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"ArchiveRecipient"},
		Subject:        "Archive Test",
		Body:           "To be archived",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Archive message.
	resp, err := h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_ARCHIVED,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify message is archived.
	msgResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, MessageState_STATE_ARCHIVED, msgResp.Message.State)
}

func TestMailService_UpdateState_Star(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents and send message.
	senderID := h.createTestAgent("StarSender")
	recipientID := h.createTestAgent("StarRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"StarRecipient"},
		Subject:        "Star Test",
		Body:           "To be starred",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Star message.
	resp, err := h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_STARRED,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify message is starred.
	msgResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, MessageState_STATE_STARRED, msgResp.Message.State)
}

func TestMailService_UpdateState_Trash(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("TrashSender")
	recipientID := h.createTestAgent("TrashRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"TrashRecipient"},
		Subject:        "Trash Test",
		Body:           "To be trashed",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Trash message.
	resp, err := h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_TRASH,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify message is trashed.
	msgResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, MessageState_STATE_TRASH, msgResp.Message.State)

	// Trashed messages should not appear in inbox.
	inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, inboxResp.Messages, 0, "trashed messages should not appear in inbox")
}

func TestMailService_UpdateState_MultipleTransitions(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("TransitionSender")
	recipientID := h.createTestAgent("TransitionRecipient")

	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"TransitionRecipient"},
		Subject:        "Transition Test",
		Body:           "Testing state transitions",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Initial state should be unread.
	msgResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	// After ReadMessage, state becomes read.
	require.Equal(t, MessageState_STATE_READ, msgResp.Message.State)

	// Transition: read -> starred.
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_STARRED,
	})
	require.NoError(t, err)

	// Transition: starred -> archived.
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_ARCHIVED,
	})
	require.NoError(t, err)

	// Transition: archived -> unread (restore).
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_UNREAD,
	})
	require.NoError(t, err)

	// Verify final state.
	msgResp, err = h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	// After ReadMessage it becomes read again.
	require.Equal(t, MessageState_STATE_READ, msgResp.Message.State)
}

func TestMailService_UpdateState_NonExistentMessage(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("NonExistStateRecipient")

	// Update state of non-existent message - succeeds but has no effect.
	// The implementation doesn't validate message existence before update.
	resp, err := h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: 999999,
		AgentId:   recipientID,
		NewState:  MessageState_STATE_ARCHIVED,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
}

func TestMailService_UpdateState_DefaultUserAgent(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create sender and send a message to User (agent_id=0 defaults to
	// "User" which is created by the init migration).
	senderID := h.createTestAgent("Sender")

	// Send a message to User via recipient name.
	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"User"},
		Subject:        "Test message",
		Body:           "Hello User",
	})
	require.NoError(t, err)

	// Update state with agent_id=0 should default to User agent.
	_, err = h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   0,
		NewState:  MessageState_STATE_ARCHIVED,
	})
	require.NoError(t, err)
}

// TestMailService_UpdateState_NoAgentID_ResolvesRecipient verifies that when
// UpdateState is called without an agent_id (as the web UI does in aggregate
// views like CodeReviewer), the server looks up the actual recipient instead
// of defaulting to "User". This is a regression test for a bug where marking
// messages as read in the CodeReviewer view had no effect because the state
// update targeted the "User" agent which was not a recipient.
func TestMailService_UpdateState_NoAgentID_ResolvesRecipient(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create a non-User agent as the recipient (simulating an agent like
	// "AzureHaven" that receives review messages).
	senderID := h.createTestAgent("reviewer-main")
	recipientID := h.createTestAgent("AzureHaven")

	// Send a message to AzureHaven (not "User").
	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"AzureHaven"},
		Subject:        "Review: approve",
		Body:           "LGTM",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Mark as read with agent_id=0 (the CodeReviewer aggregate view path).
	resp, err := h.mailClient.UpdateState(ctx, &UpdateStateRequest{
		MessageId: sendResp.MessageId,
		AgentId:   0,
		NewState:  MessageState_STATE_READ,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify the message is actually marked as read for the real recipient.
	msgResp, err := h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, MessageState_STATE_READ, msgResp.Message.State)
}

func TestMailService_PollChanges(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agent.
	agentID := h.createTestAgent("PollAgent")

	// Poll with no messages should return empty.
	resp, err := h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: agentID,
	})
	require.NoError(t, err)
	require.Empty(t, resp.NewMessages)

	// Create sender and send message.
	senderID := h.createTestAgent("PollSender")
	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"PollAgent"},
		Subject:        "Poll Test",
		Body:           "New message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Poll again should return the new message.
	resp, err = h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: agentID,
	})
	require.NoError(t, err)
	require.Len(t, resp.NewMessages, 1)
}

func TestMailService_PollChanges_MultipleMessages(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("PollMultiRecipient")
	sender1ID := h.createTestAgent("PollMultiSender1")
	sender2ID := h.createTestAgent("PollMultiSender2")

	// Send multiple messages.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender1ID,
		RecipientNames: []string{"PollMultiRecipient"},
		Subject:        "Poll Message 1",
		Body:           "First message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender2ID,
		RecipientNames: []string{"PollMultiRecipient"},
		Subject:        "Poll Message 2",
		Body:           "Second message",
		Priority:       Priority_PRIORITY_URGENT,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender1ID,
		RecipientNames: []string{"PollMultiRecipient"},
		Subject:        "Poll Message 3",
		Body:           "Third message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Poll should return all 3 messages.
	resp, err := h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, resp.NewMessages, 3)

	// Verify messages have different subjects.
	subjects := make(map[string]bool)
	for _, msg := range resp.NewMessages {
		subjects[msg.Subject] = true
	}
	require.True(t, subjects["Poll Message 1"])
	require.True(t, subjects["Poll Message 2"])
	require.True(t, subjects["Poll Message 3"])
}

func TestMailService_PollChanges_AfterRead(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("PollReadRecipient")
	senderID := h.createTestAgent("PollReadSender")

	// Send message.
	sendResp, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"PollReadRecipient"},
		Subject:        "Poll Read Test",
		Body:           "Will be read",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// First poll shows the message.
	pollResp, err := h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, pollResp.NewMessages, 1)

	// Read the message.
	_, err = h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: sendResp.MessageId,
		AgentId:   recipientID,
	})
	require.NoError(t, err)

	// Poll again - read messages should not appear as new.
	pollResp, err = h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Len(t, pollResp.NewMessages, 0, "read messages should not appear in poll")
}

func TestMailService_PollChanges_InvalidAgentId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Poll with invalid agent ID.
	_, err := h.mailClient.PollChanges(ctx, &PollChangesRequest{
		AgentId: 0,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent_id is required")
}

func TestMailService_GetStatus(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agent.
	agentID := h.createTestAgent("StatusAgent")

	// Get status with no messages.
	resp, err := h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: agentID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.UnreadCount)

	// Send a message.
	senderID := h.createTestAgent("StatusSender")
	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       senderID,
		RecipientNames: []string{"StatusAgent"},
		Subject:        "Status Test",
		Body:           "Unread message",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	// Get status should show 1 unread.
	resp, err = h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: agentID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.UnreadCount)
}

func TestMailService_GetStatus_AfterReadingMessages(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("StatusReadRecipient")
	senderID := h.createTestAgent("StatusReadSender")

	// Send 3 messages.
	for i := 1; i <= 3; i++ {
		_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
			SenderId:       senderID,
			RecipientNames: []string{"StatusReadRecipient"},
			Subject:        "Status Message",
			Body:           "Test",
			Priority:       Priority_PRIORITY_NORMAL,
		})
		require.NoError(t, err)
	}

	// Verify 3 unread.
	resp, err := h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), resp.UnreadCount)

	// Read one message (via FetchInbox then ReadMessage).
	inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: recipientID,
		Limit:   1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, inboxResp.Messages)

	_, err = h.mailClient.ReadMessage(ctx, &ReadMessageRequest{
		MessageId: inboxResp.Messages[0].Id,
		AgentId:   recipientID,
	})
	require.NoError(t, err)

	// Verify 2 unread.
	resp, err = h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), resp.UnreadCount)
}

func TestMailService_GetStatus_MultipleSenders(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	recipientID := h.createTestAgent("StatusMultiRecipient")
	sender1ID := h.createTestAgent("StatusMultiSender1")
	sender2ID := h.createTestAgent("StatusMultiSender2")

	// Send messages from different senders.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender1ID,
		RecipientNames: []string{"StatusMultiRecipient"},
		Subject:        "From Sender 1",
		Body:           "Test",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)

	_, err = h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       sender2ID,
		RecipientNames: []string{"StatusMultiRecipient"},
		Subject:        "From Sender 2",
		Body:           "Test",
		Priority:       Priority_PRIORITY_URGENT,
	})
	require.NoError(t, err)

	// Get status should count all unread messages.
	resp, err := h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: recipientID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), resp.UnreadCount)
}

func TestMailService_GetStatus_InvalidAgentId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Get status with invalid agent ID.
	_, err := h.mailClient.GetStatus(ctx, &GetStatusRequest{
		AgentId: 0,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent_id is required")
}

// ============================================================================
// Pub/Sub Tests
// ============================================================================

func TestMailService_Subscribe(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agent.
	agentID := h.createTestAgent("SubscribeAgent")

	// Subscribe to a topic.
	resp, err := h.mailClient.Subscribe(ctx, &SubscribeRequest{
		AgentId:   agentID,
		TopicName: "test-topic",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotZero(t, resp.TopicId)
}

func TestMailService_Subscribe_SameTopic(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	agentID := h.createTestAgent("SameTopicAgent")

	// Subscribe to topic.
	resp1, err := h.mailClient.Subscribe(ctx, &SubscribeRequest{
		AgentId:   agentID,
		TopicName: "same-topic",
	})
	require.NoError(t, err)
	require.True(t, resp1.Success)

	// Subscribe again - should be idempotent.
	resp2, err := h.mailClient.Subscribe(ctx, &SubscribeRequest{
		AgentId:   agentID,
		TopicName: "same-topic",
	})
	require.NoError(t, err)
	require.True(t, resp2.Success)
	require.Equal(t, resp1.TopicId, resp2.TopicId)
}

func TestMailService_Publish(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create sender and subscriber.
	senderID := h.createTestAgent("PublishSender")
	subscriberID := h.createTestAgent("PublishSubscriber")

	// Subscribe to topic.
	_, err := h.mailClient.Subscribe(ctx, &SubscribeRequest{
		AgentId:   subscriberID,
		TopicName: "publish-test-topic",
	})
	require.NoError(t, err)

	// Publish message to topic.
	resp, err := h.mailClient.Publish(ctx, &PublishRequest{
		SenderId:  senderID,
		TopicName: "publish-test-topic",
		Subject:   "Published Message",
		Body:      "This is a published message",
		Priority:  Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	require.NotZero(t, resp.MessageId)
	require.Equal(t, int32(1), resp.RecipientsCount)

	// Verify subscriber received the message.
	inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
		AgentId: subscriberID,
	})
	require.NoError(t, err)
	require.Len(t, inboxResp.Messages, 1)
	require.Equal(t, "Published Message", inboxResp.Messages[0].Subject)
}

func TestMailService_Publish_MultipleSubscribers(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create sender and multiple subscribers.
	senderID := h.createTestAgent("MultiPubSender")
	sub1ID := h.createTestAgent("MultiPubSub1")
	sub2ID := h.createTestAgent("MultiPubSub2")
	sub3ID := h.createTestAgent("MultiPubSub3")

	// Subscribe all to the same topic.
	for _, subName := range []string{"MultiPubSub1", "MultiPubSub2", "MultiPubSub3"} {
		agent, err := h.agentClient.GetAgent(ctx, &GetAgentRequest{Name: subName})
		require.NoError(t, err)
		_, err = h.mailClient.Subscribe(ctx, &SubscribeRequest{
			AgentId:   agent.Id,
			TopicName: "multi-sub-topic",
		})
		require.NoError(t, err)
	}

	// Publish message.
	resp, err := h.mailClient.Publish(ctx, &PublishRequest{
		SenderId:  senderID,
		TopicName: "multi-sub-topic",
		Subject:   "Broadcast Message",
		Body:      "For all subscribers",
		Priority:  Priority_PRIORITY_NORMAL,
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), resp.RecipientsCount)

	// Verify all subscribers received the message.
	for _, subID := range []int64{sub1ID, sub2ID, sub3ID} {
		inboxResp, err := h.mailClient.FetchInbox(ctx, &FetchInboxRequest{
			AgentId: subID,
		})
		require.NoError(t, err)
		require.Len(t, inboxResp.Messages, 1)
		require.Equal(t, "Broadcast Message", inboxResp.Messages[0].Subject)
	}
}

func TestMailService_Publish_NonExistentTopic(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("NoTopicSender")

	// Publish to non-existent topic - should fail.
	_, err := h.mailClient.Publish(ctx, &PublishRequest{
		SenderId:  senderID,
		TopicName: "non-existent-topic",
		Subject:   "Lonely Message",
		Body:      "Nobody will receive this",
		Priority:  Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMailService_Subscribe_InvalidAgentId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	_, err := h.mailClient.Subscribe(ctx, &SubscribeRequest{
		AgentId:   0,
		TopicName: "test-topic",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent_id is required")
}

func TestMailService_Publish_InvalidSenderId(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	_, err := h.mailClient.Publish(ctx, &PublishRequest{
		SenderId:  0,
		TopicName: "test-topic",
		Subject:   "Test",
		Body:      "Test",
		Priority:  Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sender_id is required")
}

// ============================================================================
// Error Path Tests
// ============================================================================

func TestMailService_SendMail_InvalidSender(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Send with invalid sender ID.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId:       0,
		RecipientNames: []string{"Recipient"},
		Subject:        "Test",
		Body:           "Test",
		Priority:       Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sender_id is required")
}

func TestMailService_SendMail_NoRecipients(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	senderID := h.createTestAgent("NoRecipSender")

	// Send with no recipients.
	_, err := h.mailClient.SendMail(ctx, &SendMailRequest{
		SenderId: senderID,
		Subject:  "Test",
		Body:     "Test",
		Priority: Priority_PRIORITY_NORMAL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "recipient_names or topic_name is required")
}

func TestMailService_ReadThread_NotFound(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	agentID := h.createTestAgent("NotFoundAgent")

	// Read non-existent thread - returns empty messages list, not error.
	resp, err := h.mailClient.ReadThread(ctx, &ReadThreadRequest{
		ThreadId: "non-existent-thread-id",
		AgentId:  agentID,
	})
	require.NoError(t, err)
	require.Empty(t, resp.Messages)
}

func TestAgentService_EnsureIdentity_EmptySession(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Ensure identity with empty session ID.
	_, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session_id is required")
}

func TestAgentService_SaveIdentity_EmptySession(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Save identity with empty session ID.
	_, err := h.agentClient.SaveIdentity(ctx, &SaveIdentityRequest{
		SessionId: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session_id is required")
}

func TestAutocompleteRecipients(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create agents with project_key and git_branch via
	// EnsureIdentity.
	_, err := h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "sess-alpha",
		ProjectDir: "/home/user/projects/subtrate",
		GitBranch:  "main",
	})
	require.NoError(t, err)

	_, err = h.agentClient.EnsureIdentity(ctx, &EnsureIdentityRequest{
		SessionId:  "sess-beta",
		ProjectDir: "/home/user/projects/lnd",
		GitBranch:  "fix-channels",
	})
	require.NoError(t, err)

	// Search by name fragment — the generated names are random, so
	// query empty string to list all agents.
	resp, err := h.mailClient.AutocompleteRecipients(
		ctx, &AutocompleteRecipientsRequest{Query: ""},
	)
	require.NoError(t, err)
	// At least "User" + 2 agents from EnsureIdentity.
	require.GreaterOrEqual(t, len(resp.Recipients), 3)

	// Search by project name "subtrate".
	resp, err = h.mailClient.AutocompleteRecipients(
		ctx, &AutocompleteRecipientsRequest{Query: "subtrate"},
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recipients)
	require.Equal(t, "/home/user/projects/subtrate",
		resp.Recipients[0].ProjectKey)

	// Search by git branch "fix-channels".
	resp, err = h.mailClient.AutocompleteRecipients(
		ctx, &AutocompleteRecipientsRequest{
			Query: "fix-channels",
		},
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Recipients)
	require.Equal(t, "fix-channels", resp.Recipients[0].GitBranch)

	// Search with no match returns empty.
	resp, err = h.mailClient.AutocompleteRecipients(
		ctx, &AutocompleteRecipientsRequest{
			Query: "nonexistent-xyz",
		},
	)
	require.NoError(t, err)
	require.Empty(t, resp.Recipients)
}

func TestListAgents_ReturnsProjectAndBranch(t *testing.T) {
	h := newTestHarness(t)
	defer h.Close()

	ctx := context.Background()

	// Create an agent with project_key and git_branch via
	// EnsureIdentity.
	identResp, err := h.agentClient.EnsureIdentity(
		ctx, &EnsureIdentityRequest{
			SessionId:  "sess-ctx",
			ProjectDir: "/home/user/projects/myrepo",
			GitBranch:  "feature-xyz",
		},
	)
	require.NoError(t, err)

	// ListAgents should include the project_key and git_branch.
	listResp, err := h.agentClient.ListAgents(
		ctx, &ListAgentsRequest{},
	)
	require.NoError(t, err)

	var found *GetAgentResponse
	for _, a := range listResp.Agents {
		if a.Id == identResp.AgentId {
			found = a
			break
		}
	}
	require.NotNil(t, found, "agent not found in list")
	require.Equal(t, "/home/user/projects/myrepo", found.ProjectKey)
	require.Equal(t, "feature-xyz", found.GitBranch)
	require.NotNil(t, found.LastActiveAt)
}

// TestSanitizeUTF8 verifies that invalid UTF-8 bytes are replaced with the
// Unicode replacement character, while valid strings pass through unchanged.
func TestSanitizeUTF8(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid ASCII",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "valid unicode",
			input: "hello \u2603 world",
			want:  "hello \u2603 world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "invalid byte in middle",
			input: "hello\x80world",
			want:  "hello\uFFFDworld",
		},
		{
			name:  "invalid bytes at end",
			input: "hello\xff\xfe",
			want:  "hello\uFFFD\uFFFD",
		},
		{
			name:  "mixed valid and invalid",
			input: "ok\xc0\xafok",
			want:  "ok\uFFFD\uFFFDok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeUTF8(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
