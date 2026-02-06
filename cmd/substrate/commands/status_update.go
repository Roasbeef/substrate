package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/queue"
	"github.com/spf13/cobra"
)

var (
	statusTo          string
	statusSummary     string
	statusWaitingFor  string
	statusSkipPending bool
)

var statusUpdateCmd = &cobra.Command{
	Use:   "status-update",
	Short: "Send a status update message",
	Long: `Send a status update message to another agent (typically User).

This command is designed for use in stop hooks to send status updates
when an agent finishes work. It includes deduplication to avoid spam.

Example:
  substrate status-update --to User --summary "Completed task X" --waiting-for "Next instructions"
  substrate status-update --to User --summary "Done" --skip-if-pending`,
	RunE: runStatusUpdate,
}

func init() {
	statusUpdateCmd.Flags().StringVar(&statusTo, "to", "User",
		"Recipient agent name (default: User)")
	statusUpdateCmd.Flags().StringVar(&statusSummary, "summary", "",
		"Summary of what was accomplished")
	statusUpdateCmd.Flags().StringVar(&statusWaitingFor, "waiting-for", "",
		"What the agent is waiting for")
	statusUpdateCmd.Flags().BoolVar(&statusSkipPending, "skip-if-pending", false,
		"Skip sending if there's an unacked status message to recipient")
}

func runStatusUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentNameResolved, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Build subject and body early so they can be reused for queueing.
	statusSubject := fmt.Sprintf("[Status] %s", agentNameResolved)
	statusBody := ""
	if statusSummary != "" {
		statusBody += statusSummary + "\n\n"
	}
	if statusWaitingFor != "" {
		statusBody += "Waiting for: " + statusWaitingFor
	}
	if statusBody == "" {
		statusBody = "Status update (no details provided)"
	}

	// In queue mode, enqueue the status update for later delivery.
	if client.Mode() == ModeQueued {
		return enqueueStatusUpdate(
			ctx, client, agentNameResolved,
			statusSubject, statusBody,
		)
	}

	// Resolve recipient agent ID for deduplication check.
	recipient, err := client.GetAgentByName(ctx, statusTo)
	if err != nil {
		return fmt.Errorf("recipient %q not found: %w", statusTo, err)
	}
	recipientID := recipient.ID

	// Check for pending status messages if deduplication enabled.
	if statusSkipPending {
		hasPending, err := client.HasUnackedStatusTo(ctx, agentID, recipientID)
		if err != nil {
			// Log but don't fail - still send if check fails.
			fmt.Printf("Warning: dedup check failed: %v\n", err)
		} else if hasPending {
			switch outputFormat {
			case "json":
				return outputJSON(map[string]any{
					"skipped": true,
					"reason":  "pending unacked status message exists",
				})
			case "hook":
				return outputJSON(map[string]any{
					"skipped": true,
					"reason":  "pending unacked status message exists",
				})
			default:
				fmt.Println("Skipped: pending unacked status message exists")
			}
			return nil
		}
	}

	req := mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{statusTo},
		Subject:        statusSubject,
		Body:           statusBody,
		Priority:       mail.PriorityNormal,
	}

	msgID, threadID, err := client.SendMail(ctx, req)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json", "hook":
		return outputJSON(map[string]any{
			"sent":       true,
			"message_id": msgID,
			"thread_id":  threadID,
		})
	default:
		fmt.Printf("Status update sent! ID: %d\n", msgID)
	}

	return nil
}

// enqueueStatusUpdate stores a status update operation in the local queue.
func enqueueStatusUpdate(
	ctx context.Context, client *Client, senderName,
	subject, body string,
) error {
	key := newIdempotencyKey()
	payload := queue.StatusUpdatePayload{
		SenderName:     senderName,
		RecipientNames: []string{statusTo},
		Subject:        subject,
		Body:           body,
	}

	payloadJSON, err := queue.MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	now := time.Now()
	op := queue.PendingOperation{
		IdempotencyKey: key,
		OperationType:  queue.OpStatusUpdate,
		PayloadJSON:    payloadJSON,
		AgentName:      senderName,
		SessionID:      sessionID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(client.queueCfg.DefaultTTL),
	}

	if err := client.queueStore.Enqueue(ctx, op); err != nil {
		return fmt.Errorf("enqueue status update: %w", err)
	}

	switch outputFormat {
	case "json", "hook":
		return outputJSON(map[string]any{
			"queued":          true,
			"idempotency_key": key,
		})
	default:
		fmt.Println("Status update queued (offline)")
	}

	return nil
}
