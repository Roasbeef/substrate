package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

const (
	// LatestMigrationVersion is the latest migration version of the
	// database. This is used to implement downgrade protection for the
	// daemon.
	//
	// NOTE: This MUST be updated when a new migration is added.
	LatestMigrationVersion uint = 2
)

// MigrationTarget is a functional option that can be passed to applyMigrations
// to specify a target version to migrate to. `currentDBVersion` is the current
// (migration) version of the database, or None if unknown.
// `maxMigrationVersion` is the maximum migration version known to the driver.
type MigrationTarget func(mig *migrate.Migrate,
	currentDBVersion int, maxMigrationVersion uint) error

var (
	// TargetLatest is a MigrationTarget that migrates to the latest
	// version available.
	TargetLatest = func(mig *migrate.Migrate, _ int, _ uint) error {
		return mig.Up()
	}

	// TargetVersion returns a MigrationTarget that migrates to the given
	// version.
	TargetVersion = func(version uint) MigrationTarget {
		return func(mig *migrate.Migrate, _ int, _ uint) error {
			return mig.Migrate(version)
		}
	}
)

var (
	// ErrMigrationDowngrade is returned when a database downgrade is
	// detected.
	ErrMigrationDowngrade = errors.New("database downgrade detected")
)

// migrateOptions holds options for migration execution.
type migrateOptions struct {
	latestVersion uint
}

// defaultMigrateOptions returns a new migrateOptions instance with default
// settings.
func defaultMigrateOptions() *migrateOptions {
	return &migrateOptions{
		latestVersion: LatestMigrationVersion,
	}
}

// MigrateOpt is a functional option that can be passed to migrate related
// methods to modify behavior.
type MigrateOpt func(*migrateOptions)

// WithLatestVersion allows callers to override the default latest version
// setting.
func WithLatestVersion(version uint) MigrateOpt {
	return func(o *migrateOptions) {
		o.latestVersion = version
	}
}

// migrationLogger wraps slog.Logger to implement the migrate.Logger interface.
type migrationLogger struct {
	log *slog.Logger
}

// Printf implements the migrate.Logger interface.
func (m *migrationLogger) Printf(format string, v ...any) {
	// Trim trailing newlines from the format.
	format = strings.TrimRight(format, "\n")
	m.log.Info(fmt.Sprintf(format, v...))
}

// Verbose returns true when verbose logging is enabled.
func (m *migrationLogger) Verbose() bool {
	return true
}

// applyMigrations executes database migration files found in the given file
// system under the given path, using the passed database driver and database
// name, up to or down to the given target version.
func applyMigrations(fsys fs.FS, driver database.Driver, path, dbName string,
	targetVersion MigrationTarget, opts *migrateOptions,
	log *slog.Logger) error {

	// Create a new migration source using the embedded file system.
	migrateFileServer, err := httpfs.New(http.FS(fsys), path)
	if err != nil {
		return err
	}

	// Create the migration instance with our driver and source.
	sqlMigrate, err := migrate.NewWithInstance(
		"migrations", migrateFileServer, dbName, driver,
	)
	if err != nil {
		return err
	}

	migrationVersion, dirty, err := sqlMigrate.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("unable to determine current migration "+
			"version: %w", err)
	}

	// If the migration version is dirty, we should not proceed with further
	// migrations, as this indicates that a previous migration did not
	// complete successfully and requires manual intervention.
	if dirty {
		return fmt.Errorf("database is in a dirty state at version "+
			"%v, manual intervention required", migrationVersion)
	}

	// As the down migrations may end up *dropping* data, we want to
	// prevent that without explicit accounting.
	if migrationVersion > opts.latestVersion {
		return fmt.Errorf("%w: database version is newer than the "+
			"latest migration version, preventing downgrade: "+
			"db_version=%v, latest_migration_version=%v",
			ErrMigrationDowngrade, migrationVersion,
			opts.latestVersion)
	}

	// Report the current version of the database before the migration.
	currentDBVersion, _, err := driver.Version()
	if err != nil {
		return fmt.Errorf("unable to get current db version: %w", err)
	}
	log.InfoContext(
		context.Background(), "Attempting to apply migration(s)",
		"current_db_version", currentDBVersion,
		"latest_migration_version", opts.latestVersion,
	)

	// Apply our local logger to the migration instance.
	sqlMigrate.Log = &migrationLogger{log}

	// Execute the migration based on the target given.
	err = targetVersion(sqlMigrate, currentDBVersion, opts.latestVersion)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	// Report the current version of the database after the migration.
	currentDBVersion, _, err = driver.Version()
	if err != nil {
		return fmt.Errorf("unable to get current db version: %w", err)
	}
	log.InfoContext(
		context.Background(), "Database version after migration",
		"current_db_version", currentDBVersion,
	)

	return nil
}

// backupSqliteDatabase creates a backup of the given SQLite database.
func backupSqliteDatabase(srcDB *sql.DB, dbFullFilePath string,
	log *slog.Logger) error {

	if srcDB == nil {
		return fmt.Errorf("backup source database is nil")
	}

	// Create a database backup file full path from the given source
	// database full file path.
	timestamp := time.Now().UnixNano()
	backupFullFilePath := fmt.Sprintf(
		"%s.%d.backup", dbFullFilePath, timestamp,
	)

	log.InfoContext(context.Background(), "Creating backup of database file",
		"source", dbFullFilePath,
		"backup", backupFullFilePath,
	)

	// Create the database backup using VACUUM INTO.
	vacuumIntoQuery := "VACUUM INTO ?;"
	stmt, err := srcDB.Prepare(vacuumIntoQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(backupFullFilePath)
	if err != nil {
		return err
	}

	return nil
}
