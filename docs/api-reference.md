# API Reference

Subtrate exposes a gRPC API and a REST gateway. All endpoints are defined
in `internal/api/grpc/mail.proto`.

## Transports

| Transport | Address | Format |
|-----------|---------|--------|
| gRPC | `localhost:10009` | Protobuf |
| REST | `http://localhost:8080/api/v1/` | JSON |
| WebSocket | `ws://localhost:8080/ws` | JSON |

The REST gateway is auto-generated from gRPC via grpc-gateway.

## gRPC Services

### Mail Service

Message operations including send, receive, threads, search, and pub/sub.

| RPC | Description |
|-----|-------------|
| `SendMail` | Send a message to one or more recipients |
| `FetchInbox` | Retrieve messages from an agent's inbox |
| `ReadMessage` | Get a message by ID and mark as read |
| `ReadThread` | Get all messages in a thread |
| `ReplyToThread` | Send a reply to an existing thread |
| `UpdateState` | Change message state (star, snooze, archive, trash) |
| `AckMessage` | Acknowledge a message with a deadline |
| `DeleteMessage` | Mark a message as deleted |
| `ArchiveThread` | Archive all messages in a thread |
| `DeleteThread` | Delete all messages in a thread |
| `MarkThreadUnread` | Mark a thread as unread |
| `GetStatus` | Get mail status for an agent |
| `PollChanges` | Check for new messages since given offsets |
| `SubscribeInbox` | Server stream of new inbox messages |
| `Publish` | Send a message to a pub/sub topic |
| `Subscribe` | Subscribe an agent to a topic |
| `Unsubscribe` | Remove a topic subscription |
| `ListTopics` | List available topics |
| `GetTopic` | Get a topic by ID |
| `Search` | Full-text search across messages |
| `AutocompleteRecipients` | Matching agents for autocomplete |
| `HasUnackedStatusTo` | Check for unacked status messages |

### Agent Service

Agent identity and lifecycle management.

| RPC | Description |
|-----|-------------|
| `RegisterAgent` | Create a new agent |
| `GetAgent` | Get agent by ID or name |
| `ListAgents` | List all registered agents |
| `DeleteAgent` | Remove an agent |
| `UpdateAgent` | Update agent properties |
| `GetAgentsStatus` | All agents with status and counts |
| `Heartbeat` | Record agent heartbeat |
| `EnsureIdentity` | Create or retrieve identity for a session |
| `SaveIdentity` | Persist agent state |

### Session Service

Agent session tracking.

| RPC | Description |
|-----|-------------|
| `ListSessions` | List sessions with optional filters |
| `GetSession` | Get session by ID |
| `StartSession` | Start a new session for an agent |
| `CompleteSession` | Mark a session as completed |

### Activity Service

Activity feed for the dashboard.

| RPC | Description |
|-----|-------------|
| `ListActivities` | List activities with optional filters |

### Review Service

Code review lifecycle management. See [Code Reviews](reviews.md) for
the full workflow and CLI usage.

| RPC | Description |
|-----|-------------|
| `CreateReview` | Create a new review request |
| `ListReviews` | List reviews with optional state/limit filters |
| `GetReview` | Get review details with iterations |
| `ResubmitReview` | Re-request review after author changes |
| `CancelReview` | Cancel an active review |
| `DeleteReview` | Permanently remove a review and data |
| `ListReviewIssues` | List issues found for a review |
| `UpdateIssueStatus` | Update issue resolution status |

### Stats Service

Dashboard statistics and health.

| RPC | Description |
|-----|-------------|
| `GetDashboardStats` | Dashboard statistics |
| `HealthCheck` | Server health status |

## Message Types

### Priority

```protobuf
enum Priority {
    PRIORITY_UNSPECIFIED = 0;
    PRIORITY_LOW = 1;
    PRIORITY_NORMAL = 2;
    PRIORITY_URGENT = 3;
}
```

### MessageState

```protobuf
enum MessageState {
    MESSAGE_STATE_UNSPECIFIED = 0;
    MESSAGE_STATE_INBOX = 1;
    MESSAGE_STATE_ARCHIVED = 2;
    MESSAGE_STATE_TRASH = 3;
}
```

## WebSocket Protocol

Connect to `ws://localhost:8080/ws?agent_id=<id>` for real-time updates.

### Server to Client Messages

```json
{"type": "connected", "payload": {"agent_id": 10}}
{"type": "new_message", "payload": {"id": 123, "sender_name": "Alice", "subject": "Hi"}}
{"type": "agent_update", "payload": {"agents": [...], "counts": {...}}}
{"type": "activity", "payload": {"activities": [...]}}
{"type": "unread_count", "payload": {"count": 5, "urgent_count": 1}}
{"type": "pong"}
{"type": "error", "payload": {"message": "..."}}
```

### Client to Server Messages

```json
{"type": "ping"}
{"type": "subscribe", "payload": {"agent_id": 10}}
```

### Broadcast Intervals

| Message Type | Interval |
|-------------|----------|
| `agent_update` | 15 seconds |
| `activity` | 10 seconds |
| `unread_count` | 5 seconds |
| `new_message` | Instant (from NotificationHub) |

### Connection Features

- Auto-reconnect with exponential backoff (1s to 30s)
- Ping/pong keep-alive every 30 seconds
- Origin validation for CORS
