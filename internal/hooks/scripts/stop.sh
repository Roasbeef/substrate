#!/bin/bash
# Subtrate Stop hook - Mail-First Persistent Agent Pattern
#
# Priority order:
# 1. Quick mail check - if mail exists, block immediately
# 2. Check Tasks - if incomplete tasks exist, block immediately
# 3. Long poll - keep agent alive, continuously checking for work
#
# Key behaviors:
# - Checks mail before tasks (mail is more actionable)
# - Long-polls for 55s (under 60s hook timeout)
# - Always outputs {"decision": "block"} to stay alive
# - User can force exit with Ctrl+C (bypasses hooks)
#
# Output format: JSON for Stop hook decision

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id // empty')

# Build session ID args if available (critical for agent identity resolution).
session_args=""
if [ -n "$session_id" ]; then
    session_args="--session-id $session_id"
fi

# ============================================================================
# Step 1: Quick mail check
# ============================================================================

# Record heartbeat (best effort)
substrate heartbeat $session_args --format context 2>/dev/null || true

# Quick (non-blocking) check for mail
quick_result=$(substrate poll $session_args --format hook --quiet 2>/dev/null || echo '{"decision": null}')
quick_decision=$(echo "$quick_result" | jq -r '.decision // empty')

if [ "$quick_decision" = "block" ]; then
    # Mail exists - output the result immediately
    echo "$quick_result"
    exit 0
fi

# ============================================================================
# Step 2: Check for incomplete tasks
# ============================================================================

# Function to count incomplete tasks for a session.
count_incomplete_tasks() {
    local task_dir="$HOME/.claude/tasks/$1"

    if [ ! -d "$task_dir" ]; then
        echo "0"
        return
    fi

    local count=0
    for task_file in "$task_dir"/*.json; do
        [ -f "$task_file" ] || continue

        # Check if status is not "completed"
        local status=$(jq -r '.status // "pending"' "$task_file" 2>/dev/null)
        if [ "$status" != "completed" ]; then
            count=$((count + 1))
        fi
    done

    echo "$count"
}

# Function to list incomplete tasks for a session.
list_incomplete_tasks() {
    local task_dir="$HOME/.claude/tasks/$1"
    local output=""

    if [ ! -d "$task_dir" ]; then
        return
    fi

    for task_file in "$task_dir"/*.json; do
        [ -f "$task_file" ] || continue

        local status=$(jq -r '.status // "pending"' "$task_file" 2>/dev/null)
        if [ "$status" != "completed" ]; then
            local id=$(jq -r '.id' "$task_file" 2>/dev/null)
            local subject=$(jq -r '.subject' "$task_file" 2>/dev/null | head -c 50)
            output="${output}#${id} [${status}], "
        fi
    done

    echo "$output"
}

if [ -n "$session_id" ]; then
    incomplete_count=$(count_incomplete_tasks "$session_id")

    if [ "$incomplete_count" -gt 0 ]; then
        task_list=$(list_incomplete_tasks "$session_id")

        cat <<EOF
{"decision": "block", "reason": "${incomplete_count} incomplete task(s): ${task_list}Complete ALL tasks before stopping."}
EOF
        exit 0
    fi
fi

# ============================================================================
# Step 3: Long poll to keep agent alive
# ============================================================================

# No mail, no tasks - do a longer poll to keep agent alive
# --always-block ensures we output block decision even with no messages.
# This keeps the agent alive indefinitely, continuously checking for work.
substrate poll $session_args --wait=55s --format hook --always-block 2>/dev/null || \
    echo '{"decision": "block", "reason": "Error resolving agent identity. Agent staying alive."}'
