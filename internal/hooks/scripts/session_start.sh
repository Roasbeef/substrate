#!/bin/bash
# Subtrate SessionStart hook - heartbeat + check for mail
#
# This hook runs when a Claude Code session starts. It:
# 1. Exports CLAUDE_SESSION_ID to the environment (via CLAUDE_ENV_FILE)
# 2. Sends a heartbeat to mark the agent as active
# 3. Checks for any pending messages to inject as context
#
# Output format: plain text for context injection

# Read hook input from stdin to get session_id.
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id // empty')
cwd=$(echo "$input" | jq -r '.cwd // empty')
is_codex=$(echo "$input" | jq -r 'if has("model") then "true" else "false" end')

# Hook input is authoritative. Environment fallbacks also make the script easy
# to test manually in either supported agent.
if [ -z "$session_id" ]; then
    session_id="${CLAUDE_SESSION_ID:-$CODEX_SESSION_ID}"
fi

project_dir="${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-$cwd}}"
agent_args=()
if [ -n "$session_id" ]; then
    agent_args+=(--session-id "$session_id")
fi
if [ -n "$project_dir" ]; then
    agent_args+=(--project "$project_dir")
fi

# Export CLAUDE_SESSION_ID via CLAUDE_ENV_FILE if available.
# This makes the session ID available to the agent during the session.
if [ -n "$session_id" ] && [ -n "$CLAUDE_ENV_FILE" ]; then
    echo "CLAUDE_SESSION_ID=$session_id" >> "$CLAUDE_ENV_FILE"
fi

# Send heartbeat to mark session start.
substrate heartbeat --session-start "${agent_args[@]}" \
    --format context 2>/dev/null || true

# Poll for new messages (non-blocking).
# Output is injected as context at session start.
result=$(substrate poll "${agent_args[@]}" --format context --quiet \
    2>/dev/null || echo "")

if [ -n "$result" ]; then
    echo "$result"
fi

# Instruct the agent to arm the mail watcher. Background tasks die with
# the Claude Code process, so every new session needs a fresh watcher.
# Skip the instruction if one is somehow already armed for this agent.
# Array form prevents word-splitting of odd session IDs.
if [ "$is_codex" != "true" ] && [ -z "$CODEX_SESSION_ID" ] && \
    ! substrate watch --check "${agent_args[@]}" >/dev/null 2>&1; then
    echo ""
    echo "[Subtrate Watch] Arm your mail watcher: run \`substrate watch --session-id ${session_id:-\${CLAUDE_SESSION_ID:-\$CODEX_SESSION_ID}}\` as a background shell command. It blocks until mail arrives, then exits with a digest. Re-arm it after handling each wake."
fi
