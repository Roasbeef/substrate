package hooks

import (
	_ "embed"
)

// Hook script templates embedded in the binary.
// These are installed to ~/.claude/hooks/substrate/ via the hooks install command.

//go:embed scripts/session_start.sh
var SessionStartScript string

//go:embed scripts/stop.sh
var StopScript string

//go:embed scripts/subagent_stop.sh
var SubagentStopScript string

//go:embed scripts/user_prompt.sh
var UserPromptScript string

//go:embed scripts/pre_compact.sh
var PreCompactScript string

//go:embed scripts/task_sync.sh
var TaskSyncScript string

//go:embed scripts/notification.sh
var NotificationScript string

//go:embed scripts/posttooluse_plan.sh
var PostToolUsePlanScript string

//go:embed scripts/pretooluse_plan.sh
var PreToolUsePlanScript string

// ScriptNames maps script identifiers to their filenames.
var ScriptNames = map[string]string{
	"session_start":    "session_start.sh",
	"stop":             "stop.sh",
	"subagent_stop":    "subagent_stop.sh",
	"user_prompt":      "user_prompt.sh",
	"pre_compact":      "pre_compact.sh",
	"task_sync":        "task_sync.sh",
	"notification":     "notification.sh",
	"posttooluse_plan": "posttooluse_plan.sh",
	"pretooluse_plan":  "pretooluse_plan.sh",
}

// GetScript returns the embedded script content by name.
func GetScript(name string) string {
	switch name {
	case "session_start":
		return SessionStartScript
	case "stop":
		return StopScript
	case "subagent_stop":
		return SubagentStopScript
	case "user_prompt":
		return UserPromptScript
	case "pre_compact":
		return PreCompactScript
	case "task_sync":
		return TaskSyncScript
	case "notification":
		return NotificationScript
	case "posttooluse_plan":
		return PostToolUsePlanScript
	case "pretooluse_plan":
		return PreToolUsePlanScript
	default:
		return ""
	}
}

// AllScripts returns all scripts as name -> content map.
func AllScripts() map[string]string {
	return map[string]string{
		"session_start":    SessionStartScript,
		"stop":             StopScript,
		"subagent_stop":    SubagentStopScript,
		"user_prompt":      UserPromptScript,
		"pre_compact":      PreCompactScript,
		"task_sync":        TaskSyncScript,
		"notification":     NotificationScript,
		"posttooluse_plan": PostToolUsePlanScript,
		"pretooluse_plan":  PreToolUsePlanScript,
	}
}
