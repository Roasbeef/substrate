package readingdiff

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGenerator replays a scripted sequence of plans, recording the feedback
// it was handed on each call so tests can assert the retry loop passes the
// compiler's rejection back.
type fakeGenerator struct {
	plans    []Plan
	err      error
	calls    int
	feedback []string
}

// Generate returns the next scripted plan, repeating the last one once the
// script is exhausted.
func (f *fakeGenerator) Generate(_ context.Context, _ Request,
	feedback string) (Plan, error) {

	f.calls++
	f.feedback = append(f.feedback, feedback)

	if f.err != nil {
		return Plan{}, f.err
	}
	if len(f.plans) == 0 {
		return emptyPlan(), nil
	}
	if f.calls-1 < len(f.plans) {
		return f.plans[f.calls-1], nil
	}

	return f.plans[len(f.plans)-1], nil
}

// TestAbridgeAcceptsFirstValidPlan asserts the happy path costs exactly one
// generator call.
func TestAbridgeAcceptsFirstValidPlan(t *testing.T) {
	t.Parallel()

	gen := &fakeGenerator{plans: []Plan{emptyPlan()}}

	res, err := Abridge(context.Background(), gen, Request{
		UnifiedDiff: goDiff,
	})
	require.NoError(t, err)
	require.Equal(t, 1, gen.calls)
	require.NotEmpty(t, res.ReadingDiff)
	require.Equal(t, "", gen.feedback[0])
}

// TestAbridgeRetriesWithCompilerFeedback asserts a rejected plan is not fatal:
// the compiler's complaint is handed back, and a corrected plan is accepted.
// This is what lets validation stay strict without making a single miscounted
// coordinate lose the whole abridgement.
func TestAbridgeRetriesWithCompilerFeedback(t *testing.T) {
	t.Parallel()

	bad := emptyPlan()
	bad.Remove = []Range{{StartLine: 1, EndLine: 99999}}

	good := emptyPlan()
	good.Summary = "corrected"

	gen := &fakeGenerator{plans: []Plan{bad, good}}

	res, err := Abridge(context.Background(), gen, Request{
		UnifiedDiff: goDiff,
	})
	require.NoError(t, err)
	require.Equal(t, 2, gen.calls)
	require.Equal(t, "corrected", res.Summary)

	require.Empty(t, gen.feedback[0])
	require.Contains(t, gen.feedback[1], "past the end")
	require.Contains(t, gen.feedback[1], "never shift")
}

// TestAbridgeFallsBackToIdentityPlan asserts that exhausting every attempt
// still yields a usable result. Import stripping alone is worth returning, and
// a reader is better served by a slightly noisy diff than by an error.
func TestAbridgeFallsBackToIdentityPlan(t *testing.T) {
	t.Parallel()

	bad := emptyPlan()
	bad.Fold = []Range{{StartLine: 1, EndLine: 2}}

	gen := &fakeGenerator{plans: []Plan{bad}}

	res, err := Abridge(context.Background(), gen, Request{
		UnifiedDiff: goDiff,
		Attempts:    2,
	})
	require.NoError(t, err)
	require.Equal(t, 2, gen.calls)
	require.Contains(t, res.Summary, "imports removed only")

	// The fallback is still a real abridgement: imports are gone.
	require.NotContains(t, res.ReadingDiff, `"math/rand"`)
	require.Contains(t, res.ReadingDiff, "rand.Read(b)")
}

// TestAbridgeSurfacesGeneratorErrors asserts a transport failure is reported
// rather than silently degraded, since it means the model was never consulted.
func TestAbridgeSurfacesGeneratorErrors(t *testing.T) {
	t.Parallel()

	gen := &fakeGenerator{err: errors.New("no credentials")}

	_, err := Abridge(context.Background(), gen, Request{
		UnifiedDiff: goDiff,
	})
	require.ErrorContains(t, err, "no credentials")
}

// TestAbridgeRejectsOversizedDiff asserts the size guard fires before any
// generator call, so a huge diff produces actionable advice instead of an
// opaque context-window error minutes later.
func TestAbridgeRejectsOversizedDiff(t *testing.T) {
	t.Parallel()

	gen := &fakeGenerator{}
	huge := strings.Repeat("+ padding line\n", (MaxDiffBytes/15)+64)

	_, err := Abridge(context.Background(), gen, Request{UnifiedDiff: huge})
	require.ErrorContains(t, err, "over the")
	require.Equal(t, 0, gen.calls)
}

// TestAbridgeEmptyDiff asserts an empty diff short-circuits without consulting
// a generator.
func TestAbridgeEmptyDiff(t *testing.T) {
	t.Parallel()

	gen := &fakeGenerator{}

	res, err := Abridge(context.Background(), gen, Request{
		UnifiedDiff: "   \n",
	})
	require.NoError(t, err)
	require.Equal(t, "No changes.", res.Summary)
	require.Equal(t, 0, gen.calls)
}

// TestAbridgeHonoursCancellation asserts a cancelled context stops the retry
// loop rather than burning every remaining attempt.
func TestAbridgeHonoursCancellation(t *testing.T) {
	t.Parallel()

	bad := emptyPlan()
	bad.Remove = []Range{{StartLine: 1, EndLine: 99999}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gen := &fakeGenerator{plans: []Plan{bad}}

	_, err := Abridge(ctx, gen, Request{
		UnifiedDiff: goDiff,
		Attempts:    5,
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, gen.calls)
}

// TestAbridgeNilGenerator asserts the guard against a caller wiring nothing up.
func TestAbridgeNilGenerator(t *testing.T) {
	t.Parallel()

	_, err := Abridge(context.Background(), nil, Request{
		UnifiedDiff: goDiff,
	})
	require.ErrorContains(t, err, "nil generator")
}

// TestNumberedGutterIsDisplayOnly asserts the gutter is a stable, aligned
// prefix and that the source text after the pipe is untouched. A generator
// addresses rows by these numbers, so an off-by-one here would corrupt every
// plan it produces.
func TestNumberedGutterIsDisplayOnly(t *testing.T) {
	t.Parallel()

	numbered := Numbered(goDiff)
	got := strings.Split(strings.TrimSuffix(numbered, "\n"), "\n")
	want := splitLines(goDiff)
	require.Len(t, got, len(want))

	width := len(got[0][:strings.Index(got[0], "|")])
	for i, row := range got {
		idx := strings.Index(row, "|")
		require.Equal(t, width, idx, "gutter width must be uniform")
		require.Equal(t, want[i].text, row[idx+1:])
	}
}

// TestNumberedEmpty asserts an empty diff numbers to an empty string rather
// than a lone gutter.
func TestNumberedEmpty(t *testing.T) {
	t.Parallel()

	require.Empty(t, Numbered(""))
}
