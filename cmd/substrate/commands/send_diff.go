package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	sendDiffTo       string
	sendDiffBase     string
	sendDiffRepoPath string
	sendDiffSubject  string
)

// sendDiffCmd sends a git diff summary as a message.
var sendDiffCmd = &cobra.Command{
	Use:   "send-diff",
	Short: "Send git diff as a message",
	Long: `Gather git diffs (uncommitted + committed changes) and send them as
a message to another agent. The diff is rendered with syntax highlighting
in the web UI.

By default, diffs are computed against the main/master branch.`,
	RunE: runSendDiff,
}

func init() {
	sendDiffCmd.Flags().StringVar(
		&sendDiffTo, "to", "User",
		"Recipient agent name (default: User)",
	)
	sendDiffCmd.Flags().StringVar(
		&sendDiffBase, "base", "",
		"Base branch to diff against (auto-detects main/master)",
	)
	sendDiffCmd.Flags().StringVar(
		&sendDiffRepoPath, "repo", "",
		"Repository path (default: current directory)",
	)
	sendDiffCmd.Flags().StringVar(
		&sendDiffSubject, "subject", "",
		"Custom subject (default: auto-generated from branch)",
	)
}

// runSendDiff gathers git diffs and sends them as a message.
func runSendDiff(cmd *cobra.Command, args []string) error {
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

	// Resolve repository path.
	repoPath := sendDiffRepoPath
	if repoPath == "" {
		repoPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Get current branch.
	branch := runGitQuiet(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		branch = "unknown"
	}

	// Detect base branch.
	base := sendDiffBase
	if base == "" {
		base = detectBaseBranch(repoPath)
	}

	// Gather uncommitted changes (staged + unstaged).
	uncommitted := runGitQuiet(repoPath, "diff", "HEAD")

	// Gather committed changes since base.
	var committed string
	if base != "" && base != branch {
		committed = runGitQuiet(
			repoPath, "diff", base+"..."+branch,
		)
	}

	// Pick the most relevant diff.
	var patch, diffCmd string
	if committed != "" {
		patch = committed
		diffCmd = fmt.Sprintf("git diff %s...%s", base, branch)
	} else if uncommitted != "" {
		patch = uncommitted
		diffCmd = "git diff HEAD"
	}

	if patch == "" {
		fmt.Println("No changes to send.")
		return nil
	}

	// Compute diff stats.
	stats := computeDiffStats(patch)

	// Generate subject.
	subject := sendDiffSubject
	if subject == "" {
		projectName := filepath.Base(repoPath)
		subject = fmt.Sprintf(
			"[Diff] %s/%s — %s",
			projectName, branch, stats.summary(),
		)
	}

	// Build message body with diff marker for frontend rendering.
	var body strings.Builder
	body.WriteString(fmt.Sprintf(
		"**Branch:** `%s` (base: `%s`)\n", branch, base,
	))
	body.WriteString(fmt.Sprintf("**Command:** `%s`\n", diffCmd))
	body.WriteString(fmt.Sprintf(
		"**Stats:** %s\n\n", stats.summary(),
	))

	// Diff marker — the frontend detects this and renders with DiffViewer.
	body.WriteString("<!-- substrate:diff -->\n")
	body.WriteString(patch)

	req := mail.SendMailRequest{
		SenderID:       agentID,
		RecipientNames: []string{sendDiffTo},
		Subject:        subject,
		Body:           body.String(),
		Priority:       mail.PriorityNormal,
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
			"stats":      stats,
		})
	default:
		fmt.Printf(
			"Diff sent! ID: %d, Thread: %s (%s)\n",
			msgID, threadID, stats.summary(),
		)
	}

	return nil
}

// diffStats holds summary statistics for a diff.
type diffStats struct {
	Files     int `json:"files"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// summary returns a human-readable summary of the diff stats.
func (s diffStats) summary() string {
	return fmt.Sprintf(
		"%d files, +%d/-%d lines",
		s.Files, s.Additions, s.Deletions,
	)
}

// computeDiffStats computes basic stats from a unified diff patch.
func computeDiffStats(patch string) diffStats {
	var s diffStats
	files := make(map[string]bool)

	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			// Extract file name from "diff --git a/foo b/foo".
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				files[strings.TrimPrefix(
					parts[3], "b/",
				)] = true
			}
		} else if strings.HasPrefix(line, "+") &&
			!strings.HasPrefix(line, "+++") {

			s.Additions++
		} else if strings.HasPrefix(line, "-") &&
			!strings.HasPrefix(line, "---") {

			s.Deletions++
		}
	}

	s.Files = len(files)

	return s
}

// runGitQuiet runs a git command and returns trimmed stdout.
func runGitQuiet(repoPath string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String())
}

// detectBaseBranch detects the default base branch (main or master).
func detectBaseBranch(repoPath string) string {
	// Check for common default branches.
	for _, candidate := range []string{"main", "master"} {
		out := runGitQuiet(
			repoPath, "rev-parse", "--verify", candidate,
		)
		if out != "" {
			return candidate
		}
	}

	return "main"
}
