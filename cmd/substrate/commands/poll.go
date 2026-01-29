package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	pollQuiet bool
)

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll for new messages",
	Long:  `Check for new messages since last poll.`,
	RunE:  runPoll,
}

func init() {
	pollCmd.Flags().BoolVar(&pollQuiet, "quiet", false,
		"Only output if there are new messages")
}

func runPoll(cmd *cobra.Command, args []string) error {
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

	// Send heartbeat to indicate agent activity.
	_ = client.UpdateHeartbeat(ctx, agentID)

	// PollChanges with empty offsets to get all unread messages.
	// The client will track offsets internally if using gRPC.
	newMessages, _, err := client.PollChanges(ctx, agentID, nil)
	if err != nil {
		return err
	}

	if len(newMessages) == 0 {
		if !pollQuiet {
			fmt.Printf("No new messages for %s.\n", agentNameStr)
		}
		return nil
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"new_messages": newMessages,
		})
	case "context":
		fmt.Print(formatContext(newMessages))
		return nil
	default:
		fmt.Printf("New messages for %s:\n\n", agentNameStr)
		for _, msg := range newMessages {
			fmt.Print(formatMessage(msg))
			fmt.Println()
		}
	}

	return nil
}
