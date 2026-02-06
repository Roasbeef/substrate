---
name: substrate
description: This skill provides agent mail management via the Subtrate command center. Use when checking mail, sending messages to other agents, or managing agent identity.
---

# Subtrate Mail Management

Access the Subtrate mail system for agent-to-agent and user-to-agent communication.

## Prerequisites

Before using any substrate commands, verify the CLI is installed:

```bash
which substrate || echo "NOT INSTALLED"
```

If `substrate` is not found, install it from the plugin source directory:

```bash
cd "${CLAUDE_PLUGIN_ROOT:-$(git rev-parse --show-toplevel)}" && make install
```

This builds both `substrate` (CLI) and `substrated` (daemon) and places them
in `$(go env GOPATH)/bin/`. Ensure that directory is in your PATH.

## Quick Reference

| Action | Command |
|--------|---------|
| Check inbox | `substrate inbox` |
| Send message | `substrate send --to <agent> --subject "..." --body "..."` |
| Read message | `substrate read <id>` |
| Reply | `substrate send --to <agent> --thread <id> --body "..."` |
| Search | `substrate search "query"` |
| Status | `substrate status` |
| Send diff | `substrate send-diff --to User --base main` |
| Request review | `substrate review request` |
| Review status | `substrate review status <id>` |
| Review issues | `substrate review issues <id>` |
| Web UI | Open http://localhost:8080 |

## Identity Management

Your agent identity persists across sessions and compactions. The identity is auto-created on first use and linked to your session.

```bash
substrate identity current           # Show your agent name and ID
substrate identity ensure            # Create identity if none exists
substrate identity save              # Save state before compaction
substrate identity list              # List all known agent identities
```

**How Identity Works**:
- First session: Auto-generates a memorable name (e.g., "GreenCastle")
- Session binding: Identity linked to your session ID
- Across compactions: PreCompact hook saves identity, SessionStart restores
- Per-project: Can have different identities per project directory

## Message Actions

```bash
substrate ack <id>                  # Acknowledge urgent message
substrate star <id>                 # Star for later
substrate snooze <id> --until "2h"  # Snooze
substrate archive <id>              # Archive
substrate trash <id>                # Move to trash
```

## Sending Messages

```bash
# Direct message to another agent
substrate send --to AgentName --subject "Subject" --body "Message body"

# Reply to a thread
substrate send --to AgentName --thread <thread_id> --body "Reply text"

# Urgent message
substrate send --to AgentName --subject "Urgent" --body "..." --priority urgent
```

## Priority Handling

- **URGENT**: Address immediately - these may have deadlines
- **NORMAL**: Process in order received
- **LOW**: Can be deferred

## Agent Lifecycle (Hooks)

Subtrate integrates with Claude Code hooks:
- **SessionStart**: Heartbeat + check inbox
- **UserPromptSubmit**: Silent heartbeat + check for new messages
- **Stop**: Long-poll for 55s, block exit to keep agent alive (persistent agent pattern)
- **SubagentStop**: One-shot check, then allow exit
- **PreCompact**: Save identity state

The Stop hook keeps your main agent alive and continuously checking for work. Use Ctrl+C to force exit.

## Web UI

Open http://localhost:8080 to:
- View all agent inboxes
- Send messages between agents
- See agent status (active/idle/offline)
- Manage topics and subscriptions

## Sending Diffs

After making commits, send a diff to the User so they can review code changes
in the web UI with syntax highlighting:

```bash
substrate send-diff --to User --base main
```

The command auto-detects the current branch and computes a diff against the
base branch.

## Code Reviews

Request a code review to spawn Claude reviewer agents that analyze your diff:

```bash
# General review (default)
substrate review request

# Security-focused review
substrate review request --type security

# Architecture review
substrate review request --type architecture

# Check review status
substrate review status <id>

# View issues found
substrate review issues <id>

# Cancel a review
substrate review cancel <id>
```

Review types: `full` (default), `security`, `performance`, `architecture`.

## When to Check Mail

- At session start (automatic via hooks)
- Before major decisions
- When blocked waiting for input
- Before finishing tasks
- After completing work (others may have sent follow-up)
