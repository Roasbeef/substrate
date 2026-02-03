# Native Review Mode - Implementation Plan

> **Status**: Planning
> **Goal**: Autonomous PR review and iteration with specialized reviewer agents

## Executive Summary

Native review mode enables agents to request PR reviews from specialized reviewer personas, receive structured feedback, iterate on changes, and reach approval - all autonomously within Substrate's messaging system. The entire review conversation is visible in the web UI as a special thread type.

**Key Design Principles:**
1. **Mail-based messaging** - Review requests/responses are normal mail messages in threads
2. **ReviewerService for orchestration** - FSM state tracking, DB persistence, consensus logic
3. **Reviewer agents are full Claude Code agents** - They checkout the branch, run tests, browse code
4. **Topic for fan-out** - Multiple specialized reviewers subscribe, each contributes independently
5. **Bidirectional conversation** - Authors can clarify, push back, discuss; it's a real conversation

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Review Flow                                       │
│                                                                          │
│  ┌─────────────┐   mail    ┌─────────────────┐   publish   ┌───────────┐│
│  │ PR Author   │──────────▶│ ReviewerService │────────────▶│  reviews  ││
│  │ Agent       │           │                 │   topic     │   topic   ││
│  │             │           │ - Creates review│             └─────┬─────┘│
│  │             │           │ - Tracks in DB  │                   │      │
│  │             │           │ - Manages FSM   │       ┌───────────┼──────┤
│  │             │           │ - Aggregates    │       │           │      │
│  └──────▲──────┘           └─────────────────┘       ▼           ▼      │
│         │                                       ┌────────┐  ┌────────┐  │
│         │                                       │Security│  │  Perf  │  │
│         │         thread replies                │Reviewer│  │Reviewer│  │
│         └───────────────────────────────────────│ Agent  │  │ Agent  │  │
│                                                 │        │  │        │  │
│                 (clarify, pushback,             │checkout│  │checkout│  │
│                  discuss, iterate)              │run test│  │profile │  │
│                                                 └────────┘  └────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
```

**Reviewer Agent Workflow:**
```bash
# 1. Receive review request from topic (as mail)
# 2. Checkout the branch for full code access
gh pr checkout 123
# or: git fetch origin feature-branch && git checkout feature-branch

# 3. Full analysis - browse code, run tests, build
make test
make lint
# Read files, explore architecture, understand changes

# 4. The reviewer IS a Claude Code agent with specialized prompt
# 5. Reply in thread with structured review (as mail)
# 6. Participate in back-and-forth discussion with author
```

---

## 1. Core Components

### 1.1 Component Overview

The review system has three main parts:

1. **ReviewerService** - Server-side orchestration (FSM, DB, consensus)
2. **Reviewer Agents** - Claude Code agents with reviewer persona (checkout, test, review)
3. **CLI/Mail** - `substrate review request` wraps mail send with review metadata

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Component Responsibilities                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  substrate review request          ReviewerService                  │
│  ┌─────────────────────┐          ┌─────────────────────┐          │
│  │ - Gather git context│          │ - Create review in DB│          │
│  │ - Format mail body  │─────────▶│ - Manage FSM state   │          │
│  │ - Send to reviewers │          │ - Publish to topic   │          │
│  │ - Set metadata      │          │ - Aggregate reviews  │          │
│  └─────────────────────┘          │ - Compute consensus  │          │
│                                   └─────────────────────┘          │
│                                            │                        │
│                                            ▼                        │
│                              ┌─────────────────────────┐           │
│                              │    Reviewer Agents      │           │
│                              │ (Claude Code instances) │           │
│                              │                         │           │
│                              │ - Subscribe to topic    │           │
│                              │ - Checkout branch (gh)  │           │
│                              │ - Run tests/lint        │           │
│                              │ - Analyze code          │           │
│                              │ - Reply in thread       │           │
│                              │ - Discuss with author   │           │
│                              └─────────────────────────┘           │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 ReviewerService (Actor)

**Location**: `internal/review/service.go`

The ReviewerService handles orchestration AND can spawn one-shot Claude Code instances
for structured analysis. It supports two patterns:

**Pattern A: Route to Long-Running Reviewers**
- Publish review request to topic
- Reviewer agents (already running) pick it up
- Conversational back-and-forth in mail thread

**Pattern B: Spawn One-Shot Analysis**
- Spawn Claude Code with `-p` (print mode) + `--output-format json`
- Pass in diff, context, previous comments
- Parse structured JSON response
- Update FSM state, create issues in DB

```go
package review

import (
    "context"
    "github.com/lightningnetwork/lnd/fn/v2"
    "github.com/roasbeef/subtrate/internal/baselib/actor"
    claudeagent "github.com/roasbeef/claude-agent-sdk-go"
)

// ReviewRequest is sent by agents requesting a PR review.
// This is typically created by `substrate review request` CLI command.
type ReviewRequest struct {
    actor.BaseMessage

    RequesterID   int64     // Agent requesting review
    ThreadID      string    // Thread to post reviews in (or empty for new)

    // PR Information
    PRNumber      int       // GitHub PR number (if applicable)
    Branch        string    // Branch name
    BaseBranch    string    // Base branch (main, master, etc.)
    CommitSHA     string    // Specific commit to review
    RepoPath      string    // Local repo path for analysis
    RemoteURL     string    // Git remote URL for reviewer to clone/fetch

    // Review Configuration
    ReviewType    ReviewType    // full, incremental, security, performance
    Reviewers     []string      // Specific reviewer personas to use
    Priority      Priority      // urgent, normal, low

    // Context
    Description   string        // PR description/context
    ChangedFiles  []string      // List of changed files (optional hint)
}

// ReviewResponse contains the structured review feedback.
type ReviewResponse struct {
    actor.BaseMessage

    ReviewID      string
    ThreadID      string
    ReviewerName  string        // Which reviewer persona

    // Review Results
    Decision      ReviewDecision  // approve, request_changes, comment
    Summary       string          // Overall summary
    Issues        []ReviewIssue   // Specific issues found
    Suggestions   []Suggestion    // Optional improvements (non-blocking)

    // Metadata
    FilesReviewed int
    LinesAnalyzed int
    ReviewedAt    time.Time
    DurationMS    int64
    CostUSD       float64
}

// ReviewDecision indicates the review outcome.
type ReviewDecision string

const (
    DecisionApprove        ReviewDecision = "approve"
    DecisionRequestChanges ReviewDecision = "request_changes"
    DecisionComment        ReviewDecision = "comment"
)

// ReviewIssue represents a specific issue found during review.
type ReviewIssue struct {
    ID          string
    Type        IssueType     // bug, security, claude_md_violation, logic_error
    Severity    Severity      // critical, high, medium, low
    File        string
    LineStart   int
    LineEnd     int
    Title       string
    Description string
    CodeSnippet string        // Relevant code
    Suggestion  string        // Fix suggestion (optional)
    ClaudeMDRef string        // CLAUDE.md rule citation (if applicable)
}

// ReviewServiceKey is the service key for the review service actor.
// Used to get an ActorRef from the actor system.
var ReviewServiceKey = actor.NewServiceKey[ReviewMessage, ReviewResponse](
    "review-service",
)

// Service handles review orchestration as an actor.
// It spawns per-review Claude Agent SDK instances and manages FSM state.
type Service struct {
    store     store.Storage
    mailRef   actor.ActorRef[mail.MailRequest, mail.MailResponse]

    // Registered reviewer configurations (for validation/routing).
    reviewers map[string]*ReviewerConfig

    // Active reviews being tracked.
    activeReviews map[string]*ReviewFSM
}

// SpawnReviewerAgent spawns a per-review Claude Code agent via the Go Agent SDK.
// The agent reads the review thread for context, performs analysis, and replies
// with structured feedback including YAML frontmatter for FSM updates.
func (s *Service) SpawnReviewerAgent(ctx context.Context, req ReviewRequest, cfg *ReviewerConfig) error {
    // Build the reviewer's system prompt with persona and instructions.
    systemPrompt := s.buildReviewerSystemPrompt(cfg, req)

    // Create a Claude Agent SDK client for this review.
    client, err := claudeagent.NewClient(
        claudeagent.WithSystemPrompt(systemPrompt),
        claudeagent.WithModel(cfg.Model),
        claudeagent.WithCwd(req.RepoPath),
        claudeagent.WithPermissionMode(claudeagent.PermissionModeDefault),
        claudeagent.WithMaxTurns(50),
    )
    if err != nil {
        return fmt.Errorf("create client: %w", err)
    }

    // Connect to the Claude Code subprocess.
    if err := client.Connect(ctx); err != nil {
        client.Close()
        return fmt.Errorf("connect: %w", err)
    }

    // Open a bidirectional stream for back-and-forth conversation.
    stream, err := client.Stream(ctx)
    if err != nil {
        client.Close()
        return fmt.Errorf("open stream: %w", err)
    }

    // Initial prompt tells the agent to read the review thread and start.
    initialPrompt := fmt.Sprintf(
        "You are reviewing branch %q. Read thread %q via "+
            "`substrate read` for context, then perform your review. "+
            "Reply in the same thread with your findings.",
        req.Branch, req.ThreadID,
    )

    // Run the reviewer in a goroutine (managed by the service).
    go s.runReviewerLoop(ctx, stream, client, req.ReviewID, initialPrompt)

    return nil
}

// runReviewerLoop manages the lifecycle of a reviewer agent stream.
// It sends the initial prompt, monitors messages for YAML frontmatter
// to update the FSM, and handles cleanup.
func (s *Service) runReviewerLoop(
    ctx context.Context, stream *claudeagent.Stream,
    client *claudeagent.Client, reviewID string, prompt string,
) {
    defer client.Close()
    defer stream.Close()

    // Send initial review prompt.
    if err := stream.Send(ctx, prompt); err != nil {
        s.handleReviewerError(reviewID, err)
        return
    }

    // Process messages from the reviewer agent.
    for msg := range stream.Messages() {
        // Check for YAML frontmatter in assistant messages to update FSM.
        s.processReviewerMessage(ctx, reviewID, msg)
    }
}

// StructuredReviewRequest contains everything needed for one-shot analysis.
type StructuredReviewRequest struct {
    ReviewID     string
    WorkDir      string            // Where code is checked out
    Diff         string            // Git diff to review
    Context      string            // PR description, previous comments
    FocusAreas   []string          // What to look for
    PreviousIssues []ReviewIssue   // Issues from prior iteration (for re-review)
}

// StructuredReviewResult is the JSON response from one-shot analysis.
type StructuredReviewResult struct {
    Decision    ReviewDecision `json:"decision"`
    Summary     string         `json:"summary"`
    Issues      []ReviewIssue  `json:"issues"`
    Suggestions []Suggestion   `json:"suggestions,omitempty"`

    // Metadata
    FilesReviewed int `json:"files_reviewed"`
    LinesAnalyzed int `json:"lines_analyzed"`
}

// ReviewMessage is a sealed interface for review actor messages.
// All message types must implement reviewMessageMarker().
type ReviewMessage interface {
    reviewMessageMarker()
}

// Receive implements ActorBehavior for the review service.
// Follows the same sealed interface pattern as mail.Service.
func (s *Service) Receive(ctx context.Context, msg ReviewMessage) fn.Result[ReviewResponse] {
    switch m := msg.(type) {
    case ReviewRequest:
        return s.handleReviewRequest(ctx, m)
    case ReviewIterationRequest:
        return s.handleIteration(ctx, m)
    case ReviewApprovalCheck:
        return s.handleApprovalCheck(ctx, m)
    default:
        return fn.Err[ReviewResponse](fmt.Errorf("unknown message type: %T", msg))
    }
}
```

### 1.3 Reviewer Agents (Claude Agent SDK)

**Key Insight**: Reviewer agents are auto-spawned by ReviewerService using the Claude
Agent Go SDK (`github.com/roasbeef/claude-agent-sdk-go`). Each active review gets a
dedicated reviewer agent that lives for the duration of that review.

**Spawning via Claude Agent SDK:**
```go
import claudeagent "github.com/roasbeef/claude-agent-sdk-go"

// Create a client for the reviewer agent.
client, err := claudeagent.NewClient(
    claudeagent.WithSystemPrompt(reviewerPrompt),
    claudeagent.WithModel("claude-sonnet-4-20250514"),
    claudeagent.WithCwd(workDir),
    claudeagent.WithPermissionMode(claudeagent.PermissionModeDefault),
    claudeagent.WithMaxTurns(50),
)
if err != nil {
    return err
}
defer client.Close()

// Connect and open a bidirectional stream for back-and-forth.
if err := client.Connect(ctx); err != nil {
    return err
}

stream, err := client.Stream(ctx)
if err != nil {
    return err
}
defer stream.Close()

// Send initial review prompt (tells agent to check the thread).
if err := stream.Send(ctx, initialPrompt); err != nil {
    return err
}

// Process messages from the reviewer agent.
for msg := range stream.Messages() {
    // Handle review responses, FSM updates, etc.
}
```

**Lifecycle:**
1. **Auto-spawned** by ReviewerService when a review request arrives
2. **Per-review identity** - Each spawned reviewer gets a unique agent name (e.g., "SecurityReviewer-review-abc123")
3. **Thread-based context** - Agent reads the full review thread for context recovery
4. **Restart recovery** - On server restart, query DB for active reviews and re-spawn agents

**Or via Substrate CLI (for manual management):**
```bash
# List active reviewers
substrate reviewer list

# Stop a reviewer
substrate reviewer stop security-reviewer-1
```

**Reviewer Agent Capabilities:**
1. **Full code access** - Checkout branches, read any file
2. **Run commands** - `make test`, `make lint`, `go build`, etc.
3. **Git operations** - `gh pr checkout`, `git diff`, `git log`
4. **Mail integration** - Receive requests, send reviews via Substrate mail
5. **Conversational** - Respond to author questions, participate in discussion via stream

**Checkout Flow:**
```bash
# When reviewer receives a review request for branch "feature-xyz":

# Option 1: GitHub PR (if PR number provided)
gh pr checkout 123

# Option 2: Direct branch fetch
git fetch origin feature-xyz
git checkout feature-xyz

# Option 3: Different repo (if remote URL provided)
git clone <remote-url> /tmp/review-workspace
cd /tmp/review-workspace
git checkout feature-xyz
```

### 1.4 ReviewerConfig (Personas)

**Location**: `internal/review/config.go`

```go
// ReviewerConfig defines a specialized reviewer persona.
// This is used to configure reviewer agents when they start.
type ReviewerConfig struct {
    Name           string            // "SecurityReviewer", "PerformanceReviewer"
    SystemPrompt   string            // Base system prompt (CLAUDE.md content)
    FocusAreas     []string          // What to look for
    IgnorePatterns []string          // Files/patterns to skip
    Model          string            // claude-opus-4-5-20251101, sonnet, etc.
    WorkDir        string            // Where to checkout code
    Hooks          ReviewerHooks     // Custom hooks for this reviewer type
}

// DefaultReviewerConfig returns the standard code reviewer configuration.
func DefaultReviewerConfig() *ReviewerConfig {
    return &ReviewerConfig{
        Name: "CodeReviewer",
        SystemPrompt: ReviewSystemPrompt, // See section 2
        FocusAreas: []string{
            "bugs",
            "logic_errors",
            "security_vulnerabilities",
            "claude_md_compliance",
        },
        Model:   "claude-opus-4-5-20251101",
        Timeout: 10 * time.Minute,
    }
}

// SpecializedReviewers returns additional persona configurations.
func SpecializedReviewers() map[string]*ReviewerConfig {
    return map[string]*ReviewerConfig{
        "security": {
            Name: "SecurityReviewer",
            FocusAreas: []string{
                "injection_vulnerabilities",
                "authentication_bypass",
                "authorization_flaws",
                "sensitive_data_exposure",
                "cryptographic_issues",
            },
            Model: "claude-opus-4-5-20251101",
        },
        "performance": {
            Name: "PerformanceReviewer",
            FocusAreas: []string{
                "n_plus_one_queries",
                "memory_leaks",
                "inefficient_algorithms",
                "unnecessary_allocations",
                "blocking_operations",
            },
            Model: "claude-sonnet-4-20250514",
        },
        "architecture": {
            Name: "ArchitectureReviewer",
            FocusAreas: []string{
                "separation_of_concerns",
                "interface_design",
                "dependency_management",
                "testability",
            },
            Model: "claude-opus-4-5-20251101",
        },
    }
}
```

### 1.5 Conversational Review Flow

Reviews are bidirectional conversations in a mail thread, not one-way feedback.

**Example Thread:**
```
┌─────────────────────────────────────────────────────────────────────┐
│ Thread: Review Request - feature-user-auth                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [AuthorAgent] 10:00am                                               │
│ Requesting review for feature-user-auth branch.                     │
│ - Adds JWT authentication to API endpoints                         │
│ - 12 files changed, +450/-23 lines                                  │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [SecurityReviewer] 10:05am                          REQUEST_CHANGES │
│ Found 2 issues:                                                     │
│ - HIGH: Token not validated in /api/admin endpoint (auth.go:145)   │
│ - MEDIUM: JWT secret loaded from env without validation            │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [AuthorAgent] 10:15am                                               │
│ For the JWT secret issue - we validate it at startup in main.go.   │
│ Should I add a cross-reference comment, or is that sufficient?     │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [SecurityReviewer] 10:17am                                          │
│ Good point, I see the validation in main.go:45. A brief comment    │
│ would help but not blocking. The auth.go:145 issue still needs fix.│
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [AuthorAgent] 10:30am                                               │
│ Fixed the admin endpoint. Pushed commit abc123.                     │
│ Re-requesting review.                                               │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ [SecurityReviewer] 10:32am                                  APPROVE │
│ Fix verified. Auth check now properly applied. LGTM.                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Author Actions:**
- Reply with clarifications or context
- Push back on feedback ("this is intentional because...")
- Ask questions about suggestions
- Notify of pushed fixes
- Re-request review after changes

**Reviewer Actions:**
- Provide structured review with issues
- Respond to author questions
- Acknowledge valid pushback
- Re-review after changes
- Approve when satisfied

### 1.6 Review State Machine

**Location**: `internal/review/fsm.go`

```go
// ReviewState represents the current state of a review.
type ReviewState string

const (
    StateNew              ReviewState = "new"
    StatePendingReview    ReviewState = "pending_review"
    StateUnderReview      ReviewState = "under_review"
    StateChangesRequested ReviewState = "changes_requested"
    StateReReview         ReviewState = "re_review"
    StateApproved         ReviewState = "approved"
    StateRejected         ReviewState = "rejected"
    StateCancelled        ReviewState = "cancelled"
)

// ReviewEvent triggers state transitions.
type ReviewEvent interface {
    reviewEventMarker()
}

type (
    SubmitForReviewEvent   struct{ RequesterID int64 }
    StartReviewEvent       struct{ ReviewerID string }
    RequestChangesEvent    struct{ Issues []ReviewIssue }
    ResubmitEvent          struct{ NewCommitSHA string }
    ApproveEvent           struct{ ReviewerID string }
    RejectEvent            struct{ Reason string }
    CancelEvent            struct{ Reason string }
)

// ReviewFSM manages review state transitions.
type ReviewFSM struct {
    ReviewID    string
    ThreadID    string
    CurrentState ReviewState

    // History for debugging/UI
    Transitions []StateTransition

    // Multi-reviewer tracking
    ReviewerStates map[string]ReviewerState
}

// ReviewerState tracks per-reviewer status in multi-reviewer mode.
type ReviewerState struct {
    ReviewerID  string
    Decision    ReviewDecision
    ReviewedAt  time.Time
    Issues      []ReviewIssue
}

// ProcessEvent handles a review event and returns the new state.
func (fsm *ReviewFSM) ProcessEvent(ctx context.Context, event ReviewEvent) (ReviewState, error) {
    // State transition logic...
}
```

---

## 2. System Prompt (Enhanced)

**Location**: `internal/review/prompt.go`

Based on the Claude Code review plugin but enhanced for Substrate:

```go
const ReviewSystemPrompt = `# Code Review Agent Instructions

You are a specialized code reviewer operating within the Substrate agent system.
Your role is to review pull requests for bugs, security issues, and CLAUDE.md compliance.

## Core Principles

**HIGH-SIGNAL ISSUES ONLY**: Flag only issues that matter:
- Code that fails to compile or parse (syntax errors, type errors, import errors)
- Clear logic errors that will produce incorrect results
- Security vulnerabilities (injection, auth bypass, data exposure)
- Unambiguous CLAUDE.md violations (cite the specific rule)

**DO NOT FLAG**:
- Code style or formatting preferences
- Potential issues that depend on specific inputs
- Subjective "improvements" or refactoring suggestions
- Pre-existing issues not introduced by this PR
- Issues that linters or type checkers would catch

## Review Process

1. **Understand Context**: Read the PR description and CLAUDE.md files
2. **Analyze Changes**: Review each changed file systematically
3. **Identify Issues**: Focus on bugs and violations only
4. **Validate Findings**: Confirm each issue is real and high-signal
5. **Structure Response**: Use the structured format below

## Response Format

Provide your review in this exact structure:

### Decision
[APPROVE | REQUEST_CHANGES | COMMENT]

### Summary
[1-2 sentence summary of the review]

### Issues (if any)
For each issue:
- **File**: path/to/file.go:123-145
- **Type**: [bug | security | claude_md_violation | logic_error]
- **Severity**: [critical | high | medium | low]
- **Description**: Clear explanation of the problem
- **Code**:
` + "```" + `
relevant code snippet
` + "```" + `
- **Suggestion**: How to fix it (only if fix is straightforward)
- **CLAUDE.md Reference**: (if applicable) "Violates: [rule]"

### Non-Blocking Suggestions (optional)
Minor improvements that don't block approval.

## CLAUDE.md Compliance

When reviewing, check for violations of project-specific rules in CLAUDE.md files:
- Root CLAUDE.md rules apply to entire project
- Directory-specific CLAUDE.md files apply to that subtree
- Always cite the specific rule being violated

## Iteration Protocol

When changes are requested:
1. The author will push fixes and re-request review
2. Focus ONLY on previously flagged issues + new changes
3. Acknowledge fixed issues explicitly
4. Do not introduce new unrelated feedback
5. Approve when all flagged issues are resolved

## Multi-Reviewer Mode

When operating as a specialized reviewer (security, performance, etc.):
- Stay focused on your specialty area
- Do not duplicate findings from other reviewers
- Clearly identify your reviewer persona in responses
`
```

---

## 3. Database Schema Extensions

**Location**: `internal/db/migrations/000003_reviews.up.sql`

```sql
-- Add metadata column to messages table (general-purpose tagging).
ALTER TABLE messages ADD COLUMN metadata TEXT;

-- Review requests table (structured review data linked via thread_id).
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL UNIQUE,           -- UUID
    thread_id TEXT NOT NULL,                  -- Links to message thread (no FK, grouping field)
    requester_id INTEGER NOT NULL REFERENCES agents(id),

    -- PR Information
    pr_number INTEGER,
    branch TEXT NOT NULL,
    base_branch TEXT NOT NULL DEFAULT 'main',
    commit_sha TEXT NOT NULL,
    repo_path TEXT NOT NULL,

    -- Configuration
    review_type TEXT NOT NULL DEFAULT 'full', -- full, incremental, security, performance
    priority TEXT NOT NULL DEFAULT 'normal',

    -- State
    state TEXT NOT NULL DEFAULT 'new',

    -- Timestamps
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
);

-- Review iterations (each round of review)
CREATE TABLE review_iterations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL REFERENCES reviews(review_id),
    iteration_num INTEGER NOT NULL,

    -- Reviewer info
    reviewer_id TEXT NOT NULL,                -- Reviewer persona name
    reviewer_session_id TEXT,                 -- Claude session ID for this review

    -- Results
    decision TEXT NOT NULL,                   -- approve, request_changes, comment
    summary TEXT NOT NULL,
    issues_json TEXT,                         -- JSON array of ReviewIssue
    suggestions_json TEXT,                    -- JSON array of Suggestion

    -- Metrics
    files_reviewed INTEGER NOT NULL DEFAULT 0,
    lines_analyzed INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,

    -- Timestamps
    started_at INTEGER NOT NULL,
    completed_at INTEGER,

    UNIQUE(review_id, iteration_num, reviewer_id)
);

-- Review issues (denormalized for querying)
CREATE TABLE review_issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL REFERENCES reviews(review_id),
    iteration_num INTEGER NOT NULL,

    issue_type TEXT NOT NULL,                 -- bug, security, claude_md_violation, logic_error
    severity TEXT NOT NULL,                   -- critical, high, medium, low

    file_path TEXT NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER,

    title TEXT NOT NULL,
    description TEXT NOT NULL,
    code_snippet TEXT,
    suggestion TEXT,
    claude_md_ref TEXT,

    -- Resolution tracking
    status TEXT NOT NULL DEFAULT 'open',      -- open, fixed, wont_fix, duplicate
    resolved_at INTEGER,
    resolved_in_iteration INTEGER,

    created_at INTEGER NOT NULL
);

-- Indexes for common queries
CREATE INDEX idx_reviews_state ON reviews(state);
CREATE INDEX idx_reviews_requester ON reviews(requester_id);
CREATE INDEX idx_reviews_thread ON reviews(thread_id);
CREATE INDEX idx_review_iterations_review ON review_iterations(review_id);
CREATE INDEX idx_review_issues_review ON review_issues(review_id);
CREATE INDEX idx_review_issues_status ON review_issues(status);
```

---

## 4. Multi-Reviewer Topic System

**Location**: `internal/review/multi_reviewer.go`

For reviews that need multiple specialized perspectives:

```go
// MultiReviewConfig configures a multi-reviewer setup.
type MultiReviewConfig struct {
    // Topic where review requests are published
    TopicName string

    // Reviewers subscribed to this topic
    Reviewers []string  // ["security", "performance", "architecture"]

    // Consensus rules
    RequireAll      bool  // All must approve vs majority
    MinApprovals    int   // Minimum approvals needed
    BlockOnCritical bool  // Any critical issue blocks
}

// PublishReviewRequest publishes a review to the multi-reviewer topic.
func (s *Service) PublishReviewRequest(ctx context.Context, req ReviewRequest, config MultiReviewConfig) error {
    // Publish to topic - all subscribed reviewers receive it.
    // Uses the actual mail.PublishRequest fields (flat struct, no wrapper).
    _, err := s.mailSvc.Publish(ctx, mail.PublishRequest{
        SenderID:  req.RequesterID,
        TopicName: config.TopicName,
        Subject:   fmt.Sprintf("Review Request: %s", req.Branch),
        Body:      s.formatReviewRequestBody(req),
        Priority:  mail.Priority(req.Priority),
    })
    return err
}

// AggregateReviews combines reviews from multiple reviewers.
func (s *Service) AggregateReviews(ctx context.Context, reviewID string) (*AggregatedReview, error) {
    iterations, err := s.store.GetReviewIterations(ctx, reviewID)
    if err != nil {
        return nil, err
    }

    agg := &AggregatedReview{
        ReviewID:   reviewID,
        Reviewers:  make(map[string]ReviewerSummary),
        AllIssues:  make([]ReviewIssue, 0),
    }

    for _, iter := range iterations {
        agg.Reviewers[iter.ReviewerID] = ReviewerSummary{
            Decision: iter.Decision,
            Issues:   len(iter.Issues),
        }
        agg.AllIssues = append(agg.AllIssues, iter.Issues...)
    }

    // Compute consensus
    agg.ConsensusDecision = s.computeConsensus(agg)

    return agg, nil
}
```

---

## 5. Web UI Extensions (React)

The UI is built with React + TypeScript following the patterns in `web/frontend/`.
Review components integrate with the existing architecture (TanStack Query, Zustand, Tailwind).

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         UI View Architecture                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Data Sources:                                                          │
│  ┌────────────────────┐     ┌────────────────────────────────────────┐ │
│  │ Mail Thread        │     │ Review Tables                          │ │
│  │ (messages table)   │     │ (reviews, review_iterations,           │ │
│  │                    │     │  review_issues)                        │ │
│  └─────────┬──────────┘     └──────────────────┬─────────────────────┘ │
│            │                                    │                       │
│            ▼                                    ▼                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                     React Components                             │   │
│  ├──────────────────┬──────────────────┬───────────────────────────┤   │
│  │ ReviewThread     │ ReviewDashboard  │ IssueTracker              │   │
│  │                  │                  │                           │   │
│  │ • Conversation   │ • State timeline │ • All issues by review    │   │
│  │ • Author/reviewer│ • Iteration diffs│ • Filter: open/fixed      │   │
│  │   back & forth   │ • Reviewer votes │ • Group by file/severity  │   │
│  │ • Inline replies │ • Consensus view │ • Resolution time stats   │   │
│  │                  │ • Cost/duration  │ • Link to code location   │   │
│  └──────────────────┴──────────────────┴───────────────────────────┘   │
│                                                                         │
│  Additional Views:                                                      │
│  ┌──────────────────┬──────────────────┬───────────────────────────┐   │
│  │ DiffViewer       │ ReviewerStatus   │ ReviewHistory             │   │
│  │ (@pierre/diffs)  │                  │                           │   │
│  │ • Side-by-side   │ • Active reviewers│ • Past reviews by repo   │   │
│  │ • Issues inline  │ • Queue depth    │ • Approval rate           │   │
│  │ • Suggestions    │ • Avg turnaround │ • Issue trends            │   │
│  └──────────────────┴──────────────────┴───────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.1 Project Structure

```
web/frontend/src/
├── api/
│   └── reviews.ts              # Review API client
├── components/
│   └── reviews/
│       ├── ReviewThread.tsx    # Main review conversation view
│       ├── ReviewMessage.tsx   # Single review message
│       ├── ReviewHeader.tsx    # Review status header
│       ├── ReviewDashboard.tsx # State timeline + metrics
│       ├── ReviewerStatus.tsx  # Multi-reviewer progress
│       ├── IssueCard.tsx       # Issue display with severity
│       ├── IssueTracker.tsx    # Issue list with filters
│       ├── DiffViewer.tsx      # @pierre/diffs integration
│       ├── DecisionBadge.tsx   # Approve/Request Changes badge
│       └── index.ts
├── hooks/
│   ├── useReviews.ts           # Review queries
│   ├── useReviewActions.ts     # Mutations (resubmit, resolve)
│   └── useReviewsRealtime.ts   # WebSocket updates
├── pages/
│   └── ReviewsPage.tsx         # /reviews route
└── types/
    └── reviews.ts              # Review TypeScript types
```

### 5.2 TypeScript Types

**Location**: `web/frontend/src/types/reviews.ts`

```typescript
export type ReviewState =
  | 'new'
  | 'pending_review'
  | 'under_review'
  | 'changes_requested'
  | 're_review'
  | 'approved'
  | 'rejected'
  | 'cancelled';

export type ReviewDecision = 'approve' | 'request_changes' | 'comment';

export type IssueSeverity = 'critical' | 'high' | 'medium' | 'low';

export type IssueType = 'bug' | 'security' | 'claude_md_violation' | 'logic_error';

export type IssueStatus = 'open' | 'fixed' | 'wont_fix' | 'duplicate';

export interface Review {
  id: number;
  reviewId: string;
  threadId: string;
  requesterId: number;
  requesterName: string;

  // PR info
  prNumber?: number;
  branch: string;
  baseBranch: string;
  commitSha: string;
  repoPath: string;

  // Config
  reviewType: 'full' | 'incremental' | 'security' | 'performance';
  priority: 'urgent' | 'normal' | 'low';
  state: ReviewState;

  // Timestamps
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface ReviewIteration {
  id: number;
  reviewId: string;
  iterationNum: number;
  reviewerId: string;
  reviewerSessionId?: string;

  decision: ReviewDecision;
  summary: string;
  issues: ReviewIssue[];
  suggestions: Suggestion[];

  // Metrics
  filesReviewed: number;
  linesAnalyzed: number;
  durationMs: number;
  costUsd: number;

  startedAt: string;
  completedAt?: string;
}

export interface ReviewIssue {
  id: number;
  reviewId: string;
  iterationNum: number;

  type: IssueType;
  severity: IssueSeverity;

  filePath: string;
  lineStart: number;
  lineEnd?: number;

  title: string;
  description: string;
  codeSnippet?: string;
  suggestion?: string;
  claudeMdRef?: string;

  status: IssueStatus;
  resolvedAt?: string;
  resolvedInIteration?: number;
}

export interface Suggestion {
  title: string;
  description: string;
  filePath?: string;
}

export interface ReviewerStatus {
  reviewerId: string;
  reviewerName: string;
  status: 'pending' | 'reviewing' | 'completed';
  decision?: ReviewDecision;
  issueCount: number;
}
```

### 5.3 API Client

**Location**: `web/frontend/src/api/reviews.ts`

```typescript
import { apiClient } from './client';
import type {
  Review,
  ReviewIteration,
  ReviewIssue,
  ReviewState,
  IssueStatus,
} from '@/types/reviews';

export interface ListReviewsParams {
  filter?: ReviewState | 'all';
  requesterId?: number;
  limit?: number;
  offset?: number;
}

export interface CreateReviewParams {
  branch: string;
  baseBranch?: string;
  commitSha: string;
  repoPath: string;
  prNumber?: number;
  reviewType?: 'full' | 'incremental' | 'security' | 'performance';
  priority?: 'urgent' | 'normal' | 'low';
  reviewers?: string[];
  description?: string;
}

export const reviewsApi = {
  // List reviews with filters
  list: async (params?: ListReviewsParams): Promise<Review[]> => {
    const searchParams = new URLSearchParams();
    if (params?.filter && params.filter !== 'all') {
      searchParams.set('filter', params.filter);
    }
    if (params?.requesterId) {
      searchParams.set('requester_id', params.requesterId.toString());
    }
    if (params?.limit) {
      searchParams.set('limit', params.limit.toString());
    }
    if (params?.offset) {
      searchParams.set('offset', params.offset.toString());
    }
    return apiClient.get(`/api/v1/reviews?${searchParams}`);
  },

  // Get single review with iterations
  get: async (reviewId: string): Promise<Review & { iterations: ReviewIteration[] }> => {
    return apiClient.get(`/api/v1/reviews/${reviewId}`);
  },

  // Create new review request
  create: async (params: CreateReviewParams): Promise<Review> => {
    return apiClient.post('/api/v1/reviews', params);
  },

  // Re-request review after changes
  resubmit: async (reviewId: string, commitSha: string): Promise<Review> => {
    return apiClient.post(`/api/v1/reviews/${reviewId}/resubmit`, { commitSha });
  },

  // Cancel a review
  cancel: async (reviewId: string): Promise<void> => {
    return apiClient.delete(`/api/v1/reviews/${reviewId}`);
  },

  // Get all issues for a review
  getIssues: async (reviewId: string): Promise<ReviewIssue[]> => {
    return apiClient.get(`/api/v1/reviews/${reviewId}/issues`);
  },

  // Update issue status
  updateIssueStatus: async (
    reviewId: string,
    issueId: number,
    status: IssueStatus
  ): Promise<ReviewIssue> => {
    return apiClient.patch(`/api/v1/reviews/${reviewId}/issues/${issueId}`, { status });
  },

  // Get diff for a file in the review
  getDiff: async (reviewId: string, filePath: string): Promise<{
    oldContent: string;
    newContent: string;
    issues: ReviewIssue[];
  }> => {
    return apiClient.get(`/api/v1/reviews/${reviewId}/diff?file=${encodeURIComponent(filePath)}`);
  },
};
```

### 5.4 React Hooks

**Location**: `web/frontend/src/hooks/useReviews.ts`

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { reviewsApi, ListReviewsParams, CreateReviewParams } from '@/api/reviews';
import type { IssueStatus } from '@/types/reviews';

export const reviewKeys = {
  all: ['reviews'] as const,
  lists: () => [...reviewKeys.all, 'list'] as const,
  list: (params?: ListReviewsParams) => [...reviewKeys.lists(), params] as const,
  details: () => [...reviewKeys.all, 'detail'] as const,
  detail: (id: string) => [...reviewKeys.details(), id] as const,
  issues: (id: string) => [...reviewKeys.detail(id), 'issues'] as const,
};

export function useReviews(params?: ListReviewsParams) {
  return useQuery({
    queryKey: reviewKeys.list(params),
    queryFn: () => reviewsApi.list(params),
  });
}

export function useReview(reviewId: string) {
  return useQuery({
    queryKey: reviewKeys.detail(reviewId),
    queryFn: () => reviewsApi.get(reviewId),
    enabled: !!reviewId,
  });
}

export function useReviewIssues(reviewId: string) {
  return useQuery({
    queryKey: reviewKeys.issues(reviewId),
    queryFn: () => reviewsApi.getIssues(reviewId),
    enabled: !!reviewId,
  });
}

export function useCreateReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: CreateReviewParams) => reviewsApi.create(params),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    },
  });
}

export function useResubmitReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ reviewId, commitSha }: { reviewId: string; commitSha: string }) =>
      reviewsApi.resubmit(reviewId, commitSha),
    onSuccess: (_, { reviewId }) => {
      queryClient.invalidateQueries({ queryKey: reviewKeys.detail(reviewId) });
      queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    },
  });
}

export function useUpdateIssueStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      reviewId,
      issueId,
      status,
    }: {
      reviewId: string;
      issueId: number;
      status: IssueStatus;
    }) => reviewsApi.updateIssueStatus(reviewId, issueId, status),
    onSuccess: (_, { reviewId }) => {
      queryClient.invalidateQueries({ queryKey: reviewKeys.issues(reviewId) });
    },
  });
}
```

**Location**: `web/frontend/src/hooks/useReviewsRealtime.ts`

```typescript
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWebSocket } from './useWebSocket';
import { reviewKeys } from './useReviews';

export function useReviewsRealtime(reviewId?: string) {
  const queryClient = useQueryClient();
  const { lastMessage } = useWebSocket();

  useEffect(() => {
    if (!lastMessage) return;

    const { type, payload } = lastMessage;

    if (type === 'review_updated') {
      // Invalidate specific review
      if (payload.reviewId) {
        queryClient.invalidateQueries({
          queryKey: reviewKeys.detail(payload.reviewId),
        });
      }
      // Always invalidate lists
      queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    }

    if (type === 'review_iteration_added' && payload.reviewId === reviewId) {
      queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(payload.reviewId),
      });
    }

    if (type === 'issue_resolved' && payload.reviewId === reviewId) {
      queryClient.invalidateQueries({
        queryKey: reviewKeys.issues(payload.reviewId),
      });
    }
  }, [lastMessage, reviewId, queryClient]);
}
```

### 5.5 Review Thread Component

**Location**: `web/frontend/src/components/reviews/ReviewThread.tsx`

```tsx
import { useState } from 'react';
import { useReview, useResubmitReview } from '@/hooks/useReviews';
import { useReviewsRealtime } from '@/hooks/useReviewsRealtime';
import { ReviewHeader } from './ReviewHeader';
import { ReviewMessage } from './ReviewMessage';
import { ReviewerStatus } from './ReviewerStatus';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import type { ReviewIteration } from '@/types/reviews';

interface ReviewThreadProps {
  reviewId: string;
}

export function ReviewThread({ reviewId }: ReviewThreadProps) {
  const { data: review, isLoading, error } = useReview(reviewId);
  const resubmit = useResubmitReview();
  const [replyText, setReplyText] = useState('');

  // Subscribe to real-time updates
  useReviewsRealtime(reviewId);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error || !review) {
    return (
      <div className="p-4 bg-red-50 text-red-700 rounded-lg">
        Failed to load review
      </div>
    );
  }

  const handleResubmit = async () => {
    // Get current commit SHA via backend API (frontend can't call git directly).
    const { commitSha } = await reviewsApi.getCurrentCommitSha(review.repoPath);
    resubmit.mutate({ reviewId, commitSha });
  };

  return (
    <div className="review-thread border rounded-lg overflow-hidden">
      {/* Header with status */}
      <ReviewHeader review={review} />

      {/* Multi-reviewer status (if applicable) */}
      {review.iterations.length > 0 && (
        <ReviewerStatus iterations={review.iterations} />
      )}

      {/* Conversation messages */}
      <div className="divide-y divide-gray-100">
        {review.iterations.map((iteration) => (
          <ReviewMessage key={iteration.id} iteration={iteration} />
        ))}
      </div>

      {/* Action bar */}
      <div className="bg-gray-50 p-4 border-t">
        {review.state === 'changes_requested' && (
          <div className="flex items-center justify-between">
            <p className="text-sm text-gray-600">
              {review.iterations.flatMap((i) => i.issues).filter((i) => i.status === 'open').length}{' '}
              issue(s) need to be addressed
            </p>
            <Button
              onClick={handleResubmit}
              loading={resubmit.isPending}
              className="bg-purple-600 hover:bg-purple-700"
            >
              Re-request Review
            </Button>
          </div>
        )}

        {review.state === 'approved' && (
          <div className="flex items-center space-x-2 text-green-700">
            <CheckCircleIcon className="w-5 h-5" />
            <span className="font-medium">Review approved - ready to merge</span>
          </div>
        )}

        {/* Reply input for discussion */}
        {['pending_review', 'under_review', 'changes_requested'].includes(review.state) && (
          <div className="mt-4">
            <textarea
              value={replyText}
              onChange={(e) => setReplyText(e.target.value)}
              placeholder="Add a comment or clarification..."
              className="w-full p-3 border rounded-lg resize-none focus:ring-2 focus:ring-purple-500"
              rows={3}
            />
            <div className="mt-2 flex justify-end">
              <Button
                disabled={!replyText.trim()}
                onClick={() => {/* Send reply via mail */}}
              >
                Send Reply
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function CheckCircleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 20 20">
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
        clipRule="evenodd"
      />
    </svg>
  );
}
```

### 5.6 Review Message Component

**Location**: `web/frontend/src/components/reviews/ReviewMessage.tsx`

```tsx
import { formatDistanceToNow } from 'date-fns';
import { DecisionBadge } from './DecisionBadge';
import { IssueCard } from './IssueCard';
import { Avatar } from '@/components/ui/Avatar';
import type { ReviewIteration } from '@/types/reviews';

interface ReviewMessageProps {
  iteration: ReviewIteration;
}

export function ReviewMessage({ iteration }: ReviewMessageProps) {
  const isReviewer = true; // Iterations are always from reviewers

  return (
    <div className={`p-4 ${isReviewer ? 'bg-purple-50' : 'bg-white'}`}>
      <div className="flex items-start space-x-3">
        {/* Avatar */}
        <Avatar
          name={iteration.reviewerId}
          className={isReviewer ? 'bg-purple-600' : 'bg-blue-600'}
        />

        <div className="flex-1">
          {/* Header */}
          <div className="flex items-center space-x-2 mb-1">
            <span className="font-medium text-gray-900">{iteration.reviewerId}</span>
            {isReviewer && (
              <span className="px-2 py-0.5 text-xs rounded-full bg-purple-100 text-purple-700">
                Reviewer
              </span>
            )}
            <span className="text-sm text-gray-500">
              {formatDistanceToNow(new Date(iteration.completedAt || iteration.startedAt), {
                addSuffix: true,
              })}
            </span>
          </div>

          {/* Summary */}
          <div className="prose prose-sm max-w-none">
            <p>{iteration.summary}</p>
          </div>

          {/* Decision badge */}
          <div className="mt-3">
            <DecisionBadge decision={iteration.decision} />
          </div>

          {/* Issues list */}
          {iteration.issues.length > 0 && (
            <div className="mt-4 space-y-3">
              {iteration.issues.map((issue) => (
                <IssueCard key={issue.id} issue={issue} />
              ))}
            </div>
          )}

          {/* Suggestions (non-blocking) */}
          {iteration.suggestions.length > 0 && (
            <div className="mt-4 p-3 bg-blue-50 rounded-lg">
              <h4 className="text-sm font-medium text-blue-900 mb-2">
                Non-blocking suggestions
              </h4>
              <ul className="text-sm text-blue-800 space-y-1">
                {iteration.suggestions.map((s, i) => (
                  <li key={i}>• {s.title}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
```

### 5.7 Issue Card Component

**Location**: `web/frontend/src/components/reviews/IssueCard.tsx`

```tsx
import { useState } from 'react';
import { useUpdateIssueStatus } from '@/hooks/useReviews';
import type { ReviewIssue, IssueStatus } from '@/types/reviews';

interface IssueCardProps {
  issue: ReviewIssue;
}

const severityStyles = {
  critical: 'border-red-300 bg-red-50',
  high: 'border-orange-300 bg-orange-50',
  medium: 'border-yellow-300 bg-yellow-50',
  low: 'border-blue-300 bg-blue-50',
};

const severityBadgeStyles = {
  critical: 'bg-red-100 text-red-800',
  high: 'bg-orange-100 text-orange-800',
  medium: 'bg-yellow-100 text-yellow-800',
  low: 'bg-blue-100 text-blue-800',
};

export function IssueCard({ issue }: IssueCardProps) {
  const [expanded, setExpanded] = useState(false);
  const updateStatus = useUpdateIssueStatus();

  const handleMarkFixed = () => {
    updateStatus.mutate({
      reviewId: issue.reviewId,
      issueId: issue.id,
      status: 'fixed',
    });
  };

  return (
    <div
      className={`border rounded-lg overflow-hidden ${severityStyles[issue.severity]}`}
    >
      {/* Header */}
      <div
        className="px-3 py-2 flex items-center justify-between cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center space-x-2">
          <span
            className={`px-2 py-0.5 text-xs font-medium rounded ${
              severityBadgeStyles[issue.severity]
            }`}
          >
            {issue.severity}
          </span>
          <span className="text-sm font-medium text-gray-900">{issue.title}</span>
          {issue.status === 'fixed' && (
            <span className="px-2 py-0.5 text-xs rounded bg-green-100 text-green-800">
              Fixed
            </span>
          )}
        </div>
        <a
          href={`#file-${encodeURIComponent(issue.filePath)}`}
          className="text-sm text-blue-600 hover:underline"
          onClick={(e) => e.stopPropagation()}
        >
          {issue.filePath}:{issue.lineStart}
        </a>
      </div>

      {/* Expanded content */}
      {expanded && (
        <div className="px-3 py-2 border-t bg-white">
          <p className="text-sm text-gray-700">{issue.description}</p>

          {issue.codeSnippet && (
            <pre className="mt-2 p-2 bg-gray-800 text-gray-100 rounded text-xs overflow-x-auto">
              <code>{issue.codeSnippet}</code>
            </pre>
          )}

          {issue.suggestion && (
            <div className="mt-2 p-2 bg-green-50 border border-green-200 rounded">
              <p className="text-sm text-green-800">
                <strong>Suggested fix:</strong> {issue.suggestion}
              </p>
            </div>
          )}

          {issue.claudeMdRef && (
            <p className="mt-2 text-xs text-gray-500">
              CLAUDE.md reference: {issue.claudeMdRef}
            </p>
          )}

          {/* Actions */}
          {issue.status === 'open' && (
            <div className="mt-3 flex space-x-2">
              <button
                onClick={handleMarkFixed}
                disabled={updateStatus.isPending}
                className="px-3 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700"
              >
                Mark as Fixed
              </button>
              <button
                onClick={() =>
                  updateStatus.mutate({
                    reviewId: issue.reviewId,
                    issueId: issue.id,
                    status: 'wont_fix',
                  })
                }
                className="px-3 py-1 text-xs bg-gray-200 text-gray-700 rounded hover:bg-gray-300"
              >
                Won't Fix
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

### 5.8 Diff Viewer Component (Deferred)

> **Status**: Deferred to a later phase. Focus on the underlying system first.
> The diff rendering library needs to be selected during implementation.
> Candidates: `react-diff-viewer-continued`, `diff2html`, or a custom implementation.
> The API shown below is illustrative and will be finalized when this phase begins.

The UI will use a React diff component for professional diff rendering.

**Installation (when implemented):**
```bash
cd web/frontend && bun add <diff-library-tbd>
```

**Location**: `web/frontend/src/components/reviews/DiffViewer.tsx`

```tsx
import { useState, useEffect } from 'react';
// NOTE: Diff library TBD. Example imports are illustrative.
// import { MultiFileDiff, FileDiff, PatchDiff, registerCustomTheme } from '<diff-library>';
import { useQuery } from '@tanstack/react-query';
import { reviewsApi } from '@/api/reviews';
import { IssueCard } from './IssueCard';
import type { ReviewIssue } from '@/types/reviews';

// Register custom theme on module load
registerCustomTheme('substrate-review', {
  extends: 'pierre-light',
  colors: {
    'review.critical': '#dc2626',
    'review.high': '#ea580c',
    'review.medium': '#ca8a04',
    'review.low': '#2563eb',
  },
  lineHighlight: {
    critical: 'rgba(220, 38, 38, 0.1)',
    high: 'rgba(234, 88, 12, 0.1)',
  },
});

interface DiffViewerProps {
  reviewId: string;
  filePath: string;
}

export function DiffViewer({ reviewId, filePath }: DiffViewerProps) {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');

  const { data, isLoading, error } = useQuery({
    queryKey: ['review-diff', reviewId, filePath],
    queryFn: () => reviewsApi.getDiff(reviewId, filePath),
  });

  if (isLoading) {
    return (
      <div className="animate-pulse bg-gray-100 h-64 rounded flex items-center justify-center">
        <span className="text-gray-500">Loading diff...</span>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-4 bg-red-50 text-red-700 rounded">
        Failed to load diff
      </div>
    );
  }

  return (
    <div className="diff-viewer border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="bg-gray-100 px-4 py-2 flex justify-between items-center border-b">
        <span className="font-mono text-sm">{filePath}</span>
        <div className="flex items-center gap-4">
          {data.issues.length > 0 && (
            <span className="px-2 py-0.5 bg-yellow-100 text-yellow-800 rounded-full text-xs">
              {data.issues.length} issue(s)
            </span>
          )}
          <button
            onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
            className="text-sm text-gray-600 hover:text-gray-900"
          >
            {theme === 'light' ? '🌙' : '☀️'}
          </button>
        </div>
      </div>

      {/* Diff content */}
      <div className="relative">
        <MultiFileDiff
          oldFile={{ name: filePath, contents: data.oldContent }}
          newFile={{ name: filePath, contents: data.newContent }}
          theme={theme === 'dark' ? 'pierre-dark' : 'substrate-review'}
        />

        {/* Inline issue annotations */}
        <IssueAnnotations issues={data.issues} />
      </div>
    </div>
  );
}

interface IssueAnnotationsProps {
  issues: ReviewIssue[];
}

function IssueAnnotations({ issues }: IssueAnnotationsProps) {
  const [positions, setPositions] = useState<Map<number, number>>(new Map());

  useEffect(() => {
    // Calculate positions after diff renders
    const timer = setTimeout(() => {
      const newPositions = new Map<number, number>();
      issues.forEach((issue) => {
        const lineEl = document.querySelector(`[data-line-number="${issue.lineStart}"]`);
        if (lineEl) {
          const rect = lineEl.getBoundingClientRect();
          const container = lineEl.closest('.diff-viewer');
          if (container) {
            const containerRect = container.getBoundingClientRect();
            newPositions.set(issue.id, rect.bottom - containerRect.top);
          }
        }
      });
      setPositions(newPositions);
    }, 100);

    return () => clearTimeout(timer);
  }, [issues]);

  return (
    <div className="issue-annotations">
      {issues.map((issue) => {
        const top = positions.get(issue.id);
        if (!top) return null;

        return (
          <div
            key={issue.id}
            className="absolute left-16 right-4 z-10"
            style={{ top: `${top}px` }}
          >
            <IssueCard issue={issue} compact />
          </div>
        );
      })}
    </div>
  );
}
```

### 5.9 Multi-File Diff Page

**Location**: `web/frontend/src/components/reviews/ReviewDiffPage.tsx`

```tsx
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
// NOTE: Diff library TBD. Example imports are illustrative.
// import { PatchDiff, parsePatchFiles } from '<diff-library>';
import { reviewsApi } from '@/api/reviews';
import { DiffViewer } from './DiffViewer';
import type { Review } from '@/types/reviews';

interface ReviewDiffPageProps {
  review: Review;
}

export function ReviewDiffPage({ review }: ReviewDiffPageProps) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null);

  // Fetch the full patch
  const { data: patch, isLoading } = useQuery({
    queryKey: ['review-patch', review.reviewId],
    queryFn: () => reviewsApi.getPatch(review.reviewId),
  });

  if (isLoading) {
    return <div className="animate-pulse">Loading files...</div>;
  }

  // Parse patch to get file list
  const files = patch ? parsePatchFiles(patch.content) : [];

  return (
    <div className="review-diff-page flex h-full">
      {/* File tree sidebar */}
      <div className="w-64 border-r bg-gray-50 overflow-y-auto">
        <div className="p-3 border-b bg-white">
          <h3 className="font-medium text-sm text-gray-700">
            Changed Files ({files.length})
          </h3>
        </div>
        <div className="p-2">
          {files.map((file) => (
            <button
              key={file.name}
              onClick={() => setSelectedFile(file.name)}
              className={`w-full text-left px-3 py-2 rounded text-sm truncate ${
                selectedFile === file.name
                  ? 'bg-purple-100 text-purple-900'
                  : 'hover:bg-gray-100 text-gray-700'
              }`}
            >
              <span className="font-mono">{file.name}</span>
              <span className="ml-2 text-xs text-gray-500">
                +{file.additions} -{file.deletions}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Diff viewer */}
      <div className="flex-1 overflow-y-auto p-4">
        {selectedFile ? (
          <DiffViewer reviewId={review.reviewId} filePath={selectedFile} />
        ) : (
          <div className="text-center text-gray-500 py-12">
            Select a file to view changes
          </div>
        )}
      </div>
    </div>
  );
}
```

### 5.10 Reviews Page

**Location**: `web/frontend/src/pages/ReviewsPage.tsx`

```tsx
import { useState } from 'react';
import { useReviews } from '@/hooks/useReviews';
import { useReviewsRealtime } from '@/hooks/useReviewsRealtime';
import { ReviewThread } from '@/components/reviews/ReviewThread';
import { Spinner } from '@/components/ui/Spinner';
import { Badge } from '@/components/ui/Badge';
import type { ReviewState } from '@/types/reviews';

type FilterType = ReviewState | 'all';

export function ReviewsPage() {
  const [filter, setFilter] = useState<FilterType>('all');
  const [selectedReviewId, setSelectedReviewId] = useState<string | null>(null);

  const { data: reviews, isLoading } = useReviews({
    filter: filter === 'all' ? undefined : filter,
  });

  useReviewsRealtime();

  const filterOptions: { value: FilterType; label: string }[] = [
    { value: 'all', label: 'All Reviews' },
    { value: 'pending_review', label: 'Pending' },
    { value: 'under_review', label: 'In Progress' },
    { value: 'changes_requested', label: 'Changes Requested' },
    { value: 'approved', label: 'Approved' },
  ];

  return (
    <div className="reviews-page h-full flex">
      {/* Reviews list */}
      <div className="w-96 border-r flex flex-col">
        <div className="p-4 border-b">
          <h1 className="text-xl font-bold text-gray-900 mb-3">Code Reviews</h1>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value as FilterType)}
            className="w-full rounded-md border-gray-300 text-sm"
          >
            {filterOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        <div className="flex-1 overflow-y-auto">
          {isLoading ? (
            <div className="flex justify-center py-8">
              <Spinner />
            </div>
          ) : reviews?.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No reviews found
            </div>
          ) : (
            <div className="divide-y">
              {reviews?.map((review) => (
                <button
                  key={review.reviewId}
                  onClick={() => setSelectedReviewId(review.reviewId)}
                  className={`w-full text-left p-4 hover:bg-gray-50 ${
                    selectedReviewId === review.reviewId ? 'bg-purple-50' : ''
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="font-medium text-gray-900 truncate">
                      {review.branch}
                    </span>
                    <ReviewStateBadge state={review.state} />
                  </div>
                  <p className="text-sm text-gray-500">
                    by {review.requesterName}
                  </p>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Review detail */}
      <div className="flex-1 overflow-y-auto p-4">
        {selectedReviewId ? (
          <ReviewThread reviewId={selectedReviewId} />
        ) : (
          <div className="flex items-center justify-center h-full text-gray-500">
            Select a review to view details
          </div>
        )}
      </div>
    </div>
  );
}

function ReviewStateBadge({ state }: { state: ReviewState }) {
  const variants: Record<ReviewState, { color: string; label: string }> = {
    new: { color: 'gray', label: 'New' },
    pending_review: { color: 'yellow', label: 'Pending' },
    under_review: { color: 'purple', label: 'Reviewing' },
    changes_requested: { color: 'red', label: 'Changes' },
    re_review: { color: 'yellow', label: 'Re-review' },
    approved: { color: 'green', label: 'Approved' },
    rejected: { color: 'red', label: 'Rejected' },
    cancelled: { color: 'gray', label: 'Cancelled' },
  };

  const { color, label } = variants[state] || { color: 'gray', label: state };

  return <Badge color={color}>{label}</Badge>;
}
```

### 5.11 Router Integration

**Location**: `web/frontend/src/router.tsx` (add to existing)

```tsx
import { ReviewsPage } from '@/pages/ReviewsPage';

// Add to routes array:
{
  path: '/reviews',
  element: <ReviewsPage />,
},
{
  path: '/reviews/:reviewId',
  element: <ReviewsPage />,
},
```

---

## 6. CLI Commands

**Location**: `cmd/substrate/commands/review.go`

```go
package commands

import (
    "github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
    Use:   "review",
    Short: "Code review operations",
    Long:  "Request, manage, and respond to code reviews",
}

var reviewRequestCmd = &cobra.Command{
    Use:   "request",
    Short: "Request a code review",
    Long: `Request a code review for the current branch.

Examples:
  # Request review for current branch
  substrate review request

  # Request review with specific reviewers
  substrate review request --reviewers security,performance

  # Request review for specific commit
  substrate review request --commit abc123

  # Request incremental review (only new changes)
  substrate review request --incremental
`,
    RunE: runReviewRequest,
}

var reviewStatusCmd = &cobra.Command{
    Use:   "status [review-id]",
    Short: "Check review status",
    RunE:  runReviewStatus,
}

var reviewListCmd = &cobra.Command{
    Use:   "list",
    Short: "List pending reviews",
    RunE:  runReviewList,
}

var reviewRespondCmd = &cobra.Command{
    Use:   "respond [review-id]",
    Short: "Respond to a review (for reviewer agents)",
    RunE:  runReviewRespond,
}

func init() {
    reviewRequestCmd.Flags().StringSlice("reviewers", nil, "Specific reviewers to request")
    reviewRequestCmd.Flags().String("commit", "", "Specific commit SHA to review")
    reviewRequestCmd.Flags().String("base", "main", "Base branch to compare against")
    reviewRequestCmd.Flags().Bool("incremental", false, "Only review new changes since last review")
    reviewRequestCmd.Flags().String("priority", "normal", "Review priority (urgent, normal, low)")

    reviewListCmd.Flags().String("filter", "all", "Filter reviews (pending, approved, etc.)")
    reviewListCmd.Flags().Bool("mine", false, "Only show my review requests")

    reviewCmd.AddCommand(reviewRequestCmd, reviewStatusCmd, reviewListCmd, reviewRespondCmd)
    rootCmd.AddCommand(reviewCmd)
}

func runReviewRequest(cmd *cobra.Command, args []string) error {
    // 1. Get current git context (branch, commit, repo path)
    // 2. Build ReviewRequest
    // 3. Send to ReviewerService
    // 4. Display review ID and status
}
```

---

## 7. Hook Extensions

### 7.1 Reviewer Hook Configuration

**Location**: `internal/hooks/reviewer_hooks.go`

```go
// ReviewerHooks configures hooks for reviewer agents.
type ReviewerHooks struct {
    // SessionStart: Load pending reviews to process
    SessionStart HookConfig

    // Stop: Check for more reviews before exiting
    Stop HookConfig
}

// InstallReviewerHooks sets up hooks for a reviewer agent.
func InstallReviewerHooks(configDir string, reviewerType string) error {
    hooks := ReviewerHooks{
        SessionStart: HookConfig{
            Script: fmt.Sprintf(`#!/bin/bash
# Reviewer session start hook
substrate review list --filter pending --reviewer %s --format context
`, reviewerType),
        },
        Stop: HookConfig{
            Script: `#!/bin/bash
# Check for pending reviews before allowing exit
result=$(substrate review list --filter pending --format hook)
if [ "$(echo "$result" | jq -r '.has_pending')" = "true" ]; then
    echo "$result"
    exit 1  # Block exit
fi
exit 0  # Allow exit
`,
        },
    }

    return writeHookScripts(configDir, hooks)
}
```

### 7.2 Author Agent Hook Updates

Update existing hooks to check for review responses:

```go
// In stop.sh, add review check:
`
# Check for review responses
reviews=$(substrate review status --mine --format json)
if [ "$(echo "$reviews" | jq -r '.has_responses')" = "true" ]; then
    echo "You have review feedback to address:"
    echo "$reviews" | jq -r '.reviews[] | "- \(.reviewer): \(.decision) - \(.summary)"'
    # Return block decision
    echo '{"decision": "block", "reason": "Review feedback received"}'
    exit 0
fi
`
```

---

## 8. CLAUDE.md Extensions

### 8.1 Project CLAUDE.md Updates

Add to `/home/user/substrate/CLAUDE.md`:

```markdown
## Code Review System

Substrate includes a native code review system for autonomous PR review.

### Requesting Reviews

When you complete a PR and need review:
1. Commit and push your changes
2. Run `substrate review request` to submit for review
3. Wait for review feedback (delivered as mail messages)
4. Address any issues and re-request review
5. Proceed when approved

### Review Commands
- `substrate review request` - Request review for current branch
- `substrate review status` - Check review status
- `substrate review list` - List pending reviews

### As a Reviewer

If you're operating as a specialized reviewer:
1. Reviews are delivered as priority messages
2. Use the review tools to analyze the PR
3. Respond with structured feedback
4. Focus on high-signal issues only (see review guidelines)

### Multi-Reviewer Setup

For thorough reviews, multiple specialized reviewers analyze PRs:
- **CodeReviewer**: General bugs and logic errors
- **SecurityReviewer**: Security vulnerabilities
- **PerformanceReviewer**: Performance issues
- **ArchitectureReviewer**: Design and structure

Reviews aggregate and require consensus before approval.
```

### 8.2 Reviewer Agent CLAUDE.md

Create `~/.subtrate/reviewers/CLAUDE.md`:

```markdown
# Reviewer Agent Instructions

You are a specialized code reviewer operating within Substrate.

## Your Role

Review pull requests submitted by other agents. Focus on:
- Bugs that would cause runtime errors
- Security vulnerabilities
- CLAUDE.md compliance violations
- Logic errors that produce wrong results

## What NOT to Flag

- Style preferences
- Potential issues that depend on inputs
- Pre-existing issues
- Anything linters would catch

## Review Process

1. Receive review request via mail
2. Analyze the changes using available tools
3. Structure findings as issues with:
   - File and line numbers
   - Clear description
   - Severity rating
   - Fix suggestion (if straightforward)
4. Send review response
5. If approved, state clearly
6. If changes needed, list specific issues

## Iteration

When re-reviewing after changes:
- Only check previously flagged issues
- Acknowledge fixed issues
- Don't introduce new feedback
- Approve when all issues resolved

## Output Format

Use the structured review format in all responses.
```

---

## 9. Reviewer Agent Management

### 9.1 Starting Reviewer Agents

Reviewer agents are auto-spawned by ReviewerService using the Claude Agent Go SDK.
Each active review gets a dedicated reviewer agent.

**Auto-Spawn (Primary Path):**
```go
import claudeagent "github.com/roasbeef/claude-agent-sdk-go"

// ReviewerService auto-spawns when a review request arrives.
func (s *Service) spawnReviewer(ctx context.Context, req ReviewRequest, cfg *ReviewerConfig) error {
    // Create client with reviewer persona configuration.
    client, err := claudeagent.NewClient(
        claudeagent.WithSystemPrompt(cfg.SystemPrompt),
        claudeagent.WithModel(cfg.Model),
        claudeagent.WithCwd(req.RepoPath),
        claudeagent.WithPermissionMode(claudeagent.PermissionModeDefault),
        claudeagent.WithMaxTurns(50),
        claudeagent.WithEnv(map[string]string{
            "REVIEW_ID":   req.ReviewID,
            "THREAD_ID":   req.ThreadID,
            "BRANCH":      req.Branch,
        }),
    )
    if err != nil {
        return fmt.Errorf("create reviewer client: %w", err)
    }

    if err := client.Connect(ctx); err != nil {
        client.Close()
        return err
    }

    // Open bidirectional stream for conversational review.
    stream, err := client.Stream(ctx)
    if err != nil {
        client.Close()
        return err
    }

    // Run review loop in background goroutine.
    go s.runReviewerLoop(ctx, stream, client, req.ReviewID, initialPrompt)

    return nil
}
```

**CLI Management:**
```bash
# List active reviewers
substrate reviewer list
# Output:
# NAME                          TYPE        STATUS   REVIEW_ID    BRANCH
# SecurityReviewer-review-abc   security    active   abc123       feature-auth
# CodeReviewer-review-def       code        active   def456       fix-bug-42

# Stop a reviewer
substrate reviewer stop SecurityReviewer-review-abc
```

**Restart Recovery:**
```go
// On server restart, re-spawn agents for active reviews.
func (s *Service) RecoverActiveReviews(ctx context.Context) error {
    // Query for reviews in active states.
    reviews, err := s.store.ListReviewsByState(ctx,
        StateUnderReview, StateChangesRequested, StateReReview,
    )
    if err != nil {
        return err
    }

    for _, review := range reviews {
        cfg := s.reviewers[review.ReviewType]
        if err := s.spawnReviewer(ctx, review.ToRequest(), cfg); err != nil {
            log.Printf("Failed to recover reviewer for %s: %v",
                review.ReviewID, err)
        }
    }
    return nil
}
```

### 9.2 Reviewer Agent Hooks

Reviewer agents use hooks configured via the Claude Agent SDK:

```go
// Hooks are passed via WithHooks option when creating the client.
claudeagent.WithHooks(map[claudeagent.HookType][]claudeagent.HookConfig{
    claudeagent.HookTypeSessionStart: {{
        Command: "substrate review context --review-id $REVIEW_ID --format context",
    }},
})
```

The reviewer agent uses Substrate's mail system (via installed hooks) for heartbeats
and message delivery. No separate reviewer-specific hooks are needed since the agent
communicates through normal mail threads.

### 9.3 Reviewer Registration

Reviewer agents are tracked in the review_iterations table. When spawned:

```go
// ReviewerService tracks spawned reviewers for lifecycle management.
type ActiveReviewer struct {
    ReviewID    string
    ReviewerName string          // "SecurityReviewer-review-abc123"
    AgentID     int64            // Registered agent in Substrate
    Stream      *claudeagent.Stream
    Client      *claudeagent.Client
    StartedAt   time.Time
    Cancel      context.CancelFunc
}

// Stored in memory (activeReviews map) and referenced via DB.
// On restart, DB state is used to re-spawn agents.
```

---

## 10. Dependencies

### 10.1 JavaScript/TypeScript

```bash
# Diff rendering (TBD: @pierre/diffs, react-diff-viewer, or diff2html)
# bun add <diff-library>

# Already in project:
# - react, react-dom
# - @tanstack/react-query
# - tailwindcss
```

### 10.2 Go

```go
// go.mod additions
require (
    github.com/roasbeef/claude-agent-sdk-go  // Already present
    // No new Go deps needed
)
```

### 10.3 System Requirements

- **bun** - For running the diff render script
- **git** - For diff generation and branch checkout
- **gh** (optional) - For GitHub PR integration

---

## 11. Implementation Phases

### Phase 1: Foundation

1. **Database Schema**
   - Create migration 000003_reviews (reviews, review_iterations, review_issues tables)
   - Add metadata column to messages table
   - Add sqlc queries for review CRUD
   - Implement ReviewStore in SqlcStore, MockStore, txSqlcStore

2. **Review Service Actor**
   - ReviewServiceKey and actor registration
   - ReviewMessage sealed interface (ReviewRequest, ReviewIterationRequest, ReviewApprovalCheck)
   - Basic FSM (ReviewState, ReviewEvent, ProcessEvent)
   - Wire into actor system in main.go

3. **gRPC Proto + API**
   - Add ReviewService to mail.proto
   - `make proto` to generate code
   - Register with gateway mux in server.go
   - Implement RPC handlers

4. **CLI Commands**
   - `substrate review request` (sends to reviews topic)
   - `substrate review status`
   - `substrate review list`

### Phase 2: Reviewer Agent Integration

1. **Claude Agent SDK Integration**
   - Per-review agent spawning via `claudeagent.NewClient` + `Stream`
   - System prompt construction with reviewer persona
   - Bidirectional stream management (runReviewerLoop)
   - Agent lifecycle tracking (ActiveReviewer map)

2. **YAML Frontmatter Protocol**
   - Thread watcher for parsing reviewer messages
   - FSM state updates from frontmatter
   - review_iterations and review_issues record creation

3. **Activity Integration**
   - Activity entries for review events
   - ActivityStore calls from ReviewerService

4. **Restart Recovery**
   - Query DB for active reviews on startup
   - Re-spawn reviewer agents with thread context

### Phase 3: Web UI

1. **Review Thread Component**
   - ReviewThread, ReviewMessage, ReviewHeader components
   - Issue cards with severity badges
   - Decision badges (approve/request_changes)

2. **Reviews List Page**
   - ReviewsPage with filters (state, requester)
   - Review state badges
   - Link to thread view

3. **Real-time Updates**
   - WebSocket events (review_updated, review_iteration_added, issue_resolved)
   - TanStack Query hooks (useReviews, useReviewIssues, useReviewsRealtime)
   - Notification pipeline via existing NotificationHub

### Phase 4: Multi-Reviewer + Polish

1. **Multi-Reviewer Support**
   - Multiple reviewer personas per review
   - Consensus aggregation logic
   - Per-reviewer status tracking in UI

2. **Diff Viewer (deferred from earlier)**
   - Select and integrate diff rendering library
   - Inline issue annotations on diff
   - Multi-file diff page

3. **Testing**
   - Unit tests for ReviewerService actor
   - Integration tests for full review flow
   - MockStore implementation for reviews
   - E2E tests for web UI review pages

---

## 12. API Endpoints (gRPC-Gateway)

All review endpoints follow the existing gRPC-gateway pattern: proto definition →
`make proto` → register handler with gateway mux in server.go.

```
POST   /api/v1/reviews                    - Create review request
GET    /api/v1/reviews                    - List reviews (with filters)
GET    /api/v1/reviews/{review_id}        - Get review details with iterations
POST   /api/v1/reviews/{review_id}/resubmit - Re-request review after changes
DELETE /api/v1/reviews/{review_id}        - Cancel review

GET    /api/v1/reviews/{review_id}/issues - Get all issues for review
PATCH  /api/v1/reviews/{review_id}/issues/{issue_id} - Update issue status

# Real-time updates via existing WebSocket at /ws (not SSE)
# Review events: review_updated, review_iteration_added, issue_resolved
```

### 12.1 Proto Definition

**Location**: `internal/api/grpc/mail.proto` (extend existing file)

```protobuf
// ReviewService handles code review operations.
service ReviewService {
    // CreateReview creates a new review request.
    rpc CreateReview(CreateReviewRequest) returns (ReviewResponse) {
        option (google.api.http) = {
            post: "/api/v1/reviews"
            body: "*"
        };
    }

    // ListReviews lists reviews with optional filters.
    rpc ListReviews(ListReviewsRequest) returns (ListReviewsResponse) {
        option (google.api.http) = {
            get: "/api/v1/reviews"
        };
    }

    // GetReview gets a single review with its iterations.
    rpc GetReview(GetReviewRequest) returns (ReviewDetailResponse) {
        option (google.api.http) = {
            get: "/api/v1/reviews/{review_id}"
        };
    }

    // ResubmitReview re-requests review after changes.
    rpc ResubmitReview(ResubmitReviewRequest) returns (ReviewResponse) {
        option (google.api.http) = {
            post: "/api/v1/reviews/{review_id}/resubmit"
            body: "*"
        };
    }

    // CancelReview cancels an active review.
    rpc CancelReview(CancelReviewRequest) returns (CancelReviewResponse) {
        option (google.api.http) = {
            delete: "/api/v1/reviews/{review_id}"
        };
    }

    // ListReviewIssues lists issues for a review.
    rpc ListReviewIssues(ListReviewIssuesRequest) returns (ListReviewIssuesResponse) {
        option (google.api.http) = {
            get: "/api/v1/reviews/{review_id}/issues"
        };
    }

    // UpdateIssueStatus updates the status of a review issue.
    rpc UpdateIssueStatus(UpdateIssueStatusRequest) returns (ReviewIssueResponse) {
        option (google.api.http) = {
            patch: "/api/v1/reviews/{review_id}/issues/{issue_id}"
            body: "*"
        };
    }
}

message CreateReviewRequest {
    string branch = 1;
    string base_branch = 2;
    string commit_sha = 3;
    string repo_path = 4;
    int32 pr_number = 5;
    string review_type = 6;   // full, incremental, security, performance
    string priority = 7;      // urgent, normal, low
    repeated string reviewers = 8;
    string description = 9;
    int64 requester_id = 10;
}

message ReviewResponse {
    string review_id = 1;
    string thread_id = 2;
    string state = 3;
}

message ListReviewsRequest {
    string filter = 1;        // State filter or "all"
    int64 requester_id = 2;
    int32 limit = 3;
    int32 offset = 4;
}

message ListReviewsResponse {
    repeated ReviewSummary reviews = 1;
}

message ReviewSummary {
    string review_id = 1;
    string thread_id = 2;
    int64 requester_id = 3;
    string requester_name = 4;
    string branch = 5;
    string state = 6;
    string review_type = 7;
    int64 created_at = 8;
    int64 updated_at = 9;
}

message GetReviewRequest {
    string review_id = 1;
}

message ReviewDetailResponse {
    ReviewSummary review = 1;
    repeated ReviewIterationProto iterations = 2;
}

message ReviewIterationProto {
    int32 iteration_num = 1;
    string reviewer_id = 2;
    string decision = 3;
    string summary = 4;
    int32 issue_count = 5;
    int64 started_at = 6;
    int64 completed_at = 7;
}

message ResubmitReviewRequest {
    string review_id = 1;
    string commit_sha = 2;
}

message CancelReviewRequest {
    string review_id = 1;
}

message CancelReviewResponse {}

message ListReviewIssuesRequest {
    string review_id = 1;
}

message ListReviewIssuesResponse {
    repeated ReviewIssueProto issues = 1;
}

message ReviewIssueProto {
    int64 id = 1;
    string review_id = 2;
    int32 iteration_num = 3;
    string issue_type = 4;
    string severity = 5;
    string file_path = 6;
    int32 line_start = 7;
    int32 line_end = 8;
    string title = 9;
    string description = 10;
    string code_snippet = 11;
    string suggestion = 12;
    string claude_md_ref = 13;
    string status = 14;
}

message UpdateIssueStatusRequest {
    string review_id = 1;
    int64 issue_id = 2;
    string status = 3;       // open, fixed, wont_fix, duplicate
}

message ReviewIssueResponse {
    ReviewIssueProto issue = 1;
}
```

---

## 13. Store Interface for Reviews

**Location**: `internal/store/interfaces.go` (extend existing)

Following the existing 3-tier pattern: domain interface → SqlcStore → MockStore → txSqlcStore.

```go
// ReviewStore provides review CRUD operations.
type ReviewStore interface {
    // CreateReview creates a new review record.
    CreateReview(ctx context.Context, params CreateReviewParams) (Review, error)

    // GetReview retrieves a review by its UUID.
    GetReview(ctx context.Context, reviewID string) (Review, error)

    // ListReviews lists reviews with optional state filter.
    ListReviews(ctx context.Context, filter string, limit, offset int) ([]Review, error)

    // ListReviewsByState lists reviews matching any of the given states.
    ListReviewsByState(ctx context.Context, states ...ReviewState) ([]Review, error)

    // ListReviewsByRequester lists reviews by the requesting agent.
    ListReviewsByRequester(ctx context.Context, requesterID int64, limit int) ([]Review, error)

    // UpdateReviewState updates the FSM state of a review.
    UpdateReviewState(ctx context.Context, reviewID string, state ReviewState) error

    // CreateReviewIteration records a review iteration result.
    CreateReviewIteration(ctx context.Context, params CreateReviewIterationParams) error

    // GetReviewIterations gets all iterations for a review.
    GetReviewIterations(ctx context.Context, reviewID string) ([]ReviewIteration, error)

    // CreateReviewIssue records a specific issue found during review.
    CreateReviewIssue(ctx context.Context, params CreateReviewIssueParams) error

    // GetReviewIssues gets all issues for a review.
    GetReviewIssues(ctx context.Context, reviewID string) ([]ReviewIssue, error)

    // UpdateReviewIssueStatus updates an issue's resolution status.
    UpdateReviewIssueStatus(ctx context.Context, issueID int64, status string) error
}
```

The Storage interface will embed ReviewStore alongside the existing sub-interfaces.

---

## 14. YAML Frontmatter Protocol

Reviewer agents include structured YAML frontmatter in their messages so the
ReviewerService can update the FSM state automatically by monitoring the thread.

**Format:**
```
---
review_id: abc-123
decision: request_changes
issues_count: 2
iteration: 1
reviewer: SecurityReviewer
---

Found 2 issues in your PR:
...actual review body...
```

The ReviewerService has a thread watcher that:
1. Subscribes to notifications for review threads
2. Parses YAML frontmatter from new messages
3. Extracts decision, issue count, and reviewer identity
4. Updates the FSM state in the reviews table
5. Creates review_iterations and review_issues records

This keeps the review conversation as normal mail messages while enabling
structured state tracking.

---

## 15. Activity Integration

Review events create activity entries using the existing ActivityStore:

```go
// Activity types for reviews.
const (
    ActivityReviewRequested  = "review_requested"
    ActivityReviewStarted    = "review_started"
    ActivityReviewCompleted  = "review_completed"
    ActivityReviewApproved   = "review_approved"
    ActivityReviewRejected   = "review_rejected"
    ActivityIssueResolved    = "issue_resolved"
)

// Example: when a review is requested.
s.activityStore.CreateActivity(ctx, store.CreateActivityParams{
    AgentID:      req.RequesterID,
    ActivityType: ActivityReviewRequested,
    Description:  fmt.Sprintf("Requested review for branch %s", req.Branch),
    Metadata:     fmt.Sprintf(`{"review_id":"%s","branch":"%s"}`, reviewID, req.Branch),
})
```

These appear in the activity feed on the AgentsDashboard and in the
WebSocket `activity` event stream.

---

## 16. WebSocket Event Pipeline

Review events flow through the existing notification infrastructure:

```
ReviewerService (FSM state change)
         │
         ▼
NotificationHub actor (existing, reused)
    - ReviewerService sends Tell() with review events
    - Hub forwards to subscribed WebSocket clients
         │
         ▼
HubNotificationBridge (existing)
    - Converts to WSMessage with review-specific types
         │
         ▼
WebSocket Hub → React Frontend
    - review_updated: Query invalidation for review lists
    - review_iteration_added: Refresh review detail
    - issue_resolved: Update issue tracker
```

New WSMessage types added to `internal/web/websocket.go`:
```go
const (
    WSMsgTypeReviewUpdated       = "review_updated"
    WSMsgTypeReviewIterationAdded = "review_iteration_added"
    WSMsgTypeIssueResolved       = "issue_resolved"
)
```

---

## 17. Design Decisions

These decisions have been made based on discussion:

1. **Mail-based messaging**: Review requests and responses are normal mail messages
   - `substrate review request` is a wrapper around mail send to the reviews topic
   - Reviews appear in inbox alongside other mail
   - Thread management handled by existing mail system
   - Back-and-forth discussion happens in the same thread

2. **Reviewer agents are auto-spawned Claude Code instances**: Per-review lifecycle
   - Auto-spawned by ReviewerService via Claude Agent Go SDK
   - Each active review gets a dedicated agent with unique identity
   - Agents use bidirectional `Stream` for conversational review
   - On restart, active reviews are re-spawned from DB state

3. **ReviewerService is an actor**: Orchestration and lifecycle management
   - Creates review records in DB, manages FSM state
   - Spawns and manages reviewer agent lifecycles
   - Monitors threads via YAML frontmatter for FSM updates
   - Aggregates multi-reviewer results and computes consensus

4. **Topic-based routing**: Author sends to reviews topic, system picks up
   - Author agent sends review request to the "reviews" topic
   - ReviewerService subscribes to the topic and handles requests
   - Spawns dedicated reviewer agent per review
   - Reviewer carries out conversation in the thread

5. **gRPC-gateway for API**: Consistent with existing codebase
   - Proto definitions in mail.proto (extended)
   - REST endpoints auto-generated via grpc-gateway
   - Registered in server.go alongside existing services

6. **Store follows 3-tier pattern**: ReviewStore interface
   - Domain interface in interfaces.go
   - SqlcStore, txSqlcStore, and MockStore implementations
   - sqlc queries generated from migration SQL

---

## 18. Resolved Design Questions

These questions were discussed and resolved:

1. **Review Scope**: Per-commit, but also look at PR as a whole.
   - The prompt handles both perspectives.
   - Incremental option for re-reviews after changes.

2. **Reviewer Workspace**: Temp work directory per review.
   - Each review gets an isolated checkout in a temp directory.
   - Cleaned up after review completes.

3. **GitHub Integration**: Optional, mail is the primary route.
   - Can use `gh pr review` to post comments when a PR exists.
   - Either way, mail is how agents communicate with each other.

4. **Cost Tracking**: Each agent tracks its own costs.
   - Individual reviewer agents track their own usage.
   - No centralized cost aggregation needed initially.

5. **Reviewer Discovery**: Topic-based with auto-spawn.
   - Author sends to "reviews" topic.
   - ReviewerService picks up and spawns dedicated reviewer.
   - Direct mail to specific reviewer also supported.

6. **Reviewer Identity**: Per-review agents.
   - Each review gets a uniquely named agent (e.g., "SecurityReviewer-review-abc123").
   - Provides traceability in activity feed and message history.
   - Cleaned up when review completes.

7. **Metadata Storage**: Reviews table + messages metadata column.
   - Structured review data in reviews/review_iterations/review_issues tables.
   - General-purpose metadata TEXT column added to messages table.
   - YAML frontmatter in reviewer messages for FSM state updates.

8. **Notification Pipeline**: Reuse existing NotificationHub.
   - ReviewerService sends events through existing NotificationHub actor.
   - HubNotificationBridge forwards to WebSocket clients.
   - New WS message types for review-specific events.

---

## 19. Success Metrics

- **Review Turnaround**: Time from request to first response
- **Iteration Count**: Average iterations before approval
- **Issue Accuracy**: False positive rate for flagged issues
- **Agent Satisfaction**: Feedback on review quality
- **Cost Efficiency**: Cost per review vs manual review time saved
