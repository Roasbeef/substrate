package readingdiff

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// MaxDiffBytes bounds the diff a single abridgement will accept. The numbered
// diff is sent to the generator in full, so a larger input would exhaust the
// model's context window partway through and fail with a confusing provider
// error instead of an actionable one.
const MaxDiffBytes = 400 << 10

// defaultAttempts is how many times Abridge will ask a generator to correct a
// plan the compiler rejected. Two corrections is enough to recover from a
// miscounted coordinate without letting a confused generator loop.
const defaultAttempts = 3

// Request describes one abridgement.
type Request struct {
	// UnifiedDiff is the diff to abridge. Required.
	UnifiedDiff string

	// RepoRoot, when set, is the directory a generator may read to judge
	// whether a row is load-bearing. An empty value means the generator must
	// work from the diff text alone.
	RepoRoot string

	// Attempts overrides defaultAttempts when positive.
	Attempts int

	// Progress, when set, receives short status lines as the run proceeds. It
	// must not block.
	Progress func(msg string)
}

// Generator turns a diff into an edit plan. It is an interface so the
// compiler can be tested without a model, and so a caller can substitute a
// different backend.
type Generator interface {
	// Generate returns a plan for the request. Feedback carries the
	// compiler's rejection message from a previous attempt and is empty on
	// the first, letting a generator correct coordinates rather than start
	// over.
	Generate(ctx context.Context, req Request, feedback string) (Plan, error)
}

// Abridge produces a reading diff by asking a generator for a plan and
// compiling it.
//
// A rejected plan is not a failure. The compiler's error names the offending
// coordinate, so it is fed back and the generator gets another attempt; this
// is the loop that lets strict validation coexist with a fallible generator.
// If every attempt is rejected, Abridge falls back to the empty plan, which
// still strips imports and prunes empty sections. Returning a slightly noisy
// reading diff beats returning nothing.
func Abridge(ctx context.Context, gen Generator,
	req Request) (*Result, error) {

	if gen == nil {
		return nil, fmt.Errorf("readingdiff: nil generator")
	}
	if strings.TrimSpace(req.UnifiedDiff) == "" {
		return &Result{Summary: "No changes."}, nil
	}
	if len(req.UnifiedDiff) > MaxDiffBytes {
		return nil, fmt.Errorf(
			"readingdiff: diff is %dKB, over the %dKB limit; abridge a "+
				"narrower range such as a single commit",
			len(req.UnifiedDiff)>>10, MaxDiffBytes>>10)
	}

	attempts := req.Attempts
	if attempts <= 0 {
		attempts = defaultAttempts
	}

	progress := req.Progress
	if progress == nil {
		progress = func(string) {}
	}

	var feedback string
	for attempt := 1; attempt <= attempts; attempt++ {
		progress(fmt.Sprintf("planning (attempt %d/%d)", attempt, attempts))

		plan, err := gen.Generate(ctx, req, feedback)
		if err != nil {
			return nil, fmt.Errorf("readingdiff: generate: %w", err)
		}

		res, err := Compile(req.UnifiedDiff, plan)
		if err == nil {
			progress(fmt.Sprintf("compiled on attempt %d: %s",
				attempt, res.ElisionLine()))

			return res, nil
		}

		progress(fmt.Sprintf(
			"attempt %d rejected by the compiler: %v", attempt, err))
		feedback = fmt.Sprintf(
			"Your previous plan was rejected by the compiler:\n\n%v\n\n"+
				"Coordinates address the ORIGINAL numbered diff and never "+
				"shift. Submit a corrected complete plan.", err)

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Every attempt was rejected. The mandatory passes still add value, so
	// return an import-stripped diff rather than an error.
	progress("falling back to the identity plan")

	res, err := Compile(req.UnifiedDiff, Plan{
		Remove:  []Range{},
		Fold:    []Range{},
		Replace: []Replacement{},
		Summary: "Automatic abridgement unavailable; imports removed only.",
	})
	if err != nil {
		return nil, fmt.Errorf(
			"readingdiff: identity plan failed to compile: %w", err)
	}

	return res, nil
}

// Numbered renders a diff with a display-only, 1-based line-number gutter for
// a generator to address. The gutter is never part of a line's source text and
// never appears in a compiled result.
func Numbered(raw string) string {
	lines := splitLines(raw)
	if len(lines) == 0 {
		return ""
	}

	width := len(strconv.Itoa(len(lines)))

	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%*d|%s\n", width, i+1, l.text)
	}

	return b.String()
}
