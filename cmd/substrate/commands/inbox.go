package commands

import (
	"context"
	"fmt"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	inboxLimit      int
	inboxUnreadOnly bool
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "View your inbox",
	Long:  `Display messages in your inbox with optional filters.`,
	RunE:  runInbox,
}

func init() {
	inboxCmd.Flags().IntVarP(&inboxLimit, "limit", "n", 20,
		"Maximum number of messages to display")
	inboxCmd.Flags().BoolVar(&inboxUnreadOnly, "unread-only", false,
		"Show only unread messages")
}

func runInbox(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentName, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	svc := mail.NewService(store)

	req := mail.FetchInboxRequest{
		AgentID:    agentID,
		Limit:      inboxLimit,
		UnreadOnly: inboxUnreadOnly,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.FetchInboxResponse)
	if resp.Error != nil {
		return resp.Error
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp.Messages)

	case "context":
		if len(resp.Messages) > 0 {
			fmt.Print(formatContext(resp.Messages))
		}
		return nil

	default:
		if len(resp.Messages) == 0 {
			fmt.Printf("Inbox for %s is empty.\n", agentName)
			return nil
		}

		fmt.Printf("Inbox for %s (%d messages):\n\n", agentName,
			len(resp.Messages))
		for _, msg := range resp.Messages {
			fmt.Print(formatMessage(msg))
			fmt.Println()
		}
	}

	return nil
}
