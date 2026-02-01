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
	mailSvc := mail.NewService(storage)

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

	// Create web server.
	cfg := web.DefaultConfig()
	cfg.Addr = addr
	cfg.MailRef = mailRef
	cfg.ActivityRef = activityRef

	server, err := web.NewServer(cfg, dbStore)
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

	env := &httpTestEnv{
		t:           t,
		store:       dbStore,
		server:      server,
		addr:        addr,
		actorSystem: actorSystem,
		mailSvc:     mailSvc,
		agents:      make(map[string]sqlc.Agent),
		topics:      make(map[string]sqlc.Topic),
		client:      &http.Client{Timeout: 5 * time.Second},
	}

	env.cleanups = append(env.cleanups, func() {
		server.Shutdown(context.Background())
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

// post makes a POST request with form data.
func (e *httpTestEnv) post(path string, data string) *http.Response {
	e.t.Helper()
	resp, err := e.client.Post(
		e.baseURL()+path,
		"application/x-www-form-urlencoded",
		strings.NewReader(data),
	)
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

	agent, err := e.store.Queries().CreateAgent(context.Background(), sqlc.CreateAgentParams{
		Name:      name,
		CreatedAt: time.Now().Unix(),
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

// TestHTTP_IndexPage tests that the index page loads.
func TestHTTP_IndexPage(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	resp := env.get("/")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	require.Contains(t, body, "Substrate")
}

// TestHTTP_InboxPage tests the inbox page.
func TestHTTP_InboxPage(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test data.
	env.createAgent("Alice")
	env.createAgent("Bob")
	env.sendMessage("Alice", "Bob", "Hello Bob", "Test message content", mail.PriorityNormal)

	resp := env.get("/inbox")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	require.Contains(t, body, "Inbox")
}

// TestHTTP_InboxMessages tests the inbox messages API endpoint.
func TestHTTP_InboxMessages(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test data.
	env.createAgent("Sender")
	env.createAgent("Receiver")
	env.sendMessage("Sender", "Receiver", "Test Subject", "Test body", mail.PriorityNormal)
	env.sendMessage("Sender", "Receiver", "Urgent Task", "Do this now!", mail.PriorityUrgent)

	// Get inbox messages for receiver.
	receiver := env.agents["Receiver"]
	resp := env.get(fmt.Sprintf("/inbox/messages?agent_id=%d", receiver.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain both messages.
	require.Contains(t, body, "Test Subject")
	require.Contains(t, body, "Urgent Task")

	t.Logf("Inbox messages response: %s", body[:min(200, len(body))])
}

// TestHTTP_AgentsDashboard tests the agents dashboard page.
func TestHTTP_AgentsDashboard(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create some agents.
	env.createAgent("WorkerA")
	env.createAgent("WorkerB")
	env.createAgent("Manager")

	resp := env.get("/agents")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	require.Contains(t, body, "Agents")
}

// TestHTTP_APITopics tests the topics API endpoint.
func TestHTTP_APITopics(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test topics.
	env.createTopic("announcements", "broadcast")
	env.createTopic("direct-channel", "direct")
	env.createTopic("task-queue", "queue")

	resp := env.get("/api/topics")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain topic names.
	require.Contains(t, body, "announcements")
	require.Contains(t, body, "direct-channel")
	require.Contains(t, body, "task-queue")

	t.Logf("Topics response: %s", body[:min(200, len(body))])
}

// TestHTTP_APIStatus tests the status API endpoint.
func TestHTTP_APIStatus(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create test data.
	env.createAgent("StatusTestSender")
	env.createAgent("StatusTestReceiver")
	env.sendMessage("StatusTestSender", "StatusTestReceiver", "Msg 1", "Body 1", mail.PriorityNormal)
	env.sendMessage("StatusTestSender", "StatusTestReceiver", "Msg 2", "Body 2", mail.PriorityUrgent)

	receiver := env.agents["StatusTestReceiver"]
	resp := env.get(fmt.Sprintf("/api/status?agent_id=%d", receiver.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	t.Logf("Status response: %s", body)
}

// TestHTTP_ThreadView tests the thread view endpoint.
func TestHTTP_ThreadView(t *testing.T) {
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
	val, err = result.Unpack()
	require.NoError(t, err)

	// Fetch thread view.
	resp := env.get(fmt.Sprintf("/thread/%s", threadID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)

	// Should contain both messages.
	require.Contains(t, body, "First message in thread")
	require.Contains(t, body, "Reply message")

	t.Logf("Thread view response: %s", body[:min(300, len(body))])
}

// TestHTTP_Heartbeat tests the heartbeat API endpoint.
func TestHTTP_Heartbeat(t *testing.T) {
	env := newHTTPTestEnv(t)
	defer env.cleanup()

	// Create an agent.
	env.createAgent("HeartbeatAgent")

	// Send heartbeat.
	data := map[string]string{"agent_name": "HeartbeatAgent"}
	resp := env.postJSON("/api/heartbeat", data)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	t.Logf("Heartbeat response: %s", body)

	// Check agent status.
	resp = env.get("/api/agents/status")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "HeartbeatAgent")

	t.Logf("Agents status response: %s", body[:min(200, len(body))])
}

// TestHTTP_E2EFlow tests a complete end-to-end flow via HTTP.
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

	// Step 3: Verify inbox page shows messages.
	worker := env.agents["WebWorker"]
	resp := env.get(fmt.Sprintf("/inbox/messages?agent_id=%d", worker.ID))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := readBody(t, resp)
	require.Contains(t, body, "Task Assignment")
	require.Contains(t, body, "Urgent Update")

	t.Log("Step 3: Verified inbox shows both messages")

	// Step 4: Create topics and verify topics API.
	env.createTopic("project-updates", "broadcast")
	env.createTopic("alerts", "queue")

	resp = env.get("/api/topics")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "project-updates")
	require.Contains(t, body, "alerts")

	t.Log("Step 4: Created and verified topics")

	// Step 5: Test heartbeat flow.
	resp = env.postJSON("/api/heartbeat", map[string]string{"agent_name": "WebWorker"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = env.get("/api/agents/status")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body = readBody(t, resp)
	require.Contains(t, body, "WebWorker")

	t.Log("Step 5: Heartbeat flow working")

	// Step 6: Verify pages render without error.
	pages := []string{"/", "/inbox", "/agents"}
	for _, page := range pages {
		resp = env.get(page)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Page %s should return 200", page)
	}

	t.Log("Step 6: All main pages render successfully")

	t.Log("E2E HTTP flow complete!")
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
