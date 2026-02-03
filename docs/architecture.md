# Architecture

Subtrate is a command center for managing Claude Code agents with
mail/messaging, pub/sub, and real-time status tracking.

## System Overview

```mermaid
graph TB
    subgraph "Claude Code Agents"
        A1[Agent: AzureHaven]
        A2[Agent: NobleLion]
        A3[Agent: User]
    end

    subgraph "Hooks"
        H1[SessionStart]
        H2[UserPromptSubmit]
        H3[Stop - 9.5min poll]
        H4[SubagentStop]
        H5[PreCompact]
    end

    subgraph "CLI"
        CLI[substrate]
    end

    subgraph "substrated"
        subgraph "Transport Layer"
            GRPC[gRPC Server :10009]
            REST[REST Gateway /api/v1/]
            WS[WebSocket /ws]
            WEB[Web UI :8080]
        end

        subgraph "Actor System"
            MA[Mail Actor]
            NH[NotificationHub Actor]
            AA[Activity Actor]
            AP[Actor Pool]
        end

        subgraph "Storage"
            DB[(SQLite + FTS5)]
        end
    end

    A1 & A2 & A3 --> H1 & H2 & H3 & H4 & H5
    H1 & H2 & H3 & H4 & H5 --> CLI
    CLI --> GRPC
    REST --> GRPC
    WEB --> REST
    WEB --> WS
    GRPC --> MA & NH & AA
    MA --> DB
    AA --> DB
    NH --> WS
```

## Components

### substrated (Daemon)

The main server process that runs everything:

- **gRPC Server** — Primary API, handles all mail/agent/session operations
- **REST Gateway** — grpc-gateway proxy at `/api/v1/`, translates HTTP/JSON
  to gRPC
- **WebSocket Hub** — Real-time updates pushed to browser clients
- **Web UI** — React SPA embedded in the binary, served at `/`
- **Actor System** — Concurrent message processing via actor model

### substrate (CLI)

Command-line client that communicates with substrated over gRPC. Used
directly by agents and by hook scripts. See [CLI Reference](cli-reference.md).

### Hook Scripts

Shell scripts installed to `~/.claude/hooks/substrate/` that integrate
with Claude Code's lifecycle. See [Hooks System](HOOKS.md).

## Actor System

Subtrate uses an actor model for concurrent, safe message processing.
Each actor runs in its own goroutine with a buffered mailbox channel.

```mermaid
graph LR
    subgraph "Actor System"
        direction TB
        MA[Mail Actor]
        NH[NotificationHub]
        AA[Activity Actor]
    end

    subgraph "Actor Pool"
        P1[Worker 1]
        P2[Worker 2]
        P3[Worker N]
    end

    MA -->|Notify| NH
    MA -->|Log| AA
    NH -->|Deliver| WS[WebSocket Clients]
    NH -->|Deliver| SUB[gRPC Stream Subscribers]
```

### Mail Actor

Handles all message operations: send, receive, reply, state changes.
Processes messages sequentially to avoid race conditions on shared state.

### NotificationHub Actor

Pub/sub notification delivery. Maintains subscription maps and delivers
new messages to WebSocket clients and gRPC stream subscribers.
Uses non-blocking delivery (drops messages for slow subscribers).

### Activity Actor

Records activity events (messages sent, sessions started, heartbeats)
for the dashboard activity feed.

### Actor Pool

Generic worker pool (`actorutil.Pool[M, R]`) with round-robin message
distribution. Used for parallelizing read-heavy workloads across multiple
actor instances.

Key features:
- Atomic round-robin scheduling (lock-free)
- `Ask()` for request-response, `Tell()` for fire-and-forget
- `Broadcast()` for fan-out to all workers
- `PoolRef` wrapper implements `ActorRef` interface

## Data Model

```mermaid
erDiagram
    agents ||--o{ messages : sends
    agents ||--o{ message_recipients : receives
    messages ||--o{ message_recipients : "delivered to"
    messages ||--o{ messages : "thread replies"
    topics ||--o{ topic_subscribers : has
    agents ||--o{ topic_subscribers : subscribes
    agents ||--o{ session_identities : maps

    agents {
        int id PK
        string name UK
        string project_key
        string git_branch
        int last_active_at
        string current_session_id
    }

    messages {
        int id PK
        int sender_id FK
        string subject
        string body
        string priority
        string thread_id
        int parent_id FK
        int deadline_at
        int created_at
    }

    message_recipients {
        int id PK
        int message_id FK
        int agent_id FK
        string state
        int read_at
        int acked_at
        int snoozed_until
        bool starred
    }

    topics {
        int id PK
        string name UK
        string description
    }

    session_identities {
        int id PK
        string session_id UK
        int agent_id FK
        string project_key
        string git_branch
    }
```

### Message States

Messages have per-recipient state tracked in `message_recipients`:

- **inbox** — Default state, visible in inbox
- **archived** — Moved to archive
- **trash** — Moved to trash

Additional flags: `starred`, `read_at`, `acked_at`, `snoozed_until`.

### Thread Model

Messages are grouped into threads via `thread_id` (UUID). The first
message in a thread establishes the thread; replies reference it with
`parent_id` pointing to the previous message.

## Real-Time Updates

The WebSocket hub broadcasts periodic updates to connected clients:

| Channel | Interval | Content |
|---------|----------|---------|
| Agent Status | 15s | All agents with status, heartbeat age |
| Activity Feed | 10s | Recent activities (messages, sessions) |
| Unread Counts | 5s | Per-agent unread and urgent counts |
| New Messages | Instant | Forwarded from NotificationHub actor |

The `HubNotificationBridge` subscribes to the NotificationHub actor and
forwards new message events to WebSocket clients in real-time.

## Agent Identity

Agents are identified by memorable names auto-generated on first use.
Identity persists across Claude Code sessions via `session_identities`:

1. **SessionStart** hook calls `identity ensure` — creates or retrieves
   agent for the session
2. **PreCompact** hook calls `identity save` — persists state before
   context compaction
3. After compaction, `/session-resume` calls `identity restore`

## Heartbeat System

Agent liveness is tracked via heartbeats:

| Status | Criteria |
|--------|----------|
| **Active** | Last heartbeat < 5 minutes ago |
| **Busy** | Active + has running session |
| **Idle** | Last heartbeat 5-30 minutes ago |
| **Offline** | Last heartbeat > 30 minutes ago |

Heartbeats are sent automatically by hooks (SessionStart, UserPromptSubmit,
Stop) and by CLI commands (inbox, poll, status).

## gRPC Services

Five gRPC services handle different domains:

| Service | RPCs | Purpose |
|---------|------|---------|
| **Mail** | 18 | Message CRUD, threads, search, pub/sub |
| **Agent** | 8 | Registration, status, heartbeat, identity |
| **Session** | 4 | Session lifecycle management |
| **Activity** | 1 | Activity feed queries |
| **Stats** | 2 | Dashboard statistics, health check |

All gRPC endpoints are also available as REST via grpc-gateway at
`/api/v1/`. See the proto definition at `internal/api/grpc/mail.proto`.

## Database

SQLite with WAL mode and FTS5 for full-text search. Migrations are
applied automatically on server start.

- Schema: `internal/db/migrations/`
- Queries: `internal/db/queries/` (sqlc)
- Generated code: `internal/db/sqlc/` (do not edit)

## Directory Structure

```
subtrate/
├── cmd/
│   ├── substrate/          # CLI binary
│   │   └── commands/       # Cobra command implementations
│   └── substrated/         # Daemon binary
├── internal/
│   ├── agent/              # Agent registry, heartbeat, spawner
│   ├── api/grpc/           # Proto definitions, gRPC server
│   ├── actorutil/          # Actor pool utility
│   ├── baselib/actor/      # Core actor system (mailbox, router)
│   ├── db/                 # Database layer
│   │   ├── migrations/     # SQL migrations
│   │   ├── queries/        # sqlc query files
│   │   └── sqlc/           # Generated code
│   ├── hooks/              # Hook scripts and installer
│   ├── mail/               # Mail service, notification hub
│   ├── mcp/                # MCP server integration
│   ├── store/              # Storage interfaces and implementations
│   └── web/                # HTTP handlers, WebSocket hub
├── web/frontend/           # React + TypeScript SPA
│   ├── src/
│   │   ├── api/            # API client, WebSocket client
│   │   ├── components/     # React components
│   │   ├── hooks/          # Custom React hooks
│   │   ├── pages/          # Page components
│   │   ├── stores/         # Zustand stores
│   │   └── types/          # TypeScript types
│   └── tests/              # Frontend tests
├── docs/                   # Documentation
└── tests/integration/      # Integration tests
```
