#!/bin/bash
# Subtrate Stop hook - Persistent Agent Pattern
#
# This hook keeps the main agent alive indefinitely by blocking exit
# and continuously checking for new mail. The agent stays alive even
# when there are no messages, effectively keeping it "on call" for work.
#
# Key behaviors:
# - Long-polls for 55s (under 60s hook timeout)
# - Always outputs {"decision": "block"} to stay alive
# - User can force exit with Ctrl+C (bypasses hooks)
#
# Output format: JSON for Stop hook decision

# Record heartbeat (best effort).
substrate heartbeat --format context 2>/dev/null || true

# Long-poll for messages (55s wait, under 60s hook timeout).
# --always-block ensures we output block decision even with no messages.
# This keeps the agent alive indefinitely, continuously checking for work.
substrate poll --wait=55s --format hook --always-block 2>/dev/null || \
    echo '{"decision": "block", "reason": "Error checking mail. Agent staying alive."}'
