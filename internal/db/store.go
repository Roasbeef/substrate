package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// Store wraps the BaseDB with transaction support and additional
// business logic methods. It provides the TransactionExecutor for automatic
// retry on serialization errors.
type Store struct {
	*BaseDB

	// txExecutor handles transactional operations with automatic retry.
	txExecutor *TransactionExecutor[*sqlc.Queries]

	log *slog.Logger
}

// NewStore creates a new Store instance wrapping the given database
// connection.
func NewStore(db *sql.DB) *Store {
	return NewStoreWithLogger(db, slog.Default())
}

// NewStoreWithLogger creates a new Store instance with a custom logger.
func NewStoreWithLogger(db *sql.DB, log *slog.Logger) *Store {
	baseDB := NewBaseDB(db)

	// Create query creator function for transaction executor.
	createQuery := func(tx *sql.Tx) *sqlc.Queries {
		return sqlc.New(tx)
	}

	return &Store{
		BaseDB: baseDB,
		txExecutor: NewTransactionExecutor(
			baseDB, createQuery, log,
		),
		log: log,
	}
}

// Queries returns the underlying sqlc Queries for direct access to generated
// query methods.
func (s *Store) Queries() *sqlc.Queries {
	return s.BaseDB.Queries
}

// ExecTx executes the given function within a database transaction with
// automatic retry on serialization errors. This is the preferred method for
// transactional operations.
func (s *Store) ExecTx(ctx context.Context, txOptions TxOptions,
	txBody func(*sqlc.Queries) error,
) error {
	return s.txExecutor.ExecTx(ctx, txOptions, txBody)
}

// TxFunc is the function signature for transaction callbacks. The callback
// receives a Queries instance bound to the transaction.
type TxFunc func(ctx context.Context, q *sqlc.Queries) error

// WithTx executes the given function within a database transaction with
// automatic retry on serialization errors. If the function returns an error,
// the transaction is rolled back. Otherwise, it is committed.
func (s *Store) WithTx(ctx context.Context, fn TxFunc) error {
	return s.ExecTx(ctx, WriteTxOption(), func(q *sqlc.Queries) error {
		return fn(ctx, q)
	})
}

// WithReadTx executes the given function within a read-only database
// transaction. This is more efficient for read-only operations.
func (s *Store) WithReadTx(ctx context.Context, fn TxFunc) error {
	return s.ExecTx(ctx, ReadTxOption(), func(q *sqlc.Queries) error {
		return fn(ctx, q)
	})
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.BaseDB.Close()
}

// DB returns the underlying database connection. This method exists for
// compatibility with code that expects a DB() method.
func (s *Store) DB() *sql.DB {
	return s.BaseDB.DB
}

// TxFuncResult is the function signature for transaction callbacks that return
// a value. The callback receives a Queries instance bound to the transaction.
type TxFuncResult[T any] func(ctx context.Context, q *sqlc.Queries) (T, error)

// WithTxResult executes the given function within a database transaction and
// returns the result. If the function returns an error, the transaction is
// rolled back. Otherwise, it is committed and the result is returned.
func WithTxResult[T any](s *Store, ctx context.Context,
	fn TxFuncResult[T],
) (T, error) {
	var result T

	err := s.ExecTx(ctx, WriteTxOption(), func(q *sqlc.Queries) error {
		var err error
		result, err = fn(ctx, q)
		return err
	})

	return result, err
}

// WithReadTxResult executes the given function within a read-only database
// transaction and returns the result.
func WithReadTxResult[T any](s *Store, ctx context.Context,
	fn TxFuncResult[T],
) (T, error) {
	var result T

	err := s.ExecTx(ctx, ReadTxOption(), func(q *sqlc.Queries) error {
		var err error
		result, err = fn(ctx, q)
		return err
	})

	return result, err
}

// NextLogOffset returns the next available log offset for the given topic.
// This must be called within a transaction to ensure atomicity.
func NextLogOffset(ctx context.Context, q *sqlc.Queries, topicID int64) (int64,
	error,
) {
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
