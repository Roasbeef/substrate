package readingdiff

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/roasbeef/subtrate/internal/store"
)

// This file is the only part of the package that knows about the application's
// storage layer. Keeping the dependency here lets the compiler, the plan
// protocol, and the service stay testable with an in-memory cache, while the
// daemon gets a persistent one without an adapter type in main.

// storeCache persists compiled abridgements in the application store.
type storeCache struct {
	store ReadingDiffStorage
}

// ReadingDiffStorage is the slice of the application store this package needs.
// Declaring it as an interface rather than taking store.Storage keeps the
// dependency narrow and lets tests substitute a stub.
type ReadingDiffStorage interface {
	GetReadingDiff(ctx context.Context, cacheKey string) (
		store.ReadingDiffRecord, error)

	SaveReadingDiff(ctx context.Context, params store.SaveReadingDiffParams) (
		store.ReadingDiffRecord, error)
}

// NewStoreCache adapts the application store into a Cache.
func NewStoreCache(st ReadingDiffStorage) Cache {
	return &storeCache{store: st}
}

// Load reads a cached abridgement.
//
// A missing row and a corrupt one are both reported as a miss. A cache is an
// optimization, so an entry written by an older encoding should cost one
// recomputation rather than break the request.
func (c *storeCache) Load(ctx context.Context, key string) (*Result, bool) {
	rec, err := c.store.GetReadingDiff(ctx, key)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			// A genuine storage fault is still treated as a miss so the
			// request can proceed, but it is worth distinguishing here if
			// this ever needs logging.
			return nil, false
		}

		return nil, false
	}

	var segments []Segment
	if err := json.Unmarshal([]byte(rec.Segments), &segments); err != nil {
		return nil, false
	}

	return &Result{
		ReadingDiff: rec.ReadingDiff,
		Summary:     rec.Summary,
		Segments:    segments,
		Stats: Stats{
			RawChanged:     rec.RawChanged,
			VisibleChanged: rec.VisibleChanged,
			RawFiles:       rec.RawFiles,
			VisibleFiles:   rec.VisibleFiles,
		},
	}, true
}

// Store persists a compiled abridgement.
func (c *storeCache) Store(ctx context.Context, key, model, rubric string,
	res *Result) error {

	segments, err := json.Marshal(res.Segments)
	if err != nil {
		return fmt.Errorf("encode segments: %w", err)
	}

	_, err = c.store.SaveReadingDiff(ctx, store.SaveReadingDiffParams{
		CacheKey:       key,
		ReadingDiff:    res.ReadingDiff,
		Summary:        res.Summary,
		Segments:       string(segments),
		RawChanged:     res.Stats.RawChanged,
		VisibleChanged: res.Stats.VisibleChanged,
		RawFiles:       res.Stats.RawFiles,
		VisibleFiles:   res.Stats.VisibleFiles,
		Model:          model,
		RubricHash:     rubric,
	})

	return err
}
