package agent

import (
	"testing"
	"time"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// TestDefaultSpawnConfig tests the default configuration.
func TestDefaultSpawnConfig(t *testing.T) {
	cfg := DefaultSpawnConfig()

	if cfg.CLIPath != "claude" {
		t.Errorf("expected 'claude', got %s", cfg.CLIPath)
	}

	if cfg.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected 'claude-opus-4-5-20251101', got %s", cfg.Model)
	}

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected 5 minutes, got %v", cfg.Timeout)
	}
}

// TestSpawner_BuildClientOptions tests the options construction.
func TestSpawner_BuildClientOptions(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *SpawnConfig
		expectedLen int
	}{
		{
			name:        "default config",
			cfg:         DefaultSpawnConfig(),
			expectedLen: 1, // Just model.
		},
		{
			name: "with custom CLI path",
			cfg: &SpawnConfig{
				CLIPath: "/custom/claude",
				Model:   "claude-sonnet-4-5-20250929",
			},
			expectedLen: 2, // Model + CLI path.
		},
		{
			name: "with work dir",
			cfg: &SpawnConfig{
				Model:   "claude-sonnet-4-5-20250929",
				WorkDir: "/tmp/work",
			},
			expectedLen: 2, // Model + work dir.
		},
		{
			name: "with system prompt",
			cfg: &SpawnConfig{
				Model:        "claude-sonnet-4-5-20250929",
				SystemPrompt: "You are a helpful assistant",
			},
			expectedLen: 2, // Model + system prompt.
		},
		{
			name: "with max turns",
			cfg: &SpawnConfig{
				Model:    "claude-sonnet-4-5-20250929",
				MaxTurns: 10,
			},
			expectedLen: 2, // Model + max turns.
		},
		{
			name: "with permission mode",
			cfg: &SpawnConfig{
				Model:          "claude-sonnet-4-5-20250929",
				PermissionMode: claudeagent.PermissionModeBypassAll,
			},
			expectedLen: 2, // Model + permission mode.
		},
		{
			name: "with no session persistence",
			cfg: &SpawnConfig{
				Model:                "claude-sonnet-4-5-20250929",
				NoSessionPersistence: true,
			},
			expectedLen: 2, // Model + no session persistence.
		},
		{
			name: "full config",
			cfg: &SpawnConfig{
				CLIPath:                         "/custom/claude",
				Model:                           "claude-opus-4-5-20250929",
				WorkDir:                         "/tmp/work",
				SystemPrompt:                    "Test prompt",
				MaxTurns:                        5,
				PermissionMode:                  claudeagent.PermissionModeAcceptEdits,
				AllowDangerouslySkipPermissions: true,
				NoSessionPersistence:            true,
			},
			expectedLen: 8, // All options.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spawner := NewSpawner(tt.cfg)
			opts := spawner.buildClientOptions()

			if len(opts) != tt.expectedLen {
				t.Fatalf("expected %d options, got %d",
					tt.expectedLen, len(opts))
			}
		})
	}
}

// TestSpawner_ProcessTracking tests process tracking.
func TestSpawner_ProcessTracking(t *testing.T) {
	spawner := NewSpawner(nil)

	// Initially no processes.
	procs := spawner.ListProcesses()
	if len(procs) != 0 {
		t.Errorf("expected 0 processes, got %d", len(procs))
	}

	// Add a process manually (simulating async spawn).
	spawner.processesMu.Lock()
	spawner.processes["session1"] = &SpawnedProcess{
		SessionID: "session1",
		StartedAt: time.Now(),
		Prompt:    "Test prompt",
	}
	spawner.processesMu.Unlock()

	// Should have one process now.
	procs = spawner.ListProcesses()
	if len(procs) != 1 {
		t.Errorf("expected 1 process, got %d", len(procs))
	}

	// Get specific process.
	proc := spawner.GetProcess("session1")
	if proc == nil {
		t.Fatal("expected to find process")
	}

	if proc.SessionID != "session1" {
		t.Errorf("expected session1, got %s", proc.SessionID)
	}

	if proc.Prompt != "Test prompt" {
		t.Errorf("expected 'Test prompt', got %s", proc.Prompt)
	}

	// Non-existent process.
	proc = spawner.GetProcess("nonexistent")
	if proc != nil {
		t.Error("expected nil for nonexistent process")
	}
}

// TestSpawner_NewSpawnerWithNilConfig tests that nil config uses defaults.
func TestSpawner_NewSpawnerWithNilConfig(t *testing.T) {
	spawner := NewSpawner(nil)

	if spawner.cfg == nil {
		t.Fatal("expected config to be set")
	}

	if spawner.cfg.CLIPath != "claude" {
		t.Errorf("expected 'claude', got %s", spawner.cfg.CLIPath)
	}

	if spawner.cfg.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected default model, got %s", spawner.cfg.Model)
	}
}

// TestSpawnResponse tests the SpawnResponse struct.
func TestSpawnResponse(t *testing.T) {
	resp := SpawnResponse{
		Result:     "Hello, world!",
		SessionID:  "abc123",
		CostUSD:    0.05,
		DurationMS: 1500,
		NumTurns:   3,
		IsError:    false,
	}

	if resp.Result != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %s", resp.Result)
	}

	if resp.SessionID != "abc123" {
		t.Errorf("expected 'abc123', got %s", resp.SessionID)
	}

	if resp.CostUSD != 0.05 {
		t.Errorf("expected 0.05, got %f", resp.CostUSD)
	}

	if resp.DurationMS != 1500 {
		t.Errorf("expected 1500, got %d", resp.DurationMS)
	}

	if resp.NumTurns != 3 {
		t.Errorf("expected 3, got %d", resp.NumTurns)
	}

	if resp.IsError {
		t.Error("expected IsError to be false")
	}
}

// TestSpawnResponse_Error tests error response handling.
func TestSpawnResponse_Error(t *testing.T) {
	resp := SpawnResponse{
		Result:    "",
		SessionID: "xyz789",
		IsError:   true,
		Error:     "Something went wrong",
	}

	if !resp.IsError {
		t.Error("expected IsError to be true")
	}

	if resp.Error != "Something went wrong" {
		t.Errorf("expected error message, got %s", resp.Error)
	}
}

// TestSpawnedProcess tests the SpawnedProcess struct.
func TestSpawnedProcess(t *testing.T) {
	now := time.Now()
	endTime := now.Add(5 * time.Second)

	proc := SpawnedProcess{
		SessionID: "test-session",
		StartedAt: now,
		EndedAt:   &endTime,
		Prompt:    "Do something",
		Response: &SpawnResponse{
			Result:    "Done!",
			SessionID: "test-session",
		},
		Error: nil,
	}

	if proc.SessionID != "test-session" {
		t.Errorf("expected 'test-session', got %s", proc.SessionID)
	}

	if proc.StartedAt != now {
		t.Errorf("expected start time to match")
	}

	if proc.EndedAt == nil || *proc.EndedAt != endTime {
		t.Errorf("expected end time to match")
	}

	if proc.Prompt != "Do something" {
		t.Errorf("expected 'Do something', got %s", proc.Prompt)
	}

	if proc.Response == nil || proc.Response.Result != "Done!" {
		t.Error("expected response with result 'Done!'")
	}

	if proc.Error != nil {
		t.Errorf("expected no error, got %v", proc.Error)
	}
}

// TestInteractiveSession_SessionID tests the session ID accessor.
func TestInteractiveSession_Close(t *testing.T) {
	// Test that Close handles nil stream and client gracefully.
	session := &InteractiveSession{
		client: nil,
		stream: nil,
	}

	err := session.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
