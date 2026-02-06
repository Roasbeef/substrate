package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/roasbeef/subtrate/internal/db"
)

// newTestQueueStore creates a QueueStore backed by a real SQLite database
// in a temporary directory. The database is automatically cleaned up when
// the test finishes.
func newTestQueueStore(t *testing.T) *QueueStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "queue.db")
	sqlDB, err := db.OpenSQLite(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB.Close()
	})

	store, err := NewQueueStore(sqlDB, DefaultQueueConfig())
	require.NoError(t, err)

	return store
}

// newRapidQueueStore creates a QueueStore for property-based tests. It
// uses a unique directory based on a counter to avoid conflicts between
// rapid iterations (each iteration needs its own DB).
func newRapidQueueStore(t *testing.T) *QueueStore {
	t.Helper()

	// Use a unique subdirectory per call to avoid SQLite lock conflicts
	// between rapid iterations.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "queue.db")
	sqlDB, err := db.OpenSQLite(dbPath)
	require.NoError(t, err)

	store, err := NewQueueStore(sqlDB, DefaultQueueConfig())
	require.NoError(t, err)

	// Note: we don't add a Cleanup since t.TempDir handles removal,
	// and the rapid callback runs many iterations under one *testing.T.
	// Each iteration gets its own dir, so this is safe.
	t.Cleanup(func() {
		sqlDB.Close()
	})

	return store
}

// makeOp creates a PendingOperation with the given type and a unique
// idempotency key.
func makeOp(opType OperationType) PendingOperation {
	return PendingOperation{
		IdempotencyKey: uuid.Must(uuid.NewV7()).String(),
		OperationType:  opType,
		PayloadJSON:    `{"test": true}`,
		AgentName:      "test-agent",
		SessionID:      "test-session",
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(7 * 24 * time.Hour),
	}
}

// TestQueueStore_EnqueueAndList verifies that enqueued operations appear in
// List output in FIFO order.
func TestQueueStore_EnqueueAndList(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	// Enqueue three operations of different types.
	ops := []PendingOperation{
		makeOp(OpSend),
		makeOp(OpPublish),
		makeOp(OpHeartbeat),
	}
	for _, op := range ops {
		require.NoError(t, store.Enqueue(ctx, op))
	}

	// List should return all three in order.
	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 3)

	for i, op := range listed {
		require.Equal(t, ops[i].IdempotencyKey, op.IdempotencyKey)
		require.Equal(t, ops[i].OperationType, op.OperationType)
		require.Equal(t, "pending", op.Status)
	}

	// Count should match.
	count, err := store.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// TestQueueStore_DrainAndDeliver verifies the drain → deliver lifecycle.
func TestQueueStore_DrainAndDeliver(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	// Enqueue two operations.
	op1 := makeOp(OpSend)
	op2 := makeOp(OpPublish)
	require.NoError(t, store.Enqueue(ctx, op1))
	require.NoError(t, store.Enqueue(ctx, op2))

	// Drain should return both and mark them as 'delivering'.
	drained, err := store.Drain(ctx)
	require.NoError(t, err)
	require.Len(t, drained, 2)
	require.Equal(t, "delivering", drained[0].Status)
	require.Equal(t, "delivering", drained[1].Status)

	// List should now return empty since none are 'pending'.
	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Empty(t, listed)

	// A second drain should also return empty.
	drained2, err := store.Drain(ctx)
	require.NoError(t, err)
	require.Empty(t, drained2)

	// Mark both as delivered.
	require.NoError(t, store.MarkDelivered(ctx, drained[0].ID))
	require.NoError(t, store.MarkDelivered(ctx, drained[1].ID))

	// Count of pending should be zero.
	count, err := store.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

// TestQueueStore_MarkFailed verifies that failed operations are returned to
// pending state with incremented attempt counts.
func TestQueueStore_MarkFailed(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	op := makeOp(OpSend)
	require.NoError(t, store.Enqueue(ctx, op))

	// Drain and mark as failed.
	drained, err := store.Drain(ctx)
	require.NoError(t, err)
	require.Len(t, drained, 1)
	require.Equal(t, 0, drained[0].Attempts)

	require.NoError(t, store.MarkFailed(
		ctx, drained[0].ID, "connection refused",
	))

	// The operation should be back in pending state with attempt count 1.
	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "pending", listed[0].Status)
	require.Equal(t, 1, listed[0].Attempts)
	require.Equal(t, "connection refused", listed[0].LastError)

	// Fail again to verify attempts increment.
	drained, err = store.Drain(ctx)
	require.NoError(t, err)
	require.NoError(t, store.MarkFailed(
		ctx, drained[0].ID, "timeout",
	))

	listed, err = store.List(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, listed[0].Attempts)
	require.Equal(t, "timeout", listed[0].LastError)
}

// TestQueueStore_MaxPending verifies that the queue enforces its maximum
// pending limit.
func TestQueueStore_MaxPending(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	// Override the config to a small max for testing.
	store.cfg.MaxPending = 3

	// Enqueue up to the limit.
	for i := 0; i < 3; i++ {
		require.NoError(t, store.Enqueue(ctx, makeOp(OpSend)))
	}

	// The next enqueue should fail with ErrQueueFull.
	err := store.Enqueue(ctx, makeOp(OpSend))
	require.ErrorIs(t, err, ErrQueueFull)

	// After draining, pending count drops and we can enqueue again.
	drained, err := store.Drain(ctx)
	require.NoError(t, err)
	require.Len(t, drained, 3)

	// Now pending count is 0 (all are 'delivering'), so we can enqueue.
	require.NoError(t, store.Enqueue(ctx, makeOp(OpSend)))
}

// TestQueueStore_PurgeExpired verifies that expired operations are removed
// by PurgeExpired.
func TestQueueStore_PurgeExpired(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	// Enqueue one expired and one non-expired operation.
	expired := makeOp(OpSend)
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)
	require.NoError(t, store.Enqueue(ctx, expired))

	active := makeOp(OpPublish)
	active.ExpiresAt = time.Now().Add(24 * time.Hour)
	require.NoError(t, store.Enqueue(ctx, active))

	// Purge should remove the expired one.
	purged, err := store.PurgeExpired(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), purged)

	// Only the active operation should remain.
	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, active.IdempotencyKey, listed[0].IdempotencyKey)
}

// TestQueueStore_Stats verifies that queue stats accurately reflect the
// current state.
func TestQueueStore_Stats(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	// Start with empty stats.
	stats, err := store.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.PendingCount)
	require.Nil(t, stats.OldestPending)

	// Enqueue three operations.
	for i := 0; i < 3; i++ {
		require.NoError(t, store.Enqueue(ctx, makeOp(OpSend)))
	}

	stats, err = store.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.PendingCount)
	require.Equal(t, int64(0), stats.DeliveredCount)
	require.NotNil(t, stats.OldestPending)

	// Drain and deliver one.
	drained, err := store.Drain(ctx)
	require.NoError(t, err)
	require.NoError(t, store.MarkDelivered(ctx, drained[0].ID))

	// After drain, all three moved out of pending. One delivered.
	stats, err = store.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.PendingCount)
	require.Equal(t, int64(1), stats.DeliveredCount)
}

// TestQueueStore_Clear verifies that Clear removes all operations.
func TestQueueStore_Clear(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, store.Enqueue(ctx, makeOp(OpSend)))
	}

	require.NoError(t, store.Clear(ctx))

	count, err := store.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Empty(t, listed)
}

// TestQueueStore_ConcurrentAccess verifies that concurrent enqueue and drain
// operations don't lose data.
func TestQueueStore_ConcurrentAccess(t *testing.T) {
	store := newTestQueueStore(t)
	store.cfg.MaxPending = 1000
	ctx := context.Background()

	const numGoroutines = 10
	const opsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Enqueue concurrently.
	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				err := store.Enqueue(ctx, makeOp(OpSend))
				require.NoError(t, err)
			}
		}()
	}
	wg.Wait()

	// All operations should be enqueued.
	count, err := store.Count(ctx)
	require.NoError(t, err)
	require.Equal(
		t, int64(numGoroutines*opsPerGoroutine), count,
	)

	// Drain all at once.
	drained, err := store.Drain(ctx)
	require.NoError(t, err)
	require.Len(t, drained, numGoroutines*opsPerGoroutine)

	// Verify no duplicates by checking idempotency keys.
	seen := make(map[string]bool)
	for _, op := range drained {
		require.False(t, seen[op.IdempotencyKey],
			"duplicate idempotency key: %s", op.IdempotencyKey,
		)
		seen[op.IdempotencyKey] = true
	}
}

// TestQueueStore_IdempotencyKeyUniqueness verifies that duplicate
// idempotency keys cause an error.
func TestQueueStore_IdempotencyKeyUniqueness(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	op := makeOp(OpSend)
	require.NoError(t, store.Enqueue(ctx, op))

	// Enqueueing with the same key should fail.
	err := store.Enqueue(ctx, op)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrQueueFull)
}

// TestQueueStore_PayloadPreservation verifies that payload JSON is stored
// and retrieved without modification.
func TestQueueStore_PayloadPreservation(t *testing.T) {
	store := newTestQueueStore(t)
	ctx := context.Background()

	payload := `{"sender_name":"agent-1","recipient_names":["agent-2"],"subject":"test","body":"hello world","priority":"normal"}`
	op := makeOp(OpSend)
	op.PayloadJSON = payload
	require.NoError(t, store.Enqueue(ctx, op))

	listed, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, payload, listed[0].PayloadJSON)
}

// TestQueueStore_OpenQueueStore verifies the full OpenQueueStore path.
func TestQueueStore_OpenQueueStore(t *testing.T) {
	projectRoot := t.TempDir()
	dbPath := QueueDBPath(projectRoot)

	store, err := OpenQueueStore(dbPath, DefaultQueueConfig())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	require.NoError(t, store.Enqueue(ctx, makeOp(OpSend)))

	count, err := store.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

// --- Property-based tests using pgregory.net/rapid ---

// TestQueueStore_EnqueueDrainInvariant verifies that for any sequence of
// valid enqueue operations, drain always returns exactly the pending ops
// and no operations are lost.
func TestQueueStore_EnqueueDrainInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := newRapidQueueStore(t)
		store.cfg.MaxPending = 200
		ctx := context.Background()

		numOps := rapid.IntRange(0, 50).Draw(rt, "numOps")
		keys := make(map[string]bool)

		for i := 0; i < numOps; i++ {
			op := makeOp(OpSend)
			keys[op.IdempotencyKey] = true
			err := store.Enqueue(ctx, op)
			require.NoError(t, err)
		}

		// Drain must return exactly the number we enqueued.
		drained, err := store.Drain(ctx)
		require.NoError(t, err)
		require.Equal(t, numOps, len(drained))

		// Every drained key must be one we enqueued.
		for _, op := range drained {
			require.True(
				t, keys[op.IdempotencyKey],
				"drained unknown key: %s", op.IdempotencyKey,
			)
		}

		// Second drain must return empty.
		drained2, err := store.Drain(ctx)
		require.NoError(t, err)
		require.Empty(t, drained2)
	})
}

// TestQueueStore_FIFOOrdering verifies that operations are always drained
// in creation order (FIFO).
func TestQueueStore_FIFOOrdering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := newRapidQueueStore(t)
		store.cfg.MaxPending = 200
		ctx := context.Background()

		numOps := rapid.IntRange(1, 30).Draw(rt, "numOps")
		var keys []string

		for i := 0; i < numOps; i++ {
			op := makeOp(OpSend)
			// Use incrementing timestamps to ensure order.
			op.CreatedAt = time.Unix(int64(1000+i), 0)
			keys = append(keys, op.IdempotencyKey)
			err := store.Enqueue(ctx, op)
			require.NoError(t, err)
		}

		drained, err := store.Drain(ctx)
		require.NoError(t, err)
		require.Equal(t, numOps, len(drained))

		// Order must match insertion order.
		for i, op := range drained {
			require.Equal(
				t, keys[i], op.IdempotencyKey,
				"FIFO violation at index %d", i,
			)
		}
	})
}

// TestQueueStore_StatsConsistency verifies that stats counts always match
// the actual state of the queue after a sequence of operations.
func TestQueueStore_StatsConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := newRapidQueueStore(t)
		store.cfg.MaxPending = 200
		ctx := context.Background()

		enqueueCount := rapid.IntRange(0, 20).Draw(rt, "enqueue")
		drainFirst := rapid.Bool().Draw(rt, "drainFirst")
		deliverCount := 0

		// Enqueue operations.
		for i := 0; i < enqueueCount; i++ {
			err := store.Enqueue(ctx, makeOp(OpSend))
			require.NoError(t, err)
		}

		if drainFirst && enqueueCount > 0 {
			drained, err := store.Drain(ctx)
			require.NoError(t, err)

			// Deliver some of them.
			deliverCount = rapid.IntRange(
				0, len(drained),
			).Draw(rt, "deliver")
			for i := 0; i < deliverCount; i++ {
				err := store.MarkDelivered(ctx, drained[i].ID)
				require.NoError(t, err)
			}
		}

		stats, err := store.Stats(ctx)
		require.NoError(t, err)

		if drainFirst {
			// All enqueued were drained, so pending = 0.
			require.Equal(t, int64(0), stats.PendingCount)
			require.Equal(
				t, int64(deliverCount), stats.DeliveredCount,
			)
		} else {
			require.Equal(
				t, int64(enqueueCount), stats.PendingCount,
			)
			require.Equal(t, int64(0), stats.DeliveredCount)
		}
	})
}

// TestQueueStore_FailRetryInvariant verifies that failed operations cycle
// back to pending with monotonically increasing attempt counts.
func TestQueueStore_FailRetryInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := newRapidQueueStore(t)
		ctx := context.Background()

		op := makeOp(OpSend)
		err := store.Enqueue(ctx, op)
		require.NoError(t, err)

		numRetries := rapid.IntRange(1, 10).Draw(rt, "retries")

		for i := 0; i < numRetries; i++ {
			drained, err := store.Drain(ctx)
			require.NoError(t, err)
			require.Len(t, drained, 1)
			require.Equal(t, i, drained[0].Attempts)

			errMsg := fmt.Sprintf("error-%d", i)
			err = store.MarkFailed(ctx, drained[0].ID, errMsg)
			require.NoError(t, err)
		}

		// After all retries, the op should be pending with the
		// cumulative attempt count.
		listed, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, numRetries, listed[0].Attempts)
		require.Equal(
			t, fmt.Sprintf("error-%d", numRetries-1),
			listed[0].LastError,
		)
	})
}
