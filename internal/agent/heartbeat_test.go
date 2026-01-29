package agent

import (
	"context"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// mockAgent creates a mock agent for testing.
func mockAgent(id int64, lastActiveAt int64) *sqlc.Agent {
	return &sqlc.Agent{
		ID:           id,
		Name:         "test-agent",
		LastActiveAt: lastActiveAt,
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name           string
		lastActiveAgo  time.Duration
		hasSession     bool
		expectedStatus AgentStatus
	}{
		{
			name:           "active within 5 minutes",
			lastActiveAgo:  2 * time.Minute,
			hasSession:     false,
			expectedStatus: StatusActive,
		},
		{
			name:           "busy with active session",
			lastActiveAgo:  1 * time.Minute,
			hasSession:     true,
			expectedStatus: StatusBusy,
		},
		{
			name:           "idle between 5-30 minutes",
			lastActiveAgo:  15 * time.Minute,
			hasSession:     false,
			expectedStatus: StatusIdle,
		},
		{
			name:           "offline after 30 minutes",
			lastActiveAgo:  45 * time.Minute,
			hasSession:     false,
			expectedStatus: StatusOffline,
		},
		{
			name:           "offline after 2 hours",
			lastActiveAgo:  2 * time.Hour,
			hasSession:     false,
			expectedStatus: StatusOffline,
		},
		{
			name:           "exactly at active threshold boundary",
			lastActiveAgo:  5 * time.Minute,
			hasSession:     false,
			expectedStatus: StatusIdle,
		},
		{
			name:           "just under active threshold",
			lastActiveAgo:  4*time.Minute + 59*time.Second,
			hasSession:     false,
			expectedStatus: StatusActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hm := NewHeartbeatManager(nil, DefaultHeartbeatConfig())

			agent := mockAgent(1, time.Now().Add(-tc.lastActiveAgo).Unix())

			if tc.hasSession {
				hm.StartSession(agent.ID, "test-session")
			}

			status := hm.ComputeStatus(agent)
			if status != tc.expectedStatus {
				t.Errorf("expected status %q, got %q", tc.expectedStatus, status)
			}
		})
	}
}

func TestSessionTracking(t *testing.T) {
	hm := NewHeartbeatManager(nil, DefaultHeartbeatConfig())

	agentID := int64(1)
	sessionID := "session-123"

	// Initially no session.
	if hm.HasActiveSession(agentID) {
		t.Error("expected no active session initially")
	}

	if got := hm.GetActiveSessionID(agentID); got != "" {
		t.Errorf("expected empty session ID, got %q", got)
	}

	// Start a session.
	hm.StartSession(agentID, sessionID)

	if !hm.HasActiveSession(agentID) {
		t.Error("expected active session after StartSession")
	}

	if got := hm.GetActiveSessionID(agentID); got != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, got)
	}

	// End the session.
	hm.EndSession(agentID)

	if hm.HasActiveSession(agentID) {
		t.Error("expected no active session after EndSession")
	}
}

func TestHeartbeatManagerWithRegistry(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	hm := NewHeartbeatManager(registry, DefaultHeartbeatConfig())

	ctx := context.Background()

	// Register an agent.
	agent, err := registry.RegisterAgent(ctx, "test-agent", "")
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Record a heartbeat.
	if err := hm.RecordHeartbeat(ctx, agent.ID); err != nil {
		t.Fatalf("RecordHeartbeat: %v", err)
	}

	// Get agent with status.
	aws, err := hm.GetAgentWithStatus(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentWithStatus: %v", err)
	}

	if aws.Status != StatusActive {
		t.Errorf("expected status %q after heartbeat, got %q",
			StatusActive, aws.Status)
	}

	// Start a session.
	hm.StartSession(agent.ID, "sess-1")

	aws, err = hm.GetAgentWithStatus(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentWithStatus: %v", err)
	}

	if aws.Status != StatusBusy {
		t.Errorf("expected status %q with active session, got %q",
			StatusBusy, aws.Status)
	}

	if aws.ActiveSessionID != "sess-1" {
		t.Errorf("expected session ID 'sess-1', got %q", aws.ActiveSessionID)
	}
}

func TestListAgentsWithStatus(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	hm := NewHeartbeatManager(registry, DefaultHeartbeatConfig())

	ctx := context.Background()

	// Register multiple agents.
	agent1, _ := registry.RegisterAgent(ctx, "agent-1", "")
	agent2, _ := registry.RegisterAgent(ctx, "agent-2", "")
	_, _ = registry.RegisterAgent(ctx, "agent-3", "")

	// Update last active for agent1 (make it active).
	hm.RecordHeartbeat(ctx, agent1.ID)

	// Update last active for agent2 (make it active with session = busy).
	hm.RecordHeartbeat(ctx, agent2.ID)
	hm.StartSession(agent2.ID, "sess-1")

	// agent3 is not updated, so it will be based on registration time.

	agents, err := hm.ListAgentsWithStatus(ctx)
	if err != nil {
		t.Fatalf("ListAgentsWithStatus: %v", err)
	}

	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}

	// Check that we have both active and busy agents.
	statusMap := make(map[AgentStatus]int)
	for _, aws := range agents {
		statusMap[aws.Status]++
	}

	if statusMap[StatusActive] < 1 {
		t.Error("expected at least one active agent")
	}
	if statusMap[StatusBusy] != 1 {
		t.Errorf("expected 1 busy agent, got %d", statusMap[StatusBusy])
	}
}

func TestGetStatusCounts(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	hm := NewHeartbeatManager(registry, DefaultHeartbeatConfig())

	ctx := context.Background()

	// Register agents and set up different statuses.
	agent1, _ := registry.RegisterAgent(ctx, "active-agent", "")
	agent2, _ := registry.RegisterAgent(ctx, "busy-agent", "")

	hm.RecordHeartbeat(ctx, agent1.ID)
	hm.RecordHeartbeat(ctx, agent2.ID)
	hm.StartSession(agent2.ID, "sess-1")

	counts, err := hm.GetStatusCounts(ctx)
	if err != nil {
		t.Fatalf("GetStatusCounts: %v", err)
	}

	if counts.Total != 2 {
		t.Errorf("expected total 2, got %d", counts.Total)
	}

	if counts.Active != 1 {
		t.Errorf("expected 1 active, got %d", counts.Active)
	}

	if counts.Busy != 1 {
		t.Errorf("expected 1 busy, got %d", counts.Busy)
	}
}
