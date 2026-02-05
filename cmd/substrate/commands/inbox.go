package commands

import (
	"context"
	"fmt"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	inboxLimit int
	inboxAll   bool
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "View your inbox",
	Long:  `Display messages in your inbox. By default shows only unread messages.`,
	RunE:  runInbox,
}

func init() {
	inboxCmd.Flags().IntVarP(&inboxLimit, "limit", "n", 20,
		"Maximum number of messages to display")
	inboxCmd.Flags().BoolVarP(&inboxAll, "all", "a", false,
		"Show all messages (default: only unread)")
}

func runInbox(cmd *cobra.Command, args []string) error {
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

	req := mail.FetchInboxRequest{
		AgentID:    agentID,
		Limit:      inboxLimit,
		UnreadOnly: !inboxAll,
	}

	messages, err := client.FetchInbox(ctx, req)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(messages)

	case "context":
		if len(messages) > 0 {
			fmt.Print(formatContext(messages))
		}
		return nil

	default:
		if len(messages) == 0 {
			fmt.Printf("Inbox for %s is empty.\n", agentNameStr)
			return nil
		}

		fmt.Printf("Inbox for %s (%d messages):\n\n", agentNameStr,
			len(messages))
		for _, msg := range messages {
			fmt.Print(formatMessage(msg))
			fmt.Println()
		}
	}

	return nil
}
