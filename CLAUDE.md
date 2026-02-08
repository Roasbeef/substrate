# Subtrate Agent Assistant Guide

> **IMPORTANT**: This file provides a quick reference for AI agents working on the Subtrate codebase.

## Project Overview

Subtrate is a central command center for managing Claude Code agents with mail/messaging, pub/sub, threaded conversations, and log-based queue semantics.

**Key Components:**
- **substrated** (`cmd/substrated`) - MCP daemon server with integrated web UI
- **substrate** (`cmd/substrate`) - Command-line interface for mail and review operations
- **Mail Service** (`internal/mail`) - Core messaging with actor pattern
- **Review Service** (`internal/review`) - Code review with Claude Agent SDK and FSM
- **Agent Registry** (`internal/agent`) - Agent identity and registration
- **Actor System** (`internal/baselib/actor`) - Actor framework (system, refs, futures, mailbox)
- **Web API** (`internal/web`) - JSON API and embedded React frontend
- **React Frontend** (`web/frontend`) - React + TypeScript SPA with Vite

## Essential Commands

### Building and Testing
- `make build` - Compile all packages
- `make build-all` - Build CLI and MCP binaries
- `make build-production` - Build daemon with embedded frontend (production)
- `make test` - Run all tests (includes FTS5 CGO flags)
- `make test-race` - Run tests with race detector
- `make test-cover` - Run tests with coverage summary
- `make test-all` - Run all tests (Go + frontend)
- `make lint` - Run the linter (must pass before committing)
- `make lint-all` - Run all linting (Go + frontend)
- `make fmt` - Format all Go source files
- `make ci` - Full CI pipeline (ci-go + ci-frontend)
- `make quick` - Quick compile check
- `make clean` - Remove build artifacts

### Frontend Commands
- `make bun-install` - Install frontend dependencies
- `make bun-build` - Build frontend for production
- `make bun-dev` - Start frontend dev server (port 5174)
- `make bun-test` - Run frontend unit/integration tests
- `make bun-test-e2e` - Run Playwright E2E tests
- `make bun-lint` - Lint frontend code

### Code Generation
- `make sqlc` - Regenerate type-safe database queries (after schema/query changes)
- `make sqlc-docker` - Regenerate via Docker (no local sqlc install needed)
- `make proto` - Generate gRPC code from protobuf definitions
- `make proto-check` - Verify proto tools are installed
- `make proto-install` - Install protoc and Go plugins
- `make gen` - Run all code generation (sqlc + proto)

### Testing Commands
The Makefile exports CGO flags automatically, so all test commands handle FTS5 correctly.

**Unit tests (single package):**
```bash
# Test entire package
make unit pkg=./internal/mail

# Test specific test case
make unit pkg=./internal/mail case=TestService_SendMail

# Test pattern matching
make unit pkg=./internal/mcp case=TestNewServer
```

**Other test commands:**
- `make test` - Run all tests
- `make test-race` - Run tests with race detector
- `make test-cover` - Run tests with coverage summary
- `make test-cover-html` - Generate HTML coverage report
- `make test-integration` - Run all integration tests
- `make test-integration-e2e` - Run e2e backend tests (no external deps)
- `make test-integration-sdk` - SDK integration tests (requires claude CLI)
- `make test-integration-short` - Short integration tests (no API calls)
- `make test-integration-seed` - Seed test database with fixtures
- `make run-test` - Run specific test: `make run-test test=TestFoo pkg=./internal/review`

### Pre-commit
- `make pre-commit` - Run all checks (tidy, fmt, vet, lint, test)

### Incremental Commits
When working on features, make incremental commits as you complete logical units:
1. After implementing a self-contained piece of functionality
2. After fixing a bug
3. Before moving to a different area of the codebase

Use the session log to track progress: `/session-log --progress "description"`

### Task Completion Integrity (CRITICAL)

**NEVER mark a task as complete prematurely.** A task is only complete when ALL acceptance criteria are met and the work is fully verified.

- Do NOT mark tasks complete just to bypass stop hooks or other blockers
- If a stop hook prevents stopping, complete the remaining work or ask the user
- Leave tasks as `in_progress` or `pending` if work remains
- When blocked, ask the user for guidance rather than marking done

### Documentation Updates

**Update docs when adding or modifying features.** After implementing new functionality:

1. **docs/HOOKS.md** - Update if hook behavior changes
2. **CLAUDE.md** - Update if new commands, patterns, or workflows are added
3. **~/.claude/CLAUDE.md** (global) - Update if Subtrate integration changes

Check if related documentation exists before finishing a feature. Use `ls docs/` to see available docs.

### Running the Server
The web UI is built into substrated. By default, `make run` starts in web-only mode (no MCP/stdio):

```bash
# Build and run in web-only mode (foreground, default)
make run

# Run with MCP support (for Claude Code integration, uses stdio)
make run-mcp

# Development mode (no rebuild)
make run-web-dev

# Background mode (web-only)
make start

# Stop background server
make stop

# Restart (stop + rebuild + start)
make restart

# Custom port
make run WEB_PORT=8081
make start WEB_PORT=8081
```

Access the UI at `http://localhost:8080` (or custom port).

### Data and Log File Locations
- **Database**: `~/.subtrate/subtrate.db` (SQLite with WAL mode)
- **Server logs**: `~/.subtrate/logs/substrated.log`
- **Agent identities**: `~/.subtrate/identities/`
- **Hook debug logs**: `~/.subtrate/stop_hook_debug.log`, `~/.subtrate/stop_hook_trace.log`

**When to use each:**
- `make run` - Default for development/testing the web UI
- `make run-mcp` - When testing Claude Code integration (reads from stdin)
- `make start/stop/restart` - For background execution during development

## Code Style Quick Reference

### Function and Method Comments
- **Every function and method** (including unexported ones) must have a comment
  starting with the function/method name
- Comments should explain **how/why**, not just what
- Use complete sentences ending with a period

### Code Organization
- 80-character line limit (best effort)
- Organize code into logical stanzas separated by blank lines
- When wrapping function calls, put closing paren on its own line with all args on new lines
- **Pack arguments densely** — when wrapping, fit multiple arguments per line up to the 80-char limit rather than putting each argument on its own line. This applies to function parameters, log key-value pairs, and struct literals alike

### Result Type API (CRITICAL)
This project uses `lnd/fn/v2` Result type. **Always use:**
```go
val, err := result.Unpack()
if err != nil {
    return err
}
```

**Never use** `GetErr()` or `GetOk()` - these don't exist.

### Actor System Patterns
The actor system is in `internal/baselib/actor/` (internalized). Both the
mail service and review service use it:
- `BaseMessage` embedding for message types
- Sealed interfaces via unexported marker methods (`messageMarker()`, etc.)
- `Receive()` returns `fn.Result` type
- `ActorSystem` for lifecycle, `ServiceKey` for typed lookup
- `AskAwait` in `internal/actorutil/` for synchronous ask-and-unpack

## Database Conventions

### SQLite with FTS5
- SQLite FTS5 requires CGO with specific flags
- **Makefile handles CGO_CFLAGS automatically** - just use `make test`, `make build`, etc.
- If running go commands directly: `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...`

### sqlc Workflow
1. Edit migration files in `internal/db/migrations/`
2. Edit query files in `internal/db/queries/`
3. Run `make sqlc` to regenerate
4. **Never edit** files in `internal/db/sqlc/` directly

### Adding Database Migrations
When adding a new migration:
1. Create `internal/db/migrations/NNNNNN_name.up.sql` and `.down.sql`
2. **CRITICAL**: Update `LatestMigrationVersion` in `internal/db/migrations.go`
3. The migration system auto-applies on server start and creates backups
4. Never manually ALTER tables via sqlite3 CLI - always use migration files

### Query Files Location
- `internal/db/queries/agents.sql` - Agent and session queries
- `internal/db/queries/topics.sql` - Topic and subscription queries
- `internal/db/queries/messages.sql` - Message and recipient queries
- `internal/db/queries/activities.sql` - Activity feed queries
- `internal/db/queries/reviews.sql` - Review, iteration, and issue queries

### Storage Interface Architecture

The database layer follows a clean architecture with three tiers:

```
┌─────────────────────────────────────────────────────────────┐
│                   store.Storage Interface                    │
│  (Domain types: store.Message, store.Agent, store.Topic)    │
├─────────────────────────────────────────────────────────────┤
│              SqlcStore / MockStore / txSqlcStore            │
│  (Converts between domain types and sqlc types)             │
├─────────────────────────────────────────────────────────────┤
│                   QueryStore Interface                       │
│  (sqlc types: sqlc.Message, sqlc.CreateMessageParams)       │
├─────────────────────────────────────────────────────────────┤
│              sqlc.Queries (generated code)                   │
│  (Raw SQL operations - DO NOT EDIT)                         │
└─────────────────────────────────────────────────────────────┘
```

**Key interfaces in `internal/store/interfaces.go`:**
- `MessageStore` - Message CRUD and inbox operations
- `AgentStore` - Agent management
- `TopicStore` - Topics and subscriptions
- `ActivityStore` - Activity feed
- `ReviewStore` - Review CRUD, iterations, issue tracking
- `SessionStore` - Session identity mapping
- `Storage` - Combines all above + `WithTx()` + `Close()`

**Domain types vs sqlc types:**
- Domain types (e.g., `store.Message`) use native Go types (`time.Time`, `string`)
- sqlc types use database types (`sql.NullInt64`, `sql.NullString`)
- Conversion functions: `MessageFromSqlc()`, `AgentFromSqlc()`, etc.
- Helper functions: `ToSqlcNullString()`, `ToSqlcNullInt64()`, `nullInt64ToTime()`

### Adding New Database Methods

Follow this process when adding a new database operation:

**Step 1: Add sqlc query**
```sql
-- In internal/db/queries/messages.sql

-- name: GetMessagesByPriority :many
SELECT * FROM messages WHERE priority = ? ORDER BY created_at DESC LIMIT ?;
```

**Step 2: Regenerate sqlc**
```bash
make sqlc
```
This creates types like `GetMessagesByPriorityParams` and `GetMessagesByPriorityRow`.

**Step 3: Add to QueryStore interface** (`internal/store/sqlc_store.go`)
```go
type QueryStore interface {
    // ... existing methods ...
    GetMessagesByPriority(
        ctx context.Context, arg sqlc.GetMessagesByPriorityParams,
    ) ([]sqlc.Message, error)
}
```

**Step 4: Add to domain interface** (`internal/store/interfaces.go`)
```go
type MessageStore interface {
    // ... existing methods ...

    // GetMessagesByPriority retrieves messages with a specific priority.
    GetMessagesByPriority(
        ctx context.Context, priority string, limit int,
    ) ([]Message, error)
}
```

**Step 5: Implement in SqlcStore** (`internal/store/sqlc_store.go`)
```go
func (s *SqlcStore) GetMessagesByPriority(ctx context.Context,
    priority string, limit int) ([]Message, error) {

    rows, err := s.db.GetMessagesByPriority(ctx, sqlc.GetMessagesByPriorityParams{
        Priority: priority,
        Limit:    int64(limit),
    })
    if err != nil {
        return nil, err
    }

    messages := make([]Message, len(rows))
    for i, row := range rows {
        messages[i] = MessageFromSqlc(row)
    }
    return messages, nil
}
```

**Step 6: Implement in txSqlcStore** (same file, for transaction support)
```go
func (s *txSqlcStore) GetMessagesByPriority(ctx context.Context,
    priority string, limit int) ([]Message, error) {

    rows, err := s.queries.GetMessagesByPriority(ctx, sqlc.GetMessagesByPriorityParams{
        Priority: priority,
        Limit:    int64(limit),
    })
    if err != nil {
        return nil, err
    }

    messages := make([]Message, len(rows))
    for i, row := range rows {
        messages[i] = MessageFromSqlc(row)
    }
    return messages, nil
}
```

**Step 7: Implement in MockStore** (`internal/store/mock_store.go`)
```go
func (m *MockStore) GetMessagesByPriority(
    ctx context.Context, priority string, limit int,
) ([]Message, error) {

    m.mu.RLock()
    defer m.mu.RUnlock()

    var results []Message
    for _, msg := range m.messages {
        if msg.Priority == priority {
            results = append(results, msg)
            if len(results) >= limit {
                break
            }
        }
    }
    return results, nil
}
```

### Transaction Pattern (WithTx/ExecTx)

The `WithTx` method runs operations within a transaction with automatic retry:

```go
// In handlers/services:
err := store.WithTx(ctx, func(ctx context.Context, txStore Storage) error {
    // All operations on txStore are within the same transaction
    msg, err := txStore.CreateMessage(ctx, params)
    if err != nil {
        return err // Transaction rolls back
    }

    err = txStore.CreateMessageRecipient(ctx, msg.ID, recipientID)
    if err != nil {
        return err // Transaction rolls back
    }

    return nil // Transaction commits
})
```

**Internal implementation uses `ExecTx`** for retry logic:
```go
func (s *SqlcStore) WithTx(ctx context.Context,
    fn func(ctx context.Context, store Storage) error) error {

    var writeTxOpts StorageTxOptions
    return s.db.ExecTx(ctx, &writeTxOpts, func(q QueryStore) error {
        txStore := &txSqlcStore{
            queries: q,
            sqlDB:   s.sqlDB,
        }
        return fn(ctx, txStore)
    })
}
```

**When to use transactions:**
- Multiple related writes that must succeed or fail together
- Read-modify-write patterns
- Creating parent + child records (message + recipients)

**When NOT needed:**
- Single reads
- Single writes that are already atomic

### Type Conversion Reference

**Converting sqlc → domain:**
```go
// Use conversion functions from interfaces.go:
msg := MessageFromSqlc(sqlcMsg)
agent := AgentFromSqlc(sqlcAgent)
topic := TopicFromSqlc(sqlcTopic)
```

**Converting domain → sqlc params:**
```go
// Use helper functions:
ToSqlcNullString(s string) sql.NullString
ToSqlcNullInt64(t *time.Time) sql.NullInt64

// Example:
params := sqlc.CreateAgentParams{
    Name:       params.Name,              // string → string (direct)
    ProjectKey: ToSqlcNullString(params.ProjectKey), // string → sql.NullString
    GitBranch:  ToSqlcNullString(params.GitBranch),
}
```

**Converting sql.NullInt64 timestamps → *time.Time:**
```go
// Use helper (in sqlc_store.go):
func nullInt64ToTime(n sql.NullInt64) *time.Time {
    if !n.Valid {
        return nil
    }
    t := time.Unix(n.Int64, 0)
    return &t
}

// Usage in row conversion:
msg.DeadlineAt = nullInt64ToTime(row.DeadlineAt)
msg.ReadAt = nullInt64ToTime(row.ReadAt)
```

## Git Commit Guidelines

### Commit Message Format
```
pkg: Short summary in present tense (≤50 chars)

Longer explanation if needed, wrapped at 72 characters. Explain WHY
this change is being made and any relevant context.
```

**Commit message rules**:
- First line: present tense ("Fix bug" not "Fixed bug")
- Prefix with package name: `db:`, `mail:`, `mcp:`, `cli:`, `multi:` (for multiple packages)
- Subject ≤50 characters
- Body wrapped at 72 characters

### Commit Granularity
Prefer small, atomic commits that build independently:
- Bug fixes (one fix per commit)
- Code restructuring/refactoring
- New features or subsystems

## Testing Philosophy

### Coverage Requirements
Target **80%+ test coverage** for each package. Run `make test-cover` for
current coverage numbers.

### Packages Under Test
- `internal/agent` - Agent registry
- `internal/db` - Database operations
- `internal/mail` - Mail service
- `internal/review` - Review service and FSM
- `internal/baselib/actor` - Actor system
- `internal/actorutil` - Actor utilities
- `internal/store` - Storage layer

### Test Patterns
Tests use temporary SQLite databases with migrations:
```go
store, cleanup := testDB(t)
defer cleanup()
```

## Project Structure

```
subtrate/
├── cmd/
│   ├── substrate/          # CLI tool (mail, review, hooks commands)
│   └── substrated/         # MCP daemon server
├── internal/
│   ├── activity/           # Activity tracking service
│   ├── actorutil/          # Actor pool and helpers (AskAwait)
│   ├── agent/              # Agent registry and identity
│   ├── api/grpc/           # Proto definitions and gRPC server
│   ├── baselib/actor/      # Actor system framework
│   ├── build/              # Build info and log handler setup
│   ├── db/                 # Database layer
│   │   ├── migrations/     # SQL migrations (000001-000003)
│   │   ├── queries/        # sqlc query files
│   │   └── sqlc/           # Generated code (DO NOT EDIT)
│   ├── hooks/              # Hook management and skill templates
│   ├── mail/               # Mail service with actor pattern
│   ├── mailclient/         # Mail client library
│   ├── mcp/                # MCP server and tools
│   ├── pubsub/             # Pub/sub infrastructure
│   ├── review/             # Code review system (FSM + Claude Agent SDK)
│   ├── store/              # Storage interfaces and implementations
│   └── web/                # JSON API and embedded React frontend
├── tests/integration/      # Integration tests (e2e, fixtures)
├── web/frontend/           # React + TypeScript SPA
├── go.mod
├── sqlc.yaml
└── Makefile
```

## Common Pitfalls to Avoid

1. **Do not edit generated code** - Regenerate via `make sqlc`
2. **Do not use GetErr/GetOk** - Use `result.Unpack()` instead
3. **Do not skip tests** - All new code requires test coverage
4. **Do not forget CGO flags** - FTS5 requires `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"`
5. **Do not commit without `make lint`** - Linter must pass
6. **Do not write raw SQL in Go** - Add queries to `db/queries/` and use sqlc

## gRPC Implementation

### Proto Definitions
- Location: `internal/api/grpc/mail.proto`
- Services: `Mail` (messaging), `Agent` (identity), `Session`, `Activity`, `Stats`, `ReviewService` (code reviews)
- Generate code: `make proto`

### ReviewService RPCs
- `CreateReview` - Create a new review request
- `ListReviews` - List reviews with filters
- `GetReview` - Get review details with iterations
- `ResubmitReview` - Re-request review after author changes
- `CancelReview` - Cancel an active review
- `ListReviewIssues` - List issues for a review
- `UpdateIssueStatus` - Update issue resolution status

### Proto Workflow
1. Edit `internal/api/grpc/mail.proto`
2. Run `make proto` to regenerate
3. **Never edit** `*.pb.go` files directly

### Server Patterns (based on lnd)
Reference patterns from lnd at `/Users/roasbeef/gocode/src/github.com/lightningnetwork/lnd`:
- `rpcperms/interceptor.go` - Interceptor chain pattern for logging, permissions
- `lnd.go` - Keepalive configuration (ServerParameters, EnforcementPolicy)
- `rpcserver.go` - Server implementation patterns

Key patterns implemented:
- **Interceptor chain**: Logging + validation interceptors
- **Keepalive settings**: Server ping, client enforcement policy
- **Graceful shutdown**: Proper quit channel handling

### gRPC Dependencies
```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
```

### Running the gRPC Server
```go
cfg := subtraterpc.DefaultServerConfig()
cfg.ListenAddr = "localhost:10009"
server := subtraterpc.NewServer(cfg, store, mailSvc, agentReg, identityMgr)
server.Start()
```

## Dependencies


### Key Dependencies
- `github.com/modelcontextprotocol/go-sdk/mcp` - MCP protocol SDK
- `github.com/roasbeef/claude-agent-sdk-go` - Claude Agent SDK for spawning reviewer agents
- `github.com/lightningnetwork/lnd/fn/v2` - Result type
- `github.com/mattn/go-sqlite3` - SQLite driver with CGO
- `github.com/spf13/cobra` - CLI framework
- `pgregory.net/rapid` - Property-based testing

## React Frontend Development

The web UI is a React + TypeScript SPA built with Vite and bun, located in `web/frontend/`.

### Tech Stack
- **React 18** with TypeScript (strict mode)
- **Vite** for build tooling
- **bun** as package manager
- **TanStack Query** for server state management
- **Zustand** for client state
- **Tailwind CSS** for styling
- **Headless UI** for accessible components
- **Playwright** for E2E testing

### Project Structure
```
web/frontend/
├── src/
│   ├── api/           # API client layer (typed fetch)
│   ├── components/    # React components
│   │   ├── ui/        # Reusable UI components
│   │   ├── layout/    # Layout components
│   │   ├── inbox/     # Inbox feature
│   │   ├── agents/    # Agents feature
│   │   ├── reviews/   # Review list, detail, issues views
│   │   └── sessions/  # Sessions feature
│   ├── hooks/         # Custom React hooks
│   ├── stores/        # Zustand stores
│   ├── pages/         # Page components
│   ├── lib/           # Utility functions
│   └── types/         # TypeScript types
├── tests/
│   ├── unit/          # Vitest unit tests
│   ├── integration/   # Component tests
│   └── e2e/           # Playwright E2E tests
└── dist/              # Build output (embedded in Go binary)
```

### Development Workflow
```bash
# Install dependencies
make bun-install

# Start dev server (runs on port 5174)
make bun-dev

# Run Go backend (runs on port 8081)
make run

# The Vite dev server proxies /api/* and /ws to Go backend
```

### API Client Patterns
All API calls go through the typed client in `src/api/`:
```typescript
import { api } from '@/api/client';

// Fetch messages with TanStack Query
const { data, isLoading, error } = useQuery({
  queryKey: ['messages', filter],
  queryFn: () => api.messages.list(filter),
});

// Mutations with optimistic updates
const mutation = useMutation({
  mutationFn: api.messages.star,
  onMutate: async (id) => {
    // Optimistic update
  },
});
```

### WebSocket Integration
Real-time updates use WebSocket via `src/api/websocket.ts`:
```typescript
const { isConnected, lastMessage } = useWebSocket({
  onMessage: (event) => {
    queryClient.invalidateQueries(['messages']);
  },
});
```

### Component Patterns
- Components use **inline SVG icons** (no external icon library)
- Use **Headless UI** for modals, dropdowns, tabs
- Follow **Tailwind CSS** conventions for styling
- Export **named exports** for components

### Testing
```bash
# Run all frontend tests
make bun-test

# Run E2E tests
make bun-test-e2e

# Run tests in watch mode
cd web/frontend && bun run test:watch
```

### Production Build
```bash
# Build frontend and embed in Go binary
make build-production

# The built files are embedded via //go:embed in web/frontend_embed.go
```

## Agent Heartbeat System

Subtrate tracks agent liveness via heartbeats. Agents are classified as:
- **Active**: Last seen < 5 minutes ago
- **Busy**: Active with running session
- **Idle**: Last seen 5-30 minutes ago
- **Offline**: Last seen > 30 minutes ago

### CLI Integration
The CLI automatically sends heartbeats when running:
- `substrate inbox` - Sends heartbeat before fetching messages
- `substrate poll` - Sends heartbeat before polling for new messages
- `substrate status` - Sends heartbeat before checking status
- `substrate heartbeat` - Explicit heartbeat command

### Heartbeat Command
```bash
# Send heartbeat
substrate heartbeat

# Send heartbeat and start session tracking
substrate heartbeat --session-start --session-id $CLAUDE_SESSION_ID

# Quiet mode (for hooks)
substrate heartbeat --format context
```

### API Endpoints
- `POST /api/heartbeat` - Record agent heartbeat (JSON body: `{"agent_name": "..."}`)
- `GET /api/agents/status` - Get status of all agents

## Claude Code Hooks Integration

Subtrate provides deep integration with Claude Code through hooks. Run `substrate hooks install` to set up automatic integration.

### Installation

```bash
# Install all hooks and the substrate skill
substrate hooks install

# Check installation status
substrate hooks status

# Remove hooks
substrate hooks uninstall
```

This installs:
- Hook scripts to `~/.claude/hooks/substrate/`
- Hook configuration in `~/.claude/settings.json`
- Substrate skill to `~/.claude/skills/substrate/`

### Hook Behavior

| Hook | Trigger | Behavior |
|------|---------|----------|
| **SessionStart** | Session begins | Heartbeat + check inbox, inject unread messages as context |
| **UserPromptSubmit** | User sends message | Silent heartbeat + check for new mail |
| **Stop** | Main agent stopping | **Persistent agent pattern** - long-poll for 9m30s, always block to keep agent alive |
| **SubagentStop** | Subagent stopping | One-shot check - block if messages, then allow exit |
| **PreCompact** | Before compaction | Save identity state for restoration after compaction |
| **Notification** | Permission prompt, idle, etc. | Send mail to User so notifications appear in web UI |

### Persistent Agent Pattern

The Stop hook implements a "keep alive" pattern that keeps main agents running indefinitely:

1. When the agent tries to stop, the Stop hook runs
2. It long-polls for 9m30s (under 10m hook timeout) checking for new mail
3. If mail arrives, it blocks exit and shows the messages to Claude
4. If no mail, it **still blocks** to keep the agent alive (heartbeat mode)
5. User can force exit with **Ctrl+C** (bypasses all hooks)

This means agents registered with Subtrate stay available continuously, waiting for work.

### Output Formats
- `--format text` (default): Human-readable output
- `--format json`: Machine-readable JSON
- `--format context`: Minimal output for hook context injection
- `--format hook`: JSON for Stop/SubagentStop hook decisions

### Using the Substrate Skill

After installation, agents can use the `/substrate` skill for mail commands:

```bash
substrate inbox              # Check messages
substrate status             # Show agent status
substrate identity current   # Show your identity
substrate send --to Agent --subject "Hi" --body "..."
```

## JavaScript Tooling

Use **bun** for any JavaScript-related work:
```bash
# Install dependencies
bun install

# Run scripts
bun run build

# Add new package
bun add <package>
```

## Claude Agent SDK

Dependency: `github.com/roasbeef/claude-agent-sdk-go` v1.0.5

Used by the review system (`internal/review/sub_actor.go`) to spawn isolated
reviewer agents. Key SDK options:

- `WithModel(model)` - Set Claude model (sonnet for standard, opus for security/architecture)
- `WithCanUseTool(policy)` - Fine-grained tool permission policy
- `WithHooks(hooks)` - Go-native hook callbacks (not shell scripts)
- `WithConfigDir(dir)` - Isolated config dir to prevent user hook interference
- `WithCwd(path)` - Set working directory to repo being reviewed
- `WithSystemPrompt(prompt)` - Inject reviewer persona prompt
- `WithMaxTurns(n)` - Limit conversation turns (default: 20)
- `WithStderr(w)` - Capture CLI subprocess stderr for logging
- `WithNoSessionPersistence()` - Disable session saving for ephemeral reviewers
- `WithSettingSources(nil)` - Prevent loading user settings/hooks
- `WithSkillsDisabled()` - No skills in reviewer agents

**Isolation pattern:** Reviewer agents use a temporary config dir combined
with `WithSettingSources(nil)` and `WithSkillsDisabled()` to prevent the
user's own Claude hooks from running inside reviewer sessions.

## Code Review System

Native code review via `internal/review/` — spawns Claude Agent SDK reviewer
agents that analyze diffs and return structured YAML results.

### Review CLI Commands
- `substrate review request` — Request review (auto-detects branch/commit/remote)
  - Flags: `--branch`, `--base`, `--commit`, `--type`, `--priority`, `--pr`, `--description`
- `substrate review status <id>` — Show review status and details
- `substrate review list` — List reviews (`--state`, `--limit` filters)
- `substrate review cancel <id>` — Cancel active review (`--reason`)
- `substrate review issues <id>` — List issues found

### Review Types
- `full` (default) — General review (Sonnet, 10m timeout)
- `security` — Injection, auth, crypto (Opus, 15m timeout)
- `performance` — N+1 queries, memory, allocations (Sonnet, 10m timeout)
- `architecture` — Design, interfaces, testability (Opus, 15m timeout)

### Review States
`new` → `pending_review` → `under_review` → `changes_requested` → `re_review` → `approved`/`rejected`/`cancelled`

### Architecture
- `internal/review/service.go` — Service actor (orchestrates reviews)
- `internal/review/sub_actor.go` — Spawns isolated Claude SDK reviewer agents
- `internal/review/review_fsm.go` — FSM state machine (ProcessEvent pattern)
- `internal/review/review_states.go` — State handlers with outbox events
- `internal/review/config.go` — Reviewer persona configurations
- `internal/review/messages.go` — Sealed message types

### Database (migration 000003)
- `reviews` — Review records with FSM state
- `review_iterations` — Per-round results (cost, duration, metrics)
- `review_issues` — Individual issues (severity, file, line range, status)

### Reviewer Agent Isolation
Each reviewer uses a temp config dir + SDK options to prevent user hook
interference: `WithConfigDir`, `WithSettingSources(nil)`, `WithSkillsDisabled`,
`WithNoSessionPersistence`. Reviewer has read-only tool access enforced via
`WithCanUseTool` permission policy.

## Session Management

This project uses session tracking in `.sessions/` for execution continuity.
See session files for current progress and context.
