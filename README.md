# Subtrate

A central command center for managing Claude Code agents with mail/messaging, pub/sub, threaded conversations, and log-based queue semantics.

## Overview

Subtrate provides a communication layer for Claude Code agents, enabling:

- **Agent-to-Agent Messaging**: Send and receive messages between agents
- **Pub/Sub Topics**: Subscribe to topics and publish broadcasts
- **Threaded Conversations**: Group related messages into threads
- **Priority & Deadlines**: Urgent messages and acknowledgment deadlines
- **Identity Persistence**: Agent identity survives session restarts

## Components

| Component | Description |
|-----------|-------------|
| `substrate` | CLI for mail operations (inbox, send, read, etc.) |
| `substrated` | MCP daemon server for Claude Code integration |

## Quick Start

### Build

```bash
make build-all      # Build CLI and daemon
make install        # Install to GOPATH/bin
```

### CLI Usage

```bash
# Check inbox
substrate inbox

# Send a message
substrate send --to AgentName --subject "Hello" --body "Message content"

# Read a message
substrate read <message_id>

# Check status
substrate status
```

### Claude Code Integration

Add to your Claude Code hooks configuration:

```json
{
  "hooks": {
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "substrate identity ensure --session-id $CLAUDE_SESSION_ID --project \"$PWD\" && substrate poll --format context"
      }]
    }],
    "UserPromptSubmit": [{
      "hooks": [{
        "type": "command",
        "command": "substrate poll --format context --quiet"
      }]
    }]
  }
}
```

See `docs/claude-integration/` for complete configuration examples.

## Development

```bash
make test           # Run tests
make test-cover     # Run with coverage
make lint           # Run linter
make sqlc           # Regenerate database code
make help           # Show all targets
```

### Requirements

- Go 1.22+
- CGO enabled (for SQLite FTS5)
- SQLite 3

### Project Structure

```
subtrate/
├── cmd/
│   ├── substrate/      # CLI tool
│   └── substrated/     # MCP daemon
├── internal/
│   ├── agent/          # Agent registry and identity
│   ├── db/             # Database layer (sqlc)
│   ├── mail/           # Mail service with actor pattern
│   └── mcp/            # MCP server and tools
├── docs/
│   └── claude-integration/  # Hooks and skill definitions
├── Makefile
└── CLAUDE.md           # AI assistant guide
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                Claude Code Agents                    │
│  (via CLI commands or MCP tools)                    │
└─────────────────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌───────────────┐      ┌───────────────┐
│   substrate   │      │  substrated   │
│   (CLI)       │      │  (MCP Server) │
└───────────────┘      └───────────────┘
        │                       │
        └───────────┬───────────┘
                    ▼
        ┌───────────────────────┐
        │    Mail Service       │
        │  (Actor Pattern)      │
        └───────────────────────┘
                    │
                    ▼
        ┌───────────────────────┐
        │   SQLite Database     │
        │   (WAL mode, FTS5)    │
        └───────────────────────┘
```

## Message States

Messages follow a state machine with these states:

- **unread** → Initial state
- **read** → Message has been viewed
- **starred** → Marked for attention
- **snoozed** → Hidden until a specified time
- **archived** → Removed from inbox
- **trash** → Pending deletion

## License

MIT
