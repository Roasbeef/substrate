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

func TestCodexStopHookLongPollsAndContinues(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "model": "gpt-test",
  "stop_hook_active": false
}`

	output := runHookScript(t, "stop.sh", input, fakeDir, logPath)
	var decision map[string]any
	require.NoError(t, json.Unmarshal(output, &decision))
	require.Equal(t, "block", decision["decision"])

	logData, err := os.ReadFile(logPath)
	require.NoError(t, err)
	log := string(logData)
	require.Contains(t, log, "heartbeat --session-id codex-session --project /work/project")
	require.Contains(t, log, "poll --session-id codex-session --project /work/project --wait=570s")
}

func TestCodexSessionStartDoesNotRequestClaudeWatcher(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "model": "gpt-test",
  "source": "startup"
}`

	output := runHookScript(t, "session_start.sh", input, fakeDir, logPath)
	require.NotContains(t, string(output), "Arm your mail watcher")

	logData, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.NotContains(t, string(logData), "watch --check")
}

func TestCodexPreCompactReturnsValidJSON(t *testing.T) {
	fakeDir, logPath := installFakeSubstrate(t)
	input := `{
  "session_id": "codex-session",
  "cwd": "/work/project",
  "model": "gpt-test",
  "trigger": "auto"
}`

	output := runHookScript(t, "pre_compact.sh", input, fakeDir, logPath)
	var result map[string]any
	require.NoError(t, json.Unmarshal(output, &result))
	require.Empty(t, result)
}

func installFakeSubstrate(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := `#!/bin/bash
printf '%s\n' "$*" >> "$FAKE_SUBSTRATE_LOG"
if [ "$1" = "poll" ]; then
    echo '{}'
fi
`
	path := filepath.Join(dir, "substrate")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	return dir, logPath
}

func runHookScript(t *testing.T, name, input, fakeDir, logPath string) []byte {
	t.Helper()
	cmd := exec.Command("/bin/bash", filepath.Join("scripts", name))
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = hookTestEnv(fakeDir, logPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	require.NoError(t, err, stderr.String())

	return output
}

func hookTestEnv(fakeDir, logPath string) []string {
	result := make([]string, 0, len(os.Environ())+3)
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
