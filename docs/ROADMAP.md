# Subtrate Roadmap

This document outlines future improvements and the current status of Subtrate.

## Current Status

### v0.1.0 - Core Foundation (Complete)
- [x] Core mail service with actor pattern
- [x] SQLite storage with FTS5 full-text search
- [x] CLI tool (`substrate`) with full command set
- [x] MCP server (`substrated`) for Claude Code integration
- [x] Thread state machine with ProtoFSM
- [x] Agent identity persistence across sessions
- [x] HTMX web frontend with Gmail-inspired design
- [x] SSE real-time updates (agents, activity, inbox)
- [x] Activity feed with database schema
- [x] Search, settings, topic view pages
- [x] Message actions (star, archive, snooze, delete)
- [x] Reply functionality with UI refresh

## Architecture Improvements

### Phase A: Functional Core / Imperative Shell Refactoring
Refactor web handlers to separate pure business logic from I/O:

- [ ] **A1**: Extract pure functions from handlers into `internal/web/logic/` package
- [ ] **A2**: Define interfaces for all external dependencies (db, mail service)
- [ ] **A3**: Create mock implementations for testing
- [ ] **A4**: Refactor handlers to be thin wrappers calling pure functions
- [ ] **A5**: Add comprehensive unit tests for logic package (target 90%+)

### Phase B: Database Actor Layer
Replace direct query access with actor-based request handling:

- [ ] **B1**: Design `DBWorker` actor with request/response message types
- [ ] **B2**: Implement worker pool manager for concurrent query handling
- [ ] **B3**: Create typed request wrappers for all query operations
- [ ] **B4**: Add circuit breaker / backpressure support
- [ ] **B5**: Migrate handlers to use actor-based DB access
- [ ] **B6**: Add metrics for query latency and pool utilization

### Phase C: Test Coverage Improvements
Target 85%+ meaningful coverage with property-based testing:

- [ ] **C1**: Add unit tests for database layer (target 85%+)
- [ ] **C2**: Integrate `rapid` for property-based testing
- [ ] **C3**: Add fuzz tests for message parsing and validation
- [ ] **C4**: Test error paths and edge cases comprehensively
- [ ] **C5**: Add concurrent scenario tests for actor system
- [ ] **C6**: Create integration test fixtures with realistic data

## Feature Roadmap

### v0.2.0 - Enhanced Agent Integration & Real-time
- [ ] **Stop hook long-polling**: Use Claude Code Stop hooks to implement mail checking
  - Agent checks inbox before stopping (configurable timeout)
  - If new mail: prompt agent to process before exiting
  - Doubles as heartbeat mechanism (agent alive while hook runs)
- [ ] **Heartbeat via hooks**: Automatic heartbeat on SessionStart, UserPromptSubmit, Stop
  - Track agent liveness status (active, idle, offline)
  - Heartbeat failures trigger agent status change
- [ ] React inbox components (for complex interactions)
- [ ] WebSocket support for bi-directional updates
- [ ] Improved thread view with message grouping
- [ ] Bulk message operations
- [ ] Keyboard shortcuts

### v0.3.0 - NATS Integration
- [ ] NATS JetStream for real-time messaging
- [ ] Subject-based routing for topics
- [ ] Durable consumer offsets via JetStream
- [ ] Consider replacing gRPC with NATS RPC

### v0.4.0 - Security & Scale
- [ ] Agent authentication (API keys / macaroons)
- [ ] Message encryption (optional E2E)
- [ ] Audit logging for all operations
- [ ] Rate limiting per agent
- [ ] Multi-node support (PostgreSQL backend option)

### v1.0.0 - Production Ready
- [ ] Comprehensive documentation
- [ ] Performance benchmarks
- [ ] Migration tools
- [ ] Desktop app (Tauri/Wails)

## Technical Notes

### Stop Hook Long-Polling Pattern
Use Claude Code's Stop hook to implement a "check mail before exit" flow:

```
Stop Hook Flow:
1. Agent receives stop signal (user ends session, timeout, etc.)
2. Stop hook triggers: `substrate poll --wait=30s`
3. If new mail arrives within 30s: return non-zero (block exit), inject mail context
4. Agent processes new mail, then Stop hook runs again
5. If no mail after timeout: return 0 (allow exit), agent stops cleanly

Benefits:
- Agents naturally stay alive while work is pending
- No separate heartbeat daemon needed (hook IS the heartbeat)
- User can always interrupt (Ctrl+C bypasses hooks)
- Graceful degradation when backend unavailable
```

### NATS RPC Benefits
- Built-in pub/sub aligns with Subtrate's messaging model
- Native request-reply pattern simpler than gRPC streaming
- JetStream provides durable message storage with consumer offsets
- Subject-based routing maps naturally to topic-based message routing

### Current Test Coverage
| Package | Coverage | Target |
|---------|----------|--------|
| `internal/agent` | 79.7% | 85% |
| `internal/db` | 73.4% | 85% |
| `internal/mail` | 68.7% | 85% |
| `internal/web` | ~40% | 85% |
| `internal/mcp` | 0% | 60% |

### Database Access Pattern Migration
```
Current:  Handler → store.Queries().Method()
Future:   Handler → DBWorkerPool.Ask(QueryRequest) → Response
```

## Slash Command

Use `/roadmap-tasks` to generate Tasks from this roadmap.
