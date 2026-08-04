package readingdiff

import (
	"regexp"
	"strings"
)

// Import churn is the single largest category of noise in a reviewed diff, and
// it is noise a reviewer can always reconstruct: if the body of a change calls
// hex.EncodeToString, the import of encoding/hex is implied. So this file
// derives import removal mechanically from the source rather than asking a
// plan generator to spend coordinates on it.
//
// Two properties make the mechanical approach safe. First, classification runs
// per side of the hunk: the old-side rows are scanned independently of the
// new-side rows, so a package substitution hides both halves and never leaks
// as a one-sided deletion. Second, an unchanged context row is hidden only
// when both sides agree it is an import, so a context row that anchors real
// code survives.
//
// The classifiers are deliberately conservative. Hiding a row that carries
// behavior is a far worse failure than leaving an import visible, so every
// rule below requires an unambiguous syntactic signal.

// mandatoryHidden returns a mask over physical lines marking every row that is
// import scaffolding and must never reach the rendered output.
func mandatoryHidden(lines []line, lay layout) []bool {
	hidden := make([]bool, len(lines))

	for _, bounds := range lay.hunkBounds {
		lang := hunkLanguage(lay, bounds)
		if lang == LangUnknown {
			continue
		}

		oldSide := classifySide(lines, lay, bounds, '-', lang)
		newSide := classifySide(lines, lay, bounds, '+', lang)

		for i := bounds.start; i < bounds.end && i < len(lines); i++ {
			kind := lay.kinds[i]
			if !kind.IsHunkSource() {
				continue
			}

			switch marker(lines[i].text, kind) {
			case '-':
				hidden[i] = oldSide[i]
			case '+':
				hidden[i] = newSide[i]
			case ' ':
				// An unchanged row is scaffolding only if it reads as an
				// import from both sides. Requiring agreement keeps a context
				// row that anchors surrounding code visible.
				hidden[i] = oldSide[i] && newSide[i]
			}
		}
	}

	return hidden
}

// hunkLanguage returns the language of the file section owning a hunk.
func hunkLanguage(lay layout, bounds span) Language {
	if bounds.start < len(lay.lang) {
		return lay.lang[bounds.start]
	}

	return LangUnknown
}

// sideRow is one source row projected onto a single side of a hunk, carrying
// its physical index so a classification can be mapped back to the input.
type sideRow struct {
	idx  int
	body string
}

// classifySide projects a hunk onto one side, runs the language's import
// recognizer over that projection, and returns a mask over physical lines.
//
// Projecting first is what makes multi-line constructs work. An import block
// whose members changed appears interleaved with the other side's rows in the
// raw diff; read as a single side, it is once again a contiguous block with a
// recognizable opener and closer.
func classifySide(lines []line, lay layout, bounds span, side byte,
	lang Language) []bool {

	var rows []sideRow
	for i := bounds.start; i < bounds.end && i < len(lines); i++ {
		kind := lay.kinds[i]
		if !kind.IsHunkSource() {
			continue
		}

		m := marker(lines[i].text, kind)
		if m != side && m != ' ' {
			continue
		}
		rows = append(rows, sideRow{
			idx:  i,
			body: body(lines[i].text, kind),
		})
	}

	flags := recognizeImports(rows, lang)

	mask := make([]bool, len(lines))
	for i, row := range rows {
		if flags[i] {
			mask[row.idx] = true
		}
	}

	return mask
}

// recognizeImports runs the language-appropriate recognizer over one side's
// rows and returns a parallel slice marking the import rows.
func recognizeImports(rows []sideRow, lang Language) []bool {
	switch lang {
	case LangGo:
		return recognizeGoImports(rows)
	case LangPython:
		return recognizePythonImports(rows)
	case LangJS:
		return recognizeJSImports(rows)
	case LangRust:
		return recognizeRustImports(rows)
	default:
		return make([]bool, len(rows))
	}
}

var (
	// goBlockStartRE matches the opener of a grouped import declaration.
	goBlockStartRE = regexp.MustCompile(`^import\s*\(\s*(?://.*)?$`)

	// goSingleRE matches a single-line import, with or without an alias.
	goSingleRE = regexp.MustCompile(
		`^import\s+(?:(?:[._]|[A-Za-z_]\w*)\s+)?` +
			"(?:\"[^\"]*\"|`[^`]*`)\\s*(?://.*)?$",
	)

	// goMemberRE matches a member line inside a grouped import: an optional
	// alias followed by a quoted import path and an optional comment.
	//
	// Recognizing a bare member matters because a hunk often starts midway
	// through an import block, with no opener in view. But the shape is easy
	// to over-match, so three conditions narrow it. The path may not be empty
	// and may contain only the characters legal in an import path, which rules
	// out ordinary string literals such as `return ""` or `t.Fatal("no such
	// file")`. There must be no trailing comma, which separates a member from
	// an element of a string slice. And the alias may not be a Go keyword,
	// which is what stops `return "ok"` from reading as an aliased import.
	goMemberRE = regexp.MustCompile(
		`^(?:([._]|[A-Za-z_]\w*)\s+)?` +
			"(?:\"([A-Za-z0-9_./~+-]+)\"|`([A-Za-z0-9_./~+-]+)`)" +
			`\s*(?://.*)?$`,
	)
)

// goKeywords are the reserved words that could otherwise be misread as an
// import alias preceding a quoted string.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true,
	"continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true,
	"var": true,
}

// isGoImportMember reports whether a trimmed line is a member of a grouped Go
// import declaration, judged without the surrounding block in view.
func isGoImportMember(trimmed string) bool {
	m := goMemberRE.FindStringSubmatch(trimmed)
	if m == nil {
		return false
	}

	return !goKeywords[m[1]]
}

// recognizeGoImports marks Go import declarations, including grouped blocks
// and members of a block whose opener falls outside the hunk.
func recognizeGoImports(rows []sideRow) []bool {
	flags := make([]bool, len(rows))
	inBlock := false
	prevWasMember := false

	for i, row := range rows {
		trimmed := strings.TrimSpace(row.body)

		if inBlock {
			flags[i] = true
			if trimmed == ")" {
				inBlock = false
			}

			continue
		}

		switch {
		case goBlockStartRE.MatchString(trimmed):
			flags[i] = true
			inBlock = true

		case goSingleRE.MatchString(trimmed):
			flags[i] = true

		case isGoImportMember(trimmed):
			flags[i] = true
			prevWasMember = true

			continue

		case trimmed == ")" && prevWasMember:
			// The hunk began partway through an import block, so the opener
			// that would have set inBlock is out of view. A lone closing
			// paren directly after a member closes that block: gofmt requires
			// a trailing comma before the closing paren of a call, so a
			// comma-less quoted string above a bare ")" cannot be a call.
			flags[i] = true
		}

		prevWasMember = false
	}

	return flags
}

var (
	// pyImportRE matches a plain import statement with optional aliases.
	pyImportRE = regexp.MustCompile(
		`^import\s+[A-Za-z_]\w*(?:\.\w+)*(?:\s+as\s+\w+)?` +
			`(?:\s*,\s*[A-Za-z_]\w*(?:\.\w+)*(?:\s+as\s+\w+)?)*\s*$`,
	)

	// pyFromRE matches a from-import, capturing whatever follows "import" so
	// an opening parenthesis can be detected.
	pyFromRE = regexp.MustCompile(
		`^from\s+\.*(?:[A-Za-z_]\w*(?:\.\w+)*)?\s+import\s+(.+)$`,
	)
)

// recognizePythonImports marks Python import statements, following a
// parenthesized member list across lines.
func recognizePythonImports(rows []sideRow) []bool {
	flags := make([]bool, len(rows))
	inParens := false

	for i, row := range rows {
		trimmed := strings.TrimSpace(row.body)

		if inParens {
			flags[i] = true
			if strings.Contains(trimmed, ")") {
				inParens = false
			}

			continue
		}

		switch {
		case pyImportRE.MatchString(trimmed):
			flags[i] = true

		case pyFromRE.MatchString(trimmed):
			flags[i] = true
			rest := pyFromRE.FindStringSubmatch(trimmed)[1]
			if strings.Contains(rest, "(") &&
				!strings.Contains(rest, ")") {

				inParens = true
			}
		}
	}

	return flags
}

var (
	// jsSideEffectRE matches a bare import used only for its side effects.
	jsSideEffectRE = regexp.MustCompile(
		`^import\s+['"][^'"]+['"]\s*;?\s*$`,
	)

	// jsFromRE matches a complete single-line import with a source clause.
	jsFromRE = regexp.MustCompile(
		`^import\s+.+\s+from\s+['"][^'"]+['"]\s*;?\s*$`,
	)

	// jsOpenRE matches an import whose member list opens but does not close on
	// the same line.
	jsOpenRE = regexp.MustCompile(`^import\s+(?:type\s+)?\{[^}]*$`)

	// jsCloseRE matches the closing line of a multi-line member list.
	jsCloseRE = regexp.MustCompile(`^\}\s*from\s+['"][^'"]+['"]\s*;?\s*$`)

	// jsRequireRE matches a require bound to a declaration.
	jsRequireRE = regexp.MustCompile(
		`^(?:const|let|var)\s+.+=\s*require\s*\(\s*['"][^'"]+['"]\s*\)` +
			`\s*;?\s*$`,
	)
)

// recognizeJSImports marks JavaScript and TypeScript imports, including
// multi-line member lists and require bindings.
func recognizeJSImports(rows []sideRow) []bool {
	flags := make([]bool, len(rows))
	inMembers := false

	for i, row := range rows {
		trimmed := strings.TrimSpace(row.body)

		if inMembers {
			flags[i] = true
			if jsCloseRE.MatchString(trimmed) {
				inMembers = false
			}

			continue
		}

		switch {
		case jsSideEffectRE.MatchString(trimmed),
			jsFromRE.MatchString(trimmed),
			jsRequireRE.MatchString(trimmed):

			flags[i] = true

		case jsOpenRE.MatchString(trimmed):
			flags[i] = true
			inMembers = true
		}
	}

	return flags
}

var (
	// rustUseRE matches a use declaration that completes on one line.
	rustUseRE = regexp.MustCompile(
		`^(?:pub(?:\s*\([^)]*\))?\s+)?use\s+[^;]+;\s*$`,
	)

	// rustUseOpenRE matches a use declaration whose brace group stays open.
	rustUseOpenRE = regexp.MustCompile(
		`^(?:pub(?:\s*\([^)]*\))?\s+)?use\s+[^;]*\{[^}]*$`,
	)
)

// recognizeRustImports marks Rust use declarations, following a braced group
// across lines until it terminates.
func recognizeRustImports(rows []sideRow) []bool {
	flags := make([]bool, len(rows))
	inGroup := false

	for i, row := range rows {
		trimmed := strings.TrimSpace(row.body)

		if inGroup {
			flags[i] = true
			if strings.Contains(trimmed, ";") {
				inGroup = false
			}

			continue
		}

		switch {
		case rustUseRE.MatchString(trimmed):
			flags[i] = true

		case rustUseOpenRE.MatchString(trimmed):
			flags[i] = true
			inGroup = true
		}
	}

	return flags
}
