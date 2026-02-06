# Implementation Plan: Subtrate - Agent Command Center

**Generated**: 2026-01-28
**Input type**: text (ideate session)
**Source**: direct input with interview refinement

## Summary

Subtrate is a central command center for managing Claude Code agents, providing a mail/messaging system with pub/sub capabilities, threaded conversations, and log-based queue semantics. The system consists of three components: an MCP server for agent communication, a Go backend with actor-based concurrency, and a hybrid HTMX/React frontend with Gmail-like UI.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Backend Language | Go | User preference, matches existing patterns in darepo-client |
| Database | SQLite with sqlc | Simple, embedded, proven patterns from darepo-client |
| MCP Server | modelcontextprotocol/go-sdk | Official SDK, Go native |
| Concurrency | Actor system from darepo-client | Type-safe, proven, supports request/response and fire-and-forget |
| State Machines | ProtoFSM from darepo-client | Thread/message lifecycle management with persistence |
| IPC | gRPC (MCP<->Backend), HTTP (Frontend) | Clean separation, typed contracts |
| Frontend | HTMX + React hybrid | HTMX for most pages, React for complex inbox/threading UI |
| Message Queue | Log-based with offsets + acknowledgments | Supports both streaming consumption and explicit ack workflow |
| Agent Identity | Named agents + project/session + topic subscriptions | Flexible addressing like NATS JetStream |
| Mail Checking | Multiple hooks (SessionStart, Stop, UserPromptSubmit, PreCompact) | Comprehensive coverage for agent awareness |
| CLI Tool | Full command set | Agents can use CLI directly or via MCP tools |

## Technical Approach

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (HTMX + React)                  │
│                              HTTP API                            │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Backend (Go)                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ HTTP Server │  │ gRPC Server │  │     Actor System        │  │
│  │  (Frontend) │  │ (MCP/CLI)   │  │  ┌─────────────────┐    │  │
│  └─────────────┘  └─────────────┘  │  │ ThreadActor     │    │  │
│         │                │          │  │ (ProtoFSM)      │    │  │
│         └────────────────┴──────────┤  ├─────────────────┤    │  │
│                                     │  │ SubscriptionMgr │    │  │
│                                     │  │ (Topics/Routing)│    │  │
│                                     │  ├─────────────────┤    │  │
│                                     │  │ NotificationMgr │    │  │
│                                     │  │ (Change events) │    │  │
│                                     │  └─────────────────┘    │  │
│                                     └─────────────────────────┘  │
│                                              │                   │
│                                              ▼                   │
│                          ┌─────────────────────────────┐         │
│                          │    SQLite (WAL mode)        │         │
│                          │    sqlc generated queries   │         │
│                          └─────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
        ▲                            ▲
        │ stdio/HTTP                 │ gRPC
        │                            │
┌───────┴───────┐            ┌───────┴───────┐
│  MCP Server   │            │   CLI Tool    │
│ (per-agent or │            │ (subtrate-cli)│
│   shared)     │            │               │
└───────────────┘            └───────────────┘
        ▲                            ▲
        │                            │
   Claude Code                  Claude Code
     Agents                       Agents
```

### Core Data Model

**Agents** (named entities that send/receive mail):
- Unique name (memorable, like "GreenCastle")
- Optional project binding
- Session tracking (current active session)
- Subscription list (topics they follow)
- Consumer offsets per topic

**Topics** (pub/sub channels):
- Name (e.g., "project/myapp/notifications", "agent/GreenCastle/inbox")
- Type: direct (1:1), broadcast (1:many), queue (load-balanced)
- Retention policy (time-based, count-based)

**Messages** (individual mail items):
- Unique ID, thread_id (for grouping)
- Sender agent, recipient(s) or topic
- Subject, body (markdown)
- Priority: urgent/normal/low
- Deadline (optional ack-by timestamp)
- Attachments (JSON blob)
- Log offset (monotonic sequence number per topic)
- Created timestamp

**Thread States** (ProtoFSM managed):
- StateUnread → StateRead → StateArchived
- StateStarred (parallel state)
- StateSnoozed (with wake time) → StateUnread
- StateTrash → (purge after TTL)

**Consumer Offsets** (log-based consumption):
- Agent ID + Topic ID → last_read_offset
- Enables "get all messages since X" queries
- Acknowledgments tracked separately for ack_required messages

### Database Schema (SQLite + sqlc)

```sql
-- Agents table
CREATE TABLE agents (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    project_key TEXT,
    current_session_id TEXT,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL
);

-- Topics table
CREATE TABLE topics (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    topic_type TEXT NOT NULL CHECK (topic_type IN ('direct', 'broadcast', 'queue')),
    retention_seconds INTEGER DEFAULT 604800, -- 7 days
    created_at INTEGER NOT NULL
);

-- Subscriptions (agent -> topic)
CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    subscribed_at INTEGER NOT NULL,
    UNIQUE(agent_id, topic_id)
);

-- Messages table (log-structured)
CREATE TABLE messages (
    id INTEGER PRIMARY KEY,
    thread_id TEXT NOT NULL,
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    log_offset INTEGER NOT NULL, -- Per-topic monotonic sequence
    sender_id INTEGER NOT NULL REFERENCES agents(id),
    subject TEXT NOT NULL,
    body_md TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('urgent', 'normal', 'low')),
    deadline_at INTEGER, -- Optional ack deadline
    attachments TEXT, -- JSON blob
    created_at INTEGER NOT NULL,
    UNIQUE(topic_id, log_offset)
);

-- Message recipients (for direct messages)
CREATE TABLE message_recipients (
    message_id INTEGER NOT NULL REFERENCES messages(id),
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    state TEXT NOT NULL DEFAULT 'unread' CHECK (state IN ('unread', 'read', 'starred', 'snoozed', 'archived', 'trash')),
    snoozed_until INTEGER,
    read_at INTEGER,
    acked_at INTEGER,
    PRIMARY KEY(message_id, agent_id)
);

-- Consumer offsets (log-based consumption tracking)
CREATE TABLE consumer_offsets (
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    last_offset INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY(agent_id, topic_id)
);

-- Full-text search
CREATE VIRTUAL TABLE messages_fts USING fts5(subject, body_md, content=messages, content_rowid=id);
```

### Actor System Design

**ThreadActor** (one per active thread):
- Manages thread state via ProtoFSM
- Handles: read, star, snooze, archive, trash transitions
- Emits: persistence events, notification events
- Terminal states: archived, trash (after TTL)

**SubscriptionManager**:
- Routes messages to subscribed agents
- Handles topic creation/deletion
- Load balances for queue-type topics

**NotificationManager**:
- Tracks change events for polling queries
- Maintains per-agent "unread count" caches
- Signals hooks/CLI when new mail arrives

**MessageRouter**:
- Receives incoming messages
- Determines target topics/agents
- Assigns log offsets
- Dispatches to ThreadActors

### MCP Server Tools

Using `modelcontextprotocol/go-sdk`:

```go
// Core mail tools
mcp.AddTool(server, &mcp.Tool{Name: "send_mail"}, SendMail)
mcp.AddTool(server, &mcp.Tool{Name: "fetch_inbox"}, FetchInbox)
mcp.AddTool(server, &mcp.Tool{Name: "read_message"}, ReadMessage)
mcp.AddTool(server, &mcp.Tool{Name: "read_thread"}, ReadThread)
mcp.AddTool(server, &mcp.Tool{Name: "ack_message"}, AckMessage)
mcp.AddTool(server, &mcp.Tool{Name: "mark_read"}, MarkRead)
mcp.AddTool(server, &mcp.Tool{Name: "star_message"}, StarMessage)
mcp.AddTool(server, &mcp.Tool{Name: "snooze_message"}, SnoozeMessage)
mcp.AddTool(server, &mcp.Tool{Name: "archive_message"}, ArchiveMessage)
mcp.AddTool(server, &mcp.Tool{Name: "trash_message"}, TrashMessage)

// Pub/sub tools
mcp.AddTool(server, &mcp.Tool{Name: "subscribe"}, Subscribe)
mcp.AddTool(server, &mcp.Tool{Name: "unsubscribe"}, Unsubscribe)
mcp.AddTool(server, &mcp.Tool{Name: "list_topics"}, ListTopics)
mcp.AddTool(server, &mcp.Tool{Name: "publish"}, Publish)

// Query tools
mcp.AddTool(server, &mcp.Tool{Name: "search"}, Search)
mcp.AddTool(server, &mcp.Tool{Name: "get_status"}, GetStatus)
mcp.AddTool(server, &mcp.Tool{Name: "poll_changes"}, PollChanges)

// Agent management
mcp.AddTool(server, &mcp.Tool{Name: "register_agent"}, RegisterAgent)
mcp.AddTool(server, &mcp.Tool{Name: "whoami"}, WhoAmI)
```

### CLI Tool Commands

```bash
subtrate-cli inbox [--agent NAME] [--limit N] [--unread-only]
subtrate-cli send --to AGENT|TOPIC --subject "..." --body "..."
subtrate-cli read MESSAGE_ID
subtrate-cli thread THREAD_ID
subtrate-cli ack MESSAGE_ID
subtrate-cli star MESSAGE_ID
subtrate-cli snooze MESSAGE_ID --until "2h" | --until "2026-01-29T10:00:00"
subtrate-cli archive MESSAGE_ID
subtrate-cli trash MESSAGE_ID

subtrate-cli subscribe TOPIC
subtrate-cli unsubscribe TOPIC
subtrate-cli topics [--subscribed]
subtrate-cli publish TOPIC --subject "..." --body "..."

subtrate-cli search "query string" [--in TOPIC]
subtrate-cli status [--agent NAME]
subtrate-cli watch [--agent NAME]  # Long-running, prints new messages
subtrate-cli poll [--agent NAME] [--since OFFSET]  # One-shot check

subtrate-cli agent register NAME [--project KEY]
subtrate-cli agent list
subtrate-cli agent whoami
```

### Claude Code Integration

**Hooks Configuration** (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [{
          "type": "command",
          "command": "subtrate-cli identity ensure --session-id $CLAUDE_SESSION_ID --project $CLAUDE_PROJECT_DIR && subtrate-cli poll --format context"
        }]
      },
      {
        "matcher": "resume",
        "hooks": [{
          "type": "command",
          "command": "subtrate-cli identity restore --session-id $CLAUDE_SESSION_ID && subtrate-cli poll --format context"
        }]
      },
      {
        "matcher": "compact",
        "hooks": [{
          "type": "command",
          "command": "subtrate-cli identity restore --session-id $CLAUDE_SESSION_ID && subtrate-cli poll --format context --include-summary"
        }]
      }
    ],
    "UserPromptSubmit": [{
      "hooks": [{
        "type": "command",
        "command": "subtrate-cli poll --format context --quiet"
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "prompt",
        "prompt": "Check if agent has unread urgent mail that needs attention before stopping. Input: $ARGUMENTS"
      }]
    }],
    "PreCompact": [{
      "hooks": [{
        "type": "command",
        "command": "subtrate-cli identity save --session-id $CLAUDE_SESSION_ID && subtrate-cli status --format summary"
      }]
    }]
  }
}
```

### Agent Identity Persistence

**The Problem**: Claude Code sessions can be interrupted, resumed, or compacted. The agent needs to maintain consistent identity across these events.

**Solution**: File-based identity persistence with session binding.

**Identity Storage** (`~/.subtrate/identities/`):
```
~/.subtrate/
├── identities/
│   ├── by-session/
│   │   └── {session_id}.json    # Session -> Agent mapping
│   └── by-project/
│       └── {project_hash}.json  # Project -> Default agent
├── config.yaml                   # Global config
└── subtrate.db                   # SQLite database
```

**Identity File Format** (`by-session/{session_id}.json`):
```json
{
  "session_id": "abc123",
  "agent_name": "GreenCastle",
  "project_key": "/Users/roasbeef/myproject",
  "created_at": "2026-01-28T10:00:00Z",
  "last_active_at": "2026-01-28T12:30:00Z",
  "consumer_offsets": {
    "inbox": 42,
    "project/myproject/notifications": 15
  }
}
```

**Identity Resolution Flow**:
1. **On startup**: `subtrate-cli identity ensure` checks for existing identity
   - If session_id has saved identity → restore it
   - If project has default agent → use that
   - Otherwise → create new agent with memorable name
2. **On resume/compact**: `subtrate-cli identity restore` loads saved state
3. **On PreCompact**: `subtrate-cli identity save` persists current offsets
4. **Current identity**: Stored in env var via `CLAUDE_ENV_FILE` mechanism

**CLI Identity Commands**:
```bash
subtrate-cli identity ensure --session-id ID [--project DIR]  # Create or restore
subtrate-cli identity restore --session-id ID                 # Restore from file
subtrate-cli identity save --session-id ID                    # Persist current state
subtrate-cli identity set-default --project DIR --agent NAME  # Set project default
subtrate-cli identity current                                 # Show current identity
subtrate-cli identity list                                    # List all known identities
```

**Database Schema Addition**:
```sql
-- Session identity mapping (also in SQLite for cross-CLI access)
CREATE TABLE session_identities (
    session_id TEXT PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    project_key TEXT,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL
);

CREATE INDEX idx_session_identities_agent ON session_identities(agent_id);
CREATE INDEX idx_session_identities_project ON session_identities(project_key);
```

### Claude Skill Definition

**Skill File** (`~/.claude/skills/mail/SKILL.md`):

```yaml
---
name: mail
description: Check and manage agent mail via Subtrate command center
allowed_tools:
  - Bash
hooks:
  PostToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "subtrate-cli poll --quiet --format context-if-new"
          once: true
---

# Mail Management Skill

You have access to the Subtrate mail system for agent-to-agent and user-to-agent communication.

## Quick Commands

Check your inbox:
\`\`\`bash
subtrate-cli inbox
\`\`\`

Read a message:
\`\`\`bash
subtrate-cli read <message_id>
\`\`\`

Send a message:
\`\`\`bash
subtrate-cli send --to <agent_or_topic> --subject "Subject" --body "Message body"
\`\`\`

Reply to a thread:
\`\`\`bash
subtrate-cli reply <thread_id> --body "Reply content"
\`\`\`

## Message Management

\`\`\`bash
subtrate-cli ack <message_id>        # Acknowledge receipt
subtrate-cli star <message_id>       # Star for later
subtrate-cli snooze <message_id> --until "2h"  # Snooze
subtrate-cli archive <message_id>    # Archive
\`\`\`

## Pub/Sub

\`\`\`bash
subtrate-cli subscribe <topic>       # Subscribe to topic
subtrate-cli publish <topic> --subject "..." --body "..."  # Publish
subtrate-cli topics --subscribed     # List subscriptions
\`\`\`

## Search

\`\`\`bash
subtrate-cli search "query"          # Full-text search
subtrate-cli search "query" --in <topic>  # Search in topic
\`\`\`

## Status

\`\`\`bash
subtrate-cli status                  # Show mail status summary
subtrate-cli identity current        # Show current agent identity
\`\`\`

## When to Check Mail

- At the start of work sessions
- Before making significant decisions (user may have sent guidance)
- When blocked and waiting for input
- Before finishing a task (check for follow-up requests)

## Priority Handling

- **URGENT**: Address immediately, before other work
- **NORMAL**: Process in order received
- **LOW**: Can be deferred or batched

Messages with deadlines should be acknowledged before the deadline expires.
```

### Frontend Design (Gmail-like)

**HTMX Pages**:
- `/` - Dashboard with unread counts, recent activity
- `/settings` - Agent configuration, topic management
- `/agents` - List of known agents
- `/topics` - Topic browser

**React Components** (embedded in HTMX pages):
- `<InboxView />` - Main inbox with threading, filters, bulk actions
- `<ThreadView />` - Conversation view with message history
- `<ComposeModal />` - New message composition
- `<SearchResults />` - Full-text search with highlighting

**Design System**:
- Gmail-inspired layout: sidebar (labels/topics), main list, preview pane
- Priority indicators: red (urgent), yellow (deadline), gray (low)
- Thread grouping with expansion
- Real-time updates via SSE or polling

### Project Structure

```
subtrate/
├── cmd/
│   ├── subtrate-backend/     # Main backend server
│   │   └── main.go
│   ├── subtrate-mcp/         # MCP server binary
│   │   └── main.go
│   └── subtrate-cli/         # CLI tool
│       └── main.go
├── internal/
│   ├── db/
│   │   ├── migrations/       # SQL migration files
│   │   │   ├── 000001_init.up.sql
│   │   │   └── 000001_init.down.sql
│   │   ├── queries/          # SQL query files for sqlc
│   │   │   ├── agents.sql
│   │   │   ├── messages.sql
│   │   │   ├── topics.sql
│   │   │   └── subscriptions.sql
│   │   ├── sqlc/             # Generated code (do not edit)
│   │   │   ├── db.go
│   │   │   ├── models.go
│   │   │   └── queries.sql.go
│   │   ├── store.go          # Store wrapper with transactions
│   │   └── sqlite.go         # SQLite connection config
│   ├── mail/
│   │   ├── service.go        # Core mail service
│   │   ├── thread_actor.go   # Thread lifecycle actor
│   │   ├── thread_states.go  # ProtoFSM states
│   │   ├── thread_events.go  # FSM events
│   │   └── router.go         # Message routing
│   ├── pubsub/
│   │   ├── manager.go        # Subscription management
│   │   └── topic.go          # Topic operations
│   ├── agent/
│   │   ├── registry.go       # Agent registration
│   │   └── identity.go       # Identity resolution & persistence
│   ├── api/
│   │   ├── grpc/             # gRPC service definitions
│   │   │   ├── mail.proto
│   │   │   └── mail_grpc.pb.go
│   │   └── http/             # HTTP handlers for frontend
│   │       ├── handlers.go
│   │       └── middleware.go
│   └── mcp/
│       ├── server.go         # MCP server setup
│       └── tools.go          # Tool implementations
├── web/
│   ├── templates/            # Go templates for HTMX
│   │   ├── layout.html
│   │   ├── inbox.html
│   │   └── partials/
│   ├── static/
│   │   ├── css/
│   │   └── js/
│   └── react/                # React components
│       ├── src/
│       │   ├── InboxView.tsx
│       │   ├── ThreadView.tsx
│       │   └── ComposeModal.tsx
│       └── package.json
├── scripts/
│   └── gen_sqlc_docker.sh    # sqlc generation script
├── go.mod
├── go.sum
├── sqlc.yaml                 # Points to internal/db/{migrations,queries,sqlc}
└── README.md
```

### go.mod with replace directives

```go
module github.com/roasbeef/subtrate

go 1.23

require (
    github.com/modelcontextprotocol/go-sdk v1.0.0
    github.com/lightninglabs/darepo-client v0.0.0
    google.golang.org/grpc v1.60.0
    github.com/mattn/go-sqlite3 v1.14.22
    // ... other deps
)

replace github.com/lightninglabs/darepo-client => /Users/roasbeef/gocode/src/github.com/lightninglabs/darepo-client
```

## Implementation Tasks

### Phase 1: Foundation
- [ ] Initialize Go module with replace directives for darepo-client
- [ ] Set up sqlc configuration and initial schema
- [ ] Create migration files (000001_init.up.sql)
- [ ] Generate sqlc code
- [ ] Implement basic Store wrapper with transaction support
- [ ] Set up actor system integration from darepo-client
- [ ] Create basic project structure

### Phase 2: Core Backend
- [ ] Implement Agent registry and identity resolution
- [ ] Implement Topic management (CRUD, types)
- [ ] Implement Message model and storage
- [ ] Implement Subscription management
- [ ] Implement Consumer offset tracking
- [ ] Create MessageRouter actor for routing
- [ ] Implement full-text search via FTS5

### Phase 3: Thread State Machine
- [ ] Define thread states (StateUnread, StateRead, etc.)
- [ ] Define thread events (ReadEvent, StarEvent, etc.)
- [ ] Implement ProtoFSM state machine for threads
- [ ] Create ThreadActor with FSM integration
- [ ] Implement persistence via outbox events
- [ ] Add snooze scheduling (background goroutine to wake snoozed threads)

### Phase 4: gRPC API
- [ ] Define protobuf service definitions
- [ ] Generate Go code from protos
- [ ] Implement gRPC server
- [ ] Wire up to actor system
- [ ] Add authentication (agent identity)

### Phase 5: MCP Server
- [ ] Set up MCP server with go-sdk
- [ ] Implement core mail tools (send, fetch, read, ack)
- [ ] Implement state transition tools (star, snooze, archive, trash)
- [ ] Implement pub/sub tools (subscribe, publish, topics)
- [ ] Implement query tools (search, status, poll)
- [ ] Add support for both stdio and HTTP transports

### Phase 6: CLI Tool
- [ ] Create CLI structure with cobra or similar
- [ ] Implement inbox, send, read, thread commands
- [ ] Implement ack, star, snooze, archive, trash commands
- [ ] Implement subscribe, unsubscribe, topics, publish commands
- [ ] Implement search, status, watch, poll commands
- [ ] Implement agent register, list, whoami commands
- [ ] Add output formatters (json, table, context for hooks)

### Phase 7: Claude Code Integration
- [ ] Implement `identity ensure` command (create or restore agent identity)
- [ ] Implement `identity restore` command (load from session file)
- [ ] Implement `identity save` command (persist offsets and state)
- [ ] Implement `identity set-default` command (project default agent)
- [ ] Implement `identity current` and `identity list` commands
- [ ] Create identity file storage structure (`~/.subtrate/identities/`)
- [ ] Add `session_identities` table to schema
- [ ] Create hook scripts for SessionStart (startup/resume/compact matchers)
- [ ] Create hook scripts for UserPromptSubmit, Stop, PreCompact
- [ ] Test hook integration with identity persistence across compaction
- [ ] Create example settings.json configuration
- [ ] Create `/mail` skill definition (`~/.claude/skills/mail/SKILL.md`)
- [ ] Create skill examples (send-report.md, handle-urgent.md)
- [ ] Test skill invocation and mail checking

### Phase 8: Frontend - HTMX
- [ ] Set up HTTP server with Go templates
- [ ] Create base layout template
- [ ] Implement dashboard page
- [ ] Implement settings page
- [ ] Implement agents list page
- [ ] Implement topics browser page
- [ ] Add HTMX partials for dynamic updates

### Phase 9: Frontend - React Components
- [ ] Set up React build pipeline (esbuild or vite)
- [ ] Implement InboxView component with threading
- [ ] Implement ThreadView component
- [ ] Implement ComposeModal component
- [ ] Implement SearchResults component
- [ ] Style with Gmail-inspired design
- [ ] Add SSE or polling for real-time updates

### Phase 10: Polish & Documentation
- [ ] Add comprehensive error handling
- [ ] Add logging throughout
- [ ] Write README with setup instructions
- [ ] Create example configurations
- [ ] Add tests for critical paths

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Private repo dependency (darepo-client) | Medium | Use go.mod replace; document setup steps |
| Agent wake-up limitations in Claude Code | Medium | Multiple hooks provide good coverage; document limitations |
| SQLite concurrency under load | Low | WAL mode + proper connection pooling handles typical workloads |
| MCP SDK maturity | Low | SDK is at v1.0.0; fallback to alternative Go SDKs if needed |
| Frontend complexity (HTMX + React hybrid) | Medium | Clear boundaries; React only for complex components |

## Testing Strategy

- [ ] Unit tests for actor message handling
- [ ] Unit tests for ProtoFSM state transitions
- [ ] Integration tests for gRPC API
- [ ] Integration tests for MCP tools
- [ ] End-to-end tests: send mail via MCP → receive via CLI
- [ ] Hook integration tests with mock Claude Code environment
- [ ] Frontend component tests (React)
- [ ] Full flow tests: agent registration → subscribe → send → receive → ack

## Files to Modify

Since this is a new project, all files will be created. Key files:

- `go.mod` - Module definition with replace directives
- `sqlc.yaml` - sqlc configuration pointing to internal/db/
- `internal/db/migrations/000001_init.up.sql` - Initial schema
- `internal/db/queries/*.sql` - SQL queries for sqlc
- `internal/db/store.go` - Database store wrapper with transactions
- `internal/db/sqlite.go` - SQLite connection and WAL config
- `internal/mail/service.go` - Core mail service
- `internal/mail/thread_actor.go` - Thread lifecycle actor
- `internal/mail/thread_states.go` - ProtoFSM states
- `internal/agent/identity.go` - Agent identity persistence
- `internal/mcp/server.go` - MCP server setup
- `internal/mcp/tools.go` - MCP tool implementations
- `cmd/subtrate-backend/main.go` - Backend entry point
- `cmd/subtrate-mcp/main.go` - MCP server entry point
- `cmd/subtrate-cli/main.go` - CLI entry point
- `~/.claude/skills/mail/SKILL.md` - Claude skill definition

## Notes for Implementation

1. **Actor System Import**: The actor system from darepo-client uses generics extensively. Ensure Go 1.22+ for full compatibility.

2. **ProtoFSM Integration**: Thread states should emit VTXOStatusUpdate-style outbox events for persistence. Follow the pattern in darepo-client/vtxo.

3. **Hook Context Injection**: The CLI poll command should output in a format that Claude Code hooks can inject as context. Use JSON with `hookSpecificOutput.additionalContext` for structured injection.

4. **MCP Transport Modes**: Support both stdio (for per-agent processes) and HTTP/SSE (for shared server mode). The go-sdk supports custom transports via the jsonrpc package.

5. **Log Offset Assignment**: Use a SQLite sequence or MAX(offset)+1 with proper locking to ensure monotonic offsets per topic.

6. **Snooze Implementation**: A background goroutine should periodically check for snoozed messages past their wake time and transition them back to unread.

7. **Topic Naming Convention**: Consider hierarchical naming like `agent/{name}/inbox`, `project/{key}/notifications`, `broadcast/all` for clear routing semantics.

## Verification

To test the implementation end-to-end:

1. **Start backend**: `go run ./cmd/subtrate-backend`
2. **Register agent**: `subtrate-cli agent register TestAgent`
3. **Send mail**: `subtrate-cli send --to TestAgent --subject "Hello" --body "Test message"`
4. **Check inbox**: `subtrate-cli inbox --agent TestAgent`
5. **Read message**: `subtrate-cli read <message_id>`
6. **Test MCP**: Configure Claude Code with MCP server, use send_mail tool
7. **Test hooks**: Run Claude Code session, verify mail check on start
8. **Test frontend**: Open browser to backend HTTP port, verify Gmail-like UI
