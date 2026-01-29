package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/spf13/cobra"
)

// identityCmd is the parent command for identity management.
var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Manage agent identity",
	Long:  `Commands for managing agent identity across Claude Code sessions.`,
}

var identityEnsureCmd = &cobra.Command{
	Use:   "ensure",
	Short: "Ensure agent identity exists",
	Long:  `Create or restore agent identity for the current session.`,
	RunE:  runIdentityEnsure,
}

var identityRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore agent identity",
	Long:  `Restore agent identity from a previous session.`,
	RunE:  runIdentityRestore,
}

var identitySaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save agent identity",
	Long:  `Save current agent identity and consumer offsets.`,
	RunE:  runIdentitySave,
}

var identityCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current identity",
	Long:  `Display the current agent identity.`,
	RunE:  runIdentityCurrent,
}

var identityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known identities",
	Long:  `List all known agent identities.`,
	RunE:  runIdentityList,
}

var identitySetDefaultCmd = &cobra.Command{
	Use:   "set-default",
	Short: "Set default agent for project",
	Long:  `Set the default agent to use for a project.`,
	RunE:  runIdentitySetDefault,
}

var setDefaultAgentName string

func init() {
	identityCmd.AddCommand(identityEnsureCmd)
	identityCmd.AddCommand(identityRestoreCmd)
	identityCmd.AddCommand(identitySaveCmd)
	identityCmd.AddCommand(identityCurrentCmd)
	identityCmd.AddCommand(identityListCmd)
	identityCmd.AddCommand(identitySetDefaultCmd)

	identitySetDefaultCmd.Flags().StringVar(&setDefaultAgentName, "agent", "",
		"Agent name to set as default (required)")
	identitySetDefaultCmd.MarkFlagRequired("agent")
}

func runIdentityEnsure(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Get session ID from flag or environment.
	sid := sessionID
	if sid == "" {
		sid = os.Getenv("CLAUDE_SESSION_ID")
	}
	if sid == "" {
		return fmt.Errorf("session ID required (use --session-id or set CLAUDE_SESSION_ID)")
	}

	// Get project dir from flag or environment.
	proj := projectDir
	if proj == "" {
		proj = os.Getenv("CLAUDE_PROJECT_DIR")
	}

	registry := agent.NewRegistry(store)
	mgr, err := agent.NewIdentityManager(store, registry)
	if err != nil {
		return fmt.Errorf("failed to create identity manager: %w", err)
	}

	identity, err := mgr.EnsureIdentity(ctx, sid, proj)
	if err != nil {
		return fmt.Errorf("failed to ensure identity: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(identity)
	case "context":
		fmt.Printf("[Subtrate] Agent: %s\n", identity.AgentName)
		return nil
	default:
		fmt.Printf("Agent identity ensured.\n")
		fmt.Printf("  Name: %s\n", identity.AgentName)
		fmt.Printf("  Session: %s\n", identity.SessionID)
		if identity.ProjectKey != "" {
			fmt.Printf("  Project: %s\n", identity.ProjectKey)
		}
	}

	return nil
}

func runIdentityRestore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	sid := sessionID
	if sid == "" {
		sid = os.Getenv("CLAUDE_SESSION_ID")
	}
	if sid == "" {
		return fmt.Errorf("session ID required (use --session-id or set CLAUDE_SESSION_ID)")
	}

	registry := agent.NewRegistry(store)
	mgr, err := agent.NewIdentityManager(store, registry)
	if err != nil {
		return fmt.Errorf("failed to create identity manager: %w", err)
	}

	identity, err := mgr.RestoreIdentity(ctx, sid)
	if err != nil {
		return fmt.Errorf("failed to restore identity: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(identity)
	case "context":
		fmt.Printf("[Subtrate] Restored agent: %s\n", identity.AgentName)
		return nil
	default:
		fmt.Printf("Identity restored.\n")
		fmt.Printf("  Name: %s\n", identity.AgentName)
		fmt.Printf("  Session: %s\n", identity.SessionID)
	}

	return nil
}

func runIdentitySave(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	sid := sessionID
	if sid == "" {
		sid = os.Getenv("CLAUDE_SESSION_ID")
	}
	if sid == "" {
		return fmt.Errorf("session ID required (use --session-id or set CLAUDE_SESSION_ID)")
	}

	registry := agent.NewRegistry(store)
	mgr, err := agent.NewIdentityManager(store, registry)
	if err != nil {
		return fmt.Errorf("failed to create identity manager: %w", err)
	}

	// First restore the identity to get current state.
	identity, err := mgr.RestoreIdentity(ctx, sid)
	if err != nil {
		return fmt.Errorf("failed to restore identity for saving: %w", err)
	}

	// Get current offsets from database.
	offsets, err := store.Queries().ListConsumerOffsetsByAgent(ctx, identity.AgentID)
	if err != nil {
		return fmt.Errorf("failed to get offsets: %w", err)
	}

	identity.ConsumerOffsets = make(map[string]int64)
	for _, offset := range offsets {
		identity.ConsumerOffsets[offset.TopicName] = offset.LastOffset
	}

	// Save identity.
	if err := mgr.SaveIdentity(ctx, identity); err != nil {
		return fmt.Errorf("failed to save identity: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]string{"status": "saved"})
	default:
		fmt.Printf("Identity saved for %s.\n", identity.AgentName)
	}

	return nil
}

func runIdentityCurrent(cmd *cobra.Command, args []string) error {
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

	agentRow, err := store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent details: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(map[string]interface{}{
			"id":         agentID,
			"name":       agentName,
			"project":    nullStringToInterface(agentRow.ProjectKey),
			"session":    nullStringToInterface(agentRow.CurrentSessionID),
			"created_at": agentRow.CreatedAt,
		})
	default:
		fmt.Printf("Current agent: %s (ID: %d)\n", agentName, agentID)
		if agentRow.ProjectKey.Valid {
			fmt.Printf("  Project: %s\n", agentRow.ProjectKey.String)
		}
		if agentRow.CurrentSessionID.Valid {
			fmt.Printf("  Session: %s\n", agentRow.CurrentSessionID.String)
		}
	}

	return nil
}

func runIdentityList(cmd *cobra.Command, args []string) error {
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

		fmt.Println("Known agents:")
		for _, a := range agents {
			proj := ""
			if a.ProjectKey.Valid {
				proj = fmt.Sprintf(" (project: %s)", a.ProjectKey.String)
			}
			fmt.Printf("  - %s%s\n", a.Name, proj)
		}
	}

	return nil
}

func runIdentitySetDefault(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	store, err := getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	proj := projectDir
	if proj == "" {
		proj = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	if proj == "" {
		return fmt.Errorf("project required (use --project or set CLAUDE_PROJECT_DIR)")
	}

	registry := agent.NewRegistry(store)
	mgr, err := agent.NewIdentityManager(store, registry)
	if err != nil {
		return fmt.Errorf("failed to create identity manager: %w", err)
	}

	if err := mgr.SetProjectDefault(ctx, proj, setDefaultAgentName); err != nil {
		return fmt.Errorf("failed to set default: %w", err)
	}

	fmt.Printf("Default agent for %s set to %s.\n", proj, setDefaultAgentName)
	return nil
}

// nullStringToInterface converts a sql.NullString to either string or nil.
func nullStringToInterface(s sql.NullString) interface{} {
	if s.Valid {
		return s.String
	}
	return nil
}
