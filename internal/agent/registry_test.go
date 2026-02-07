package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/stretchr/testify/require"
)

// testDB creates a temporary test database with migrations applied.
func testDB(t *testing.T) (*db.Store, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "subtrate-agent-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := db.Open(dbPath)
	require.NoError(t, err)

	migrationsDir := findMigrationsDir(t)
	err = db.RunMigrations(store.DB(), migrationsDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()

	paths := []string{
		"../db/migrations",
		"../../internal/db/migrations",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		p := filepath.Join(gopath, "src/github.com/roasbeef/subtrate/internal/db/migrations")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatal("Could not find migrations directory")
	return ""
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	require.NotNil(t, registry)
}

func TestRegistry_RegisterAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	agent, err := registry.RegisterAgent(ctx, "TestAgent", "", "")
	require.NoError(t, err)
	require.NotNil(t, agent)
	require.Equal(t, "TestAgent", agent.Name)
	require.Greater(t, agent.ID, int64(0))
}

func TestRegistry_RegisterAgent_WithProject(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent with project.
	agent, err := registry.RegisterAgent(ctx, "ProjectAgent", "/path/to/project", "")
	require.NoError(t, err)
	require.NotNil(t, agent)
	require.Equal(t, "ProjectAgent", agent.Name)
	require.True(t, agent.ProjectKey.Valid)
	require.Equal(t, "/path/to/project", agent.ProjectKey.String)
}

func TestRegistry_RegisterAgent_DuplicateName(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register first agent.
	_, err := registry.RegisterAgent(ctx, "DuplicateAgent", "", "")
	require.NoError(t, err)

	// Try to register with same name - should fail.
	_, err = registry.RegisterAgent(ctx, "DuplicateAgent", "", "")
	require.Error(t, err)
}

func TestRegistry_GetAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	created, err := registry.RegisterAgent(ctx, "GetAgent", "", "")
	require.NoError(t, err)

	// Get by ID.
	agent, err := registry.GetAgent(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, agent.ID)
	require.Equal(t, created.Name, agent.Name)
}

func TestRegistry_GetAgent_NotFound(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Try to get non-existent agent.
	_, err := registry.GetAgent(ctx, 9999)
	require.Error(t, err)
}

func TestRegistry_GetAgentByName(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	created, err := registry.RegisterAgent(ctx, "NameLookupAgent", "", "")
	require.NoError(t, err)

	// Get by name.
	agent, err := registry.GetAgentByName(ctx, "NameLookupAgent")
	require.NoError(t, err)
	require.Equal(t, created.ID, agent.ID)
}

func TestRegistry_GetAgentByName_NotFound(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Try to get non-existent agent.
	_, err := registry.GetAgentByName(ctx, "NonExistentAgent")
	require.Error(t, err)
}

func TestRegistry_ListAgents(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register multiple agents.
	for _, name := range []string{"Agent1", "Agent2", "Agent3"} {
		_, err := registry.RegisterAgent(ctx, name, "", "")
		require.NoError(t, err)
	}

	// List all agents.
	agents, err := registry.ListAgents(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 3)
}

func TestRegistry_ListAgentsByProject(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register agents in different projects.
	_, err := registry.RegisterAgent(ctx, "ProjectA1", "/project/a", "")
	require.NoError(t, err)
	_, err = registry.RegisterAgent(ctx, "ProjectA2", "/project/a", "")
	require.NoError(t, err)
	_, err = registry.RegisterAgent(ctx, "ProjectB1", "/project/b", "")
	require.NoError(t, err)

	// List agents in project A.
	agents, err := registry.ListAgentsByProject(ctx, "/project/a")
	require.NoError(t, err)
	require.Len(t, agents, 2)
}

func TestRegistry_UpdateLastActive(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	agent, err := registry.RegisterAgent(ctx, "ActiveAgent", "", "")
	require.NoError(t, err)
	originalLastActive := agent.LastActiveAt

	// Update last active.
	err = registry.UpdateLastActive(ctx, agent.ID)
	require.NoError(t, err)

	// Get agent and verify update.
	updated, err := registry.GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, updated.LastActiveAt, originalLastActive)
}

func TestGenerateMemoableName(t *testing.T) {
	t.Parallel()

	// Generate multiple names and verify they are valid.
	names := make(map[string]bool)

	for i := 0; i < 100; i++ {
		name := GenerateMemoableName()
		require.NotEmpty(t, name)
		require.Greater(t, len(name), 5) // At least "X" + "Y"
		names[name] = true
	}

	// Should have generated mostly unique names.
	require.Greater(t, len(names), 50)
}

func TestRegistry_EnsureUniqueAgentName(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Generate a unique name.
	name, err := registry.EnsureUniqueAgentName(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, name)

	// Register with that name.
	_, err = registry.RegisterAgent(ctx, name, "", "")
	require.NoError(t, err)

	// Generate another unique name - should be different.
	name2, err := registry.EnsureUniqueAgentName(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, name2)
}

func TestRegistry_DeleteAgent(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	agent, err := registry.RegisterAgent(ctx, "delete-test-agent", "", "")
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Verify agent exists.
	found, err := registry.GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	// Delete the agent.
	err = registry.DeleteAgent(ctx, agent.ID)
	require.NoError(t, err)

	// Verify agent no longer exists.
	_, err = registry.GetAgent(ctx, agent.ID)
	require.Error(t, err)
}

func TestRegistry_DeleteAgent_NotFound(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Delete non-existent agent - should not error.
	err := registry.DeleteAgent(ctx, 999999)
	require.NoError(t, err)
}

func TestRegistry_DiscoverAgents(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register multiple agents with different projects.
	_, err := registry.RegisterAgent(
		ctx, "DiscoverAlice", "/project/alpha", "main",
	)
	require.NoError(t, err)
	_, err = registry.RegisterAgent(
		ctx, "DiscoverBob", "/project/beta", "feature",
	)
	require.NoError(t, err)

	// Discover all agents.
	rows, err := registry.DiscoverAgents(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// Verify the rows have the expected fields populated.
	for _, row := range rows {
		require.NotEmpty(t, row.Name)
		require.Greater(t, row.ID, int64(0))
		// UnreadCount should be 0 since no messages sent.
		require.Equal(t, int64(0), row.UnreadCount)
	}
}

func TestRegistry_UpdateDiscoveryInfo(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent.
	ag, err := registry.RegisterAgent(
		ctx, "DiscoveryInfoAgent", "/work/dir", "",
	)
	require.NoError(t, err)

	// Update discovery info.
	err = registry.UpdateDiscoveryInfo(
		ctx, ag.ID, "backend dev", "/home/user/project",
		"dev-machine",
	)
	require.NoError(t, err)

	// Verify via DiscoverAgents.
	rows, err := registry.DiscoverAgents(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Equal(t, "backend dev", row.Purpose.String)
	require.Equal(t, "/home/user/project", row.WorkingDir.String)
	require.Equal(t, "dev-machine", row.Hostname.String)
}

func TestRegistry_UpdateDiscoveryInfo_PartialUpdate(t *testing.T) {
	t.Parallel()

	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)
	ctx := context.Background()

	// Register an agent and set initial discovery info.
	ag, err := registry.RegisterAgent(ctx, "PartialAgent", "", "")
	require.NoError(t, err)

	err = registry.UpdateDiscoveryInfo(
		ctx, ag.ID, "reviewer", "/path/one", "host1",
	)
	require.NoError(t, err)

	// Update only purpose (empty strings should preserve existing).
	err = registry.UpdateDiscoveryInfo(
		ctx, ag.ID, "security reviewer", "", "",
	)
	require.NoError(t, err)

	// Verify existing fields were preserved.
	rows, err := registry.DiscoverAgents(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Equal(t, "security reviewer", row.Purpose.String)
	require.Equal(t, "/path/one", row.WorkingDir.String)
	require.Equal(t, "host1", row.Hostname.String)
}
