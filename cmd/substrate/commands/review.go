package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/spf13/cobra"
)

// reviewCmd is the parent command for code review operations.
var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Manage code reviews",
	Long:  "Request, track, and manage code reviews via the review service.",
}

// reviewRequestCmd requests a new code review.
var reviewRequestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request a new code review",
	Long: `Request a new code review for the current branch. Gathers git
context (branch, commit SHA, remote URL) automatically from the
current working directory unless overridden by flags.`,
	RunE: runReviewRequest,
}

// reviewStatusCmd shows the status of a specific review.
var reviewStatusCmd = &cobra.Command{
	Use:   "status <review-id>",
	Short: "Show review status and details",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewStatus,
}

// reviewListCmd lists reviews with optional filters.
var reviewListCmd = &cobra.Command{
	Use:   "list",
	Short: "List reviews with optional filters",
	RunE:  runReviewList,
}

// reviewCancelCmd cancels an active review.
var reviewCancelCmd = &cobra.Command{
	Use:   "cancel <review-id>",
	Short: "Cancel an active review",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewCancel,
}

// reviewIssuesCmd lists issues for a review.
var reviewIssuesCmd = &cobra.Command{
	Use:   "issues <review-id>",
	Short: "List issues found in a review",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewIssues,
}

// reviewResubmitCmd resubmits a review after the author has pushed fixes.
var reviewResubmitCmd = &cobra.Command{
	Use:   "resubmit <review-id>",
	Short: "Resubmit a review after pushing fixes",
	Long: `Resubmit a review after the author has pushed fixes. This triggers
the reviewer to re-review the changes. If the original reviewer is still
active (stop hook polling), the feedback is delivered as mail. Otherwise
a fresh reviewer is spawned.`,
	Args: cobra.ExactArgs(1),
	RunE: runReviewResubmit,
}

// reviewDeleteCmd deletes a review and all associated data.
var reviewDeleteCmd = &cobra.Command{
	Use:   "delete <review-id>",
	Short: "Delete a review and all associated data",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewDelete,
}

// Review command flags.
var (
	reviewBranch     string
	reviewBaseBranch string
	reviewCommitSHA  string
	reviewRepoPath   string
	reviewRemoteURL  string
	reviewType       string
	reviewPriority   string
	reviewPRNumber   int
	reviewDesc       string

	// List filters.
	reviewFilterState string
	reviewListLimit   int

	// Cancel reason.
	reviewCancelReason string
)

func init() {
	// Request flags.
	reviewRequestCmd.Flags().StringVar(
		&reviewBranch, "branch", "",
		"Branch to review (auto-detected if empty)",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewBaseBranch, "base", "main",
		"Base branch to diff against",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewCommitSHA, "commit", "",
		"Commit SHA (auto-detected if empty)",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewRepoPath, "repo", "",
		"Repository path (auto-detected if empty)",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewRemoteURL, "remote-url", "",
		"Remote URL (auto-detected if empty)",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewType, "type", "full",
		"Review type: full, security, performance, architecture",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewPriority, "priority", "normal",
		"Priority: urgent, normal, low",
	)
	reviewRequestCmd.Flags().IntVar(
		&reviewPRNumber, "pr", 0,
		"Pull request number (optional)",
	)
	reviewRequestCmd.Flags().StringVar(
		&reviewDesc, "description", "",
		"Description of what to review",
	)

	// List filters.
	reviewListCmd.Flags().StringVar(
		&reviewFilterState, "state", "",
		"Filter by state (pending_review, under_review, etc.)",
	)
	reviewListCmd.Flags().IntVar(
		&reviewListLimit, "limit", 20,
		"Maximum number of reviews to show",
	)

	// Cancel flags.
	reviewCancelCmd.Flags().StringVar(
		&reviewCancelReason, "reason", "",
		"Reason for cancellation",
	)

	// Register subcommands.
	reviewCmd.AddCommand(reviewRequestCmd)
	reviewCmd.AddCommand(reviewStatusCmd)
	reviewCmd.AddCommand(reviewListCmd)
	reviewCmd.AddCommand(reviewCancelCmd)
	reviewCmd.AddCommand(reviewIssuesCmd)
	reviewCmd.AddCommand(reviewResubmitCmd)
	reviewCmd.AddCommand(reviewDeleteCmd)
}

// runReviewRequest handles the `substrate review request` command.
func runReviewRequest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Resolve agent identity.
	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return fmt.Errorf("resolve identity: %w", err)
	}

	// Determine which flags were explicitly provided.
	commitExplicit := cmd.Flags().Changed("commit")
	prExplicit := cmd.Flags().Changed("pr")

	// Always auto-detect branch for naming/display purposes.
	if reviewBranch == "" {
		reviewBranch = getGitBranch()
	}
	if reviewCommitSHA == "" {
		reviewCommitSHA = gitCommitSHA()
	}
	if reviewRepoPath == "" {
		reviewRepoPath = gitRepoRoot()
	}
	if reviewRemoteURL == "" {
		reviewRemoteURL = gitRemoteURL()
	}

	// Validate required fields.
	if reviewBranch == "" {
		return fmt.Errorf(
			"could not detect branch; use --branch flag",
		)
	}
	if reviewCommitSHA == "" {
		return fmt.Errorf(
			"could not detect commit SHA; use --commit flag",
		)
	}

	// Build the request with the appropriate target type.
	req := &subtraterpc.CreateReviewRequest{
		RequesterId: agentID,
		RepoPath:    reviewRepoPath,
		RemoteUrl:   reviewRemoteURL,
		ReviewType:  reviewType,
		Priority:    reviewPriority,
		Description: reviewDesc,
	}

	// Set the target based on what was explicitly requested.
	switch {
	case prExplicit && reviewPRNumber > 0:
		// PR review mode.
		req.Target = &subtraterpc.CreateReviewRequest_PrTarget{
			PrTarget: &subtraterpc.PRTarget{
				Number:     int32(reviewPRNumber),
				Branch:     reviewBranch,
				BaseBranch: reviewBaseBranch,
			},
		}
	case commitExplicit:
		// Single commit review mode.
		req.Target = &subtraterpc.CreateReviewRequest_CommitTarget{
			CommitTarget: &subtraterpc.CommitTarget{
				Sha:    reviewCommitSHA,
				Branch: reviewBranch,
			},
		}
	default:
		// Full branch diff review mode.
		req.Target = &subtraterpc.CreateReviewRequest_BranchTarget{
			BranchTarget: &subtraterpc.BranchTarget{
				Branch:     reviewBranch,
				BaseBranch: reviewBaseBranch,
			},
		}
	}

	resp, err := client.CreateReview(ctx, req)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("review error: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Review created:\n")
		fmt.Printf("  ID:       %s\n", resp.ReviewId)
		fmt.Printf("  Thread:   %s\n", resp.ThreadId)
		fmt.Printf("  State:    %s\n", resp.State)
		fmt.Printf("  Branch:   %s\n", reviewBranch)
		fmt.Printf("  Type:     %s\n", reviewType)
		fmt.Printf("  Priority: %s\n", reviewPriority)
	}

	return nil
}

// runReviewStatus handles the `substrate review status <id>` command.
func runReviewStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	reviewID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.GetReview(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("get review: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("review error: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Review %s\n", resp.ReviewId)
		fmt.Printf("  State:       %s\n", resp.State)
		fmt.Printf("  Branch:      %s\n", resp.Branch)
		fmt.Printf("  Base:        %s\n", resp.BaseBranch)
		fmt.Printf("  Type:        %s\n", resp.ReviewType)
		fmt.Printf("  Iterations:  %d\n", resp.Iterations)
		fmt.Printf("  Open Issues: %d\n", resp.OpenIssues)
	}

	return nil
}

// runReviewList handles the `substrate review list` command.
func runReviewList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	req := &subtraterpc.ListReviewsProtoRequest{
		State: reviewFilterState,
		Limit: int32(reviewListLimit),
	}

	resp, err := client.ListReviews(ctx, req)
	if err != nil {
		return fmt.Errorf("list reviews: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		if len(resp.Reviews) == 0 {
			fmt.Println("No reviews found.")
			return nil
		}

		fmt.Printf(
			"%-12s %-20s %-18s %-10s %s\n",
			"REVIEW ID", "BRANCH", "STATE", "TYPE",
			"CREATED",
		)
		fmt.Println(strings.Repeat("-", 78))

		for _, r := range resp.Reviews {
			created := time.Unix(r.CreatedAt, 0).
				Format("2006-01-02 15:04")

			// Truncate review ID for display.
			id := r.ReviewId
			if len(id) > 12 {
				id = id[:12]
			}

			// Truncate branch for display.
			branch := r.Branch
			if len(branch) > 20 {
				branch = branch[:17] + "..."
			}

			fmt.Printf(
				"%-12s %-20s %-18s %-10s %s\n",
				id, branch, r.State, r.ReviewType,
				created,
			)
		}
	}

	return nil
}

// runReviewCancel handles the `substrate review cancel <id>` command.
func runReviewCancel(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	reviewID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.CancelReview(ctx, reviewID, reviewCancelReason)
	if err != nil {
		return fmt.Errorf("cancel review: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("cancel error: %s", resp.Error)
	}

	fmt.Printf("Review %s cancelled.\n", reviewID)
	return nil
}

// runReviewResubmit handles the `substrate review resubmit <id>` command.
func runReviewResubmit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	reviewID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Auto-detect current commit SHA.
	commitSHA := gitCommitSHA()
	if commitSHA == "" {
		return fmt.Errorf(
			"could not detect commit SHA; " +
				"run from within a git repository",
		)
	}

	resp, err := client.ResubmitReview(ctx, reviewID, commitSHA)
	if err != nil {
		return fmt.Errorf("resubmit review: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("resubmit error: %s", resp.Error)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		fmt.Printf("Review %s resubmitted.\n", reviewID)
		fmt.Printf("  New State: %s\n", resp.State)
	}

	return nil
}

// runReviewIssues handles the `substrate review issues <id>` command.
func runReviewIssues(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	reviewID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.ListReviewIssues(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	default:
		if len(resp.Issues) == 0 {
			fmt.Println("No issues found for this review.")
			return nil
		}

		for _, issue := range resp.Issues {
			severityIcon := severityToIcon(issue.Severity)
			fmt.Printf(
				"%s [%s] %s (%s:%d)\n",
				severityIcon, issue.IssueType,
				issue.Title, issue.FilePath,
				issue.LineStart,
			)
			fmt.Printf(
				"    Status: %s | Severity: %s\n",
				issue.Status, issue.Severity,
			)
		}
	}

	return nil
}

// runReviewDelete handles the `substrate review delete <id>` command.
func runReviewDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	reviewID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	resp, err := client.DeleteReview(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("delete error: %s", resp.Error)
	}

	fmt.Printf("Review %s deleted.\n", reviewID)
	return nil
}

// severityToIcon returns a text indicator for issue severity.
func severityToIcon(severity string) string {
	switch severity {
	case "critical":
		return "[!!]"
	case "high":
		return "[! ]"
	case "medium":
		return "[- ]"
	case "low":
		return "[  ]"
	default:
		return "[  ]"
	}
}

// Git helper functions for auto-detecting review context.

// gitCommitSHA returns the current HEAD commit SHA.
func gitCommitSHA() string {
	out, err := exec.Command(
		"git", "rev-parse", "HEAD",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRepoRoot returns the root directory of the git repository.
func gitRepoRoot() string {
	out, err := exec.Command(
		"git", "rev-parse", "--show-toplevel",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRemoteURL returns the origin remote URL.
func gitRemoteURL() string {
	out, err := exec.Command(
		"git", "remote", "get-url", "origin",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
