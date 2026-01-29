package commands

import (
	"context"
	"fmt"

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
	rootCmd.AddCommand(subscribeCmd)
	rootCmd.AddCommand(unsubscribeCmd)
}

func runSubscribe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	topicName := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentName, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	if err := client.Subscribe(ctx, agentID, topicName); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"agent":  agentName,
			"topic":  topicName,
			"status": "subscribed",
		})
	default:
		fmt.Printf("%s subscribed to %s.\n", agentName, topicName)
	}

	return nil
}

func runUnsubscribe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	topicName := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentName, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	if err := client.Unsubscribe(ctx, agentID, topicName); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"agent":  agentName,
			"topic":  topicName,
			"status": "unsubscribed",
		})
	default:
		fmt.Printf("%s unsubscribed from %s.\n", agentName, topicName)
	}

	return nil
}
