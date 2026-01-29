package commands

import (
	"context"
	"fmt"
	"strconv"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read <message_id>",
	Short: "Read a message",
	Long:  `Display the full content of a message and mark it as read.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRead,
}

func runRead(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	msgID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid message ID: %w", err)
	}

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, _, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	svc := mail.NewService(store)

	req := mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: msgID,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.ReadMessageResponse)
	if resp.Error != nil {
		return resp.Error
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp.Message)
	default:
		fmt.Print(formatMessageFull(resp.Message))
	}

	return nil
}
