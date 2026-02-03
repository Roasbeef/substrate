package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// gatewayTestHarness holds all components needed for gateway tests.
type gatewayTestHarness struct {
	t *testing.T

	// Server components.
	storage     store.Storage
	mailSvc     *mail.Service
	agentReg    *agent.Registry
	identityMgr *agent.IdentityManager
	webServer   *Server
	httpServer  *httptest.Server
	grpcServer  *subtraterpc.Server
	actorSystem *actor.ActorSystem

	// Server addresses.
	webAddr  string
	grpcAddr string

	// HTTP client.
	client *http.Client

	// Cleanup function.
	cleanup func()
}

// newGatewayTestHarness creates a test harness with both gRPC and web servers.
func newGatewayTestHarness(t *testing.T) *gatewayTestHarness {
	t.Helper()

	// Create temp directory for test database.
	tmpDir, err := os.MkdirTemp("", "subtrate-gateway-test-*")
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
	heartbeatMgr := agent.NewHeartbeatManager(agentReg, nil)
	identityMgr, err := agent.NewIdentityManager(sqliteStore.Store, agentReg)
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

	// Create gRPC server config with a random port.
	grpcCfg := subtraterpc.ServerConfig{
		ListenAddr:                   "localhost:0", // Random port.
		ServerPingTime:               5 * time.Minute,
		ServerPingTimeout:            1 * time.Minute,
		ClientPingMinWait:            5 * time.Second,
		ClientAllowPingWithoutStream: true,
		MailRef:                      mailRef,
		ActivityRef:                  activityRef,
	}

	// Create and start gRPC server.
	grpcServer := subtraterpc.NewServer(
		grpcCfg, sqliteStore.Store, mailSvc, agentReg, identityMgr,
		heartbeatMgr, nil,
	)
	err = grpcServer.Start()
	require.NoError(t, err)

	// Get the actual gRPC address from the server's listener.
	grpcAddr := grpcServer.Addr()

	// Create web server config with grpc-gateway enabled.
	webCfg := &Config{
		Addr:         "localhost:0", // Will be ignored - we use httptest.
		MailRef:      mailRef,
		ActivityRef:  activityRef,
		GRPCEndpoint: grpcAddr,
	}

	// Create web server (but don't call Start - we'll use httptest).
	webServer, err := NewServer(webCfg, storage, agentReg)
	require.NoError(t, err)

	// Create httptest server wrapping the web server's mux.
	httpServer := httptest.NewServer(webServer.mux)
	webAddr := httpServer.Listener.Addr().String()

	h := &gatewayTestHarness{
		t:           t,
		storage:     storage,
		mailSvc:     mailSvc,
		agentReg:    agentReg,
		identityMgr: identityMgr,
		webServer:   webServer,
		httpServer:  httpServer,
		grpcServer:  grpcServer,
		actorSystem: actorSystem,
		webAddr:     webAddr,
		grpcAddr:    grpcAddr,
		client:      &http.Client{Timeout: 10 * time.Second},
	}

	h.cleanup = func() {
		httpServer.Close()
		webServer.Shutdown(context.Background())
		grpcServer.Stop()
		actorSystem.Shutdown(context.Background())
		sqliteStore.Close()
		os.RemoveAll(tmpDir)
	}

	return h
}

// Close cleans up the test harness.
func (h *gatewayTestHarness) Close() {
	h.cleanup()
}

// apiURL returns the full URL for an API endpoint.
func (h *gatewayTestHarness) apiURL(path string) string {
	return fmt.Sprintf("http://%s%s", h.webAddr, path)
}

// httpGet performs a GET request and returns the response body.
func (h *gatewayTestHarness) httpGet(url string) (int, []byte, error) {
	resp, err := h.client.Get(url)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}

// httpPost performs a POST request with JSON body and returns the response.
func (h *gatewayTestHarness) httpPost(url string, body interface{}) (int, []byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}

	resp, err := h.client.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, respBody, nil
}

// createTestAgent creates a test agent via the gRPC API and returns its ID.
func (h *gatewayTestHarness) createTestAgent(name string) int64 {
	h.t.Helper()

	req := map[string]string{"name": name}
	status, body, err := h.httpPost(h.apiURL("/api/v1/agents"), req)
	require.NoError(h.t, err)
	require.Equal(h.t, http.StatusOK, status,
		"failed to create agent: %s", string(body))

	var resp struct {
		AgentID string `json:"agent_id"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(h.t, err)

	// Parse agent ID from string.
	var id int64
	fmt.Sscanf(resp.AgentID, "%d", &id)
	return id
}

// TestGatewayVerify_ListAgents verifies the grpc-gateway ListAgents endpoint.
func TestGatewayVerify_ListAgents(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create some test agents.
	h.createTestAgent("Agent1")
	h.createTestAgent("Agent2")
	h.createTestAgent("Agent3")

	// Call gateway handler.
	status, body, err := h.httpGet(h.apiURL("/api/v1/agents"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Parse response.
	var resp struct {
		Agents []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"agents"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	// Verify we have 3 agents.
	require.Len(t, resp.Agents, 3, "expected 3 agents")

	// Verify agent names.
	names := make(map[string]bool)
	for _, a := range resp.Agents {
		names[a.Name] = true
	}
	require.True(t, names["Agent1"], "should have Agent1")
	require.True(t, names["Agent2"], "should have Agent2")
	require.True(t, names["Agent3"], "should have Agent3")

	t.Logf("ListAgents: %d agents", len(resp.Agents))
}

// TestGatewayVerify_RegisterAgent verifies agent registration.
func TestGatewayVerify_RegisterAgent(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	req := map[string]string{"name": "NewAgent"}
	status, body, err := h.httpPost(h.apiURL("/api/v1/agents"), req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp struct {
		AgentID string `json:"agent_id"`
		Name    string `json:"name"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.Equal(t, "NewAgent", resp.Name)
	require.NotEmpty(t, resp.AgentID)

	t.Logf("RegisterAgent: agent_id=%s, name=%s", resp.AgentID, resp.Name)
}

// TestGatewayVerify_FetchInbox verifies fetching inbox messages.
func TestGatewayVerify_FetchInbox(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create sender and recipient.
	senderID := h.createTestAgent("InboxSender")
	recipientID := h.createTestAgent("InboxRecipient")

	// Send a message via the mail service.
	ctx := context.Background()
	_, err := h.mailSvc.Send(ctx, mail.SendMailRequest{
		SenderID:       senderID,
		RecipientNames: []string{"InboxRecipient"},
		Subject:        "Gateway Test Message",
		Body:           "Testing gateway endpoint",
		Priority:       mail.PriorityNormal,
	})
	require.NoError(t, err)

	// Fetch inbox.
	url := fmt.Sprintf("%s?agent_id=%d", h.apiURL("/api/v1/messages"), recipientID)
	status, body, err := h.httpGet(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp struct {
		Messages []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
		} `json:"messages"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	require.Len(t, resp.Messages, 1, "should have 1 message")
	require.Equal(t, "Gateway Test Message", resp.Messages[0].Subject)

	t.Logf("FetchInbox: %d messages", len(resp.Messages))
}

// TestGatewayVerify_SendMail verifies sending messages via gateway.
func TestGatewayVerify_SendMail(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create agents.
	senderID := h.createTestAgent("SendSender")
	h.createTestAgent("SendRecipient")

	// Send message via gateway.
	req := map[string]interface{}{
		"sender_id":       senderID,
		"recipient_names": []string{"SendRecipient"},
		"subject":         "Gateway Send Test",
		"body":            "Sent via gateway",
		"priority":        2, // PRIORITY_NORMAL = 2 in proto enum.
	}
	status, body, err := h.httpPost(h.apiURL("/api/v1/messages"), req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp struct {
		MessageID string `json:"message_id"`
		ThreadID  string `json:"thread_id"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.MessageID)
	require.NotEmpty(t, resp.ThreadID)

	t.Logf("SendMail: message_id=%s, thread_id=%s", resp.MessageID, resp.ThreadID)
}

// TestGatewayVerify_ListTopics verifies listing topics.
func TestGatewayVerify_ListTopics(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create an agent and subscribe to some topics.
	agentID := h.createTestAgent("TopicAgent")

	ctx := context.Background()
	_, err := h.mailSvc.Subscribe(ctx, agentID, "topic-alpha")
	require.NoError(t, err)
	_, err = h.mailSvc.Subscribe(ctx, agentID, "topic-beta")
	require.NoError(t, err)

	// List topics.
	status, body, err := h.httpGet(h.apiURL("/api/v1/topics"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	var resp struct {
		Topics []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"topics"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	// Agent gets a personal inbox topic plus the two we subscribed to.
	require.GreaterOrEqual(t, len(resp.Topics), 2, "should have at least 2 topics")

	// Verify our topics are included.
	topicNames := make(map[string]bool)
	for _, topic := range resp.Topics {
		topicNames[topic.Name] = true
	}
	require.True(t, topicNames["topic-alpha"], "should have topic-alpha")
	require.True(t, topicNames["topic-beta"], "should have topic-beta")

	t.Logf("ListTopics: %d topics", len(resp.Topics))
}

// TestGatewayVerify_Search verifies search functionality.
func TestGatewayVerify_Search(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create agents and send a searchable message.
	senderID := h.createTestAgent("SearchSender")
	recipientID := h.createTestAgent("SearchRecipient")

	ctx := context.Background()
	_, err := h.mailSvc.Send(ctx, mail.SendMailRequest{
		SenderID:       senderID,
		RecipientNames: []string{"SearchRecipient"},
		Subject:        "Unique Searchable Keyword",
		Body:           "This message contains the special search term XYZ123",
		Priority:       mail.PriorityNormal,
	})
	require.NoError(t, err)

	// Wait a moment for FTS indexing.
	time.Sleep(50 * time.Millisecond)

	// Search via gateway.
	url := fmt.Sprintf("%s?query=XYZ123&agent_id=%d", h.apiURL("/api/v1/search"), recipientID)
	status, body, err := h.httpGet(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp struct {
		Results []interface{} `json:"results"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(resp.Results), 1, "should have search results")

	t.Logf("Search: %d results", len(resp.Results))
}

// TestGatewayVerify_EmptyResponses verifies empty list responses.
func TestGatewayVerify_EmptyResponses(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create agent with no messages.
	agentID := h.createTestAgent("EmptyAgent")

	// Fetch empty inbox.
	url := fmt.Sprintf("%s?agent_id=%d", h.apiURL("/api/v1/messages"), agentID)
	status, body, err := h.httpGet(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	var resp struct {
		Messages []interface{} `json:"messages"`
	}
	err = json.Unmarshal(body, &resp)
	require.NoError(t, err)

	// Gateway should return empty array (not null).
	require.NotNil(t, resp.Messages, "messages should be non-nil")
	require.Empty(t, resp.Messages, "messages should be empty")

	t.Logf("Empty response: messages=%v", resp.Messages)
}

// TestGatewayVerify_ContentTypeHeaders verifies content-type headers.
func TestGatewayVerify_ContentTypeHeaders(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	h.createTestAgent("HeaderAgent")

	resp, err := h.client.Get(h.apiURL("/api/v1/agents"))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	require.Contains(t, contentType, "application/json",
		"response should be JSON: got %s", contentType)

	t.Logf("Content-Type: %s", contentType)
}
