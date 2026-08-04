package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/roasbeef/subtrate/internal/readingdiff"
)

// stubGenerator returns a fixed plan without contacting a model, so the HTTP
// surface, the service, the cache, and the compiler can all be exercised
// deterministically.
type stubGenerator struct {
	calls atomic.Int32
	plan  readingdiff.Plan
}

// Generate returns the configured plan.
func (s *stubGenerator) Generate(_ context.Context, _ readingdiff.Request,
	_ string) (readingdiff.Plan, error) {

	s.calls.Add(1)

	return s.plan, nil
}

// readingDiffFixture is a small Go patch with an import swap, a behavioral
// change, and a noisy error message, so the response exercises import
// stripping, folding, and partial elision at once.
const readingDiffFixture = `diff --git a/token.go b/token.go
index 1111111..2222222 100644
--- a/token.go
+++ b/token.go
@@ -1,9 +1,13 @@
 package token

 import (
-	"math/rand"
+	"crypto/rand"
+	"encoding/hex"
 )

 func New(n int) string {
 	b := make([]byte, n)
-	return ""
+	if _, err := rand.Read(b); err != nil {
+		panic("token: entropy source failed: " + err.Error())
+	}
+	return hex.EncodeToString(b)
 }
`

// withStubReadingDiffs installs a reading-diff service backed by a stub
// generator on the harness's server, returning the generator so a test can
// assert how often it ran.
func withStubReadingDiffs(t *testing.T, h *gatewayTestHarness,
	plan readingdiff.Plan) *stubGenerator {

	t.Helper()

	gen := &stubGenerator{plan: plan}
	svc, err := readingdiff.NewService(readingdiff.ServiceConfig{
		Generator:  gen,
		Cache:      readingdiff.NewStoreCache(h.storage),
		Model:      "stub-model",
		RubricHash: "stubrubric",
	})
	require.NoError(t, err)

	h.webServer.readingDiffs = svc

	return gen
}

// emptyReadingPlan is a plan with no model edits, which still triggers the
// mandatory import removal.
func emptyReadingPlan() readingdiff.Plan {
	return readingdiff.Plan{
		Remove:  []readingdiff.Range{},
		Fold:    []readingdiff.Range{},
		Replace: []readingdiff.Replacement{},
		Summary: "Generate tokens from a cryptographic source.",
	}
}

// TestReadingDiffEndpoint drives the whole chain over HTTP and asserts the
// abridgement actually abridges: imports vanish, behavior survives, and the
// response carries the segment map the viewer needs to expand elided regions.
func TestReadingDiffEndpoint(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	gen := withStubReadingDiffs(t, h, emptyReadingPlan())

	status, body, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"),
		map[string]any{"patch": readingDiffFixture},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp ReadingDiffResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	// Import churn is gone on both sides, so a package swap cannot read as a
	// one-sided deletion.
	require.NotContains(t, resp.ReadingDiff, `"math/rand"`)
	require.NotContains(t, resp.ReadingDiff, `"crypto/rand"`)
	require.NotContains(t, resp.ReadingDiff, `"encoding/hex"`)

	// The behavior that motivated the import change survives.
	require.Contains(t, resp.ReadingDiff, "rand.Read(b)")
	require.Contains(t, resp.ReadingDiff, "hex.EncodeToString(b)")

	require.Equal(t, "Generate tokens from a cryptographic source.",
		resp.Summary)
	require.Contains(t, resp.Elision, "changed lines")
	require.NotEmpty(t, resp.Segments)
	require.Greater(t, resp.Stats.RawChanged, resp.Stats.VisibleChanged,
		"the abridgement must hide something")
	require.Equal(t, int32(1), gen.calls.Load())
}

// TestReadingDiffEndpointCaches asserts a repeated request is served from the
// database cache rather than regenerating, which is what makes the feature
// affordable to open twice.
func TestReadingDiffEndpointCaches(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	gen := withStubReadingDiffs(t, h, emptyReadingPlan())
	payload := map[string]any{"patch": readingDiffFixture}

	status, first, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"), payload,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	status, second, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"), payload,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	require.Equal(t, int32(1), gen.calls.Load(),
		"the second request must be served from cache")

	var a, b ReadingDiffResponse
	require.NoError(t, json.Unmarshal(first, &a))
	require.NoError(t, json.Unmarshal(second, &b))
	require.Equal(t, a.ReadingDiff, b.ReadingDiff)
	require.Equal(t, a.Segments, b.Segments)
	require.Equal(t, a.Stats, b.Stats)
}

// TestReadingDiffEndpointAppliesFolds asserts a fold reaches the rendered
// output as a single machine-generated ellipsis row, preserving the marker and
// indentation of the code it replaced.
func TestReadingDiffEndpointAppliesFolds(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// Fold the three added rows of the panic branch.
	start := readingDiffLineOf(t, `+	if _, err := rand.Read(b); err != nil {`)
	plan := emptyReadingPlan()
	plan.Fold = []readingdiff.Range{{StartLine: start, EndLine: start + 2}}

	withStubReadingDiffs(t, h, plan)

	status, body, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"),
		map[string]any{"patch": readingDiffFixture},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var resp ReadingDiffResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	require.Contains(t, resp.ReadingDiff, "+\t...")
	require.NotContains(t, resp.ReadingDiff, "entropy source failed")
	require.Contains(t, resp.ReadingDiff, "hex.EncodeToString(b)")

	var folded bool
	for _, seg := range resp.Segments {
		if seg.Kind == readingdiff.SegFolded {
			folded = true
		}
	}
	require.True(t, folded, "a folded segment must be reported")
}

// TestReadingDiffEndpointRejectsBadRequests covers the input guards.
func TestReadingDiffEndpointRejectsBadRequests(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	withStubReadingDiffs(t, h, emptyReadingPlan())

	status, _, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"), map[string]any{"patch": "  "},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status)

	status, _, err = h.httpGet(h.apiURL("/api/v1/reading-diff"))
	require.NoError(t, err)
	require.Equal(t, http.StatusMethodNotAllowed, status)
}

// TestReadingDiffEndpointDisabled asserts a daemon started without a generator
// says so plainly instead of failing obscurely at request time.
func TestReadingDiffEndpointDisabled(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	h.webServer.readingDiffs = nil

	status, _, err := h.httpPost(
		h.apiURL("/api/v1/reading-diff"),
		map[string]any{"patch": readingDiffFixture},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, status)
}

// readingDiffLineOf returns the 1-based line number of a row in the fixture,
// so fold coordinates track fixture edits instead of drifting silently.
func readingDiffLineOf(t *testing.T, want string) int {
	t.Helper()

	line := 1
	for _, row := range splitFixtureLines(readingDiffFixture) {
		if row == want {
			return line
		}
		line++
	}
	t.Fatalf("fixture has no line %q", want)

	return 0
}

// splitFixtureLines splits the fixture into physical lines without a trailing
// phantom entry.
func splitFixtureLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}

	return out
}
