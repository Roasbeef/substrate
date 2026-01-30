# NATS RPC Specification for Subtrate

This document provides a detailed specification for migrating Subtrate from gRPC to NATS-based RPC, including JetStream integration for durable messaging.

## Table of Contents

1. [Overview](#overview)
2. [NATS Semantics Deep Dive](#nats-semantics-deep-dive)
3. [Architecture Decision: JetStream Required](#architecture-decision-jetstream-required)
4. [Subject Hierarchy Design](#subject-hierarchy-design)
5. [JetStream Streams & Consumers](#jetstream-streams--consumers)
6. [SQLite Integration Changes](#sqlite-integration-changes)
7. [Embedded NATS Server](#embedded-nats-server)
8. [RPC Protocol Design](#rpc-protocol-design)
9. [CLI Changes](#cli-changes)
10. [Server Changes](#server-changes)
11. [Migration Strategy](#migration-strategy)
12. [Integration Test Changes](#integration-test-changes)
13. [Security Considerations](#security-considerations)

---

## Overview

### Current Architecture

```
┌─────────────┐     gRPC      ┌─────────────┐     SQLite     ┌─────────────┐
│  CLI Tool   │──────────────▶│  substrated │───────────────▶│  Database   │
│ (substrate) │               │   (daemon)  │                │  (*.db)     │
└─────────────┘               └─────────────┘                └─────────────┘
      │                              │
      │ Direct mode (fallback)       │ Actor System
      └──────────────────────────────┘
```

### Proposed Architecture

```
┌─────────────┐    NATS RPC    ┌─────────────┐    JetStream    ┌─────────────┐
│  CLI Tool   │───────────────▶│  substrated │◀───────────────▶│   Streams   │
│ (substrate) │                │   (daemon)  │                 │ (messages)  │
└─────────────┘                └─────────────┘                 └─────────────┘
      │                              │                               │
      │         ┌────────────────────┴────────────────────┐          │
      │         │          Embedded NATS Server           │          │
      │         │  ┌─────────────────────────────────┐    │          │
      └────────▶│  │  In-process nats-server/v2      │────┼──────────┘
                │  │  with JetStream enabled         │    │
                │  └─────────────────────────────────┘    │
                └─────────────────────────────────────────┘
                                    │
                                    ▼
                           ┌─────────────┐
                           │   SQLite    │
                           │  (agents,   │
                           │   offsets)  │
                           └─────────────┘
```

### Key Benefits

1. **Native pub/sub**: NATS subjects map directly to Subtrate topics
2. **Request-reply**: Built-in pattern replaces gRPC streaming complexity
3. **JetStream durability**: Message persistence with consumer offsets
4. **Embedded deployment**: Single binary with no external dependencies
5. **Subject-based routing**: Natural fit for agent inbox routing
6. **Horizontal scalability**: Future multi-node support via NATS clustering

---

## NATS Semantics Deep Dive

### Core NATS vs JetStream

| Feature | Core NATS | JetStream |
|---------|-----------|-----------|
| Delivery | At-most-once | At-least-once / exactly-once |
| Persistence | None (fire-and-forget) | Durable streams with replay |
| Consumer state | None | Server-tracked offsets |
| Request-reply | ✓ | ✓ (via streams) |
| Pub/sub | ✓ | ✓ (with durability) |
| Wildcards | ✓ | ✓ |
| Backpressure | None | Flow control |

### Core NATS Patterns

**Publish-Subscribe (fire-and-forget)**:
```
Publisher → NATS Server → All Subscribers
              │
              └─ Messages lost if no subscribers
```

**Request-Reply**:
```
Client ──Request──▶ NATS Server ──▶ Service
       ◀──Reply────             ◀──
              │
              └─ Automatic inbox for replies
```

**Queue Groups (load balancing)**:
```
Producer → NATS Server → Queue Group "workers"
                │            │
                └─▶ Worker 1 (gets some)
                └─▶ Worker 2 (gets others)
                └─▶ Worker 3 (gets rest)
```

### JetStream Concepts

**Streams**: Ordered, persistent sequence of messages
- Messages stored in append-only log (like Kafka)
- Configurable retention (time, size, message count)
- Subject-based filtering for what goes into stream

**Consumers**: Named position in a stream
- **Durable**: Server tracks position across disconnects
- **Ephemeral**: Position lost on disconnect
- **Push**: Server sends messages to subscriber
- **Pull**: Client explicitly fetches messages

**Consumer Acknowledgments**:
- `AckExplicit`: Client must ack each message
- `AckAll`: Acking message N implicitly acks 1..N-1
- `AckNone`: No acks required (at-most-once)

### Why Subtrate Needs JetStream

Subtrate's current semantics require durability:

| Subtrate Feature | NATS Requirement |
|------------------|------------------|
| Inbox messages persist until read | JetStream stream |
| Consumer offsets (PollChanges) | JetStream consumer position |
| Message state tracking | SQLite (per-recipient state) |
| Thread grouping | JetStream + SQLite (thread_id) |
| Priority ordering | JetStream stream ordering |
| Deadline acknowledgments | Consumer ack with timeout |

**Decision**: Use JetStream for all message storage, keeping SQLite for:
- Agent registry (identity)
- Per-recipient message state (read/starred/archived)
- Consumer offset checkpoints (backup to JetStream)
- Session identities
- Full-text search index

---

## Subject Hierarchy Design

NATS subjects use dot-separated hierarchies with wildcard support:
- `*` matches single token: `inbox.*` matches `inbox.alice`
- `>` matches multiple tokens: `topic.>` matches `topic.updates.urgent`

### Proposed Subject Structure

```
subtrate.
├── rpc.                           # Request-Reply RPC calls
│   ├── mail.                      # Mail service RPCs
│   │   ├── send                   # subtrate.rpc.mail.send
│   │   ├── fetch                  # subtrate.rpc.mail.fetch
│   │   ├── read                   # subtrate.rpc.mail.read
│   │   ├── update-state           # subtrate.rpc.mail.update-state
│   │   ├── ack                    # subtrate.rpc.mail.ack
│   │   ├── status                 # subtrate.rpc.mail.status
│   │   ├── poll                   # subtrate.rpc.mail.poll
│   │   ├── publish                # subtrate.rpc.mail.publish
│   │   ├── subscribe              # subtrate.rpc.mail.subscribe
│   │   ├── unsubscribe            # subtrate.rpc.mail.unsubscribe
│   │   ├── list-topics            # subtrate.rpc.mail.list-topics
│   │   └── search                 # subtrate.rpc.mail.search
│   └── agent.                     # Agent service RPCs
│       ├── register               # subtrate.rpc.agent.register
│       ├── get                    # subtrate.rpc.agent.get
│       ├── list                   # subtrate.rpc.agent.list
│       ├── ensure-identity        # subtrate.rpc.agent.ensure-identity
│       └── save-identity          # subtrate.rpc.agent.save-identity
│
├── inbox.                         # Agent inboxes (JetStream)
│   ├── <agent_name>               # subtrate.inbox.alice
│   └── <agent_name>               # subtrate.inbox.bob
│
├── topic.                         # Pub/sub topics (JetStream)
│   ├── <topic_name>               # subtrate.topic.announcements
│   └── <topic_name>.urgent        # subtrate.topic.updates.urgent
│
└── sys.                           # System events
    ├── heartbeat.<agent_name>     # subtrate.sys.heartbeat.alice
    ├── activity.<type>            # subtrate.sys.activity.commit
    └── notify.<agent_name>        # subtrate.sys.notify.alice
```

### Subject Mapping Examples

| Current Operation | NATS Subject | Pattern |
|-------------------|--------------|---------|
| Send to Alice | `subtrate.inbox.alice` | Publish to stream |
| Send to Bob | `subtrate.inbox.bob` | Publish to stream |
| Publish to "updates" | `subtrate.topic.updates` | Publish to stream |
| RPC: FetchInbox | `subtrate.rpc.mail.fetch` | Request-Reply |
| RPC: GetAgent | `subtrate.rpc.agent.get` | Request-Reply |
| Agent heartbeat | `subtrate.sys.heartbeat.alice` | Publish (no persist) |

---

## JetStream Streams & Consumers

### Stream Definitions

#### 1. INBOXES Stream
Stores all direct messages to agent inboxes.

```go
StreamConfig{
    Name:        "INBOXES",
    Description: "Agent inbox messages",
    Subjects:    []string{"subtrate.inbox.>"},
    Storage:     FileStorage,    // Persistent to disk
    Retention:   LimitsPolicy,   // Keep until limits hit
    MaxAge:      7 * 24 * time.Hour,  // 7 days default
    MaxBytes:    1 << 30,        // 1GB max
    MaxMsgs:     1_000_000,      // 1M messages max
    Discard:     DiscardOld,     // Remove oldest when full
    Replicas:    1,              // Single node initially

    // De-duplication window (prevent duplicate sends)
    Duplicates:  5 * time.Minute,
}
```

#### 2. TOPICS Stream
Stores all pub/sub topic messages.

```go
StreamConfig{
    Name:        "TOPICS",
    Description: "Pub/sub topic messages",
    Subjects:    []string{"subtrate.topic.>"},
    Storage:     FileStorage,
    Retention:   LimitsPolicy,
    MaxAge:      7 * 24 * time.Hour,
    MaxBytes:    1 << 30,
    MaxMsgs:     1_000_000,
    Discard:     DiscardOld,
    Replicas:    1,
}
```

#### 3. ACTIVITIES Stream (optional)
Stores system activity events for the dashboard.

```go
StreamConfig{
    Name:        "ACTIVITIES",
    Description: "Agent activity events",
    Subjects:    []string{"subtrate.sys.activity.>"},
    Storage:     FileStorage,
    Retention:   LimitsPolicy,
    MaxAge:      24 * time.Hour,  // Keep 24h of activity
    MaxMsgs:     100_000,
    Discard:     DiscardOld,
    Replicas:    1,
}
```

### Consumer Definitions

#### Per-Agent Inbox Consumer
Each agent gets a durable consumer for their inbox.

```go
ConsumerConfig{
    Name:          "inbox-{agent_name}",
    Durable:       "inbox-{agent_name}",
    Description:   "Inbox consumer for {agent_name}",
    FilterSubject: "subtrate.inbox.{agent_name}",

    DeliverPolicy: DeliverAll,      // Start from beginning
    AckPolicy:     AckExplicitPolicy,
    AckWait:       30 * time.Second,
    MaxDeliver:    5,               // Retry 5 times

    // For push delivery to active subscribers
    DeliverSubject: "_INBOX.inbox.{agent_name}",
}
```

#### Per-Agent Topic Consumers
Each agent gets consumers for their subscribed topics.

```go
ConsumerConfig{
    Name:          "topic-{topic_name}-{agent_name}",
    Durable:       "topic-{topic_name}-{agent_name}",
    FilterSubject: "subtrate.topic.{topic_name}",

    DeliverPolicy: DeliverNew,      // Only new messages
    AckPolicy:     AckExplicitPolicy,
    AckWait:       30 * time.Second,
}
```

### Message Format

Messages published to JetStream include metadata headers:

```go
type NATSMessage struct {
    // NATS headers
    Headers nats.Header{
        "Nats-Msg-Id":      []string{uuid},           // De-dup ID
        "Subtrate-Sender":  []string{sender_name},
        "Subtrate-Thread":  []string{thread_id},
        "Subtrate-Priority": []string{"urgent|normal|low"},
        "Subtrate-Deadline": []string{unix_timestamp}, // Optional
    }

    // Payload (JSON)
    Data []byte // Serialized MailMessage
}

type MailMessage struct {
    ID         string    `json:"id"`          // UUID
    ThreadID   string    `json:"thread_id"`
    SenderID   int64     `json:"sender_id"`
    SenderName string    `json:"sender_name"`
    Subject    string    `json:"subject"`
    Body       string    `json:"body"`
    Priority   string    `json:"priority"`
    Deadline   *int64    `json:"deadline,omitempty"`   // Unix timestamp
    Attachments string   `json:"attachments,omitempty"` // JSON
    CreatedAt  int64     `json:"created_at"`
}
```

### Sequence Numbers and Offsets

JetStream provides two sequence numbers:

1. **Stream Sequence**: Global position in stream (replaces `log_offset`)
2. **Consumer Sequence**: Position for this consumer

Mapping to current system:
- `log_offset` → JetStream stream sequence
- `consumer_offsets.last_offset` → JetStream consumer sequence (auto-tracked)

---

## SQLite Integration Changes

### Tables to Keep

| Table | Reason |
|-------|--------|
| `agents` | Agent identity, project keys, sessions |
| `session_identities` | Claude Code session mapping |
| `message_recipients` | Per-recipient state (read/starred/archived) |
| `activities` | Activity feed (unless moved to JetStream) |
| `messages_fts` | Full-text search index |

### Tables to Remove

| Table | Replacement |
|-------|-------------|
| `messages` | JetStream INBOXES + TOPICS streams |
| `topics` | JetStream stream subjects |
| `subscriptions` | JetStream consumers |
| `consumer_offsets` | JetStream consumer state |

### New Tables

```sql
-- Maps JetStream message IDs to internal tracking
CREATE TABLE js_message_index (
    js_stream TEXT NOT NULL,           -- "INBOXES" or "TOPICS"
    js_sequence INTEGER NOT NULL,      -- JetStream stream sequence
    msg_id TEXT NOT NULL,              -- Our UUID
    thread_id TEXT NOT NULL,
    sender_id INTEGER NOT NULL,
    subject TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY(js_stream, js_sequence)
);

CREATE INDEX idx_js_message_thread ON js_message_index(thread_id);
CREATE INDEX idx_js_message_sender ON js_message_index(sender_id);

-- FTS trigger for JetStream-indexed messages
CREATE TRIGGER js_message_ai AFTER INSERT ON js_message_index BEGIN
    INSERT INTO messages_fts(rowid, subject, body_md)
    VALUES (new.rowid, new.subject, ''); -- Body fetched from JetStream
END;
```

### Modified message_recipients Table

```sql
-- Links to JetStream instead of local messages table
CREATE TABLE message_recipients (
    js_stream TEXT NOT NULL,
    js_sequence INTEGER NOT NULL,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    state TEXT NOT NULL DEFAULT 'unread',
    snoozed_until INTEGER,
    read_at INTEGER,
    acked_at INTEGER,
    PRIMARY KEY(js_stream, js_sequence, agent_id),
    FOREIGN KEY(js_stream, js_sequence)
        REFERENCES js_message_index(js_stream, js_sequence)
);
```

### Migration Script

```sql
-- Migration: 000005_jetstream_integration.up.sql

-- 1. Create new index table
CREATE TABLE js_message_index (...);

-- 2. Modify message_recipients to use composite key
-- (Requires data migration - see migration strategy)

-- 3. Drop old tables after data migrated to JetStream
-- DROP TABLE messages;  -- Only after verified migration
-- DROP TABLE topics;
-- DROP TABLE subscriptions;
-- DROP TABLE consumer_offsets;
```

---

## Embedded NATS Server

### Design Decision: Embedded vs External

| Approach | Pros | Cons |
|----------|------|------|
| Embedded | Single binary, zero config, no dependencies | Larger binary, memory overhead |
| External | Separate scaling, existing clusters | Deployment complexity, another service |

**Decision**: Embedded NATS server for v0.3.0, with optional external connection.

### Implementation

```go
// internal/nats/embedded.go

package natsserver

import (
    "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
    "github.com/nats-io/nats.go/jetstream"
)

// EmbeddedServer wraps an in-process NATS server with JetStream.
type EmbeddedServer struct {
    server   *server.Server
    conn     *nats.Conn
    js       jetstream.JetStream
    dataDir  string

    // Configuration
    opts     *server.Options
}

// Config holds embedded NATS server configuration.
type Config struct {
    // Data directory for JetStream storage
    DataDir string

    // Port for client connections (0 for in-process only)
    Port int

    // JetStream storage limits
    MaxMemory int64  // Max memory for streams
    MaxStore  int64  // Max disk for streams

    // Optional: allow external connections
    AllowExternal bool

    // TLS configuration (for external connections)
    TLSConfig *tls.Config
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig(dataDir string) *Config {
    return &Config{
        DataDir:       dataDir,
        Port:          0,  // In-process only by default
        MaxMemory:     256 << 20,  // 256MB
        MaxStore:      1 << 30,    // 1GB
        AllowExternal: false,
    }
}

// New creates and starts an embedded NATS server.
func New(cfg *Config) (*EmbeddedServer, error) {
    opts := &server.Options{
        ServerName:     "subtrate-embedded",
        DontListen:     !cfg.AllowExternal,
        Port:           cfg.Port,
        JetStream:      true,
        StoreDir:       cfg.DataDir,
        JetStreamMaxMemory: cfg.MaxMemory,
        JetStreamMaxStore:  cfg.MaxStore,
    }

    if cfg.TLSConfig != nil {
        opts.TLSConfig = cfg.TLSConfig
    }

    // Start server
    ns, err := server.NewServer(opts)
    if err != nil {
        return nil, fmt.Errorf("failed to create NATS server: %w", err)
    }

    go ns.Start()

    if !ns.ReadyForConnections(10 * time.Second) {
        return nil, fmt.Errorf("NATS server failed to start")
    }

    // Connect in-process
    conn, err := nats.Connect(ns.ClientURL(),
        nats.InProcessServer(ns),
    )
    if err != nil {
        ns.Shutdown()
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    // Get JetStream context
    js, err := jetstream.New(conn)
    if err != nil {
        conn.Close()
        ns.Shutdown()
        return nil, fmt.Errorf("failed to get JetStream: %w", err)
    }

    return &EmbeddedServer{
        server:  ns,
        conn:    conn,
        js:      js,
        dataDir: cfg.DataDir,
        opts:    opts,
    }, nil
}

// InitializeStreams creates the required JetStream streams.
func (s *EmbeddedServer) InitializeStreams(ctx context.Context) error {
    streams := []jetstream.StreamConfig{
        {
            Name:        "INBOXES",
            Description: "Agent inbox messages",
            Subjects:    []string{"subtrate.inbox.>"},
            Storage:     jetstream.FileStorage,
            Retention:   jetstream.LimitsPolicy,
            MaxAge:      7 * 24 * time.Hour,
            MaxBytes:    1 << 30,
            Discard:     jetstream.DiscardOld,
            Replicas:    1,
            Duplicates:  5 * time.Minute,
        },
        {
            Name:        "TOPICS",
            Description: "Pub/sub topic messages",
            Subjects:    []string{"subtrate.topic.>"},
            Storage:     jetstream.FileStorage,
            Retention:   jetstream.LimitsPolicy,
            MaxAge:      7 * 24 * time.Hour,
            MaxBytes:    1 << 30,
            Discard:     jetstream.DiscardOld,
            Replicas:    1,
        },
    }

    for _, cfg := range streams {
        _, err := s.js.CreateOrUpdateStream(ctx, cfg)
        if err != nil {
            return fmt.Errorf("failed to create stream %s: %w",
                cfg.Name, err)
        }
    }

    return nil
}

// Conn returns the NATS connection for clients.
func (s *EmbeddedServer) Conn() *nats.Conn {
    return s.conn
}

// JetStream returns the JetStream context.
func (s *EmbeddedServer) JetStream() jetstream.JetStream {
    return s.js
}

// ClientURL returns the URL for external clients (if enabled).
func (s *EmbeddedServer) ClientURL() string {
    return s.server.ClientURL()
}

// Close shuts down the embedded server.
func (s *EmbeddedServer) Close() error {
    s.conn.Close()
    s.server.Shutdown()
    s.server.WaitForShutdown()
    return nil
}
```

### Data Directory Layout

```
~/.subtrate/
├── subtrate.db              # SQLite database
├── nats/                    # NATS JetStream data
│   ├── jetstream/
│   │   └── SUBTRATE/        # Account name
│   │       ├── streams/
│   │       │   ├── INBOXES/
│   │       │   │   ├── meta.inf
│   │       │   │   └── msgs/
│   │       │   └── TOPICS/
│   │       │       ├── meta.inf
│   │       │       └── msgs/
│   │       └── consumers/
│   └── nats.pid
└── logs/
    └── substrated.log
```

---

## RPC Protocol Design

### Request-Reply Pattern

NATS request-reply uses automatic inbox subjects:

```
Client                    NATS Server                    Service
   │                          │                             │
   │── Request ──────────────▶│── subtrate.rpc.mail.send ──▶│
   │   Reply-To: _INBOX.xxx   │                             │
   │                          │                             │
   │◀───────────────── Reply ─│◀──────────── Response ──────│
   │   On: _INBOX.xxx         │                             │
```

### RPC Message Format

```go
// Request envelope
type RPCRequest struct {
    Method  string          `json:"method"`   // e.g., "send", "fetch"
    TraceID string          `json:"trace_id"` // For distributed tracing
    Payload json.RawMessage `json:"payload"`  // Method-specific data
}

// Response envelope
type RPCResponse struct {
    Success bool            `json:"success"`
    TraceID string          `json:"trace_id"`
    Error   *RPCError       `json:"error,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}

type RPCError struct {
    Code    string `json:"code"`    // e.g., "NOT_FOUND", "INVALID_ARG"
    Message string `json:"message"`
}
```

### Service Implementation

```go
// internal/nats/rpc/service.go

package rpc

import (
    "github.com/nats-io/nats.go"
)

// Service handles NATS RPC requests.
type Service struct {
    conn    *nats.Conn
    mailSvc *mail.Service
    agentMgr *agent.Registry

    subs    []*nats.Subscription
}

// Start begins listening for RPC requests.
func (s *Service) Start(ctx context.Context) error {
    handlers := map[string]nats.MsgHandler{
        // Mail service RPCs
        "subtrate.rpc.mail.send":         s.handleSendMail,
        "subtrate.rpc.mail.fetch":        s.handleFetchInbox,
        "subtrate.rpc.mail.read":         s.handleReadMessage,
        "subtrate.rpc.mail.update-state": s.handleUpdateState,
        "subtrate.rpc.mail.ack":          s.handleAckMessage,
        "subtrate.rpc.mail.status":       s.handleGetStatus,
        "subtrate.rpc.mail.poll":         s.handlePollChanges,
        "subtrate.rpc.mail.publish":      s.handlePublish,
        "subtrate.rpc.mail.subscribe":    s.handleSubscribe,
        "subtrate.rpc.mail.unsubscribe":  s.handleUnsubscribe,
        "subtrate.rpc.mail.list-topics":  s.handleListTopics,
        "subtrate.rpc.mail.search":       s.handleSearch,

        // Agent service RPCs
        "subtrate.rpc.agent.register":        s.handleRegisterAgent,
        "subtrate.rpc.agent.get":             s.handleGetAgent,
        "subtrate.rpc.agent.list":            s.handleListAgents,
        "subtrate.rpc.agent.ensure-identity": s.handleEnsureIdentity,
        "subtrate.rpc.agent.save-identity":   s.handleSaveIdentity,
    }

    for subject, handler := range handlers {
        sub, err := s.conn.Subscribe(subject, handler)
        if err != nil {
            return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
        }
        s.subs = append(s.subs, sub)
    }

    return nil
}

// Example handler
func (s *Service) handleSendMail(msg *nats.Msg) {
    var req RPCRequest
    if err := json.Unmarshal(msg.Data, &req); err != nil {
        s.replyError(msg, "INVALID_REQUEST", err.Error())
        return
    }

    var sendReq SendMailRequest
    if err := json.Unmarshal(req.Payload, &sendReq); err != nil {
        s.replyError(msg, "INVALID_PAYLOAD", err.Error())
        return
    }

    // Convert to internal request and process
    resp, err := s.mailSvc.Send(context.Background(), mail.SendMailRequest{
        SenderID:       sendReq.SenderID,
        RecipientNames: sendReq.RecipientNames,
        Subject:        sendReq.Subject,
        Body:           sendReq.Body,
        Priority:       mail.Priority(sendReq.Priority),
    })

    if err != nil {
        s.replyError(msg, "INTERNAL_ERROR", err.Error())
        return
    }

    s.replySuccess(msg, req.TraceID, SendMailResponse{
        MessageID: resp.MessageID,
        ThreadID:  resp.ThreadID,
    })
}

func (s *Service) replySuccess(msg *nats.Msg, traceID string, payload any) {
    data, _ := json.Marshal(payload)
    resp := RPCResponse{
        Success: true,
        TraceID: traceID,
        Payload: data,
    }
    respData, _ := json.Marshal(resp)
    msg.Respond(respData)
}

func (s *Service) replyError(msg *nats.Msg, code, message string) {
    resp := RPCResponse{
        Success: false,
        Error: &RPCError{
            Code:    code,
            Message: message,
        },
    }
    respData, _ := json.Marshal(resp)
    msg.Respond(respData)
}
```

### Streaming Subscriptions

For real-time inbox updates, use JetStream push consumers:

```go
// Subscribe to inbox stream for real-time delivery
func (s *Service) SubscribeInbox(ctx context.Context, agentName string) (
    <-chan *MailMessage, func(), error) {

    consumerName := fmt.Sprintf("inbox-%s", agentName)
    filterSubject := fmt.Sprintf("subtrate.inbox.%s", agentName)

    // Create or get durable consumer
    consumer, err := s.js.CreateOrUpdateConsumer(ctx, "INBOXES",
        jetstream.ConsumerConfig{
            Name:          consumerName,
            Durable:       consumerName,
            FilterSubject: filterSubject,
            DeliverPolicy: jetstream.DeliverNewPolicy,
            AckPolicy:     jetstream.AckExplicitPolicy,
        })
    if err != nil {
        return nil, nil, err
    }

    // Start consuming
    msgCh := make(chan *MailMessage, 100)

    cons, err := consumer.Consume(func(msg jetstream.Msg) {
        var mailMsg MailMessage
        if err := json.Unmarshal(msg.Data(), &mailMsg); err != nil {
            msg.Nak()  // Negative ack - will be redelivered
            return
        }

        select {
        case msgCh <- &mailMsg:
            msg.Ack()
        default:
            msg.Nak()  // Channel full, retry later
        }
    })
    if err != nil {
        return nil, nil, err
    }

    cancel := func() {
        cons.Stop()
        close(msgCh)
    }

    return msgCh, cancel, nil
}
```

---

## CLI Changes

### Client Architecture

```go
// cmd/substrate/commands/nats_client.go

package commands

import (
    "github.com/nats-io/nats.go"
)

// NATSClient handles NATS-based communication with the daemon.
type NATSClient struct {
    conn     *nats.Conn
    timeout  time.Duration
}

// NewNATSClient connects to the substrate daemon via NATS.
func NewNATSClient(url string) (*NATSClient, error) {
    // Try to connect with timeout
    conn, err := nats.Connect(url,
        nats.Timeout(2*time.Second),
        nats.RetryOnFailedConnect(true),
        nats.MaxReconnects(3),
        nats.ReconnectWait(500*time.Millisecond),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    return &NATSClient{
        conn:    conn,
        timeout: 30 * time.Second,
    }, nil
}

// SendMail sends a mail message via RPC.
func (c *NATSClient) SendMail(ctx context.Context, req SendMailRequest) (
    *SendMailResponse, error) {

    payload, _ := json.Marshal(req)
    rpcReq := RPCRequest{
        Method:  "send",
        TraceID: uuid.New().String(),
        Payload: payload,
    }

    reqData, _ := json.Marshal(rpcReq)

    msg, err := c.conn.RequestWithContext(ctx,
        "subtrate.rpc.mail.send", reqData)
    if err != nil {
        return nil, fmt.Errorf("RPC failed: %w", err)
    }

    var resp RPCResponse
    if err := json.Unmarshal(msg.Data, &resp); err != nil {
        return nil, fmt.Errorf("invalid response: %w", err)
    }

    if !resp.Success {
        return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
    }

    var sendResp SendMailResponse
    if err := json.Unmarshal(resp.Payload, &sendResp); err != nil {
        return nil, fmt.Errorf("invalid payload: %w", err)
    }

    return &sendResp, nil
}
```

### Updated Client Wrapper

```go
// cmd/substrate/commands/client.go - Updated for dual NATS/direct mode

type Client struct {
    // NATS mode (preferred)
    natsClient *NATSClient

    // Direct mode (fallback)
    store       *db.Store
    mailService *mail.Service
    registry    *agent.Registry
    identityMgr *agent.IdentityManager

    // Mode flags
    useNATS bool
    cleanup func()
}

// NewClient creates a client, preferring NATS connection.
func NewClient(dataDir string) (*Client, error) {
    // Try NATS first
    natsURL := os.Getenv("SUBTRATE_NATS_URL")
    if natsURL == "" {
        natsURL = "nats://localhost:4222"
    }

    natsClient, err := NewNATSClient(natsURL)
    if err == nil {
        return &Client{
            natsClient: natsClient,
            useNATS:    true,
            cleanup:    func() { natsClient.conn.Close() },
        }, nil
    }

    // Fall back to direct mode
    dbPath := filepath.Join(dataDir, "subtrate.db")
    store, err := db.Open(dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    mailService := mail.NewService(store)
    registry := agent.NewRegistry(store)
    identityMgr := agent.NewIdentityManager(store)

    return &Client{
        store:       store,
        mailService: mailService,
        registry:    registry,
        identityMgr: identityMgr,
        useNATS:     false,
        cleanup:     func() { store.Close() },
    }, nil
}
```

### New CLI Flags

```go
// Global flags for NATS configuration
var (
    natsURL     string
    natsTimeout time.Duration
)

func init() {
    rootCmd.PersistentFlags().StringVar(&natsURL, "nats-url",
        "nats://localhost:4222", "NATS server URL")
    rootCmd.PersistentFlags().DurationVar(&natsTimeout, "nats-timeout",
        30*time.Second, "NATS request timeout")
}
```

---

## Server Changes

### Updated substrated Main

```go
// cmd/substrated/main.go

func main() {
    cfg := loadConfig()

    // Initialize data directory
    dataDir := cfg.DataDir
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        log.Fatalf("Failed to create data dir: %v", err)
    }

    // Start embedded NATS server
    natsCfg := natsserver.DefaultConfig(filepath.Join(dataDir, "nats"))
    natsCfg.Port = cfg.NATSPort
    natsCfg.AllowExternal = cfg.AllowExternalNATS

    natsServer, err := natsserver.New(natsCfg)
    if err != nil {
        log.Fatalf("Failed to start NATS server: %v", err)
    }
    defer natsServer.Close()

    // Initialize JetStream streams
    ctx := context.Background()
    if err := natsServer.InitializeStreams(ctx); err != nil {
        log.Fatalf("Failed to initialize streams: %v", err)
    }

    // Open SQLite database
    dbPath := filepath.Join(dataDir, "subtrate.db")
    store, err := db.Open(dbPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer store.Close()

    // Run migrations
    if err := db.RunMigrations(store.DB(), migrationsPath); err != nil {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    // Create services
    mailSvc := mail.NewServiceWithJetStream(store, natsServer.JetStream())
    agentReg := agent.NewRegistry(store)
    identityMgr := agent.NewIdentityManager(store)

    // Start RPC service
    rpcSvc := rpc.NewService(natsServer.Conn(), mailSvc, agentReg, identityMgr)
    if err := rpcSvc.Start(ctx); err != nil {
        log.Fatalf("Failed to start RPC service: %v", err)
    }

    // Start web UI (if enabled)
    if cfg.WebEnabled {
        webHandler := web.NewHandler(store, mailSvc, agentReg)
        go func() {
            addr := fmt.Sprintf(":%d", cfg.WebPort)
            log.Printf("Web UI: http://localhost%s", addr)
            http.ListenAndServe(addr, webHandler)
        }()
    }

    log.Printf("substrated started (NATS: %s)", natsServer.ClientURL())

    // Wait for shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    log.Println("Shutting down...")
}
```

### Mail Service with JetStream

```go
// internal/mail/service_jetstream.go

package mail

import (
    "github.com/nats-io/nats.go/jetstream"
)

// ServiceJS extends Service with JetStream integration.
type ServiceJS struct {
    *Service
    js jetstream.JetStream
}

// NewServiceWithJetStream creates a mail service with JetStream backend.
func NewServiceWithJetStream(store *db.Store, js jetstream.JetStream) *ServiceJS {
    return &ServiceJS{
        Service: NewService(store),
        js:      js,
    }
}

// Send publishes a message to JetStream and creates recipient entries.
func (s *ServiceJS) Send(ctx context.Context, req SendMailRequest) (
    SendMailResponse, error) {

    var resp SendMailResponse

    // Generate IDs
    msgID := uuid.New().String()
    threadID := req.ThreadID
    if threadID == "" {
        threadID = uuid.New().String()
    }

    // Build NATS message
    msg := MailMessage{
        ID:         msgID,
        ThreadID:   threadID,
        SenderID:   req.SenderID,
        SenderName: "", // Resolve from registry
        Subject:    req.Subject,
        Body:       req.Body,
        Priority:   string(req.Priority),
        CreatedAt:  time.Now().Unix(),
    }

    if req.Deadline != nil {
        deadline := req.Deadline.Unix()
        msg.Deadline = &deadline
    }

    data, err := json.Marshal(msg)
    if err != nil {
        return resp, fmt.Errorf("failed to marshal message: %w", err)
    }

    // Publish to each recipient's inbox
    for _, recipientName := range req.RecipientNames {
        subject := fmt.Sprintf("subtrate.inbox.%s", recipientName)

        natsMsg := nats.NewMsg(subject)
        natsMsg.Data = data
        natsMsg.Header.Set("Nats-Msg-Id", msgID)
        natsMsg.Header.Set("Subtrate-Thread", threadID)
        natsMsg.Header.Set("Subtrate-Priority", string(req.Priority))

        ack, err := s.js.PublishMsg(ctx, natsMsg)
        if err != nil {
            return resp, fmt.Errorf("failed to publish to %s: %w",
                recipientName, err)
        }

        // Index in SQLite for search and state tracking
        err = s.indexMessage(ctx, ack.Stream, ack.Sequence, msg, recipientName)
        if err != nil {
            return resp, fmt.Errorf("failed to index message: %w", err)
        }
    }

    resp.MessageID = 0 // JetStream doesn't use int64 IDs
    resp.ThreadID = threadID

    return resp, nil
}

// indexMessage creates SQLite entries for the published message.
func (s *ServiceJS) indexMessage(ctx context.Context, stream string,
    seq uint64, msg MailMessage, recipientName string) error {

    return s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
        // Get recipient agent ID
        agent, err := q.GetAgentByName(ctx, recipientName)
        if err != nil {
            return fmt.Errorf("recipient %q not found: %w", recipientName, err)
        }

        // Insert into index
        err = q.CreateJSMessageIndex(ctx, sqlc.CreateJSMessageIndexParams{
            JsStream:   stream,
            JsSequence: int64(seq),
            MsgID:      msg.ID,
            ThreadID:   msg.ThreadID,
            SenderID:   msg.SenderID,
            Subject:    msg.Subject,
            CreatedAt:  msg.CreatedAt,
        })
        if err != nil {
            return fmt.Errorf("failed to index: %w", err)
        }

        // Create recipient entry with unread state
        err = q.CreateJSMessageRecipient(ctx, sqlc.CreateJSMessageRecipientParams{
            JsStream:   stream,
            JsSequence: int64(seq),
            AgentID:    agent.ID,
        })
        if err != nil {
            return fmt.Errorf("failed to create recipient: %w", err)
        }

        return nil
    })
}
```

---

## Migration Strategy

### Phase 1: Parallel Operation (v0.3.0)

Run both gRPC and NATS RPC simultaneously:

1. Add embedded NATS server to substrated
2. Implement NATS RPC handlers alongside gRPC
3. New messages go to JetStream AND SQLite
4. CLI prefers NATS, falls back to gRPC, then direct

### Phase 2: JetStream Migration (v0.3.1)

Migrate message storage to JetStream:

1. Create `js_message_index` table
2. Migrate existing messages to JetStream streams
3. Update `message_recipients` to use composite keys
4. Keep SQLite messages table as read-only backup

```go
// Migration script
func MigrateToJetStream(ctx context.Context, store *db.Store,
    js jetstream.JetStream) error {

    // Get all messages from SQLite
    messages, err := store.Queries().GetAllMessages(ctx)
    if err != nil {
        return err
    }

    for _, msg := range messages {
        // Determine target stream based on topic type
        topic, _ := store.Queries().GetTopic(ctx, msg.TopicID)
        var stream, subject string

        if topic.TopicType == "direct" {
            stream = "INBOXES"
            subject = fmt.Sprintf("subtrate.inbox.%s",
                getRecipientName(msg))
        } else {
            stream = "TOPICS"
            subject = fmt.Sprintf("subtrate.topic.%s", topic.Name)
        }

        // Publish to JetStream
        ack, err := js.Publish(ctx, subject, marshalMessage(msg))
        if err != nil {
            log.Printf("Failed to migrate message %d: %v", msg.ID, err)
            continue
        }

        // Update index mapping
        store.Queries().CreateJSMessageMapping(ctx,
            sqlc.CreateJSMessageMappingParams{
                OldMessageID: msg.ID,
                JsStream:     stream,
                JsSequence:   int64(ack.Sequence),
            })
    }

    return nil
}
```

### Phase 3: gRPC Deprecation (v0.4.0)

1. Remove gRPC server and client code
2. Remove old SQLite messages/topics/subscriptions tables
3. CLI requires NATS connection (no more direct mode)
4. Update documentation

### Rollback Plan

If issues arise during migration:

1. Keep SQLite messages table intact
2. gRPC server remains functional
3. Feature flag to disable NATS: `--disable-nats`
4. Restore from SQLite backup if JetStream data corrupted

---

## Integration Test Changes

### New Test Infrastructure

```go
// tests/integration/nats_test.go

package integration_test

import (
    "testing"

    natsserver "github.com/nats-io/nats-server/v2/test"
    "github.com/nats-io/nats.go"
    "github.com/nats-io/nats.go/jetstream"
)

// testNATSEnv holds the test environment with NATS.
type testNATSEnv struct {
    t *testing.T

    // NATS components
    natsServer *natsserver.Server
    conn       *nats.Conn
    js         jetstream.JetStream

    // SQLite components
    store *db.Store

    // Services
    mailSvc *mail.ServiceJS
    rpcSvc  *rpc.Service
}

// newTestNATSEnv creates a test environment with embedded NATS.
func newTestNATSEnv(t *testing.T) *testNATSEnv {
    t.Helper()

    // Start test NATS server with JetStream
    opts := natsserver.DefaultTestOptions
    opts.JetStream = true
    opts.StoreDir = t.TempDir()

    ns := natsserver.RunServer(&opts)

    // Connect
    conn, err := nats.Connect(ns.ClientURL())
    require.NoError(t, err)

    js, err := jetstream.New(conn)
    require.NoError(t, err)

    // Create streams
    ctx := context.Background()
    _, err = js.CreateStream(ctx, jetstream.StreamConfig{
        Name:     "INBOXES",
        Subjects: []string{"subtrate.inbox.>"},
    })
    require.NoError(t, err)

    _, err = js.CreateStream(ctx, jetstream.StreamConfig{
        Name:     "TOPICS",
        Subjects: []string{"subtrate.topic.>"},
    })
    require.NoError(t, err)

    // Create SQLite store
    store, cleanup := testDB(t)
    t.Cleanup(cleanup)

    // Create services
    mailSvc := mail.NewServiceWithJetStream(store, js)

    return &testNATSEnv{
        t:          t,
        natsServer: ns,
        conn:       conn,
        js:         js,
        store:      store,
        mailSvc:    mailSvc,
    }
}

func (e *testNATSEnv) cleanup() {
    e.conn.Close()
    e.natsServer.Shutdown()
}
```

### Test Cases to Add

```go
// TestNATS_DirectMail tests direct messaging via NATS.
func TestNATS_DirectMail(t *testing.T) {
    env := newTestNATSEnv(t)
    defer env.cleanup()

    ctx := context.Background()

    // Create agents
    alice := env.createAgent("alice")
    bob := env.createAgent("bob")

    // Alice sends message to Bob via NATS
    resp, err := env.mailSvc.Send(ctx, mail.SendMailRequest{
        SenderID:       alice.ID,
        RecipientNames: []string{"bob"},
        Subject:        "Hello via NATS",
        Body:           "Testing JetStream messaging",
        Priority:       mail.PriorityNormal,
    })
    require.NoError(t, err)
    require.NotEmpty(t, resp.ThreadID)

    // Verify message in JetStream
    stream, err := env.js.Stream(ctx, "INBOXES")
    require.NoError(t, err)

    info, err := stream.Info(ctx)
    require.NoError(t, err)
    require.Equal(t, uint64(1), info.State.Msgs)

    // Bob reads message
    consumer, err := env.js.CreateConsumer(ctx, "INBOXES",
        jetstream.ConsumerConfig{
            FilterSubject: "subtrate.inbox.bob",
        })
    require.NoError(t, err)

    msgs, err := consumer.Fetch(1)
    require.NoError(t, err)

    var receivedMsg mail.MailMessage
    for msg := range msgs.Messages() {
        err = json.Unmarshal(msg.Data(), &receivedMsg)
        require.NoError(t, err)
        msg.Ack()
    }

    require.Equal(t, "Hello via NATS", receivedMsg.Subject)
}

// TestNATS_ConsumerOffsets tests offset-based polling.
func TestNATS_ConsumerOffsets(t *testing.T) {
    env := newTestNATSEnv(t)
    defer env.cleanup()

    ctx := context.Background()

    sender := env.createAgent("sender")
    env.createAgent("receiver")

    // Create topic
    env.createTopic("updates", "broadcast")
    env.subscribe("receiver", "updates")

    // Publish 5 messages
    for i := 0; i < 5; i++ {
        _, err := env.mailSvc.Publish(ctx, mail.PublishRequest{
            SenderID:  sender.ID,
            TopicName: "updates",
            Subject:   fmt.Sprintf("Update %d", i),
            Body:      "Content",
        })
        require.NoError(t, err)
    }

    // Create durable consumer
    consumer, err := env.js.CreateConsumer(ctx, "TOPICS",
        jetstream.ConsumerConfig{
            Durable:       "receiver-updates",
            FilterSubject: "subtrate.topic.updates",
            DeliverPolicy: jetstream.DeliverAllPolicy,
        })
    require.NoError(t, err)

    // Consume first 3 messages
    msgs, err := consumer.Fetch(3)
    require.NoError(t, err)

    count := 0
    for msg := range msgs.Messages() {
        msg.Ack()
        count++
    }
    require.Equal(t, 3, count)

    // Consumer should have 2 pending
    info, err := consumer.Info(ctx)
    require.NoError(t, err)
    require.Equal(t, uint64(2), info.NumPending)
}

// TestNATS_RPC tests the request-reply RPC pattern.
func TestNATS_RPC(t *testing.T) {
    env := newTestNATSEnv(t)
    defer env.cleanup()

    ctx := context.Background()

    // Start RPC service
    rpcSvc := rpc.NewService(env.conn, env.mailSvc, env.registry, env.identityMgr)
    err := rpcSvc.Start(ctx)
    require.NoError(t, err)

    // Create RPC client
    client := commands.NewNATSClient(env.conn)

    // Register agent via RPC
    resp, err := client.RegisterAgent(ctx, commands.RegisterAgentRequest{
        Name: "rpc-test-agent",
    })
    require.NoError(t, err)
    require.Greater(t, resp.AgentID, int64(0))

    // Verify agent exists
    agent, err := client.GetAgent(ctx, commands.GetAgentRequest{
        Name: "rpc-test-agent",
    })
    require.NoError(t, err)
    require.Equal(t, resp.AgentID, agent.ID)
}

// TestNATS_RedeliveryOnFailure tests message redelivery.
func TestNATS_RedeliveryOnFailure(t *testing.T) {
    env := newTestNATSEnv(t)
    defer env.cleanup()

    ctx := context.Background()

    sender := env.createAgent("sender")
    env.createAgent("receiver")

    // Send message
    _, err := env.mailSvc.Send(ctx, mail.SendMailRequest{
        SenderID:       sender.ID,
        RecipientNames: []string{"receiver"},
        Subject:        "Redelivery test",
        Body:           "Should be redelivered on NAK",
    })
    require.NoError(t, err)

    // Create consumer with explicit ack
    consumer, err := env.js.CreateConsumer(ctx, "INBOXES",
        jetstream.ConsumerConfig{
            FilterSubject: "subtrate.inbox.receiver",
            AckPolicy:     jetstream.AckExplicitPolicy,
            AckWait:       1 * time.Second,
            MaxDeliver:    3,
        })
    require.NoError(t, err)

    // Fetch and NAK
    msgs, _ := consumer.Fetch(1)
    for msg := range msgs.Messages() {
        msg.Nak() // Negative acknowledge
    }

    // Wait for redelivery
    time.Sleep(2 * time.Second)

    // Should be redelivered
    msgs, _ = consumer.Fetch(1)
    redelivered := false
    for msg := range msgs.Messages() {
        meta, _ := msg.Metadata()
        if meta.NumDelivered > 1 {
            redelivered = true
        }
        msg.Ack()
    }

    require.True(t, redelivered, "message should have been redelivered")
}

// TestNATS_Deduplication tests message deduplication.
func TestNATS_Deduplication(t *testing.T) {
    env := newTestNATSEnv(t)
    defer env.cleanup()

    ctx := context.Background()

    // Publish same message ID twice
    msgID := uuid.New().String()

    for i := 0; i < 2; i++ {
        msg := nats.NewMsg("subtrate.inbox.test")
        msg.Data = []byte(`{"subject": "dup test"}`)
        msg.Header.Set("Nats-Msg-Id", msgID)

        _, err := env.js.PublishMsg(ctx, msg)
        require.NoError(t, err)
    }

    // Should only have 1 message (duplicate rejected)
    stream, _ := env.js.Stream(ctx, "INBOXES")
    info, _ := stream.Info(ctx)
    require.Equal(t, uint64(1), info.State.Msgs)
}
```

### Test Matrix

| Test Category | Core NATS | JetStream | SQLite | RPC |
|---------------|-----------|-----------|--------|-----|
| Direct messaging | | ✓ | ✓ | |
| Pub/sub broadcasting | | ✓ | ✓ | |
| Consumer offsets | | ✓ | | |
| Message state tracking | | | ✓ | |
| Request-reply RPC | ✓ | | | ✓ |
| Streaming subscriptions | | ✓ | | |
| Message deduplication | | ✓ | | |
| Redelivery on failure | | ✓ | | |
| Full-text search | | | ✓ | |
| Agent registration | | | ✓ | ✓ |
| Session identity | | | ✓ | ✓ |

---

## Security Considerations

### Authentication (Future v0.4.0)

NATS supports multiple authentication methods:

1. **Token-based**: Simple shared token
2. **NKey**: Ed25519 key pairs
3. **JWT**: Account-based with permissions

Recommended: NKey for agent authentication

```go
// Agent generates key pair on registration
kp, _ := nkeys.CreateUser()
pubKey, _ := kp.PublicKey()
seed, _ := kp.Seed()

// Server stores public key, agent stores seed
// Connection uses seed for authentication
conn, _ := nats.Connect(url, nats.Nkey(pubKey, func(nonce []byte) ([]byte, error) {
    return kp.Sign(nonce)
}))
```

### Authorization

JetStream permissions per subject:

```json
{
  "permissions": {
    "publish": {
      "allow": ["subtrate.inbox.alice", "subtrate.rpc.>"]
    },
    "subscribe": {
      "allow": ["subtrate.inbox.alice", "_INBOX.>"]
    }
  }
}
```

### Encryption

1. **TLS**: Encrypt connections (required for external access)
2. **Payload encryption**: Optional E2E for message bodies

```go
// Enable TLS for external connections
opts := &server.Options{
    TLSConfig: &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    },
}
```

---

## Appendix: Dependencies

### New Go Dependencies

```go
// go.mod additions
require (
    github.com/nats-io/nats-server/v2 v2.10.x
    github.com/nats-io/nats.go v1.31.x
    github.com/nats-io/nkeys v0.4.x
)
```

### Estimated Binary Size Impact

| Component | Size |
|-----------|------|
| Current substrated | ~15MB |
| nats-server embedded | +8MB |
| JetStream | +2MB |
| **Total** | ~25MB |

### Performance Characteristics

| Operation | gRPC (current) | NATS RPC | Notes |
|-----------|----------------|----------|-------|
| RPC latency | ~1ms | ~0.5ms | NATS lower overhead |
| Throughput | 50k req/s | 100k+ req/s | NATS optimized for speed |
| Memory (idle) | ~50MB | ~80MB | JetStream caches |
| Memory (load) | ~200MB | ~150MB | NATS more efficient |

---

## Summary

This specification outlines a comprehensive migration from gRPC to NATS-based RPC with JetStream for Subtrate. Key decisions:

1. **JetStream required** for durable message storage and consumer offsets
2. **Embedded NATS server** for zero-dependency deployment
3. **SQLite retained** for agent identity, per-recipient state, and FTS
4. **Phased migration** to minimize risk and allow rollback
5. **Dual-mode operation** during transition

The migration aligns naturally with Subtrate's existing log-based queue semantics and provides a foundation for future horizontal scaling via NATS clustering.
