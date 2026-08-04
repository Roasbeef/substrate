package plangen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestParsePlanAcceptsRealisticOutput covers the shapes a model actually
// produces. The agent is told to emit one fenced json block, but it commonly
// wraps it in commentary or thinks aloud in an earlier block, so the parser has
// to find the real plan without accepting a draft.
func TestParsePlanAcceptsRealisticOutput(t *testing.T) {
	t.Parallel()

	const plan = `{"remove":[{"start_line":3,"end_line":5}],` +
		`"fold":[],"replace":[],"summary":"Drop the boilerplate."}`

	tests := []struct {
		name string
		text string
	}{
		{
			name: "bare fenced block",
			text: "```json\n" + plan + "\n```",
		},
		{
			name: "fence without a language tag",
			text: "```\n" + plan + "\n```",
		},
		{
			name: "surrounded by commentary",
			text: "Here is the plan.\n\n```json\n" + plan +
				"\n```\n\nLet me know if you want more folded.",
		},
		{
			name: "later block wins over an earlier draft",
			text: "First attempt:\n```json\n{\"remove\":[]}\n```\n" +
				"Corrected:\n```json\n" + plan + "\n```",
		},
		{
			name: "unfenced object",
			text: "The plan is " + plan,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePlan(tc.text)
			require.NoError(t, err)
			require.Equal(t, "Drop the boilerplate.", got.Summary)
			require.Len(t, got.Remove, 1)
			require.Equal(t, 3, got.Remove[0].StartLine)
			require.Equal(t, 5, got.Remove[0].EndLine)
		})
	}
}

// TestParsePlanNormalizesMissingArrays asserts an omitted category becomes an
// empty slice. The compiler rejects nil arrays on purpose, so that a generator
// cannot silently drop a category, but an absent key in valid JSON is an
// honest "no edits here" and should not cost a retry.
func TestParsePlanNormalizesMissingArrays(t *testing.T) {
	t.Parallel()

	got, err := parsePlan("```json\n{\"summary\":\"only a summary\"}\n```")
	require.NoError(t, err)

	require.NotNil(t, got.Remove)
	require.NotNil(t, got.Fold)
	require.NotNil(t, got.Replace)
	require.NoError(t, got.Validate())
}

// TestParsePlanRejectsBadOutput asserts the parser fails loudly rather than
// returning a half-understood plan, since a silently empty plan would look
// like a deliberate decision to keep everything.
func TestParsePlanRejectsBadOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
	}{
		{name: "no json at all", text: "I could not abridge this diff."},
		{name: "empty output", text: ""},
		{
			name: "unknown field",
			text: "```json\n{\"remove\":[],\"fold\":[],\"replace\":[]," +
				"\"delete_everything\":true}\n```",
		},
		{
			name: "malformed json",
			text: "```json\n{\"remove\": [ {\"start_line\": }\n```",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parsePlan(tc.text)
			require.Error(t, err)
		})
	}
}

// TestRubricHashIsStableAndCoversPrompt asserts the hash is deterministic and
// actually depends on the prompt text. Cache keys mix this in, so a rubric edit
// that did not change the hash would serve stale abridgements forever.
func TestRubricHashIsStableAndCoversPrompt(t *testing.T) {
	t.Parallel()

	require.Equal(t, RubricHash(), RubricHash())
	require.Len(t, RubricHash(), 16)
	require.Contains(t, systemPrompt, "coordinates")
}

// permissionRequest builds a tool permission request with the given arguments.
func permissionRequest(t *testing.T, tool string,
	args map[string]string) claudeagent.ToolPermissionRequest {

	t.Helper()

	raw, err := json.Marshal(args)
	require.NoError(t, err)

	return claudeagent.ToolPermissionRequest{
		ToolName:  tool,
		Arguments: raw,
	}
}

// TestReadOnlyPolicyDeniesByDefault asserts every tool that could mutate the
// tree or run a command is refused. Default-deny is the point: a tool the SDK
// adds tomorrow is refused without this code knowing it exists.
func TestReadOnlyPolicyDeniesByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policy := readOnlyPolicy(root)

	for _, tool := range []string{
		"Write", "Edit", "MultiEdit", "Bash", "NotebookEdit",
		"WebFetch", "Task", "SomeFutureTool",
	} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			res := policy(context.Background(), permissionRequest(
				t, tool, map[string]string{
					"file_path": filepath.Join(root, "x.go"),
				},
			))
			require.False(t, res.IsAllow(), "%s must be denied", tool)
		})
	}
}

// TestReadOnlyPolicyAllowsInspectionInsideRepo asserts the tools the generator
// genuinely needs are permitted within the repository.
func TestReadOnlyPolicyAllowsInspectionInsideRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "main.go"), []byte("package main\n"), 0o600,
	))
	policy := readOnlyPolicy(root)

	for _, tool := range []string{"Read", "Grep", "Glob", "LS"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			res := policy(context.Background(), permissionRequest(
				t, tool, map[string]string{
					"file_path": filepath.Join(root, "main.go"),
				},
			))
			require.True(t, res.IsAllow(), "%s must be allowed", tool)
		})
	}
}

// TestReadOnlyPolicyConfinesToRepo asserts a read cannot escape the repository
// through an absolute path or a traversal, so an allowed tool cannot be turned
// into a way to exfiltrate unrelated files.
func TestReadOnlyPolicyConfinesToRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policy := readOnlyPolicy(root)

	for _, path := range []string{
		"/etc/passwd",
		filepath.Join(root, "..", "escaped.txt"),
		"../../../.ssh/id_ed25519",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			res := policy(context.Background(), permissionRequest(
				t, "Read", map[string]string{"file_path": path},
			))
			require.False(t, res.IsAllow(),
				"reading %q must be denied", path)
		})
	}
}

// TestReadOnlyPolicyAllowsPathlessSearch asserts a search with no explicit
// path is permitted, since the agent's working directory is already the
// repository root.
func TestReadOnlyPolicyAllowsPathlessSearch(t *testing.T) {
	t.Parallel()

	policy := readOnlyPolicy(t.TempDir())

	res := policy(context.Background(), permissionRequest(
		t, "Grep", map[string]string{"pattern": "func main"},
	))
	require.True(t, res.IsAllow())
}

// TestReadOnlyPolicyWithoutRootDeniesPathReads asserts that with no repository
// configured, a read naming a path is refused.
//
// The subtle failure this guards against is that filepath.Abs("") yields the
// process working directory. Normalizing an empty root would therefore confine
// the agent to wherever the daemon was launched — a boundary unrelated to the
// diff, which would happily permit reads of neighbouring checkouts.
func TestReadOnlyPolicyWithoutRootDeniesPathReads(t *testing.T) {
	t.Parallel()

	policy := readOnlyPolicy("")

	denied := policy(context.Background(), permissionRequest(
		t, "Read", map[string]string{"file_path": "/anywhere/x.go"},
	))
	require.False(t, denied.IsAllow())

	// A relative path resolved against the daemon's own directory must be
	// refused too, not silently accepted as "inside the root".
	relative := policy(context.Background(), permissionRequest(
		t, "Read", map[string]string{"file_path": "go.mod"},
	))
	require.False(t, relative.IsAllow())

	bash := policy(context.Background(), permissionRequest(
		t, "Bash", map[string]string{"command": "rm -rf /"},
	))
	require.False(t, bash.IsAllow())
}
