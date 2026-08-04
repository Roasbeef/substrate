package readingdiff

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testMarker = "<!-- substrate:diff -->"

// fakeLister returns fixed message bodies.
type fakeLister struct {
	bodies []string
	calls  int
	err    error
}

// RecentDiffBodies returns the configured bodies.
func (f *fakeLister) RecentDiffBodies(_ context.Context, _ int) (
	[]string, error) {

	f.calls++

	return f.bodies, f.err
}

// newWarmer builds a warmer over the given bodies, returning it plus the
// generator so a test can count model calls.
func newWarmer(t *testing.T, bodies []string) (*Warmer, *countingGenerator) {
	t.Helper()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, newMemCache())

	w, err := NewWarmer(WarmerConfig{
		Service:  svc,
		Messages: &fakeLister{bodies: bodies},
		Marker:   testMarker,
	})
	require.NoError(t, err)

	return w, gen
}

// diffBody wraps a patch in a message body the way send-diff does.
func diffBody(patch string) string {
	return "Here is a change.\n\n" + testMarker + "\n" + patch
}

// TestWarmerPrecomputesDiffMail asserts a patch arriving by mail is abridged
// before anyone asks, which is the whole point: the reader should never wait
// for work that could have happened while they were elsewhere.
func TestWarmerPrecomputesDiffMail(t *testing.T) {
	t.Parallel()

	w, gen := newWarmer(t, []string{diffBody(goDiff)})

	w.warmOnce(context.Background())
	require.Equal(t, int32(1), gen.calls.Load())

	// The reader's own request must now be a cache hit.
	res, err := w.cfg.Service.Get(context.Background(), Request{
		UnifiedDiff: goDiff,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.ReadingDiff)
	require.Equal(t, int32(1), gen.calls.Load(),
		"a warmed patch must not be regenerated when opened")
}

// TestWarmerSkipsAlreadyCached asserts a warm cache costs nothing.
func TestWarmerSkipsAlreadyCached(t *testing.T) {
	t.Parallel()

	w, gen := newWarmer(t, []string{diffBody(goDiff)})

	_, err := w.cfg.Service.Get(context.Background(), Request{
		UnifiedDiff: goDiff,
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), gen.calls.Load())

	w.warmOnce(context.Background())
	require.Equal(t, int32(1), gen.calls.Load(),
		"the warmer must not recompute a cached patch")
}

// TestWarmerAttemptsEachPatchOnce asserts a patch that fails is not retried on
// every pass. Without this a patch the model cannot handle would burn tokens
// in a loop for as long as it stays in the recent-mail window.
func TestWarmerAttemptsEachPatchOnce(t *testing.T) {
	t.Parallel()

	w, gen := newWarmer(t, []string{diffBody(goDiff)})

	for i := 0; i < 4; i++ {
		w.warmOnce(context.Background())
	}

	require.Equal(t, int32(1), gen.calls.Load())
}

// TestWarmerRespectsSizeCap asserts an oversized patch is left for an explicit
// request, since the warmer spends tokens nobody asked it to spend.
func TestWarmerRespectsSizeCap(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, newMemCache())

	big := goDiff + strings.Repeat("+ padding\n", 4096)
	w, err := NewWarmer(WarmerConfig{
		Service:  svc,
		Messages: &fakeLister{bodies: []string{diffBody(big)}},
		Marker:   testMarker,
		MaxBytes: 1024,
	})
	require.NoError(t, err)

	w.warmOnce(context.Background())
	require.Equal(t, int32(0), gen.calls.Load())
}

// TestWarmerIgnoresNonDiffMail asserts ordinary messages cost nothing.
func TestWarmerIgnoresNonDiffMail(t *testing.T) {
	t.Parallel()

	w, gen := newWarmer(t, []string{
		"Just a status update, no patch here.",
		"",
	})

	w.warmOnce(context.Background())
	require.Equal(t, int32(0), gen.calls.Load())
}

// TestWarmerSurvivesListingFailure asserts a store error is logged rather than
// fatal, since the warmer is an optimization and must never take the daemon
// down with it.
func TestWarmerSurvivesListingFailure(t *testing.T) {
	t.Parallel()

	gen := &countingGenerator{}
	svc := newTestService(t, gen, newMemCache())

	w, err := NewWarmer(WarmerConfig{
		Service:  svc,
		Messages: &fakeLister{err: context.DeadlineExceeded},
		Marker:   testMarker,
	})
	require.NoError(t, err)

	require.NotPanics(t, func() { w.warmOnce(context.Background()) })
	require.Equal(t, int32(0), gen.calls.Load())
}

// TestWarmerRunStopsOnCancel asserts Run returns promptly when cancelled, so a
// shutdown is not held up by the warmer's interval.
func TestWarmerRunStopsOnCancel(t *testing.T) {
	t.Parallel()

	w, _ := newWarmer(t, []string{diffBody(goDiff)})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

// TestExtractPatch covers pulling a patch out of a message body.
func TestExtractPatch(t *testing.T) {
	t.Parallel()

	require.Equal(t, "diff --git a/x b/x",
		extractPatch("text\n"+testMarker+"\ndiff --git a/x b/x\n", testMarker))
	require.Empty(t, extractPatch("no marker here", testMarker))
	require.Empty(t, extractPatch("text"+testMarker, ""))
}

// TestNewWarmerRequiresDependencies asserts misconfiguration fails at
// construction rather than silently doing nothing forever.
func TestNewWarmerRequiresDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewWarmer(WarmerConfig{})
	require.ErrorContains(t, err, "service is required")

	gen := &countingGenerator{}
	_, err = NewWarmer(WarmerConfig{
		Service: newTestService(t, gen, nil),
	})
	require.ErrorContains(t, err, "messages is required")
}
