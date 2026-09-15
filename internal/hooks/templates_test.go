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

// TestStopScriptNamesProjectForArmAndCheck is the regression test for a
// lease the hook could never find. The watcher is armed from the agent's
// shell, which gets no project environment, while this hook has one. When
// each side derived the project for itself, they agreed only when the
// project directory happened to be the nearest .git-directory ancestor of
// the agent's cwd — false inside a linked worktree, whose .git is a file,
// and false for a session started in a subdirectory. The lease then went
// one place and --check looked in another, so the hook nudged forever and
// each re-arm answered "already armed".
//
// The fix is that neither side derives anything: the hook names the
// project on the check and in the arming instruction it hands back. This
// asserts those two values are the same, which is the whole invariant.
func TestStopScriptNamesProjectForArmAndCheck(t *testing.T) {
	// The script is executed for real, so it needs its interpreter and
	// the JSON parser it pipes through. Skipping names the dependency
	// instead of failing on an empty payload further down.
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}

	workspace := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workspace, ".git"), 0o755))

	// A worktree-shaped layout: the agent works below the .git root, so a
	// derived root and the declared project directory disagree.
	nested := filepath.Join(workspace, "nested", "deeper")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	hookHome := t.TempDir()
	binDir := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "stop.sh")
	argsLog := filepath.Join(t.TempDir(), "check-args")

	require.NoError(t, os.WriteFile(scriptPath, []byte(StopScript), 0o755))

	// The fake CLI records the arguments it was handed rather than
	// consulting an expectation the test supplies, so the assertions below
	// describe the script's behavior and not their own setup.
	require.NoError(t, os.WriteFile(
		filepath.Join(binDir, "substrate"),
		[]byte(`#!/bin/sh
case "$1" in
heartbeat) exit 0 ;;
poll) printf '{"decision":"allow"}' ;;
watch)
  shift
  printf '%s\n' "$*" > "$CHECK_ARGS_LOG"
  [ -f "$ARMED_MARKER" ] && exit 0
  exit 1
  ;;
esac
exit 1
`),
		0o755,
	))

	run := func(active, dir, project, armedMarker string) string {
		cmd := exec.Command("bash", scriptPath)
		cmd.Dir = dir
		cmd.Env = []string{
			"HOME=" + hookHome,
			"PATH=" + binDir + ":" + os.Getenv("PATH"),
			"CHECK_ARGS_LOG=" + argsLog,
			"ARMED_MARKER=" + armedMarker,
		}
		if project != "" {
			cmd.Env = append(cmd.Env, "CLAUDE_PROJECT_DIR="+project)
		}
		cmd.Stdin = strings.NewReader(
			`{"session_id":"same-session","stop_hook_active":` +
				active + `}`,
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		return string(output)
	}

	// No watcher armed: the hook blocks once and tells the agent to arm.
	first := run("false", nested, workspace, "")
	require.Contains(t, first, `"decision":"block"`)

	checkArgs, err := os.ReadFile(argsLog)
	require.NoError(t, err)

	// The check named the project rather than deriving it from the cwd.
	require.Contains(t, string(checkArgs), "--project "+workspace)
	require.NotContains(t, string(checkArgs), nested)

	// The arming instruction names the same project, so the lease the
	// agent creates is the one this check just looked for. The path is
	// quoted because the agent runs this text as a shell command and a
	// project directory may contain spaces.
	require.Contains(t, first, "--project '"+workspace+"'")

	// The stamp is hook-local state and follows the same directory.
	require.FileExists(t, filepath.Join(
		workspace, ".substrate", "watch", "nudged-same-session",
	))
	require.NoFileExists(t, filepath.Join(
		hookHome, ".subtrate", "watch", "nudged-same-session",
	))

	// The hook creates that directory, so the hook also makes it ignore
	// itself — the CLI may not have run yet, or may predate this.
	ignored, err := os.ReadFile(
		filepath.Join(workspace, ".substrate", ".gitignore"),
	)
	require.NoError(t, err)
	require.Equal(t, "*\n", string(ignored))

	// Second stop in the same cycle: nudge already fired, so allow exit
	// rather than stack another block.
	require.JSONEq(t, `{}`, run("true", nested, workspace, ""))

	// Once a watcher is armed the hook allows the exit outright.
	armed := filepath.Join(t.TempDir(), "armed")
	require.NoError(t, os.WriteFile(armed, []byte("armed"), 0o600))
	require.JSONEq(t, `{}`, run("false", nested, workspace, armed))
}
