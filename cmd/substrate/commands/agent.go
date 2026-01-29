package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// agentCmd is the parent command for agent management.
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
	Long:  `Commands for managing agent registration and configuration.`,
}

var agentRegisterCmd = &cobra.Command{
	Use:   "register [name]",
	Short: "Register a new agent",
	Long:  `Register a new agent with the given name, or generate a memorable name.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAgentRegister,
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Long:  `List all registered agents.`,
	RunE:  runAgentList,
}

var agentWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current agent",
	Long:  `Display the current agent identity.`,
	RunE:  runAgentWhoami,
}

var registerProject string

func init() {
	agentCmd.AddCommand(agentRegisterCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentWhoamiCmd)

	agentRegisterCmd.Flags().StringVar(&registerProject, "project", "",
		"Project key to associate with the agent")
}

func runAgentRegister(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// RegisterAgent handles name generation if empty.
	agentID, agentNameResult, err := client.RegisterAgent(ctx, name, registerProject)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	topicName := fmt.Sprintf("agent/%s/inbox", agentNameResult)

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"id":      agentID,
			"name":    agentNameResult,
			"project": registerProject,
			"topic":   topicName,
		})
	default:
		fmt.Printf("Agent registered: %s (ID: %d)\n", agentNameResult, agentID)
		fmt.Printf("  Inbox topic: %s\n", topicName)
		if registerProject != "" {
			fmt.Printf("  Project: %s\n", registerProject)
		}
	}

	return nil
}

func runAgentList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agents, err := client.ListAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(agents)
	default:
		if len(agents) == 0 {
			fmt.Println("No agents registered.")
			return nil
		}

		fmt.Printf("Registered agents (%d):\n\n", len(agents))
		for _, a := range agents {
			fmt.Printf("  %s (ID: %d)\n", a.Name, a.ID)
			if a.ProjectKey.Valid {
				fmt.Printf("    Project: %s\n", a.ProjectKey.String)
			}
			if a.CurrentSessionID.Valid {
				fmt.Printf("    Session: %s\n", a.CurrentSessionID.String)
			}
			lastActive := time.Unix(a.LastActiveAt, 0).Format(time.RFC3339)
			fmt.Printf("    Last active: %s\n", lastActive)
			fmt.Println()
		}
	}

	return nil
}

func runAgentWhoami(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agentID, agentNameStr, err := getCurrentAgentWithClient(ctx, client)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"id":   agentID,
			"name": agentNameStr,
		})
	case "context":
		fmt.Printf("[Subtrate] You are %s\n", agentNameStr)
		return nil
	default:
		fmt.Printf("You are %s (ID: %d)\n", agentNameStr, agentID)
	}

	return nil
}
