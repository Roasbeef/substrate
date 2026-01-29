package commands

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
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

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		// Generate a memorable name.
		registry := agent.NewRegistry(store)
		name, err = registry.EnsureUniqueAgentName(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate name: %w", err)
		}
	}

	// Check if agent already exists.
	_, err = store.Queries().GetAgentByName(ctx, name)
	if err == nil {
		return fmt.Errorf("agent %q already exists", name)
	}

	// Create the agent.
	now := time.Now().Unix()
	params := sqlc.CreateAgentParams{
		Name: name,
		ProjectKey: sql.NullString{
			String: registerProject,
			Valid:  registerProject != "",
		},
		CreatedAt:    now,
		LastActiveAt: now,
	}

	agentRow, err := store.Queries().CreateAgent(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Create the agent's inbox topic.
	topicName := fmt.Sprintf("agent/%s/inbox", name)
	topicParams := sqlc.CreateTopicParams{
		Name:             topicName,
		TopicType:        "direct",
		RetentionSeconds: sql.NullInt64{Int64: 604800, Valid: true},
		CreatedAt:        now,
	}

	topic, err := store.Queries().CreateTopic(ctx, topicParams)
	if err != nil {
		return fmt.Errorf("failed to create inbox topic: %w", err)
	}

	// Subscribe the agent to their inbox.
	subParams := sqlc.CreateSubscriptionParams{
		AgentID:      agentRow.ID,
		TopicID:      topic.ID,
		SubscribedAt: now,
	}
	err = store.Queries().CreateSubscription(ctx, subParams)
	if err != nil {
		return fmt.Errorf("failed to subscribe to inbox: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"id":      agentRow.ID,
			"name":    name,
			"project": registerProject,
			"topic":   topicName,
		})
	default:
		fmt.Printf("Agent registered: %s (ID: %d)\n", name, agentRow.ID)
		fmt.Printf("  Inbox topic: %s\n", topicName)
		if registerProject != "" {
			fmt.Printf("  Project: %s\n", registerProject)
		}
	}

	return nil
}

func runAgentList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agents, err := store.Queries().ListAgents(ctx)
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

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	agentID, agentName, err := getCurrentAgent(ctx, store)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"id":   agentID,
			"name": agentName,
		})
	case "context":
		fmt.Printf("[Subtrate] You are %s\n", agentName)
		return nil
	default:
		fmt.Printf("You are %s (ID: %d)\n", agentName, agentID)
	}

	return nil
}
