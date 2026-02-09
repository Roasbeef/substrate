package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInstallPlanHooks verifies plan hooks are added to settings.
func TestInstallPlanHooks(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookEntry),
	}

	InstallPlanHooks(settings)

	// Should have PostToolUse and PreToolUse entries.
	postEntries, ok := settings.Hooks["PostToolUse"]
	require.True(t, ok, "PostToolUse should be present")
	require.Len(t, postEntries, 1)
	require.Equal(t, "Write", postEntries[0].Matcher)
	require.Contains(t, postEntries[0].Hooks[0].Command,
		"posttooluse_plan.sh",
	)

	preEntries, ok := settings.Hooks["PreToolUse"]
	require.True(t, ok, "PreToolUse should be present")
	require.Len(t, preEntries, 1)
	require.Equal(t, "ExitPlanMode", preEntries[0].Matcher)
	require.Contains(t, preEntries[0].Hooks[0].Command,
		"pretooluse_plan.sh",
	)
	require.Equal(t, 600, preEntries[0].Hooks[0].Timeout)
}

// TestInstallPlanHooksIdempotent verifies double-install is safe.
func TestInstallPlanHooksIdempotent(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookEntry),
	}

	InstallPlanHooks(settings)
	InstallPlanHooks(settings)

	// Should still have exactly one entry per event.
	require.Len(t, settings.Hooks["PostToolUse"], 1)
	require.Len(t, settings.Hooks["PreToolUse"], 1)
}

// TestInstallPlanHooksPreservesExisting verifies existing hooks are kept.
func TestInstallPlanHooksPreservesExisting(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: map[string][]HookEntry{
			"PostToolUse": {
				{
					Matcher: "Write",
					Hooks: []HookCommand{{
						Type:    "command",
						Command: "/custom/hook.sh",
					}},
				},
			},
		},
	}

	InstallPlanHooks(settings)

	// Should have both: existing + plan hook.
	entries := settings.Hooks["PostToolUse"]
	require.Len(t, entries, 2)
	require.Equal(t, "/custom/hook.sh", entries[0].Hooks[0].Command)
	require.Contains(t, entries[1].Hooks[0].Command,
		"posttooluse_plan.sh",
	)
}

// TestUninstallPlanHooks verifies plan hooks are removed.
func TestUninstallPlanHooks(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookEntry),
	}

	InstallPlanHooks(settings)
	require.True(t, IsPlanHooksInstalled(settings))

	UninstallPlanHooks(settings)
	require.False(t, IsPlanHooksInstalled(settings))

	// Events with no remaining hooks should be removed.
	_, hasPost := settings.Hooks["PostToolUse"]
	_, hasPre := settings.Hooks["PreToolUse"]
	require.False(t, hasPost, "PostToolUse should be removed")
	require.False(t, hasPre, "PreToolUse should be removed")
}

// TestUninstallPlanHooksPreservesOthers verifies non-plan hooks survive.
func TestUninstallPlanHooksPreservesOthers(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: map[string][]HookEntry{
			"PostToolUse": {
				{
					Matcher: "Write",
					Hooks: []HookCommand{{
						Type:    "command",
						Command: "/custom/hook.sh",
					}},
				},
			},
		},
	}

	InstallPlanHooks(settings)
	require.Len(t, settings.Hooks["PostToolUse"], 2)

	UninstallPlanHooks(settings)

	// Custom hook should survive.
	entries := settings.Hooks["PostToolUse"]
	require.Len(t, entries, 1)
	require.Equal(t, "/custom/hook.sh", entries[0].Hooks[0].Command)
}

// TestIsPlanHooksInstalled verifies installation detection.
func TestIsPlanHooksInstalled(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookEntry),
	}

	require.False(t, IsPlanHooksInstalled(settings))

	InstallPlanHooks(settings)
	require.True(t, IsPlanHooksInstalled(settings))
}

// TestIsPlanHook verifies plan hook identification.
func TestIsPlanHook(t *testing.T) {
	tests := []struct {
		name     string
		entry    HookEntry
		expected bool
	}{
		{
			name: "posttooluse plan hook",
			entry: HookEntry{
				Hooks: []HookCommand{{
					Command: "~/.claude/hooks/substrate/posttooluse_plan.sh",
				}},
			},
			expected: true,
		},
		{
			name: "pretooluse plan hook",
			entry: HookEntry{
				Hooks: []HookCommand{{
					Command: "~/.claude/hooks/substrate/pretooluse_plan.sh",
				}},
			},
			expected: true,
		},
		{
			name: "task sync hook",
			entry: HookEntry{
				Hooks: []HookCommand{{
					Command: "~/.claude/hooks/substrate/task_sync.sh",
				}},
			},
			expected: false,
		},
		{
			name: "custom hook",
			entry: HookEntry{
				Hooks: []HookCommand{{
					Command: "/custom/my_hook.sh",
				}},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isPlanHook(tc.entry))
		})
	}
}

// TestPlanHookDefinitions verifies the definition map is correct.
func TestPlanHookDefinitions(t *testing.T) {
	// Should have exactly two entries.
	require.Len(t, PlanHookDefinitions, 2)

	postDef, ok := PlanHookDefinitions["PostToolUse"]
	require.True(t, ok)
	require.Equal(t, "Write", postDef.Matcher)

	preDef, ok := PlanHookDefinitions["PreToolUse"]
	require.True(t, ok)
	require.Equal(t, "ExitPlanMode", preDef.Matcher)
	require.Equal(t, 600, preDef.Hooks[0].Timeout)
}

// TestPlanAndTaskHooksCoexist verifies both hook types can be installed.
func TestPlanAndTaskHooksCoexist(t *testing.T) {
	settings := &ClaudeSettings{
		Hooks: make(map[string][]HookEntry),
	}

	InstallPlanHooks(settings)
	InstallTaskHooks(settings)

	// Both should be installed.
	require.True(t, IsPlanHooksInstalled(settings))
	require.True(t, IsTaskHooksInstalled(settings))

	// PostToolUse should have both entries.
	entries := settings.Hooks["PostToolUse"]
	require.Len(t, entries, 2)

	// Uninstalling plan hooks should not affect task hooks.
	UninstallPlanHooks(settings)
	require.False(t, IsPlanHooksInstalled(settings))
	require.True(t, IsTaskHooksInstalled(settings))
}
