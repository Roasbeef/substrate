#!/bin/bash
# check-tasks-complete.sh
# Checks if all tasks for this project are completed.
# Outputs JSON for Claude Code Stop hook decision control.
#
# Hook Input (stdin JSON):
#   session_id: The current session UUID
#   stop_hook_active: true if this is a retry after previous block
#
# Output (stdout JSON):
#   decision: "block" to prevent stopping, null/omit to allow
#   reason: Message shown to Claude when blocked

# Read hook input from stdin.
input=$(cat)

# Extract session_id from hook input.
SESSION_ID=$(echo "$input" | jq -r '.session_id // empty' 2>/dev/null)
STOP_HOOK_ACTIVE=$(echo "$input" | jq -r '.stop_hook_active // false' 2>/dev/null)

# If no session_id, allow stop.
if [ -z "$SESSION_ID" ]; then
    exit 0
fi

TASKS_DIR="$HOME/.claude/tasks/$SESSION_ID"

# If stop_hook_active is true, we already blocked once and Claude processed.
# Allow exit this time to prevent infinite loop.
if [ "$STOP_HOOK_ACTIVE" = "true" ]; then
    exit 0
fi

# If no tasks directory, allow stop.
if [ ! -d "$TASKS_DIR" ]; then
    exit 0
fi

# Count incomplete tasks.
incomplete=0
incomplete_list=""

for task_file in "$TASKS_DIR"/*.json; do
    if [ ! -f "$task_file" ]; then
        continue
    fi

    status=$(jq -r '.status' "$task_file" 2>/dev/null)
    id=$(jq -r '.id' "$task_file" 2>/dev/null)

    if [ "$status" != "completed" ]; then
        incomplete=$((incomplete + 1))
        if [ -n "$incomplete_list" ]; then
            incomplete_list="$incomplete_list, #$id [$status]"
        else
            incomplete_list="#$id [$status]"
        fi
    fi
done

if [ $incomplete -gt 0 ]; then
    # Block stopping - output JSON with decision and reason.
    cat << EOF
{
  "decision": "block",
  "reason": "$incomplete incomplete task(s): $incomplete_list. Complete remaining tasks before stopping session."
}
EOF
    exit 0
fi

# All tasks complete - allow stop.
exit 0
