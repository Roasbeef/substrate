package commands

import (
	"github.com/spf13/cobra"
)

var (
	// dbPath is the path to the SQLite database.
	dbPath string

	// agentName is the name of the current agent.
	agentName string

	// sessionID is the Claude Code session ID.
	sessionID string

	// projectDir is the project directory.
	projectDir string

	// outputFormat controls output format (text, json, context).
	outputFormat string
)

// rootCmd is the base command for the CLI.
var rootCmd = &cobra.Command{
	Use:   "subtrate-cli",
	Short: "Subtrate agent command center CLI",
	Long: `Subtrate CLI provides mail/messaging capabilities for Claude Code agents.

Use this CLI to send and receive messages, subscribe to topics, and manage
agent identity across Claude Code sessions.`,
}

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags.
	rootCmd.PersistentFlags().StringVar(
		&dbPath, "db", "",
		"Path to SQLite database (default: ~/.subtrate/subtrate.db)",
	)
	rootCmd.PersistentFlags().StringVar(
		&agentName, "agent", "",
		"Agent name to use for operations",
	)
	rootCmd.PersistentFlags().StringVar(
		&sessionID, "session-id", "",
		"Claude Code session ID (from $CLAUDE_SESSION_ID)",
	)
	rootCmd.PersistentFlags().StringVar(
		&projectDir, "project", "",
		"Project directory (from $CLAUDE_PROJECT_DIR)",
	)
	rootCmd.PersistentFlags().StringVar(
		&outputFormat, "format", "text",
		"Output format: text, json, context",
	)

	// Add subcommands.
	rootCmd.AddCommand(inboxCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(ackCmd)
	rootCmd.AddCommand(starCmd)
	rootCmd.AddCommand(snoozeCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(trashCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(pollCmd)
	rootCmd.AddCommand(identityCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(topicsCmd)
	rootCmd.AddCommand(subscribeCmd)
	rootCmd.AddCommand(publishCmd)
}
