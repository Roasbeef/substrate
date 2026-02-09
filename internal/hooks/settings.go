package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ClaudeSettings represents the structure of ~/.claude/settings.json.
type ClaudeSettings struct {
	Hooks   map[string][]HookEntry `json:"hooks,omitempty"`
	Other   map[string]any         `json:"-"` // Preserve other settings
	rawData map[string]any         // Keep original data for merge
}

// HookEntry represents a hook configuration in settings.json.
type HookEntry struct {
	Matcher string        `json:"matcher"`
	Hooks   []HookCommand `json:"hooks"`
}

// HookCommand represents a single hook command.
type HookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// substrateHookID is used to identify Subtrate hooks in settings.json.
const substrateHookID = "substrate"

// HookDefinitions defines all Subtrate hooks to install.
var HookDefinitions = map[string]HookEntry{
	"SessionStart": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/session_start.sh",
		}},
	},
	"UserPromptSubmit": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/user_prompt.sh",
		}},
	},
	"Stop": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/stop.sh",
			Timeout: 600,
		}},
	},
	"SubagentStop": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/subagent_stop.sh",
		}},
	},
	"PreCompact": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/pre_compact.sh",
		}},
	},
	"Notification": {
		Matcher: "",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/notification.sh",
			Timeout: 10,
		}},
	},
}

// PlanHookDefinitions defines hooks for plan mode integration.
// PostToolUse tracks plan file writes; PreToolUse intercepts ExitPlanMode
// to submit plans for review before proceeding.
var PlanHookDefinitions = map[string]HookEntry{
	"PostToolUse": {
		Matcher: "Write",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/posttooluse_plan.sh",
		}},
	},
	"PreToolUse": {
		Matcher: "ExitPlanMode",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/pretooluse_plan.sh",
			Timeout: 600,
		}},
	},
}

// TaskHookDefinitions defines PostToolUse hooks for task tool sync.
// These are installed separately when task sync is enabled.
var TaskHookDefinitions = map[string]HookEntry{
	"PostToolUse": {
		Matcher: "TaskCreate|TaskUpdate|TaskList|TaskGet",
		Hooks: []HookCommand{{
			Type:    "command",
			Command: "~/.claude/hooks/substrate/task_sync.sh",
		}},
	},
}

// LoadSettings loads the Claude settings file.
func LoadSettings(claudeDir string) (*ClaudeSettings, error) {
	settingsPath := filepath.Join(claudeDir, "settings.json")

	settings := &ClaudeSettings{
		Hooks:   make(map[string][]HookEntry),
		rawData: make(map[string]any),
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	if err := json.Unmarshal(data, &settings.rawData); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	// Parse hooks section if present.
	if hooksRaw, ok := settings.rawData["hooks"].(map[string]any); ok {
		for event, entries := range hooksRaw {
			// Each event can have an array of hook entries.
			entriesArr, ok := entries.([]any)
			if !ok {
				continue
			}

			var hookEntries []HookEntry
			for _, entryRaw := range entriesArr {
				entryMap, ok := entryRaw.(map[string]any)
				if !ok {
					continue
				}

				entry := HookEntry{
					Matcher: getStringField(entryMap, "matcher"),
				}

				// Parse hooks array within entry.
				if hooksArr, ok := entryMap["hooks"].([]any); ok {
					for _, hookRaw := range hooksArr {
						hookMap, ok := hookRaw.(map[string]any)
						if !ok {
							continue
						}
						entry.Hooks = append(entry.Hooks, HookCommand{
							Type:    getStringField(hookMap, "type"),
							Command: getStringField(hookMap, "command"),
							Timeout: getIntField(hookMap, "timeout"),
						})
					}
				}

				hookEntries = append(hookEntries, entry)
			}
			settings.Hooks[event] = hookEntries
		}
	}

	return settings, nil
}

// SaveSettings saves the Claude settings file.
func SaveSettings(claudeDir string, settings *ClaudeSettings) error {
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Merge hooks back into raw data.
	if settings.rawData == nil {
		settings.rawData = make(map[string]any)
	}

	// Convert hooks to raw format.
	hooksRaw := make(map[string]any)
	for event, entries := range settings.Hooks {
		entriesRaw := make([]any, 0, len(entries))
		for _, entry := range entries {
			entryMap := map[string]any{
				"matcher": entry.Matcher,
			}

			hooksArr := make([]any, 0, len(entry.Hooks))
			for _, hook := range entry.Hooks {
				hookMap := map[string]any{
					"type":    hook.Type,
					"command": hook.Command,
				}
				if hook.Timeout > 0 {
					hookMap["timeout"] = hook.Timeout
				}
				hooksArr = append(hooksArr, hookMap)
			}
			entryMap["hooks"] = hooksArr

			entriesRaw = append(entriesRaw, entryMap)
		}
		hooksRaw[event] = entriesRaw
	}
	settings.rawData["hooks"] = hooksRaw

	data, err := json.MarshalIndent(settings.rawData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Ensure directory exists.
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	return nil
}

// InstallHooks adds Subtrate hooks to the settings.
// This appends to existing hooks rather than replacing them.
func InstallHooks(settings *ClaudeSettings) {
	for event, hookDef := range HookDefinitions {
		// Check if we already have a Subtrate hook for this event.
		entries := settings.Hooks[event]
		alreadyInstalled := slices.ContainsFunc(entries, isSubstrateHook)

		if !alreadyInstalled {
			settings.Hooks[event] = append(entries, hookDef)
		}
	}
}

// UninstallHooks removes Subtrate hooks from the settings.
func UninstallHooks(settings *ClaudeSettings) {
	for event, entries := range settings.Hooks {
		filtered := make([]HookEntry, 0, len(entries))
		for _, entry := range entries {
			if !isSubstrateHook(entry) {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) > 0 {
			settings.Hooks[event] = filtered
		} else {
			delete(settings.Hooks, event)
		}
	}
}

// IsInstalled checks if Subtrate hooks are installed.
func IsInstalled(settings *ClaudeSettings) bool {
	// Check if at least the SessionStart hook is present.
	entries, ok := settings.Hooks["SessionStart"]
	if !ok {
		return false
	}

	return slices.ContainsFunc(entries, isSubstrateHook)
}

// GetInstalledHookEvents returns which events have Subtrate hooks installed.
func GetInstalledHookEvents(settings *ClaudeSettings) []string {
	var events []string
	for event, entries := range settings.Hooks {
		if slices.ContainsFunc(entries, isSubstrateHook) {
			events = append(events, event)
		}
	}
	return events
}

// InstallTaskHooks adds task sync hooks to the settings.
func InstallTaskHooks(settings *ClaudeSettings) {
	for event, hookDef := range TaskHookDefinitions {
		entries := settings.Hooks[event]
		alreadyInstalled := slices.ContainsFunc(entries, isTaskSyncHook)

		if !alreadyInstalled {
			settings.Hooks[event] = append(entries, hookDef)
		}
	}
}

// UninstallTaskHooks removes task sync hooks from the settings.
func UninstallTaskHooks(settings *ClaudeSettings) {
	for event, entries := range settings.Hooks {
		filtered := make([]HookEntry, 0, len(entries))
		for _, entry := range entries {
			if !isTaskSyncHook(entry) {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) > 0 {
			settings.Hooks[event] = filtered
		} else {
			delete(settings.Hooks, event)
		}
	}
}

// IsTaskHooksInstalled checks if task sync hooks are installed.
func IsTaskHooksInstalled(settings *ClaudeSettings) bool {
	entries, ok := settings.Hooks["PostToolUse"]
	if !ok {
		return false
	}

	return slices.ContainsFunc(entries, isTaskSyncHook)
}

// InstallPlanHooks adds plan mode hooks to the settings.
func InstallPlanHooks(settings *ClaudeSettings) {
	for event, hookDef := range PlanHookDefinitions {
		entries := settings.Hooks[event]
		alreadyInstalled := slices.ContainsFunc(entries, isPlanHook)

		if !alreadyInstalled {
			settings.Hooks[event] = append(entries, hookDef)
		}
	}
}

// UninstallPlanHooks removes plan mode hooks from the settings.
func UninstallPlanHooks(settings *ClaudeSettings) {
	for event, entries := range settings.Hooks {
		filtered := make([]HookEntry, 0, len(entries))
		for _, entry := range entries {
			if !isPlanHook(entry) {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) > 0 {
			settings.Hooks[event] = filtered
		} else {
			delete(settings.Hooks, event)
		}
	}
}

// IsPlanHooksInstalled checks if plan mode hooks are installed.
func IsPlanHooksInstalled(settings *ClaudeSettings) bool {
	// Check if the PreToolUse ExitPlanMode hook is present.
	entries, ok := settings.Hooks["PreToolUse"]
	if !ok {
		return false
	}

	return slices.ContainsFunc(entries, isPlanHook)
}

// isPlanHook checks if a hook entry is a plan mode hook.
func isPlanHook(entry HookEntry) bool {
	for _, hook := range entry.Hooks {
		if strings.Contains(hook.Command, "posttooluse_plan.sh") ||
			strings.Contains(hook.Command, "pretooluse_plan.sh") {

			return true
		}
	}
	return false
}

// isTaskSyncHook checks if a hook entry is a task sync hook.
func isTaskSyncHook(entry HookEntry) bool {
	for _, hook := range entry.Hooks {
		if strings.Contains(hook.Command, "task_sync.sh") {
			return true
		}
	}
	return false
}

// isSubstrateHook checks if a hook entry is a Subtrate hook.
func isSubstrateHook(entry HookEntry) bool {
	for _, hook := range entry.Hooks {
		// Check if the command references our hook scripts.
		if strings.Contains(hook.Command, "hooks/substrate/") ||
			strings.Contains(hook.Command, substrateHookID) {
			return true
		}
	}
	return false
}

// getStringField safely gets a string field from a map.
func getStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getIntField safely gets an int field from a map. JSON numbers
// unmarshal as float64, so we handle that conversion.
func getIntField(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
