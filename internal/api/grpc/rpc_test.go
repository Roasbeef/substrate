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

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
)

// testHarness holds all the components needed for gRPC integration tests.
type testHarness struct {
	t *testing.T

	// Server components.
	store       *db.Store
	mailSvc     *mail.Service
	agentReg    *agent.Registry
	identityMgr *agent.IdentityManager
	server      *Server

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

	store := sqliteStore.Store

	// Create services.
	mailSvc := mail.NewService(store)
	agentReg := agent.NewRegistry(store)
	identityMgr, err := agent.NewIdentityManager(store, agentReg)
	require.NoError(t, err)

	// Create server config with a random port.
	cfg := ServerConfig{
		ListenAddr:                   "localhost:0", // Random port.
		ServerPingTime:               5 * time.Minute,
		ServerPingTimeout:            1 * time.Minute,
		ClientPingMinWait:            5 * time.Second,
		ClientAllowPingWithoutStream: true,
	}

	// Create and start server.
	server := NewServer(cfg, store, mailSvc, agentReg, identityMgr, nil)
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

	h := &testHarness{
		t:           t,
		store:       store,
		mailSvc:     mailSvc,
		agentReg:    agentReg,
		identityMgr: identityMgr,
		server:      server,
		conn:        conn,
		mailClient:  mailClient,
		agentClient: agentClient,
	}

	h.cleanup = func() {
		conn.Close()
		server.Stop()
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

	// List all agents.
	resp, err := h.agentClient.ListAgents(ctx, &ListAgentsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 3)

	// Verify agent names.
	names := make(map[string]bool)
	for _, a := range resp.Agents {
		names[a.Name] = true
	}
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
	deadline := time.Now().Add(1 * time.Hour).Unix()
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
