package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// Store wraps the sqlc Queries with transaction support and additional
// business logic methods.
type Store struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewStore creates a new Store instance wrapping the given database
// connection.
func NewStore(db *sql.DB) *Store {
	return &Store{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Queries returns the underlying sqlc Queries for direct access to generated
// query methods.
func (s *Store) Queries() *sqlc.Queries {
	return s.queries
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// TxFunc is the function signature for transaction callbacks. The callback
// receives a Queries instance bound to the transaction.
type TxFunc func(ctx context.Context, q *sqlc.Queries) error

// WithTx executes the given function within a database transaction. If the
// function returns an error, the transaction is rolled back. Otherwise, it is
// committed.
func (s *Store) WithTx(ctx context.Context, fn TxFunc) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new Queries instance bound to this transaction.
	txQueries := sqlc.New(tx)

	// Execute the callback.
	if err := fn(ctx, txQueries); err != nil {
		// Attempt rollback, but prioritize returning the original error.
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx error: %w, rollback error: %v", err, rbErr)
		}

		return err
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// TxFuncResult is the function signature for transaction callbacks that return
// a value. The callback receives a Queries instance bound to the transaction.
type TxFuncResult[T any] func(ctx context.Context, q *sqlc.Queries) (T, error)

// WithTxResult executes the given function within a database transaction and
// returns the result. If the function returns an error, the transaction is
// rolled back. Otherwise, it is committed and the result is returned.
func (s *Store) WithTxResult(ctx context.Context, fn TxFuncResult[int64]) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new Queries instance bound to this transaction.
	txQueries := sqlc.New(tx)

	// Execute the callback.
	result, err := fn(ctx, txQueries)
	if err != nil {
		// Attempt rollback, but prioritize returning the original error.
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, fmt.Errorf("tx error: %w, rollback error: %v", err, rbErr)
		}

		return 0, err
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// NextLogOffset returns the next available log offset for the given topic.
// This must be called within a transaction to ensure atomicity.
func NextLogOffset(ctx context.Context, q *sqlc.Queries, topicID int64) (int64,
	error) {

	result, err := q.GetMaxLogOffset(ctx, topicID)
	if err != nil {
		return 0, fmt.Errorf("failed to get max log offset: %w", err)
	}

	// COALESCE returns interface{}, need to type assert.
	var maxOffset int64
	switch v := result.(type) {
	case int64:
		maxOffset = v
	case int:
		maxOffset = int64(v)
	case nil:
		maxOffset = 0
	default:
		return 0, fmt.Errorf("unexpected type for max offset: %T", result)
	}

	return maxOffset + 1, nil
}
