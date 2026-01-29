package commands

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

// ackCmd acknowledges a message.
var ackCmd = &cobra.Command{
	Use:   "ack <message_id>",
	Short: "Acknowledge a message",
	Long:  `Mark a message as acknowledged.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAck,
}

// starCmd stars a message.
var starCmd = &cobra.Command{
	Use:   "star <message_id>",
	Short: "Star a message",
	Long:  `Star a message for later.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runStar,
}

// snoozeCmd snoozes a message.
var snoozeCmd = &cobra.Command{
	Use:   "snooze <message_id>",
	Short: "Snooze a message",
	Long:  `Snooze a message until a specified time.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSnooze,
}

// archiveCmd archives a message.
var archiveCmd = &cobra.Command{
	Use:   "archive <message_id>",
	Short: "Archive a message",
	Long:  `Move a message to the archive.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runArchive,
}

// trashCmd moves a message to trash.
var trashCmd = &cobra.Command{
	Use:   "trash <message_id>",
	Short: "Move a message to trash",
	Long:  `Move a message to the trash.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTrash,
}

var snoozeUntil string

func init() {
	snoozeCmd.Flags().StringVar(&snoozeUntil, "until", "",
		"When to wake up (e.g., '2h', '2026-01-29T10:00:00') (required)")
	snoozeCmd.MarkFlagRequired("until")
}

func runAck(cmd *cobra.Command, args []string) error {
	return runMessageAction(args[0], "ack", nil)
}

func runStar(cmd *cobra.Command, args []string) error {
	return runMessageAction(args[0], "starred", nil)
}

func runSnooze(cmd *cobra.Command, args []string) error {
	t, err := parseDuration(snoozeUntil)
	if err != nil {
		return fmt.Errorf("invalid snooze time: %w", err)
	}
	return runMessageAction(args[0], "snoozed", &t)
}

func runArchive(cmd *cobra.Command, args []string) error {
	return runMessageAction(args[0], "archived", nil)
}

func runTrash(cmd *cobra.Command, args []string) error {
	return runMessageAction(args[0], "trash", nil)
}

func runMessageAction(msgIDStr string, action string,
	snoozedUntil *time.Time) error {

	ctx := context.Background()

	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
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

	// Handle ack specially.
	if action == "ack" {
		req := mail.AckMessageRequest{
			AgentID:   agentID,
			MessageID: msgID,
		}

		result := svc.Receive(ctx, req)
		val, err := result.Unpack()
		if err != nil {
			return err
		}

		resp := val.(mail.AckMessageResponse)
		if resp.Error != nil {
			return resp.Error
		}

		fmt.Printf("Message #%d acknowledged.\n", msgID)
		return nil
	}

	// Other state changes.
	req := mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    msgID,
		NewState:     action,
		SnoozedUntil: snoozedUntil,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.UpdateStateResponse)
	if resp.Error != nil {
		return resp.Error
	}

	fmt.Printf("Message #%d moved to %s.\n", msgID, action)
	return nil
}
