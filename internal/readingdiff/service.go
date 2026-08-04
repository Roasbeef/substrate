package readingdiff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// Cache stores compiled abridgements so a patch is only ever abridged once per
// model and rubric. It mirrors the subset of the application's storage layer
// this package needs, which keeps the package testable without a database.
type Cache interface {
	// Load returns a cached entry, reporting whether one was found.
	Load(ctx context.Context, key string) (*Result, bool)

	// Store records an entry. A failure to persist is not fatal to the
	// caller, so implementations may log and return an error that callers
	// treat as advisory.
	Store(ctx context.Context, key string, model, rubric string,
		res *Result) error
}

// Service produces reading diffs, reusing cached results and collapsing
// duplicate concurrent requests for the same patch.
type Service struct {
	gen    Generator
	cache  Cache
	model  string
	rubric string

	// inflight deduplicates concurrent work. Abridging costs minutes and
	// real money, so two viewers opening the same diff at once must not
	// launch two agents.
	mu       sync.Mutex
	inflight map[string]*inflightCall
}

// inflightCall is a single in-progress abridgement other callers can await.
type inflightCall struct {
	done chan struct{}
	res  *Result
	err  error
}

// ServiceConfig configures a Service.
type ServiceConfig struct {
	// Generator produces plans. Required.
	Generator Generator

	// Cache persists compiled results. Optional; without it every request
	// recomputes.
	Cache Cache

	// Model names the model the generator runs, recorded for provenance and
	// mixed into the cache key.
	Model string

	// RubricHash identifies the generation protocol. Mixing it into the key
	// means editing the rubric invalidates stored results rather than
	// serving output the current code would not produce.
	RubricHash string
}

// NewService builds a Service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Generator == nil {
		return nil, fmt.Errorf("readingdiff: generator is required")
	}

	return &Service{
		gen:      cfg.Generator,
		cache:    cfg.Cache,
		model:    cfg.Model,
		rubric:   cfg.RubricHash,
		inflight: make(map[string]*inflightCall),
	}, nil
}

// CacheKey returns the content hash identifying an abridgement of raw under
// the given model and rubric.
//
// Every input that shapes the answer is covered, so a hit is always a result
// the current code would reproduce. The NUL separators keep the fields
// unambiguous: without them, a model name ending in a digit could combine with
// a rubric hash to collide with a different pairing.
func CacheKey(raw, model, rubric string) string {
	h := sha256.New()
	h.Write([]byte(rubric))
	h.Write([]byte{0})
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(raw))

	return hex.EncodeToString(h.Sum(nil))
}

// Get returns the reading diff for a patch, computing it only on a cache miss.
//
// Concurrent callers asking for the same patch share one computation. Without
// that, opening the same diff in two browser tabs would spawn two agents and
// bill for both.
func (s *Service) Get(ctx context.Context, req Request) (*Result, error) {
	key := CacheKey(req.UnifiedDiff, s.model, s.rubric)

	if s.cache != nil {
		if res, ok := s.cache.Load(ctx, key); ok {
			return res, nil
		}
	}

	s.mu.Lock()
	if call, ok := s.inflight[key]; ok {
		s.mu.Unlock()

		select {
		case <-call.done:
			return call.res, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &inflightCall{done: make(chan struct{})}
	s.inflight[key] = call
	s.mu.Unlock()

	call.res, call.err = Abridge(ctx, s.gen, req)

	// Publish the result before removing the entry, so a late joiner either
	// finds the completed call or misses it and recomputes, never observing
	// a half-populated one.
	close(call.done)

	s.mu.Lock()
	delete(s.inflight, key)
	s.mu.Unlock()

	if call.err != nil {
		return nil, call.err
	}

	if s.cache != nil {
		if err := s.cache.Store(
			ctx, key, s.model, s.rubric, call.res,
		); err != nil {
			// A cache write failure costs a recomputation later; it must not
			// fail a request whose answer is already in hand.
			return call.res, nil
		}
	}

	return call.res, nil
}
