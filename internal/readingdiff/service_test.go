package readingdiff

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// countingGenerator records how many times it was consulted and can be held
// open, so tests can observe deduplication of concurrent requests.
type countingGenerator struct {
	calls   atomic.Int32
	release chan struct{}
}

// Generate blocks until released, when a release channel is configured.
func (c *countingGenerator) Generate(ctx context.Context, _ Request,
	_ string) (Plan, error) {

	c.calls.Add(1)

	if c.release != nil {
		select {
		case <-c.release:
		case <-ctx.Done():
			return Plan{}, ctx.Err()
		}
	}

	return emptyPlan(), nil
}

// memCache is an in-memory Cache for tests.
type memCache struct {
	mu      sync.Mutex
	entries map[string]*Result
	stores  int
}

// newMemCache builds an empty cache.
func newMemCache() *memCache {
	return &memCache{entries: make(map[string]*Result)}
}

// Load returns a stored entry if present.
func (m *memCache) Load(_ context.Context, key string) (*Result, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.entries[key]

	return res, ok
}

// Store records an entry.
func (m *memCache) Store(_ context.Context, key, _, _ string,
	res *Result) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries[key] = res
	m.stores++

	return nil
}

// newTestService builds a service over the given generator and cache.
func newTestService(t *testing.T, gen Generator, cache Cache) *Service {
	t.Helper()

	svc, err := NewService(ServiceConfig{
		Generator:  gen,
		Cache:      cache,
		Model:      "test-model",
		RubricHash: "deadbeef",
	})
	require.NoError(t, err)

	return svc
}

// TestServiceCachesResults asserts the generator runs once for a given patch
// and the second request is served from cache.
func TestServiceCachesResults(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	cache := newMemCache()
	svc := newTestService(t, gen, cache)

	req := Request{UnifiedDiff: goDiff}

	first, err := svc.Get(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, first.ReadingDiff)

	second, err := svc.Get(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, int32(1), gen.calls.Load(),
		"a cached patch must not be regenerated")
	require.Equal(t, first.ReadingDiff, second.ReadingDiff)
	require.Equal(t, 1, cache.stores)
}

// TestServiceCollapsesConcurrentRequests asserts that several viewers opening
// the same diff at once share one computation. Abridging costs minutes and
// real money, so a duplicate agent run is a bug with a bill attached.
func TestServiceCollapsesConcurrentRequests(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{release: make(chan struct{})}
	svc := newTestService(t, gen, newMemCache())

	const callers = 8

	var wg sync.WaitGroup
	results := make([]*Result, callers)
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			results[idx], errs[idx] = svc.Get(
				context.Background(), Request{UnifiedDiff: goDiff},
			)
		}(i)
	}

	// Give every caller time to arrive before the single generation
	// completes, so they must rendezvous on the in-flight entry.
	require.Eventually(t, func() bool {
		return gen.calls.Load() == 1
	}, time.Second, 5*time.Millisecond)

	close(gen.release)
	wg.Wait()

	require.Equal(t, int32(1), gen.calls.Load(),
		"concurrent requests for one patch must share a computation")
	for i := 0; i < callers; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, results[0].ReadingDiff, results[i].ReadingDiff)
	}
}

// TestServiceDistinguishesPatches asserts different patches do not collide in
// the cache.
func TestServiceDistinguishesPatches(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, newMemCache())

	_, err := svc.Get(context.Background(), Request{UnifiedDiff: goDiff})
	require.NoError(t, err)

	other := goDiff + "\ndiff --git a/z.go b/z.go\n"
	_, err = svc.Get(context.Background(), Request{UnifiedDiff: other})
	require.NoError(t, err)

	require.Equal(t, int32(2), gen.calls.Load())
}

// TestCacheKeyCoversEveryInput asserts the key changes when any input that
// shapes the answer changes. A key that ignored the rubric would serve
// abridgements the current prompt would never produce.
func TestCacheKeyCoversEveryInput(t *testing.T) {
	t.Parallel()

	base := CacheKey("diff", "model", "rubric")

	require.Equal(t, base, CacheKey("diff", "model", "rubric"))
	require.NotEqual(t, base, CacheKey("other", "model", "rubric"))
	require.NotEqual(t, base, CacheKey("diff", "other", "rubric"))
	require.NotEqual(t, base, CacheKey("diff", "model", "other"))

	// Field boundaries must be unambiguous: concatenating differently split
	// fields must not collide.
	require.NotEqual(t,
		CacheKey("d", "mo", "rubric"),
		CacheKey("d", "m", "orubric"),
	)
}

// TestServiceWithoutCacheRecomputes asserts a nil cache is supported and
// simply means every request recomputes.
func TestServiceWithoutCacheRecomputes(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, nil)

	req := Request{UnifiedDiff: goDiff}
	_, err := svc.Get(context.Background(), req)
	require.NoError(t, err)
	_, err = svc.Get(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, int32(2), gen.calls.Load())
}

// TestNewServiceRequiresGenerator asserts the constructor rejects an
// unconfigured service rather than failing at the first request.
func TestNewServiceRequiresGenerator(t *testing.T) {
	t.Parallel()

	_, err := NewService(ServiceConfig{})
	require.ErrorContains(t, err, "generator is required")
}

// TestServiceNormalizesWhitespace asserts the same patch spelled with
// different surrounding whitespace hits one cache entry.
//
// This is not hypothetical tidiness. The web client slices a patch out of a
// message body and trims it, while a CLI pipes the same patch through with its
// trailing newline intact. Hashing the raw bytes made those two spellings miss
// each other's cache entry and pay for the abridgement twice.
func TestServiceNormalizesWhitespace(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, newMemCache())

	variants := []string{
		goDiff,
		strings.TrimSpace(goDiff),
		"\n\n" + goDiff + "\n\n",
		goDiff + "\n",
	}

	var first string
	for i, patch := range variants {
		res, err := svc.Get(context.Background(), Request{
			UnifiedDiff: patch,
		})
		require.NoError(t, err)

		if i == 0 {
			first = res.ReadingDiff

			continue
		}
		require.Equal(t, first, res.ReadingDiff,
			"variant %d produced a different reading diff", i)
	}

	require.Equal(t, int32(1), gen.calls.Load(),
		"whitespace variants of one patch must share a cache entry")
}
