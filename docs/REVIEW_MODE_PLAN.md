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
    "github.com/Roasbeef/claude-agent-sdk-go/claudeagent"
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

// Service handles review orchestration and can spawn structured analysis.
type Service struct {
    store     store.Storage
    mailSvc   *mail.Service
    spawner   *agent.Spawner  // For one-shot structured analysis

    // Registered reviewer configurations (for validation/routing)
    reviewers map[string]*ReviewerConfig

    // Active reviews being tracked
    activeReviews map[string]*ReviewFSM
}

// SpawnStructuredReview runs a one-shot Claude Code analysis with JSON output.
// This is used for automated review passes that need structured data.
func (s *Service) SpawnStructuredReview(ctx context.Context, req StructuredReviewRequest) (*StructuredReviewResult, error) {
    // Build prompt with diff, context, and expected JSON schema
    prompt := s.buildStructuredPrompt(req)

    // Spawn with -p (print mode) and JSON output
    resp, err := s.spawner.Spawn(ctx, prompt, agent.SpawnOpts{
        PrintMode:    true,
        OutputFormat: "json",
        WorkDir:      req.WorkDir,
        Timeout:      5 * time.Minute,
    })
    if err != nil {
        return nil, fmt.Errorf("spawn failed: %w", err)
    }

    // Parse structured JSON response
    var result StructuredReviewResult
    if err := json.Unmarshal([]byte(resp.Result), &result); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    // Update FSM based on decision
    s.processReviewResult(ctx, req.ReviewID, &result)

    return &result, nil
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

// Receive implements ActorBehavior for the review service.
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

### 1.3 Reviewer Agents (Claude Code Instances)

**Key Insight**: Reviewer agents are NOT spawned subprocesses - they are full Claude Code
agents running independently with a specialized reviewer persona.

**Setup**: Each reviewer agent is started with:
```bash
# Start a reviewer agent (runs independently)
claude --profile reviewer-security \
       --system-prompt "$(cat ~/.substrate/reviewers/security/prompt.md)" \
       --subscribe reviews-topic
```

**Or via Substrate CLI:**
```bash
# Register and start a reviewer agent
substrate reviewer start --type security --subscribe reviews

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
5. **Conversational** - Respond to author questions, participate in discussion

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

**Location**: `internal/db/migrations/000008_reviews.up.sql`

```sql
-- Review requests table
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL UNIQUE,           -- UUID
    thread_id TEXT NOT NULL,                  -- Links to message thread
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
    completed_at INTEGER,

    FOREIGN KEY (thread_id) REFERENCES messages(thread_id)
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
    // Publish to topic - all subscribed reviewers receive it
    return s.mailSvc.Publish(ctx, mail.PublishRequest{
        TopicName: config.TopicName,
        Message: mail.TopicMessage{
            Subject: fmt.Sprintf("Review Request: %s", req.Branch),
            Body:    s.formatReviewRequestBody(req),
            Metadata: map[string]string{
                "review_id":   req.ReviewID,
                "review_type": "multi",
            },
        },
    })
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

## 5. Web UI Extensions

The UI provides multiple views leveraging both mail threads and review tables:

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
│  │                        UI Views                                  │   │
│  ├──────────────────┬──────────────────┬───────────────────────────┤   │
│  │ Conversation     │ Review Dashboard │ Issue Tracker             │   │
│  │                  │                  │                           │   │
│  │ • Mail thread    │ • State timeline │ • All issues by review    │   │
│  │ • Author/reviewer│ • Iteration diffs│ • Filter: open/fixed      │   │
│  │   back & forth   │ • Reviewer votes │ • Group by file/severity  │   │
│  │ • Inline replies │ • Consensus view │ • Resolution time stats   │   │
│  │                  │ • Cost/duration  │ • Link to code location   │   │
│  └──────────────────┴──────────────────┴───────────────────────────┘   │
│                                                                         │
│  Additional Views:                                                      │
│  ┌──────────────────┬──────────────────┬───────────────────────────┐   │
│  │ Diff Annotations │ Reviewer Status  │ Review History            │   │
│  │                  │                  │                           │   │
│  │ • Side-by-side   │ • Active reviewers│ • Past reviews by repo   │   │
│  │ • Issues inline  │ • Queue depth    │ • Approval rate           │   │
│  │ • Suggestions    │ • Avg turnaround │ • Issue trends            │   │
│  └──────────────────┴──────────────────┴───────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.1 Review Thread Template

**Location**: `web/templates/partials/review-thread.html`

```html
{{define "review-thread"}}
<div id="review-thread-{{.ReviewID}}" class="review-thread">
    <!-- Review Header -->
    <div class="review-header bg-gradient-to-r from-purple-50 to-indigo-50 p-4 rounded-t-lg border-b">
        <div class="flex items-center justify-between">
            <div class="flex items-center space-x-3">
                <div class="w-10 h-10 rounded-full bg-purple-600 flex items-center justify-center">
                    <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                    </svg>
                </div>
                <div>
                    <h2 class="text-lg font-semibold text-gray-900">PR Review: {{.Branch}}</h2>
                    <p class="text-sm text-gray-500">
                        Requested by {{.RequesterName}} &bull; {{.RelativeTime}}
                    </p>
                </div>
            </div>
            <div class="flex items-center space-x-2">
                {{template "review-status-badge" .State}}
            </div>
        </div>

        <!-- Review Progress (multi-reviewer) -->
        {{if .IsMultiReviewer}}
        <div class="mt-4 flex items-center space-x-4">
            {{range .ReviewerStatuses}}
            <div class="flex items-center space-x-2">
                <span class="w-2 h-2 rounded-full {{if eq .Decision "approve"}}bg-green-500{{else if eq .Decision "request_changes"}}bg-red-500{{else}}bg-gray-300{{end}}"></span>
                <span class="text-sm text-gray-600">{{.ReviewerName}}</span>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>

    <!-- Review Conversation -->
    <div class="review-conversation divide-y divide-gray-100">
        {{range .Messages}}
        <div class="review-message p-4 {{if .IsReviewer}}bg-purple-50{{else}}bg-white{{end}}">
            <div class="flex items-start space-x-3">
                <!-- Avatar -->
                <div class="w-8 h-8 rounded-full {{if .IsReviewer}}bg-purple-600{{else}}bg-blue-600{{end}} flex items-center justify-center text-white text-sm font-medium">
                    {{.SenderInitials}}
                </div>

                <div class="flex-1">
                    <div class="flex items-center space-x-2 mb-1">
                        <span class="font-medium text-gray-900">{{.SenderName}}</span>
                        {{if .IsReviewer}}
                        <span class="px-2 py-0.5 text-xs rounded-full bg-purple-100 text-purple-700">
                            Reviewer
                        </span>
                        {{end}}
                        <span class="text-sm text-gray-500">{{.RelativeTime}}</span>
                    </div>

                    <!-- Message Body -->
                    <div class="prose prose-sm max-w-none">
                        {{.HTMLBody}}
                    </div>

                    <!-- Review Decision Badge (for review messages) -->
                    {{if .ReviewDecision}}
                    <div class="mt-3">
                        {{template "review-decision-badge" .ReviewDecision}}
                    </div>
                    {{end}}

                    <!-- Issues List (for reviews with issues) -->
                    {{if .Issues}}
                    <div class="mt-4 space-y-3">
                        {{range .Issues}}
                        {{template "review-issue-card" .}}
                        {{end}}
                    </div>
                    {{end}}
                </div>
            </div>
        </div>
        {{end}}
    </div>

    <!-- Action Bar -->
    <div class="review-actions bg-gray-50 p-4 rounded-b-lg border-t">
        {{if eq .State "changes_requested"}}
        <div class="flex items-center justify-between">
            <p class="text-sm text-gray-600">
                {{.OpenIssueCount}} issue(s) need to be addressed
            </p>
            <button hx-post="/api/reviews/{{.ReviewID}}/resubmit"
                    hx-target="#review-thread-{{.ReviewID}}"
                    hx-swap="outerHTML"
                    class="px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 text-sm">
                Re-request Review
            </button>
        </div>
        {{else if eq .State "approved"}}
        <div class="flex items-center space-x-2 text-green-700">
            <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/>
            </svg>
            <span class="font-medium">Review approved - ready to merge</span>
        </div>
        {{end}}
    </div>
</div>
{{end}}

{{define "review-issue-card"}}
<div class="review-issue border rounded-lg overflow-hidden {{if eq .Severity "critical"}}border-red-300 bg-red-50{{else if eq .Severity "high"}}border-orange-300 bg-orange-50{{else}}border-yellow-300 bg-yellow-50{{end}}">
    <div class="px-3 py-2 flex items-center justify-between">
        <div class="flex items-center space-x-2">
            <span class="px-2 py-0.5 text-xs font-medium rounded {{if eq .Severity "critical"}}bg-red-100 text-red-800{{else if eq .Severity "high"}}bg-orange-100 text-orange-800{{else}}bg-yellow-100 text-yellow-800{{end}}">
                {{.Severity}}
            </span>
            <span class="text-sm font-medium text-gray-900">{{.Title}}</span>
        </div>
        <a href="#" class="text-sm text-blue-600 hover:underline">
            {{.File}}:{{.LineStart}}
        </a>
    </div>
    <div class="px-3 py-2 border-t bg-white">
        <p class="text-sm text-gray-700">{{.Description}}</p>
        {{if .CodeSnippet}}
        <pre class="mt-2 p-2 bg-gray-800 text-gray-100 rounded text-xs overflow-x-auto"><code>{{.CodeSnippet}}</code></pre>
        {{end}}
        {{if .Suggestion}}
        <div class="mt-2 p-2 bg-green-50 border border-green-200 rounded">
            <p class="text-sm text-green-800"><strong>Suggested fix:</strong> {{.Suggestion}}</p>
        </div>
        {{end}}
    </div>
</div>
{{end}}

{{define "review-status-badge"}}
{{if eq . "approved"}}
<span class="px-3 py-1 text-sm font-medium rounded-full bg-green-100 text-green-800">
    Approved
</span>
{{else if eq . "changes_requested"}}
<span class="px-3 py-1 text-sm font-medium rounded-full bg-red-100 text-red-800">
    Changes Requested
</span>
{{else if eq . "under_review"}}
<span class="px-3 py-1 text-sm font-medium rounded-full bg-purple-100 text-purple-800">
    Under Review
</span>
{{else if eq . "pending_review"}}
<span class="px-3 py-1 text-sm font-medium rounded-full bg-yellow-100 text-yellow-800">
    Pending Review
</span>
{{else}}
<span class="px-3 py-1 text-sm font-medium rounded-full bg-gray-100 text-gray-800">
    {{.}}
</span>
{{end}}
{{end}}
```

### 5.2 Diff Annotation View

**Location**: `web/templates/partials/review-diff.html`

Shows the diff with review issues annotated inline:

```html
{{define "review-diff"}}
<div class="review-diff">
    {{range .Files}}
    <div class="diff-file border rounded-lg mb-4">
        <div class="diff-file-header bg-gray-100 px-4 py-2 flex justify-between">
            <span class="font-mono text-sm">{{.Path}}</span>
            <span class="text-sm text-gray-500">+{{.Additions}} -{{.Deletions}}</span>
        </div>

        <div class="diff-content font-mono text-sm">
            {{range .Hunks}}
            <div class="diff-hunk">
                <div class="hunk-header bg-blue-50 px-4 py-1 text-blue-700">
                    @@ -{{.OldStart}},{{.OldLines}} +{{.NewStart}},{{.NewLines}} @@
                </div>

                {{range .Lines}}
                <div class="diff-line flex {{if .IsAddition}}bg-green-50{{else if .IsDeletion}}bg-red-50{{end}}">
                    <span class="line-num w-12 text-right pr-2 text-gray-400 select-none">{{.Number}}</span>
                    <span class="line-indicator w-4 {{if .IsAddition}}text-green-600{{else if .IsDeletion}}text-red-600{{end}}">
                        {{if .IsAddition}}+{{else if .IsDeletion}}-{{else}} {{end}}
                    </span>
                    <span class="line-content flex-1 px-2">{{.Content}}</span>
                </div>

                {{/* Inline issue annotation */}}
                {{if .Issues}}
                {{range .Issues}}
                <div class="issue-annotation mx-4 my-2 p-3 rounded border-l-4
                            {{if eq .Severity "critical"}}border-red-500 bg-red-50
                            {{else if eq .Severity "high"}}border-orange-500 bg-orange-50
                            {{else}}border-yellow-500 bg-yellow-50{{end}}">
                    <div class="flex items-center gap-2 mb-1">
                        <span class="px-2 py-0.5 text-xs font-medium rounded
                                    {{if eq .Severity "critical"}}bg-red-100 text-red-800
                                    {{else if eq .Severity "high"}}bg-orange-100 text-orange-800
                                    {{else}}bg-yellow-100 text-yellow-800{{end}}">
                            {{.Severity}}
                        </span>
                        <span class="font-medium text-gray-900">{{.Title}}</span>
                        <span class="text-xs text-gray-500">by {{.ReviewerName}}</span>
                    </div>
                    <p class="text-sm text-gray-700">{{.Description}}</p>
                    {{if .Suggestion}}
                    <div class="mt-2 p-2 bg-green-50 border border-green-200 rounded text-sm">
                        <strong>Suggestion:</strong> {{.Suggestion}}
                    </div>
                    {{end}}
                </div>
                {{end}}
                {{end}}
                {{end}}
            </div>
            {{end}}
        </div>
    </div>
    {{end}}
</div>
{{end}}
```

### 5.3 Review Dashboard

**Location**: `web/templates/partials/review-dashboard.html`

Shows review state, iterations, and multi-reviewer consensus:

```html
{{define "review-dashboard"}}
<div class="review-dashboard grid grid-cols-3 gap-4">
    <!-- State Timeline -->
    <div class="col-span-2 bg-white rounded-lg border p-4">
        <h3 class="font-semibold mb-4">Review Timeline</h3>
        <div class="space-y-4">
            {{range .Iterations}}
            <div class="flex items-start gap-4">
                <div class="w-10 h-10 rounded-full flex items-center justify-center
                            {{if eq .Decision "approve"}}bg-green-100 text-green-600
                            {{else if eq .Decision "request_changes"}}bg-red-100 text-red-600
                            {{else}}bg-gray-100 text-gray-600{{end}}">
                    {{.IterationNum}}
                </div>
                <div class="flex-1">
                    <div class="flex items-center gap-2">
                        <span class="font-medium">{{.ReviewerName}}</span>
                        <span class="text-sm text-gray-500">{{.RelativeTime}}</span>
                        {{template "review-decision-badge" .Decision}}
                    </div>
                    <p class="text-sm text-gray-600 mt-1">{{.Summary}}</p>
                    {{if .Issues}}
                    <div class="mt-2 text-sm text-gray-500">
                        {{len .Issues}} issue(s) flagged
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>
    </div>

    <!-- Multi-Reviewer Status -->
    <div class="bg-white rounded-lg border p-4">
        <h3 class="font-semibold mb-4">Reviewer Status</h3>
        {{range .ReviewerStatuses}}
        <div class="flex items-center justify-between py-2 border-b last:border-0">
            <span class="text-sm">{{.ReviewerName}}</span>
            <div class="flex items-center gap-2">
                {{if eq .Status "pending"}}
                <span class="w-2 h-2 rounded-full bg-gray-300"></span>
                <span class="text-xs text-gray-500">Pending</span>
                {{else if eq .Status "reviewing"}}
                <span class="w-2 h-2 rounded-full bg-yellow-400 animate-pulse"></span>
                <span class="text-xs text-yellow-600">Reviewing</span>
                {{else if eq .Decision "approve"}}
                <span class="w-2 h-2 rounded-full bg-green-500"></span>
                <span class="text-xs text-green-600">Approved</span>
                {{else}}
                <span class="w-2 h-2 rounded-full bg-red-500"></span>
                <span class="text-xs text-red-600">Changes</span>
                {{end}}
            </div>
        </div>
        {{end}}

        <!-- Consensus -->
        <div class="mt-4 pt-4 border-t">
            <div class="text-sm text-gray-500">Consensus</div>
            <div class="mt-1">
                {{template "review-status-badge" .ConsensusDecision}}
            </div>
        </div>
    </div>
</div>
{{end}}
```

### 5.4 Reviews List Page

**Location**: `web/templates/reviews.html`

```html
{{template "layout" .}}
{{define "content"}}
<div class="reviews-page">
    <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-bold text-gray-900">Code Reviews</h1>
        <div class="flex items-center space-x-2">
            <select hx-get="/api/reviews" hx-target="#reviews-list" hx-trigger="change"
                    name="filter" class="rounded-md border-gray-300 text-sm">
                <option value="all">All Reviews</option>
                <option value="pending">Pending</option>
                <option value="in_progress">In Progress</option>
                <option value="changes_requested">Changes Requested</option>
                <option value="approved">Approved</option>
            </select>
        </div>
    </div>

    <div id="reviews-list" hx-get="/api/reviews" hx-trigger="load" hx-swap="innerHTML">
        <div class="animate-pulse">Loading reviews...</div>
    </div>
</div>
{{end}}
```

---

## 6. CLI Commands

**Location**: `cmd/substrate/cmd_review.go`

```go
package main

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

Reviewer agents can be started via CLI or programmatically via the Go SDK:

**CLI Approach:**
```bash
# Start a security reviewer (long-running, subscribes to reviews topic)
substrate reviewer start \
    --type security \
    --subscribe reviews \
    --workdir /tmp/reviewer-security

# Start with custom model
substrate reviewer start \
    --type performance \
    --model claude-sonnet-4-20250514 \
    --subscribe reviews

# List active reviewers
substrate reviewer list
# Output:
# NAME                  TYPE        STATUS   REVIEWS  LAST_ACTIVE
# security-reviewer-1   security    active   12       2m ago
# perf-reviewer-1       performance idle     8        15m ago

# Stop a reviewer
substrate reviewer stop security-reviewer-1
```

**Programmatic (Go SDK):**
```go
// Using the Claude Agent SDK to spawn a reviewer
import "github.com/Roasbeef/claude-agent-sdk-go/claudeagent"

cfg := &ReviewerConfig{
    Name:         "security-reviewer",
    Type:         "security",
    SystemPrompt: securityReviewerPrompt,
    Model:        "claude-opus-4-5-20251101",
    WorkDir:      "/tmp/reviewer-security",
}

// Create spawner with reviewer config
spawner := agent.NewSpawner(agent.SpawnConfig{
    SystemPrompt:   cfg.SystemPrompt,
    Model:          cfg.Model,
    WorkDir:        cfg.WorkDir,
    PermissionMode: claudeagent.PermissionModeAcceptEdits,
})

// Start interactive session (long-running)
session, err := spawner.SpawnInteractive(ctx)
if err != nil {
    return err
}

// Inject initial prompt to subscribe to reviews
session.Send("Subscribe to the 'reviews' topic and wait for review requests.")
```

### 9.2 Reviewer Hooks

Each reviewer agent has specialized hooks:

**SessionStart Hook:**
```bash
#!/bin/bash
# Check for pending reviews assigned to this reviewer
substrate review list --filter pending --assignee $REVIEWER_NAME --format context
```

**Stop Hook (Persistent Reviewer Pattern):**
```bash
#!/bin/bash
# Long-poll for new reviews - keep reviewer alive
result=$(substrate poll --topics reviews --wait 55s --format hook)

if [ "$(echo "$result" | jq -r '.has_messages')" = "true" ]; then
    echo "$result"
    exit 1  # Block - new reviews to process
fi

# Still block to keep reviewer alive (heartbeat mode)
echo '{"decision": "block", "reason": "Reviewer standing by for reviews"}'
exit 0
```

### 9.3 Reviewer Registration

Reviewers register with Substrate on startup:

```go
// When reviewer agent starts, register with the system
type ReviewerRegistration struct {
    Name       string   // "security-reviewer-1"
    Type       string   // "security", "performance", "architecture"
    Topics     []string // ["reviews", "security-reviews"]
    Capabilities []string // ["checkout", "test", "lint"]
    Model      string
    StartedAt  time.Time
}

// Stored in DB for tracking
// Heartbeats keep registration alive
```

---

## 10. Implementation Phases

### Phase 1: Foundation (Week 1)

1. **Database Schema**
   - Create migration for reviews tables
   - Add sqlc queries
   - Implement store methods

2. **Review Service Core**
   - Basic ReviewRequest/Response types
   - Service struct with spawner integration
   - Single-reviewer workflow

3. **CLI Commands**
   - `substrate review request`
   - `substrate review status`
   - `substrate review list`

### Phase 2: Reviewer Agent (Week 2)

1. **System Prompt**
   - Port and enhance Claude Code review prompt
   - Add Substrate-specific context

2. **Spawner Integration**
   - Configure spawner for review sessions
   - Handle streaming responses
   - Parse structured review output

3. **Review State Machine**
   - Implement FSM
   - State persistence
   - Transition handling

### Phase 3: Web UI (Week 3)

1. **Review Thread Template**
   - Specialized review thread view
   - Issue cards with severity
   - Decision badges

2. **Reviews List Page**
   - Filter and sort
   - Status indicators
   - Quick actions

3. **Real-time Updates**
   - SSE for review progress
   - Live status changes

### Phase 4: Multi-Reviewer (Week 4)

1. **Topic-Based Distribution**
   - Review request topic
   - Reviewer subscriptions
   - Consensus aggregation

2. **Specialized Reviewers**
   - Security reviewer config
   - Performance reviewer config
   - Architecture reviewer config

3. **Consensus Logic**
   - Approval requirements
   - Critical issue blocking
   - Final decision computation

### Phase 5: Integration & Polish (Week 5)

1. **Hook Integration**
   - Reviewer agent hooks
   - Author notification hooks
   - Session tracking

2. **CLAUDE.md Documentation**
   - Project-level docs
   - Reviewer agent docs
   - User guide

3. **Testing**
   - Unit tests for review service
   - Integration tests for full flow
   - Multi-reviewer scenarios

---

## 10. API Endpoints

```
POST   /api/reviews                    - Create review request
GET    /api/reviews                    - List reviews (with filters)
GET    /api/reviews/{id}               - Get review details
POST   /api/reviews/{id}/resubmit      - Re-request review after changes
DELETE /api/reviews/{id}               - Cancel review

GET    /api/reviews/{id}/iterations    - Get all review iterations
POST   /api/reviews/{id}/respond       - Submit review response (for reviewers)

GET    /api/reviews/{id}/thread        - Get review thread (HTML partial)
GET    /api/reviews/{id}/issues        - Get all issues for review
PATCH  /api/reviews/{id}/issues/{iid}  - Update issue status

# SSE endpoints
GET    /api/reviews/{id}/stream        - Stream review updates
```

---

## 11. Message Types

Extend the mail system with review-specific message metadata:

```go
// MessageType constants for reviews
const (
    MessageTypeReviewRequest  = "review_request"
    MessageTypeReviewResponse = "review_response"
    MessageTypeReviewApproval = "review_approval"
)

// Review metadata stored in message.Metadata JSON field
type ReviewMetadata struct {
    ReviewID     string         `json:"review_id"`
    ReviewType   string         `json:"review_type"`
    Decision     ReviewDecision `json:"decision,omitempty"`
    IssueCount   int            `json:"issue_count,omitempty"`
    ReviewerName string         `json:"reviewer_name,omitempty"`
}
```

---

## 12. Design Decisions

These decisions have been made based on discussion:

1. **Mail-based messaging**: Review requests and responses are normal mail messages
   - `substrate review request` is a wrapper around `substrate send`
   - Reviews appear in inbox alongside other mail
   - Thread management handled by existing mail system

2. **Reviewer agents are Claude Code agents**: Not spawned subprocesses
   - Full Claude Code instances with reviewer persona
   - Can checkout code, run tests, browse files
   - Participate in conversation with author
   - Subscribe to reviews topic for incoming requests

3. **ReviewerService for orchestration**: Server-side tracking only
   - Creates review records in DB
   - Manages FSM state transitions
   - Aggregates multi-reviewer results
   - Does NOT do actual reviewing

---

## 13. Open Questions

1. **Review Scope**: Should reviews be per-commit or per-PR?
   - Recommend: Per-commit with incremental option for re-reviews

2. **Reviewer Workspace**: Where do reviewers checkout code?
   - Option A: Temp directory per review (isolation)
   - Option B: Persistent workspace per reviewer (faster)
   - Recommend: Configurable, default to temp

3. **GitHub Integration**: Should we post reviews to GitHub PRs?
   - Recommend: Optional via `--github` flag
   - Could use `gh pr review` to post comments

4. **Cost Tracking**: How to attribute review costs?
   - Each reviewer agent tracks its own usage
   - ReviewerService aggregates per-review

5. **Reviewer Discovery**: How do reviewers find review requests?
   - Topic subscription (reviews topic)
   - Direct mail to specific reviewer
   - Recommend: Both supported

---

## 13. Success Metrics

- **Review Turnaround**: Time from request to first response
- **Iteration Count**: Average iterations before approval
- **Issue Accuracy**: False positive rate for flagged issues
- **Agent Satisfaction**: Feedback on review quality
- **Cost Efficiency**: Cost per review vs manual review time saved
