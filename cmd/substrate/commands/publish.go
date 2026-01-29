package commands

import (
	"context"
	"fmt"

	"github.com/roasbeef/subtrate/internal/mail"
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

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, _, err := getCurrentAgent(ctx, store)
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

	svc := mail.NewService(store)

	req := mail.PublishRequest{
		SenderID:  agentID,
		TopicName: topicName,
		Subject:   publishSubject,
		Body:      publishBody,
		Priority:  priority,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.PublishResponse)
	if resp.Error != nil {
		return resp.Error
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"message_id":       resp.MessageID,
			"topic":            topicName,
			"recipients_count": resp.RecipientsCount,
		})
	default:
		fmt.Printf("Published to %s! Message ID: %d, Recipients: %d\n",
			topicName, resp.MessageID, resp.RecipientsCount)
	}

	return nil
}
