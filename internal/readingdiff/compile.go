package readingdiff

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SegmentKind describes what happened to a run of original lines.
type SegmentKind string

const (
	// SegKept means the run appears in the reading diff. A kept run may still
	// contain partially elided rows.
	SegKept SegmentKind = "kept"

	// SegRemoved means the run was dropped with no placeholder. The reader
	// sees nothing at all where these lines were.
	SegRemoved SegmentKind = "removed"

	// SegFolded means the run collapsed into a single ellipsis row, so the
	// reader can see that omitted code exists.
	SegFolded SegmentKind = "folded"
)

// Segment maps a run of original physical lines onto its fate in the reading
// diff. The ordered segment list is what lets a viewer offer to expand an
// elided region in place: the original text is still addressable, because a
// reading diff is a projection of the input rather than a rewrite of it.
type Segment struct {
	Kind      SegmentKind `json:"kind"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
}

// LineCount returns how many original lines the segment covers.
func (s Segment) LineCount() int {
	return s.EndLine - s.StartLine + 1
}

// Stats records how much of the input survived, counted locally rather than
// taken from any generator's claim.
type Stats struct {
	// RawChanged is the number of added and removed rows in the input.
	RawChanged int `json:"raw_changed"`

	// VisibleChanged is how many of those rows the reader still sees.
	VisibleChanged int `json:"visible_changed"`

	// RemovedChanged is how many changed rows vanished without a placeholder.
	RemovedChanged int `json:"removed_changed"`

	// FoldedChanged is how many changed rows collapsed into ellipsis rows.
	FoldedChanged int `json:"folded_changed"`

	// FoldCount is the number of ellipsis rows emitted.
	FoldCount int `json:"fold_count"`

	// RawFiles and VisibleFiles count file sections before and after.
	RawFiles     int `json:"raw_files"`
	VisibleFiles int `json:"visible_files"`
}

// RetentionPercent returns the share of changed rows still visible, or 100
// when the input had no changed rows.
func (s Stats) RetentionPercent() int {
	if s.RawChanged == 0 {
		return 100
	}

	return s.VisibleChanged * 100 / s.RawChanged
}

// Result is a compiled reading diff together with everything a caller needs to
// render it, explain it, and expand it back to the original.
type Result struct {
	// ReadingDiff is the abridged diff. It keeps unified-diff shape so an
	// ordinary diff viewer can colour it, but it is not an applicable patch:
	// hidden rows leave the hunk headers' line counts deliberately stale.
	ReadingDiff string `json:"reading_diff"`

	// Summary is the generator's one-line description of the change.
	Summary string `json:"summary"`

	// Segments maps every original line onto its fate, in order.
	Segments []Segment `json:"segments"`

	// Stats records how much was elided.
	Stats Stats `json:"stats"`
}

// ElisionLine renders a one-line manifest of how much the reader is not
// reading, for example "kept 12/240 changed lines in 3/7 files".
func (r *Result) ElisionLine() string {
	s := r.Stats
	if s.RawChanged == 0 {
		return ""
	}
	if s.VisibleChanged == 0 {
		return fmt.Sprintf("elided all %d changed lines in %d files",
			s.RawChanged, s.RawFiles)
	}
	if s.RawFiles > 0 {
		return fmt.Sprintf("kept %d/%d changed lines in %d/%d files",
			s.VisibleChanged, s.RawChanged, s.VisibleFiles, s.RawFiles)
	}

	return fmt.Sprintf("kept %d/%d changed lines",
		s.VisibleChanged, s.RawChanged)
}

// compiledFold is a validated fold with the rendering details derived from the
// source it replaces. The generator chooses only the coordinates; the marker
// and indentation come from the input, which is why a fold cannot smuggle in
// invented text.
type compiledFold struct {
	rng    Range
	marker byte
	indent string
	eol    string
}

// planState tracks, for every physical line, whether it is hidden and which
// fold covers or is emitted at it.
type planState struct {
	hidden    []bool
	mandatory []bool
	foldAt    []int
	folded    []int
	folds     []compiledFold
	reps      map[int][]Replacement
}

// newPlanState allocates state for a diff of n physical lines.
func newPlanState(n int) *planState {
	st := &planState{
		hidden:    make([]bool, n),
		mandatory: make([]bool, n),
		foldAt:    make([]int, n),
		folded:    make([]int, n),
		reps:      make(map[int][]Replacement),
	}
	for i := 0; i < n; i++ {
		st.foldAt[i] = -1
		st.folded[i] = -1
	}

	return st
}

// represented reports whether a line still shows the reader something, either
// because it is visible or because a fold emits an ellipsis in its place.
func (st *planState) represented(i int) bool {
	return !st.hidden[i] || st.folded[i] >= 0
}

// Compile validates a plan against a diff and applies it, returning the
// reading diff along with the segment map and retention statistics.
//
// Import removal is merged in before the plan's own edits and is not optional,
// so an empty plan still yields an import-free reading diff. Every coordinate
// in the plan addresses the original input, so validation can be exhaustive
// and the result is always the input minus explicit, checked deletions.
func Compile(raw string, plan Plan) (*Result, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}

	lines := splitLines(raw)
	if len(lines) == 0 {
		return &Result{Summary: plan.Summary}, nil
	}
	lay := analyze(lines)

	st := newPlanState(len(lines))
	st.mandatory = mandatoryHidden(lines, lay)
	copy(st.hidden, st.mandatory)

	var problems []error

	// Model removals come first so folds and replacements can detect a
	// contradiction with them.
	modelRemoved := make([]bool, len(lines))
	for i, r := range plan.Remove {
		if err := checkBounds(r, len(lines)); err != nil {
			problems = append(problems,
				fmt.Errorf("remove[%d]: %w", i, err))

			continue
		}
		for n := r.StartLine; n <= r.EndLine; n++ {
			if modelRemoved[n-1] {
				problems = append(problems, fmt.Errorf(
					"remove[%d]: line %d is already removed by an "+
						"earlier range", i, n))

				break
			}
			modelRemoved[n-1] = true
			st.hidden[n-1] = true
		}
	}

	for i, r := range plan.Fold {
		fold, err := compileFold(lines, lay, st, modelRemoved, r)
		if err != nil {
			problems = append(problems, fmt.Errorf("fold[%d]: %w", i, err))

			continue
		}

		idx := len(st.folds)
		st.folds = append(st.folds, fold)
		st.foldAt[r.StartLine-1] = idx
		for n := r.StartLine; n <= r.EndLine; n++ {
			st.hidden[n-1] = true
			st.folded[n-1] = idx
		}
	}

	for i, rep := range plan.Replace {
		if err := checkReplacement(lines, lay, st, modelRemoved, rep); err != nil {
			problems = append(problems,
				fmt.Errorf("replace[%d]: %w", i, err))

			continue
		}
		st.reps[rep.Line-1] = append(st.reps[rep.Line-1], rep)
	}

	if err := checkReplacementOverlap(lines, lay, st); err != nil {
		problems = append(problems, err)
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	pruneEmptyStructure(lay, st)

	return render(lines, lay, st, plan.Summary), nil
}

// checkBounds verifies a range lies inside the diff.
func checkBounds(r Range, n int) error {
	if err := r.validate(); err != nil {
		return err
	}
	if r.EndLine > n {
		return fmt.Errorf("line %d is past the end of the diff (%d lines)",
			r.EndLine, n)
	}

	return nil
}

// compileFold validates one fold and derives the marker, indentation, and line
// ending its ellipsis row will use.
//
// A fold must cover at least two contiguous source rows that share a hunk and
// a diff marker. Requiring a single marker is what keeps a fold from merging
// an addition with a deletion into one row that would misrepresent both. A
// fold may not span a row that import stripping already hid, because the
// resulting ellipsis would ambiguously stand for both scaffolding and code.
func compileFold(lines []line, lay layout, st *planState,
	modelRemoved []bool, r Range) (compiledFold, error) {

	var zero compiledFold
	if err := checkBounds(r, len(lines)); err != nil {
		return zero, err
	}
	if r.StartLine == r.EndLine {
		return zero, fmt.Errorf("a fold must cover at least two lines")
	}

	first := r.StartLine - 1
	kind := lay.kinds[first]
	if !kind.IsHunkSource() {
		return zero, fmt.Errorf("line %d is not a source row inside a hunk",
			r.StartLine)
	}

	want := marker(lines[first].text, kind)
	hunk := lay.hunkID[first]

	for n := r.StartLine; n <= r.EndLine; n++ {
		i := n - 1
		k := lay.kinds[i]
		if !k.IsHunkSource() {
			return zero, fmt.Errorf(
				"line %d is not a source row inside a hunk", n)
		}
		if lay.hunkID[i] != hunk {
			return zero, fmt.Errorf(
				"line %d is in a different hunk than line %d",
				n, r.StartLine)
		}
		if got := marker(lines[i].text, k); got != want {
			return zero, fmt.Errorf(
				"line %d has marker %q but the fold starts with %q",
				n, string(got), string(want))
		}
		if st.mandatory[i] {
			return zero, fmt.Errorf(
				"line %d is an import row, which is already hidden; fold "+
					"only the behavioral rows", n)
		}
		if modelRemoved[i] {
			return zero, fmt.Errorf(
				"line %d is already removed by this plan", n)
		}
		if st.folded[i] >= 0 {
			return zero, fmt.Errorf(
				"line %d is already covered by an earlier fold", n)
		}
	}

	eol := lines[first].eol
	if eol == "" {
		eol = "\n"
	}

	return compiledFold{
		rng:    r,
		marker: want,
		indent: leadingWhitespace(body(lines[first].text, kind)),
		eol:    eol,
	}, nil
}

// checkReplacement validates a single partial elision against its source row.
func checkReplacement(lines []line, lay layout, st *planState,
	modelRemoved []bool, rep Replacement) error {

	if rep.Line > len(lines) {
		return fmt.Errorf("line %d is past the end of the diff (%d lines)",
			rep.Line, len(lines))
	}

	i := rep.Line - 1
	kind := lay.kinds[i]
	if !kind.IsHunkSource() {
		return fmt.Errorf("line %d is not a source row inside a hunk",
			rep.Line)
	}
	if modelRemoved[i] || st.folded[i] >= 0 {
		return fmt.Errorf(
			"line %d is already hidden by this plan, so eliding part of "+
				"it has no effect", rep.Line)
	}

	text := body(lines[i].text, kind)
	switch strings.Count(text, rep.Old) {
	case 1:
	case 0:
		return fmt.Errorf("old text %q does not occur on line %d",
			rep.Old, rep.Line)
	default:
		return fmt.Errorf(
			"old text %q occurs more than once on line %d; extend it "+
				"until it is unique", rep.Old, rep.Line)
	}

	if !isElisionProjection(rep.Old, rep.New) {
		return fmt.Errorf(
			"new text %q is not an elision of %q; characters may only be "+
				"removed, and every omitted span must be marked with ...",
			rep.New, rep.Old)
	}

	return nil
}

// checkReplacementOverlap verifies that several elisions on one row address
// disjoint spans, and sorts each row's replacements into source order so the
// renderer can apply them in one left-to-right pass.
func checkReplacementOverlap(lines []line, lay layout, st *planState) error {
	for i, reps := range st.reps {
		if len(reps) < 2 {
			continue
		}

		text := body(lines[i].text, lay.kinds[i])
		sort.Slice(reps, func(a, b int) bool {
			return strings.Index(text, reps[a].Old) <
				strings.Index(text, reps[b].Old)
		})

		prevEnd := 0
		for _, rep := range reps {
			start := strings.Index(text, rep.Old)
			if start < prevEnd {
				return fmt.Errorf(
					"replace: two elisions on line %d overlap", i+1)
			}
			prevEnd = start + len(rep.Old)
		}
		st.reps[i] = reps
	}

	return nil
}

// pruneEmptyStructure hides hunk headers and file sections that no longer
// introduce anything, so the reader never meets an orphan header announcing a
// change that was entirely elided.
func pruneEmptyStructure(lay layout, st *planState) {
	for hunkID, bounds := range lay.hunkBounds {
		if hunkVisible(lay, st, hunkID, bounds) {
			continue
		}
		for i := bounds.start; i < bounds.end && i < len(st.hidden); i++ {
			st.hidden[i] = true
			st.foldAt[i] = -1
		}
	}

	for fileID, bounds := range lay.fileBounds {
		if fileVisible(lay, st, fileID, bounds) {
			continue
		}
		for i := bounds.start; i < bounds.end && i < len(st.hidden); i++ {
			st.hidden[i] = true
			st.foldAt[i] = -1
		}
	}
}

// hunkVisible reports whether a hunk still represents at least one changed
// row, either directly or through a fold.
func hunkVisible(lay layout, st *planState, hunkID int, bounds span) bool {
	for i := bounds.start; i < bounds.end && i < len(st.hidden); i++ {
		if lay.hunkID[i] != hunkID || !lay.kinds[i].IsChange() {
			continue
		}
		if st.represented(i) {
			return true
		}
	}

	return false
}

// fileVisible reports whether a file section still contains a visible hunk.
func fileVisible(lay layout, st *planState, fileID int, bounds span) bool {
	for i := bounds.start; i < bounds.end && i < len(st.hidden); i++ {
		if lay.fileID[i] != fileID {
			continue
		}
		if lay.kinds[i] == KindHunkHeader && !st.hidden[i] {
			return true
		}
	}

	return false
}

// render walks the original lines once, emitting the surviving rows, and
// records the segment map and statistics as it goes.
func render(lines []line, lay layout, st *planState, summary string) *Result {
	var b strings.Builder
	res := &Result{Summary: summary}

	for i := range lines {
		kind := lay.kinds[i]

		if kind == KindFileHeader {
			res.Stats.RawFiles++
			if !st.hidden[i] {
				res.Stats.VisibleFiles++
			}
		}
		if kind.IsChange() {
			res.Stats.RawChanged++
			switch {
			case !st.hidden[i]:
				res.Stats.VisibleChanged++
			case st.folded[i] >= 0:
				res.Stats.FoldedChanged++
			default:
				res.Stats.RemovedChanged++
			}
		}

		if idx := st.foldAt[i]; idx >= 0 {
			fold := st.folds[idx]
			b.WriteByte(fold.marker)
			b.WriteString(fold.indent)
			b.WriteString(foldPlaceholder)
			b.WriteString(fold.eol)
			res.Stats.FoldCount++

			continue
		}
		if st.hidden[i] {
			continue
		}

		text := lines[i].text
		if reps := st.reps[i]; len(reps) > 0 {
			m := marker(text, kind)
			text = string(m) + applyReplacements(body(text, kind), reps)
		}
		b.WriteString(text)
		b.WriteString(lines[i].eol)
	}

	res.ReadingDiff = b.String()
	res.Segments = buildSegments(st, len(lines))

	return res
}

// buildSegments groups consecutive original lines that shared a fate into runs.
func buildSegments(st *planState, n int) []Segment {
	if n == 0 {
		return nil
	}

	var segs []Segment
	classify := func(i int) SegmentKind {
		switch {
		case !st.hidden[i]:
			return SegKept
		case st.folded[i] >= 0:
			return SegFolded
		default:
			return SegRemoved
		}
	}

	cur := classify(0)
	start := 0
	for i := 1; i < n; i++ {
		k := classify(i)
		if k == cur {
			continue
		}
		segs = append(segs, Segment{
			Kind:      cur,
			StartLine: start + 1,
			EndLine:   i,
		})
		cur, start = k, i
	}
	segs = append(segs, Segment{
		Kind:      cur,
		StartLine: start + 1,
		EndLine:   n,
	})

	return segs
}
