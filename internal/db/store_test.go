package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

// testDB creates a temporary test database with migrations applied.
func testDB(t *testing.T) (*Store, func()) {
	t.Helper()

	// Create temp directory for test database.
	tmpDir, err := os.MkdirTemp("", "subtrate-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	store, err := Open(dbPath)
	require.NoError(t, err)

	// Find migrations directory.
	migrationsDir := findMigrationsDir(t)

	// Run migrations.
	err = RunMigrations(store.DB(), migrationsDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

// findMigrationsDir locates the migrations directory relative to the test.
func findMigrationsDir(t *testing.T) string {
	t.Helper()

	// Try relative paths.
	paths := []string{
		"migrations",
		"../db/migrations",
		"../../internal/db/migrations",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fall back to absolute path from GOPATH.
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

func TestNewStore(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	require.NotNil(t, store)
	require.NotNil(t, store.Queries())
	require.NotNil(t, store.DB())
}

func TestWithTx_Commit(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent within a transaction.
	err := store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		_, err := q.CreateAgent(ctx, sqlc.CreateAgentParams{
			Name:      "TestAgent",
			CreatedAt: 1234567890,
		})
		return err
	})
	require.NoError(t, err)

	// Verify the agent was created.
	agent, err := store.Queries().GetAgentByName(ctx, "TestAgent")
	require.NoError(t, err)
	require.Equal(t, "TestAgent", agent.Name)
}

func TestWithTx_Rollback(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create an agent, then return an error to trigger rollback.
	err := store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		_, err := q.CreateAgent(ctx, sqlc.CreateAgentParams{
			Name:      "RollbackAgent",
			CreatedAt: 1234567890,
		})
		if err != nil {
			return err
		}

		// Force rollback by returning error.
		return sql.ErrNoRows
	})
	require.Error(t, err)

	// Verify the agent was NOT created.
	_, err = store.Queries().GetAgentByName(ctx, "RollbackAgent")
	require.Error(t, err)
}

func TestNextLogOffset(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a topic.
	topic, err := store.Queries().CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      "test-topic",
		TopicType: "broadcast",
		CreatedAt: 1234567890,
	})
	require.NoError(t, err)

	// First offset should be 1.
	err = store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		offset, err := NextLogOffset(ctx, q, topic.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), offset)
		return nil
	})
	require.NoError(t, err)

	// Create a message to increment the offset.
	err = store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		// First create an agent for the sender.
		agent, err := q.CreateAgent(ctx, sqlc.CreateAgentParams{
			Name:      "Sender",
			CreatedAt: 1234567890,
		})
		if err != nil {
			return err
		}

		_, err = q.CreateMessage(ctx, sqlc.CreateMessageParams{
			ThreadID:  "thread-1",
			TopicID:   topic.ID,
			LogOffset: 1,
			SenderID:  agent.ID,
			Subject:   "Test",
			BodyMd:    "Body",
			Priority:  "normal",
			CreatedAt: 1234567890,
		})
		return err
	})
	require.NoError(t, err)

	// Next offset should be 2.
	err = store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		offset, err := NextLogOffset(ctx, q, topic.ID)
		require.NoError(t, err)
		require.Equal(t, int64(2), offset)
		return nil
	})
	require.NoError(t, err)
}
