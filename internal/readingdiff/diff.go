// Package readingdiff abridges a unified diff down to the rows a reviewer
// actually needs to read.
//
// Reviewing agent-written code in diff form is mostly about understanding the
// high-level change to the program, not sweating mechanical detail: the code
// compiles and its tests pass. A reviewer does not need to read the exact
// spelling of an error message, the import block that the formatter rewrote,
// or a generated file. They need to know what changed, where the data came
// from, and where it went.
//
// The package takes a whole unified diff and an edit plan expressed purely as
// coordinates into that diff, then applies the plan to the immutable input.
// Nothing in this package ever authors diff text. A plan can only hide rows,
// collapse a run of rows into a single machine-generated ellipsis, or elide
// part of one row, and every one of those operations is validated against the
// original before it is applied. A caller that generates plans with a language
// model therefore cannot invent an identifier, a comment, or a behavior: the
// output is provably the input minus explicit deletions.
//
// The plan generator lives elsewhere. This package is deliberately free of any
// model dependency so the compiler can be tested exhaustively on its own.
package readingdiff

import (
	"regexp"
	"strconv"
	"strings"
)

// LineKind classifies one physical line of a unified diff. Classification is
// hunk-aware: inside a hunk body, a source line beginning with "--" is source
// text, not a "---" file marker, so the kind of a line depends on the parser
// state that precedes it rather than on its prefix alone.
type LineKind uint8

const (
	// KindOther is any line that carries no diff structure, such as a commit
	// message body emitted by git show.
	KindOther LineKind = iota

	// KindFileHeader is a "diff --git a/x b/y" line starting a file section.
	KindFileHeader

	// KindMeta is per-file metadata: index, mode, rename, copy, or binary
	// notices.
	KindMeta

	// KindOldFile is the "---" old-path marker outside a hunk body.
	KindOldFile

	// KindNewFile is the "+++" new-path marker outside a hunk body.
	KindNewFile

	// KindHunkHeader is an "@@ -a,b +c,d @@" hunk header.
	KindHunkHeader

	// KindContext is an unchanged source row inside a hunk.
	KindContext

	// KindAdd is an added source row inside a hunk.
	KindAdd

	// KindDelete is a removed source row inside a hunk.
	KindDelete

	// KindNoNewline is the "\ No newline at end of file" marker.
	KindNoNewline
)

// IsHunkSource reports whether the kind is a source row inside a hunk body,
// meaning it carries program text behind a +, -, or space marker. Only these
// rows may be folded or partially elided.
func (k LineKind) IsHunkSource() bool {
	return k == KindContext || k == KindAdd || k == KindDelete
}

// IsChange reports whether the kind is an added or removed source row. These
// are the rows that retention statistics count, because context rows are
// orientation rather than change.
func (k LineKind) IsChange() bool {
	return k == KindAdd || k == KindDelete
}

// Language identifies the source language of a file section, which selects
// the import-recognition rules applied to its hunks.
type Language uint8

const (
	// LangUnknown disables language-specific handling.
	LangUnknown Language = iota

	// LangGo is Go source.
	LangGo

	// LangPython is Python source.
	LangPython

	// LangJS covers JavaScript and TypeScript, including JSX and TSX.
	LangJS

	// LangRust is Rust source.
	LangRust
)

// line is one physical line of the input together with the exact ending it
// carried. Preserving the ending separately lets the renderer reproduce CRLF
// input byte for byte instead of normalizing it.
type line struct {
	text string
	eol  string
}

// layout is the parsed shape of a diff: one entry per physical line giving its
// kind, the file section and hunk it belongs to, and the language of that
// file. Indices are 0-based and parallel to the line slice.
type layout struct {
	kinds  []LineKind
	fileID []int
	hunkID []int
	lang   []Language

	// hunkBounds maps a hunk ID to the half-open physical line range it
	// spans, including its @@ header.
	hunkBounds []span

	// fileBounds maps a file ID to the half-open physical line range of its
	// whole section, including the "diff --git" header.
	fileBounds []span
}

// span is a half-open range of 0-based physical line indices.
type span struct {
	start int
	end   int
}

// splitLines splits text into physical lines while retaining each line's exact
// ending. A trailing newline belongs to the line it terminates and does not
// produce a phantom empty final line.
func splitLines(text string) []line {
	if text == "" {
		return nil
	}

	var lines []line
	for len(text) > 0 {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			lines = append(lines, line{text: text})
			break
		}

		body, eol := text[:i], "\n"
		if strings.HasSuffix(body, "\r") {
			body = strings.TrimSuffix(body, "\r")
			eol = "\r\n"
		}
		lines = append(lines, line{text: body, eol: eol})
		text = text[i+1:]
	}

	return lines
}

// hunkHeaderRE matches a unified hunk header and captures the old and new
// starting lines with their optional counts. A missing count means one, per
// the unified diff format.
var hunkHeaderRE = regexp.MustCompile(
	`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`,
)

// metaPrefixes are the per-file metadata lines git emits between a file header
// and the first hunk. They are recognized only outside a hunk body.
var metaPrefixes = []string{
	"index ", "old mode ", "new mode ", "new file mode ",
	"deleted file mode ", "similarity index ", "dissimilarity index ",
	"rename from ", "rename to ", "copy from ", "copy to ",
	"Binary files ", "GIT binary patch",
}

// analyze classifies every physical line of a diff and records the file and
// hunk each belongs to.
//
// The parser tracks how many old-side and new-side rows a hunk header promised
// and leaves the hunk body only once both budgets are spent. Counting rather
// than prefix-sniffing is what makes the classifier correct on source that
// happens to look like diff syntax: a Go line reading `--- a/x` inside a hunk
// is consumed as a deletion row because the enclosing hunk still owes old-side
// rows, whereas the same text after the budget is exhausted starts a new file
// section.
func analyze(lines []line) layout {
	n := len(lines)
	lay := layout{
		kinds:  make([]LineKind, n),
		fileID: make([]int, n),
		hunkID: make([]int, n),
		lang:   make([]Language, n),
	}
	for i := range lines {
		lay.fileID[i] = -1
		lay.hunkID[i] = -1
	}

	var (
		inHunk       bool
		oldLeft      int
		newLeft      int
		curFile      = -1
		curHunk      = -1
		curLang      Language
		pendingPaths bool
	)

	// closeHunk finalizes the current hunk's extent. It is called whenever the
	// parser leaves a hunk body, whether because the budget ran out or because
	// a structural line interrupted it.
	closeHunk := func(end int) {
		if curHunk >= 0 {
			lay.hunkBounds[curHunk].end = end
		}
		inHunk = false
	}

	// closeFile finalizes the current file section's extent.
	closeFile := func(end int) {
		if curFile >= 0 {
			lay.fileBounds[curFile].end = end
		}
	}

	for i := 0; i < n; i++ {
		text := lines[i].text

		// Inside a hunk body the marker character decides the row, but only
		// while the header's row budget still has room. A blank line is git's
		// rendering of an empty context row.
		if inHunk && (oldLeft > 0 || newLeft > 0) {
			var kind LineKind
			consumed := true

			switch {
			case text == "":
				kind = KindContext
			case text[0] == '+':
				kind = KindAdd
			case text[0] == '-':
				kind = KindDelete
			case text[0] == ' ':
				kind = KindContext
			case text[0] == '\\':
				// "\ No newline at end of file" annotates the row above and
				// spends no budget of its own.
				kind, consumed = KindNoNewline, false
			default:
				consumed = false
			}

			if consumed || kind == KindNoNewline {
				if consumed {
					switch kind {
					case KindAdd:
						newLeft--
					case KindDelete:
						oldLeft--
					case KindContext:
						oldLeft--
						newLeft--
					}
				}

				lay.kinds[i] = kind
				lay.fileID[i] = curFile
				lay.hunkID[i] = curHunk
				lay.lang[i] = curLang

				continue
			}

			// A line that is neither a marker row nor a newline notice ends
			// the hunk early, even though the header promised more rows.
			closeHunk(i)
		} else if inHunk {
			closeHunk(i)
		}

		switch {
		case strings.HasPrefix(text, "diff --git "):
			closeFile(i)
			curFile = len(lay.fileBounds)
			lay.fileBounds = append(lay.fileBounds, span{start: i, end: n})
			curLang = languageFromGitHeader(text)
			pendingPaths = true
			lay.kinds[i] = KindFileHeader

		case hunkHeaderRE.MatchString(text):
			m := hunkHeaderRE.FindStringSubmatch(text)
			oldLeft = parseCount(m[2])
			newLeft = parseCount(m[4])
			curHunk = len(lay.hunkBounds)
			lay.hunkBounds = append(lay.hunkBounds, span{start: i, end: n})
			inHunk = true
			pendingPaths = false
			lay.kinds[i] = KindHunkHeader

		case pendingPaths && strings.HasPrefix(text, "--- "):
			lay.kinds[i] = KindOldFile

		case pendingPaths && strings.HasPrefix(text, "+++ "):
			lay.kinds[i] = KindNewFile
			if lang := languageFromPath(text[4:]); lang != LangUnknown {
				curLang = lang
			}

		case hasAnyPrefix(text, metaPrefixes):
			lay.kinds[i] = KindMeta

		default:
			lay.kinds[i] = KindOther
		}

		lay.fileID[i] = curFile
		lay.lang[i] = curLang
		if lay.kinds[i] == KindHunkHeader {
			lay.hunkID[i] = curHunk
		}
	}

	closeHunk(n)
	closeFile(n)

	return lay
}

// parseCount converts an optional hunk-header count to an integer. The unified
// diff format omits the count when it is exactly one.
func parseCount(s string) int {
	if s == "" {
		return 1
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 1
	}

	return v
}

// hasAnyPrefix reports whether text begins with any of the given prefixes.
func hasAnyPrefix(text string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}

	return false
}

// gitHeaderRE captures the b-side path of a "diff --git a/x b/y" header. The
// b-side is preferred because it names the file as it exists after the change,
// which is the correct language for an added or renamed file.
var gitHeaderRE = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)

// languageFromGitHeader extracts the language from a file header line, falling
// back to the a-side path when the b-side is absent.
func languageFromGitHeader(text string) Language {
	m := gitHeaderRE.FindStringSubmatch(text)
	if m == nil {
		return LangUnknown
	}
	if lang := languageFromPath(m[2]); lang != LangUnknown {
		return lang
	}

	return languageFromPath(m[1])
}

// languageFromPath maps a file path to its language by extension. A path of
// /dev/null, which git uses for an added or deleted file, yields no language.
func languageFromPath(path string) Language {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")

	// git quotes paths containing unusual bytes; drop the quoting so the
	// extension is still visible.
	path = strings.Trim(path, `"`)

	// A tab separates the path from an optional timestamp in some diff
	// dialects.
	if i := strings.IndexByte(path, '\t'); i >= 0 {
		path = path[:i]
	}

	dot := strings.LastIndexByte(path, '.')
	if dot < 0 {
		return LangUnknown
	}

	switch strings.ToLower(path[dot+1:]) {
	case "go":
		return LangGo
	case "py", "pyi":
		return LangPython
	case "js", "jsx", "mjs", "cjs", "ts", "tsx", "mts", "cts":
		return LangJS
	case "rs":
		return LangRust
	default:
		return LangUnknown
	}
}

// marker returns the leading diff marker of a hunk source row, or zero when
// the line is not a hunk source row. A blank line inside a hunk is an empty
// context row and its marker is a space.
func marker(text string, kind LineKind) byte {
	if !kind.IsHunkSource() {
		return 0
	}
	if text == "" {
		return ' '
	}

	return text[0]
}

// body returns the source text of a hunk row with its diff marker stripped.
func body(text string, kind LineKind) string {
	if !kind.IsHunkSource() || text == "" {
		return ""
	}

	return text[1:]
}

// leadingWhitespace returns the run of spaces and tabs that begins s, which a
// fold reuses so its ellipsis row lines up with the code it replaces.
func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}

	return s[:i]
}
