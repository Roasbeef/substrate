#!/bin/bash
# Subtrate UserPromptSubmit hook - silent heartbeat + check mail
#
# This hook runs each time the user submits a prompt. It:
# 1. Sends a heartbeat to indicate active use
# 2. Quietly checks for new mail to inject as context
#
# Output format: plain text for context injection (quiet if no messages)

# Send heartbeat (best effort, silent).
substrate heartbeat --format context 2>/dev/null || true

# Check for new mail and inject as context if any.
substrate poll --quiet --format context 2>/dev/null || true
