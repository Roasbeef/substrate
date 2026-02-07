#!/bin/bash
# task_sync.sh - PostToolUse hook for syncing Claude Code task operations to Substrate.
# This script is called after TaskCreate, TaskUpdate, TaskList, or TaskGet tools execute.
#
# Environment variables (provided by Claude Code):
#   TOOL_INPUT  - JSON input parameters passed to the tool
#   TOOL_OUTPUT - JSON output/result from the tool execution
#   TOOL_NAME   - Name of the tool (TaskCreate, TaskUpdate, etc.)
#
# This hook runs asynchronously to avoid blocking the agent.

set -euo pipefail

# Get tool name from environment or extract from context.
TOOL="${TOOL_NAME:-}"

# Get task list ID from environment (set by CLAUDE_CODE_TASK_LIST_ID).
LIST_ID="${CLAUDE_CODE_TASK_LIST_ID:-}"

# Get session ID if available.
SESSION="${CLAUDE_SESSION_ID:-}"

# If no list ID, skip sync (tasks not configured).
if [[ -z "$LIST_ID" ]]; then
    exit 0
fi

# Run sync in background to avoid blocking.
(
    # Determine substrate command location.
    SUBSTRATE="${SUBSTRATE_CLI:-substrate}"

    # Check if substrate CLI is available.
    if ! command -v "$SUBSTRATE" &>/dev/null; then
        # Try common locations.
        if [[ -x "$HOME/go/bin/substrate" ]]; then
            SUBSTRATE="$HOME/go/bin/substrate"
        elif [[ -x "/usr/local/bin/substrate" ]]; then
            SUBSTRATE="/usr/local/bin/substrate"
        else
            exit 0
        fi
    fi

    # Route based on tool name.
    case "$TOOL" in
        TaskCreate)
            # Sync the newly created task.
            # TOOL_OUTPUT contains the new task with its ID.
            echo "$TOOL_OUTPUT" | "$SUBSTRATE" tasks hook-sync --tool create --list "$LIST_ID" 2>/dev/null || true
            ;;
        TaskUpdate)
            # Sync the updated task.
            echo "$TOOL_INPUT" | "$SUBSTRATE" tasks hook-sync --tool update --list "$LIST_ID" 2>/dev/null || true
            ;;
        TaskList)
            # Sync all tasks from the list response.
            echo "$TOOL_OUTPUT" | "$SUBSTRATE" tasks hook-sync --tool list --list "$LIST_ID" 2>/dev/null || true
            ;;
        TaskGet)
            # Sync the retrieved task.
            echo "$TOOL_OUTPUT" | "$SUBSTRATE" tasks hook-sync --tool get --list "$LIST_ID" 2>/dev/null || true
            ;;
        *)
            # Unknown tool, ignore.
            ;;
    esac
) &

# Exit immediately, sync runs in background.
exit 0
