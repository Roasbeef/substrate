package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
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
// interface. It supports gRPC, direct database access, and queue mode.
func getCurrentAgentWithClient(ctx context.Context, client *Client) (int64,
	string, error,
) {
	// If agent name is specified directly, use it.
	if agentName != "" {
		// In queue mode, we can't resolve names to IDs — return
		// zero ID with the name so the caller can use the name
		// for queue payloads.
		if client.mode == ModeQueued {
			return 0, agentName, nil
		}

		ag, err := client.GetAgentByName(ctx, agentName)
		if err != nil {
			// Try to suggest a close match.
			if agents, listErr := client.ListAgents(
				ctx,
			); listErr == nil {
				names := make([]string, len(agents))
				for i, a := range agents {
					names[i] = a.Name
				}
				if suggestion := suggestClosestMatch(
					agentName, names,
				); suggestion != "" {
					return 0, "", fmt.Errorf(
						"agent %q not found; "+
							"did you mean %q?",
						agentName, suggestion,
					)
				}
			}
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

	// In queue mode, use cached identity from the filesystem.
	if client.mode == ModeQueued {
		return resolveQueuedIdentity(sessID)
	}

	// Use identity resolution via client.
	gitBranch := getGitBranch()
	identity, err := client.EnsureIdentity(ctx, sessID, projDir, gitBranch)
	if err != nil {
		return 0, "", fmt.Errorf("failed to resolve identity: %w", err)
	}

	return identity.AgentID, identity.AgentName, nil
}

// resolveQueuedIdentity loads a cached identity file for queue mode where
// the database is unavailable. The agent name is used in queue payloads
// and resolved to an ID at drain time.
func resolveQueuedIdentity(sessID string) (int64, string, error) {
	if sessID == "" {
		return 0, "", fmt.Errorf(
			"queue mode requires --session-id or " +
				"CLAUDE_SESSION_ID for identity resolution",
		)
	}

	identity, err := agent.LoadCachedIdentity(sessID)
	if err != nil {
		return 0, "", fmt.Errorf(
			"no cached identity for session %q (agent must have "+
				"been online at least once): %w",
			sessID, err,
		)
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
	fmt.Fprintf(&sb, "#%d: %s\n", msg.ID, msg.Subject)
	senderDisplay := msg.SenderName
	if senderDisplay == "" {
		senderDisplay = fmt.Sprintf("Agent#%d", msg.SenderID)
	}
	fmt.Fprintf(&sb, "  From: %s | %s\n",
		senderDisplay, msg.CreatedAt.Format(time.RFC3339))

	if msg.Deadline != nil {
		fmt.Fprintf(&sb, "  Deadline: %s\n",
			msg.Deadline.Format(time.RFC3339))
	}

	return sb.String()
}

// formatMessageFull formats a message with full body for reading.
func formatMessageFull(msg *mail.InboxMessage) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Message #%d\n", msg.ID)
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	fmt.Fprintf(&sb, "Subject: %s\n", msg.Subject)
	senderDisplay := msg.SenderName
	if senderDisplay == "" {
		senderDisplay = fmt.Sprintf("Agent#%d", msg.SenderID)
	}
	fmt.Fprintf(&sb, "From: %s\n", senderDisplay)
	fmt.Fprintf(&sb, "Thread: %s\n", msg.ThreadID)
	fmt.Fprintf(&sb, "Priority: %s\n", msg.Priority)
	fmt.Fprintf(&sb, "State: %s\n", msg.State)
	fmt.Fprintf(&sb, "Created: %s\n",
		msg.CreatedAt.Format(time.RFC3339))

	if msg.Deadline != nil {
		fmt.Fprintf(&sb, "Deadline: %s\n",
			msg.Deadline.Format(time.RFC3339))
	}

	if msg.ReadAt != nil {
		fmt.Fprintf(&sb, "Read: %s\n",
			msg.ReadAt.Format(time.RFC3339))
	}

	if msg.AckedAt != nil {
		fmt.Fprintf(&sb, "Acked: %s\n",
			msg.AckedAt.Format(time.RFC3339))
	}

	sb.WriteString(strings.Repeat("-", 60) + "\n")
	sb.WriteString(msg.Body + "\n")

	return sb.String()
}

// formatStatus formats agent status for output.
func formatStatus(status mail.AgentStatus) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Agent: %s (ID: %d)\n",
		status.AgentName, status.AgentID)
	sb.WriteString(strings.Repeat("-", 40) + "\n")
	fmt.Fprintf(&sb, "Unread: %d\n", status.UnreadCount)
	fmt.Fprintf(&sb, "Urgent: %d\n", status.UrgentCount)
	fmt.Fprintf(&sb, "Starred: %d\n", status.StarredCount)
	fmt.Fprintf(&sb, "Snoozed: %d\n", status.SnoozedCount)

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
		fmt.Fprintf(&sb, "You have %d URGENT", urgentCount)
		if normalCount > 0 {
			fmt.Fprintf(&sb, " and %d other", normalCount)
		}
		sb.WriteString(" unread messages:\n")
	} else {
		fmt.Fprintf(&sb, "You have %d unread messages:\n",
			len(msgs))
	}

	for _, msg := range msgs {
		sb.WriteString("- ")
		if msg.Priority == mail.PriorityUrgent {
			sb.WriteString("[URGENT] ")
		}
		senderDisplay := msg.SenderName
		if senderDisplay == "" {
			senderDisplay = fmt.Sprintf("Agent#%d", msg.SenderID)
		}
		// Include message ID and thread ID for replies.
		fmt.Fprintf(&sb, "#%d From: %s - %q (thread: %s)",
			msg.ID, senderDisplay, msg.Subject, msg.ThreadID)
		if msg.Deadline != nil {
			remaining := time.Until(*msg.Deadline)
			if remaining > 0 {
				fmt.Fprintf(&sb, " (deadline: %s)",
					formatDuration(remaining))
			} else {
				sb.WriteString(" (OVERDUE)")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nUse `substrate read <id>` to read a message.\n")
	sb.WriteString("To reply in thread: `substrate send --to <sender> " +
		"--thread <thread-id> --subject \"Re: ...\" --body \"...\"`")

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

// Exit codes for semantic error classification.
const (
	// ExitSuccess indicates the command completed successfully.
	ExitSuccess = 0

	// ExitError indicates a general error.
	ExitError = 1

	// ExitValidation indicates invalid arguments or input.
	ExitValidation = 2

	// ExitAuth indicates an authentication or authorization failure.
	ExitAuth = 3

	// ExitNotFound indicates a resource was not found.
	ExitNotFound = 4

	// ExitConflict indicates a conflict (e.g., already exists).
	ExitConflict = 5
)

// CLIError wraps an error with a semantic exit code for structured output.
type CLIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error.
func (e *CLIError) Unwrap() error {
	return e.Err
}

// ExitCode returns the semantic exit code for this error.
func (e *CLIError) ExitCode() int {
	return e.Code
}

// NewValidationError creates a CLIError for input validation failures.
func NewValidationError(msg string, err error) *CLIError {
	return &CLIError{Code: ExitValidation, Message: msg, Err: err}
}

// NewNotFoundError creates a CLIError for resource not found failures.
func NewNotFoundError(msg string, err error) *CLIError {
	return &CLIError{Code: ExitNotFound, Message: msg, Err: err}
}

// OutputError writes a structured error to stderr when format is JSON,
// or a plain text error otherwise.
func OutputError(err error) int {
	if outputFormat == "json" {
		code := ExitError
		if cliErr, ok := err.(*CLIError); ok {
			code = cliErr.Code
		}

		errObj := struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}{}
		errObj.Error.Code = code
		errObj.Error.Message = err.Error()

		data, _ := json.Marshal(errObj)
		fmt.Fprintln(os.Stderr, string(data))

		return code
	}

	fmt.Fprintln(os.Stderr, err)

	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.ExitCode()
	}

	return ExitError
}

// outputJSON outputs data as JSON, respecting --compact and --fields flags.
func outputJSON(v interface{}) error {
	// Apply field filtering if requested.
	if fieldsFilter != "" {
		v = filterJSONFields(v, strings.Split(fieldsFilter, ","))
	}

	if compact {
		return outputCompactJSON(v)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	return nil
}

// outputCompactJSON outputs JSON in compact single-line form.
func outputCompactJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	return nil
}

// filterJSONFields filters a JSON-serializable value to only include
// the specified top-level fields. Works on maps and slices of maps.
func filterJSONFields(v interface{}, fields []string) interface{} {
	// Marshal to JSON and back to map for generic field filtering.
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}

	// Try as array of objects.
	var arr []map[string]interface{}
	if json.Unmarshal(data, &arr) == nil && len(arr) > 0 {
		var filtered []map[string]interface{}
		for _, item := range arr {
			filtered = append(filtered, pickFields(item, fields))
		}
		return filtered
	}

	// Try as single object.
	var obj map[string]interface{}
	if json.Unmarshal(data, &obj) == nil {
		return pickFields(obj, fields)
	}

	return v
}

// pickFields returns a new map containing only the specified fields.
func pickFields(
	obj map[string]interface{}, fields []string,
) map[string]interface{} {
	result := make(map[string]interface{})
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if val, ok := obj[f]; ok {
			result[f] = val
		}
	}

	return result
}

// paginationEnvelope wraps list results with pagination metadata.
type paginationEnvelope struct {
	Items         interface{} `json:"items"`
	NextPageToken string      `json:"next_page_token,omitempty"`
}

// outputWithPagination wraps items in a pagination envelope for JSON output.
// If there are more results (len(items) == limit), a next_page_token is
// generated from the current offset + limit.
func outputWithPagination(
	items interface{}, offset, limit, count int,
) error {
	envelope := paginationEnvelope{Items: items}
	if count >= limit {
		nextOffset := offset + limit
		token := fmt.Sprintf("%d", nextOffset)
		envelope.NextPageToken = token
	}

	return outputJSON(envelope)
}

// maxInboxBodyLen is the maximum body length in inbox JSON output.
// Messages longer than this are truncated with a hint to use `read`.
const maxInboxBodyLen = 200

// truncateInboxBodies returns a copy of messages with bodies truncated
// for JSON output. This prevents large message bodies from flooding
// agent context windows. Use `substrate read <id>` for full content.
func truncateInboxBodies(msgs []mail.InboxMessage) []mail.InboxMessage {
	result := make([]mail.InboxMessage, len(msgs))
	copy(result, msgs)

	for i := range result {
		body := result[i].Body
		if len(body) <= maxInboxBodyLen {
			continue
		}

		hint := fmt.Sprintf(
			"... [truncated, use `substrate read %d` for full message]",
			result[i].ID,
		)
		result[i].Body = body[:maxInboxBodyLen] + hint
	}

	return result
}
