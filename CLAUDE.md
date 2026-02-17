# Subtrate Development Guide

## Project Overview

Subtrate is a command center for managing Claude Code agents with mail/messaging,
pub/sub, threaded conversations, and log-based queue semantics.

**Key Components:**
- **substrated** (`cmd/substrated`) — MCP daemon server with integrated web UI
- **substrate** (`cmd/substrate`) — CLI for mail and review operations
- **Mail Service** (`internal/mail`) — Core messaging with actor pattern
- **Review Service** (`internal/review`) — Code review with Claude Agent SDK and FSM
- **Agent Registry** (`internal/agent`) — Agent identity and registration
- **Actor System** (`internal/baselib/actor`) — Actor framework (system, refs, futures, mailbox)
- **Web API** (`internal/web`) — JSON API and embedded React frontend
- **React Frontend** (`web/frontend`) — React + TypeScript SPA with Vite

## Essential Commands

### Build & Test
- `make build` — Compile all packages
- `make build-all` — Build CLI and MCP binaries
- `make build-production` — Build daemon with embedded frontend
- `make test` — Run all tests (handles FTS5 CGO flags automatically)
- `make test-race` — Run tests with race detector
- `make test-cover` — Run tests with coverage summary
- `make test-cover-html` — Generate HTML coverage report
- `make lint` — Run the linter (must pass before committing)
- `make fmt` — Format all Go source files
- `make ci` — Full CI pipeline (ci-go + ci-frontend)
- `make pre-commit` — Run all checks (tidy, fmt, vet, lint, test)

### Unit Tests (Single Package)
```bash
make unit pkg=./internal/mail                       # Entire package
make unit pkg=./internal/mail case=TestService_Send  # Specific test
make run-test test=TestFoo pkg=./internal/review     # Alternative
```

### Integration Tests
- `make test-integration` — All integration tests
- `make test-integration-e2e` — E2E backend tests (no external deps)
- `make test-integration-sdk` — SDK integration tests (requires claude CLI)
- `make test-integration-short` — Short tests (no API calls)

### Code Generation
- `make sqlc` — Regenerate type-safe DB queries (after schema/query changes)
- `make proto` — Generate gRPC code from protobuf definitions
- `make gen` — Run all code generation (sqlc + proto)

### Frontend
- `make bun-install` — Install frontend dependencies
- `make bun-build` — Build frontend for production
- `make bun-dev` — Start frontend dev server (port 5174)
- `make bun-test` — Run frontend unit/integration tests
- `make bun-lint` — Lint frontend code

### Running the Server
```bash
make run            # Web-only mode (foreground, default, port 8080)
make run-mcp        # MCP support (for Claude Code integration, uses stdio)
make start          # Background mode (web-only)
make stop           # Stop background server
make restart        # stop + rebuild + start
```

### Data & Log Locations
- **Database**: `~/.subtrate/subtrate.db` (SQLite with WAL mode)
- **Server logs**: `~/.subtrate/logs/substrated.log`
- **Agent identities**: `~/.subtrate/identities/`

## Code Style

### Function Comments
- **Every function/method** (including unexported) must have a comment starting
  with the function name. Explain how/why, not just what. Complete sentences.

### Formatting
- 80-character line limit (best effort).
- Organize code into logical stanzas separated by blank lines.
- When wrapping function calls, put closing paren on its own line.
- **Pack arguments densely** — fit multiple arguments per line up to 80 chars.

### Result Type API (CRITICAL)
This project uses `lnd/fn/v2` Result type. **Always use:**
```go
val, err := result.Unpack()
if err != nil {
    return err
}
```
**Never use** `GetErr()` or `GetOk()` — these don't exist.

### Actor System Patterns
Both the mail service and review service use the actor system in
`internal/baselib/actor/`:
- `BaseMessage` embedding for message types
- Sealed interfaces via unexported marker methods (`messageMarker()`, etc.)
- `Receive()` returns `fn.Result` type
- `ActorSystem` for lifecycle, `ServiceKey` for typed lookup
- `AskAwait` in `internal/actorutil/` for synchronous ask-and-unpack

## Common Pitfalls

1. **Do not edit generated code** — Regenerate via `make sqlc` or `make proto`
2. **Do not use GetErr/GetOk** — Use `result.Unpack()` instead
3. **Do not skip tests** — All new code requires test coverage
4. **Do not forget CGO flags** — FTS5 requires `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"`
   (Makefile handles this automatically; only needed for direct `go` commands)
5. **Do not commit without `make lint`** — Linter must pass
6. **Do not write raw SQL in Go** — Add queries to `db/queries/` and use sqlc

## Database Conventions

### SQLite with FTS5
Makefile handles `CGO_CFLAGS` automatically. If running go commands directly:
`CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...`

### sqlc Workflow
1. Edit migration files in `internal/db/migrations/`
2. Edit query files in `internal/db/queries/`
3. Run `make sqlc` to regenerate
4. **Never edit** files in `internal/db/sqlc/` directly

### Adding Database Migrations
1. Create `internal/db/migrations/NNNNNN_name.up.sql` and `.down.sql`
2. **CRITICAL**: Update `LatestMigrationVersion` in `internal/db/migrations.go`
3. Migrations auto-apply on server start and create backups

### Query Files
- `internal/db/queries/agents.sql` — Agent and session queries
- `internal/db/queries/topics.sql` — Topic and subscription queries
- `internal/db/queries/messages.sql` — Message and recipient queries
- `internal/db/queries/activities.sql` — Activity feed queries
- `internal/db/queries/reviews.sql` — Review, iteration, and issue queries

### Storage Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   store.Storage Interface                    │
│  (Domain types: store.Message, store.Agent, store.Topic)    │
├─────────────────────────────────────────────────────────────┤
│              SqlcStore / MockStore / txSqlcStore            │
├─────────────────────────────────────────────────────────────┤
│                   QueryStore Interface                       │
│  (sqlc types: sqlc.Message, sqlc.CreateMessageParams)       │
├─────────────────────────────────────────────────────────────┤
│              sqlc.Queries (generated code — DO NOT EDIT)     │
└─────────────────────────────────────────────────────────────┘
```

**Key interfaces** in `internal/store/interfaces.go`:
`MessageStore`, `AgentStore`, `TopicStore`, `ActivityStore`, `ReviewStore`,
`SessionStore`, `Storage` (combines all + `WithTx()` + `Close()`)

### Adding New Database Methods

Follow these 7 steps (see existing methods for patterns):
1. Add sqlc query in `internal/db/queries/*.sql`
2. Run `make sqlc` to regenerate types
3. Add to `QueryStore` interface in `internal/store/sqlc_store.go`
4. Add to domain interface in `internal/store/interfaces.go`
5. Implement in `SqlcStore` (convert sqlc types → domain types)
6. Implement in `txSqlcStore` (same pattern, uses `s.queries`)
7. Implement in `MockStore` (`internal/store/mock_store.go`)

### Transactions
```go
err := store.WithTx(ctx, func(ctx context.Context, txStore Storage) error {
    msg, err := txStore.CreateMessage(ctx, params)
    if err != nil {
        return err
    }
    return txStore.CreateMessageRecipient(ctx, msg.ID, recipientID)
})
```

### Type Conversion Helpers (in `internal/store/`)
- `MessageFromSqlc()`, `AgentFromSqlc()`, `TopicFromSqlc()` — sqlc → domain
- `ToSqlcNullString()`, `ToSqlcNullInt64()` — domain → sqlc params
- `nullInt64ToTime()` — sql.NullInt64 → *time.Time

## Git Commit Guidelines

```
pkg: Short summary in present tense (≤50 chars)

Longer explanation if needed, wrapped at 72 characters.
```

- Prefix with package name: `db:`, `mail:`, `mcp:`, `cli:`, `multi:`
- Present tense ("Fix bug" not "Fixed bug")
- Prefer small, atomic commits

## Testing

Target **80%+ coverage** per package. Run `make test-cover` for current numbers.

Tests use temporary SQLite databases:
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
│   │   ├── migrations/     # SQL migrations (000001-000008)
│   │   ├── queries/        # sqlc query files
│   │   └── sqlc/           # Generated code (DO NOT EDIT)
│   ├── hooks/              # Hook management and skill templates
│   ├── mail/               # Mail service with actor pattern
│   ├── mailclient/         # Mail client library
│   ├── mcp/                # MCP server and tools
│   ├── pubsub/             # Pub/sub infrastructure
│   ├── queue/              # Store-and-forward local queue
│   ├── review/             # Code review system (FSM + Claude Agent SDK)
│   ├── store/              # Storage interfaces and implementations
│   └── web/                # JSON API and embedded React frontend
├── tests/integration/      # Integration tests (e2e, fixtures)
├── web/frontend/           # React + TypeScript SPA
├── go.mod
├── sqlc.yaml
└── Makefile
```

## gRPC Implementation

- **Proto file**: `internal/api/grpc/mail.proto`
- **Services**: `Mail`, `Agent`, `Session`, `Activity`, `Stats`, `ReviewService`
- **Generate**: `make proto` (never edit `*.pb.go` directly)

### ReviewService RPCs
`CreateReview`, `ListReviews`, `GetReview`, `ResubmitReview`, `CancelReview`,
`ListReviewIssues`, `UpdateIssueStatus`

## Code Review System

Native code review via `internal/review/` — spawns Claude Agent SDK reviewer
agents that analyze diffs and return structured YAML results.

### CLI Commands
- `substrate review request` — Request review (auto-detects branch/commit/remote)
  - Flags: `--branch`, `--base`, `--commit`, `--type`, `--priority`, `--pr`
- `substrate review status <id>` — Show review status
- `substrate review list` — List reviews (`--state`, `--limit` filters)
- `substrate review resubmit <id>` — Resubmit after fixing issues
- `substrate review cancel <id>` — Cancel active review
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

## Claude Agent SDK

Dependency: `github.com/roasbeef/claude-agent-sdk-go`

Used by the review system (`internal/review/sub_actor.go`) to spawn isolated
reviewer agents. Key SDK options:

- `WithModel(model)` — Sonnet for standard, Opus for security/architecture
- `WithCanUseTool(policy)` — Fine-grained tool permission policy
- `WithHooks(hooks)` — Go-native hook callbacks (not shell scripts)
- `WithConfigDir(dir)` — Isolated config dir to prevent user hook interference
- `WithCwd(path)` — Set working directory to repo being reviewed
- `WithSystemPrompt(prompt)` — Inject reviewer persona prompt
- `WithMaxTurns(n)` — Limit conversation turns (default: 20)
- `WithStderr(w)` — Capture CLI subprocess stderr for logging
- `WithNoSessionPersistence()` — Disable session saving
- `WithSettingSources(nil)` — Prevent loading user settings/hooks
- `WithSkillsDisabled()` — No skills in reviewer agents

**Isolation pattern:** Reviewer agents use temp config dir + `WithSettingSources(nil)`
+ `WithSkillsDisabled()` to prevent user hooks from running inside reviewers.
Read-only tool access enforced via `WithCanUseTool` permission policy.

## Agent Systems

### Heartbeat
Agents are classified by last-seen time: **Active** (<5m), **Busy** (active + session),
**Idle** (5-30m), **Offline** (>30m). CLI commands automatically send heartbeats.

### Agent Discovery
`substrate agent discover` provides a unified view of all agents with metadata:
status, project key, git branch, working directory, hostname, purpose, unread count.
Supports `--status`, `--project`, `--name`, `--format json` filters.

## Claude Code Hooks Integration

Run `substrate hooks install` to set up automatic integration.

| Hook | Behavior |
|------|----------|
| **SessionStart** | Heartbeat + inject unread messages as context |
| **UserPromptSubmit** | Silent heartbeat + check for new mail |
| **Stop** | Long-poll 9m30s, always block to keep agent alive |
| **SubagentStop** | Block once if messages exist, then allow exit |
| **PreCompact** | Save identity for restoration after compaction |
| **Notification** | Send mail to User on permission prompts |
| **PostToolUse** (Write) | Track plan files in `.claude/.substrate-plan-context` |
| **PreToolUse** (ExitPlanMode) | Submit plan for review, block up to 9m for approval |

The Stop hook keeps agents alive indefinitely. **Ctrl+C** to force exit.

## Plan Mode Integration

Subtrate intercepts ExitPlanMode to enable async human review of plans:

1. Agent writes plan → PostToolUse tracks the file
2. Agent calls ExitPlanMode → PreToolUse intercepts
3. Hook submits plan via `substrate plan submit` and blocks up to 9m
4. If approved: ExitPlanMode proceeds. If timeout: denied, Stop hook keeps alive
5. Plan reviewed via web UI at `/plans/:id`

CLI: `substrate plan status`, `plan approve <id>`, `plan reject <id>`,
`plan request-changes <id>`

## React Frontend

React 18 + TypeScript SPA built with Vite and bun in `web/frontend/`.

**Tech stack:** TanStack Query (server state), Zustand (client state),
Tailwind CSS (styling), Headless UI (accessible components), Playwright (E2E).

```
web/frontend/src/
├── api/           # Typed API client
├── components/    # React components (ui/, layout/, inbox/, agents/, reviews/, sessions/)
├── hooks/         # Custom React hooks
├── stores/        # Zustand stores
├── pages/         # Page components
├── lib/           # Utility functions
└── types/         # TypeScript types
```

Dev workflow: `make bun-dev` (port 5174) proxies to Go backend (`make run`, port 8080).
Production: `make build-production` embeds built files via `//go:embed`.

## Key Dependencies

- `github.com/modelcontextprotocol/go-sdk/mcp` — MCP protocol SDK
- `github.com/roasbeef/claude-agent-sdk-go` — Claude Agent SDK
- `github.com/lightningnetwork/lnd/fn/v2` — Result type
- `github.com/mattn/go-sqlite3` — SQLite driver with CGO
- `github.com/spf13/cobra` — CLI framework
- `pgregory.net/rapid` — Property-based testing

## JavaScript Tooling

Use **bun** for all JavaScript work: `bun install`, `bun run build`, `bun add <pkg>`.

## Session Management

This project uses session tracking in `.sessions/` for execution continuity.
See session files for current progress and context.
