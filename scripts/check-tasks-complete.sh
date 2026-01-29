#!/bin/bash
# check-tasks-complete.sh
# Checks if all tasks for this project are completed.
# Outputs JSON for Claude Code Stop hook decision control.

# The project ID for subtrate (can be detected from cwd hash if needed).
PROJECT_ID="8294fd83-dc5f-4027-9423-6ef8b8cb194d"
TASKS_DIR="$HOME/.claude/tasks/$PROJECT_ID"

if [ ! -d "$TASKS_DIR" ]; then
    # No tasks directory - allow stop (no JSON output = undefined decision).
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
    subject=$(jq -r '.subject' "$task_file" 2>/dev/null)
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

# Allow stop - no output means undefined decision (allowed).
exit 0
