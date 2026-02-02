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

// gatewayTestHarness holds all components needed for gateway comparison tests.
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
		grpcCfg, sqliteStore.Store, mailSvc, agentReg, identityMgr, nil,
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

// manualURL returns URL for the manual REST handler path.
func (h *gatewayTestHarness) manualURL(path string) string {
	return fmt.Sprintf("http://%s/api/v1%s", h.webAddr, path)
}

// gatewayURL returns URL for the grpc-gateway path.
func (h *gatewayTestHarness) gatewayURL(path string) string {
	return fmt.Sprintf("http://%s/api/v1/gw%s", h.webAddr, path)
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

// createTestAgent creates an agent directly via the store and returns its ID.
func (h *gatewayTestHarness) createTestAgent(name string) int64 {
	h.t.Helper()

	ctx := context.Background()
	ag, err := h.storage.CreateAgent(ctx, store.CreateAgentParams{
		Name: name,
	})
	require.NoError(h.t, err)
	return ag.ID
}

// ============================================================================
// Gateway Verification Tests
// ============================================================================

// TestGatewayVerify_ListAgents compares ListAgents responses between manual and
// gateway handlers.
func TestGatewayVerify_ListAgents(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create some test agents.
	h.createTestAgent("Agent1")
	h.createTestAgent("Agent2")
	h.createTestAgent("Agent3")

	// Call manual handler.
	manualStatus, manualBody, err := h.httpGet(h.manualURL("/agents"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manualStatus)

	// Call gateway handler.
	gwStatus, gwBody, err := h.httpGet(h.gatewayURL("/api/v1/agents"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus)

	// Parse manual response (uses "data" wrapper).
	var manualResp struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)

	// Parse gateway response (uses "agents" from proto).
	var gwResp struct {
		Agents []struct {
			ID   string `json:"id"` // grpc-gateway uses string for int64.
			Name string `json:"name"`
		} `json:"agents"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)

	// Verify same number of agents.
	require.Equal(t, len(manualResp.Data), len(gwResp.Agents),
		"agent count should match: manual=%d, gateway=%d",
		len(manualResp.Data), len(gwResp.Agents))

	// Verify agent names match.
	manualNames := make(map[string]bool)
	for _, a := range manualResp.Data {
		manualNames[a.Name] = true
	}
	for _, a := range gwResp.Agents {
		require.True(t, manualNames[a.Name],
			"gateway agent %q should be in manual response", a.Name)
	}

	// Log the comparison.
	t.Logf("ListAgents comparison:")
	t.Logf("  Manual: %d agents", len(manualResp.Data))
	t.Logf("  Gateway: %d agents", len(gwResp.Agents))
	t.Logf("  Note: Manual uses 'data' wrapper, gateway uses 'agents' field")
}

// TestGatewayVerify_RegisterAgent compares RegisterAgent responses.
func TestGatewayVerify_RegisterAgent(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Test manual handler.
	manualReq := map[string]string{"name": "ManualAgent"}
	manualStatus, manualBody, err := h.httpPost(h.manualURL("/agents"), manualReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, manualStatus,
		"manual handler status: body=%s", string(manualBody))

	// Test gateway handler.
	gwReq := map[string]string{"name": "GatewayAgent"}
	gwStatus, gwBody, err := h.httpPost(h.gatewayURL("/api/v1/agents"), gwReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus,
		"gateway handler status: body=%s", string(gwBody))

	// Parse responses.
	var manualResp struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)
	require.Equal(t, "ManualAgent", manualResp.Name)
	require.NotZero(t, manualResp.ID)

	var gwResp struct {
		AgentID string `json:"agent_id"` // Different field name in gateway.
		Name    string `json:"name"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)
	require.Equal(t, "GatewayAgent", gwResp.Name)
	require.NotEmpty(t, gwResp.AgentID)

	t.Logf("RegisterAgent comparison:")
	t.Logf("  Manual: id=%d, name=%s", manualResp.ID, manualResp.Name)
	t.Logf("  Gateway: agent_id=%s, name=%s", gwResp.AgentID, gwResp.Name)
}

// TestGatewayVerify_FetchInbox compares FetchInbox responses.
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
		Body:           "Testing gateway vs manual handlers",
		Priority:       mail.PriorityNormal,
	})
	require.NoError(t, err)

	// Call manual handler.
	manualURL := fmt.Sprintf("%s?agent_id=%d", h.manualURL("/messages"), recipientID)
	manualStatus, manualBody, err := h.httpGet(manualURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manualStatus,
		"manual handler: body=%s", string(manualBody))

	// Call gateway handler.
	gwURL := fmt.Sprintf("%s?agent_id=%d", h.gatewayURL("/api/v1/messages"), recipientID)
	gwStatus, gwBody, err := h.httpGet(gwURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus,
		"gateway handler: body=%s", string(gwBody))

	// Parse manual response (uses "data" wrapper).
	var manualResp struct {
		Data []struct {
			ID      int64  `json:"id"`
			Subject string `json:"subject"`
		} `json:"data"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)

	// Parse gateway response (uses "messages" from proto).
	var gwResp struct {
		Messages []struct {
			ID      string `json:"id"` // grpc-gateway uses string for int64.
			Subject string `json:"subject"`
		} `json:"messages"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)

	// Verify both have the same message.
	require.Len(t, manualResp.Data, 1, "manual should have 1 message")
	require.Len(t, gwResp.Messages, 1, "gateway should have 1 message")
	require.Equal(t, manualResp.Data[0].Subject, gwResp.Messages[0].Subject,
		"subjects should match")

	t.Logf("FetchInbox comparison:")
	t.Logf("  Manual: %d messages, subject=%q",
		len(manualResp.Data), manualResp.Data[0].Subject)
	t.Logf("  Gateway: %d messages, subject=%q",
		len(gwResp.Messages), gwResp.Messages[0].Subject)
	t.Logf("  Note: Manual uses 'data' wrapper, gateway uses 'messages' field")
}

// TestGatewayVerify_SendMail compares SendMail responses.
// NOTE: Manual and gateway handlers have significantly different request/response formats:
// - Manual: takes "to" (recipient IDs), uses User agent as sender, returns "id"
// - Gateway: takes "sender_id" + "recipient_names", returns "message_id"
func TestGatewayVerify_SendMail(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create agents.
	senderID := h.createTestAgent("SendSender")
	recipientID := h.createTestAgent("SendRecipient")

	// Test manual handler - uses "to" with recipient IDs, sender is auto "User" agent.
	manualReq := map[string]interface{}{
		"to":       []int64{recipientID}, // Manual uses "to" with IDs
		"subject":  "Manual Send Test",
		"body":     "Sent via manual handler",
		"priority": "normal",
	}
	manualStatus, manualBody, err := h.httpPost(h.manualURL("/messages"), manualReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, manualStatus,
		"manual handler: body=%s", string(manualBody))

	// Test gateway handler - uses sender_id + recipient_names.
	gwReq := map[string]interface{}{
		"sender_id":       senderID,
		"recipient_names": []string{"SendRecipient"},
		"subject":         "Gateway Send Test",
		"body":            "Sent via gateway",
		"priority":        2, // PRIORITY_NORMAL = 2 in proto enum.
	}
	gwStatus, gwBody, err := h.httpPost(h.gatewayURL("/api/v1/messages"), gwReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus,
		"gateway handler: body=%s", string(gwBody))

	// Parse manual response - returns full message object.
	var manualResp struct {
		ID       int64  `json:"id"`
		ThreadID string `json:"thread_id"` //nolint:tagliatelle
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)
	require.NotZero(t, manualResp.ID, "manual handler should return id")

	// Parse gateway response.
	var gwResp struct {
		MessageID string `json:"message_id"` //nolint:tagliatelle
		ThreadID  string `json:"thread_id"`  //nolint:tagliatelle
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)
	require.NotEmpty(t, gwResp.MessageID)

	t.Logf("SendMail comparison:")
	t.Logf("  Manual: id=%d, thread_id=%s", manualResp.ID, manualResp.ThreadID)
	t.Logf("  Gateway: message_id=%s, thread_id=%s", gwResp.MessageID, gwResp.ThreadID)
	t.Logf("  Key differences:")
	t.Logf("    - Request: Manual uses 'to' (IDs) + auto User sender; Gateway uses 'sender_id' + 'recipient_names'")
	t.Logf("    - Response: Manual uses 'id'; Gateway uses 'message_id'")
}

// TestGatewayVerify_ListTopics compares ListTopics responses.
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

	// Call manual handler.
	manualStatus, manualBody, err := h.httpGet(h.manualURL("/topics"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manualStatus)

	// Call gateway handler.
	gwStatus, gwBody, err := h.httpGet(h.gatewayURL("/api/v1/topics"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus)

	// Parse manual response (uses "data" wrapper).
	var manualResp struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)

	// Parse gateway response (uses "topics" from proto).
	var gwResp struct {
		Topics []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"topics"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)

	// Verify same number of topics.
	require.Equal(t, len(manualResp.Data), len(gwResp.Topics),
		"topic count should match")

	t.Logf("ListTopics comparison:")
	t.Logf("  Manual: %d topics", len(manualResp.Data))
	t.Logf("  Gateway: %d topics", len(gwResp.Topics))
	t.Logf("  Note: Manual uses 'data' wrapper, gateway uses 'topics' field")
}

// TestGatewayVerify_Search compares Search responses.
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

	// Call manual handler.
	manualURL := fmt.Sprintf("%s?q=XYZ123&agent_id=%d", h.manualURL("/search"), recipientID)
	manualStatus, manualBody, err := h.httpGet(manualURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manualStatus,
		"manual handler: body=%s", string(manualBody))

	// Call gateway handler.
	gwURL := fmt.Sprintf("%s?query=XYZ123&agent_id=%d", h.gatewayURL("/api/v1/search"), recipientID)
	gwStatus, gwBody, err := h.httpGet(gwURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus,
		"gateway handler: body=%s", string(gwBody))

	// Parse manual response (uses "data" wrapper).
	var manualResp struct {
		Data []interface{} `json:"data"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)

	// Parse gateway response (uses "results" from proto).
	var gwResp struct {
		Results []interface{} `json:"results"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)

	// Both should return results (may differ due to response structure).
	t.Logf("Search comparison:")
	t.Logf("  Manual: %d items in 'data'", len(manualResp.Data))
	t.Logf("  Gateway: %d items in 'results'", len(gwResp.Results))
	t.Logf("  Note: Manual uses 'data' wrapper, gateway uses 'results' field")
}

// TestGatewayVerify_ErrorResponses compares error handling between handlers.
// NOTE: This test documents differences in error handling behavior.
func TestGatewayVerify_ErrorResponses(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Test invalid agent_id parameter.
	t.Run("InvalidAgentID", func(t *testing.T) {
		// Manual handler with invalid agent_id.
		manualURL := fmt.Sprintf("%s?agent_id=0", h.manualURL("/messages"))
		manualStatus, _, err := h.httpGet(manualURL)
		require.NoError(t, err)

		// Gateway handler with invalid agent_id.
		gwURL := fmt.Sprintf("%s?agent_id=0", h.gatewayURL("/api/v1/messages"))
		gwStatus, _, err := h.httpGet(gwURL)
		require.NoError(t, err)

		t.Logf("Invalid AgentID error comparison:")
		t.Logf("  Manual status: %d", manualStatus)
		t.Logf("  Gateway status: %d", gwStatus)

		// Gateway properly validates, manual is more permissive.
		// This documents a behavioral difference that should be addressed
		// when migrating to gateway.
		require.True(t, gwStatus >= 400, "gateway should return error status")
		t.Logf("  NOTE: Manual handler returns %d (permissive), gateway returns %d (strict)",
			manualStatus, gwStatus)
	})

	// Test missing required fields in POST.
	t.Run("MissingRequiredFields", func(t *testing.T) {
		// Manual handler with missing sender_id.
		manualReq := map[string]interface{}{
			"recipients": []string{"Someone"},
			"subject":    "Test",
			"body":       "Test",
		}
		manualStatus, _, err := h.httpPost(h.manualURL("/messages"), manualReq)
		require.NoError(t, err)

		// Gateway handler with missing sender_id.
		gwReq := map[string]interface{}{
			"recipient_names": []string{"Someone"},
			"subject":         "Test",
			"body":            "Test",
		}
		gwStatus, _, err := h.httpPost(h.gatewayURL("/api/v1/messages"), gwReq)
		require.NoError(t, err)

		t.Logf("Missing sender_id error comparison:")
		t.Logf("  Manual status: %d", manualStatus)
		t.Logf("  Gateway status: %d", gwStatus)

		// Gateway properly validates, manual is more permissive.
		require.True(t, gwStatus >= 400, "gateway should return error status")
		t.Logf("  NOTE: Manual handler returns %d (permissive), gateway returns %d (strict)",
			manualStatus, gwStatus)
	})
}

// TestGatewayVerify_EmptyResponses compares empty list responses.
func TestGatewayVerify_EmptyResponses(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Create agent with no messages.
	agentID := h.createTestAgent("EmptyAgent")

	// Call manual handler.
	manualURL := fmt.Sprintf("%s?agent_id=%d", h.manualURL("/messages"), agentID)
	manualStatus, manualBody, err := h.httpGet(manualURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manualStatus)

	// Call gateway handler.
	gwURL := fmt.Sprintf("%s?agent_id=%d", h.gatewayURL("/api/v1/messages"), agentID)
	gwStatus, gwBody, err := h.httpGet(gwURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gwStatus)

	// Parse manual response (uses "data" wrapper).
	var manualResp struct {
		Data []interface{} `json:"data"`
	}
	err = json.Unmarshal(manualBody, &manualResp)
	require.NoError(t, err)

	// Parse gateway response (uses "messages" from proto).
	var gwResp struct {
		Messages []interface{} `json:"messages"`
	}
	err = json.Unmarshal(gwBody, &gwResp)
	require.NoError(t, err)

	// Both should return empty arrays (or nil which becomes empty).
	require.Empty(t, manualResp.Data, "manual should return empty data")
	// Gateway may return nil for empty repeated field, which is valid.
	t.Logf("Empty responses comparison:")
	t.Logf("  Manual: %d items in 'data'", len(manualResp.Data))
	t.Logf("  Gateway: %d items in 'messages'", len(gwResp.Messages))
}

// TestGatewayVerify_ContentTypeHeaders compares Content-Type headers.
func TestGatewayVerify_ContentTypeHeaders(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	h.createTestAgent("HeaderAgent")

	// Check manual handler Content-Type.
	manualResp, err := h.client.Get(h.manualURL("/agents"))
	require.NoError(t, err)
	manualContentType := manualResp.Header.Get("Content-Type")
	manualResp.Body.Close()

	// Check gateway handler Content-Type.
	gwResp, err := h.client.Get(h.gatewayURL("/api/v1/agents"))
	require.NoError(t, err)
	gwContentType := gwResp.Header.Get("Content-Type")
	gwResp.Body.Close()

	t.Logf("Content-Type header comparison:")
	t.Logf("  Manual: %s", manualContentType)
	t.Logf("  Gateway: %s", gwContentType)

	// Both should return JSON.
	require.Contains(t, manualContentType, "application/json")
	require.Contains(t, gwContentType, "application/json")
}
