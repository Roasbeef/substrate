package actorutil

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// poolTestBehavior tracks which pool member handled each message.
type poolTestBehavior struct {
	idx      int
	handled  *atomic.Int64
	received []int
	mu       sync.Mutex
}

func newPoolTestBehavior(idx int) *poolTestBehavior {
	return &poolTestBehavior{
		idx:     idx,
		handled: &atomic.Int64{},
	}
}

func (b *poolTestBehavior) Receive(
	ctx context.Context, msg testMessage,
) fn.Result[int] {
	b.mu.Lock()
	b.received = append(b.received, msg.value)
	b.mu.Unlock()

	b.handled.Add(1)
	return fn.Ok(msg.value * 2)
}

func (b *poolTestBehavior) ReceivedValues() []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]int, len(b.received))
	copy(result, b.received)
	return result
}

// TestNewPool tests pool creation.
func TestNewPool(t *testing.T) {
	t.Parallel()

	behaviors := make([]*poolTestBehavior, 0)

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool",
		Size: 3,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			b := newPoolTestBehavior(idx)
			behaviors = append(behaviors, b)
			return b
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	if pool.Size() != 3 {
		t.Errorf("expected pool size 3, got %d", pool.Size())
	}

	if pool.ID() != "test-pool" {
		t.Errorf("expected pool ID 'test-pool', got '%s'", pool.ID())
	}

	actors := pool.Actors()
	if len(actors) != 3 {
		t.Errorf("expected 3 actors, got %d", len(actors))
	}
}

// TestPool_Ask tests round-robin message distribution with Ask.
func TestPool_Ask(t *testing.T) {
	t.Parallel()

	const poolSize = 3
	const numMessages = 9

	behaviors := make([]*poolTestBehavior, 0)

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-ask",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			b := newPoolTestBehavior(idx)
			behaviors = append(behaviors, b)
			return b
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	ctx := context.Background()

	// Send messages and collect results.
	for i := 0; i < numMessages; i++ {
		msg := testMessage{value: i + 1}
		future := pool.Ask(ctx, msg)
		result := future.Await(ctx)

		val, err := result.Unpack()
		if err != nil {
			t.Errorf("message %d: unexpected error: %v", i, err)
			continue
		}

		expected := (i + 1) * 2
		if val != expected {
			t.Errorf("message %d: expected %d, got %d", i, expected, val)
		}
	}

	// Give time for all messages to be processed.
	time.Sleep(50 * time.Millisecond)

	// Verify round-robin distribution: each actor should have handled 3
	// messages.
	for i, b := range behaviors {
		if b.handled.Load() != 3 {
			t.Errorf("behavior %d: expected 3 messages, handled %d",
				i, b.handled.Load())
		}
	}
}

// TestPool_Tell tests round-robin message distribution with Tell.
func TestPool_Tell(t *testing.T) {
	t.Parallel()

	const poolSize = 3
	const numMessages = 6

	behaviors := make([]*poolTestBehavior, 0)

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-tell",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			b := newPoolTestBehavior(idx)
			behaviors = append(behaviors, b)
			return b
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	ctx := context.Background()

	// Send messages using Tell.
	for i := 0; i < numMessages; i++ {
		msg := testMessage{value: i + 1}
		pool.Tell(ctx, msg)
	}

	// Give time for messages to be processed.
	time.Sleep(100 * time.Millisecond)

	// Verify each actor handled messages.
	totalHandled := int64(0)
	for i, b := range behaviors {
		handled := b.handled.Load()
		totalHandled += handled
		if handled != 2 {
			t.Errorf("behavior %d: expected 2 messages, handled %d", i, handled)
		}
	}

	if totalHandled != numMessages {
		t.Errorf("expected %d total messages, got %d", numMessages, totalHandled)
	}
}

// TestPool_Broadcast tests broadcasting messages to all pool actors.
func TestPool_Broadcast(t *testing.T) {
	t.Parallel()

	const poolSize = 4

	behaviors := make([]*poolTestBehavior, 0)

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-broadcast",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			b := newPoolTestBehavior(idx)
			behaviors = append(behaviors, b)
			return b
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	ctx := context.Background()
	msg := testMessage{value: 42}

	pool.Broadcast(ctx, msg)

	// Give time for messages to be processed.
	time.Sleep(100 * time.Millisecond)

	// Every actor should have received the message.
	for i, b := range behaviors {
		if b.handled.Load() != 1 {
			t.Errorf("behavior %d: expected 1 message, handled %d",
				i, b.handled.Load())
		}

		values := b.ReceivedValues()
		if len(values) != 1 || values[0] != 42 {
			t.Errorf("behavior %d: expected value 42, got %v", i, values)
		}
	}
}

// TestPool_BroadcastAsk tests broadcasting with Ask to all pool actors.
func TestPool_BroadcastAsk(t *testing.T) {
	t.Parallel()

	const poolSize = 3

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-broadcast-ask",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			return newPoolTestBehavior(idx)
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	ctx := context.Background()
	msg := testMessage{value: 5}

	futures := pool.BroadcastAsk(ctx, msg)

	if len(futures) != poolSize {
		t.Fatalf("expected %d futures, got %d", poolSize, len(futures))
	}

	// Wait for all responses.
	for i, f := range futures {
		result := f.Await(ctx)
		val, err := result.Unpack()
		if err != nil {
			t.Errorf("future %d: unexpected error: %v", i, err)
			continue
		}
		if val != 10 { // 5 * 2
			t.Errorf("future %d: expected 10, got %d", i, val)
		}
	}
}

// TestPool_DefaultSize tests that pool defaults to size 1 if not specified.
func TestPool_DefaultSize(t *testing.T) {
	t.Parallel()

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-default",
		Size: 0, // Should default to 1.
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			return newPoolTestBehavior(idx)
		},
	})
	defer pool.Stop()

	if pool.Size() != 1 {
		t.Errorf("expected default pool size 1, got %d", pool.Size())
	}
}

// TestPool_Stop tests graceful pool shutdown.
func TestPool_Stop(t *testing.T) {
	t.Parallel()

	const poolSize = 3

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-stop",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			return newPoolTestBehavior(idx)
		},
		MailboxSize: 10,
	})

	// Send some messages.
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		pool.Tell(ctx, testMessage{value: i})
	}

	// Give time for processing.
	time.Sleep(50 * time.Millisecond)

	// Stop should complete without hanging.
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("pool.Stop() timed out")
	}
}

// TestPoolRef tests the PoolRef wrapper.
func TestPoolRef(t *testing.T) {
	t.Parallel()

	behaviors := make([]*poolTestBehavior, 0)

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-poolref",
		Size: 2,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			b := newPoolTestBehavior(idx)
			behaviors = append(behaviors, b)
			return b
		},
		MailboxSize: 10,
	})
	defer pool.Stop()

	// Create PoolRef wrapper.
	ref := NewPoolRef(pool)

	if ref.ID() != "test-poolref" {
		t.Errorf("expected ID 'test-poolref', got '%s'", ref.ID())
	}

	ctx := context.Background()

	// Test Tell through PoolRef.
	ref.Tell(ctx, testMessage{value: 1})

	// Test Ask through PoolRef.
	future := ref.Ask(ctx, testMessage{value: 2})
	result := future.Await(ctx)

	val, err := result.Unpack()
	if err != nil {
		t.Fatalf("PoolRef.Ask returned error: %v", err)
	}

	if val != 4 { // 2 * 2
		t.Errorf("expected 4, got %d", val)
	}

	// Give time for Tell to process.
	time.Sleep(50 * time.Millisecond)

	// Verify both behaviors received messages.
	totalHandled := int64(0)
	for _, b := range behaviors {
		totalHandled += b.handled.Load()
	}

	if totalHandled != 2 {
		t.Errorf("expected 2 total messages, got %d", totalHandled)
	}
}

// TestPoolRef_ImplementsActorRef verifies PoolRef implements ActorRef.
func TestPoolRef_ImplementsActorRef(t *testing.T) {
	t.Parallel()

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-interface",
		Size: 1,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			return newPoolTestBehavior(idx)
		},
	})
	defer pool.Stop()

	// This should compile if PoolRef implements ActorRef.
	ref := NewPoolRef(pool)
	_ = ref
}

// TestPool_ConcurrentAccess tests that the pool is safe for concurrent use.
func TestPool_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	const poolSize = 4
	const numGoroutines = 10
	const messagesPerGoroutine = 100

	pool := NewPool(PoolConfig[testMessage, int]{
		ID:   "test-pool-concurrent",
		Size: poolSize,
		Factory: func(idx int) actor.ActorBehavior[testMessage, int] {
			return newPoolTestBehavior(idx)
		},
		MailboxSize: 200,
	})
	defer pool.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < messagesPerGoroutine; i++ {
				msg := testMessage{value: goroutineID*1000 + i}

				if i%2 == 0 {
					pool.Tell(ctx, msg)
				} else {
					future := pool.Ask(ctx, msg)
					result := future.Await(ctx)
					_, err := result.Unpack()
					if err != nil {
						t.Errorf("goroutine %d message %d: error: %v",
							goroutineID, i, err)
					}
				}
			}
		}(g)
	}

	wg.Wait()

	// Give time for remaining Tell messages to process.
	time.Sleep(100 * time.Millisecond)
}
