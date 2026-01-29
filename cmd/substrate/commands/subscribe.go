package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/spf13/cobra"
)

// subscribeCmd subscribes to a topic.
var subscribeCmd = &cobra.Command{
	Use:   "subscribe <topic>",
	Short: "Subscribe to a topic",
	Long:  `Subscribe the current agent to a topic for receiving messages.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSubscribe,
}

// unsubscribeCmd unsubscribes from a topic.
var unsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe <topic>",
	Short: "Unsubscribe from a topic",
	Long:  `Unsubscribe the current agent from a topic.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runUnsubscribe,
}

func init() {
	rootCmd.AddCommand(unsubscribeCmd)
}

func runSubscribe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	topicName := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentName, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	// Get or create the topic.
	topic, err := store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found", topicName)
	}

	// Check if already subscribed.
	_, err = store.Queries().GetSubscription(ctx, sqlc.GetSubscriptionParams{
		AgentID: agentID,
		TopicID: topic.ID,
	})
	if err == nil {
		fmt.Printf("%s is already subscribed to %s.\n", agentName, topicName)
		return nil
	}

	// Create subscription.
	err = store.Queries().CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      agentID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"agent":   agentName,
			"topic":   topicName,
			"status":  "subscribed",
		})
	default:
		fmt.Printf("%s subscribed to %s.\n", agentName, topicName)
	}

	return nil
}

func runUnsubscribe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	topicName := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentName, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	// Get the topic.
	topic, err := store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found", topicName)
	}

	// Delete subscription.
	err = store.Queries().DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
		AgentID: agentID,
		TopicID: topic.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"agent":   agentName,
			"topic":   topicName,
			"status":  "unsubscribed",
		})
	default:
		fmt.Printf("%s unsubscribed from %s.\n", agentName, topicName)
	}

	return nil
}
