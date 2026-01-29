package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	searchIn    string
	searchLimit int
)

// searchCmd searches for messages.
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search messages",
	Long:  `Search messages using full-text search.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchIn, "in", "",
		"Limit search to a specific topic")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 20,
		"Maximum number of results")
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	query := args[0]

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, _, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	// Use the search implementation on store.
	results, err := store.SearchMessagesForAgent(ctx, query, agentID, searchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(results)
	default:
		if len(results) == 0 {
			fmt.Printf("No results for %q.\n", query)
			return nil
		}

		fmt.Printf("Search results for %q (%d):\n\n", query, len(results))
		for _, r := range results {
			fmt.Printf("  #%d: %s\n", r.ID, r.Subject)
			fmt.Printf("    Thread: %s | Priority: %s\n",
				r.ThreadID, r.Priority)
			fmt.Println()
		}
	}

	return nil
}
