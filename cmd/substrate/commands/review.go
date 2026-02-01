package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	// reviewListLimit is the maximum number of reviews to list.
	reviewListLimit int

	// reviewListState filters reviews by state.
	reviewListState string

	// reviewListMine shows only reviews requested by the current agent.
	reviewListMine bool

	// reviewListActive shows only active (not completed) reviews.
	reviewListActive bool

	// reviewRequestBranch is the branch to review.
	reviewRequestBranch string

	// reviewRequestBaseBranch is the base branch for the review.
	reviewRequestBaseBranch string

	// reviewRequestType is the type of review to request.
	reviewRequestType string

	// reviewRequestPriority is the priority of the review.
	reviewRequestPriority string

	// reviewRequestReviewer is the reviewer agent name.
	reviewRequestReviewer string

	// reviewShowIssues shows issues when displaying review status.
	reviewShowIssues bool

	// reviewShowIterations shows iterations when displaying review status.
	reviewShowIterations bool
)

// reviewCmd is the parent command for review operations.
var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Code review operations",
	Long: `Manage code reviews for PR branches.

Request reviews from specialized reviewer agents, check status, and list
pending reviews.`,
}

// reviewListCmd lists code reviews.
var reviewListCmd = &cobra.Command{
	Use:   "list",
	Short: "List code reviews",
	Long:  `List code reviews, optionally filtered by state or requester.`,
	RunE:  runReviewList,
}

// reviewStatusCmd shows the status of a review.
var reviewStatusCmd = &cobra.Command{
	Use:   "status [review-id]",
	Short: "Show review status",
	Long:  `Show the status of a code review, including iterations and issues.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runReviewStatus,
}

// reviewStatsCmd shows review statistics.
var reviewStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show review statistics",
	Long:  `Display aggregate statistics about code reviews.`,
	RunE:  runReviewStats,
}

// reviewRequestCmd requests a new code review.
var reviewRequestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request a code review",
	Long: `Request a code review for a branch.

This sends a review request message to a reviewer agent, which will
spawn a new Claude session to review the changes.`,
	RunE: runReviewRequest,
}

// reviewCancelCmd cancels an active review.
var reviewCancelCmd = &cobra.Command{
	Use:   "cancel <review-id>",
	Short: "Cancel a review",
	Long:  `Cancel an active code review.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewCancel,
}

func init() {
	// List command flags.
	reviewListCmd.Flags().IntVarP(&reviewListLimit, "limit", "n", 20,
		"Maximum number of reviews to list")
	reviewListCmd.Flags().StringVar(&reviewListState, "state", "",
		"Filter by state (new, pending_review, under_review, changes_requested, approved, rejected, cancelled)")
	reviewListCmd.Flags().BoolVar(&reviewListMine, "mine", false,
		"Show only reviews requested by the current agent")
	reviewListCmd.Flags().BoolVar(&reviewListActive, "active", false,
		"Show only active (not completed) reviews")

	// Status command flags.
	reviewStatusCmd.Flags().BoolVarP(&reviewShowIssues, "issues", "i", false,
		"Show issues found in the review")
	reviewStatusCmd.Flags().BoolVar(&reviewShowIterations, "iterations", false,
		"Show all review iterations")

	// Request command flags.
	reviewRequestCmd.Flags().StringVarP(&reviewRequestBranch, "branch", "b", "",
		"Branch to review (default: current branch)")
	reviewRequestCmd.Flags().StringVar(&reviewRequestBaseBranch, "base", "main",
		"Base branch for comparison")
	reviewRequestCmd.Flags().StringVarP(&reviewRequestType, "type", "t", "full",
		"Review type: full, incremental, security, performance")
	reviewRequestCmd.Flags().StringVar(&reviewRequestPriority, "priority", "normal",
		"Review priority: urgent, normal, low")
	reviewRequestCmd.Flags().StringVarP(&reviewRequestReviewer, "reviewer", "r", "",
		"Reviewer agent name (default: reviewer topic)")

	// Add subcommands.
	reviewCmd.AddCommand(reviewListCmd)
	reviewCmd.AddCommand(reviewStatusCmd)
	reviewCmd.AddCommand(reviewStatsCmd)
	reviewCmd.AddCommand(reviewRequestCmd)
	reviewCmd.AddCommand(reviewCancelCmd)
}

func runReviewList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var reviews []ReviewInfo

	switch {
	case reviewListMine:
		agentID, _, err := getCurrentAgentWithClient(ctx, client)
		if err != nil {
			return err
		}
		reviews, err = client.ListReviewsByRequester(ctx, agentID, reviewListLimit)
		if err != nil {
			return err
		}
	case reviewListState != "":
		reviews, err = client.ListReviewsByState(ctx, reviewListState, reviewListLimit)
		if err != nil {
			return err
		}
	case reviewListActive:
		reviews, err = client.ListActiveReviews(ctx, reviewListLimit)
		if err != nil {
			return err
		}
	default:
		reviews, err = client.ListReviews(ctx, reviewListLimit)
		if err != nil {
			return err
		}
	}

	switch outputFormat {
	case "json":
		return outputJSON(reviews)
	case "context":
		if len(reviews) == 0 {
			return nil
		}
		fmt.Printf("[Subtrate] %d code reviews\n", len(reviews))
		return nil
	default:
		if len(reviews) == 0 {
			fmt.Println("No code reviews found.")
			return nil
		}
		fmt.Println(formatReviewList(reviews))
	}

	return nil
}

func runReviewStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var reviewID string
	if len(args) > 0 {
		reviewID = args[0]
	} else {
		// If no review ID, show active reviews for current agent.
		agentID, _, err := getCurrentAgentWithClient(ctx, client)
		if err != nil {
			return err
		}
		reviews, err := client.ListReviewsByRequester(ctx, agentID, 1)
		if err != nil {
			return err
		}
		if len(reviews) == 0 {
			fmt.Println("No active reviews for this agent.")
			return nil
		}
		reviewID = reviews[0].ReviewID
	}

	review, err := client.GetReview(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("failed to get review: %w", err)
	}

	// Build response struct for JSON output.
	type reviewStatusOutput struct {
		Review     *ReviewInfo           `json:"review"`
		Iterations []ReviewIterationInfo `json:"iterations,omitempty"`
		Issues     []ReviewIssueInfo     `json:"issues,omitempty"`
	}

	output := reviewStatusOutput{Review: review}

	if reviewShowIterations {
		iters, err := client.ListReviewIterations(ctx, reviewID)
		if err != nil {
			return fmt.Errorf("failed to get iterations: %w", err)
		}
		output.Iterations = iters
	}

	if reviewShowIssues {
		issues, err := client.ListReviewIssues(ctx, reviewID)
		if err != nil {
			return fmt.Errorf("failed to get issues: %w", err)
		}
		output.Issues = issues
	}

	switch outputFormat {
	case "json":
		return outputJSON(output)
	case "context":
		fmt.Printf("[Review %s] %s - %s\n",
			review.ReviewID[:8], review.Branch, review.State)
		return nil
	default:
		fmt.Println(formatReviewDetail(review))
		if reviewShowIterations && len(output.Iterations) > 0 {
			fmt.Println("\n" + formatReviewIterations(output.Iterations))
		}
		if reviewShowIssues && len(output.Issues) > 0 {
			fmt.Println("\n" + formatReviewIssues(output.Issues))
		}
	}

	return nil
}

func runReviewStats(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	stats, err := client.GetReviewStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get review stats: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(stats)
	case "context":
		fmt.Printf("[Subtrate] %d total reviews, %d pending, %d approved\n",
			stats.TotalReviews, stats.Pending, stats.Approved)
		return nil
	default:
		fmt.Println(formatReviewStats(stats))
	}

	return nil
}

func runReviewRequest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentName, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Get the current branch if not specified.
	branch := reviewRequestBranch
	if branch == "" {
		out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branch = strings.TrimSpace(string(out))
	}

	// Get the current commit SHA.
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("failed to get commit SHA: %w", err)
	}
	commitSHA := strings.TrimSpace(string(out))

	// Get the repo path.
	out, err = exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("failed to get repo path: %w", err)
	}
	repoPath := strings.TrimSpace(string(out))

	// Validate review type.
	validTypes := map[string]bool{
		"full": true, "incremental": true, "security": true, "performance": true,
	}
	if !validTypes[reviewRequestType] {
		return fmt.Errorf("invalid review type: %s (must be full, incremental, security, or performance)", reviewRequestType)
	}

	// Validate priority.
	validPriorities := map[string]bool{"urgent": true, "normal": true, "low": true}
	if !validPriorities[reviewRequestPriority] {
		return fmt.Errorf("invalid priority: %s (must be urgent, normal, or low)", reviewRequestPriority)
	}

	// Build the review request message.
	reviewRequest := map[string]interface{}{
		"type":        "review_request",
		"branch":      branch,
		"base_branch": reviewRequestBaseBranch,
		"commit_sha":  commitSHA,
		"repo_path":   repoPath,
		"review_type": reviewRequestType,
		"priority":    reviewRequestPriority,
		"requester":   agentName,
	}

	requestJSON, err := json.Marshal(reviewRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal review request: %w", err)
	}

	subject := fmt.Sprintf("[Review Request] %s - %s", filepath.Base(repoPath), branch)
	body := fmt.Sprintf(`# Review Request

**Branch:** %s
**Base:** %s
**Commit:** %s
**Type:** %s
**Priority:** %s

Please review the changes in this branch.

---
%s`, branch, reviewRequestBaseBranch, commitSHA[:8], reviewRequestType, reviewRequestPriority, string(requestJSON))

	// Send the message to the reviewer.
	var messageID int64
	var threadID string

	if reviewRequestReviewer != "" {
		// Send directly to a specific reviewer agent.
		messageID, threadID, err = client.SendMail(ctx, mail.SendMailRequest{
			SenderID:       agentID,
			RecipientNames: []string{reviewRequestReviewer},
			Subject:        subject,
			Body:           body,
			Priority:       getPriority(reviewRequestPriority),
		})
	} else {
		// Publish to the reviewers topic.
		messageID, _, err = client.Publish(
			ctx, agentID, "reviewers", subject, body,
			getPriority(reviewRequestPriority),
		)
		threadID = fmt.Sprintf("review-%d", time.Now().UnixNano())
	}

	if err != nil {
		return fmt.Errorf("failed to send review request: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"message_id": messageID,
			"thread_id":  threadID,
			"branch":     branch,
			"commit":     commitSHA,
		})
	case "context":
		fmt.Printf("[Subtrate] Review requested for %s\n", branch)
		return nil
	default:
		fmt.Printf("Review requested successfully!\n")
		fmt.Printf("  Branch:    %s\n", branch)
		fmt.Printf("  Base:      %s\n", reviewRequestBaseBranch)
		fmt.Printf("  Commit:    %s\n", commitSHA[:8])
		fmt.Printf("  Type:      %s\n", reviewRequestType)
		fmt.Printf("  Priority:  %s\n", reviewRequestPriority)
		fmt.Printf("  Message:   #%d\n", messageID)
		fmt.Printf("  Thread:    %s\n", threadID)
	}

	return nil
}

func runReviewCancel(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	reviewID := args[0]

	// Check if the review exists and is cancellable.
	review, err := client.GetReview(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("failed to get review: %w", err)
	}

	terminalStates := map[string]bool{
		"approved": true, "rejected": true, "cancelled": true,
	}
	if terminalStates[review.State] {
		return fmt.Errorf("cannot cancel review in state: %s", review.State)
	}

	if err := client.CancelReview(ctx, reviewID); err != nil {
		return fmt.Errorf("failed to cancel review: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]string{
			"review_id": reviewID,
			"status":    "cancelled",
		})
	default:
		fmt.Printf("Review %s cancelled.\n", reviewID)
	}

	return nil
}

func getPriority(p string) mail.Priority {
	switch p {
	case "urgent":
		return mail.PriorityUrgent
	case "low":
		return mail.PriorityLow
	default:
		return mail.PriorityNormal
	}
}

// Formatting functions.

func formatReviewList(reviews []ReviewInfo) string {
	var sb strings.Builder
	sb.WriteString("Code Reviews\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, r := range reviews {
		stateIcon := getStateIcon(r.State)
		prInfo := ""
		if r.PRNumber != nil {
			prInfo = fmt.Sprintf(" (PR #%d)", *r.PRNumber)
		}

		sb.WriteString(fmt.Sprintf("%s %s%s\n", stateIcon, r.Branch, prInfo))
		sb.WriteString(fmt.Sprintf("   ID: %s  State: %s  Type: %s\n",
			r.ReviewID[:8], r.State, r.ReviewType))
		sb.WriteString(fmt.Sprintf("   Created: %s\n\n",
			r.CreatedAt.Format("2006-01-02 15:04")))
	}

	return sb.String()
}

func formatReviewDetail(r *ReviewInfo) string {
	var sb strings.Builder
	stateIcon := getStateIcon(r.State)

	sb.WriteString(fmt.Sprintf("Review: %s\n", r.ReviewID))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	sb.WriteString(fmt.Sprintf("Status:      %s %s\n", stateIcon, r.State))
	sb.WriteString(fmt.Sprintf("Branch:      %s -> %s\n", r.Branch, r.BaseBranch))
	sb.WriteString(fmt.Sprintf("Commit:      %s\n", r.CommitSHA[:8]))
	sb.WriteString(fmt.Sprintf("Repository:  %s\n", r.RepoPath))
	sb.WriteString(fmt.Sprintf("Type:        %s\n", r.ReviewType))
	sb.WriteString(fmt.Sprintf("Priority:    %s\n", r.Priority))

	if r.PRNumber != nil {
		sb.WriteString(fmt.Sprintf("PR Number:   #%d\n", *r.PRNumber))
	}

	sb.WriteString(fmt.Sprintf("\nCreated:     %s\n", r.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Updated:     %s\n", r.UpdatedAt.Format(time.RFC3339)))
	if r.CompletedAt != nil {
		sb.WriteString(fmt.Sprintf("Completed:   %s\n", r.CompletedAt.Format(time.RFC3339)))
	}

	return sb.String()
}

func formatReviewIterations(iters []ReviewIterationInfo) string {
	var sb strings.Builder
	sb.WriteString("Review Iterations\n")
	sb.WriteString(strings.Repeat("-", 40) + "\n\n")

	for _, iter := range iters {
		decisionIcon := getDecisionIcon(iter.Decision)
		sb.WriteString(fmt.Sprintf("Iteration %d: %s %s\n",
			iter.IterationNum, decisionIcon, iter.Decision))
		sb.WriteString(fmt.Sprintf("   Reviewer: %s\n", iter.ReviewerID))
		sb.WriteString(fmt.Sprintf("   Files: %d  Lines: %d  Duration: %dms\n",
			iter.FilesReviewed, iter.LinesAnalyzed, iter.DurationMS))
		if iter.Summary != "" {
			summary := iter.Summary
			if len(summary) > 100 {
				summary = summary[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("   Summary: %s\n", summary))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatReviewIssues(issues []ReviewIssueInfo) string {
	var sb strings.Builder
	sb.WriteString("Review Issues\n")
	sb.WriteString(strings.Repeat("-", 40) + "\n\n")

	for _, issue := range issues {
		severityIcon := getSeverityIcon(issue.Severity)
		statusIcon := getIssueStatusIcon(issue.Status)

		sb.WriteString(fmt.Sprintf("%s %s [%s] %s\n",
			severityIcon, statusIcon, issue.IssueType, issue.Title))
		sb.WriteString(fmt.Sprintf("   File: %s:%d\n",
			issue.FilePath, issue.LineStart))
		if issue.Description != "" {
			desc := issue.Description
			if len(desc) > 80 {
				desc = desc[:80] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", desc))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatReviewStats(stats *ReviewStatsInfo) string {
	var sb strings.Builder
	sb.WriteString("Review Statistics\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	sb.WriteString(fmt.Sprintf("Total Reviews:      %d\n", stats.TotalReviews))
	sb.WriteString(fmt.Sprintf("Approved:           %d\n", stats.Approved))
	sb.WriteString(fmt.Sprintf("Pending:            %d\n", stats.Pending))
	sb.WriteString(fmt.Sprintf("In Progress:        %d\n", stats.InProgress))
	sb.WriteString(fmt.Sprintf("Changes Requested:  %d\n", stats.ChangesRequested))

	return sb.String()
}

func getStateIcon(state string) string {
	switch state {
	case "approved":
		return "[OK]"
	case "rejected":
		return "[X]"
	case "under_review":
		return "[~]"
	case "changes_requested":
		return "[!]"
	case "cancelled":
		return "[-]"
	default:
		return "[ ]"
	}
}

func getDecisionIcon(decision string) string {
	switch decision {
	case "approve":
		return "[OK]"
	case "request_changes":
		return "[!]"
	default:
		return "[?]"
	}
}

func getSeverityIcon(severity string) string {
	switch severity {
	case "critical":
		return "[!!]"
	case "high":
		return "[!]"
	case "medium":
		return "[~]"
	default:
		return "[-]"
	}
}

func getIssueStatusIcon(status string) string {
	switch status {
	case "fixed":
		return "[OK]"
	case "wont_fix":
		return "[X]"
	case "duplicate":
		return "[D]"
	default:
		return "[ ]"
	}
}
