package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DefaultDBPath returns the default path for the subtrate database.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".subtrate", "subtrate.db"), nil
}

// OpenSQLite opens a SQLite database connection with WAL mode enabled and
// appropriate pragmas for performance and reliability.
func OpenSQLite(dbPath string) (*sql.DB, error) {
	// Ensure the directory exists.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open the database with foreign keys and WAL mode enabled via URI.
	dsn := fmt.Sprintf(
		"file:%s?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000",
		dbPath,
	)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite (single writer, multiple readers).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Verify connection and apply additional pragmas.
	if err := configurePragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	return db, nil
}

// configurePragmas sets additional SQLite pragmas for optimal performance.
func configurePragmas(db *sql.DB) error {
	pragmas := []string{
		// Synchronous mode: NORMAL provides good durability with better
		// performance than FULL.
		"PRAGMA synchronous = NORMAL",

		// Cache size: Negative value is in KiB, 64MB cache.
		"PRAGMA cache_size = -65536",

		// Memory-mapped I/O: 256MB for faster reads.
		"PRAGMA mmap_size = 268435456",

		// Temp store: Keep temporary tables in memory.
		"PRAGMA temp_store = MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute %q: %w", pragma, err)
		}
	}

	return nil
}

// Open opens the SQLite database and returns a Store wrapping it.
func Open(dbPath string) (*Store, error) {
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}

	return NewStore(db), nil
}

// RunMigrations applies all pending database migrations. For now, this
// executes the init migration directly. A proper migration system could be
// added later.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	// Read and execute the init migration.
	initPath := filepath.Join(migrationsDir, "000001_init.up.sql")

	migrationSQL, err := os.ReadFile(initPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	if _, err := db.Exec(string(migrationSQL)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}
