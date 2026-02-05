package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	sendTo       string
	sendSubject  string
	sendBody     string
	sendBodyFile string
	sendPriority string
	sendDeadline string
	sendThreadID string
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message",
	Long: `Send a message to another agent or topic.

The message body can be specified inline with --body, or read from a
file with --body-file. When both are provided, --body-file takes
precedence. Use --body-file for long multi-line markdown content to
avoid shell quoting issues.`,
	RunE: runSend,
}

func init() {
	sendCmd.Flags().StringVar(&sendTo, "to", "",
		"Recipient agent name or topic (required)")
	sendCmd.Flags().StringVar(&sendSubject, "subject", "",
		"Message subject (required)")
	sendCmd.Flags().StringVar(&sendBody, "body", "",
		"Message body in markdown")
	sendCmd.Flags().StringVar(&sendBodyFile, "body-file", "",
		"Read message body from file (overrides --body)")
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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Resolve the message body. --body-file takes precedence over
	// --body so agents can write rich markdown to a temp file and
	// avoid shell quoting issues with long content.
	body := sendBody
	if sendBodyFile != "" {
		data, err := os.ReadFile(sendBodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		body = strings.TrimSpace(string(data))
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

	req := mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{sendTo},
		Subject:        sendSubject,
		Body:           body,
		Priority:       priority,
		Deadline:       deadline,
		ThreadID:       sendThreadID,
	}

	msgID, threadID, err := client.SendMail(ctx, req)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"message_id": msgID,
			"thread_id":  threadID,
		})
	default:
		fmt.Printf("Message sent! ID: %d, Thread: %s\n",
			msgID, threadID)
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

	return time.Time{}, fmt.Errorf(
		"cannot parse %q as duration or timestamp", s,
	)
}
