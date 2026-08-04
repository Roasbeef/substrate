package readingdiff

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Abridging on demand is the wrong shape. A reader who opens a diff message
// waits minutes for a model to read a patch that arrived long before they
// looked at it, and the wait buys nothing that could not have happened while
// they were doing something else.
//
// The Warmer closes that gap. Diffs arrive by mail, so their arrival is already
// an event the daemon sees; the Warmer scans for diff messages with no cached
// abridgement and computes them ahead of time. By the time anyone opens one,
// the request is a cache hit.
//
// Scanning rather than hooking the send path is deliberate. A warmer that only
// reacted to live sends would silently skip everything that arrived while the
// daemon was down or busy, and would need the mail service to know about
// reading diffs. Polling recent messages is self-healing and keeps the
// dependency pointing one way.

// DefaultWarmInterval is how often the Warmer looks for new work.
const DefaultWarmInterval = 90 * time.Second

// DefaultWarmMaxBytes caps the patch size the Warmer will abridge unasked.
//
// Cost and latency scale with the patch, and a warmer is spending tokens
// nobody explicitly asked for, so it stays well below the size at which a
// reader would be prompted to opt in. A large branch diff remains available on
// demand.
const DefaultWarmMaxBytes = 64 << 10

// DefaultWarmScan is how many recent messages one pass examines.
const DefaultWarmScan = 40

// MessageLister is the slice of the store the Warmer needs: recent message
// bodies, newest first.
type MessageLister interface {
	RecentDiffBodies(ctx context.Context, limit int) ([]string, error)
}

// WarmerConfig configures a Warmer.
type WarmerConfig struct {
	// Service performs the abridgement and owns the cache. Required.
	Service *Service

	// Messages supplies recent message bodies. Required.
	Messages MessageLister

	// Marker separates a message's prose from its embedded patch.
	Marker string

	// Interval, MaxBytes, and Scan default to the constants above.
	Interval time.Duration
	MaxBytes int
	Scan     int

	// Log receives progress and failures.
	Log *slog.Logger
}

// Warmer pre-computes abridgements for diffs that have arrived by mail.
type Warmer struct {
	cfg WarmerConfig

	// attempted remembers keys already tried, successfully or not, so a patch
	// the model cannot handle is not retried on every pass forever. A restart
	// clears it, which is the right amount of forgiveness: a transient failure
	// gets another chance without burning tokens in a loop.
	mu        sync.Mutex
	attempted map[string]bool
}

// NewWarmer builds a Warmer, filling in defaults.
func NewWarmer(cfg WarmerConfig) (*Warmer, error) {
	if cfg.Service == nil {
		return nil, errRequired("service")
	}
	if cfg.Messages == nil {
		return nil, errRequired("messages")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultWarmInterval
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultWarmMaxBytes
	}
	if cfg.Scan <= 0 {
		cfg.Scan = DefaultWarmScan
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	return &Warmer{
		cfg:       cfg,
		attempted: make(map[string]bool),
	}, nil
}

// errRequired reports a missing dependency.
func errRequired(field string) error {
	return &configError{field: field}
}

// configError names a missing required config field.
type configError struct{ field string }

// Error implements error.
func (e *configError) Error() string {
	return "readingdiff: warmer " + e.field + " is required"
}

// Run warms caches until the context is cancelled.
//
// The first pass runs immediately so a daemon restart picks up whatever
// arrived while it was down, rather than waiting out a full interval.
func (w *Warmer) Run(ctx context.Context) {
	w.warmOnce(ctx)

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.warmOnce(ctx)
		}
	}
}

// warmOnce scans recent messages and abridges any patch not already cached.
//
// Patches are processed one at a time on purpose. Each one occupies a model
// subprocess for tens of seconds, and a burst of diff mail should not spawn a
// dozen agents competing for the machine the operator is also using.
func (w *Warmer) warmOnce(ctx context.Context) {
	bodies, err := w.cfg.Messages.RecentDiffBodies(ctx, w.cfg.Scan)
	if err != nil {
		w.cfg.Log.WarnContext(ctx, "reading diff warmer: list messages",
			"err", err)

		return
	}

	for _, body := range bodies {
		if ctx.Err() != nil {
			return
		}

		patch := extractPatch(body, w.cfg.Marker)
		if patch == "" || len(patch) > w.cfg.MaxBytes {
			continue
		}

		key := CacheKey(patch, w.cfg.Service.model, w.cfg.Service.rubric)

		w.mu.Lock()
		seen := w.attempted[key]
		w.mu.Unlock()
		if seen {
			continue
		}

		// A cached patch needs no work, and checking first keeps a warm cache
		// from being marked attempted merely because it was seen.
		if w.cfg.Service.cache != nil {
			if _, ok := w.cfg.Service.cache.Load(ctx, key); ok {
				continue
			}
		}

		w.mu.Lock()
		w.attempted[key] = true
		w.mu.Unlock()

		start := time.Now()
		w.cfg.Log.InfoContext(ctx, "reading diff warmer: abridging",
			"bytes", len(patch), "key", key[:12])

		res, err := w.cfg.Service.Get(ctx, Request{UnifiedDiff: patch})
		if err != nil {
			w.cfg.Log.WarnContext(ctx, "reading diff warmer: abridge failed",
				"key", key[:12], "elapsed", time.Since(start).String(),
				"err", err)

			continue
		}

		w.cfg.Log.InfoContext(ctx, "reading diff warmer: cached",
			"key", key[:12], "elapsed", time.Since(start).String(),
			"elision", res.ElisionLine())
	}
}

// extractPatch pulls the unified diff out of a message body, returning empty
// when the body carries none.
func extractPatch(body, marker string) string {
	if marker == "" {
		return ""
	}

	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}

	return strings.TrimSpace(body[idx+len(marker):])
}
