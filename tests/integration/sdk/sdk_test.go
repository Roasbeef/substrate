package sdk_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// testEnv holds the test environment configuration.
type testEnv struct {
	// dbPath is the path to the test database.
	dbPath string

	// store is the database store.
	store *db.Store

	// spawner is the agent spawner.
	spawner *agent.Spawner

	// cleanup functions to run after test.
	cleanups []func()
}

// setup creates a test environment with a temporary database.
func setup(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory for test data.
	tmpDir, err := os.MkdirTemp("", "subtrate-sdk-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	store, err := db.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations.
	migrationsDir := filepath.Join(
		os.Getenv("GOPATH"), "src", "github.com", "roasbeef",
		"subtrate", "internal", "db", "migrations",
	)
	if err := db.RunMigrations(store.DB(), migrationsDir); err != nil {
		store.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create spawner with test configuration.
	spawner := agent.NewSpawner(&agent.SpawnConfig{
		CLIPath:              "claude",
		Model:                "claude-sonnet-4-5-20250929",
		Timeout:              2 * time.Minute,
		NoSessionPersistence: true,
	})

	env := &testEnv{
		dbPath:  dbPath,
		store:   store,
		spawner: spawner,
	}

	env.cleanups = append(env.cleanups, func() {
		store.Close()
		os.RemoveAll(tmpDir)
	})

	return env
}

// cleanup runs all cleanup functions.
func (e *testEnv) cleanup() {
	for i := len(e.cleanups) - 1; i >= 0; i-- {
		e.cleanups[i]()
	}
}

// skipIfNoCLI skips the test if the claude CLI is not available.
func skipIfNoCLI(t *testing.T) {
	t.Helper()

	_, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not found, skipping SDK integration test")
	}
}

// skipIfNoAuth skips the test if Claude CLI is not authenticated.
// Claude CLI can authenticate via ANTHROPIC_API_KEY or OAuth token.
func skipIfNoAuth(t *testing.T) {
	t.Helper()

	// Check for API key.
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return
	}

	// Check for OAuth token.
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		return
	}

	// Try running claude --version to see if it's authenticated.
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		t.Skip("Claude CLI not authenticated, skipping SDK integration test")
	}
}

// TestSDK_ClientCreation tests that the SDK client can be created.
func TestSDK_ClientCreation(t *testing.T) {
	skipIfNoCLI(t)

	opts := []claudeagent.Option{
		claudeagent.WithModel("claude-sonnet-4-5-20250929"),
		claudeagent.WithNoSessionPersistence(),
	}

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Client creation should succeed even without API key.
	// Connection will fail without it.
}

// TestSDK_SpawnerConfig tests that the spawner configuration works correctly.
func TestSDK_SpawnerConfig(t *testing.T) {
	cfg := &agent.SpawnConfig{
		CLIPath:              "/custom/path/claude",
		Model:                "claude-opus-4-5-20250929",
		WorkDir:              "/tmp/test-work",
		SystemPrompt:         "You are a test agent.",
		MaxTurns:             5,
		PermissionMode:       claudeagent.PermissionModeAcceptEdits,
		NoSessionPersistence: true,
	}

	spawner := agent.NewSpawner(cfg)

	// Verify spawner was created with config.
	if spawner == nil {
		t.Fatal("spawner is nil")
	}

	// Verify no processes initially.
	procs := spawner.ListProcesses()
	if len(procs) != 0 {
		t.Errorf("expected 0 processes, got %d", len(procs))
	}
}

// TestSDK_ProcessTracking tests that process tracking works correctly.
func TestSDK_ProcessTracking(t *testing.T) {
	spawner := agent.NewSpawner(nil)

	// Initially no processes.
	procs := spawner.ListProcesses()
	if len(procs) != 0 {
		t.Fatalf("expected 0 processes initially, got %d", len(procs))
	}

	// Get non-existent process.
	proc := spawner.GetProcess("nonexistent")
	if proc != nil {
		t.Error("expected nil for nonexistent process")
	}
}

// TestSDK_CLIIntegration tests using the SDK to invoke CLI commands.
// This test requires the claude CLI and API key to be available.
func TestSDK_CLIIntegration(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setup(t)
	defer env.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a spawner with restricted permissions for safety.
	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:                "sonnet", // Use cheaper model.
		MaxTurns:             1,
		NoSessionPersistence: true,
		PermissionMode:       claudeagent.PermissionModeDefault,
	})

	// Spawn an agent with a simple prompt that uses substrate CLI.
	resp, err := spawner.Spawn(ctx,
		"Run 'substrate --help' and tell me what commands are available. "+
			"Only respond with a brief list of the main commands.",
	)
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	if resp.IsError {
		t.Errorf("agent returned error: %s", resp.Error)
	}

	// Verify we got a response.
	if resp.Result == "" {
		t.Error("expected non-empty result")
	}

	t.Logf("Agent response: %s", resp.Result)
	t.Logf("Session ID: %s", resp.SessionID)
	t.Logf("Cost: $%.4f", resp.CostUSD)
	t.Logf("Duration: %dms", resp.DurationMS)
}

// TestSDK_MCPToolIntegration tests that the SDK can use MCP tools.
// This test requires substrated to be running with MCP enabled.
func TestSDK_MCPToolIntegration(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if substrated MCP config exists.
	home, _ := os.UserHomeDir()
	mcpConfigPath := filepath.Join(home, ".claude", "claude_mcp_config.json")
	if _, err := os.Stat(mcpConfigPath); os.IsNotExist(err) {
		t.Skip("MCP config not found, skipping MCP integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:                "sonnet",
		MaxTurns:             3,
		NoSessionPersistence: true,
	})

	// Ask agent to use the MCP tools to check inbox.
	resp, err := spawner.Spawn(ctx,
		"Use the subtrate MCP tools to fetch the inbox. "+
			"Report how many messages you found.",
	)
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	if resp.IsError {
		t.Logf("Agent error (may be expected if MCP not configured): %s", resp.Error)
	}

	t.Logf("Agent response: %s", resp.Result)
}

// TestSDK_SessionResume tests that session resume works correctly.
func TestSDK_SessionResume(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:    "haiku",
		MaxTurns: 1,
		// Note: NOT setting NoSessionPersistence so session is saved.
	})

	// First spawn to establish session.
	resp1, err := spawner.Spawn(ctx, "Remember the number 42. Just acknowledge.")
	if err != nil {
		t.Fatalf("first spawn failed: %v", err)
	}

	sessionID := resp1.SessionID
	if sessionID == "" {
		t.Fatal("expected session ID from first spawn")
	}

	t.Logf("First session ID: %s", sessionID)

	// Resume session and ask about the number.
	resp2, err := spawner.SpawnWithResume(ctx, sessionID,
		"What number did I ask you to remember?",
	)
	if err != nil {
		t.Fatalf("resume spawn failed: %v", err)
	}

	// Check if response mentions 42.
	if !strings.Contains(resp2.Result, "42") {
		t.Errorf("expected response to mention 42, got: %s", resp2.Result)
	}

	t.Logf("Resume response: %s", resp2.Result)
}

// TestSDK_StreamingSpawn tests the streaming spawn functionality.
func TestSDK_StreamingSpawn(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:                "sonnet",
		MaxTurns:             1,
		NoSessionPersistence: true,
	})

	// Track messages received.
	messageCount := 0

	resp, err := spawner.StreamingSpawn(ctx,
		"Count from 1 to 5.",
		func(msg claudeagent.Message) {
			messageCount++
			t.Logf("Received message type: %s", msg.MessageType())
		},
	)
	if err != nil {
		t.Fatalf("streaming spawn failed: %v", err)
	}

	if messageCount == 0 {
		t.Error("expected to receive at least one message")
	}

	t.Logf("Received %d messages", messageCount)
	t.Logf("Final result: %s", resp.Result)
}

// TestSDK_InteractiveSession tests the interactive session functionality.
func TestSDK_InteractiveSession(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:                "sonnet",
		MaxTurns:             5,
		NoSessionPersistence: true,
	})

	session, err := spawner.SpawnInteractive(ctx)
	if err != nil {
		t.Fatalf("failed to create interactive session: %v", err)
	}
	defer session.Close()

	// Send first message.
	if err := session.Send("Hello, remember the word 'banana'."); err != nil {
		t.Fatalf("failed to send first message: %v", err)
	}

	// Collect first response.
	for msg := range session.Messages() {
		t.Logf("Message type: %s", msg.MessageType())
		if msg.MessageType() == "result" {
			break
		}
	}

	// Send second message.
	if err := session.Send("What word did I ask you to remember?"); err != nil {
		t.Fatalf("failed to send second message: %v", err)
	}

	// Collect second response.
	var result string
	for msg := range session.Messages() {
		if rm, ok := msg.(claudeagent.ResultMessage); ok {
			result = rm.Result
			break
		}
	}

	if !strings.Contains(strings.ToLower(result), "banana") {
		t.Errorf("expected response to contain 'banana', got: %s", result)
	}

	t.Logf("Interactive session ID: %s", session.SessionID())
}

// TestSDK_SendReceiveMessage tests the full send/receive message flow.
// This requires substrated to be running.
func TestSDK_SendReceiveMessage(t *testing.T) {
	skipIfNoCLI(t)
	skipIfNoAuth(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if substrate CLI is available.
	if _, err := exec.LookPath("substrate"); err != nil {
		t.Skip("substrate CLI not found, skipping send/receive test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	spawner := agent.NewSpawner(&agent.SpawnConfig{
		Model:                "sonnet",
		MaxTurns:             5,
		NoSessionPersistence: true,
		PermissionMode:       claudeagent.PermissionModeAcceptEdits,
	})

	// Have agent send a message using substrate CLI.
	resp, err := spawner.Spawn(ctx, `
		Use the substrate CLI to:
		1. First run 'substrate identity ensure' to get an identity
		2. Then send a message with subject "Test from SDK" and body "Hello from integration test"
		3. Report the result
	`)
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	t.Logf("Send message result: %s", resp.Result)

	// Have another agent check inbox.
	resp2, err := spawner.Spawn(ctx, `
		Use the substrate CLI to:
		1. Run 'substrate inbox' to check for messages
		2. Tell me if there are any messages with subject containing "Test from SDK"
	`)
	if err != nil {
		t.Fatalf("second spawn failed: %v", err)
	}

	t.Logf("Check inbox result: %s", resp2.Result)
}
