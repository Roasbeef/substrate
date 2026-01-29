package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

const (
	// defaultMaxConns is the number of permitted active and idle
	// connections. For SQLite, we want single writer, multiple readers.
	defaultMaxConns = 25

	// defaultConnMaxLifetime is the maximum amount of time a connection can
	// be reused for before it is closed.
	defaultConnMaxLifetime = 10 * time.Minute
)

// SqliteConfig holds all the config arguments needed to interact with our
// sqlite DB.
type SqliteConfig struct {
	// SkipMigrations if true, then all the tables will be created on start
	// up if they don't already exist.
	SkipMigrations bool

	// SkipMigrationDBBackup if true, then a backup of the database will not
	// be created before applying migrations.
	SkipMigrationDBBackup bool

	// DatabaseFileName is the full file path where the database file can be
	// found.
	DatabaseFileName string
}

// SqliteStore is a sqlite3 based database for the daemon.
type SqliteStore struct {
	cfg *SqliteConfig
	log *slog.Logger

	*Store
}

// NewSqliteStore attempts to open a new sqlite database based on the passed
// config.
func NewSqliteStore(cfg *SqliteConfig, log *slog.Logger) (*SqliteStore, error) {
	// Ensure the directory exists.
	dir := filepath.Dir(cfg.DatabaseFileName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open the database with foreign keys and WAL mode enabled via URI.
	dsn := fmt.Sprintf(
		"file:%s?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000",
		cfg.DatabaseFileName,
	)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(defaultMaxConns)
	db.SetMaxIdleConns(defaultMaxConns)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)

	// Apply additional pragmas.
	if err := configurePragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	s := &SqliteStore{
		cfg:   cfg,
		log:   log,
		Store: NewStore(db),
	}

	// Run migrations unless skipped.
	if !cfg.SkipMigrations {
		err := s.ExecuteMigrations(s.backupAndMigrate)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("error executing migrations: %w", err)
		}
	}

	return s, nil
}

// backupAndMigrate is a helper function that creates a database backup before
// initiating the migration, and then migrates the database to the latest
// version.
func (s *SqliteStore) backupAndMigrate(mig *migrate.Migrate,
	currentDBVersion int, maxMigrationVersion uint) error {

	// Determine if a database migration is necessary given the current
	// database version and the maximum migration version.
	versionUpgradePending := currentDBVersion < int(maxMigrationVersion)
	if !versionUpgradePending {
		s.log.InfoContext(
			context.Background(),
			"Current database version is up-to-date, skipping "+
				"migration attempt and backup creation",
			"current_db_version", currentDBVersion,
			"max_migration_version", maxMigrationVersion,
		)

		return nil
	}

	// At this point, we know that a database migration is necessary.
	// Create a backup of the database before starting the migration.
	if !s.cfg.SkipMigrationDBBackup {
		s.log.InfoContext(
			context.Background(),
			"Creating database backup (before applying migration(s))",
		)

		err := backupSqliteDatabase(
			s.DB(), s.cfg.DatabaseFileName, s.log,
		)
		if err != nil {
			return err
		}
	} else {
		s.log.InfoContext(
			context.Background(),
			"Skipping database backup creation before applying "+
				"migration(s)",
		)
	}

	s.log.InfoContext(context.Background(), "Applying migrations to database")

	return mig.Up()
}

// ExecuteMigrations runs migrations for the sqlite database, depending on the
// target given, either all migrations or up to a given version.
func (s *SqliteStore) ExecuteMigrations(target MigrationTarget,
	optFuncs ...MigrateOpt) error {

	opts := defaultMigrateOptions()
	for _, optFunc := range optFuncs {
		optFunc(opts)
	}

	driver, err := sqlite_migrate.WithInstance(
		s.DB(), &sqlite_migrate.Config{},
	)
	if err != nil {
		return fmt.Errorf("error creating sqlite migration: %w", err)
	}

	return applyMigrations(
		sqlSchemas, driver, "migrations", "sqlite", target, opts,
		s.log,
	)
}

// DefaultDBPath returns the default path for the subtrate database.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".subtrate", "subtrate.db"), nil
}

// OpenSQLite opens a SQLite database connection with WAL mode enabled and
// appropriate pragmas for performance and reliability. This is a low-level
// function; prefer NewSqliteStore for full functionality with migrations.
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
// NOTE: This is a low-level function that doesn't run migrations.
// For daemon use, prefer NewSqliteStore.
func Open(dbPath string) (*Store, error) {
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}

	return NewStore(db), nil
}

// RunMigrations applies all pending database migrations using the legacy
// method of reading from file. Prefer NewSqliteStore which uses golang-migrate.
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
