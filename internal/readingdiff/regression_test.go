package readingdiff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// structDiff has two hunks in one file plus a second file, so a plan can try to
// hide a structural row while keeping rows that depend on it.
const structDiff = `diff --git a/a.go b/a.go
index 111..222 100644
--- a/a.go
+++ b/a.go
@@ -1,4 +1,4 @@
 ctxone
 ctxtwo
-oldA
+newA
@@ -20,4 +20,4 @@
 ctxthree
 ctxfour
-oldB
+newB
diff --git a/b.go b/b.go
index 333..444 100644
--- a/b.go
+++ b/b.go
@@ -1,2 +1,2 @@
-oldC
+newC
`

// TestPrunedHunkClearsFoldState is a regression test for a segment map that
// promised a fold the renderer never emitted.
//
// Folding context rows and then removing the hunk's changed rows leaves the
// hunk with nothing to show, so it is pruned and no ellipsis row is rendered.
// The prune used to clear only foldAt, leaving folded set, so buildSegments
// still reported a folded segment. The viewer walks segments in step with the
// rendered rows and would consume the next row for that phantom fold, shifting
// every later block onto the wrong lines — real code under the wrong heading,
// with nothing anywhere reporting an error.
func TestPrunedHunkClearsFoldState(t *testing.T) {
	t.Parallel()

	foldStart := lineOf(t, structDiff, " ctxone")
	delLine := lineOf(t, structDiff, "-oldA")

	plan := emptyPlan()
	plan.Fold = []Range{{StartLine: foldStart, EndLine: foldStart + 1}}
	plan.Remove = []Range{{StartLine: delLine, EndLine: delLine + 1}}

	res, err := Compile(structDiff, plan)
	require.NoError(t, err)

	var folded int
	for _, seg := range res.Segments {
		if seg.Kind == SegFolded {
			folded++
		}
	}
	require.Equal(t, res.Stats.FoldCount, folded,
		"every folded segment must correspond to an emitted ellipsis row")
	require.Equal(t, 0, res.Stats.FoldCount)
	require.NotContains(t, res.ReadingDiff, "...")
}

// TestRejectsOrphaningAHunk asserts a plan cannot hide a hunk header while
// keeping its rows, which would render them as a continuation of the hunk above
// and place the change at the wrong line.
func TestRejectsOrphaningAHunk(t *testing.T) {
	t.Parallel()

	header := lineOf(t, structDiff, "@@ -20,4 +20,4 @@")

	plan := emptyPlan()
	plan.Remove = []Range{{StartLine: header, EndLine: header}}

	_, err := Compile(structDiff, plan)
	require.ErrorContains(t, err, "hides the header of the hunk")
}

// TestRejectsOrphaningAFile asserts a plan cannot hide a file's header block
// while keeping its hunks, which would attribute that file's changes to the
// preceding file.
func TestRejectsOrphaningAFile(t *testing.T) {
	t.Parallel()

	head := lineOf(t, structDiff, "diff --git a/b.go b/b.go")

	plan := emptyPlan()
	plan.Remove = []Range{{StartLine: head, EndLine: head + 3}}

	_, err := Compile(structDiff, plan)
	require.ErrorContains(t, err, "hides the header of the file")
}

// TestWholeFileRemovalStaysLegal asserts the check above does not break the
// case the rubric actively asks for: dropping a generated file entirely,
// header block included.
func TestWholeFileRemovalStaysLegal(t *testing.T) {
	t.Parallel()

	head := lineOf(t, structDiff, "diff --git a/b.go b/b.go")
	total := len(splitLines(structDiff))

	plan := emptyPlan()
	plan.Remove = []Range{{StartLine: head, EndLine: total}}

	res, err := Compile(structDiff, plan)
	require.NoError(t, err)
	require.NotContains(t, res.ReadingDiff, "b.go")
	require.NotContains(t, res.ReadingDiff, "newC")
	require.Contains(t, res.ReadingDiff, "newA")
}

// TestMetaOnlyFileSurvives asserts a rename or binary change is not deleted by
// the emptiness prune.
//
// Those sections carry their whole meaning in metadata and have no hunks, so
// requiring a visible hunk header removed them from every reading diff — even
// under the empty plan, where no generator chose to hide anything.
func TestMetaOnlyFileSurvives(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
diff --git a/bin.dat b/bin.dat
index 777..888 100644
Binary files a/bin.dat and b/bin.dat differ
diff --git a/c.go b/c.go
index 555..666 100644
--- a/c.go
+++ b/c.go
@@ -1,1 +1,1 @@
-x
+y
`

	res, err := Compile(raw, emptyPlan())
	require.NoError(t, err)

	require.Contains(t, res.ReadingDiff, "rename to new.go")
	require.Contains(t, res.ReadingDiff, "Binary files")
	require.Contains(t, res.ReadingDiff, "+y")
}

// TestSegmentsMatchRenderedRows is the property that would have caught the
// pruned-fold desync, asserted directly rather than through the viewer.
//
// Walking the segment list must consume exactly the rows the renderer emitted:
// a kept run contributes its own length, a folded run exactly one ellipsis row,
// and a removed run nothing at all. Any disagreement silently shifts the
// viewer's expansion mapping.
func TestSegmentsMatchRenderedRows(t *testing.T) {
	t.Parallel()

	foldStart := lineOf(t, structDiff, " ctxthree")

	plan := emptyPlan()
	plan.Fold = []Range{{StartLine: foldStart, EndLine: foldStart + 1}}
	plan.Remove = []Range{{
		StartLine: lineOf(t, structDiff, "-oldA"),
		EndLine:   lineOf(t, structDiff, "-oldA"),
	}}

	res, err := Compile(structDiff, plan)
	require.NoError(t, err)

	out := splitLines(res.ReadingDiff)
	cursor := 0
	for _, seg := range res.Segments {
		switch seg.Kind {
		case SegKept:
			cursor += seg.LineCount()
		case SegFolded:
			require.Less(t, cursor, len(out),
				"folded segment %v has no row to consume", seg)
			require.True(t,
				strings.TrimSpace(out[cursor].text) == "..." ||
					strings.HasSuffix(out[cursor].text, "..."),
				"folded segment %v should map to an ellipsis row, got %q",
				seg, out[cursor].text)
			cursor++
		case SegRemoved:
		}
	}
	require.Equal(t, len(out), cursor,
		"segment walk consumed %d rows but %d were rendered",
		cursor, len(out))
}
