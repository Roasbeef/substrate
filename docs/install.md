# Installation Guide

This guide covers installing Subtrate as a Claude Code plugin or manually via
the CLI, starting the daemon, and configuring your global `~/.claude/CLAUDE.md`
with Subtrate integration fragments.

## Prerequisites

- **Go 1.22+** with CGO enabled (for building from source)
- **jq** (used by hook scripts for JSON parsing)
- **Claude Code** 1.0.33+ (for plugin support)

## Option 1: Plugin Installation (Recommended)

The plugin approach lets Claude Code discover Subtrate's hooks and skills
automatically from the repository directory.

### Step 1: Install the substrate CLI and daemon

The plugin provides hooks and skills, but the `substrate` and `substrated`
binaries must be installed since the hook scripts call the CLI directly. Build
from source to embed the frontend into the daemon binary:

```bash
git clone https://github.com/roasbeef/subtrate.git
cd subtrate
make install
```

This builds the frontend, then runs `go install` for both `substrate` and
`substrated`. Binaries are placed in `$(go env GOPATH)/bin/`. Ensure that
directory is in your PATH.

Verify:

```bash
substrate --help
substrated --help
```

### Step 2: Load the plugin

Point Claude Code at the cloned repository:

```bash
claude --plugin-dir /path/to/subtrate
```

This registers the hooks (SessionStart, UserPromptSubmit, Stop, SubagentStop,
PreCompact) and the `/substrate:substrate` skill automatically.

To load the plugin every session without the flag, install it to your user
scope from a marketplace or add it to your settings manually.

### Step 3: Start the daemon

The `substrated` daemon provides the web UI, messaging backend, and API that
the CLI and hooks communicate with. By default it runs in web+gRPC mode (no
MCP stdio).

```bash
# Start in the background
substrated &

# Or from the source directory:
make start
```

The web UI is available at http://localhost:8080.

## Option 2: Manual Installation

The manual path installs hook scripts to `~/.claude/hooks/substrate/` and
registers them in `~/.claude/settings.json`.

### Step 1: Install binaries

Build from source (required to embed the frontend into the daemon):

```bash
git clone https://github.com/roasbeef/subtrate.git
cd subtrate
make install
```

Ensure `$(go env GOPATH)/bin/` is in your PATH.

### Step 2: Install hooks and skill

```bash
substrate hooks install
```

This creates:
- `~/.claude/hooks/substrate/` — 5 hook scripts (session_start.sh, stop.sh,
  subagent_stop.sh, user_prompt.sh, pre_compact.sh)
- `~/.claude/settings.json` — Hook registrations appended (preserves existing
  hooks)
- `~/.claude/skills/substrate/SKILL.md` — The `/substrate` skill

### Step 3: Verify

```bash
substrate hooks status
```

Expected output:

```
Status: INSTALLED

Scripts directory: ~/.claude/hooks/substrate
  Scripts: All present

Skill: ~/.claude/skills/substrate
  SKILL.md: Present

Hooks in settings.json:
  - PreCompact
  - SessionStart
  - Stop
  - SubagentStop
  - UserPromptSubmit
```

### Step 4: Start the daemon

```bash
substrated &
```

## Starting the Daemon

The `substrated` daemon must be running for mail, reviews, and the web UI to
work. The hook scripts will fail silently if the daemon is not running.

```bash
# Foreground (for development)
substrated

# Background
substrated &

# From source with embedded frontend
cd /path/to/subtrate
make run          # Foreground
make start        # Background
make stop         # Stop background daemon
make restart      # Restart
```

Default port is 8080. Override with `--web`:

```bash
substrated --web :8081
```

To enable MCP stdio transport (for direct Claude Code integration via stdin):

```bash
substrated --mcp
```

## Configuring CLAUDE.md

To teach Claude Code agents how to use Subtrate, add the following fragments to
your global `~/.claude/CLAUDE.md`. Copy the sections that are relevant to your
workflow.

### Fragment 1: Subtrate Command Center

This is the core reference for mail and messaging. Add this to give agents full
awareness of Subtrate capabilities.

````markdown
# Subtrate - Agent Command Center

Subtrate provides mail/messaging between Claude Code agents with automatic
identity management and lifecycle hooks. **Subtrate is the primary way to
communicate with the user** -- when you need to reach the user or send status
updates, use Subtrate mail rather than just printing to the console.

## Quick Start - Use the /substrate Skill

The easiest way to use Subtrate is via the `/substrate` skill:
```
/substrate inbox           # Check your messages
/substrate status          # Show mail counts
/substrate send AgentName  # Send a message
```

The skill handles session ID and formatting automatically.

## CLI Commands Reference

**IMPORTANT**: Always pass `--session-id "$CLAUDE_SESSION_ID"` to CLI commands,
or they will fail with "no agent specified".

| Command | Description | Example |
|---------|-------------|---------|
| `inbox` | List inbox messages | `substrate inbox --session-id "$CLAUDE_SESSION_ID"` |
| `read <id>` | Read a specific message | `substrate read 42 --session-id "$CLAUDE_SESSION_ID"` |
| `send` | Send a new message | `substrate send --session-id "$CLAUDE_SESSION_ID" --to User --subject "Hi" --body "..."` |
| `status` | Show mail counts | `substrate status --session-id "$CLAUDE_SESSION_ID"` |
| `poll` | Wait for new messages | `substrate poll --session-id "$CLAUDE_SESSION_ID" --wait=30s` |
| `heartbeat` | Send liveness signal | `substrate heartbeat --session-id "$CLAUDE_SESSION_ID"` |
| `identity current` | Show your agent name | `substrate identity current --session-id "$CLAUDE_SESSION_ID"` |
| `review request` | Request code review | `substrate review request --session-id "$CLAUDE_SESSION_ID"` |
| `review status <id>` | Show review status | `substrate review status abc --session-id "$CLAUDE_SESSION_ID"` |
| `review list` | List reviews | `substrate review list --session-id "$CLAUDE_SESSION_ID"` |
| `review issues <id>` | List review issues | `substrate review issues abc --session-id "$CLAUDE_SESSION_ID"` |
| `review cancel <id>` | Cancel review | `substrate review cancel abc --session-id "$CLAUDE_SESSION_ID"` |

**There is NO `reply` command** - to reply, use `send` with the sender as
recipient:
```bash
substrate send --session-id "$CLAUDE_SESSION_ID" \
  --to AgentX \
  --subject "Re: Original Subject" \
  --body "Your reply here..."
```

## Setup

```bash
# Check if hooks are installed
substrate hooks status

# Install hooks (idempotent - safe to run multiple times)
substrate hooks install
```

No manual identity setup needed - your agent identity is auto-created on first
use and persists across sessions and compactions.

## What the Hooks Do

| Hook | Behavior |
|------|----------|
| **SessionStart** | Heartbeat + inject unread messages as context |
| **UserPromptSubmit** | Silent heartbeat + check for new mail |
| **Stop** | Long-poll 9m30s, always block to keep agent alive (Ctrl+C to force exit) |
| **SubagentStop** | Block once if messages exist, then allow exit |
| **PreCompact** | Save identity for restoration after compaction |

The Stop hook keeps your agent alive indefinitely, checking for work from other
agents. Press **Ctrl+C** to force exit.

## When Stop Hook Shows Mail (ACTION REQUIRED)

**CRITICAL**: When the stop hook blocks with "You have X unread messages", you
MUST:

1. **Read your mail immediately**:
   ```bash
   substrate inbox --session-id "$CLAUDE_SESSION_ID"
   ```

2. **Process each message** - read the full content with
   `substrate read <id> --session-id "$CLAUDE_SESSION_ID"`

3. **Respond or act** on what's requested in the messages

4. **Only then** should you wait for the next user request

**DO NOT** just say "Standing by" or "Ready" when you have mail - this ignores
messages from other agents who need your help!

## Agent Message Context (IMPORTANT)

When sending messages via Subtrate, **ALWAYS** include a brief context intro:

**Format:**
```
[Context: Working on <project> in <directory>, branch: <branch>]
[Current task: <brief description of what you're doing>]

<actual message body>
```

## Sending Diffs to the User

After making commits, **send a diff to the User** so they can see actual code
changes with syntax highlighting in the web UI:

```bash
substrate send-diff --session-id "$CLAUDE_SESSION_ID" --to User --base main
```
````

### Fragment 2: Code Review Workflow

Add this if you use Subtrate's code review system, which spawns Claude reviewer
agents to analyze diffs.

````markdown
# Code Review Workflow

After completing a task or feature, request a code review via Subtrate's native
review system before creating a PR. This spawns Claude reviewer agents that
analyze diffs and return structured feedback with issues.

## Post-Commit Review
```bash
# Review current branch against main (auto-detects branch/commit/remote)
substrate review request --session-id "$CLAUDE_SESSION_ID"

# Review a specific branch against a base
substrate review request --session-id "$CLAUDE_SESSION_ID" --branch feature-x --base main

# Review a specific commit
substrate review request --session-id "$CLAUDE_SESSION_ID" --commit abc123

# Review a specific PR
substrate review request --session-id "$CLAUDE_SESSION_ID" --pr 42

# Security-focused review
substrate review request --session-id "$CLAUDE_SESSION_ID" --type security

# Performance review
substrate review request --session-id "$CLAUDE_SESSION_ID" --type performance

# Architecture review
substrate review request --session-id "$CLAUDE_SESSION_ID" --type architecture
```

## Review Back-and-Forth
1. Commit changes on feature branch
2. Run `substrate review request --session-id "$CLAUDE_SESSION_ID"`
3. Check status: `substrate review status <id> --session-id "$CLAUDE_SESSION_ID"`
4. View issues: `substrate review issues <id> --session-id "$CLAUDE_SESSION_ID"`
5. Address issues, commit fixes
6. Review system tracks iterations automatically

## Review Types
- **full** (default) — General review (bugs, logic, security, CLAUDE.md compliance)
- **security** — Injection, auth bypass, data exposure, crypto
- **performance** — N+1 queries, memory leaks, allocations
- **architecture** — Separation of concerns, interface design, testability

## When to Request Reviews
- After completing a task or feature (before opening a PR)
- After significant refactoring
- When touching security-sensitive code (`--type security`)
- When adding new public interfaces (`--type architecture`)
````

### Fragment 3: Session Management with Hooks

Add this if you want agents to use session tracking with Subtrate's lifecycle
hooks for continuity across context compactions.

````markdown
# Context Management & Compaction Recovery

Your context window will be automatically compacted as it approaches its limit,
allowing you to continue working indefinitely. Do not stop tasks early due to
token budget concerns.

**After context compaction, your FIRST action MUST be `/session-resume`.**
Do NOT respond to the user's request until you have run it.

Signs compaction just occurred:
- The conversation feels "fresh" but user expects you to continue work
- SessionStart hook shows an active session with compaction_count > 0
- User says "continue", "keep going", "where were we"

If unsure, check: `ls .sessions/active/` -- if files exist, run `/session-resume`.
````

## Updating

### Plugin path

Pull the latest changes to the cloned repository:

```bash
cd /path/to/subtrate
git pull
```

Claude Code will pick up changes on next session start.

### Manual path

Reinstall the binary and hooks:

```bash
go install github.com/roasbeef/subtrate/cmd/substrate@latest
go install github.com/roasbeef/subtrate/cmd/substrated@latest
substrate hooks install
```

The hooks install command is idempotent and will overwrite scripts with the
latest versions.

## Uninstalling

### Plugin path

Remove the `--plugin-dir` flag from your Claude Code invocation, or uninstall
via the plugin manager:

```bash
claude plugin uninstall substrate
```

### Manual path

```bash
substrate hooks uninstall
```

This removes hook scripts from `~/.claude/hooks/substrate/`, cleans up
`~/.claude/settings.json`, and deletes the skill from
`~/.claude/skills/substrate/`.

## Troubleshooting

### `substrate: command not found`

The `substrate` binary is not in your PATH. Ensure `$GOPATH/bin` (or
`$HOME/go/bin`) is in your PATH:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Add this to your shell profile (`.bashrc`, `.zshrc`, etc.).

### Hooks fail silently

The hook scripts suppress errors by default. Check if the daemon is running:

```bash
curl -s http://localhost:8080/api/agents/status | jq .
```

If the daemon is not running, start it with `substrated &`.

### Duplicate hooks (plugin + manual)

If you installed both via the plugin and manually, hooks will fire twice. Use
only one method:

```bash
# If using the plugin, remove manual hooks:
substrate hooks uninstall

# If using manual hooks, don't load the plugin:
# (remove --plugin-dir flag)
```

### Hook debug logs

The Stop hook writes debug information to:

- `~/.subtrate/stop_hook_debug.log` — Status update attempts
- `~/.subtrate/stop_hook_trace.log` — Long-poll trace

### CGO / FTS5 errors when building from source

SQLite FTS5 requires CGO flags:

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go install ./cmd/substrate
```

Or use `make build-all` which sets the flags automatically.
