package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/mail"
)

// getGitBranch returns the current git branch name, or empty string if not in
// a git repo or git is not available.
func getGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getStore opens the database and returns a store instance.
// NOTE: This is only used as fallback when daemon is not running.
func getStore() (*db.Store, error) {
	path := dbPath
	if path == "" {
		var err error
		path, err = db.DefaultDBPath()
		if err != nil {
			return nil, err
		}
	}

	sqlDB, err := db.OpenSQLite(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db.NewStore(sqlDB), nil
}

// getCurrentAgentWithClient returns the current agent ID using the Client
// interface. It supports both gRPC and direct database access.
func getCurrentAgentWithClient(ctx context.Context, client *Client) (int64,
	string, error) {

	// If agent name is specified directly, use it.
	if agentName != "" {
		ag, err := client.GetAgentByName(ctx, agentName)
		if err != nil {
			return 0, "", fmt.Errorf("agent %q not found: %w",
				agentName, err)
		}
		return ag.ID, ag.Name, nil
	}

	// Try to get session ID from environment.
	sessID := sessionID
	if sessID == "" {
		sessID = os.Getenv("CLAUDE_SESSION_ID")
	}

	projDir := projectDir
	if projDir == "" {
		projDir = os.Getenv("CLAUDE_PROJECT_DIR")
	}

	if sessID == "" && projDir == "" {
		return 0, "", fmt.Errorf("no agent specified; use --agent, " +
			"--session-id, or set CLAUDE_SESSION_ID")
	}

	// Use identity resolution via client.
	gitBranch := getGitBranch()
	identity, err := client.EnsureIdentity(ctx, sessID, projDir, gitBranch)
	if err != nil {
		return 0, "", fmt.Errorf("failed to resolve identity: %w", err)
	}

	return identity.AgentID, identity.AgentName, nil
}

// formatMessage formats a message for output.
func formatMessage(msg mail.InboxMessage) string {
	var sb strings.Builder

	// Priority indicator.
	switch msg.Priority {
	case mail.PriorityUrgent:
		sb.WriteString("[URGENT] ")
	case mail.PriorityLow:
		sb.WriteString("[low] ")
	}

	// State indicator.
	switch msg.State {
	case "unread":
		sb.WriteString("* ")
	case "starred":
		sb.WriteString("★ ")
	case "snoozed":
		sb.WriteString("⏰ ")
	}

	// Message summary.
	sb.WriteString(fmt.Sprintf("#%d: %s\n", msg.ID, msg.Subject))
	sb.WriteString(fmt.Sprintf("  From: Agent#%d | %s\n",
		msg.SenderID, msg.CreatedAt.Format(time.RFC3339)))

	if msg.Deadline != nil {
		sb.WriteString(fmt.Sprintf("  Deadline: %s\n",
			msg.Deadline.Format(time.RFC3339)))
	}

	return sb.String()
}

// formatMessageFull formats a message with full body for reading.
func formatMessageFull(msg *mail.InboxMessage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Message #%d\n", msg.ID))
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	sb.WriteString(fmt.Sprintf("Subject: %s\n", msg.Subject))
	sb.WriteString(fmt.Sprintf("From: Agent#%d\n", msg.SenderID))
	sb.WriteString(fmt.Sprintf("Thread: %s\n", msg.ThreadID))
	sb.WriteString(fmt.Sprintf("Priority: %s\n", msg.Priority))
	sb.WriteString(fmt.Sprintf("State: %s\n", msg.State))
	sb.WriteString(fmt.Sprintf("Created: %s\n",
		msg.CreatedAt.Format(time.RFC3339)))

	if msg.Deadline != nil {
		sb.WriteString(fmt.Sprintf("Deadline: %s\n",
			msg.Deadline.Format(time.RFC3339)))
	}

	if msg.ReadAt != nil {
		sb.WriteString(fmt.Sprintf("Read: %s\n",
			msg.ReadAt.Format(time.RFC3339)))
	}

	if msg.AckedAt != nil {
		sb.WriteString(fmt.Sprintf("Acked: %s\n",
			msg.AckedAt.Format(time.RFC3339)))
	}

	sb.WriteString(strings.Repeat("-", 60) + "\n")
	sb.WriteString(msg.Body + "\n")

	return sb.String()
}

// formatStatus formats agent status for output.
func formatStatus(status mail.AgentStatus) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Agent: %s (ID: %d)\n",
		status.AgentName, status.AgentID))
	sb.WriteString(strings.Repeat("-", 40) + "\n")
	sb.WriteString(fmt.Sprintf("Unread: %d\n", status.UnreadCount))
	sb.WriteString(fmt.Sprintf("Urgent: %d\n", status.UrgentCount))
	sb.WriteString(fmt.Sprintf("Starred: %d\n", status.StarredCount))
	sb.WriteString(fmt.Sprintf("Snoozed: %d\n", status.SnoozedCount))

	return sb.String()
}

// formatContext formats output for Claude Code hook injection.
func formatContext(msgs []mail.InboxMessage) string {
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Subtrate Mail] ")

	urgentCount := 0
	normalCount := 0
	for _, msg := range msgs {
		if msg.Priority == mail.PriorityUrgent {
			urgentCount++
		} else {
			normalCount++
		}
	}

	if urgentCount > 0 {
		sb.WriteString(fmt.Sprintf("You have %d URGENT", urgentCount))
		if normalCount > 0 {
			sb.WriteString(fmt.Sprintf(" and %d other", normalCount))
		}
		sb.WriteString(" unread messages:\n")
	} else {
		sb.WriteString(fmt.Sprintf("You have %d unread messages:\n",
			len(msgs)))
	}

	for _, msg := range msgs {
		sb.WriteString("- ")
		if msg.Priority == mail.PriorityUrgent {
			sb.WriteString("[URGENT] ")
		}
		sb.WriteString(fmt.Sprintf("From: Agent#%d - %q",
			msg.SenderID, msg.Subject))
		if msg.Deadline != nil {
			remaining := time.Until(*msg.Deadline)
			if remaining > 0 {
				sb.WriteString(fmt.Sprintf(" (deadline: %s)",
					formatDuration(remaining)))
			} else {
				sb.WriteString(" (OVERDUE)")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nUse `substrate inbox` to see details, or " +
		"`substrate read <id>` to read a message.")

	return sb.String()
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// outputJSON outputs data as JSON.
func outputJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
