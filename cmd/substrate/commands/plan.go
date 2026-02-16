package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
	"github.com/spf13/cobra"
)

// planCmd is the parent command for plan review operations.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage plan reviews",
	Long: `Submit, review, and manage implementation plan reviews.

Plans are submitted for human review before agents proceed with
implementation. The review workflow supports approve, reject, and
request-changes decisions.`,
}

// planSubmitCmd submits a plan for review.
var planSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a plan for review",
	Long: `Submit an implementation plan for human review. Reads plan
content from the plan context file or --file flag, generates an AI
summary, sends a [PLAN] mail to the reviewer, and creates a plan
review record.`,
	RunE: runPlanSubmit,
}

// planWaitCmd blocks until a plan review decision is made.
var planWaitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for a plan review decision",
	Long: `Block until the plan review is approved, rejected, or
changes are requested. Polls the plan review state every 5 seconds
and also checks for keyword-based replies in the thread.`,
	RunE: runPlanWait,
}

// planStatusCmd shows the current plan review status.
var planStatusCmd = &cobra.Command{
	Use:   "status [plan-review-id]",
	Short: "Check plan review status",
	Long: `Show the current state of a plan review. If no ID is given,
looks up the pending review for the current session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPlanStatus,
}

// planApproveCmd approves a plan review.
var planApproveCmd = &cobra.Command{
	Use:   "approve <plan-review-id>",
	Short: "Approve a plan",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanApprove,
}

// planRejectCmd rejects a plan review.
var planRejectCmd = &cobra.Command{
	Use:   "reject <plan-review-id>",
	Short: "Reject a plan",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanReject,
}

// planRequestChangesCmd requests changes to a plan.
var planRequestChangesCmd = &cobra.Command{
	Use:   "request-changes <plan-review-id>",
	Short: "Request changes to a plan",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanRequestChanges,
}

// Plan command flags.
var (
	planFile      string
	planCwd       string
	planTo        string
	planNoAI      bool
	planSessionID string

	planReviewID string
	planTimeout  time.Duration
	planComment  string
)

func init() {
	// Submit flags.
	planSubmitCmd.Flags().StringVar(
		&planFile, "file", "",
		"Path to plan file (overrides context file)",
	)
	planSubmitCmd.Flags().StringVar(
		&planCwd, "cwd", "",
		"Working directory for plan context",
	)
	planSubmitCmd.Flags().StringVar(
		&planTo, "to", "User",
		"Reviewer name",
	)
	planSubmitCmd.Flags().BoolVar(
		&planNoAI, "no-ai", false,
		"Skip AI summarization, use regex fallback",
	)
	planSubmitCmd.Flags().StringVar(
		&planSessionID, "session-id", "",
		"Session ID (defaults to --session-id or $CLAUDE_SESSION_ID)",
	)

	// Wait flags.
	planWaitCmd.Flags().StringVar(
		&planReviewID, "plan-review-id", "",
		"Plan review ID to wait for",
	)
	planWaitCmd.Flags().DurationVar(
		&planTimeout, "timeout", 9*time.Minute,
		"Maximum time to wait for a decision",
	)
	planWaitCmd.Flags().StringVar(
		&planSessionID, "session-id", "",
		"Session ID (defaults to --session-id or $CLAUDE_SESSION_ID)",
	)

	// Approve/reject/request-changes flags.
	planApproveCmd.Flags().StringVar(
		&planComment, "comment", "",
		"Reviewer comment",
	)
	planRejectCmd.Flags().StringVar(
		&planComment, "comment", "",
		"Reviewer comment",
	)
	planRequestChangesCmd.Flags().StringVar(
		&planComment, "comment", "",
		"Reviewer comment",
	)

	// Register subcommands.
	planCmd.AddCommand(planSubmitCmd)
	planCmd.AddCommand(planWaitCmd)
	planCmd.AddCommand(planStatusCmd)
	planCmd.AddCommand(planApproveCmd)
	planCmd.AddCommand(planRejectCmd)
	planCmd.AddCommand(planRequestChangesCmd)
}

// =============================================================================
// Plan context file I/O
// =============================================================================

// planContext represents a single entry in the plan context file.
type planContext struct {
	SessionID string `json:"session_id"`
	PlanPath  string `json:"plan_path"`
	Timestamp int64  `json:"timestamp"`
}

// loadPlanContext reads plan context entries from the JSON lines file.
func loadPlanContext(cwd string) ([]planContext, error) {
	path := filepath.Join(cwd, ".claude", ".substrate-plan-context")

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open plan context: %w", err)
	}
	defer f.Close()

	var entries []planContext
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry planContext
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// =============================================================================
// Plan content extraction helpers
// =============================================================================

// extractPlanTitle extracts the title from a plan's markdown content.
// Returns the first H1 heading or a fallback based on the filename.
func extractPlanTitle(content, path string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	// Fallback to filename.
	if path != "" {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext)
	}

	return "Implementation Plan"
}

// regexSummaryPatterns are section headings that typically contain a summary.
var regexSummaryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^##\s+(Summary|Overview|TL;?DR)`),
	regexp.MustCompile(`(?i)^##\s+Context`),
}

// extractRegexSummary extracts a summary from plan content using regex.
// Looks for ## Summary, ## Overview, or ## Context sections and returns
// the first few lines of content.
func extractRegexSummary(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range regexSummaryPatterns {
			if pattern.MatchString(trimmed) {
				return extractSectionContent(lines, i+1, 5)
			}
		}
	}

	// Fallback: first non-heading, non-empty paragraph.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "---") {

			continue
		}

		return trimmed
	}

	return ""
}

// extractSectionContent extracts up to maxLines of non-empty content
// starting at startIdx, stopping at the next heading.
func extractSectionContent(lines []string, startIdx, maxLines int) string {
	var result []string
	for i := startIdx; i < len(lines) && len(result) < maxLines; i++ {
		line := strings.TrimSpace(lines[i])

		// Stop at next heading.
		if strings.HasPrefix(line, "#") {
			break
		}

		if line != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// extractFilesSection extracts the "Files to Modify" or similar section.
func extractFilesSection(content string) string {
	lines := strings.Split(content, "\n")
	filePattern := regexp.MustCompile(
		`(?i)^##\s+(Files|Files to (Modify|Change|Create))`,
	)

	for i, line := range lines {
		if filePattern.MatchString(strings.TrimSpace(line)) {
			return extractSectionContent(lines, i+1, 20)
		}
	}

	return ""
}

// =============================================================================
// AI summarization
// =============================================================================

// summarizePlan generates an AI summary of plan content using Claude SDK
// with Haiku model. Returns empty string on any error (caller should fall
// back to regex extraction).
func summarizePlan(content, title string) string {
	prompt := fmt.Sprintf(
		"Summarize this implementation plan in 2-3 concise sentences. "+
			"Focus on what will be built and the key approach.\n\n"+
			"Plan Title: %s\n\n%s",
		title, content,
	)

	// Use Haiku for fast, cheap summarization.
	opts := []claudeagent.Option{
		claudeagent.WithModel("haiku"),
		claudeagent.WithMaxTurns(1),
		claudeagent.WithNoSessionPersistence(),
		claudeagent.WithSystemPrompt(
			"You are a concise technical summarizer. " +
				"Output only the summary, no preamble.",
		),
	}

	client, err := claudeagent.NewClient(opts...)
	if err != nil {
		return ""
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(), 30*time.Second,
	)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return ""
	}

	var summary string
	for msg := range client.Query(ctx, prompt) {
		switch m := msg.(type) {
		case claudeagent.AssistantMessage:
			text := m.ContentText()
			if text != "" {
				summary += text
			}
		case claudeagent.ResultMessage:
			// Final result — iteration ends.
		}
	}

	return strings.TrimSpace(summary)
}

// =============================================================================
// Plan message formatting
// =============================================================================

// cleanSubject sanitizes a subject line for mail.
func cleanSubject(s string) string {
	s = strings.TrimSpace(s)
	// Replace newlines with spaces.
	s = strings.ReplaceAll(s, "\n", " ")
	// Truncate if too long.
	if len(s) > 200 {
		s = s[:200] + "..."
	}

	return s
}

// formatPlanMessage builds a structured mail body for a plan submission.
func formatPlanMessage(
	content, path, summary string,
) string {
	var b strings.Builder

	if summary != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", summary)
	}

	// Include files section if present in the plan.
	files := extractFilesSection(content)
	if files != "" {
		fmt.Fprintf(&b, "## Key Files\n\n%s\n\n", files)
	}

	fmt.Fprintf(&b, "## Plan Details\n\n")
	fmt.Fprintf(&b, "**File**: `%s`\n\n", path)
	fmt.Fprintf(&b, "---\n\n")
	b.WriteString(content)

	return b.String()
}

// =============================================================================
// Submit command
// =============================================================================

// runPlanSubmit implements the `substrate plan submit` command.
func runPlanSubmit(cmd *cobra.Command, args []string) error {
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

	// Resolve session ID.
	sessID := resolvePlanSessionID()

	// Load plan content.
	content, planPath, err := loadPlanContent(sessID)
	if err != nil {
		return err
	}

	// Extract title and generate summary.
	title := extractPlanTitle(content, planPath)
	var summary string
	if !planNoAI {
		summary = summarizePlan(content, title)
	}
	if summary == "" {
		summary = extractRegexSummary(content)
	}

	// Send mail to reviewer.
	subject := cleanSubject(fmt.Sprintf("[PLAN] %s", title))
	body := formatPlanMessage(content, planPath, summary)

	msgID, threadID, err := client.SendMail(ctx, mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{planTo},
		Subject:        subject,
		Body:           body,
		Priority:       mail.PriorityNormal,
	})
	if err != nil {
		return fmt.Errorf("send plan mail: %w", err)
	}

	// Create plan review record. V7 UUID failure is non-fatal since
	// the function returns a valid V4 fallback.
	prID, err := newPlanReviewID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	review, err := client.CreatePlanReview(
		ctx, store.CreatePlanReviewParams{
			PlanReviewID: prID,
			MessageID:    &msgID,
			ThreadID:     threadID,
			RequesterID:  agentID,
			ReviewerName: planTo,
			PlanPath:     planPath,
			PlanTitle:    title,
			PlanSummary:  summary,
			SessionID:    sessID,
		},
	)
	if err != nil {
		return fmt.Errorf("create plan review: %w", err)
	}

	// Output result.
	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"plan_review_id": review.PlanReviewID,
			"message_id":     msgID,
			"thread_id":      threadID,
			"title":          title,
			"state":          review.State,
		})
	case "hook":
		return outputJSON(map[string]any{
			"plan_review_id": review.PlanReviewID,
			"thread_id":      threadID,
		})
	case "context":
		fmt.Printf("plan_review_id=%s thread_id=%s\n",
			review.PlanReviewID, threadID,
		)
	default:
		fmt.Printf("Plan submitted for review.\n")
		fmt.Printf("  Review ID: %s\n", review.PlanReviewID)
		fmt.Printf("  Title:     %s\n", title)
		fmt.Printf("  Reviewer:  %s\n", planTo)
		fmt.Printf("  Thread:    %s\n", threadID)
	}

	return nil
}

// loadPlanContent reads plan content from --file flag or context file.
func loadPlanContent(sessID string) (string, string, error) {
	// If --file is specified, read from that directly.
	if planFile != "" {
		data, err := os.ReadFile(planFile)
		if err != nil {
			return "", "", fmt.Errorf("read plan file: %w", err)
		}
		return string(data), planFile, nil
	}

	// Otherwise, load from plan context file.
	cwd := planCwd
	if cwd == "" {
		cwd = os.Getenv("CLAUDE_CWD")
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("get cwd: %w", err)
		}
	}

	entries, err := loadPlanContext(cwd)
	if err != nil {
		return "", "", err
	}

	if len(entries) == 0 {
		return "", "", fmt.Errorf(
			"no plan context found; use --file or write a " +
				"plan to ~/.claude/plans/ first",
		)
	}

	// Find the most recent entry for this session (or any entry).
	var best *planContext
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if sessID != "" && e.SessionID == sessID {
			best = &e
			break
		}
	}
	if best == nil {
		best = &entries[len(entries)-1]
	}

	data, err := os.ReadFile(best.PlanPath)
	if err != nil {
		return "", "", fmt.Errorf(
			"read plan file %s: %w", best.PlanPath, err,
		)
	}

	return string(data), best.PlanPath, nil
}

// resolvePlanSessionID returns the session ID from flags or env.
func resolvePlanSessionID() string {
	if planSessionID != "" {
		return planSessionID
	}
	if sessionID != "" {
		return sessionID
	}
	return os.Getenv("CLAUDE_SESSION_ID")
}

// newPlanReviewID generates a new UUID for a plan review. Uses V7
// (time-ordered) when available, falling back to V4 with a warning.
func newPlanReviewID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		fallbackID := uuid.New()
		return fallbackID.String(), fmt.Errorf(
			"V7 UUID failed, using V4 fallback: %w", err,
		)
	}
	return id.String(), nil
}

// =============================================================================
// Wait command
// =============================================================================

// planWaitPollInterval is how often to check for state changes.
const planWaitPollInterval = 5 * time.Second

// keyword patterns for detecting approval/rejection in thread replies.
var (
	approveKeywords = regexp.MustCompile(
		`(?i)\b(approve[ds]?|lgtm|looks?\s+good|ship\s+it)\b`,
	)
	rejectKeywords = regexp.MustCompile(
		`(?i)\b(reject(ed)?|nack|do\s+not\s+proceed)\b`,
	)
	changesKeywords = regexp.MustCompile(
		`(?i)\b(changes?\s+request(ed)?|needs?\s+changes?|revise|rework)\b`,
	)
)

// runPlanWait implements the `substrate plan wait` command.
func runPlanWait(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Resolve which plan review to wait for.
	prID := planReviewID
	if prID == "" {
		// Try to find by session.
		sessID := resolvePlanSessionID()
		if sessID == "" {
			return fmt.Errorf(
				"--plan-review-id or --session-id required",
			)
		}

		review, err := client.GetPlanReviewBySession(ctx, sessID)
		if err != nil {
			return fmt.Errorf(
				"no pending plan review for session: %w", err,
			)
		}
		prID = review.PlanReviewID
	}

	deadline := time.Now().Add(planTimeout)

	for {
		// Check plan review state.
		review, err := client.GetPlanReview(ctx, prID)
		if err != nil {
			return fmt.Errorf("get plan review: %w", err)
		}

		if review.State != "pending" {
			return outputPlanDecision(review)
		}

		// Check thread for keyword-based replies.
		state := checkThreadForKeywords(ctx, client, review)
		if state != "" {
			// Update the DB state.
			err := client.UpdatePlanReviewState(
				ctx, store.UpdatePlanReviewStateParams{
					PlanReviewID:    prID,
					State:           state,
					ReviewerComment: "Detected from thread reply",
				},
			)
			if err != nil {
				return fmt.Errorf(
					"update plan review state: %w", err,
				)
			}

			// Re-fetch updated review.
			review, err = client.GetPlanReview(ctx, prID)
			if err != nil {
				return fmt.Errorf("get plan review: %w", err)
			}
			return outputPlanDecision(review)
		}

		// Check timeout.
		if time.Now().After(deadline) {
			return outputPlanTimeout(review)
		}

		time.Sleep(planWaitPollInterval)
	}
}

// checkThreadForKeywords scans thread replies for approval/rejection
// keywords. Returns the detected state or empty string.
func checkThreadForKeywords(
	ctx context.Context, client *Client, review store.PlanReview,
) string {
	agentID := review.RequesterID
	messages, err := client.ReadThread(ctx, agentID, review.ThreadID)
	if err != nil {
		return ""
	}

	// Skip the original plan message (first in thread), check replies.
	for _, msg := range messages {
		// Skip messages from the requester.
		if msg.SenderID == review.RequesterID {
			continue
		}

		body := strings.ToLower(msg.Body)
		if approveKeywords.MatchString(body) {
			return "approved"
		}
		if rejectKeywords.MatchString(body) {
			return "rejected"
		}
		if changesKeywords.MatchString(body) {
			return "changes_requested"
		}
	}

	return ""
}

// outputPlanDecision outputs the plan review decision in the appropriate
// format.
func outputPlanDecision(review store.PlanReview) error {
	switch outputFormat {
	case "hook":
		return outputHookDecision(review)
	case "json":
		return outputJSON(map[string]any{
			"plan_review_id":   review.PlanReviewID,
			"state":            review.State,
			"reviewer_comment": review.ReviewerComment,
			"reviewed_at":      review.ReviewedAt,
		})
	default:
		fmt.Printf("Plan review %s: %s\n",
			review.PlanReviewID, review.State,
		)
		if review.ReviewerComment != "" {
			fmt.Printf("Comment: %s\n", review.ReviewerComment)
		}
	}

	return nil
}

// outputHookDecision outputs a hook-format JSON decision.
func outputHookDecision(review store.PlanReview) error {
	switch review.State {
	case "approved":
		comment := "Plan approved."
		if review.ReviewerComment != "" {
			comment = fmt.Sprintf(
				"Plan approved: %s", review.ReviewerComment,
			)
		}
		return outputJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"permissionDecision": "allow",
				"additionalContext":  comment,
			},
		})

	case "rejected", "changes_requested":
		reason := fmt.Sprintf("Plan %s.", review.State)
		if review.ReviewerComment != "" {
			reason = fmt.Sprintf(
				"Plan %s: %s",
				review.State, review.ReviewerComment,
			)
		}
		return outputJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"permissionDecision":       "deny",
				"permissionDecisionReason": reason,
			},
		})

	default:
		return outputPlanTimeout(review)
	}
}

// outputPlanTimeout outputs a timeout decision for hook format.
func outputPlanTimeout(review store.PlanReview) error {
	switch outputFormat {
	case "hook":
		return outputJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"permissionDecision": "deny",
				"permissionDecisionReason": "Plan still pending " +
					"review. Waiting for approval...",
			},
		})
	case "json":
		return outputJSON(map[string]any{
			"plan_review_id": review.PlanReviewID,
			"state":          "timeout",
			"message":        "Plan review timed out waiting for decision",
		})
	default:
		fmt.Printf("Timed out waiting for plan review %s\n",
			review.PlanReviewID,
		)
	}

	return nil
}

// =============================================================================
// Status command
// =============================================================================

// runPlanStatus implements the `substrate plan status` command.
func runPlanStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var review store.PlanReview

	if len(args) > 0 {
		review, err = client.GetPlanReview(ctx, args[0])
	} else {
		sessID := resolvePlanSessionID()
		if sessID == "" {
			return fmt.Errorf(
				"provide a plan-review-id or --session-id",
			)
		}
		review, err = client.GetPlanReviewBySession(ctx, sessID)
	}

	if err != nil {
		return fmt.Errorf("get plan review: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"plan_review_id":   review.PlanReviewID,
			"title":            review.PlanTitle,
			"state":            review.State,
			"reviewer_name":    review.ReviewerName,
			"reviewer_comment": review.ReviewerComment,
			"created_at":       review.CreatedAt,
			"updated_at":       review.UpdatedAt,
			"reviewed_at":      review.ReviewedAt,
		})
	default:
		fmt.Printf("Plan Review: %s\n", review.PlanReviewID)
		fmt.Printf("  Title:    %s\n", review.PlanTitle)
		fmt.Printf("  State:    %s\n", review.State)
		fmt.Printf("  Reviewer: %s\n", review.ReviewerName)
		if review.ReviewerComment != "" {
			fmt.Printf("  Comment:  %s\n",
				review.ReviewerComment,
			)
		}
		fmt.Printf("  Created:  %s\n",
			review.CreatedAt.Format(time.RFC3339),
		)
		if review.ReviewedAt != nil {
			fmt.Printf("  Reviewed: %s\n",
				review.ReviewedAt.Format(time.RFC3339),
			)
		}
	}

	return nil
}

// =============================================================================
// Approve/Reject/Request-Changes commands
// =============================================================================

// runPlanApprove implements the `substrate plan approve` command.
func runPlanApprove(cmd *cobra.Command, args []string) error {
	return updatePlanReview(args[0], "approved")
}

// runPlanReject implements the `substrate plan reject` command.
func runPlanReject(cmd *cobra.Command, args []string) error {
	return updatePlanReview(args[0], "rejected")
}

// runPlanRequestChanges implements the `substrate plan request-changes`
// command.
func runPlanRequestChanges(cmd *cobra.Command, args []string) error {
	return updatePlanReview(args[0], "changes_requested")
}

// updatePlanReview updates a plan review state and sends notification
// mail.
func updatePlanReview(prID, state string) error {
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

	// Fetch the review to get thread info.
	review, err := client.GetPlanReview(ctx, prID)
	if err != nil {
		return fmt.Errorf("get plan review: %w", err)
	}

	// Update state.
	err = client.UpdatePlanReviewState(
		ctx, store.UpdatePlanReviewStateParams{
			PlanReviewID:    prID,
			State:           state,
			ReviewerComment: planComment,
			ReviewedBy:      &agentID,
		},
	)
	if err != nil {
		return fmt.Errorf("update plan review state: %w", err)
	}

	// Build notification subject.
	var subjectPrefix string
	switch state {
	case "approved":
		subjectPrefix = "[PLAN APPROVED]"
	case "rejected":
		subjectPrefix = "[PLAN REJECTED]"
	case "changes_requested":
		subjectPrefix = "[PLAN CHANGES REQUESTED]"
	}

	subject := cleanSubject(
		fmt.Sprintf("%s %s", subjectPrefix, review.PlanTitle),
	)

	// Build notification body.
	var body strings.Builder
	fmt.Fprintf(&body, "Plan review decision: **%s**\n\n", state)
	fmt.Fprintf(&body, "**Plan**: %s\n", review.PlanTitle)
	if planComment != "" {
		fmt.Fprintf(&body, "\n**Comment**: %s\n", planComment)
	}

	// Send reply in the plan thread.
	_, _, err = client.SendMail(ctx, mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{review.ReviewerName},
		Subject:        subject,
		Body:           body.String(),
		Priority:       mail.PriorityNormal,
		ThreadID:       review.ThreadID,
	})
	if err != nil {
		// Log but don't fail — the state update succeeded.
		if verbose {
			fmt.Fprintf(os.Stderr,
				"Warning: failed to send thread reply: %v\n",
				err,
			)
		}
	}

	// Send direct notification mail to the requesting agent.
	// Look up the requester's agent name for addressing.
	requester, err := client.GetAgent(ctx, review.RequesterID)
	if err == nil && requester != nil {
		_, _, sendErr := client.SendMail(ctx, mail.SendMailRequest{
			SenderID:       agentID,
			RecipientNames: []string{requester.Name},
			Subject:        subject,
			Body:           body.String(),
			Priority:       mail.PriorityUrgent,
		})
		if sendErr != nil && verbose {
			fmt.Fprintf(os.Stderr,
				"Warning: failed to notify requester: %v\n",
				sendErr,
			)
		}
	}

	// Output.
	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"plan_review_id": prID,
			"state":          state,
			"comment":        planComment,
		})
	default:
		fmt.Printf("Plan review %s: %s\n", prID, state)
		if planComment != "" {
			fmt.Printf("Comment: %s\n", planComment)
		}
	}

	return nil
}

// =============================================================================
// Client methods for plan reviews
// =============================================================================

// CreatePlanReview creates a new plan review record.
func (c *Client) CreatePlanReview(
	ctx context.Context, params store.CreatePlanReviewParams,
) (store.PlanReview, error) {
	if c.mode == ModeDirect {
		s := store.FromDB(c.store.DB())
		defer s.Close()

		return s.CreatePlanReview(ctx, params)
	}

	// gRPC mode.
	req := &subtraterpc.CreatePlanReviewRequest{
		PlanReviewId: params.PlanReviewID,
		ThreadId:     params.ThreadID,
		RequesterId:  params.RequesterID,
		ReviewerName: params.ReviewerName,
		PlanPath:     params.PlanPath,
		PlanTitle:    params.PlanTitle,
		PlanSummary:  params.PlanSummary,
		SessionId:    params.SessionID,
	}
	if params.MessageID != nil {
		req.MessageId = *params.MessageID
	}

	resp, err := c.planReviewClient.CreatePlanReview(ctx, req)
	if err != nil {
		return store.PlanReview{}, err
	}

	return planReviewFromProto(resp), nil
}

// GetPlanReview retrieves a plan review by its UUID.
func (c *Client) GetPlanReview(
	ctx context.Context, planReviewID string,
) (store.PlanReview, error) {
	if c.mode == ModeDirect {
		s := store.FromDB(c.store.DB())
		defer s.Close()

		return s.GetPlanReview(ctx, planReviewID)
	}

	resp, err := c.planReviewClient.GetPlanReview(
		ctx, &subtraterpc.GetPlanReviewRequest{
			PlanReviewId: planReviewID,
		},
	)
	if err != nil {
		return store.PlanReview{}, err
	}

	return planReviewFromProto(resp), nil
}

// GetPlanReviewBySession retrieves the pending plan review for a session.
func (c *Client) GetPlanReviewBySession(
	ctx context.Context, sessionID string,
) (store.PlanReview, error) {
	if c.mode == ModeDirect {
		s := store.FromDB(c.store.DB())
		defer s.Close()

		return s.GetPlanReviewBySession(ctx, sessionID)
	}

	resp, err := c.planReviewClient.GetPlanReviewBySession(
		ctx, &subtraterpc.GetPlanReviewBySessionRequest{
			SessionId: sessionID,
		},
	)
	if err != nil {
		return store.PlanReview{}, err
	}

	return planReviewFromProto(resp), nil
}

// UpdatePlanReviewState updates the state of a plan review.
func (c *Client) UpdatePlanReviewState(
	ctx context.Context, params store.UpdatePlanReviewStateParams,
) error {
	if c.mode == ModeDirect {
		s := store.FromDB(c.store.DB())
		defer s.Close()

		return s.UpdatePlanReviewState(ctx, params)
	}

	req := &subtraterpc.UpdatePlanReviewStatusRequest{
		PlanReviewId:    params.PlanReviewID,
		State:           params.State,
		ReviewerComment: params.ReviewerComment,
	}
	if params.ReviewedBy != nil {
		req.ReviewedBy = *params.ReviewedBy
	}

	_, err := c.planReviewClient.UpdatePlanReviewStatus(ctx, req)
	return err
}

// planReviewFromProto converts a PlanReviewProto to a store.PlanReview.
func planReviewFromProto(p *subtraterpc.PlanReviewProto) store.PlanReview {
	pr := store.PlanReview{
		ID:              p.Id,
		PlanReviewID:    p.PlanReviewId,
		ThreadID:        p.ThreadId,
		RequesterID:     p.RequesterId,
		ReviewerName:    p.ReviewerName,
		PlanPath:        p.PlanPath,
		PlanTitle:       p.PlanTitle,
		PlanSummary:     p.PlanSummary,
		State:           p.State,
		ReviewerComment: p.ReviewerComment,
		SessionID:       p.SessionId,
		CreatedAt:       time.Unix(p.CreatedAt, 0),
		UpdatedAt:       time.Unix(p.UpdatedAt, 0),
	}

	if p.MessageId != 0 {
		msgID := p.MessageId
		pr.MessageID = &msgID
	}
	if p.ReviewedBy != 0 {
		reviewedBy := p.ReviewedBy
		pr.ReviewedBy = &reviewedBy
	}
	if p.ReviewedAt != 0 {
		t := time.Unix(p.ReviewedAt, 0)
		pr.ReviewedAt = &t
	}

	return pr
}
