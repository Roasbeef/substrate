package db

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
)

var (
	// ErrRetriesExceeded is returned when a transaction is retried more
	// than the max allowed value without a success.
	ErrRetriesExceeded = errors.New("db tx retries exceeded")
)

// MapSQLError attempts to interpret a given error as a database agnostic SQL
// error.
func MapSQLError(err error) error {
	// Attempt to interpret the error as a sqlite error.
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return parseSqliteError(sqliteErr)
	}

	// Return the error if it could not be classified as a database specific
	// error.
	return err
}

// parseSqliteError attempts to parse a sqlite error as a database agnostic
// SQL error.
func parseSqliteError(sqliteErr sqlite3.Error) error {
	switch sqliteErr.Code {
	// Handle unique constraint violation error.
	case sqlite3.ErrConstraint:
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {

			return &ErrSQLUniqueConstraintViolation{
				DBError: sqliteErr,
			}
		}

		return fmt.Errorf("sqlite constraint error: %w", sqliteErr)

	// Database is currently busy, so we'll need to try again.
	case sqlite3.ErrBusy:
		return &ErrSerializationError{
			DBError: sqliteErr,
		}

	// A write operation could not continue because of a conflict within
	// the same database connection.
	case sqlite3.ErrLocked:
		return &ErrDeadlockError{
			DBError: sqliteErr,
		}

	// Generic error, need to parse the message further.
	case sqlite3.ErrError:
		errMsg := sqliteErr.Error()

		switch {
		case strings.Contains(errMsg, "no such table"):
			return &ErrSchemaError{
				DBError: sqliteErr,
			}

		default:
			return fmt.Errorf("unknown sqlite error: %w", sqliteErr)
		}

	default:
		return fmt.Errorf("unknown sqlite error: %w", sqliteErr)
	}
}

// ErrSQLUniqueConstraintViolation is an error type which represents a database
// agnostic SQL unique constraint violation.
type ErrSQLUniqueConstraintViolation struct {
	DBError error
}

// Error returns the error message.
func (e ErrSQLUniqueConstraintViolation) Error() string {
	return fmt.Sprintf("sql unique constraint violation: %v", e.DBError)
}

// ErrSerializationError is an error type which represents a database agnostic
// error that a transaction couldn't be serialized with other concurrent db
// transactions.
type ErrSerializationError struct {
	DBError error
}

// Unwrap returns the wrapped error.
func (e ErrSerializationError) Unwrap() error {
	return e.DBError
}

// Error returns the error message.
func (e ErrSerializationError) Error() string {
	return e.DBError.Error()
}

// ErrDeadlockError is an error type which represents a database agnostic error
// where transactions have led to cyclic dependencies in lock acquisition.
type ErrDeadlockError struct {
	DBError error
}

// Unwrap returns the wrapped error.
func (e ErrDeadlockError) Unwrap() error {
	return e.DBError
}

// Error returns the error message.
func (e ErrDeadlockError) Error() string {
	return e.DBError.Error()
}

// IsSerializationError returns true if the given error is a serialization
// error.
func IsSerializationError(err error) bool {
	var serializationError *ErrSerializationError
	return errors.As(err, &serializationError)
}

// IsDeadlockError returns true if the given error is a deadlock error.
func IsDeadlockError(err error) bool {
	var deadlockError *ErrDeadlockError
	return errors.As(err, &deadlockError)
}

// IsSerializationOrDeadlockError returns true if the given error is either a
// deadlock error or a serialization error.
func IsSerializationOrDeadlockError(err error) bool {
	return IsDeadlockError(err) || IsSerializationError(err)
}

// ErrSchemaError is an error type which represents a database agnostic error
// that the schema of the database is incorrect for the given query.
type ErrSchemaError struct {
	DBError error
}

// Unwrap returns the wrapped error.
func (e ErrSchemaError) Unwrap() error {
	return e.DBError
}

// Error returns the error message.
func (e ErrSchemaError) Error() string {
	return e.DBError.Error()
}

// IsSchemaError returns true if the given error is a schema error.
func IsSchemaError(err error) bool {
	var schemaError *ErrSchemaError
	return errors.As(err, &schemaError)
}
