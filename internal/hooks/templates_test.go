package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStopScriptArmsWithoutEndingTheCurrentTurn protects the watcher contract.
// The Stop hook is reached when an agent tries to finish a turn, but its
// watcher is only an idle-notification mechanism. Arming it must not direct
// an agent to abandon work that is still in progress.
func TestStopScriptArmsWithoutEndingTheCurrentTurn(t *testing.T) {
	require.Contains(t, StopScript,
		"it does not require ending your turn. Continue the current task if work remains",
	)
	require.NotContains(t, StopScript, "then end your turn")

	installed, ok := AllScripts()["stop"]
	require.True(t, ok)
	require.Equal(t, StopScript, installed)
	require.Equal(t, StopScript, GetScript("stop"))
}

// TestStopScriptSharesProjectWatchStateWithTool verifies the Stop hook uses
// project-local notification state instead of its own HOME. Loom points an
// imported hook at the operator home to resolve its script, while a tool
// receives a private home below the project.
func TestStopScriptSharesProjectWatchStateWithTool(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workspace, ".git"), 0o755))
	projectSubdir := filepath.Join(workspace, "project-subdir")
	nested := filepath.Join(workspace, "nested", "deeper")
	require.NoError(t, os.MkdirAll(projectSubdir, 0o755))
	require.NoError(t, os.MkdirAll(nested, 0o755))
	hookHome := t.TempDir()
	toolHome := t.TempDir()
	binDir := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "stop.sh")

	require.NoError(t, os.WriteFile(scriptPath, []byte(StopScript), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(binDir, "substrate"),
		[]byte(`#!/bin/sh
case "$1" in
heartbeat) exit 0 ;;
poll) printf '{"decision":"allow"}' ;;
watch)
  if [ "$2" = "--check" ] && [ -f "$EXPECTED_WATCH_ROOT/.substrate/watch/armed" ]; then
    exit 0
  fi
  exit 1
  ;;
esac
exit 1
`),
		0o755,
	))

	run := func(active, dir, project, expectedRoot string) string {
		cmd := exec.Command("bash", scriptPath)
		cmd.Dir = dir
		cmd.Env = []string{
			"HOME=" + hookHome,
			"PATH=" + binDir + ":" + os.Getenv("PATH"),
			"EXPECTED_WATCH_ROOT=" + expectedRoot,
		}
		if project != "" {
			cmd.Env = append(cmd.Env, "CLAUDE_PROJECT_DIR="+project)
		}
		cmd.Stdin = strings.NewReader(
			`{"session_id":"same-session","stop_hook_active":` + active + `}`,
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
		return string(output)
	}

	first := run("false", nested, projectSubdir, projectSubdir)
	require.Contains(t, first, `"decision":"block"`)
	require.FileExists(t, filepath.Join(
		projectSubdir, ".substrate", "watch", "nudged-same-session",
	))
	require.NoFileExists(t, filepath.Join(
		workspace, ".substrate", "watch", "nudged-same-session",
	))
	require.NoFileExists(t, filepath.Join(
		hookHome, ".subtrate", "watch", "nudged-same-session",
	))
	require.NoFileExists(t, filepath.Join(
		toolHome, ".subtrate", "watch", "nudged-same-session",
	))

	second := run("true", nested, projectSubdir, projectSubdir)
	require.JSONEq(t, `{}`, second)

	armed := filepath.Join(projectSubdir, ".substrate", "watch", "armed")
	require.NoError(t, os.WriteFile(armed, []byte("armed"), 0o600))
	third := run("false", nested, projectSubdir, projectSubdir)
	require.JSONEq(t, `{}`, third)

	noEnvironment := run("false", nested, "", workspace)
	require.Contains(t, noEnvironment, `"decision":"block"`)
	require.FileExists(t, filepath.Join(
		workspace, ".substrate", "watch", "nudged-same-session",
	))
	require.NoFileExists(t, filepath.Join(
		nested, ".substrate", "watch", "nudged-same-session",
	))
}
