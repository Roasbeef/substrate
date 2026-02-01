package agent

import (
	"context"
	"sync"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// AgentStatus represents the current liveness status of an agent.
type AgentStatus string

const (
	// StatusActive means the agent has sent a heartbeat within the active
	// threshold (e.g., last 5 minutes).
	StatusActive AgentStatus = "active"

	// StatusBusy means the agent is active and currently running a session.
	StatusBusy AgentStatus = "busy"

	// StatusIdle means the agent's last heartbeat was between the active and
	// offline thresholds (e.g., 5-30 minutes ago).
	StatusIdle AgentStatus = "idle"

	// StatusOffline means the agent hasn't sent a heartbeat within the
	// offline threshold (e.g., more than 30 minutes ago).
	StatusOffline AgentStatus = "offline"
)

// Default status thresholds.
const (
	// DefaultActiveThreshold is the maximum time since last heartbeat for an
	// agent to be considered active.
	DefaultActiveThreshold = 5 * time.Minute

	// DefaultOfflineThreshold is the minimum time since last heartbeat for an
	// agent to be considered offline.
	DefaultOfflineThreshold = 30 * time.Minute
)

// HeartbeatConfig holds configuration for the heartbeat system.
type HeartbeatConfig struct {
	// ActiveThreshold is how long ago the last heartbeat can be for an agent
	// to still be considered active.
	ActiveThreshold time.Duration

	// OfflineThreshold is how long ago the last heartbeat must be for an
	// agent to be considered offline.
	OfflineThreshold time.Duration
}

// DefaultHeartbeatConfig returns the default heartbeat configuration.
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		ActiveThreshold:  DefaultActiveThreshold,
		OfflineThreshold: DefaultOfflineThreshold,
	}
}

// HeartbeatManager tracks agent liveness and computes status.
type HeartbeatManager struct {
	registry *Registry
	cfg      *HeartbeatConfig

	// activeSessions tracks agents with active sessions (keyed by agent ID).
	// Used to distinguish "busy" from "active".
	activeSessions   map[int64]string
	activeSessionsMu sync.RWMutex
}

// NewHeartbeatManager creates a new heartbeat manager.
func NewHeartbeatManager(registry *Registry, cfg *HeartbeatConfig) *HeartbeatManager {
	if cfg == nil {
		cfg = DefaultHeartbeatConfig()
	}

	return &HeartbeatManager{
		registry:       registry,
		cfg:            cfg,
		activeSessions: make(map[int64]string),
	}
}

// RecordHeartbeat records a heartbeat from an agent, updating its last active
// timestamp.
func (h *HeartbeatManager) RecordHeartbeat(ctx context.Context,
	agentID int64,
) error {
	return h.registry.UpdateLastActive(ctx, agentID)
}

// RecordHeartbeatByName records a heartbeat from an agent by name.
func (h *HeartbeatManager) RecordHeartbeatByName(ctx context.Context,
	agentName string,
) error {
	agent, err := h.registry.GetAgentByName(ctx, agentName)
	if err != nil {
		return err
	}

	return h.RecordHeartbeat(ctx, agent.ID)
}

// StartSession marks an agent as having an active session.
func (h *HeartbeatManager) StartSession(agentID int64, sessionID string) {
	h.activeSessionsMu.Lock()
	defer h.activeSessionsMu.Unlock()

	h.activeSessions[agentID] = sessionID
}

// EndSession marks an agent's session as ended.
func (h *HeartbeatManager) EndSession(agentID int64) {
	h.activeSessionsMu.Lock()
	defer h.activeSessionsMu.Unlock()

	delete(h.activeSessions, agentID)
}

// HasActiveSession checks if an agent has an active session.
func (h *HeartbeatManager) HasActiveSession(agentID int64) bool {
	h.activeSessionsMu.RLock()
	defer h.activeSessionsMu.RUnlock()

	_, ok := h.activeSessions[agentID]
	return ok
}

// GetActiveSessionID returns the active session ID for an agent, or empty
// string if none.
func (h *HeartbeatManager) GetActiveSessionID(agentID int64) string {
	h.activeSessionsMu.RLock()
	defer h.activeSessionsMu.RUnlock()

	return h.activeSessions[agentID]
}

// ComputeStatus computes an agent's status based on its last heartbeat time.
func (h *HeartbeatManager) ComputeStatus(agent *sqlc.Agent) AgentStatus {
	lastActive := time.Unix(agent.LastActiveAt, 0)
	elapsed := time.Since(lastActive)

	// Check if offline first.
	if elapsed > h.cfg.OfflineThreshold {
		return StatusOffline
	}

	// Check if idle (between active and offline thresholds).
	if elapsed > h.cfg.ActiveThreshold {
		return StatusIdle
	}

	// Agent is active - check if busy with a session.
	if h.HasActiveSession(agent.ID) {
		return StatusBusy
	}

	return StatusActive
}

// AgentWithStatus wraps an agent with its computed status.
type AgentWithStatus struct {
	Agent           *sqlc.Agent
	Status          AgentStatus
	ActiveSessionID string
	LastActive      time.Time
}

// GetAgentWithStatus retrieves an agent with its computed status.
func (h *HeartbeatManager) GetAgentWithStatus(ctx context.Context,
	agentID int64,
) (*AgentWithStatus, error) {
	agent, err := h.registry.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return &AgentWithStatus{
		Agent:           agent,
		Status:          h.ComputeStatus(agent),
		ActiveSessionID: h.GetActiveSessionID(agent.ID),
		LastActive:      time.Unix(agent.LastActiveAt, 0),
	}, nil
}

// GetAgentWithStatusByName retrieves an agent by name with its computed status.
func (h *HeartbeatManager) GetAgentWithStatusByName(ctx context.Context,
	agentName string,
) (*AgentWithStatus, error) {
	agent, err := h.registry.GetAgentByName(ctx, agentName)
	if err != nil {
		return nil, err
	}

	return &AgentWithStatus{
		Agent:           agent,
		Status:          h.ComputeStatus(agent),
		ActiveSessionID: h.GetActiveSessionID(agent.ID),
		LastActive:      time.Unix(agent.LastActiveAt, 0),
	}, nil
}

// ListAgentsWithStatus returns all agents with their computed status.
func (h *HeartbeatManager) ListAgentsWithStatus(
	ctx context.Context,
) ([]AgentWithStatus, error) {
	agents, err := h.registry.ListAgents(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]AgentWithStatus, len(agents))
	for i := range agents {
		agent := &agents[i]
		result[i] = AgentWithStatus{
			Agent:           agent,
			Status:          h.ComputeStatus(agent),
			ActiveSessionID: h.GetActiveSessionID(agent.ID),
			LastActive:      time.Unix(agent.LastActiveAt, 0),
		}
	}

	return result, nil
}

// CountByStatus returns counts of agents by status.
type StatusCounts struct {
	Active  int
	Busy    int
	Idle    int
	Offline int
	Total   int
}

// GetStatusCounts returns counts of agents by status.
func (h *HeartbeatManager) GetStatusCounts(
	ctx context.Context,
) (*StatusCounts, error) {
	agents, err := h.registry.ListAgents(ctx)
	if err != nil {
		return nil, err
	}

	counts := &StatusCounts{Total: len(agents)}

	for i := range agents {
		status := h.ComputeStatus(&agents[i])
		switch status {
		case StatusActive:
			counts.Active++
		case StatusBusy:
			counts.Busy++
		case StatusIdle:
			counts.Idle++
		case StatusOffline:
			counts.Offline++
		}
	}

	return counts, nil
}
