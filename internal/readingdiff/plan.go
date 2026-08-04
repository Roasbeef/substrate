package readingdiff

import (
	"fmt"
	"strings"
)

// maxReplacementBytes bounds one replacement's text so a malformed plan cannot
// balloon the rendered output.
const maxReplacementBytes = 4 << 10

// maxSummaryBytes bounds the one-line summary a generator may attach.
const maxSummaryBytes = 500

// Range is an inclusive, 1-based range of physical lines in the original
// diff. Coordinates always address the original and never shift as earlier
// rows are hidden, so a plan is absolute rather than incremental.
type Range struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

// Replacement elides part of a single source row. Old is matched against the
// row's text after its +, -, or space marker and must occur there exactly
// once. New must be an elision projection of Old: characters may only be
// removed, and every removed span must be visibly marked with an ellipsis.
type Replacement struct {
	Line int    `json:"line"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// Plan is a complete set of source-anchored edits against one diff. Every
// field is required, using an empty slice rather than null, so a generator
// cannot accidentally omit one and have it silently treated as "no edits".
type Plan struct {
	// Remove lists ranges to omit from the output entirely.
	Remove []Range `json:"remove"`

	// Fold lists ranges of two or more contiguous same-marker source rows to
	// replace with one machine-generated ellipsis row.
	Fold []Range `json:"fold"`

	// Replace lists partial elisions of individual source rows.
	Replace []Replacement `json:"replace"`

	// Summary is a one-line description of what the change does. It is the
	// only prose a generator contributes, and it never enters the diff body.
	Summary string `json:"summary"`
}

// ellipsisRunes are the placeholder forms a replacement may use to mark an
// omitted span. Both the ASCII and typographic forms are accepted because
// models emit either.
const (
	asciiEllipsis = "..."
	unicodeEllips = "…"
)

// foldPlaceholder is the exact text a fold emits after the marker and the
// preserved indentation. It is fixed so no generator-authored prose can reach
// the rendered diff.
const foldPlaceholder = "..."

// Validate checks a plan's internal consistency without reference to any
// particular diff. Coordinate-level checks that need the diff live in Compile;
// this catches the malformed shapes early and with better messages.
func (p Plan) Validate() error {
	if p.Remove == nil || p.Fold == nil || p.Replace == nil {
		return fmt.Errorf("remove, fold, and replace must all be arrays; " +
			"use [] rather than null when empty")
	}
	if len(p.Summary) > maxSummaryBytes {
		return fmt.Errorf("summary is %d bytes, over the %d byte limit",
			len(p.Summary), maxSummaryBytes)
	}
	if strings.ContainsAny(p.Summary, "\n\r") {
		return fmt.Errorf("summary must be a single line")
	}

	for i, r := range p.Remove {
		if err := r.validate(); err != nil {
			return fmt.Errorf("remove[%d]: %w", i, err)
		}
	}
	for i, r := range p.Fold {
		if err := r.validate(); err != nil {
			return fmt.Errorf("fold[%d]: %w", i, err)
		}
		if r.StartLine == r.EndLine {
			return fmt.Errorf("fold[%d]: a fold must cover at least two "+
				"lines; use remove for a single row", i)
		}
	}
	for i, rep := range p.Replace {
		if rep.Line < 1 {
			return fmt.Errorf("replace[%d]: line %d is not 1-based",
				i, rep.Line)
		}
		if rep.Old == "" {
			return fmt.Errorf("replace[%d]: old must not be empty", i)
		}
		if len(rep.New) > maxReplacementBytes {
			return fmt.Errorf("replace[%d]: new is %d bytes, over the %d "+
				"byte limit", i, len(rep.New), maxReplacementBytes)
		}
		if strings.ContainsAny(rep.Old+rep.New, "\n\r") {
			return fmt.Errorf("replace[%d]: old and new must be single "+
				"line spans", i)
		}
	}

	return nil
}

// validate checks that a range is well formed in isolation.
func (r Range) validate() error {
	if r.StartLine < 1 || r.EndLine < 1 {
		return fmt.Errorf("invalid 1-based range %d-%d",
			r.StartLine, r.EndLine)
	}
	if r.StartLine > r.EndLine {
		return fmt.Errorf("start line %d is after end line %d",
			r.StartLine, r.EndLine)
	}

	return nil
}

// isElisionProjection reports whether new can be produced from old purely by
// deleting spans and marking each deletion with an ellipsis.
//
// This is the rule that keeps a partial elision honest. Splitting new on its
// ellipsis placeholders yields the literal fragments the generator claims to
// have kept; those fragments must occur in old, in order, without overlapping.
// A fragment sequence that does not anchor to the start or end of old means
// characters vanished without a placeholder, so the projection is rejected.
//
// The check deliberately allows an ellipsis to stand for an empty span. That
// costs nothing: marking a deletion that removed nothing is honest, merely
// redundant, and rejecting it would fail plans that bracket a kept fragment.
func isElisionProjection(old, new string) bool {
	if old == new {
		return true
	}

	// Normalize the typographic ellipsis so one splitter handles both forms.
	normalized := strings.ReplaceAll(new, unicodeEllips, asciiEllipsis)
	if !strings.Contains(normalized, asciiEllipsis) {
		// Without a placeholder, the only honest projection is the identity,
		// which the equality check above already accepted.
		return false
	}

	fragments := strings.Split(normalized, asciiEllipsis)

	// A leading fragment must match the head of old, and a trailing fragment
	// must match its tail. An empty fragment at either end means the
	// replacement opened or closed with an ellipsis, which imposes no anchor.
	if head := fragments[0]; head != "" {
		if !strings.HasPrefix(old, head) {
			return false
		}
	}
	if tail := fragments[len(fragments)-1]; tail != "" {
		if !strings.HasSuffix(old, tail) {
			return false
		}
	}

	// Walk the interior fragments forward through old. Consuming greedily from
	// the current cursor enforces both ordering and non-overlap.
	cursor := 0
	for i, frag := range fragments {
		if frag == "" {
			continue
		}

		// The final fragment was already anchored to the tail; verifying it
		// again from the cursor would wrongly reject a repeated substring.
		if i == len(fragments)-1 {
			if len(old)-len(frag) < cursor {
				return false
			}

			break
		}

		idx := strings.Index(old[cursor:], frag)
		if idx < 0 {
			return false
		}
		cursor += idx + len(frag)
	}

	return true
}

// applyReplacements rewrites a row body by substituting each replacement's old
// span with its new text. Replacements are applied left to right against the
// original body so their offsets, computed during validation, stay accurate.
func applyReplacements(text string, reps []Replacement) string {
	var b strings.Builder
	cursor := 0

	for _, rep := range reps {
		idx := strings.Index(text[cursor:], rep.Old)
		if idx < 0 {
			continue
		}
		start := cursor + idx
		b.WriteString(text[cursor:start])
		b.WriteString(rep.New)
		cursor = start + len(rep.Old)
	}
	b.WriteString(text[cursor:])

	return b.String()
}
