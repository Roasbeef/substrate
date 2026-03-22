package commands

import (
	"os"

	"github.com/roasbeef/subtrate/internal/build"
	"github.com/spf13/cobra"
)

var (
	// dbPath is the path to the SQLite database.
	dbPath string

	// grpcAddr is the address of the substrated daemon.
	grpcAddr string

	// agentName is the name of the current agent.
	agentName string

	// sessionID is the Claude Code session ID.
	sessionID string

	// projectDir is the project directory.
	projectDir string

	// outputFormat controls output format (text, json, context).
	outputFormat string

	// verbose enables verbose output.
	verbose bool

	// noQueue disables the offline queue fallback.
	noQueue bool

	// queueOnly forces all write operations to go through the local queue.
	queueOnly bool

	// autoYes skips confirmation prompts for destructive operations.
	autoYes bool

	// compact enables compact (single-line) JSON output.
	compact bool

	// fieldsFilter selects specific fields in JSON output.
	fieldsFilter string

	// pageToken is a pagination token for list commands.
	pageToken string
)

// rootCmd is the base command for the CLI.
var rootCmd = &cobra.Command{
	Use:     "subtrate-cli",
	Short:   "Subtrate agent command center CLI",
	Version: build.Version(),
	Long: `Subtrate CLI provides mail/messaging capabilities for Claude Code agents.

Use this CLI to send and receive messages, subscribe to topics, and manage
agent identity across Claude Code sessions.`,
}

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Auto-detect non-TTY stdout and default to JSON output. Using
	// cobra.OnInitialize ensures this runs for all subcommands without
	// breaking PersistentPreRun hook chaining.
	cobra.OnInitialize(func() {
		if !rootCmd.Flags().Changed("format") &&
			!isTerminal(os.Stdout) {

			outputFormat = "json"
		}
	})

	// Global flags.
	rootCmd.PersistentFlags().StringVar(
		&dbPath, "db", "",
		"Path to SQLite database (default: ~/.subtrate/subtrate.db)",
	)
	rootCmd.PersistentFlags().StringVar(
		&grpcAddr, "grpc-addr", "",
		"Address of substrated daemon (default: localhost:10009)",
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
	rootCmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false,
		"Enable verbose output",
	)
	rootCmd.PersistentFlags().BoolVar(
		&noQueue, "no-queue", false,
		"Disable offline queue fallback",
	)
	rootCmd.PersistentFlags().BoolVar(
		&queueOnly, "queue-only", false,
		"Force all write operations through the local queue",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&autoYes, "yes", "y", false,
		"Skip confirmation prompts for destructive operations",
	)
	rootCmd.PersistentFlags().BoolVar(
		&compact, "compact", false,
		"Compact JSON output (single-line, JSONL for arrays)",
	)
	rootCmd.PersistentFlags().StringVar(
		&fieldsFilter, "fields", "",
		"Comma-separated list of fields to include in JSON output",
	)
	rootCmd.PersistentFlags().StringVar(
		&pageToken, "page-token", "",
		"Pagination token for list commands",
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
	rootCmd.AddCommand(statusUpdateCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(sendDiffCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(tasksCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(schemaCmd)
}
