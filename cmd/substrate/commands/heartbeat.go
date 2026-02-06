package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/queue"
	"github.com/spf13/cobra"
)

var heartbeatSessionStart bool

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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// In queue mode, enqueue the heartbeat for later delivery.
	if client.Mode() == ModeQueued {
		return enqueueHeartbeat(ctx, client, agentNameStr)
	}

	// Record the heartbeat.
	if err := client.UpdateHeartbeat(ctx, agentID); err != nil {
		return fmt.Errorf("failed to record heartbeat: %w", err)
	}

	// If session start is requested, track the session via identity.
	var sessInfo *SessionInfo
	if heartbeatSessionStart && sessionID != "" {
		sessInfo = &SessionInfo{
			SessionID: sessionID,
			StartedAt: time.Now(),
		}
	}

	// Compute status based on recent heartbeat.
	status := "active"

	result := HeartbeatResult{
		AgentID:   agentID,
		AgentName: agentNameStr,
		Status:    status,
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

// enqueueHeartbeat stores a heartbeat operation in the local queue.
func enqueueHeartbeat(
	ctx context.Context, client *Client, agentNameStr string,
) error {
	key := newIdempotencyKey()
	payload := queue.HeartbeatPayload{
		AgentName:    agentNameStr,
		SessionStart: heartbeatSessionStart,
	}

	payloadJSON, err := queue.MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	now := time.Now()
	op := queue.PendingOperation{
		IdempotencyKey: key,
		OperationType:  queue.OpHeartbeat,
		PayloadJSON:    payloadJSON,
		AgentName:      agentNameStr,
		SessionID:      sessionID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(client.queueCfg.DefaultTTL),
	}

	if err := client.queueStore.Enqueue(ctx, op); err != nil {
		return fmt.Errorf("enqueue heartbeat: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"queued":          true,
			"idempotency_key": key,
		})
	case "context":
		return nil
	default:
		fmt.Println("Heartbeat queued (offline)")
	}

	return nil
}
