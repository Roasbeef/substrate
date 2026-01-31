// Package actorutil provides utility functions for working with the actor
// system from darepo-client/baselib/actor.
package actorutil

import (
	"context"
	"fmt"

	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/lightningnetwork/lnd/fn/v2"
)

// AskAwait is a convenience function that sends an Ask message to an actor
// and blocks until the response is available. It unpacks the Result and
// returns the response or error directly.
func AskAwait[M actor.Message, R any](
	ctx context.Context,
	ref actor.ActorRef[M, R],
	msg M,
) (R, error) {

	future := ref.Ask(ctx, msg)
	result := future.Await(ctx)
	return result.Unpack()
}

// AskAwaitTyped is like AskAwait but with an additional type assertion on the
// response. This is useful when the actor response is a union type and you
// need a specific concrete type.
func AskAwaitTyped[M actor.Message, R any, T any](
	ctx context.Context,
	ref actor.ActorRef[M, R],
	msg M,
) (T, error) {

	resp, err := AskAwait(ctx, ref, msg)
	if err != nil {
		var zero T
		return zero, err
	}

	typed, ok := any(resp).(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf(
			"unexpected response type: got %T, want %T",
			resp, zero,
		)
	}

	return typed, nil
}

// TellAll sends a message to all actors in the provided slice using
// fire-and-forget semantics. This is useful for broadcasting messages to
// multiple actors simultaneously.
func TellAll[M actor.Message](
	ctx context.Context,
	refs []actor.TellOnlyRef[M],
	msg M,
) {

	for _, ref := range refs {
		ref.Tell(ctx, msg)
	}
}

// ParallelAsk sends messages to multiple actors concurrently and collects
// all results. The refs and msgs slices must have the same length. Results
// are returned in the same order as the input refs.
func ParallelAsk[M actor.Message, R any](
	ctx context.Context,
	refs []actor.ActorRef[M, R],
	msgs []M,
) []fn.Result[R] {

	if len(refs) != len(msgs) {
		panic("refs and msgs must have same length")
	}

	// Send all Ask requests concurrently.
	futures := make([]actor.Future[R], len(refs))
	for i, ref := range refs {
		futures[i] = ref.Ask(ctx, msgs[i])
	}

	// Await all results.
	results := make([]fn.Result[R], len(futures))
	for i, f := range futures {
		results[i] = f.Await(ctx)
	}

	return results
}

// ParallelAskSame sends the same message to multiple actors concurrently and
// collects all results. Results are returned in the same order as the input
// refs.
func ParallelAskSame[M actor.Message, R any](
	ctx context.Context,
	refs []actor.ActorRef[M, R],
	msg M,
) []fn.Result[R] {

	// Send all Ask requests concurrently.
	futures := make([]actor.Future[R], len(refs))
	for i, ref := range refs {
		futures[i] = ref.Ask(ctx, msg)
	}

	// Await all results.
	results := make([]fn.Result[R], len(futures))
	for i, f := range futures {
		results[i] = f.Await(ctx)
	}

	return results
}

// FirstSuccess sends the same message to multiple actors concurrently and
// returns the first successful response. If all actors return errors, the
// last error is returned.
func FirstSuccess[M actor.Message, R any](
	ctx context.Context,
	refs []actor.ActorRef[M, R],
	msg M,
) (R, error) {

	if len(refs) == 0 {
		var zero R
		return zero, fmt.Errorf("no actors provided")
	}

	// Create a channel to receive results.
	type resultWithIndex struct {
		result fn.Result[R]
		idx    int
	}
	resultCh := make(chan resultWithIndex, len(refs))

	// Create a cancellable context for early termination.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Send all Ask requests concurrently.
	for i, ref := range refs {
		go func(idx int, r actor.ActorRef[M, R]) {
			future := r.Ask(ctx, msg)
			result := future.Await(ctx)
			select {
			case resultCh <- resultWithIndex{result: result, idx: idx}:
			case <-ctx.Done():
			}
		}(i, ref)
	}

	// Wait for first success or all failures.
	var lastErr error
	receivedCount := 0
	for receivedCount < len(refs) {
		select {
		case res := <-resultCh:
			receivedCount++
			val, err := res.result.Unpack()
			if err == nil {
				// Cancel remaining requests.
				cancel()
				return val, nil
			}
			lastErr = err

		case <-ctx.Done():
			var zero R
			return zero, ctx.Err()
		}
	}

	var zero R
	return zero, lastErr
}

// MapResponses transforms a slice of results using the provided function.
// Any error results are passed through unchanged.
func MapResponses[R any, T any](
	results []fn.Result[R],
	mapFn func(R) T,
) []fn.Result[T] {

	mapped := make([]fn.Result[T], len(results))
	for i, r := range results {
		val, err := r.Unpack()
		if err != nil {
			mapped[i] = fn.Err[T](err)
		} else {
			mapped[i] = fn.Ok(mapFn(val))
		}
	}
	return mapped
}

// CollectSuccesses filters a slice of results and returns only the successful
// values, discarding any errors.
func CollectSuccesses[R any](results []fn.Result[R]) []R {
	var successes []R
	for _, r := range results {
		val, err := r.Unpack()
		if err == nil {
			successes = append(successes, val)
		}
	}
	return successes
}

// AllSucceeded returns true if all results in the slice are successful.
func AllSucceeded[R any](results []fn.Result[R]) bool {
	for _, r := range results {
		if _, err := r.Unpack(); err != nil {
			return false
		}
	}
	return true
}

// FirstError returns the first error from a slice of results, or nil if all
// succeeded.
func FirstError[R any](results []fn.Result[R]) error {
	for _, r := range results {
		if _, err := r.Unpack(); err != nil {
			return err
		}
	}
	return nil
}
