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
	publishSubject  string
	publishBody     string
	publishPriority string
)

// publishCmd publishes a message to a topic.
var publishCmd = &cobra.Command{
	Use:   "publish <topic>",
	Short: "Publish a message to a topic",
	Long:  `Publish a message to all subscribers of a topic.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPublish,
}

func init() {
	publishCmd.Flags().StringVar(&publishSubject, "subject", "",
		"Message subject (required)")
	publishCmd.Flags().StringVar(&publishBody, "body", "",
		"Message body in markdown")
	publishCmd.Flags().StringVar(&publishPriority, "priority", "normal",
		"Priority: urgent, normal, low")

	publishCmd.MarkFlagRequired("subject")
}

func runPublish(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	topicName := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Parse priority.
	var priority mail.Priority
	switch publishPriority {
	case "urgent":
		priority = mail.PriorityUrgent
	case "normal":
		priority = mail.PriorityNormal
	case "low":
		priority = mail.PriorityLow
	default:
		return fmt.Errorf("invalid priority: %s", publishPriority)
	}

	// In queue mode, enqueue the operation for later delivery.
	if client.Mode() == ModeQueued {
		return enqueuePublish(
			ctx, client, agentNameStr, topicName,
			string(priority),
		)
	}

	msgID, recipientsCount, err := client.Publish(
		ctx, agentID, topicName, publishSubject, publishBody, priority,
	)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"message_id":       msgID,
			"topic":            topicName,
			"recipients_count": recipientsCount,
		})
	default:
		fmt.Printf("Published to %s! Message ID: %d, Recipients: %d\n",
			topicName, msgID, recipientsCount)
	}

	return nil
}

// enqueuePublish stores a publish operation in the local queue.
func enqueuePublish(
	ctx context.Context, client *Client, senderName, topicName,
	priority string,
) error {
	key := newIdempotencyKey()
	payload := queue.PublishPayload{
		SenderName: senderName,
		TopicName:  topicName,
		Subject:    publishSubject,
		Body:       publishBody,
		Priority:   priority,
	}

	payloadJSON, err := queue.MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	now := time.Now()
	op := queue.PendingOperation{
		IdempotencyKey: key,
		OperationType:  queue.OpPublish,
		PayloadJSON:    payloadJSON,
		AgentName:      senderName,
		SessionID:      sessionID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(client.queueCfg.DefaultTTL),
	}

	if err := client.queueStore.Enqueue(ctx, op); err != nil {
		return fmt.Errorf("enqueue publish: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"queued":          true,
			"idempotency_key": key,
			"topic":           topicName,
		})
	default:
		fmt.Printf("Publish to %s queued (offline)\n", topicName)
	}

	return nil
}
