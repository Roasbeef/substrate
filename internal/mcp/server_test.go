package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/stretchr/testify/require"
)

// testDB creates a temporary test database with migrations applied.
func testDB(t *testing.T) (*db.Store, func()) {
	t.Helper()

	// Create temp directory for test database.
	tmpDir, err := os.MkdirTemp("", "subtrate-mcp-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database.
	store, err := db.Open(dbPath)
	require.NoError(t, err)

	// Find migrations directory.
	migrationsDir := findMigrationsDir(t)

	// Run migrations.
	err = db.RunMigrations(store.DB(), migrationsDir)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

// findMigrationsDir locates the migrations directory.
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

// TestNewServer verifies that the MCP server can be created without panicking.
// This tests that all tool schemas are valid.
func TestNewServer(t *testing.T) {
	store, cleanup := testDB(t)
	defer cleanup()

	// This should not panic - if it does, the schema validation is broken.
	server := NewServer(store)
	require.NotNil(t, server)
}
