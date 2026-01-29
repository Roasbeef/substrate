package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/spf13/cobra"
)

var (
	heartbeatSessionStart bool
)

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Send a heartbeat to indicate agent is alive",
	Long: `Send a heartbeat to update the agent's last active timestamp.

This is useful for Claude Code hooks to signal agent presence.
The heartbeat updates the agent's status to active/busy.`,
	RunE: runHeartbeat,
}

func init() {
	heartbeatCmd.Flags().BoolVar(&heartbeatSessionStart, "session-start", false,
		"Start a new session (marks agent as busy)")

	rootCmd.AddCommand(heartbeatCmd)
}

// HeartbeatResult contains the result of a heartbeat operation.
type HeartbeatResult struct {
	AgentID   int64        `json:"agent_id"`
	AgentName string       `json:"agent_name"`
	Status    string       `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Session   *SessionInfo `json:"session,omitempty"`
}

// SessionInfo contains session information for heartbeat.
type SessionInfo struct {
	SessionID string    `json:"session_id"`
	StartedAt time.Time `json:"started_at"`
}

func runHeartbeat(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentNameStr, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	registry := agent.NewRegistry(store)
	heartbeatMgr := agent.NewHeartbeatManager(registry, nil)

	// Record the heartbeat.
	if err := heartbeatMgr.RecordHeartbeat(ctx, agentID); err != nil {
		return fmt.Errorf("failed to record heartbeat: %w", err)
	}

	// If session start is requested, track the session.
	var sessInfo *SessionInfo
	if heartbeatSessionStart && sessionID != "" {
		heartbeatMgr.StartSession(agentID, sessionID)
		sessInfo = &SessionInfo{
			SessionID: sessionID,
			StartedAt: time.Now(),
		}
	}

	// Get current status.
	agentObj, err := registry.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	status := heartbeatMgr.ComputeStatus(agentObj)

	result := HeartbeatResult{
		AgentID:   agentID,
		AgentName: agentNameStr,
		Status:    string(status),
		Timestamp: time.Now(),
		Session:   sessInfo,
	}

	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))

	case "context":
		// Quiet output for hook integration.
		return nil

	default:
		fmt.Printf("Heartbeat sent for %s (status: %s)\n",
			agentNameStr, status)
		if sessInfo != nil {
			fmt.Printf("Session started: %s\n", sessInfo.SessionID)
		}
	}

	return nil
}

// sendHeartbeatQuiet is a helper function for other commands to send heartbeats.
// It silently updates the agent's last active timestamp without any output.
func sendHeartbeatQuiet(ctx context.Context, store *db.Store, agentID int64) {
	registry := agent.NewRegistry(store)
	// Ignore errors for quiet heartbeat - best effort.
	_ = registry.UpdateLastActive(ctx, agentID)
}
