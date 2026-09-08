package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexHookDefinitions(t *testing.T) {
	require.Len(t, CodexHookDefinitions, 5)
	require.NotContains(t, CodexHookDefinitions, "Notification")
	require.NotContains(t, CodexHookDefinitions, "PermissionRequest")
	require.NotContains(t, CodexHookDefinitions, "PostToolUse")

	for event, entry := range CodexHookDefinitions {
		require.Len(t, entry.Hooks, 1, event)
		require.Contains(t, entry.Hooks[0].Command,
			"~/.codex/hooks/substrate/", event)
	}
	require.Equal(t, 600,
		CodexHookDefinitions["Stop"].Hooks[0].Timeout)
}

func TestInstallCodexHooksIdempotentAndPreservesExisting(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: map[string][]HookEntry{
			"SessionStart": {
				{
					Matcher: "startup",
					Hooks: []HookCommand{{
						Type:    "command",
						Command: "/custom/session-start.sh",
					}},
				},
			},
		},
	}

	InstallCodexHooks(settings)
	InstallCodexHooks(settings)

	require.Len(t, settings.Hooks["SessionStart"], 2)
	for event := range CodexHookDefinitions {
		require.True(t, slicesContainSubstrateHook(settings.Hooks[event]),
			event)
	}

	UninstallCodexHooks(settings)
	require.Len(t, settings.Hooks["SessionStart"], 1)
	require.Equal(t, "/custom/session-start.sh",
		settings.Hooks["SessionStart"][0].Hooks[0].Command)
}

func TestHookFilePreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	original := `{
  "description": "existing hooks",
  "custom": {"enabled": true},
  "hooks": {
    "PostToolUse": [{
      "matcher": "Bash",
      "customEntry": "keep",
      "hooks": [{
        "type": "command",
        "command": "/custom/review.sh",
        "timeout": 12,
        "async": true,
        "statusMessage": "Reviewing"
      }]
    }]
  }
}`
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	settings, err := LoadSettingsFile(path)
	require.NoError(t, err)
	InstallCodexHooks(settings)
	require.NoError(t, SaveSettingsFile(path, settings))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	require.Equal(t, "existing hooks", result["description"])
	require.Equal(t, map[string]any{"enabled": true}, result["custom"])

	hookMap := result["hooks"].(map[string]any)
	postToolUse := hookMap["PostToolUse"].([]any)[0].(map[string]any)
	require.Equal(t, "keep", postToolUse["customEntry"])
	handler := postToolUse["hooks"].([]any)[0].(map[string]any)
	require.Equal(t, true, handler["async"])
	require.Equal(t, "Reviewing", handler["statusMessage"])
}

func slicesContainSubstrateHook(entries []HookEntry) bool {
	for _, entry := range entries {
		if isSubstrateHook(entry) {
			return true
		}
	}

	return false
}
