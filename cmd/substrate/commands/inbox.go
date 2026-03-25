package commands

import (
	"context"
	"fmt"
	"strconv"

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

	// Parse page token offset for pagination.
	offset := 0
	if pageToken != "" {
		var parseErr error
		offset, parseErr = strconv.Atoi(pageToken)
		if parseErr != nil {
			return fmt.Errorf(
				"invalid page-token %q: %w",
				pageToken, parseErr,
			)
		}
	}

	req := mail.FetchInboxRequest{
		AgentID:    agentID,
		Limit:      inboxLimit,
		Offset:     offset,
		UnreadOnly: !inboxAll,
	}

	messages, err := client.FetchInbox(ctx, req)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		// Truncate message bodies in inbox listings to keep JSON
		// output concise for agent context windows. Use `read <id>`
		// for full content.
		truncated := truncateInboxBodies(messages)
		return outputWithPagination(
			truncated, offset, inboxLimit, len(truncated),
		)

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
