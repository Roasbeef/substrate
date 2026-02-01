package review

import (
	"time"
)

// ReviewType indicates the type of review to perform.
type ReviewType string

const (
	ReviewTypeFull        ReviewType = "full"
	ReviewTypeIncremental ReviewType = "incremental"
	ReviewTypeSecurity    ReviewType = "security"
	ReviewTypePerformance ReviewType = "performance"
)

// Priority indicates the urgency of the review request.
type Priority string

const (
	PriorityUrgent Priority = "urgent"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// ReviewDecision indicates the review outcome.
type ReviewDecision string

const (
	DecisionApprove        ReviewDecision = "approve"
	DecisionRequestChanges ReviewDecision = "request_changes"
	DecisionComment        ReviewDecision = "comment"
)

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

// IssueType categorizes the type of issue found.
type IssueType string

const (
	IssueTypeBug             IssueType = "bug"
	IssueTypeSecurity        IssueType = "security"
	IssueTypeClaudeMD        IssueType = "claude_md_violation"
	IssueTypeLogicError      IssueType = "logic_error"
	IssueTypePerformance     IssueType = "performance"
	IssueTypeArchitecture    IssueType = "architecture"
)

// Severity indicates the severity of an issue.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// IssueStatus tracks the resolution state of an issue.
type IssueStatus string

const (
	IssueStatusOpen      IssueStatus = "open"
	IssueStatusFixed     IssueStatus = "fixed"
	IssueStatusWontFix   IssueStatus = "wont_fix"
	IssueStatusDuplicate IssueStatus = "duplicate"
)

// ReviewRequest is sent by agents requesting a PR review.
type ReviewRequest struct {
	// RequesterID is the agent ID requesting the review.
	RequesterID int64

	// ThreadID is the mail thread to use (empty for new thread).
	ThreadID string

	// PR Information
	PRNumber   int    // GitHub PR number (if applicable)
	Branch     string // Branch name to review
	BaseBranch string // Base branch (main, master, etc.)
	CommitSHA  string // Specific commit to review
	RepoPath   string // Local repo path for analysis
	RemoteURL  string // Git remote URL for reviewer to clone/fetch

	// Review Configuration
	ReviewType ReviewType // Type of review to perform
	Reviewers  []string   // Specific reviewer personas to use
	Priority   Priority   // Urgency of the review

	// Context
	Description  string   // PR description/context
	ChangedFiles []string // List of changed files (optional hint)
}

// ReviewResponse contains the structured review feedback.
type ReviewResponse struct {
	ReviewID     string
	ThreadID     string
	ReviewerName string // Which reviewer persona

	// Review Results
	Decision    ReviewDecision
	Summary     string
	Issues      []ReviewIssue
	Suggestions []Suggestion

	// Metadata
	FilesReviewed int
	LinesAnalyzed int
	ReviewedAt    time.Time
	DurationMS    int64
	CostUSD       float64
}

// ReviewIssue represents a specific issue found during review.
type ReviewIssue struct {
	ID          string
	Type        IssueType
	Severity    Severity
	File        string
	LineStart   int
	LineEnd     int
	Title       string
	Description string
	CodeSnippet string // Relevant code
	Suggestion  string // Fix suggestion (optional)
	ClaudeMDRef string // CLAUDE.md rule citation (if applicable)
}

// Suggestion represents a non-blocking improvement suggestion.
type Suggestion struct {
	File        string
	LineStart   int
	LineEnd     int
	Title       string
	Description string
	CodeSnippet string
	Rationale   string
}

// StructuredReviewRequest contains everything needed for one-shot analysis.
type StructuredReviewRequest struct {
	ReviewID       string
	WorkDir        string        // Where code is checked out
	Diff           string        // Git diff to review
	Context        string        // PR description, previous comments
	FocusAreas     []string      // What to look for
	PreviousIssues []ReviewIssue // Issues from prior iteration (for re-review)
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

// ReviewerState tracks per-reviewer status in multi-reviewer mode.
type ReviewerState struct {
	ReviewerID string
	Decision   ReviewDecision
	ReviewedAt time.Time
	Issues     []ReviewIssue
}

// StateTransition records a state change in the review FSM.
type StateTransition struct {
	FromState ReviewState
	ToState   ReviewState
	Event     string
	Timestamp time.Time
	Metadata  map[string]string
}

// AggregatedReview combines reviews from multiple reviewers.
type AggregatedReview struct {
	ReviewID          string
	Reviewers         map[string]ReviewerSummary
	AllIssues         []ReviewIssue
	ConsensusDecision ReviewDecision
}

// ReviewerSummary provides a summary of a single reviewer's feedback.
type ReviewerSummary struct {
	Decision ReviewDecision
	Issues   int
}
