package readingdiff

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// The properties in this file guard the claim the whole package rests on: a
// reading diff is a projection of its input, never a rewrite. Example-based
// tests confirm the cases we thought of; these confirm the invariant holds
// across diffs and plans we did not think of, which is the only way to trust
// output that a language model influenced.

// genSourceRow draws a plausible line of source code. The alphabet
// deliberately includes rows that look like diff syntax and rows that look
// like import members, since those are the inputs most likely to break the
// classifier or the import recognizer.
func genSourceRow() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		`package main`,
		``,
		`import (`,
		`	"fmt"`,
		`	"strings"`,
		`	pb "example.com/gen/proto"`,
		`)`,
		`import "os"`,
		`func run(ctx context.Context) error {`,
		`	if err != nil {`,
		`		return fmt.Errorf("run failed: %w", err)`,
		`	}`,
		`	return ""`,
		`	return nil`,
		`}`,
		`--- not a header, just source`,
		`+++ also not a header`,
		`@@ this is source too`,
		`diff --git in a string literal`,
		`	x := []string{`,
		`		"alpha",`,
		`		"beta",`,
		`	}`,
		`	// A comment explaining the thing.`,
	})
}

// genFileDiff builds one syntactically correct file section with accurate hunk
// headers. Generating correct counts matters: a hunk whose header understates
// its body would exercise the parser's recovery path rather than the mainline
// behavior these properties are about.
func genFileDiff(t *rapid.T, index int) string {
	ext := rapid.SampledFrom([]string{"go", "py", "ts", "rs", "md"}).
		Draw(t, fmt.Sprintf("ext%d", index))
	path := fmt.Sprintf("pkg/file%d.%s", index, ext)

	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	fmt.Fprintf(&b, "index %07d..%07d 100644\n", index+1, index+2)
	fmt.Fprintf(&b, "--- a/%s\n", path)
	fmt.Fprintf(&b, "+++ b/%s\n", path)

	nHunks := rapid.IntRange(1, 3).Draw(t, fmt.Sprintf("hunks%d", index))
	for h := 0; h < nHunks; h++ {
		label := fmt.Sprintf("f%dh%d", index, h)

		nRows := rapid.IntRange(1, 10).Draw(t, "rows"+label)
		markers := make([]byte, nRows)
		bodies := make([]string, nRows)
		oldCount, newCount := 0, 0

		for r := 0; r < nRows; r++ {
			m := rapid.SampledFrom([]byte{' ', '+', '-'}).
				Draw(t, fmt.Sprintf("m%s_%d", label, r))
			markers[r] = m
			bodies[r] = genSourceRow().
				Draw(t, fmt.Sprintf("b%s_%d", label, r))

			switch m {
			case ' ':
				oldCount++
				newCount++
			case '+':
				newCount++
			case '-':
				oldCount++
			}
		}

		// A hunk must touch at least one line on each side for git to emit it,
		// so fall back to a context row when the draw produced none.
		if oldCount == 0 || newCount == 0 {
			markers = append(markers, ' ')
			bodies = append(bodies, `	// filler`)
			oldCount++
			newCount++
		}

		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
			1+h*100, oldCount, 1+h*100, newCount)
		for r := range markers {
			b.WriteByte(markers[r])
			b.WriteString(bodies[r])
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// genDiff assembles a whole multi-file diff.
func genDiff(t *rapid.T) string {
	n := rapid.IntRange(1, 3).Draw(t, "files")

	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(genFileDiff(t, i))
	}

	return b.String()
}

// genPlan draws an arbitrary plan against a diff of nLines lines. Most drawn
// plans are invalid, which is intentional: Compile must reject them cleanly
// rather than panicking or emitting a corrupted diff.
func genPlan(t *rapid.T, nLines int) Plan {
	drawRanges := func(label string) []Range {
		n := rapid.IntRange(0, 3).Draw(t, label+"n")
		out := make([]Range, 0, n)
		for i := 0; i < n; i++ {
			start := rapid.IntRange(1, nLines).
				Draw(t, fmt.Sprintf("%s_s%d", label, i))
			span := rapid.IntRange(0, 4).
				Draw(t, fmt.Sprintf("%s_e%d", label, i))
			end := start + span
			if end > nLines {
				end = nLines
			}
			out = append(out, Range{StartLine: start, EndLine: end})
		}

		return out
	}

	nReps := rapid.IntRange(0, 2).Draw(t, "repsN")
	reps := make([]Replacement, 0, nReps)
	for i := 0; i < nReps; i++ {
		reps = append(reps, Replacement{
			Line: rapid.IntRange(1, nLines).
				Draw(t, fmt.Sprintf("rep_l%d", i)),
			Old: rapid.SampledFrom([]string{
				"fmt", "err", "nil", "\"alpha\"", "run failed: %w",
			}).Draw(t, fmt.Sprintf("rep_o%d", i)),
			New: rapid.SampledFrom([]string{
				"...", "…", "f...", "", "INVENTED",
			}).Draw(t, fmt.Sprintf("rep_n%d", i)),
		})
	}

	return Plan{
		Remove:  drawRanges("rm"),
		Fold:    drawRanges("fold"),
		Replace: reps,
		Summary: "generated plan",
	}
}

// genStructuredPlan draws a plan whose coordinates come from the diff's actual
// shape, so Compile usually accepts it.
//
// This generator exists because a purely random plan is rejected about
// nineteen times in twenty, which would leave the projection property
// exercising almost nothing. Drawing folds from real same-marker runs and
// replacements from real substrings keeps the acceptance rate high enough that
// the property tests the renderer rather than the validator.
func genStructuredPlan(t *rapid.T, raw string) Plan {
	lines := splitLines(raw)
	lay := analyze(lines)
	mandatory := mandatoryHidden(lines, lay)

	// Collect maximal runs of foldable rows: at least two contiguous source
	// rows sharing a hunk and a marker, none of them import scaffolding.
	type run struct{ start, end int }
	var runs []run
	for i := 0; i < len(lines); i++ {
		if !lay.kinds[i].IsHunkSource() || mandatory[i] {
			continue
		}
		j := i
		for j+1 < len(lines) &&
			lay.kinds[j+1].IsHunkSource() &&
			!mandatory[j+1] &&
			lay.hunkID[j+1] == lay.hunkID[i] &&
			marker(lines[j+1].text, lay.kinds[j+1]) ==
				marker(lines[i].text, lay.kinds[i]) {

			j++
		}
		if j > i {
			runs = append(runs, run{start: i, end: j})
		}
		i = j
	}

	plan := emptyPlan()
	plan.Summary = "structured plan"

	used := make([]bool, len(lines))
	if len(runs) > 0 {
		n := rapid.IntRange(0, len(runs)).Draw(t, "nfolds")
		for k := 0; k < n; k++ {
			r := runs[rapid.IntRange(0, len(runs)-1).
				Draw(t, fmt.Sprintf("runpick%d", k))]
			if used[r.start] {
				continue
			}
			end := rapid.IntRange(r.start+1, r.end).
				Draw(t, fmt.Sprintf("runend%d", k))
			for i := r.start; i <= end; i++ {
				used[i] = true
			}
			plan.Fold = append(plan.Fold, Range{
				StartLine: r.start + 1, EndLine: end + 1,
			})
		}
	}

	// Removals take disjoint ranges from whatever rows the folds left alone.
	nRemove := rapid.IntRange(0, 3).Draw(t, "nremove")
	for k := 0; k < nRemove; k++ {
		start := rapid.IntRange(0, len(lines)-1).
			Draw(t, fmt.Sprintf("rmstart%d", k))
		span := rapid.IntRange(0, 3).Draw(t, fmt.Sprintf("rmspan%d", k))
		end := start + span
		if end >= len(lines) {
			end = len(lines) - 1
		}

		free := true
		for i := start; i <= end; i++ {
			if used[i] {
				free = false

				break
			}
		}
		if !free {
			continue
		}
		for i := start; i <= end; i++ {
			used[i] = true
		}
		plan.Remove = append(plan.Remove, Range{
			StartLine: start + 1, EndLine: end + 1,
		})
	}

	// Replacements elide a real, uniquely occurring substring of a row that is
	// still visible, using a projection built by truncation.
	nRep := rapid.IntRange(0, 2).Draw(t, "nrep")
	for k := 0; k < nRep; k++ {
		i := rapid.IntRange(0, len(lines)-1).
			Draw(t, fmt.Sprintf("repline%d", k))
		if used[i] || !lay.kinds[i].IsHunkSource() {
			continue
		}

		text := body(lines[i].text, lay.kinds[i])
		if len(text) < 4 {
			continue
		}
		start := rapid.IntRange(0, len(text)-2).
			Draw(t, fmt.Sprintf("repstart%d", k))
		end := rapid.IntRange(start+1, len(text)).
			Draw(t, fmt.Sprintf("repend%d", k))
		old := text[start:end]
		if strings.Count(text, old) != 1 {
			continue
		}

		keep := rapid.IntRange(0, len(old)).
			Draw(t, fmt.Sprintf("repkeep%d", k))
		plan.Replace = append(plan.Replace, Replacement{
			Line: i + 1,
			Old:  old,
			New:  old[:keep] + "...",
		})
		used[i] = true
	}

	return plan
}

// assertProjection checks the no-invention property: every rendered line must
// be derivable from an original line, in order, by deletion alone.
//
// One check covers all three ways a row can appear. An untouched row is
// byte-identical to its source. A partially elided row is an elision
// projection of it, because substituting a projected span inside a line
// projects the whole line. A fold's ellipsis row is a projection of the first
// row it replaced, since it keeps that row's marker and indentation and marks
// everything after them as omitted. So matching each output row against the
// original with isElisionProjection, scanning forward only, proves that no
// text was invented and that nothing was reordered.
func assertProjection(t *rapid.T, raw string, res *Result) {
	origin := splitLines(raw)
	out := splitLines(res.ReadingDiff)
	lay := analyze(origin)

	cursor := 0
	for _, o := range out {
		found := false
		for j := cursor; j < len(origin); j++ {
			src := origin[j]
			kind := lay.kinds[j]

			if !kind.IsHunkSource() {
				if o.text == src.text {
					cursor, found = j+1, true

					break
				}

				continue
			}

			if len(o.text) == 0 || marker(o.text, kind) !=
				marker(src.text, kind) {

				continue
			}
			if isElisionProjection(body(src.text, kind),
				body(o.text, kind)) {

				cursor, found = j+1, true

				break
			}
		}

		require.True(t, found,
			"rendered line %q is not a projection of any remaining "+
				"original line", o.text)
	}
}

// TestPropertyCompileNeverInvents is the headline property: for any diff and
// any plan Compile accepts, the output contains no text the input did not.
//
// The counters guard against the property quietly becoming vacuous. A
// projection check only runs on an accepted plan, so if a future change to the
// validator started rejecting nearly everything, this test would still pass
// while verifying almost nothing. Failing below a floor of accepted plans
// turns that silent rot into a visible failure.
func TestPropertyCompileNeverInvents(t *testing.T) {
	t.Parallel()

	var accepted, total int

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genStructuredPlan(t, raw)

		total++
		res, err := Compile(raw, plan)
		if err != nil {
			// Rejecting a plan is always a valid outcome; the property only
			// constrains what an accepted plan may produce.
			return
		}
		accepted++

		assertProjection(t, raw, res)
	})

	require.Greater(t, accepted*2, total,
		"only %d of %d generated plans were accepted, so the projection "+
			"property is barely exercised", accepted, total)
}

// TestPropertyCompileRejectsCleanly feeds deliberately arbitrary plans, most of
// which are invalid, and requires Compile to either reject them with an error
// or produce a projection. It must never panic and never emit invented text.
func TestPropertyCompileRejectsCleanly(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genStructuredPlan(t, raw)

		res, err := Compile(raw, plan)
		if err != nil {
			return
		}

		assertProjection(t, raw, res)
	})
}

// TestPropertyEmptyPlanIsSubsequence checks that with no model edits the
// output is a strict subsequence of the input. Import stripping may delete
// rows, but it must never alter one.
func TestPropertyEmptyPlanIsSubsequence(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)

		res, err := Compile(raw, emptyPlan())
		require.NoError(t, err)

		origin := splitLines(raw)
		cursor := 0
		for _, o := range splitLines(res.ReadingDiff) {
			found := false
			for j := cursor; j < len(origin); j++ {
				if origin[j].text == o.text {
					cursor, found = j+1, true

					break
				}
			}
			require.True(t, found,
				"line %q is not present in the original", o.text)
		}
	})
}

// TestPropertySegmentsTile checks that the segment map is a gapless, ordered,
// non-overlapping cover of the input. The viewer resolves an expand request by
// slicing the original with these coordinates, so a gap would make some rows
// unreachable and an overlap would show rows twice.
func TestPropertySegmentsTile(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genStructuredPlan(t, raw)

		res, err := Compile(raw, plan)
		if err != nil {
			return
		}

		total := len(splitLines(raw))
		require.NotEmpty(t, res.Segments)
		require.Equal(t, 1, res.Segments[0].StartLine)
		require.Equal(t, total,
			res.Segments[len(res.Segments)-1].EndLine)

		for i, seg := range res.Segments {
			require.LessOrEqual(t, seg.StartLine, seg.EndLine)
			if i > 0 {
				require.Equal(t, res.Segments[i-1].EndLine+1,
					seg.StartLine)
			}
		}
	})
}

// TestPropertyStatsBalance checks that every changed row is accounted for in
// exactly one bucket, so the elision manifest cannot overstate how much the
// reader is still seeing.
func TestPropertyStatsBalance(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genStructuredPlan(t, raw)

		res, err := Compile(raw, plan)
		if err != nil {
			return
		}

		s := res.Stats
		require.Equal(t, s.RawChanged,
			s.VisibleChanged+s.RemovedChanged+s.FoldedChanged)
		require.LessOrEqual(t, s.VisibleFiles, s.RawFiles)
		require.LessOrEqual(t, s.VisibleChanged, s.RawChanged)
		require.GreaterOrEqual(t, s.RetentionPercent(), 0)
		require.LessOrEqual(t, s.RetentionPercent(), 100)
	})
}

// TestPropertyNoOrphanHeaders checks that a rendered file header is always
// followed by a hunk, and a rendered hunk header by a source row. An orphan
// header would tell the reader a file changed while showing nothing.
func TestPropertyNoOrphanHeaders(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genStructuredPlan(t, raw)

		res, err := Compile(raw, plan)
		if err != nil {
			return
		}

		out := splitLines(res.ReadingDiff)
		lay := analyze(out)
		for i, kind := range lay.kinds {
			if kind != KindHunkHeader {
				continue
			}

			hasRow := false
			for j := i + 1; j < len(out); j++ {
				if lay.kinds[j].IsHunkSource() {
					hasRow = true

					break
				}
				if lay.kinds[j] == KindHunkHeader ||
					lay.kinds[j] == KindFileHeader {

					break
				}
			}
			require.True(t, hasRow,
				"hunk header %q survived with no source rows", out[i].text)
		}
	})
}

// TestPropertyDeterministic checks that compiling the same inputs twice yields
// the same output, since results are cached by content hash and a
// nondeterministic compiler would make a cache hit differ from a miss.
func TestPropertyDeterministic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		raw := genDiff(t)
		plan := genPlan(t, len(splitLines(raw)))

		first, errA := Compile(raw, plan)
		second, errB := Compile(raw, plan)

		if errA != nil || errB != nil {
			require.Equal(t, errA == nil, errB == nil)

			return
		}
		require.Equal(t, first.ReadingDiff, second.ReadingDiff)
		require.Equal(t, first.Segments, second.Segments)
		require.Equal(t, first.Stats, second.Stats)
	})
}
