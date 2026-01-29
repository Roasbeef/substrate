package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	sendTo       string
	sendSubject  string
	sendBody     string
	sendPriority string
	sendDeadline string
	sendThreadID string
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message",
	Long:  `Send a message to another agent or topic.`,
	RunE:  runSend,
}

func init() {
	sendCmd.Flags().StringVar(&sendTo, "to", "",
		"Recipient agent name or topic (required)")
	sendCmd.Flags().StringVar(&sendSubject, "subject", "",
		"Message subject (required)")
	sendCmd.Flags().StringVar(&sendBody, "body", "",
		"Message body in markdown")
	sendCmd.Flags().StringVar(&sendPriority, "priority", "normal",
		"Priority: urgent, normal, low")
	sendCmd.Flags().StringVar(&sendDeadline, "deadline", "",
		"Acknowledgment deadline (e.g., '2h', '2026-01-29T10:00:00')")
	sendCmd.Flags().StringVar(&sendThreadID, "thread", "",
		"Thread ID for replies")

	sendCmd.MarkFlagRequired("to")
	sendCmd.MarkFlagRequired("subject")
}

func runSend(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, _, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	// Parse priority.
	var priority mail.Priority
	switch sendPriority {
	case "urgent":
		priority = mail.PriorityUrgent
	case "normal":
		priority = mail.PriorityNormal
	case "low":
		priority = mail.PriorityLow
	default:
		return fmt.Errorf("invalid priority: %s", sendPriority)
	}

	// Parse deadline if provided.
	var deadline *time.Time
	if sendDeadline != "" {
		d, err := parseDuration(sendDeadline)
		if err != nil {
			return fmt.Errorf("invalid deadline: %w", err)
		}
		deadline = &d
	}

	svc := mail.NewService(store)

	req := mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{sendTo},
		Subject:        sendSubject,
		Body:           sendBody,
		Priority:       priority,
		Deadline:       deadline,
		ThreadID:       sendThreadID,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.SendMailResponse)
	if resp.Error != nil {
		return resp.Error
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"message_id": resp.MessageID,
			"thread_id":  resp.ThreadID,
		})
	default:
		fmt.Printf("Message sent! ID: %d, Thread: %s\n",
			resp.MessageID, resp.ThreadID)
	}

	return nil
}

// parseDuration parses a duration string or RFC3339 timestamp.
func parseDuration(s string) (time.Time, error) {
	// Try parsing as duration (e.g., "2h", "30m").
	d, err := time.ParseDuration(s)
	if err == nil {
		return time.Now().Add(d), nil
	}

	// Try parsing as RFC3339 timestamp.
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	// Try parsing as date.
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("cannot parse %q as duration or timestamp",
		s)
}
