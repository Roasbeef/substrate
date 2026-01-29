#!/bin/bash
# check-tasks-complete.sh
# Checks if all tasks for this project are completed.
# Exits with error if any tasks are incomplete.

# The project ID for subtrate (can be detected from cwd hash if needed).
PROJECT_ID="8294fd83-dc5f-4027-9423-6ef8b8cb194d"
TASKS_DIR="$HOME/.claude/tasks/$PROJECT_ID"

if [ ! -d "$TASKS_DIR" ]; then
    echo "No tasks directory found, allowing stop."
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
        incomplete_list="$incomplete_list\n  #$id [$status] $subject"
    fi
done

if [ $incomplete -gt 0 ]; then
    echo "STOP BLOCKED: $incomplete incomplete task(s):"
    echo -e "$incomplete_list"
    echo ""
    echo "Complete remaining tasks before stopping session."
    exit 1
fi

echo "All tasks completed. Session may stop."
exit 0
