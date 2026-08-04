package readingdiff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// goDiff is a small but structurally complete Go diff: two file sections, an
// import block whose package changed, a behavioral hunk, and a test file with
// a repetitive assertion batch. It exercises import stripping, folding, and
// partial elision together.
const goDiff = `diff --git a/token.go b/token.go
index 1111111..2222222 100644
--- a/token.go
+++ b/token.go
@@ -1,11 +1,15 @@
 package token

 import (
 	"fmt"
-	"math/rand"
+	"crypto/rand"
+	"encoding/hex"
 )

 func New(n int) string {
 	b := make([]byte, n)
-	return fmt.Sprintf("%x", b)
+	if _, err := rand.Read(b); err != nil {
+		return ""
+	}
+	return hex.EncodeToString(b)
 }
diff --git a/token_test.go b/token_test.go
index 3333333..4444444 100644
--- a/token_test.go
+++ b/token_test.go
@@ -1,5 +1,8 @@
 package token

 func TestNew(t *testing.T) {
+	if got := New(4); len(got) != 8 {
+		t.Errorf("New(4) length = %d, want %d", len(got), 8)
+	}
 	_ = New(8)
 }
`

// lineOf returns the 1-based physical line number of the first line whose text
// equals want, failing the test when the fixture does not contain it. Tests use
// it so a fixture edit surfaces as a clear failure instead of a silent
// off-by-one in a hardcoded coordinate.
func lineOf(t *testing.T, diff, want string) int {
	t.Helper()

	for i, l := range strings.Split(diff, "\n") {
		if l == want {
			return i + 1
		}
	}
	t.Fatalf("fixture has no line %q", want)

	return 0
}

// emptyPlan returns a plan with no model edits, which still triggers the
// mandatory import removal.
func emptyPlan() Plan {
	return Plan{
		Remove:  []Range{},
		Fold:    []Range{},
		Replace: []Replacement{},
	}
}

// TestCompileStripsImportsWithEmptyPlan asserts the central guarantee of the
// mandatory pass: import churn disappears even when the generator asked for
// nothing, while the behavioral rows that motivated the import change stay.
func TestCompileStripsImportsWithEmptyPlan(t *testing.T) {
	t.Parallel()

	res, err := Compile(goDiff, emptyPlan())
	require.NoError(t, err)

	require.NotContains(t, res.ReadingDiff, `"math/rand"`)
	require.NotContains(t, res.ReadingDiff, `"crypto/rand"`)
	require.NotContains(t, res.ReadingDiff, `"encoding/hex"`)
	require.NotContains(t, res.ReadingDiff, "import (")

	// The body that explains the import change must survive.
	require.Contains(t, res.ReadingDiff, "rand.Read(b)")
	require.Contains(t, res.ReadingDiff, "hex.EncodeToString(b)")

	// Both file sections still have visible changes, so both headers remain.
	require.Equal(t, 2, res.Stats.RawFiles)
	require.Equal(t, 2, res.Stats.VisibleFiles)
}

// TestCompileHidesImportsOnBothSides checks that a package substitution is
// hidden symmetrically. Hiding only the added row would make a swap read as a
// pure deletion, which is exactly the misreading the per-side pass prevents.
func TestCompileHidesImportsOnBothSides(t *testing.T) {
	t.Parallel()

	res, err := Compile(goDiff, emptyPlan())
	require.NoError(t, err)

	for _, row := range []string{
		`-	"math/rand"`,
		`+	"crypto/rand"`,
		`+	"encoding/hex"`,
	} {
		require.NotContains(t, res.ReadingDiff, row)
	}
}

// TestCompileFoldEmitsFixedPlaceholder asserts a fold collapses its range into
// exactly one machine-generated row that preserves the marker and indentation
// of the code it replaces.
func TestCompileFoldEmitsFixedPlaceholder(t *testing.T) {
	t.Parallel()

	start := lineOf(t, goDiff, `+	if _, err := rand.Read(b); err != nil {`)
	plan := emptyPlan()
	plan.Fold = []Range{{StartLine: start, EndLine: start + 2}}

	res, err := Compile(goDiff, plan)
	require.NoError(t, err)

	require.Contains(t, res.ReadingDiff, "+\t...\n")
	require.NotContains(t, res.ReadingDiff, "rand.Read(b)")
	require.Equal(t, 1, res.Stats.FoldCount)
	require.Equal(t, 3, res.Stats.FoldedChanged)
}

// TestCompileReplaceElidesPartOfLine asserts a partial elision keeps the
// control flow of an error branch while dropping the message arguments.
func TestCompileReplaceElidesPartOfLine(t *testing.T) {
	t.Parallel()

	target := `+		t.Errorf("New(4) length = %d, want %d", len(got), 8)`
	n := lineOf(t, goDiff, target)

	plan := emptyPlan()
	plan.Replace = []Replacement{{
		Line: n,
		Old:  `"New(4) length = %d, want %d", len(got), 8`,
		New:  `...`,
	}}

	res, err := Compile(goDiff, plan)
	require.NoError(t, err)

	require.Contains(t, res.ReadingDiff, "t.Errorf(...)")
	require.NotContains(t, res.ReadingDiff, "want %d")
}

// TestCompilePrunesFullyElidedFile asserts that removing every changed row of
// a file drops its header and metadata too, rather than leaving an orphan
// heading that announces a change the reader cannot see.
func TestCompilePrunesFullyElidedFile(t *testing.T) {
	t.Parallel()

	start := lineOf(t, goDiff, "diff --git a/token_test.go b/token_test.go")
	total := len(splitLines(goDiff))

	plan := emptyPlan()
	plan.Remove = []Range{{StartLine: start, EndLine: total}}

	res, err := Compile(goDiff, plan)
	require.NoError(t, err)

	require.NotContains(t, res.ReadingDiff, "token_test.go")
	require.Equal(t, 2, res.Stats.RawFiles)
	require.Equal(t, 1, res.Stats.VisibleFiles)
}

// TestCompileRejectsInvalidPlans covers every validation rule that protects
// the no-invention guarantee. Each case must fail, because each one would
// otherwise let a generator put text on screen that the original did not
// contain, or hide code under a placeholder that misrepresents it.
func TestCompileRejectsInvalidPlans(t *testing.T) {
	t.Parallel()

	importLine := lineOf(t, goDiff, `-	"math/rand"`)
	bodyLine := lineOf(t, goDiff, `+	return hex.EncodeToString(b)`)
	addLine := lineOf(t, goDiff, `+	if _, err := rand.Read(b); err != nil {`)
	hunkHeader := lineOf(t, goDiff, "@@ -1,11 +1,15 @@")

	// The deletion immediately precedes an addition, so folding across the
	// two would merge opposite polarities into one row.
	delLine := lineOf(t, goDiff, `-	return fmt.Sprintf("%x", b)`)

	tests := []struct {
		name string
		plan func(Plan) Plan
		want string
	}{
		{
			name: "nil slices rejected",
			plan: func(Plan) Plan { return Plan{} },
			want: "must all be arrays",
		},
		{
			name: "remove past end of diff",
			plan: func(p Plan) Plan {
				p.Remove = []Range{{StartLine: 1, EndLine: 100000}}

				return p
			},
			want: "past the end",
		},
		{
			name: "overlapping removes",
			plan: func(p Plan) Plan {
				p.Remove = []Range{
					{StartLine: 5, EndLine: 10},
					{StartLine: 8, EndLine: 12},
				}

				return p
			},
			want: "already removed",
		},
		{
			name: "single line fold",
			plan: func(p Plan) Plan {
				p.Fold = []Range{{StartLine: addLine, EndLine: addLine}}

				return p
			},
			want: "at least two lines",
		},
		{
			name: "fold spanning mixed markers",
			plan: func(p Plan) Plan {
				p.Fold = []Range{{
					StartLine: delLine, EndLine: delLine + 1,
				}}

				return p
			},
			want: "marker",
		},
		{
			name: "fold over a hunk header",
			plan: func(p Plan) Plan {
				p.Fold = []Range{{
					StartLine: hunkHeader, EndLine: hunkHeader + 1,
				}}

				return p
			},
			want: "not a source row",
		},
		{
			name: "fold over an import row",
			plan: func(p Plan) Plan {
				p.Fold = []Range{{
					StartLine: importLine, EndLine: importLine + 1,
				}}

				return p
			},
			want: "import row",
		},
		{
			name: "replace with absent old text",
			plan: func(p Plan) Plan {
				p.Replace = []Replacement{{
					Line: bodyLine, Old: "nonexistent", New: "...",
				}}

				return p
			},
			want: "does not occur",
		},
		{
			name: "replace that invents text",
			plan: func(p Plan) Plan {
				p.Replace = []Replacement{{
					Line: bodyLine,
					Old:  "hex.EncodeToString(b)",
					New:  "base64.Encode(b)",
				}}

				return p
			},
			want: "not an elision",
		},
		{
			name: "replace that silently drops characters",
			plan: func(p Plan) Plan {
				p.Replace = []Replacement{{
					Line: bodyLine,
					Old:  "hex.EncodeToString(b)",
					New:  "hex.Encode(b)",
				}}

				return p
			},
			want: "not an elision",
		},
		{
			name: "replace on a hunk header",
			plan: func(p Plan) Plan {
				p.Replace = []Replacement{{
					Line: hunkHeader, Old: "@@", New: "...",
				}}

				return p
			},
			want: "not a source row",
		},
		{
			name: "multiline summary",
			plan: func(p Plan) Plan {
				p.Summary = "first\nsecond"

				return p
			},
			want: "single line",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := Compile(goDiff, tc.plan(emptyPlan()))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestSegmentsTileTheInput asserts the segment map is a complete, ordered,
// non-overlapping tiling of the original lines. The web viewer relies on this
// to expand an elided region: a gap or an overlap would make some original
// line unreachable or shown twice.
func TestSegmentsTileTheInput(t *testing.T) {
	t.Parallel()

	start := lineOf(t, goDiff, `+	if _, err := rand.Read(b); err != nil {`)
	plan := emptyPlan()
	plan.Fold = []Range{{StartLine: start, EndLine: start + 2}}
	plan.Remove = []Range{{StartLine: 2, EndLine: 2}}

	res, err := Compile(goDiff, plan)
	require.NoError(t, err)
	require.NotEmpty(t, res.Segments)

	total := len(splitLines(goDiff))
	require.Equal(t, 1, res.Segments[0].StartLine)
	require.Equal(t, total, res.Segments[len(res.Segments)-1].EndLine)

	for i, seg := range res.Segments {
		require.LessOrEqual(t, seg.StartLine, seg.EndLine)
		if i == 0 {
			continue
		}
		require.Equal(t, res.Segments[i-1].EndLine+1, seg.StartLine,
			"segment %d does not start where segment %d ended", i, i-1)

		// Adjacent segments normally differ in kind, but two neighbouring
		// folds stay separate on purpose: each emits its own ellipsis row.
		if res.Segments[i-1].Kind == seg.Kind {
			require.Equal(t, SegFolded, seg.Kind,
				"only folds may repeat as adjacent segments")
		}
	}
}

// TestElisionLineCountsLocally asserts the manifest reports counts derived
// from the compiled result rather than from anything a generator claimed.
func TestElisionLineCountsLocally(t *testing.T) {
	t.Parallel()

	res, err := Compile(goDiff, emptyPlan())
	require.NoError(t, err)

	require.Contains(t, res.ElisionLine(), "changed lines in 2/2 files")
	require.Equal(t, res.Stats.RawChanged,
		res.Stats.VisibleChanged+res.Stats.RemovedChanged+
			res.Stats.FoldedChanged)
}

// TestCompileEmptyDiff asserts an empty input is handled without error, since
// a caller may legitimately abridge a branch with no changes.
func TestCompileEmptyDiff(t *testing.T) {
	t.Parallel()

	res, err := Compile("", emptyPlan())
	require.NoError(t, err)
	require.Empty(t, res.ReadingDiff)
	require.Empty(t, res.ElisionLine())
}

// TestSegmentsSplitPerFold asserts each fold gets its own segment even when two
// folds are adjacent.
//
// A viewer walks the segment list in step with the rendered rows, and one fold
// emits exactly one ellipsis row. If two adjacent folds merged into a single
// folded segment, the viewer would consume one row where two were emitted and
// every block after the pair would be shifted.
func TestSegmentsSplitPerFold(t *testing.T) {
	t.Parallel()

	first := lineOf(t, goDiff, `+	if _, err := rand.Read(b); err != nil {`)

	// Two folds that touch: rows N..N+1 and N+2..N+3, all additions in one
	// hunk.
	plan := emptyPlan()
	plan.Fold = []Range{
		{StartLine: first, EndLine: first + 1},
		{StartLine: first + 2, EndLine: first + 3},
	}

	res, err := Compile(goDiff, plan)
	require.NoError(t, err)
	require.Equal(t, 2, res.Stats.FoldCount)

	var folded []Segment
	for _, seg := range res.Segments {
		if seg.Kind == SegFolded {
			folded = append(folded, seg)
		}
	}
	require.Len(t, folded, 2,
		"each fold must own a segment so one segment means one emitted row")
	require.Equal(t, first, folded[0].StartLine)
	require.Equal(t, first+2, folded[1].StartLine)

	// The rendered output really does carry one ellipsis row per fold.
	require.Equal(t, 2,
		strings.Count(res.ReadingDiff, "+\t...\n"))
}
