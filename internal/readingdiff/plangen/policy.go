package plangen

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// readOnlyTools are the only tools a plan generator needs. It inspects source
// to decide whether a diff row is load-bearing, which requires reading files
// and searching for call sites and nothing else.
var readOnlyTools = map[string]bool{
	"Read": true,
	"Grep": true,
	"Glob": true,
	"LS":   true,
}

// readOnlyPolicy returns a permission callback that allows only read-only
// tools, and only inside the repository being described.
//
// Denying by default rather than blocking a list of dangerous tools is the
// safer shape: a future SDK release can add a tool that writes, and a
// default-deny policy will refuse it without needing to know it exists. The
// path check then stops a Read of, say, an SSH key from succeeding merely
// because Read is an allowed tool.
func readOnlyPolicy(repoRoot string) claudeagent.CanUseToolFunc {
	// Normalize only when a root was actually supplied. filepath.Abs("")
	// returns the process working directory, which would quietly turn "no
	// repository configured" into "confine to wherever the daemon happens to
	// be running" — a boundary with no relationship to the diff.
	var root string
	if repoRoot != "" {
		root = repoRoot
		if abs, err := filepath.Abs(repoRoot); err == nil {
			root = abs
		}
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		}
	}

	return func(_ context.Context,
		req claudeagent.ToolPermissionRequest) claudeagent.PermissionResult {

		if !readOnlyTools[req.ToolName] {
			return claudeagent.PermissionDeny{
				Reason: fmt.Sprintf(
					"%s is not available; a reading-diff plan is built "+
						"from the diff plus read-only inspection",
					req.ToolName),
			}
		}

		path, ok := toolPath(req.Arguments)

		if root == "" {
			// With no repository to confine to, the generator was asked to
			// judge from the diff alone. Allow only operations that name no
			// path, so an unconfined read cannot reach arbitrary files.
			if ok {
				return claudeagent.PermissionDeny{
					Reason: "no repository is configured for this " +
						"abridgement; judge from the diff text alone",
				}
			}

			return claudeagent.PermissionAllow{}
		}

		if !ok {
			// A search with no explicit path is scoped to the working
			// directory, which is already the repository root.
			return claudeagent.PermissionAllow{}
		}
		if !withinRoot(root, path) {
			return claudeagent.PermissionDeny{
				Reason: fmt.Sprintf(
					"path %q is outside the repository being reviewed",
					path),
			}
		}

		return claudeagent.PermissionAllow{}
	}
}

// toolPath pulls the filesystem path out of a tool's arguments, reporting
// whether one was present. The SDK's read and search tools spell it
// differently, so both spellings are checked.
func toolPath(raw json.RawMessage) (string, bool) {
	var args struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", false
	}

	switch {
	case args.FilePath != "":
		return args.FilePath, true
	case args.Path != "":
		return args.Path, true
	default:
		return "", false
	}
}

// withinRoot reports whether path resolves inside root, defeating traversal
// through .. and absolute paths that point elsewhere.
func withinRoot(root, path string) bool {
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)

	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = resolved
	}

	if candidate == root {
		return true
	}

	return strings.HasPrefix(candidate, root+string(filepath.Separator))
}
