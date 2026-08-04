package plangen

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/roasbeef/subtrate/internal/readingdiff"
)

// stubGenerator returns a generator wired to the stub CLI, along with the path
// its recorded argv will be written to.
//
// Running the real binary would need credentials and would bill for every test
// run, so the flags and the message handling are exercised against a stub that
// speaks the same protocol. The scenario selects which stream the stub replays.
func stubGenerator(t *testing.T, scenario string) (*Generator, string) {
	t.Helper()

	stub, err := filepath.Abs(filepath.Join("testdata", "stubcli.py"))
	require.NoError(t, err)

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required to run the stub CLI")
	}

	argvOut := filepath.Join(t.TempDir(), "argv")

	// The SDK executes the CLI path directly, so the stub needs its
	// interpreter baked into a wrapper rather than relying on the shebang bit
	// surviving a checkout.
	wrapper := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nexec " + python + " " + stub + " \"$@\"\n"
	require.NoError(t, os.WriteFile(wrapper, []byte(script), 0o700))

	t.Setenv("STUB_ARGV_OUT", argvOut)
	t.Setenv("STUB_SCENARIO", scenario)

	gen := New(Config{
		CLIPath: wrapper,
		Logf:    func(format string, args ...any) { t.Logf(format, args...) },
	})

	return gen, argvOut
}

// TestGeneratorPassesBoundsToCLI asserts the turn ceiling and the tool
// withholding actually reach the subprocess command line.
//
// This guards a failure mode that reads as working code: the SDK's
// WithMaxTurns and WithDisallowedTools set fields on its Options struct that
// its transport never consults when building argv, so calling them compiles,
// looks like a bound, and enforces nothing. Asserting on the argv is the only
// way to tell an effective option from a decorative one.
func TestGeneratorPassesBoundsToCLI(t *testing.T) {
	gen, argvOut := stubGenerator(t, "plan")

	_, err := gen.Generate(context.Background(), readingdiff.Request{
		UnifiedDiff: sampleDiff,
	}, "")
	require.NoError(t, err)

	argv, err := os.ReadFile(argvOut)
	require.NoError(t, err)
	args := strings.Split(string(argv), "\n")

	require.Contains(t, args, "--max-turns")
	require.Contains(t, args, "1",
		"a text-only abridgement must be capped at one turn")

	require.Contains(t, args, "--disallowed-tools")
	require.Contains(t, args, "Glob,Grep,LS,Read",
		"with no repository, the read tools must not even be offered")
}

// TestGeneratorAllowsInspectionWhenRepoGiven asserts the bounds relax when the
// generator is handed a repository, since looking up a call site legitimately
// takes more than one turn.
func TestGeneratorAllowsInspectionWhenRepoGiven(t *testing.T) {
	gen, argvOut := stubGenerator(t, "plan")

	_, err := gen.Generate(context.Background(), readingdiff.Request{
		UnifiedDiff: sampleDiff,
		RepoRoot:    t.TempDir(),
	}, "")
	require.NoError(t, err)

	argv, err := os.ReadFile(argvOut)
	require.NoError(t, err)
	args := strings.Split(string(argv), "\n")

	require.Contains(t, args, "12")
	require.NotContains(t, args, "--disallowed-tools")
}

// TestGeneratorReportsSilentRuns asserts that a run producing no model text
// fails with the cause rather than with a parser complaint.
//
// Each scenario here previously surfaced as the same unhelpful message. The
// worst of them reported "agent reported an error: " with nothing after the
// colon, because the CLI had left Result empty and put its diagnostic in
// Errors.
func TestGeneratorReportsSilentRuns(t *testing.T) {
	tests := []struct {
		scenario string
		want     string
	}{
		{scenario: "retries", want: "the API was never reached"},
		{scenario: "hooks", want: "loaded external automation"},
		{scenario: "denials", want: "tool calls were denied"},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			gen, _ := stubGenerator(t, tc.scenario)

			_, err := gen.Generate(context.Background(),
				readingdiff.Request{UnifiedDiff: sampleDiff}, "")

			require.Error(t, err)
			require.ErrorIs(t, err, ErrModelSilent)
			require.Contains(t, err.Error(), tc.want)

			// The counts that justify the diagnosis travel with it, so a log
			// line is enough to confirm the reading without a rerun.
			require.Contains(t, err.Error(), "messages=")
		})
	}
}

// TestGeneratorParsesPlanFromStream asserts the happy path still works through
// the real SDK client, not just through parsePlan in isolation.
func TestGeneratorParsesPlanFromStream(t *testing.T) {
	gen, _ := stubGenerator(t, "plan")

	plan, err := gen.Generate(context.Background(), readingdiff.Request{
		UnifiedDiff: sampleDiff,
	}, "")
	require.NoError(t, err)

	require.Equal(t, "Drops the boilerplate.", plan.Summary)
	require.Len(t, plan.Remove, 1)
	require.Equal(t, 4, plan.Remove[0].StartLine)
	require.Equal(t, 6, plan.Remove[0].EndLine)
	require.False(t, errors.Is(err, ErrModelSilent))
}

// sampleDiff is a small well-formed patch, enough to exercise the request path
// without making the assertions about it.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

+// Added.
 func main() {}
`
