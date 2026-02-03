package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/activity"
	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/roasbeef/subtrate/internal/web"
	"github.com/stretchr/testify/require"
)

// httpTestEnv holds the test environment with the web server running.
type httpTestEnv struct {
	t *testing.T

	// Server components.
	store       *db.Store
	server      *web.Server
	grpcServer  *subtraterpc.Server
	addr        string
	actorSystem *actor.ActorSystem

	// Services.
	mailSvc *mail.Service

	// Test data.
	agents map[string]sqlc.Agent
	topics map[string]sqlc.Topic

	// HTTP client.
	client *http.Client

	// Cleanup functions.
	cleanups []func()
}

// newHTTPTestEnv creates a test environment with the web server running.
func newHTTPTestEnv(t *testing.T) *httpTestEnv {
	t.Helper()

	// Create temp directory for test data.
	tmpDir, err := os.MkdirTemp("", "subtrate-http-e2e-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	dbStore, err := db.Open(dbPath)
	require.NoError(t, err)

	// Find and run migrations.
	migrationsDir := findMigrationsDir(t)
	err = db.RunMigrations(dbStore.DB(), migrationsDir)
	require.NoError(t, err)

	// Find a free port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	// Create storage and services.
	storage := store.FromDB(dbStore.DB())
	mailSvc := mail.NewServiceWithStore(storage)

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

	// Create agent registry and heartbeat manager.
	registry := agent.NewRegistry(dbStore)
	heartbeatMgr := agent.NewHeartbeatManager(registry, nil)

	// Create identity manager.
	identityMgr, err := agent.NewIdentityManager(dbStore, registry)
	require.NoError(t, err)

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
		grpcCfg, dbStore, mailSvc, registry, identityMgr,
		heartbeatMgr, nil,
	)
	err = grpcServer.Start()
	require.NoError(t, err)

	// Get the actual gRPC address from the server's listener.
	grpcAddr := grpcServer.Addr()

	// Create web server with gateway enabled.
	cfg := web.DefaultConfig()
	cfg.Addr = addr
	cfg.MailRef = mailRef
	cfg.ActivityRef = activityRef
	cfg.GRPCEndpoint = grpcAddr

	server, err := web.NewServer(cfg, storage, registry)
	require.NoError(t, err)

	// Start server in background.
	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready.
	waitForServer(t, addr)

	// Create the default "User" agent that the system uses for global inbox.
	ctx := context.Background()
	_, err = registry.RegisterAgent(ctx, "User", "", "")
	require.NoError(t, err)

	env := &httpTestEnv{
		t:           t,
		store:       dbStore,
		server:      server,
		grpcServer:  grpcServer,
		addr:        addr,
		actorSystem: actorSystem,
		mailSvc:     mailSvc,
		agents:      make(map[string]sqlc.Agent),
		topics:      make(map[string]sqlc.Topic),
		client:      &http.Client{Timeout: 5 * time.Second},
	}

	env.cleanups = append(env.cleanups, func() {
		server.Shutdown(context.Background())
		grpcServer.Stop()
		actorSystem.Shutdown(context.Background())
		dbStore.Close()
		os.RemoveAll(tmpDir)
	})

	return env
}

// cleanup tears down the test environment.
func (e *httpTestEnv) cleanup() {
	for i := len(e.cleanups) - 1; i >= 0; i-- {
		e.cleanups[i]()
	}
}

// baseURL returns the base URL for the test server.
func (e *httpTestEnv) baseURL() string {
	return "http://" + e.addr
}

// get makes a GET request to the given path.
func (e *httpTestEnv) get(path string) *http.Response {
	e.t.Helper()
	resp, err := e.client.Get(e.baseURL() + path)
	require.NoError(e.t, err)
	return resp
}

// postJSON makes a POST request with JSON data.
func (e *httpTestEnv) postJSON(path string, data any) *http.Response {
	e.t.Helper()
	body, err := json.Marshal(data)
	require.NoError(e.t, err)
	resp, err := e.client.Post(
		e.baseURL()+path,
		"application/json",
		strings.NewReader(string(body)),
	)
	require.NoError(e.t, err)
	return resp
}

// readBody reads and returns the response body.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// createAgent creates a test agent directly in the database.
func (e *httpTestEnv) createAgent(name string) sqlc.Agent {
	e.t.Helper()

	now := time.Now().Unix()
	agent, err := e.store.Queries().CreateAgent(context.Background(), sqlc.CreateAgentParams{
		Name:         name,
		CreatedAt:    now,
		LastActiveAt: now,
	})
	require.NoError(e.t, err)

	e.agents[name] = agent
	return agent
}

// createTopic creates a test topic directly in the database.
func (e *httpTestEnv) createTopic(name, topicType string) sqlc.Topic {
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

// sendMessage sends a message using the mail service.
func (e *httpTestEnv) sendMessage(senderName, recipientName, subject, body string, priority mail.Priority) int64 {
	e.t.Helper()

	sender := e.agents[senderName]
	ctx := context.Background()

	req := mail.SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{recipientName},
		Subject:        subject,
		Body:           body,
		Priority:       priority,
	}

	result := e.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	require.NoError(e.t, err)

	resp := val.(mail.SendMailResponse)
	require.NoError(e.t, resp.Error)

	return resp.MessageID
}

// waitForServer waits until the server is responding.
func waitForServer(t *testing.T, addr string) {
	t.Helper()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("Server did not start in time")
}

// TestHTTP_IndexPage tests that the React SPA shell is served.
func TestHTTP_IndexPage(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	resp := env.get("/")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	// React SPA shell should contain the root div and JS/CSS assets.
	require.Contains(t, body, `<div id="root">`)
	require.Contains(t, body, "assets/js")
}

// TestHTTP_InboxPage tests that the SPA is served for client-side routes.
func TestHTTP_InboxPage(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// SPA routing: /inbox should serve the React shell which handles routing.
	resp := env.get("/inbox")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	// React SPA shell should be served for all routes.
	require.Contains(t, body, `<div id="root">`)
}

// TestHTTP_APIV1Messages tests the API v1 messages endpoint.
func TestHTTP_APIV1Messages(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test data.
	env.createAgent("Sender")
	env.createAgent("Receiver")
	env.sendMessage("Sender", "Receiver", "Test Subject", "Test body", mail.PriorityNormal)
	env.sendMessage("Sender", "Receiver", "Urgent Task", "Do this now!", mail.PriorityUrgent)

	// Get inbox messages via API v1 for the Receiver agent.
	// Without agent_id, it defaults to User's inbox which is empty.
	receiver := env.agents["Receiver"]
	resp := env.get(fmt.Sprintf("/api/v1/messages?agent_id=%d", receiver.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain both messages in JSON response.
	require.Contains(t, body, "Test Subject")
	require.Contains(t, body, "Urgent Task")

	t.Logf("API v1 messages response: %s", body[:min(300, len(body))])
}

// TestHTTP_APIV1Agents tests the API v1 agents endpoint.
func TestHTTP_APIV1Agents(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create some agents.
	env.createAgent("WorkerA")
	env.createAgent("WorkerB")
	env.createAgent("Manager")

	resp := env.get("/api/v1/agents")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	// Should contain agent names in JSON response.
	require.Contains(t, body, "WorkerA")
	require.Contains(t, body, "WorkerB")
	require.Contains(t, body, "Manager")
}

// TestHTTP_APIV1Topics tests the API v1 topics endpoint.
func TestHTTP_APIV1Topics(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test topics.
	env.createTopic("announcements", "broadcast")
	env.createTopic("direct-channel", "direct")
	env.createTopic("task-queue", "queue")

	resp := env.get("/api/v1/topics")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain topic names in JSON response.
	require.Contains(t, body, "announcements")
	require.Contains(t, body, "direct-channel")
	require.Contains(t, body, "task-queue")

	t.Logf("API v1 topics response: %s", body[:min(200, len(body))])
}

// TestHTTP_APIV1AgentStatus tests the API v1 agent status endpoint.
func TestHTTP_APIV1AgentStatus(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test agents.
	env.createAgent("StatusTestSender")
	env.createAgent("StatusTestReceiver")

	// Note: The gateway route is /api/v1/agents-status (with hyphen, not slash)
	// to avoid route conflict with /api/v1/agents/{id}.
	resp := env.get("/api/v1/agents-status")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	// Should contain agent names in status response.
	require.Contains(t, body, "StatusTestSender")
	require.Contains(t, body, "StatusTestReceiver")

	t.Logf("API v1 agent status response: %s", body)
}

// TestHTTP_APIV1MessagesInThread tests that messages in a thread can be
// retrieved via the messages API.
func TestHTTP_APIV1MessagesInThread(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test data with a thread.
	env.createAgent("ThreadSender")
	env.createAgent("ThreadReceiver")

	// Send initial message.
	ctx := context.Background()
	sender := env.agents["ThreadSender"]

	req := mail.SendMailRequest{
		SenderID:       sender.ID,
		RecipientNames: []string{"ThreadReceiver"},
		Subject:        "Thread Test",
		Body:           "First message in thread",
		Priority:       mail.PriorityNormal,
	}

	result := env.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	require.NoError(t, err)
	sendResp := val.(mail.SendMailResponse)
	threadID := sendResp.ThreadID

	// Reply in the same thread.
	receiver := env.agents["ThreadReceiver"]
	replyReq := mail.SendMailRequest{
		SenderID:       receiver.ID,
		RecipientNames: []string{"ThreadSender"},
		Subject:        "Re: Thread Test",
		Body:           "Reply message",
		Priority:       mail.PriorityNormal,
		ThreadID:       threadID,
	}

	result = env.mailSvc.Receive(ctx, replyReq)
	_, err = result.Unpack()
	require.NoError(t, err)

	// Fetch messages via API v1 - messages in same thread share thread_id.
	// Query for ThreadReceiver's inbox to see the messages they received.
	threadReceiverAgent := env.agents["ThreadReceiver"]
	resp := env.get(fmt.Sprintf("/api/v1/messages?agent_id=%d", threadReceiverAgent.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain the first message and thread ID.
	require.Contains(t, body, "First message in thread")
	require.Contains(t, body, threadID)

	// Also check ThreadSender's inbox for the reply.
	threadSenderAgent := env.agents["ThreadSender"]
	resp2 := env.get(fmt.Sprintf("/api/v1/messages?agent_id=%d", threadSenderAgent.ID))
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	body2 := readBody(t, resp2)
	require.Contains(t, body2, "Reply message")

	t.Logf("API v1 messages with thread response: %s", body[:min(400, len(body))])
}

// TestHTTP_APIV1Heartbeat tests the API v1 heartbeat endpoint.
func TestHTTP_APIV1Heartbeat(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create an agent.
	agent := env.createAgent("HeartbeatAgent")

	// Send heartbeat via API v1 using the gateway endpoint.
	// The gRPC Heartbeat expects agent_id (int64), not agent_name.
	data := map[string]any{
		"agent_id":   agent.ID,
		"session_id": "test-session",
	}
	resp := env.postJSON("/api/v1/heartbeat", data)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the heartbeat response.
	body := readBody(t, resp)
	require.Contains(t, body, "success")

	// Check agent status via API v1 (using agents-status with hyphen).
	resp = env.get("/api/v1/agents-status")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "HeartbeatAgent")

	t.Logf("Agents status response: %s", body[:min(200, len(body))])
}

// TestHTTP_E2EFlow tests a complete end-to-end flow via HTTP using API v1.
func TestHTTP_E2EFlow(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Step 1: Create agents.
	env.createAgent("WebUser")
	env.createAgent("WebWorker")

	t.Log("Step 1: Created agents WebUser and WebWorker")

	// Step 2: Send messages.
	env.sendMessage("WebUser", "WebWorker", "Task Assignment", "Please complete this task", mail.PriorityNormal)
	env.sendMessage("WebUser", "WebWorker", "Urgent Update", "Critical issue detected!", mail.PriorityUrgent)

	t.Log("Step 2: Sent 2 messages from WebUser to WebWorker")

	// Step 3: Verify API v1 messages endpoint returns messages for WebWorker.
	webWorkerAgent := env.agents["WebWorker"]
	resp := env.get(fmt.Sprintf("/api/v1/messages?agent_id=%d", webWorkerAgent.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	require.Contains(t, body, "Task Assignment")
	require.Contains(t, body, "Urgent Update")

	t.Log("Step 3: Verified API v1 messages returns both messages")

	// Step 4: Create topics and verify topics API v1.
	env.createTopic("project-updates", "broadcast")
	env.createTopic("alerts", "queue")

	resp = env.get("/api/v1/topics")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "project-updates")
	require.Contains(t, body, "alerts")

	t.Log("Step 4: Created and verified topics via API v1")

	// Step 5: Test heartbeat flow via API v1.
	resp = env.postJSON("/api/v1/heartbeat", map[string]any{
		"agent_id":   webWorkerAgent.ID,
		"session_id": "test-session",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = env.get("/api/v1/agents-status")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "WebWorker")

	t.Log("Step 5: Heartbeat flow working via API v1")

	// Step 6: Verify SPA shell is served for all routes.
	pages := []string{"/", "/inbox", "/agents"}
	for _, page := range pages {
		resp = env.get(page)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Page %s should return 200", page)
		body = readBody(t, resp)
		require.Contains(t, body, `<div id="root">`, "Page %s should serve React shell", page)
	}

	t.Log("Step 6: SPA shell served for all routes")

	t.Log("E2E HTTP flow complete!")
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
