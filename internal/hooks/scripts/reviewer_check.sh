#!/bin/bash
# Subtrate Reviewer Check script - check for pending reviews
#
# This script can be called by reviewer agents to check for pending reviews.
# It outputs review context that can be injected into the agent.
#
# Usage: reviewer_check.sh [--session-id <id>]
#
# Output format: plain text for context injection

# Parse arguments
session_args=""
while [ $# -gt 0 ]; do
    case "$1" in
        --session-id)
            shift
            session_args="--session-id $1"
            ;;
    esac
    shift
done

# Check for pending reviews assigned to this agent
pending_reviews=$(substrate review list $session_args --state pending_review --format json 2>/dev/null || echo '{"reviews":[]}')
pending_count=$(echo "$pending_reviews" | jq -r '.reviews | length')

# Check for reviews needing re-review
rereview=$(substrate review list $session_args --state re_review --format json 2>/dev/null || echo '{"reviews":[]}')
rereview_count=$(echo "$rereview" | jq -r '.reviews | length')

# Output context if there are pending reviews
if [ "$pending_count" -gt 0 ] || [ "$rereview_count" -gt 0 ]; then
    echo "=== Pending Code Reviews ==="
    echo ""

    if [ "$pending_count" -gt 0 ]; then
        echo "## New Reviews ($pending_count)"
        echo "$pending_reviews" | jq -r '.reviews[] | "- Review #\(.id): \(.branch) (from \(.requester_name // "unknown"))"'
        echo ""
    fi

    if [ "$rereview_count" -gt 0 ]; then
        echo "## Re-Reviews Needed ($rereview_count)"
        echo "$rereview" | jq -r '.reviews[] | "- Review #\(.id): \(.branch) - fixes pushed, needs re-review"'
        echo ""
    fi

    echo "Use 'substrate review status <id>' for details."
    echo ""
fi
