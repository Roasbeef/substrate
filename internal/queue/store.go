package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// QueueStore provides access to the local offline operation queue. It wraps
// a db.SqliteStore which uses the standard migration system to initialize
// the database schema, including the pending_operations table from migration
// 000004.
type QueueStore struct {
	sqliteStore *db.SqliteStore
	cfg         QueueConfig
}

// OpenQueueStore opens a queue database at the given path and runs
// migrations to ensure the schema is up to date. It uses db.NewSqliteStore
// for consistent connection settings and migration handling.
func OpenQueueStore(dbPath string, cfg QueueConfig) (*QueueStore, error) {
	if err := EnsureQueueDir(
		// The queue.db lives inside .substrate/, so we need the
		// parent of the parent directory.
		dbPath[:len(dbPath)-len("/queue.db")],
	); err != nil {
		return nil, fmt.Errorf("ensure queue dir: %w", err)
	}

	sqliteStore, err := db.NewSqliteStore(&db.SqliteConfig{
		DatabaseFileName:      dbPath,
		SkipMigrationDBBackup: true,
	}, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("open queue db: %w", err)
	}

	return &QueueStore{
		sqliteStore: sqliteStore,
		cfg:         cfg,
	}, nil
}

// Enqueue adds a new operation to the queue. It returns ErrQueueFull if the
// number of pending operations has reached MaxPending.
func (s *QueueStore) Enqueue(
	ctx context.Context, op PendingOperation,
) error {

	return s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		// Check current count against the configured maximum.
		count, err := q.CountPendingOperations(ctx)
		if err != nil {
			return fmt.Errorf("count pending: %w", err)
		}
		if int(count) >= s.cfg.MaxPending {
			return ErrQueueFull
		}

		_, err = q.EnqueueOperation(ctx, sqlc.EnqueueOperationParams{
			IdempotencyKey: op.IdempotencyKey,
			OperationType:  string(op.OperationType),
			PayloadJson:    op.PayloadJSON,
			AgentName:      op.AgentName,
			SessionID:      toSqlcNullString(op.SessionID),
			CreatedAt:      op.CreatedAt.Unix(),
			ExpiresAt:      op.ExpiresAt.Unix(),
		})
		if err != nil {
			return fmt.Errorf("enqueue operation: %w", err)
		}

		return nil
	})
}

// List returns all pending operations in FIFO order without changing
// their status.
func (s *QueueStore) List(ctx context.Context) ([]PendingOperation, error) {
	var ops []PendingOperation

	err := s.sqliteStore.WithReadTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		rows, err := q.ListPendingOperations(ctx)
		if err != nil {
			return err
		}

		ops = make([]PendingOperation, len(rows))
		for i, row := range rows {
			ops[i] = PendingOperationFromSqlc(row)
		}

		return nil
	})

	return ops, err
}

// Drain atomically marks all pending operations as 'delivering' and returns
// them. This prevents concurrent drain from processing the same operations.
func (s *QueueStore) Drain(ctx context.Context) ([]PendingOperation, error) {
	var ops []PendingOperation

	err := s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		rows, err := q.DrainPendingOperations(ctx)
		if err != nil {
			return err
		}

		ops = make([]PendingOperation, len(rows))
		for i, row := range rows {
			ops[i] = PendingOperationFromSqlc(row)
		}

		return nil
	})

	return ops, err
}

// MarkDelivered marks an operation as successfully delivered.
func (s *QueueStore) MarkDelivered(ctx context.Context, id int64) error {
	return s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		return q.MarkOperationDelivered(ctx, id)
	})
}

// MarkFailed marks an operation as failed and increments the attempt count.
// The operation status is reset to 'pending' so it will be retried on the
// next drain cycle.
func (s *QueueStore) MarkFailed(
	ctx context.Context, id int64, errMsg string,
) error {

	return s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		return q.MarkOperationFailed(ctx, sqlc.MarkOperationFailedParams{
			ID:        id,
			LastError: toSqlcNullString(errMsg),
		})
	})
}

// Clear deletes all operations from the queue regardless of status.
func (s *QueueStore) Clear(ctx context.Context) error {
	return s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		return q.ClearAllOperations(ctx)
	})
}

// PurgeExpired removes operations that have passed their expiry time and
// are in a pending or failed state. Returns the number of purged rows.
func (s *QueueStore) PurgeExpired(ctx context.Context) (int64, error) {
	var purged int64

	err := s.sqliteStore.WithTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		var err error
		purged, err = q.PurgeExpiredOperations(
			ctx, time.Now().Unix(),
		)
		return err
	})

	return purged, err
}

// Stats returns aggregate counts for all operations in the queue.
func (s *QueueStore) Stats(ctx context.Context) (QueueStats, error) {
	var stats QueueStats

	err := s.sqliteStore.WithReadTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		row, err := q.GetQueueStats(ctx)
		if err != nil {
			return err
		}

		stats = QueueStatsFromSqlc(row)

		return nil
	})

	return stats, err
}

// Count returns the number of pending operations.
func (s *QueueStore) Count(ctx context.Context) (int64, error) {
	var count int64

	err := s.sqliteStore.WithReadTx(ctx, func(
		ctx context.Context, q *sqlc.Queries,
	) error {
		var err error
		count, err = q.CountPendingOperations(ctx)
		return err
	})

	return count, err
}

// Close closes the underlying database connection.
func (s *QueueStore) Close() error {
	return s.sqliteStore.DB().Close()
}
