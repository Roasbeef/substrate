package commands

import (
	"context"
	"fmt"
	"strings"
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

var agentDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete an agent",
	Long:  `Delete an agent by name. This also removes associated session identities.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentDelete,
}

var agentDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover all agents with status and metadata",
	Long: `Show all agents with rich discovery metadata including status,
working directory, git branch, purpose, hostname, and unread message counts.

Use --status to filter by agent status (active, busy, idle, offline).
Use --project to filter by project key prefix.
Use --name to filter by agent name substring.`,
	RunE: runAgentDiscover,
}

var (
	registerProject string
	forceDelete     bool
	discoverStatus  string
	discoverProject string
	discoverName    string
)

func init() {
	agentCmd.AddCommand(agentRegisterCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentWhoamiCmd)
	agentCmd.AddCommand(agentDeleteCmd)
	agentCmd.AddCommand(agentDiscoverCmd)

	agentRegisterCmd.Flags().StringVar(&registerProject, "project", "",
		"Project key to associate with the agent")

	agentDeleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false,
		"Skip confirmation prompt")

	agentDiscoverCmd.Flags().StringVar(
		&discoverStatus, "status", "",
		"Filter by status (comma-separated: active,busy,idle,offline)",
	)
	agentDiscoverCmd.Flags().StringVar(
		&discoverProject, "project", "",
		"Filter by project key prefix",
	)
	agentDiscoverCmd.Flags().StringVar(
		&discoverName, "name", "",
		"Filter by agent name substring",
	)
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
	gitBranch := getGitBranch()
	agentID, agentNameResult, err := client.RegisterAgent(
		ctx, name, registerProject, gitBranch,
	)
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

// runAgentDiscover executes the discover subcommand, fetching all agents
// with rich metadata and applying optional client-side filters.
func runAgentDiscover(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	agents, err := client.DiscoverAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover agents: %w", err)
	}

	// Apply client-side filters.
	agents = filterDiscoveredAgents(agents)

	switch outputFormat {
	case "json":
		return outputJSON(agents)

	case "context":
		if len(agents) == 0 {
			fmt.Println("[Subtrate Discovery] No agents found")
			return nil
		}
		parts := make([]string, len(agents))
		for i, a := range agents {
			parts[i] = fmt.Sprintf("%s(%s)", a.Name, a.Status)
		}
		fmt.Printf(
			"[Subtrate Discovery] %d agents: %s\n",
			len(agents), strings.Join(parts, ", "),
		)
		return nil

	default:
		if len(agents) == 0 {
			fmt.Println("No agents discovered.")
			return nil
		}

		fmt.Printf("Discovered %d agents:\n", len(agents))

		for _, a := range agents {
			elapsed := formatDuration(
				time.Duration(a.SecondsSince) * time.Second,
			)
			fmt.Printf(
				"\n  %s [%s] (%s ago)\n",
				a.Name, a.Status, elapsed,
			)
			if a.ProjectKey != "" {
				fmt.Printf("    Project:  %s\n", a.ProjectKey)
			}
			if a.GitBranch != "" {
				fmt.Printf("    Branch:   %s\n", a.GitBranch)
			}
			if a.WorkingDir != "" {
				fmt.Printf("    Dir:      %s\n", a.WorkingDir)
			}
			if a.Purpose != "" {
				fmt.Printf("    Purpose:  %s\n", a.Purpose)
			}
			if a.Hostname != "" {
				fmt.Printf("    Host:     %s\n", a.Hostname)
			}
			if a.SessionID != "" {
				fmt.Printf(
					"    Session:  %s\n", a.SessionID,
				)
			}
			fmt.Printf("    Unread:   %d messages\n", a.UnreadCount)
		}
	}

	return nil
}

// filterDiscoveredAgents applies the --status, --project, and --name
// flags to filter the discovered agents list.
func filterDiscoveredAgents(
	agents []DiscoveredAgentInfo,
) []DiscoveredAgentInfo {
	if discoverStatus == "" && discoverProject == "" &&
		discoverName == "" {
		return agents
	}

	// Parse status filter into a set.
	statusSet := make(map[string]bool)
	if discoverStatus != "" {
		for _, s := range strings.Split(discoverStatus, ",") {
			statusSet[strings.TrimSpace(strings.ToLower(s))] = true
		}
	}

	filtered := make([]DiscoveredAgentInfo, 0, len(agents))
	for _, a := range agents {
		if len(statusSet) > 0 && !statusSet[strings.ToLower(a.Status)] {
			continue
		}
		if discoverProject != "" &&
			!strings.HasPrefix(a.ProjectKey, discoverProject) {
			continue
		}
		if discoverName != "" &&
			!strings.Contains(
				strings.ToLower(a.Name),
				strings.ToLower(discoverName),
			) {
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered
}

func runAgentDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	agentNameToDelete := args[0]

	// Prevent deleting the User agent.
	if agentNameToDelete == "User" {
		return fmt.Errorf("cannot delete the User agent")
	}

	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Verify agent exists.
	agentRow, err := client.GetAgentByName(ctx, agentNameToDelete)
	if err != nil {
		return fmt.Errorf("agent not found: %s", agentNameToDelete)
	}

	// Delete the agent.
	if err := client.DeleteAgent(ctx, agentRow.ID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"deleted": agentNameToDelete,
			"id":      agentRow.ID,
		})
	default:
		fmt.Printf("Deleted agent: %s (ID: %d)\n", agentNameToDelete, agentRow.ID)
	}

	return nil
}
