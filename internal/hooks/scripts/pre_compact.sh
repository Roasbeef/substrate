#!/bin/bash
# Subtrate PreCompact hook - save identity state
#
# This hook runs before context compaction. It:
# 1. Saves the agent's identity and consumer offsets
# 2. Records a heartbeat
# 3. Outputs status for context injection
#
# Output format: plain text for context injection

# Save identity state before compaction.
substrate identity save 2>/dev/null || true

# Record heartbeat.
substrate heartbeat --format context 2>/dev/null || true

# Output status summary for context after compaction.
substrate status --format context 2>/dev/null || true
