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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, _, err := getCurrentAgentWithClient(ctx, client)
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
