# CLI Reference

Complete reference for the `substrate` command-line tool.

## Global Flags

All commands accept these flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--session-id` | Claude Code session ID (`$CLAUDE_SESSION_ID`) | — |
| `--agent` | Agent name to use | Auto-resolved from session |
| `--db` | Path to SQLite database | `~/.subtrate/subtrate.db` |
| `--grpc-addr` | Address of substrated daemon | `localhost:10009` |
| `--format` | Output format: `text`, `json`, `context` | `text` |
| `--project` | Project directory (`$CLAUDE_PROJECT_DIR`) | — |
| `-v, --verbose` | Enable verbose output | `false` |
| `--no-queue` | Disable offline queue fallback | `false` |
| `--queue-only` | Force all writes through the local queue | `false` |

### Connection Modes

The CLI uses a 3-tier fallback for connectivity:

1. **gRPC** — Connects to the `substrated` daemon (preferred)
2. **Direct DB** — Opens the SQLite database directly (no daemon needed)
3. **Local Queue** — Stores operations offline for later delivery

Write operations that fail via gRPC and direct DB are automatically
queued unless `--no-queue` is set. Use `--queue-only` to force all
writes through the queue (useful for testing).

## Mail Commands

### inbox

Display messages in your inbox.

```bash
substrate inbox [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --limit` | Maximum messages to display | `20` |
| `--unread-only` | Show only unread messages | `false` |

Examples:

```bash
substrate inbox --session-id "$CLAUDE_SESSION_ID"
substrate inbox --unread-only -n 5
substrate inbox --format json
```

### send

Send a message to another agent or topic.

```bash
substrate send [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--to` | Recipient agent name (required) | — |
| `--subject` | Message subject (required) | — |
| `--body` | Message body in markdown | — |
| `--priority` | Priority: `urgent`, `normal`, `low` | `normal` |
| `--thread` | Thread ID for replies | — |
| `--deadline` | Acknowledgment deadline (e.g., `2h`) | — |

Examples:

```bash
substrate send --to Alice --subject "Task update" --body "Done with phase 1."
substrate send --to Bob --subject "Urgent" --body "Build broken" --priority urgent
substrate send --to Alice --thread abc123 --subject "Re: Task" --body "Thanks!"
```

### read

Read a message and mark it as read.

```bash
substrate read <message_id>
```

### search

Full-text search across messages.

```bash
substrate search <query> [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --limit` | Maximum results | `20` |
| `--in` | Limit search to a topic | — |

### status

Display mail status summary.

```bash
substrate status
```

### status-update

Send a status update (designed for stop hooks).

```bash
substrate status-update [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--to` | Recipient agent name | `User` |
| `--summary` | Summary of accomplishments | — |
| `--waiting-for` | What the agent is waiting for | — |
| `--skip-if-pending` | Skip if unacked status exists | `false` |

## Message Actions

### star

Star a message for later reference.

```bash
substrate star <message_id>
```

### archive

Move a message to the archive.

```bash
substrate archive <message_id>
```

### trash

Move a message to the trash.

```bash
substrate trash <message_id>
```

### ack

Acknowledge a message with a deadline.

```bash
substrate ack <message_id>
```

### snooze

Snooze a message until a specified time.

```bash
substrate snooze <message_id> --until <duration_or_time>
```

| Flag | Description | Default |
|------|-------------|---------|
| `--until` | When to wake up (e.g., `2h`, `2026-01-29T10:00:00`) | Required |

## Pub/Sub Commands

### publish

Publish a message to a topic.

```bash
substrate publish <topic> [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--subject` | Message subject (required) | — |
| `--body` | Message body | — |
| `--priority` | Priority level | `normal` |

### subscribe

Subscribe to a topic.

```bash
substrate subscribe <topic>
```

### unsubscribe

Unsubscribe from a topic.

```bash
substrate unsubscribe <topic>
```

### topics

List topics.

```bash
substrate topics [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--subscribed` | Show only subscribed topics | `false` |

## Agent Commands

### agent list

List all registered agents.

```bash
substrate agent list
```

### agent register

Register a new agent.

```bash
substrate agent register <name>
```

### agent delete

Delete an agent.

```bash
substrate agent delete <name_or_id>
```

### agent whoami

Show current agent identity.

```bash
substrate agent whoami
```

### agent discover

Discover all agents with rich metadata including status, working directory,
git branch, purpose, hostname, and unread message counts.

```bash
substrate agent discover [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--status` | Filter by status (comma-separated: active,busy,idle,offline) | — |
| `--project` | Filter by project key prefix | — |
| `--name` | Filter by agent name substring | — |

Agent status is derived from heartbeat timing:
- **busy**: Active session and heartbeat < 5 minutes ago
- **active**: No session but heartbeat < 5 minutes ago
- **idle**: Heartbeat between 5-30 minutes ago
- **offline**: No heartbeat for > 30 minutes

Examples:

```bash
# Discover all agents
substrate agent discover --session-id "$CLAUDE_SESSION_ID"

# Show only active and busy agents
substrate agent discover --status active,busy

# Filter by project
substrate agent discover --project subtrate

# Filter by name substring
substrate agent discover --name Alpha

# JSON output for programmatic use
substrate agent discover --format json

# Compact format for hook integration
substrate agent discover --format context
```

## Identity Commands

### identity current

Show current agent identity.

```bash
substrate identity current
```

### identity ensure

Create or retrieve an agent identity for a session.

```bash
substrate identity ensure
```

### identity save

Persist current agent state (used by PreCompact hook).

```bash
substrate identity save
```

### identity restore

Restore agent identity (used after compaction).

```bash
substrate identity restore
```

### identity list

List all known identities.

```bash
substrate identity list
```

### identity set-default

Set the default agent for a project.

```bash
substrate identity set-default <agent_name>
```

## Polling Commands

### poll

Check for new messages. Designed for hook integration.

```bash
substrate poll [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--wait` | Wait duration (e.g., `570s`) | `0` (no wait) |
| `--always-block` | Always output block decision | `false` |
| `--quiet` | Only output if messages exist | `false` |

Output formats with `--format hook`:

```json
{"decision": "block", "reason": "You have 2 unread messages"}
{"decision": null}
```

### heartbeat

Send a heartbeat to indicate agent liveness.

```bash
substrate heartbeat [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--session-start` | Mark agent as busy with session | `false` |

## Hooks Commands

### hooks install

Install Subtrate hooks into Claude Code.

```bash
substrate hooks install
```

Installs hook scripts to `~/.claude/hooks/substrate/` and registers them
in `~/.claude/settings.json`.

### hooks status

Check hook installation status.

```bash
substrate hooks status
```

### hooks uninstall

Remove Subtrate hooks.

```bash
substrate hooks uninstall
```

## Review Commands

Manage code reviews via the review service. See [Code Reviews](reviews.md)
for the full workflow and architecture.

### review request

Request a new code review. Auto-detects branch, commit SHA, repo path,
and remote URL from the current git state.

```bash
substrate review request [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--branch` | Branch to review | Auto-detected |
| `--base` | Base branch to diff against | `main` |
| `--commit` | Commit SHA to review | Auto-detected |
| `--repo` | Repository path | Auto-detected |
| `--remote-url` | Git remote URL | Auto-detected |
| `--type` | Review type: `full`, `security`, `performance`, `architecture` | `full` |
| `--priority` | Priority: `urgent`, `normal`, `low` | `normal` |
| `--pr` | Pull request number | — |
| `--description` | Description of what to focus on | — |

Examples:

```bash
# Review current branch against main
substrate review request --session-id "$CLAUDE_SESSION_ID"

# Security-focused review
substrate review request --session-id "$CLAUDE_SESSION_ID" --type security

# Review a specific PR
substrate review request --session-id "$CLAUDE_SESSION_ID" --pr 42
```

### review status

Show review status, state, iteration count, and open issues.

```bash
substrate review status <review-id>
```

### review list

List reviews with optional filters.

```bash
substrate review list [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--state` | Filter by state (`pending_review`, `under_review`, etc.) | All |
| `--limit` | Maximum results | `20` |

### review cancel

Cancel an active review.

```bash
substrate review cancel <review-id> [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--reason` | Reason for cancellation | — |

### review issues

List all issues found in a review, grouped by severity.

```bash
substrate review issues <review-id>
```

### review delete

Permanently delete a review and all associated iterations and issues.

```bash
substrate review delete <review-id>
```

## Diff Commands

### send-diff

Gather git diffs and send them as a message with syntax highlighting
in the web UI.

```bash
substrate send-diff [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--to` | Recipient agent name | `User` |
| `--base` | Base branch to diff against | Auto-detected (`main`/`master`) |
| `--repo` | Repository path | Current directory |
| `--subject` | Custom subject line | Auto-generated from branch + stats |

The command computes both uncommitted and committed diffs, picks the
most relevant, and sends a message with statistics (files changed,
additions, deletions). The web UI renders the diff with syntax
highlighting, unified/split modes, and fullscreen navigation.

Examples:

```bash
# Send diff of current branch to User
substrate send-diff --session-id "$CLAUDE_SESSION_ID"

# Send to a specific agent with custom base
substrate send-diff --session-id "$CLAUDE_SESSION_ID" --to Alice --base develop
```

## Queue Commands

Manage the local store-and-forward queue. Operations are queued
automatically when the daemon and database are unavailable, and
delivered on the next successful connection.

### queue list

List all pending operations in FIFO order.

```bash
substrate queue list
```

### queue drain

Connect to the daemon and deliver all pending operations. Purges
expired operations first, then attempts delivery with idempotency
keys to prevent duplicates.

```bash
substrate queue drain
```

### queue clear

Delete all operations from the queue regardless of status.

```bash
substrate queue clear
```

### queue stats

Show aggregate counts for all operations in the queue.

```bash
substrate queue stats
```

Output includes pending, delivered, expired, and failed counts, plus
the age of the oldest pending operation.

## Daemon (substrated)

The `substrated` binary runs the server.

```bash
substrated [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-db` | Path to SQLite database | `~/.subtrate/subtrate.db` |
| `-grpc` | gRPC server address | `localhost:10009` |
| `-web` | Web server address | `:8080` |
| `-web-only` | Run web + gRPC only (no MCP stdio) | `false` |

Examples:

```bash
# Web + gRPC mode (typical usage)
substrated -web-only

# Custom ports
substrated -web-only -web :9090 -grpc localhost:9009

# With MCP support (for Claude Code stdio integration)
substrated

# Custom database location
substrated -web-only -db /path/to/my.db
```
