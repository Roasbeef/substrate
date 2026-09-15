#!/bin/bash
# Subtrate Stop hook - Arm-Once Watcher Pattern
#
# The persistence model is the background watcher (`substrate watch`),
# not the Stop hook. The watcher runs as a Claude Code background task;
# when mail arrives it exits, and the harness re-invokes the agent with
# the digest. The Stop hook's only job is to make sure a watcher is
# armed before the agent goes idle.
#
# Priority order:
# 1. Quick mail check - if mail exists, block with the digest (the agent
#    will read mail via tool calls, which resets Claude Code's
#    consecutive-block counter).
# 2. If a watcher is armed, allow exit - the agent will be woken by the
#    watcher's exit notification when there is work.
# 3. If the arming nudge already fired this stop cycle (tracked via a
#    stamp file, NOT stop_hook_active - a mail block also sets
#    stop_hook_active and must not suppress the arming nudge), allow
#    exit. This respects the Claude Code 2.1.143+ cap on consecutive
#    Stop hook blocks (default 8): we emit at most one arming block
#    per stop cycle.
# 4. Otherwise block ONCE with arming instructions (folding in a
#    reminder about incomplete tasks, if any).
#
# Codex has no background-task equivalent, so when invoked with --codex the
# script takes a separate path that long-polls inside the hook instead.
#
# Output format: JSON for Stop hook decision

# Read hook input from stdin
input=$(cat)
session_id=$(echo "$input" | jq -r '.session_id // empty')
stop_hook_active=$(echo "$input" | jq -r '.stop_hook_active // false')
cwd=$(echo "$input" | jq -r '.cwd // empty')

# Determine the host agent. The Codex hook definitions append --codex to the
# installed command; Claude Code invokes the same script with no arguments.
# Sniffing the payload or the environment instead would misfire whenever a
# Claude session inherits CODEX_* variables from a parent shell.
is_codex=false
if [ "${1:-}" = "--codex" ]; then
    is_codex=true
fi

if [ -z "$session_id" ]; then
    session_id="${CLAUDE_SESSION_ID:-$CODEX_SESSION_ID}"
fi

# Build session ID args if available (critical for agent identity
# resolution). Use a bash array so a session_id containing whitespace
# or glob metacharacters cannot word-split or expand into extra args.
session_args=()
if [ -n "$session_id" ]; then
    session_args=(--session-id "$session_id")
fi
project_dir="${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-$cwd}}"
if [ -n "$project_dir" ]; then
    session_args+=(--project "$project_dir")
fi

# Codex has no equivalent of the Claude Code background task that the
# `substrate watch` arming pattern relies on, so persistence has to live in
# the hook itself: long-poll for mail and always block, which is exactly what
# `substrate poll --always-block` emits. It blocks with the digest when mail
# arrives, blocks with a keep-alive reason when the wait expires, and blocks
# rather than failing when the daemon is unreachable. Only a missing or
# broken CLI reaches the fallback below, where allowing the exit beats
# trapping the agent in a loop it cannot escape.
if [ "$is_codex" = "true" ]; then
    substrate heartbeat "${session_args[@]}" --format context \
        2>/dev/null || true

    substrate poll "${session_args[@]}" --wait=570s --format hook \
        --always-block --quiet 2>/dev/null || echo '{}'

    exit 0
fi

# Arming-nudge stamp: marks that we already blocked once for arming in
# this stop cycle. stop_hook_active alone cannot distinguish "blocked
# for mail" from "blocked for arming", and a mail block must not
# suppress the arming nudge (that would strand the agent watcher-less
# after every mail interaction). A fresh stop cycle is detected by
# stop_hook_active=false, which clears the stamp.
stamp_dir="$HOME/.subtrate/watch"
mkdir -p "$stamp_dir" 2>/dev/null
nudge_stamp="$stamp_dir/nudged-${session_id:-default}"

if [ "$stop_hook_active" != "true" ]; then
    rm -f "$nudge_stamp"
fi

# ============================================================================
# Step 1: Quick mail check
# ============================================================================

# Record heartbeat (best effort)
substrate heartbeat "${session_args[@]}" --format context 2>/dev/null || true

# Quick (non-blocking) check for mail
quick_result=$(substrate poll "${session_args[@]}" --format hook --quiet 2>/dev/null || echo '{}')
quick_decision=$(echo "$quick_result" | jq -r '.decision // empty')

if [ "$quick_decision" = "block" ]; then
    # Mail exists - output the result immediately
    echo "$quick_result"
    exit 0
fi

# ============================================================================
# Step 2: If a watcher is armed, exit cleanly
# ============================================================================

# Note: allow is an empty object — newer Claude Code rejects
# {"decision": null}.
if substrate watch --check "${session_args[@]}" >/dev/null 2>&1; then
    rm -f "$nudge_stamp"
    echo '{}'
    exit 0
fi

# ============================================================================
# Step 3: Allow exit if the arming nudge already fired this cycle
# ============================================================================

# We already blocked once with arming instructions this stop cycle and
# the agent still has no watcher; allow exit rather than accumulate
# consecutive blocks. The next user prompt or session start re-nudges.
if [ -f "$nudge_stamp" ]; then
    echo '{}'
    exit 0
fi

# ============================================================================
# Step 4: Block once with arming instructions
# ============================================================================

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
            output="${output}#${id} [${status}], "
        fi
    done

    echo "$output"
}

# Fold an incomplete-task reminder into the single arming block, rather
# than blocking separately for it (repeated task nags are what tripped
# the consecutive-block cap; /goal handles task gating natively now).
task_note=""
if [ -n "$session_id" ]; then
    task_list=$(list_incomplete_tasks "$session_id")
    if [ -n "$task_list" ]; then
        task_note="Note: incomplete task(s) remain: ${task_list}finish or update them first if appropriate. "
    fi
fi

reason="No mail watcher is armed. ${task_note}Arm the watcher now: run \`substrate watch --session-id ${session_id:-\$CLAUDE_SESSION_ID}\` via the Bash tool with run_in_background set to true. The watcher is only for notification while this session is idle; it does not require ending your turn. Continue the current task if work remains, and re-arm it after handling each wake."

# Record that the nudge fired so we do not block again this cycle.
touch "$nudge_stamp" 2>/dev/null

jq -cn --arg reason "$reason" '{"decision": "block", "reason": $reason}'
