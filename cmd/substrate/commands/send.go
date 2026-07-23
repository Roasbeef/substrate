package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/queue"
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
	sendAttach   []string
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
	sendCmd.Flags().StringSliceVar(&sendAttach, "attach", nil,
		"Image file(s) to attach; copied into the shared "+
			"attachments dir and embedded as markdown")

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

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
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

	// Copy attachments into the shared attachments directory and
	// embed markdown references so the web UI renders them inline.
	if len(sendAttach) > 0 {
		refs, err := stageAttachments(sendAttach)
		if err != nil {
			return err
		}
		body = strings.TrimSpace(body + "\n\n" + refs)
	}

	// Validate and parse priority.
	if err := validateEnum(
		sendPriority, "priority",
		[]string{"urgent", "normal", "low"},
	); err != nil {
		return err
	}
	priority := mail.Priority(sendPriority)

	// Parse deadline if provided.
	var deadline *time.Time
	if sendDeadline != "" {
		d, err := parseDuration(sendDeadline)
		if err != nil {
			return fmt.Errorf("invalid deadline: %w", err)
		}
		deadline = &d
	}

	// In queue mode, enqueue the operation for later delivery.
	if client.Mode() == ModeQueued {
		return enqueueSend(
			ctx, client, agentNameStr, body,
			string(priority), deadline,
		)
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

// enqueueSend stores a send operation in the local queue for later delivery.
func enqueueSend(
	ctx context.Context, client *Client, senderName, body,
	priority string, deadline *time.Time,
) error {
	key := newIdempotencyKey()
	payload := queue.SendPayload{
		SenderName:     senderName,
		RecipientNames: []string{sendTo},
		Subject:        sendSubject,
		Body:           body,
		Priority:       priority,
		TopicName:      "",
		ThreadID:       sendThreadID,
		DeadlineAt:     deadline,
	}

	payloadJSON, err := queue.MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	now := time.Now()
	op := queue.PendingOperation{
		IdempotencyKey: key,
		OperationType:  queue.OpSend,
		PayloadJSON:    payloadJSON,
		AgentName:      senderName,
		SessionID:      sessionID,
		CreatedAt:      now,
		ExpiresAt:      now.Add(client.queueCfg.DefaultTTL),
	}

	if err := client.queueStore.Enqueue(ctx, op); err != nil {
		return fmt.Errorf("enqueue send: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"queued":          true,
			"idempotency_key": key,
		})
	default:
		fmt.Println("Message queued (offline)")
	}

	return nil
}

// attachmentExts whitelists attachable image types, mirroring the
// daemon's serving whitelist.
var attachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".svg": true,
}

// stageAttachments copies image files into the shared attachments
// directory (~/.subtrate/attachments) under random names and returns
// the markdown image references to append to a message body. The
// daemon serves that directory at /api/v1/attachments/.
func stageAttachments(paths []string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	dir := filepath.Join(home, ".subtrate", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create attachments dir: %w", err)
	}

	var refs []string
	for _, src := range paths {
		ext := strings.ToLower(filepath.Ext(src))
		if !attachmentExts[ext] {
			return "", fmt.Errorf(
				"unsupported attachment type: %s", src,
			)
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("read attachment: %w", err)
		}

		name := uuid.NewString() + ext
		dst := filepath.Join(dir, name)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return "", fmt.Errorf("write attachment: %w", err)
		}

		refs = append(refs, fmt.Sprintf(
			"![%s](/api/v1/attachments/%s)",
			filepath.Base(src), name,
		))
	}

	return strings.Join(refs, "\n"), nil
}
