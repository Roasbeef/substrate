package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var topicsSubscribed bool

// topicsCmd lists topics.
var topicsCmd = &cobra.Command{
	Use:   "topics",
	Short: "List topics",
	Long:  `List all topics or subscribed topics.`,
	RunE:  runTopics,
}

func init() {
	topicsCmd.Flags().BoolVar(&topicsSubscribed, "subscribed", false,
		"Show only subscribed topics")
}

func runTopics(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if topicsSubscribed {
		agentID, agentName, err := getCurrentAgentWithClient(ctx, client)
		if err != nil {
			return err
		}

		subs, err := client.ListSubscriptionsByAgent(ctx, agentID)
		if err != nil {
			return fmt.Errorf("failed to list subscriptions: %w", err)
		}

		switch outputFormat {
		case "json":
			return outputJSON(subs)
		default:
			if len(subs) == 0 {
				fmt.Printf("%s has no subscriptions.\n", agentName)
				return nil
			}

			fmt.Printf("Subscriptions for %s (%d):\n\n", agentName, len(subs))
			for _, s := range subs {
				retention := "default"
				if s.RetentionSeconds > 0 {
					retention = fmt.Sprintf("%dd", s.RetentionSeconds/86400)
				}
				fmt.Printf("  %s (%s, %s retention)\n",
					s.TopicName, s.TopicType, retention)
			}
		}
		return nil
	}

	// List all topics.
	topics, err := client.ListTopics(ctx)
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(topics)
	default:
		if len(topics) == 0 {
			fmt.Println("No topics exist.")
			return nil
		}

		fmt.Printf("Topics (%d):\n\n", len(topics))
		for _, t := range topics {
			retention := "default"
			if t.RetentionSeconds > 0 {
				retention = fmt.Sprintf("%dd", t.RetentionSeconds/86400)
			}

			fmt.Printf("  %s\n", t.Name)
			fmt.Printf("    Type: %s | Retention: %s | Subscribers: %d\n",
				t.Type, retention, t.SubscriberCount)
		}
	}

	return nil
}
