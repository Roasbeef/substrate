package commands

import (
	"context"
	"fmt"

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

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, _, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	// Send heartbeat to indicate agent activity.
	_ = client.UpdateHeartbeat(ctx, agentID)

	status, err := client.GetStatus(ctx, agentID)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(status)
	case "context":
		if status.UrgentCount > 0 {
			fmt.Printf("[Subtrate] %d urgent, %d unread messages\n",
				status.UrgentCount, status.UnreadCount)
		} else if status.UnreadCount > 0 {
			fmt.Printf("[Subtrate] %d unread messages\n",
				status.UnreadCount)
		}
		return nil
	default:
		fmt.Print(formatStatus(*status))
	}

	return nil
}
