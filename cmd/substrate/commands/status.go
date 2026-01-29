package commands

import (
	"context"
	"fmt"

	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show mail status",
	Long:  `Display mail status summary for the current agent.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, _, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	svc := mail.NewService(store)

	req := mail.GetStatusRequest{
		AgentID: agentID,
	}

	result := svc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return err
	}

	resp := val.(mail.GetStatusResponse)
	if resp.Error != nil {
		return resp.Error
	}

	switch outputFormat {
	case "json":
		return outputJSON(resp.Status)
	case "context":
		if resp.Status.UrgentCount > 0 {
			fmt.Printf("[Subtrate] %d urgent, %d unread messages\n",
				resp.Status.UrgentCount, resp.Status.UnreadCount)
		} else if resp.Status.UnreadCount > 0 {
			fmt.Printf("[Subtrate] %d unread messages\n",
				resp.Status.UnreadCount)
		}
		return nil
	default:
		fmt.Print(formatStatus(resp.Status))
	}

	return nil
}
