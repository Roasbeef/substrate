package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCodexStopHookLongPollsAndContinues checks that the Codex Stop path keeps
// the agent alive through the CLI's persistent-agent mode rather than the
// watcher-arming flow Codex cannot run.
func TestCodexStopHookLongPollsAndContinues(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "stop_hook_active": false
}`

	output := runCodexHookScript(t, "stop.sh", input, fakeDir, logPath)
	var decision map[string]any
	require.NoError(t, json.Unmarshal(output, &decision))
	require.Equal(t, "block", decision["decision"])

	log := readCallLog(t, logPath)
	require.Contains(t, log,
		"heartbeat --session-id codex-session --project /work/project")
	require.Contains(t, log, "poll --session-id codex-session "+
		"--project /work/project --wait=570s --format hook "+
		"--always-block")
}

// TestCodexSessionStartDoesNotRequestClaudeWatcher checks that Codex sessions
// are never told to arm a `substrate watch` background task, which depends on
// a Claude Code feature Codex does not have.
func TestCodexSessionStartDoesNotRequestClaudeWatcher(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "source": "startup"
}`

	output := runCodexHookScript(
		t, "session_start.sh", input, fakeDir, logPath,
	)
	require.NotContains(t, string(output), "Arm your mail watcher")
	require.NotContains(t, readCallLog(t, logPath), "watch --check")
}

// TestCodexPreCompactReturnsValidJSON checks that the Codex PreCompact path
// emits a single JSON object, since Codex rejects the plain status text the
// Claude path injects as context.
func TestCodexPreCompactReturnsValidJSON(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "trigger": "auto"
}`

	output := runCodexHookScript(t, "pre_compact.sh", input, fakeDir, logPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(output, &result))
	require.Empty(t, result)
}

// TestClaudeStopHookIgnoresCodexEnvironment pins down why the host agent is
// selected by an explicit flag rather than by sniffing the environment. A
// Claude Code session launched from a Codex agent inherits CODEX_SESSION_ID,
// and taking the Codex branch there would swap the watcher-arming flow for an
// in-hook long poll that traps the session.
func TestClaudeStopHookIgnoresCodexEnvironment(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "claude-session",
  "cwd": "/work/project",
  "stop_hook_active": false
}`

	env := append(
		hookTestEnv(fakeDir, logPath),
		"CODEX_SESSION_ID=codex-session",
		"HOME="+t.TempDir(),
	)
	output := runHookScript(t, "stop.sh", input, nil, env)

	log := readCallLog(t, logPath)
	require.NotContains(t, log, "--always-block")
	require.Contains(t, log, "watch --check")

	// With no watcher armed, the Claude path blocks once with arming
	// instructions rather than long-polling in the hook.
	var decision map[string]any
	require.NoError(t, json.Unmarshal(output, &decision))
	require.Equal(t, "block", decision["decision"])
	require.Contains(t, decision["reason"], "No mail watcher is armed")
}

// installFakeSubstrate places a stub `substrate` on PATH that records every
// invocation, so the tests can assert on the arguments a hook script builds.
// Every subcommand fails except the ones a script reads output from, which
// mirrors a session with no armed watcher and no pending mail.
func installFakeSubstrate(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := `#!/bin/bash
printf '%s\n' "$*" >> "$FAKE_SUBSTRATE_LOG"
case "$1" in
    poll)
        if [[ "$*" == *--always-block* ]]; then
            echo '{"decision":"block","reason":"No new messages."}'
        else
            echo '{}'
        fi
        ;;
    watch)
        # No watcher is armed for this session.
        exit 1
        ;;
esac
`
	path := filepath.Join(dir, "substrate")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	return dir, logPath
}

// runCodexHookScript runs a lifecycle script the way the Codex hook
// definitions invoke it, with the Codex target flag appended.
func runCodexHookScript(t *testing.T, name, input, fakeDir,
	logPath string,
) []byte {
	t.Helper()

	return runHookScript(
		t, name, input, []string{CodexTargetFlag},
		hookTestEnv(fakeDir, logPath),
	)
}

// runHookScript feeds a hook payload to a lifecycle script and returns its
// stdout, failing the test if the script exits non-zero.
func runHookScript(t *testing.T, name, input string, args,
	env []string,
) []byte {
	t.Helper()
	cmd := exec.Command(
		"/bin/bash", append([]string{filepath.Join("scripts", name)},
			args...)...,
	)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	require.NoError(t, err, stderr.String())

	return output
}

// readCallLog returns everything the fake `substrate` binary recorded.
func readCallLog(t *testing.T, logPath string) string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	return string(data)
}

// hookTestEnv builds a minimal environment for a hook script, stripping any
// agent session variables inherited from the test runner so the scripts see
// only the hook payload and the target flag.
func hookTestEnv(fakeDir, logPath string) []string {
	result := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PATH=") ||
			strings.HasPrefix(entry, "CODEX_SESSION_ID=") ||
			strings.HasPrefix(entry, "CLAUDE_SESSION_ID=") {

			continue
		}
		result = append(result, entry)
	}
	result = append(result,
		"PATH="+fakeDir+":/usr/bin:/bin",
		"FAKE_SUBSTRATE_LOG="+logPath,
		"CODEX_SESSION_ID=",
		"CLAUDE_SESSION_ID=",
	)

	return result
}
