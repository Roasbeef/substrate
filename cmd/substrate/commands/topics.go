package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	topicsSubscribed bool
)

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

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if topicsSubscribed {
		agentID, agentName, err := getCurrentAgent(ctx, store)
		if err != nil {
			return err
		}

		topics, err := store.Queries().ListSubscriptionsByAgent(ctx, agentID)
		if err != nil {
			return fmt.Errorf("failed to list subscriptions: %w", err)
		}

		switch outputFormat {
		case "json":
			return outputJSON(topics)
		default:
			if len(topics) == 0 {
				fmt.Printf("%s has no subscriptions.\n", agentName)
				return nil
			}

			fmt.Printf("Subscriptions for %s (%d):\n\n", agentName, len(topics))
			for _, t := range topics {
				retention := "default"
				if t.RetentionSeconds.Valid {
					retention = fmt.Sprintf("%dd", t.RetentionSeconds.Int64/86400)
				}
				fmt.Printf("  %s (%s, %s retention)\n",
					t.Name, t.TopicType, retention)
			}
		}
		return nil
	}

	// List all topics.
	topics, err := store.Queries().ListTopics(ctx)
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
			if t.RetentionSeconds.Valid {
				retention = fmt.Sprintf("%dd", t.RetentionSeconds.Int64/86400)
			}

			// Get subscriber count.
			count, err := store.Queries().CountSubscribersByTopic(ctx, t.ID)
			if err != nil {
				count = 0
			}

			fmt.Printf("  %s\n", t.Name)
			fmt.Printf("    Type: %s | Retention: %s | Subscribers: %d\n",
				t.TopicType, retention, count)
		}
	}

	return nil
}
