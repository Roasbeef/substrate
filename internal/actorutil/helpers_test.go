package actorutil

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// testMessage is a simple message type for testing.
type testMessage struct {
	actor.BaseMessage
	value int
}

func (m testMessage) MessageType() string { return "test" }

// testBehavior implements ActorBehavior for testing.
type testBehavior struct {
	delay    time.Duration
	err      error
	received *atomic.Int64
}

func newTestBehavior() *testBehavior {
	return &testBehavior{
		received: &atomic.Int64{},
	}
}

func (b *testBehavior) Receive(ctx context.Context, msg testMessage) fn.Result[int] {
	b.received.Add(1)

	if b.delay > 0 {
		select {
		case <-time.After(b.delay):
		case <-ctx.Done():
			return fn.Err[int](ctx.Err())
		}
	}

	if b.err != nil {
		return fn.Err[int](b.err)
	}

	return fn.Ok(msg.value * 2)
}

// createTestActor creates a test actor with the given behavior.
func createTestActor(id string, behavior *testBehavior) *actor.Actor[testMessage, int] {
	cfg := actor.ActorConfig[testMessage, int]{
		ID:          id,
		Behavior:    behavior,
		MailboxSize: 10,
	}
	a := actor.NewActor(cfg)
	a.Start()
	return a
}

// TestAskAwait tests the AskAwait helper function.
func TestAskAwait(t *testing.T) {
	t.Parallel()

	behavior := newTestBehavior()
	a := createTestActor("test-ask-await", behavior)
	defer a.Stop()

	ctx := context.Background()
	msg := testMessage{value: 21}

	result, err := AskAwait(ctx, a.Ref(), msg)
	if err != nil {
		t.Fatalf("AskAwait returned error: %v", err)
	}

	// The behavior doubles the value.
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	if behavior.received.Load() != 1 {
		t.Errorf("expected behavior to receive 1 message, got %d", behavior.received.Load())
	}
}

// TestAskAwait_Error tests AskAwait when the actor returns an error.
func TestAskAwait_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")
	behavior := newTestBehavior()
	behavior.err = testErr

	a := createTestActor("test-ask-await-error", behavior)
	defer a.Stop()

	ctx := context.Background()
	msg := testMessage{value: 10}

	_, err := AskAwait(ctx, a.Ref(), msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, testErr) {
		t.Errorf("expected test error, got %v", err)
	}
}

// TestAskAwait_ContextCancelled tests AskAwait with a cancelled context.
func TestAskAwait_ContextCancelled(t *testing.T) {
	t.Parallel()

	behavior := newTestBehavior()
	behavior.delay = 100 * time.Millisecond

	a := createTestActor("test-ask-await-cancelled", behavior)
	defer a.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	msg := testMessage{value: 10}

	_, err := AskAwait(ctx, a.Ref(), msg)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

// TestAskAwaitTyped tests the AskAwaitTyped helper function.
func TestAskAwaitTyped(t *testing.T) {
	t.Parallel()

	behavior := newTestBehavior()
	a := createTestActor("test-ask-await-typed", behavior)
	defer a.Stop()

	ctx := context.Background()
	msg := testMessage{value: 5}

	// int is the expected type (response is int).
	result, err := AskAwaitTyped[testMessage, int, int](ctx, a.Ref(), msg)
	if err != nil {
		t.Fatalf("AskAwaitTyped returned error: %v", err)
	}

	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}

// TestTellAll tests the TellAll helper function.
func TestTellAll(t *testing.T) {
	t.Parallel()

	const numActors = 3
	behaviors := make([]*testBehavior, numActors)
	actors := make([]*actor.Actor[testMessage, int], numActors)
	refs := make([]actor.TellOnlyRef[testMessage], numActors)

	for i := 0; i < numActors; i++ {
		behaviors[i] = newTestBehavior()
		actors[i] = createTestActor("test-tell-all-"+string(rune('a'+i)), behaviors[i])
		refs[i] = actors[i].TellRef()
		defer actors[i].Stop()
	}

	ctx := context.Background()
	msg := testMessage{value: 100}

	TellAll(ctx, refs, msg)

	// Give actors time to process.
	time.Sleep(50 * time.Millisecond)

	for i, b := range behaviors {
		if b.received.Load() != 1 {
			t.Errorf("actor %d: expected 1 received message, got %d", i, b.received.Load())
		}
	}
}

// TestParallelAsk tests the ParallelAsk helper function.
func TestParallelAsk(t *testing.T) {
	t.Parallel()

	const numActors = 3
	behaviors := make([]*testBehavior, numActors)
	actors := make([]*actor.Actor[testMessage, int], numActors)
	refs := make([]actor.ActorRef[testMessage, int], numActors)
	msgs := make([]testMessage, numActors)

	for i := 0; i < numActors; i++ {
		behaviors[i] = newTestBehavior()
		actors[i] = createTestActor("test-parallel-ask-"+string(rune('a'+i)), behaviors[i])
		refs[i] = actors[i].Ref()
		msgs[i] = testMessage{value: (i + 1) * 10}
		defer actors[i].Stop()
	}

	ctx := context.Background()
	results := ParallelAsk(ctx, refs, msgs)

	if len(results) != numActors {
		t.Fatalf("expected %d results, got %d", numActors, len(results))
	}

	for i, r := range results {
		val, err := r.Unpack()
		if err != nil {
			t.Errorf("result %d: unexpected error: %v", i, err)
			continue
		}

		expected := (i + 1) * 10 * 2 // value * 2
		if val != expected {
			t.Errorf("result %d: expected %d, got %d", i, expected, val)
		}
	}
}

// TestParallelAsk_Panic tests that ParallelAsk panics when refs and msgs have
// different lengths.
func TestParallelAsk_Panic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched slice lengths")
		}
	}()

	behavior := newTestBehavior()
	a := createTestActor("test-parallel-panic", behavior)
	defer a.Stop()

	refs := []actor.ActorRef[testMessage, int]{a.Ref()}
	msgs := []testMessage{{value: 1}, {value: 2}}

	ParallelAsk(context.Background(), refs, msgs)
}

// TestParallelAskSame tests the ParallelAskSame helper function.
func TestParallelAskSame(t *testing.T) {
	t.Parallel()

	const numActors = 3
	behaviors := make([]*testBehavior, numActors)
	actors := make([]*actor.Actor[testMessage, int], numActors)
	refs := make([]actor.ActorRef[testMessage, int], numActors)

	for i := 0; i < numActors; i++ {
		behaviors[i] = newTestBehavior()
		actors[i] = createTestActor("test-parallel-same-"+string(rune('a'+i)), behaviors[i])
		refs[i] = actors[i].Ref()
		defer actors[i].Stop()
	}

	ctx := context.Background()
	msg := testMessage{value: 50}

	results := ParallelAskSame(ctx, refs, msg)

	if len(results) != numActors {
		t.Fatalf("expected %d results, got %d", numActors, len(results))
	}

	for i, r := range results {
		val, err := r.Unpack()
		if err != nil {
			t.Errorf("result %d: unexpected error: %v", i, err)
			continue
		}

		if val != 100 { // 50 * 2
			t.Errorf("result %d: expected 100, got %d", i, val)
		}
	}
}

// TestFirstSuccess tests the FirstSuccess helper function.
func TestFirstSuccess(t *testing.T) {
	t.Parallel()

	// Create actors: first two fail, third succeeds.
	failErr := errors.New("intentional failure")

	b1 := newTestBehavior()
	b1.err = failErr
	b1.delay = 20 * time.Millisecond

	b2 := newTestBehavior()
	b2.err = failErr
	b2.delay = 20 * time.Millisecond

	b3 := newTestBehavior() // This one succeeds.
	b3.delay = 10 * time.Millisecond

	a1 := createTestActor("fail-1", b1)
	a2 := createTestActor("fail-2", b2)
	a3 := createTestActor("success", b3)
	defer a1.Stop()
	defer a2.Stop()
	defer a3.Stop()

	refs := []actor.ActorRef[testMessage, int]{
		a1.Ref(), a2.Ref(), a3.Ref(),
	}

	ctx := context.Background()
	msg := testMessage{value: 25}

	result, err := FirstSuccess(ctx, refs, msg)
	if err != nil {
		t.Fatalf("FirstSuccess returned error: %v", err)
	}

	if result != 50 { // 25 * 2
		t.Errorf("expected 50, got %d", result)
	}
}

// TestFirstSuccess_AllFail tests FirstSuccess when all actors fail.
func TestFirstSuccess_AllFail(t *testing.T) {
	t.Parallel()

	failErr := errors.New("intentional failure")

	b1 := newTestBehavior()
	b1.err = failErr
	b2 := newTestBehavior()
	b2.err = failErr

	a1 := createTestActor("fail-all-1", b1)
	a2 := createTestActor("fail-all-2", b2)
	defer a1.Stop()
	defer a2.Stop()

	refs := []actor.ActorRef[testMessage, int]{a1.Ref(), a2.Ref()}

	ctx := context.Background()
	msg := testMessage{value: 10}

	_, err := FirstSuccess(ctx, refs, msg)
	if err == nil {
		t.Fatal("expected error when all actors fail")
	}
}

// TestFirstSuccess_NoActors tests FirstSuccess with an empty actor slice.
func TestFirstSuccess_NoActors(t *testing.T) {
	t.Parallel()

	refs := []actor.ActorRef[testMessage, int]{}

	ctx := context.Background()
	msg := testMessage{value: 10}

	_, err := FirstSuccess(ctx, refs, msg)
	if err == nil {
		t.Fatal("expected error for empty actor slice")
	}
}

// TestMapResponses tests the MapResponses helper function.
func TestMapResponses(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")

	results := []fn.Result[int]{
		fn.Ok(10),
		fn.Err[int](testErr),
		fn.Ok(20),
	}

	// Double each success.
	mapped := MapResponses(results, func(v int) int { return v * 2 })

	if len(mapped) != 3 {
		t.Fatalf("expected 3 mapped results, got %d", len(mapped))
	}

	// First should be 20.
	v1, err := mapped[0].Unpack()
	if err != nil {
		t.Errorf("mapped[0] unexpected error: %v", err)
	}
	if v1 != 20 {
		t.Errorf("mapped[0] expected 20, got %d", v1)
	}

	// Second should be error.
	_, err = mapped[1].Unpack()
	if !errors.Is(err, testErr) {
		t.Errorf("mapped[1] expected test error, got %v", err)
	}

	// Third should be 40.
	v3, err := mapped[2].Unpack()
	if err != nil {
		t.Errorf("mapped[2] unexpected error: %v", err)
	}
	if v3 != 40 {
		t.Errorf("mapped[2] expected 40, got %d", v3)
	}
}

// TestCollectSuccesses tests the CollectSuccesses helper function.
func TestCollectSuccesses(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")

	results := []fn.Result[int]{
		fn.Ok(10),
		fn.Err[int](testErr),
		fn.Ok(20),
		fn.Err[int](testErr),
		fn.Ok(30),
	}

	successes := CollectSuccesses(results)

	if len(successes) != 3 {
		t.Fatalf("expected 3 successes, got %d", len(successes))
	}

	expected := []int{10, 20, 30}
	for i, v := range successes {
		if v != expected[i] {
			t.Errorf("successes[%d]: expected %d, got %d", i, expected[i], v)
		}
	}
}

// TestAllSucceeded tests the AllSucceeded helper function.
func TestAllSucceeded(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")

	tests := []struct {
		name     string
		results  []fn.Result[int]
		expected bool
	}{
		{
			name:     "all success",
			results:  []fn.Result[int]{fn.Ok(1), fn.Ok(2), fn.Ok(3)},
			expected: true,
		},
		{
			name:     "one failure",
			results:  []fn.Result[int]{fn.Ok(1), fn.Err[int](testErr), fn.Ok(3)},
			expected: false,
		},
		{
			name:     "all failures",
			results:  []fn.Result[int]{fn.Err[int](testErr), fn.Err[int](testErr)},
			expected: false,
		},
		{
			name:     "empty",
			results:  []fn.Result[int]{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AllSucceeded(tc.results)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestFirstError tests the FirstError helper function.
func TestFirstError(t *testing.T) {
	t.Parallel()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	tests := []struct {
		name     string
		results  []fn.Result[int]
		expected error
	}{
		{
			name:     "all success",
			results:  []fn.Result[int]{fn.Ok(1), fn.Ok(2)},
			expected: nil,
		},
		{
			name:     "first is error",
			results:  []fn.Result[int]{fn.Err[int](err1), fn.Ok(2)},
			expected: err1,
		},
		{
			name:     "second is error",
			results:  []fn.Result[int]{fn.Ok(1), fn.Err[int](err2)},
			expected: err2,
		},
		{
			name:     "empty",
			results:  []fn.Result[int]{},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FirstError(tc.results)
			if !errors.Is(result, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}
