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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Get topic ID if searching within a specific topic.
	var topicID int64
	// Note: Topic lookup would need to be added to the client if needed.
	// For now, we pass 0 to search all topics.

	results, err := client.Search(ctx, agentID, query, topicID, searchLimit)
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
