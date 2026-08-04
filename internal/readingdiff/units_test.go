package readingdiff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsElisionProjection pins the rule that decides whether a partial elision
// is honest. Everything the package promises about not inventing text reduces
// to this predicate, so both directions matter: it must accept every genuine
// truncation and reject anything that adds or silently drops characters.
func TestIsElisionProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		old  string
		new  string
		want bool
	}{
		{
			name: "identity",
			old:  `t.Errorf("boom: %v", err)`,
			new:  `t.Errorf("boom: %v", err)`,
			want: true,
		},
		{
			name: "whole span elided",
			old:  `"route SSH Key ID = %d, want %d", a, b`,
			new:  `...`,
			want: true,
		},
		{
			name: "head kept",
			old:  `resp.SSHKeyID = rd.sshKeyID`,
			new:  `resp.SSHKeyID = rd...`,
			want: true,
		},
		{
			name: "head and tail kept",
			old:  `fmt.Errorf("a very long message: %w", err)`,
			new:  `fmt.Errorf(...err)`,
			want: true,
		},
		{
			name: "typographic ellipsis accepted",
			old:  `logger.Info("starting", "addr", addr)`,
			new:  `logger.Info(…)`,
			want: true,
		},
		{
			name: "multiple elided spans",
			old:  `call(alpha, beta, gamma, delta)`,
			new:  `call(alpha, ..., delta)`,
			want: true,
		},
		{
			name: "invented identifier rejected",
			old:  `hex.EncodeToString(b)`,
			new:  `base64.Encode(b)`,
			want: false,
		},
		{
			name: "silent character drop rejected",
			old:  `hex.EncodeToString(b)`,
			new:  `hex.Encode(b)`,
			want: false,
		},
		{
			name: "reordering rejected",
			old:  `call(alpha, delta)`,
			new:  `call(delta, ...alpha)`,
			want: false,
		},
		{
			name: "invented comment rejected",
			old:  `x := compute(a, b)`,
			new:  `x := compute(...) // sums a and b`,
			want: false,
		},
		{
			name: "wrong head anchor rejected",
			old:  `alpha beta gamma`,
			new:  `zeta...`,
			want: false,
		},
		{
			name: "wrong tail anchor rejected",
			old:  `alpha beta gamma`,
			new:  `...zeta`,
			want: false,
		},
		{
			name: "unmarked truncation rejected",
			old:  `alpha beta gamma`,
			new:  `alpha`,
			want: false,
		},
		{
			name: "empty replacement without marker rejected",
			old:  `alpha`,
			new:  ``,
			want: false,
		},
		{
			name: "repeated fragment stays anchored",
			old:  `foo(x, foo, y)`,
			new:  `foo(...)`,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want,
				isElisionProjection(tc.old, tc.new))
		})
	}
}

// TestAnalyzeClassifiesSourceThatLooksLikeSyntax checks the hunk-aware
// classifier on the case that defeats a prefix-only parser: source rows whose
// text begins with ---, +++, @@, or "diff --git". Inside a hunk body those are
// program text, and misreading one as structure would let a plan hide or fold
// across a boundary that does not exist.
func TestAnalyzeClassifiesSourceThatLooksLikeSyntax(t *testing.T) {
	t.Parallel()

	raw := "diff --git a/x.md b/x.md\n" +
		"--- a/x.md\n" +
		"+++ b/x.md\n" +
		"@@ -1,3 +1,4 @@\n" +
		" --- a/not/a/header\n" +
		"-@@ fake hunk\n" +
		"+++ added row whose text starts with plus\n" +
		"+diff --git in source\n" +
		" trailing context\n"

	lines := splitLines(raw)
	lay := analyze(lines)

	require.Equal(t, KindFileHeader, lay.kinds[0])
	require.Equal(t, KindOldFile, lay.kinds[1])
	require.Equal(t, KindNewFile, lay.kinds[2])
	require.Equal(t, KindHunkHeader, lay.kinds[3])

	// Everything inside the hunk body is source, decided by the row budget
	// rather than by the leading characters.
	require.Equal(t, KindContext, lay.kinds[4])
	require.Equal(t, KindDelete, lay.kinds[5])
	require.Equal(t, KindAdd, lay.kinds[6])
	require.Equal(t, KindAdd, lay.kinds[7])
	require.Equal(t, KindContext, lay.kinds[8])
}

// TestAnalyzeAssignsLanguageFromPath checks that the file extension selects the
// import recognizer, including for a file added from /dev/null where only the
// b-side path names the language.
func TestAnalyzeAssignsLanguageFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		header string
		want   Language
	}{
		{"diff --git a/a.go b/a.go", LangGo},
		{"diff --git a/a.py b/a.py", LangPython},
		{"diff --git a/a.ts b/a.ts", LangJS},
		{"diff --git a/a.tsx b/a.tsx", LangJS},
		{"diff --git a/a.rs b/a.rs", LangRust},
		{"diff --git a/README.md b/README.md", LangUnknown},
		{"diff --git a/Makefile b/Makefile", LangUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, languageFromGitHeader(tc.header))
		})
	}
}

// recognized runs a language recognizer over raw source rows and returns the
// bodies it marked as imports, which keeps the recognizer cases readable.
func recognized(rows []string, lang Language) []string {
	side := make([]sideRow, len(rows))
	for i, r := range rows {
		side[i] = sideRow{idx: i, body: r}
	}

	flags := recognizeImports(side, lang)

	var out []string
	for i, ok := range flags {
		if ok {
			out = append(out, rows[i])
		}
	}

	return out
}

// TestRecognizeGoImports covers grouped blocks, single-line forms, aliases, and
// the two shapes that must NOT be mistaken for imports: a bare string return
// and an element of a string slice.
func TestRecognizeGoImports(t *testing.T) {
	t.Parallel()

	rows := []string{
		`import (`,
		`	"fmt"`,
		`	_ "embed"`,
		`	pb "example.com/gen/proto"`,
		``,
		`	"strings"`,
		`)`,
		`import "os"`,
		`func f() string {`,
		`	return ""`,
		`	return "ok"`,
		`	tags := []string{`,
		`		"alpha",`,
		`	}`,
		`}`,
	}

	got := recognized(rows, LangGo)
	require.Equal(t, []string{
		`import (`,
		`	"fmt"`,
		`	_ "embed"`,
		`	pb "example.com/gen/proto"`,
		``,
		`	"strings"`,
		`)`,
		`import "os"`,
	}, got)
}

// TestRecognizeGoImportsWithoutOpener checks that members are still recognized
// when the hunk begins partway through an import block, which is the common
// case for a one-line import change.
func TestRecognizeGoImportsWithoutOpener(t *testing.T) {
	t.Parallel()

	rows := []string{
		`	"context"`,
		`	"errors"`,
		`)`,
		`func g() {}`,
	}

	require.Equal(t, []string{
		`	"context"`,
		`	"errors"`,
		`)`,
	}, recognized(rows, LangGo))
}

// TestRecognizePythonImports covers plain imports, from-imports, aliases, and a
// parenthesized member list spanning several lines.
func TestRecognizePythonImports(t *testing.T) {
	t.Parallel()

	rows := []string{
		`import os`,
		`import os.path as p`,
		`from . import sibling`,
		`from typing import (`,
		`    Any,`,
		`    Optional,`,
		`)`,
		`from foo.bar import baz, qux`,
		`def handler(request):`,
		`    return import_helper()`,
	}

	require.Equal(t, []string{
		`import os`,
		`import os.path as p`,
		`from . import sibling`,
		`from typing import (`,
		`    Any,`,
		`    Optional,`,
		`)`,
		`from foo.bar import baz, qux`,
	}, recognized(rows, LangPython))
}

// TestRecognizeJSImports covers default, named, side-effect, type-only, and
// require forms, plus a multi-line member list.
func TestRecognizeJSImports(t *testing.T) {
	t.Parallel()

	rows := []string{
		`import React from 'react';`,
		`import './styles.css';`,
		`import type { Foo } from './types.js';`,
		`import {`,
		`  useState,`,
		`  useMemo,`,
		`} from 'react';`,
		`const path = require('node:path');`,
		`export function Component() {`,
		`  return null;`,
		`}`,
	}

	require.Equal(t, []string{
		`import React from 'react';`,
		`import './styles.css';`,
		`import type { Foo } from './types.js';`,
		`import {`,
		`  useState,`,
		`  useMemo,`,
		`} from 'react';`,
		`const path = require('node:path');`,
	}, recognized(rows, LangJS))
}

// TestRecognizeRustImports covers plain and visibility-qualified use
// declarations plus a braced group spanning lines.
func TestRecognizeRustImports(t *testing.T) {
	t.Parallel()

	rows := []string{
		`use std::collections::HashMap;`,
		`pub use crate::error::Error;`,
		`use std::{`,
		`    fmt,`,
		`    io::Write,`,
		`};`,
		`fn main() {}`,
	}

	require.Equal(t, []string{
		`use std::collections::HashMap;`,
		`pub use crate::error::Error;`,
		`use std::{`,
		`    fmt,`,
		`    io::Write,`,
		`};`,
	}, recognized(rows, LangRust))
}

// TestRecognizeUnknownLanguageMarksNothing checks that a file whose extension
// we do not know keeps every row, since guessing import syntax for an unknown
// language could hide real code.
func TestRecognizeUnknownLanguageMarksNothing(t *testing.T) {
	t.Parallel()

	rows := []string{`import os`, `use std::fmt;`, `import "os"`}
	require.Empty(t, recognized(rows, LangUnknown))
}

// TestSplitLinesPreservesEndings checks that CRLF input survives a round trip
// and that a final line without a newline does not gain one.
func TestSplitLinesPreservesEndings(t *testing.T) {
	t.Parallel()

	raw := "alpha\r\nbeta\ngamma"
	lines := splitLines(raw)
	require.Len(t, lines, 3)
	require.Equal(t, "\r\n", lines[0].eol)
	require.Equal(t, "\n", lines[1].eol)
	require.Equal(t, "", lines[2].eol)

	var rebuilt string
	for _, l := range lines {
		rebuilt += l.text + l.eol
	}
	require.Equal(t, raw, rebuilt)
}
