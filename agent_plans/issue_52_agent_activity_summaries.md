# Issue #52: Real-time Agent Activity Summaries

## Overview

Add real-time session summaries using Claude Haiku via Go Agent SDK so the
agent dashboard shows *what agents are actually doing*, not just if they're
online. Enhanced with a click-to-focus detail view inspired by the multi-agent
inbox reference design.

## Phase 1: Backend — Transcript Reader

**Goal**: Read Claude Code session transcripts from disk.

### Files
- `internal/summary/transcript.go` — NEW

### Details
- `TranscriptReader` struct with configurable base path (`~/.claude/`)
- `ReadRecentTranscript(projectKey, sessionID, maxLines)` — tail of session
- `FindActiveSession(projectKey)` — discover most recent session file
- SHA-256 hash of content for cache invalidation
- Graceful fallback when transcript missing/unreadable

## Phase 2: Backend — Haiku Summarizer via Go Agent SDK

**Goal**: Spawn lightweight Haiku agents to generate 1-2 sentence summaries.

### Files
- `internal/summary/summarizer.go` — NEW (sub-actor, SDK spawn)
- `internal/summary/prompts.go` — NEW (system prompt)
- `internal/summary/types.go` — NEW (SummaryResult, cachedSummary)
- `internal/summary/messages.go` — NEW (sealed message types)
- `internal/summary/service.go` — NEW (service actor with cache)
- `internal/summary/config.go` — NEW (Config struct)

### SDK Options
```go
claudeagent.WithModel("claude-haiku-4-5-20251001")
claudeagent.WithMaxTurns(1)
claudeagent.WithSystemPrompt(summarizerPrompt)
claudeagent.WithConfigDir(tempDir)
claudeagent.WithSettingSources(nil)
claudeagent.WithSkillsDisabled()
claudeagent.WithNoSessionPersistence()
claudeagent.WithCanUseTool(denyAllPolicy)
```

### Cache Strategy
- In-memory map with 30-second TTL
- Transcript hash dedup (don't re-summarize unchanged content)
- Return stale value + `is_stale` flag while regenerating (non-blocking)
- WebSocket broadcast on fresh summary

## Phase 3: Backend — API & Database

### Database
- `internal/db/migrations/000004_summaries.up.sql` — agent_summaries table
- `internal/db/migrations/000004_summaries.down.sql` — drop table
- `internal/db/migrations.go` — bump LatestMigrationVersion to 4
- `internal/db/queries/summaries.sql` — sqlc queries
- Run `make sqlc` to regenerate

### Store Layer
- `internal/store/interfaces.go` — add SummaryStore interface
- `internal/store/sqlc_store.go` — implement SummaryStore
- `internal/store/mock_store.go` — mock implementation

### Proto/API
- `internal/api/grpc/mail.proto` — AgentSummary message + RPCs
- `internal/api/grpc/mail.yaml` — REST route mappings
- `internal/api/grpc/mail_rpc.go` — RPC handlers
- Run `make proto` to regenerate

### New Endpoints
- `GET /api/v1/agents/{agent_id}/summary`
- `GET /api/v1/agents/summaries`
- `GET /api/v1/agents/{agent_id}/summary-history`

## Phase 4: Frontend — Enhanced Agent Cards

### Files
- `web/frontend/src/api/summaries.ts` — NEW (API client)
- `web/frontend/src/types/api.ts` — EDIT (add AgentSummary type)
- `web/frontend/src/hooks/useSummaries.ts` — NEW (TanStack Query hooks)
- `web/frontend/src/hooks/useSummariesRealtime.ts` — NEW (WebSocket)
- `web/frontend/src/components/agents/AgentCard.tsx` — EDIT (add summary)
- `web/frontend/src/pages/AgentsDashboard.tsx` — EDIT (wire summaries)

### Enhanced Card Layout
```
┌──────────────────────────────────────┐
│  [Avatar]  agent-name     [● Active] │
│            substrate/main             │
│            2m ago                     │
│                                       │
│  ┌─ Current Activity ─────────────┐  │
│  │ Working on WebSocket reconnect │  │
│  │ logic with exponential backoff │  │
│  │                                │  │
│  │ Δ Now on error handling        │  │
│  │                     12s ago ↻  │  │
│  └────────────────────────────────┘  │
│                                       │
│  Session #482                         │
└──────────────────────────────────────┘
```

## Phase 5: Frontend — Agent Detail View (Click-to-Focus)

### Files
- `web/frontend/src/components/agents/AgentDetailPanel.tsx` — NEW
- `web/frontend/src/components/agents/ActivityTimeline.tsx` — NEW
- `web/frontend/src/pages/AgentsDashboard.tsx` — EDIT (add detail state)

### Detail View Layout
- Back navigation ("← Back to all agents")
- Large agent header with full context
- Current activity summary card
- Unified activity timeline merging summaries + activities + messages
- Vertical dot-timeline with timestamps (inspired by reference design)

## Phase 6: Service Wiring

### Files
- `cmd/substrated/main.go` — EDIT (wire summary service)
- `internal/web/server.go` — EDIT (register gateway, WS event)

### Background Refresh
- Every 30s, iterate active/busy agents
- Read transcript, check hash, spawn Haiku if changed
- Broadcast via WebSocket

## Implementation Order

1. Phase 3 (DB + proto) — unblocks everything
2. Phase 1 (transcript reader) — no deps
3. Phase 2 (summarizer) — depends on 1
4. Phase 6 (wiring) — depends on 2+3
5. Phase 4 (enhanced cards) — depends on 3 API
6. Phase 5 (detail view) — depends on 4
