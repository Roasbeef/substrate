#!/bin/bash
# Subtrate SessionStart hook - heartbeat + check for mail
#
# This hook runs when a Claude Code session starts. It:
# 1. Sends a heartbeat to mark the agent as active
# 2. Checks for any pending messages to inject as context
#
# Output format: plain text for context injection

# Send heartbeat to mark session start.
substrate heartbeat --session-start --format context 2>/dev/null || true

# Poll for new messages (non-blocking).
# Output is injected as context at session start.
result=$(substrate poll --format context --quiet 2>/dev/null || echo "")

if [ -n "$result" ]; then
    echo "$result"
fi
