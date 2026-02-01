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

# Export CLAUDE_SESSION_ID via CLAUDE_ENV_FILE if available.
# This makes the session ID available to the agent during the session.
if [ -n "$session_id" ] && [ -n "$CLAUDE_ENV_FILE" ]; then
    echo "CLAUDE_SESSION_ID=$session_id" >> "$CLAUDE_ENV_FILE"
fi

# Send heartbeat to mark session start.
if [ -n "$session_id" ]; then
    substrate heartbeat --session-start --session-id "$session_id" --format context 2>/dev/null || true
else
    substrate heartbeat --session-start --format context 2>/dev/null || true
fi

# Poll for new messages (non-blocking).
# Output is injected as context at session start.
if [ -n "$session_id" ]; then
    result=$(substrate poll --session-id "$session_id" --format context --quiet 2>/dev/null || echo "")
else
    result=$(substrate poll --format context --quiet 2>/dev/null || echo "")
fi

if [ -n "$result" ]; then
    echo "$result"
fi

# Check for pending code reviews.
# This injects review context for agents operating as reviewers.
review_args=""
if [ -n "$session_id" ]; then
    review_args="--session-id $session_id"
fi

# Check pending reviews (assigned to this agent)
pending_reviews=$(substrate review list $review_args --state pending_review --format json 2>/dev/null || echo '{"reviews":[]}')
pending_count=$(echo "$pending_reviews" | jq -r '.reviews | length' 2>/dev/null || echo "0")

# Check re-review requests
rereview=$(substrate review list $review_args --state re_review --format json 2>/dev/null || echo '{"reviews":[]}')
rereview_count=$(echo "$rereview" | jq -r '.reviews | length' 2>/dev/null || echo "0")

# Output review context if there are pending reviews
if [ "$pending_count" -gt 0 ] || [ "$rereview_count" -gt 0 ]; then
    echo ""
    echo "=== Code Review Queue ==="

    if [ "$pending_count" -gt 0 ]; then
        echo ""
        echo "Pending Reviews ($pending_count):"
        echo "$pending_reviews" | jq -r '.reviews[] | "  - #\(.id): \(.branch) from \(.requester_name // "unknown")"' 2>/dev/null || true
    fi

    if [ "$rereview_count" -gt 0 ]; then
        echo ""
        echo "Re-Reviews Needed ($rereview_count):"
        echo "$rereview" | jq -r '.reviews[] | "  - #\(.id): \(.branch) - fixes pushed"' 2>/dev/null || true
    fi

    echo ""
    echo "Use 'substrate review status <id>' for review details."
fi
