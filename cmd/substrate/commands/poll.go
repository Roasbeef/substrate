package commands

import (
	"context"
	"fmt"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var (
	pollQuiet bool
)

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll for new messages",
	Long:  `Check for new messages since last poll.`,
	RunE:  runPoll,
}

func init() {
	pollCmd.Flags().BoolVar(&pollQuiet, "quiet", false,
		"Only output if there are new messages")
}

func runPoll(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentName, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	// Get current offsets from database.
	offsets, err := store.Queries().ListConsumerOffsetsByAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to get offsets: %w", err)
	}

	sinceOffsets := make(map[int64]int64)
	for _, offset := range offsets {
		sinceOffsets[offset.TopicID] = offset.LastOffset
	}

	svc := mail.NewService(store)

	req := mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.PollChangesResponse)
	if resp.Error != nil {
		return resp.Error
	}

	if len(resp.NewMessages) == 0 {
		if !pollQuiet {
			fmt.Printf("No new messages for %s.\n", agentName)
		}
		return nil
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp)
	case "context":
		fmt.Print(formatContext(resp.NewMessages))
		return nil
	default:
		fmt.Printf("New messages for %s:\n\n", agentName)
		for _, msg := range resp.NewMessages {
			fmt.Print(formatMessage(msg))
			fmt.Println()
		}
	}

	return nil
}
