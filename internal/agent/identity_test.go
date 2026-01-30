package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

// testIdentityManager creates an IdentityManager with a temporary directory.
func testIdentityManager(t *testing.T) (*IdentityManager, *db.Store, func()) {
	t.Helper()

	store, storeCleanup := testDB(t)
	registry := NewRegistry(store)

	// Create temporary identity directory.
	tmpDir, err := os.MkdirTemp("", "subtrate-identity-test-*")
	require.NoError(t, err)

	// Create directory structure.
	dirs := []string{
		filepath.Join(tmpDir, "by-session"),
		filepath.Join(tmpDir, "by-project"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0700)
		require.NoError(t, err)
	}

	mgr := &IdentityManager{
		store:       store,
		registry:    registry,
		identityDir: tmpDir,
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
		storeCleanup()
	}

	return mgr, store, cleanup
}

func TestNewIdentityManager(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	registry := NewRegistry(store)

	mgr, err := NewIdentityManager(store, registry)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	require.NotEmpty(t, mgr.identityDir)

	// Verify directories were created.
	sessionDir := filepath.Join(mgr.identityDir, "by-session")
	projectDir := filepath.Join(mgr.identityDir, "by-project")

	_, err = os.Stat(sessionDir)
	require.NoError(t, err)

	_, err = os.Stat(projectDir)
	require.NoError(t, err)
}

func TestIdentityManager_EnsureIdentity_NewAgent(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Ensure identity for a new session.
	identity, err := mgr.EnsureIdentity(ctx, "test-session-1", "", "")
	require.NoError(t, err)
	require.NotNil(t, identity)

	require.Equal(t, "test-session-1", identity.SessionID)
	require.NotEmpty(t, identity.AgentName)
	require.Greater(t, identity.AgentID, int64(0))
	require.NotZero(t, identity.CreatedAt)
	require.NotZero(t, identity.LastActiveAt)

	// Verify agent was created in database.
	agent, err := mgr.registry.GetAgent(ctx, identity.AgentID)
	require.NoError(t, err)
	require.Equal(t, identity.AgentName, agent.Name)
}

func TestIdentityManager_EnsureIdentity_WithProject(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Ensure identity with project key.
	identity, err := mgr.EnsureIdentity(
		ctx, "test-session-project", "/path/to/project", "",
	)
	require.NoError(t, err)
	require.NotNil(t, identity)

	require.Equal(t, "test-session-project", identity.SessionID)
	require.Equal(t, "/path/to/project", identity.ProjectKey)

	// Verify project default was set.
	defaultIdentity, err := mgr.GetProjectDefaultIdentity(
		ctx, "/path/to/project",
	)
	require.NoError(t, err)
	require.Equal(t, identity.AgentID, defaultIdentity.AgentID)
}

func TestIdentityManager_EnsureIdentity_RestoresExisting(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial identity.
	identity1, err := mgr.EnsureIdentity(ctx, "test-session-restore", "", "")
	require.NoError(t, err)

	// Wait a bit to ensure time difference.
	time.Sleep(10 * time.Millisecond)

	// Ensure identity again - should restore, not create new.
	identity2, err := mgr.EnsureIdentity(ctx, "test-session-restore", "", "")
	require.NoError(t, err)

	require.Equal(t, identity1.AgentID, identity2.AgentID)
	require.Equal(t, identity1.AgentName, identity2.AgentName)
	require.True(t, identity2.LastActiveAt.After(identity1.LastActiveAt) ||
		identity2.LastActiveAt.Equal(identity1.LastActiveAt))
}

func TestIdentityManager_EnsureIdentity_UsesProjectDefault(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent and set as project default.
	agent, err := mgr.registry.RegisterAgent(ctx, "ProjectAgent", "/my/project", "")
	require.NoError(t, err)

	err = mgr.SetProjectDefault(ctx, "/my/project", agent.Name)
	require.NoError(t, err)

	// New session for same project should use existing agent.
	identity, err := mgr.EnsureIdentity(ctx, "new-session-for-project", "/my/project", "")
	require.NoError(t, err)

	require.Equal(t, agent.ID, identity.AgentID)
	require.Equal(t, agent.Name, identity.AgentName)
	require.Equal(t, "/my/project", identity.ProjectKey)
}

func TestIdentityManager_RestoreIdentity_FromFile(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent.
	agent, err := mgr.registry.RegisterAgent(ctx, "FileRestoreAgent", "", "")
	require.NoError(t, err)

	// Write identity file directly.
	identityFile := &IdentityFile{
		SessionID:       "file-session",
		AgentName:       agent.Name,
		AgentID:         agent.ID,
		CreatedAt:       time.Now().Add(-time.Hour),
		LastActiveAt:    time.Now(),
		ConsumerOffsets: map[string]int64{"topic1": 10},
	}

	filePath := filepath.Join(
		mgr.identityDir, "by-session", "file-session.json",
	)
	data, err := json.MarshalIndent(identityFile, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)

	// Restore should find the file.
	restored, err := mgr.RestoreIdentity(ctx, "file-session")
	require.NoError(t, err)
	require.Equal(t, agent.ID, restored.AgentID)
	require.Equal(t, agent.Name, restored.AgentName)
}

func TestIdentityManager_RestoreIdentity_FromDatabase(t *testing.T) {
	mgr, store, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent.
	agent, err := mgr.registry.RegisterAgent(ctx, "DBRestoreAgent", "", "")
	require.NoError(t, err)

	// Insert session identity directly into database.
	err = store.Queries().CreateSessionIdentity(
		ctx, sqlc.CreateSessionIdentityParams{
			SessionID:    "db-session",
			AgentID:      agent.ID,
			CreatedAt:    time.Now().Unix(),
			LastActiveAt: time.Now().Unix(),
		},
	)
	require.NoError(t, err)

	// Restore should find from database.
	restored, err := mgr.RestoreIdentity(ctx, "db-session")
	require.NoError(t, err)
	require.Equal(t, agent.ID, restored.AgentID)
	require.Equal(t, agent.Name, restored.AgentName)
}

func TestIdentityManager_RestoreIdentity_NotFound(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Should fail for non-existent session.
	_, err := mgr.RestoreIdentity(ctx, "non-existent-session")
	require.Error(t, err)
}

func TestIdentityManager_RestoreIdentity_AgentNoLongerExists(t *testing.T) {
	mgr, store, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent.
	agent, err := mgr.registry.RegisterAgent(ctx, "DeletedAgent", "", "")
	require.NoError(t, err)

	// Write identity file.
	identityFile := &IdentityFile{
		SessionID:    "deleted-agent-session",
		AgentName:    agent.Name,
		AgentID:      agent.ID,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}
	filePath := filepath.Join(
		mgr.identityDir, "by-session", "deleted-agent-session.json",
	)
	data, err := json.MarshalIndent(identityFile, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)

	// Delete the agent from database.
	err = store.Queries().DeleteAgent(ctx, agent.ID)
	require.NoError(t, err)

	// Restore should fail because agent no longer exists.
	_, err = mgr.RestoreIdentity(ctx, "deleted-agent-session")
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent no longer exists")
}

func TestIdentityManager_SaveIdentity(t *testing.T) {
	mgr, store, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial identity.
	identity, err := mgr.EnsureIdentity(ctx, "save-session", "", "")
	require.NoError(t, err)

	// Create a topic for testing consumer offsets.
	topic, err := store.Queries().CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      "test-topic",
		TopicType: "broadcast",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Update identity with consumer offset.
	identity.ConsumerOffsets = map[string]int64{
		"test-topic": 42,
	}

	err = mgr.SaveIdentity(ctx, identity)
	require.NoError(t, err)

	// Verify file was updated.
	filePath := filepath.Join(
		mgr.identityDir, "by-session", "save-session.json",
	)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var saved IdentityFile
	err = json.Unmarshal(data, &saved)
	require.NoError(t, err)
	require.Equal(t, int64(42), saved.ConsumerOffsets["test-topic"])

	// Verify offset was saved to database.
	lastOffset, err := store.Queries().GetConsumerOffset(
		ctx, sqlc.GetConsumerOffsetParams{
			AgentID: identity.AgentID,
			TopicID: topic.ID,
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(42), lastOffset)
}

func TestIdentityManager_GetProjectDefaultIdentity_FromFile(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent and set as project default.
	agent, err := mgr.registry.RegisterAgent(ctx, "ProjectDefaultAgent", "", "")
	require.NoError(t, err)

	err = mgr.SetProjectDefault(ctx, "/project/path", agent.Name)
	require.NoError(t, err)

	// Get project default.
	identity, err := mgr.GetProjectDefaultIdentity(ctx, "/project/path")
	require.NoError(t, err)
	require.Equal(t, agent.ID, identity.AgentID)
	require.Equal(t, agent.Name, identity.AgentName)
}

func TestIdentityManager_GetProjectDefaultIdentity_NotFound(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Should fail for unknown project.
	_, err := mgr.GetProjectDefaultIdentity(ctx, "/unknown/project")
	require.Error(t, err)
}

func TestIdentityManager_SetProjectDefault(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent.
	agent, err := mgr.registry.RegisterAgent(ctx, "SetDefaultAgent", "", "")
	require.NoError(t, err)

	// Set as project default.
	err = mgr.SetProjectDefault(ctx, "/new/project", agent.Name)
	require.NoError(t, err)

	// Verify file was created.
	hash := hashProjectKey("/new/project")
	filePath := filepath.Join(mgr.identityDir, "by-project", hash+".json")

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var identity IdentityFile
	err = json.Unmarshal(data, &identity)
	require.NoError(t, err)
	require.Equal(t, agent.ID, identity.AgentID)
	require.Equal(t, agent.Name, identity.AgentName)
	require.Equal(t, "/new/project", identity.ProjectKey)
}

func TestIdentityManager_SetProjectDefault_AgentNotFound(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Should fail for non-existent agent.
	err := mgr.SetProjectDefault(ctx, "/project", "NonExistentAgent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent not found")
}

func TestIdentityManager_CurrentIdentity(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create identity first.
	identity, err := mgr.EnsureIdentity(ctx, "current-session", "", "")
	require.NoError(t, err)

	// Get current identity.
	current, err := mgr.CurrentIdentity(ctx, "current-session")
	require.NoError(t, err)
	require.Equal(t, identity.AgentID, current.AgentID)
	require.Equal(t, identity.AgentName, current.AgentName)
}

func TestIdentityManager_ListIdentities(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create several identities.
	_, err := mgr.EnsureIdentity(ctx, "list-session-1", "", "")
	require.NoError(t, err)

	_, err = mgr.EnsureIdentity(ctx, "list-session-2", "", "")
	require.NoError(t, err)

	_, err = mgr.EnsureIdentity(ctx, "list-session-3", "/project/a", "")
	require.NoError(t, err)

	// List all identities.
	identities, err := mgr.ListIdentities(ctx)
	require.NoError(t, err)
	require.Len(t, identities, 3)

	// Verify all sessions are present.
	sessionIDs := make(map[string]bool)
	for _, id := range identities {
		sessionIDs[id.SessionID] = true
	}
	require.True(t, sessionIDs["list-session-1"])
	require.True(t, sessionIDs["list-session-2"])
	require.True(t, sessionIDs["list-session-3"])
}

func TestIdentityManager_ListIdentities_Empty(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// List with no identities.
	identities, err := mgr.ListIdentities(ctx)
	require.NoError(t, err)
	require.Empty(t, identities)
}

func TestHashProjectKey(t *testing.T) {
	// Test that same input produces same hash.
	hash1 := hashProjectKey("/path/to/project")
	hash2 := hashProjectKey("/path/to/project")
	require.Equal(t, hash1, hash2)

	// Test that different inputs produce different hashes.
	hash3 := hashProjectKey("/different/path")
	require.NotEqual(t, hash1, hash3)

	// Test that hash is 8 hex characters (32-bit).
	require.Len(t, hash1, 8)
	require.Regexp(t, "^[0-9a-f]+$", hash1)
}

func TestIdentityManager_SaveIdentityFile_EmptySessionID(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	// Empty session ID should be a no-op.
	identity := &IdentityFile{
		SessionID: "",
		AgentName: "TestAgent",
		AgentID:   1,
	}

	err := mgr.saveIdentityFile(identity)
	require.NoError(t, err)

	// Verify no file was created.
	entries, err := os.ReadDir(filepath.Join(mgr.identityDir, "by-session"))
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestIdentityManager_SaveSessionDB_EmptySessionID(t *testing.T) {
	mgr, _, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Empty session ID should be a no-op.
	identity := &IdentityFile{
		SessionID: "",
		AgentName: "TestAgent",
		AgentID:   1,
	}

	err := mgr.saveSessionDB(ctx, identity)
	require.NoError(t, err)
}

func TestIdentityManager_EnsureIdentity_UpdatesGitBranch(t *testing.T) {
	mgr, store, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial identity without git branch.
	identity1, err := mgr.EnsureIdentity(
		ctx, "git-branch-session", "/project/path", "",
	)
	require.NoError(t, err)
	require.Empty(t, identity1.GitBranch)

	// Verify agent was created without git branch.
	agent, err := store.Queries().GetAgent(ctx, identity1.AgentID)
	require.NoError(t, err)
	require.False(t, agent.GitBranch.Valid)

	// Call EnsureIdentity again with git branch - should update.
	identity2, err := mgr.EnsureIdentity(
		ctx, "git-branch-session", "/project/path", "feature-branch",
	)
	require.NoError(t, err)
	require.Equal(t, identity1.AgentID, identity2.AgentID)
	require.Equal(t, "feature-branch", identity2.GitBranch)

	// Verify agent record was updated.
	agent, err = store.Queries().GetAgent(ctx, identity2.AgentID)
	require.NoError(t, err)
	require.True(t, agent.GitBranch.Valid)
	require.Equal(t, "feature-branch", agent.GitBranch.String)
}

func TestIdentityManager_EnsureIdentity_UpdatesGitBranch_ProjectDefault(t *testing.T) {
	mgr, store, cleanup := testIdentityManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent and set as project default.
	agent, err := mgr.registry.RegisterAgent(
		ctx, "ProjectDefaultBranch", "/branch/project", "",
	)
	require.NoError(t, err)

	err = mgr.SetProjectDefault(ctx, "/branch/project", agent.Name)
	require.NoError(t, err)

	// New session for same project should use existing agent and update branch.
	identity, err := mgr.EnsureIdentity(
		ctx, "new-session-branch", "/branch/project", "main",
	)
	require.NoError(t, err)
	require.Equal(t, agent.ID, identity.AgentID)
	require.Equal(t, "main", identity.GitBranch)

	// Verify agent record was updated with git branch.
	updatedAgent, err := store.Queries().GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	require.True(t, updatedAgent.GitBranch.Valid)
	require.Equal(t, "main", updatedAgent.GitBranch.String)
}
