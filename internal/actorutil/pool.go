package actorutil

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// Pool distributes messages across multiple actor instances using round-robin
// scheduling. This enables horizontal scaling of actor workloads by spreading
// requests across a set of worker actors.
type Pool[M actor.Message, R any] struct {
	// id is the identifier for this pool.
	id string

	// actors holds the pooled actor references for message sending.
	actors []actor.ActorRef[M, R]

	// rawActors holds the underlying Actor instances for lifecycle management.
	rawActors []*actor.Actor[M, R]

	// next is the atomic counter for round-robin selection.
	next atomic.Uint64

	// wg tracks the lifecycle of all actors in the pool.
	wg sync.WaitGroup
}

// PoolConfig holds configuration for creating a new actor pool.
type PoolConfig[M actor.Message, R any] struct {
	// ID is the identifier for the pool.
	ID string

	// Size is the number of actor instances to create.
	Size int

	// Factory creates a new actor behavior for each pool member.
	Factory func(idx int) actor.ActorBehavior[M, R]

	// MailboxSize is the buffer capacity for each actor's mailbox.
	MailboxSize int

	// DLO is the dead letter office reference for undeliverable messages.
	DLO actor.ActorRef[actor.Message, any]
}

// NewPool creates a pool with the specified number of actor instances.
// Each actor is created using the provided factory function and started
// immediately.
func NewPool[M actor.Message, R any](
	cfg PoolConfig[M, R],
) *Pool[M, R] {
	if cfg.Size <= 0 {
		cfg.Size = 1
	}
	if cfg.MailboxSize <= 0 {
		cfg.MailboxSize = 100
	}

	p := &Pool[M, R]{
		id:        cfg.ID,
		actors:    make([]actor.ActorRef[M, R], cfg.Size),
		rawActors: make([]*actor.Actor[M, R], cfg.Size),
	}

	// Create and start each actor in the pool.
	for i := 0; i < cfg.Size; i++ {
		behavior := cfg.Factory(i)
		actorCfg := actor.ActorConfig[M, R]{
			ID:          fmt.Sprintf("%s-%d", cfg.ID, i),
			Behavior:    behavior,
			MailboxSize: cfg.MailboxSize,
			DLO:         cfg.DLO,
			Wg:          &p.wg,
		}

		a := actor.NewActor(actorCfg)
		a.Start()
		p.rawActors[i] = a
		p.actors[i] = a.Ref()
	}

	return p
}

// ID returns the identifier for this pool.
func (p *Pool[M, R]) ID() string {
	return p.id
}

// Ask sends a message to the next actor in round-robin order and returns a
// Future for the response.
func (p *Pool[M, R]) Ask(ctx context.Context, msg M) actor.Future[R] {
	idx := p.next.Add(1) % uint64(len(p.actors))
	return p.actors[idx].Ask(ctx, msg)
}

// Tell sends a fire-and-forget message to the next actor in round-robin order.
func (p *Pool[M, R]) Tell(ctx context.Context, msg M) {
	idx := p.next.Add(1) % uint64(len(p.actors))
	p.actors[idx].Tell(ctx, msg)
}

// Broadcast sends a message to ALL actors in the pool. This is useful for
// cache invalidation, configuration updates, or graceful shutdown signals.
func (p *Pool[M, R]) Broadcast(ctx context.Context, msg M) {
	for _, a := range p.actors {
		a.Tell(ctx, msg)
	}
}

// BroadcastAsk sends a message to all actors and returns a slice of Futures.
// This is useful when you need responses from all actors in the pool.
func (p *Pool[M, R]) BroadcastAsk(ctx context.Context, msg M) []actor.Future[R] {
	futures := make([]actor.Future[R], len(p.actors))
	for i, a := range p.actors {
		futures[i] = a.Ask(ctx, msg)
	}
	return futures
}

// Size returns the number of actors in the pool.
func (p *Pool[M, R]) Size() int {
	return len(p.actors)
}

// Actors returns a copy of the actor references in the pool.
func (p *Pool[M, R]) Actors() []actor.ActorRef[M, R] {
	actors := make([]actor.ActorRef[M, R], len(p.actors))
	copy(actors, p.actors)
	return actors
}

// Stop gracefully stops all actors in the pool and waits for them to exit.
func (p *Pool[M, R]) Stop() {
	// Stop all actors using the underlying Actor instances.
	for _, a := range p.rawActors {
		a.Stop()
	}

	// Wait for all actors to exit.
	p.wg.Wait()
}

// PoolRef wraps a Pool to implement the ActorRef interface directly.
// This allows a pool to be used anywhere an ActorRef is expected.
type PoolRef[M actor.Message, R any] struct {
	pool *Pool[M, R]
}

// NewPoolRef creates an ActorRef wrapper around a pool.
func NewPoolRef[M actor.Message, R any](
	pool *Pool[M, R],
) actor.ActorRef[M, R] {
	return &PoolRef[M, R]{pool: pool}
}

// ID returns the pool's identifier.
func (pr *PoolRef[M, R]) ID() string {
	return pr.pool.ID()
}

// Tell sends a message to the pool (round-robin).
func (pr *PoolRef[M, R]) Tell(ctx context.Context, msg M) {
	pr.pool.Tell(ctx, msg)
}

// Ask sends a message to the pool (round-robin) and returns a Future.
func (pr *PoolRef[M, R]) Ask(ctx context.Context, msg M) actor.Future[R] {
	return pr.pool.Ask(ctx, msg)
}

// Ensure PoolRef implements ActorRef.
var _ actor.ActorRef[actor.Message, any] = (*PoolRef[actor.Message, any])(nil)
