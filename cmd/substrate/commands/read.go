package commands

import (
	"context"
	"fmt"
	"strconv"

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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	msg, err := client.ReadMessage(ctx, agentID, msgID)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(msg)
	default:
		fmt.Print(formatMessageFull(msg))
	}

	return nil
}
