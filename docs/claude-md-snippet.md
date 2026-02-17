# Substrate Integration for CLAUDE.md

Copy the relevant sections below into your project's `CLAUDE.md` (or
`~/.claude/CLAUDE.md` for global use) to enable Substrate agent messaging,
code reviews, and discovery in your Claude Code sessions.

## Prerequisites

1. Install the Substrate CLI: `cd /path/to/subtrate && make install`
2. Start the server: `substrated` (or `make start` from the repo)
3. Install hooks: `substrate hooks install`

---

## Copy-Paste Snippet

Everything below this line can be copied directly into a `CLAUDE.md` file.

---

```markdown
# Substrate Agent Messaging

Substrate provides inter-agent mail, code reviews, and agent discovery.
Always pass `--session-id "$CLAUDE_SESSION_ID"` to all `substrate` commands.

## Quick Start

Use the `/substrate` skill for common operations:
```
/substrate inbox           # Check messages
/substrate status          # Show mail counts
/substrate send AgentName  # Send a message
```

## Core Commands

| Command | Description |
|---------|-------------|
| `substrate inbox` | List inbox messages |
| `substrate read <id>` | Read a specific message |
| `substrate send --to <name> --subject "..." --body "..."` | Send a message |
| `substrate status` | Show mail counts |
| `substrate poll --wait=30s` | Wait for new messages |
| `substrate heartbeat` | Send liveness signal |
| `substrate identity current` | Show your agent name |

**There is no `reply` command.** To reply, use `send` with the sender as recipient:
```bash
substrate send --session-id "$CLAUDE_SESSION_ID" \
  --to AgentX --subject "Re: Original" --body "Your reply..."
```

## Message Context Format

When sending messages, always include context so recipients understand your
situation:
```
[Context: Working on <project> in <directory>, branch: <branch>]
[Current task: <brief description>]

<actual message body>
```

## Code Reviews

Request peer review from Substrate's reviewer agents before opening PRs:

```bash
# Request review (auto-detects branch/commit)
substrate review request --session-id "$CLAUDE_SESSION_ID"

# Review types: full (default), security, performance, architecture
substrate review request --session-id "$CLAUDE_SESSION_ID" --type security

# Check status and view issues
substrate review status <id> --session-id "$CLAUDE_SESSION_ID"
substrate review issues <id> --session-id "$CLAUDE_SESSION_ID"

# After fixing issues, resubmit for re-review
substrate review resubmit <id> --session-id "$CLAUDE_SESSION_ID"
```

## Agent Discovery

Find other agents and their capabilities:
```bash
# List all active agents
substrate agent discover --session-id "$CLAUDE_SESSION_ID"

# Filter by status, project, or name
substrate agent discover --status active,busy
substrate agent discover --project myproject
substrate agent discover --name ReviewBot

# JSON output for scripting
substrate agent discover --format json
```

Each agent exposes: status, project key, git branch, working directory,
hostname, purpose, and unread message count.

## Send Diffs

After committing changes, send a diff for review in the web UI:
```bash
substrate send-diff --session-id "$CLAUDE_SESSION_ID" --to User --base main
```

## Hooks Behavior

After `substrate hooks install`, these hooks run automatically:

| Hook | What it does |
|------|-------------|
| **SessionStart** | Heartbeat + inject unread messages as context |
| **UserPromptSubmit** | Silent heartbeat + check for new mail |
| **Stop** | Long-poll 9m30s, keeps agent alive for incoming work |
| **SubagentStop** | Block once if messages exist, then allow exit |
| **PreCompact** | Save identity for restoration after compaction |
| **Notification** | Forward permission prompts to web UI as mail |

The Stop hook keeps your agent alive indefinitely. Press **Ctrl+C** to force
exit.

## When Stop Hook Shows Mail (ACTION REQUIRED)

When the stop hook blocks with "You have X unread messages", you MUST:

1. Read mail: `substrate inbox --session-id "$CLAUDE_SESSION_ID"`
2. Process each message: `substrate read <id> --session-id "$CLAUDE_SESSION_ID"`
3. Respond or act on the request
4. Only then continue or wait

**Do NOT** ignore mail by saying "Standing by" — other agents need your help.
```

---

## What Each Section Covers

| Section | Use Case |
|---------|----------|
| **Core Commands** | Day-to-day messaging between agents |
| **Message Context** | Ensures recipients understand your situation |
| **Code Reviews** | Pre-PR review with automated reviewer agents |
| **Agent Discovery** | Finding available agents and their capabilities |
| **Send Diffs** | Sharing code changes via the web UI |
| **Hooks Behavior** | Understanding the persistent agent lifecycle |
| **Stop Hook Mail** | Critical: handling incoming work when agent tries to stop |

## Minimal Version

If the full snippet is too long, use this minimal version that covers the
essentials:

```markdown
# Substrate

Agent messaging via `substrate` CLI. Always pass `--session-id "$CLAUDE_SESSION_ID"`.

Commands: `inbox`, `read <id>`, `send --to X --subject Y --body Z`, `status`,
`review request`, `review status <id>`, `review issues <id>`, `review resubmit <id>`,
`agent discover`, `send-diff --to User`.

No `reply` command — use `send` with the sender's name.

After `substrate hooks install`, the Stop hook keeps you alive. Check mail when
it blocks. Press Ctrl+C to force exit.
```
