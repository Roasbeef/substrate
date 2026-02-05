package review

import (
	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// ReviewRequest is the union type for all review service requests.
type ReviewRequest interface {
	actor.Message
	isReviewRequest()
}

// ReviewResponse is the union type for all review service responses.
type ReviewResponse interface {
	isReviewResponse()
}

// Ensure all request types implement ReviewRequest.
func (CreateReviewMsg) isReviewRequest() {}
func (GetReviewMsg) isReviewRequest()    {}
func (ListReviewsMsg) isReviewRequest()  {}
func (ResubmitMsg) isReviewRequest()     {}
func (CancelReviewMsg) isReviewRequest() {}
func (DeleteReviewMsg) isReviewRequest() {}
func (GetIssuesMsg) isReviewRequest()    {}
func (UpdateIssueMsg) isReviewRequest()  {}

// Ensure all response types implement ReviewResponse.
func (CreateReviewResp) isReviewResponse() {}
func (GetReviewResp) isReviewResponse()    {}
func (ListReviewsResp) isReviewResponse()  {}
func (ResubmitResp) isReviewResponse()     {}
func (CancelReviewResp) isReviewResponse() {}
func (DeleteReviewResp) isReviewResponse() {}
func (GetIssuesResp) isReviewResponse()    {}
func (UpdateIssueResp) isReviewResponse()  {}

// =============================================================================
// Reviewer sub-actor messages
// =============================================================================

// ReviewerRequest is the union type for all reviewer sub-actor requests.
// These messages are sent to individual reviewer actors spawned per review.
type ReviewerRequest interface {
	actor.Message
	isReviewerRequest()
}

// Ensure all reviewer request types implement ReviewerRequest.
func (RunReviewMsg) isReviewerRequest()    {}
func (ResumeReviewMsg) isReviewerRequest() {}

// RunReviewMsg tells a reviewer sub-actor to execute a fresh code review.
type RunReviewMsg struct {
	actor.BaseMessage
}

// MessageType implements actor.Message.
func (RunReviewMsg) MessageType() string { return "RunReviewMsg" }

// ResumeReviewMsg tells a reviewer sub-actor to resume a review after the
// author has pushed new changes (resubmit).
type ResumeReviewMsg struct {
	actor.BaseMessage

	CommitSHA string
}

// MessageType implements actor.Message.
func (ResumeReviewMsg) MessageType() string { return "ResumeReviewMsg" }

// =============================================================================
// Request messages
// =============================================================================

// CreateReviewMsg requests creation of a new code review.
type CreateReviewMsg struct {
	actor.BaseMessage

	RequesterID int64
	PRNumber    int
	Branch      string
	BaseBranch  string
	CommitSHA   string
	RepoPath    string
	RemoteURL   string
	ReviewType  string // full, incremental, security, performance.
	Priority    string // urgent, normal, low.
	Reviewers   []string
	Description string
}

// MessageType implements actor.Message.
func (CreateReviewMsg) MessageType() string { return "CreateReviewMsg" }

// GetReviewMsg requests details for a specific review.
type GetReviewMsg struct {
	actor.BaseMessage

	ReviewID string
}

// MessageType implements actor.Message.
func (GetReviewMsg) MessageType() string { return "GetReviewMsg" }

// ListReviewsMsg requests a list of reviews.
type ListReviewsMsg struct {
	actor.BaseMessage

	State       string // Optional filter by state.
	RequesterID int64  // Optional filter by requester.
	Limit       int
	Offset      int
}

// MessageType implements actor.Message.
func (ListReviewsMsg) MessageType() string { return "ListReviewsMsg" }

// ResubmitMsg re-requests review after author has pushed changes.
type ResubmitMsg struct {
	actor.BaseMessage

	ReviewID  string
	CommitSHA string
}

// MessageType implements actor.Message.
func (ResubmitMsg) MessageType() string { return "ResubmitMsg" }

// CancelReviewMsg cancels an active review.
type CancelReviewMsg struct {
	actor.BaseMessage

	ReviewID string
	Reason   string
}

// MessageType implements actor.Message.
func (CancelReviewMsg) MessageType() string { return "CancelReviewMsg" }

// DeleteReviewMsg deletes a review and all associated data.
type DeleteReviewMsg struct {
	actor.BaseMessage

	ReviewID string
}

// MessageType implements actor.Message.
func (DeleteReviewMsg) MessageType() string { return "DeleteReviewMsg" }

// GetIssuesMsg requests issues for a specific review.
type GetIssuesMsg struct {
	actor.BaseMessage

	ReviewID string
}

// MessageType implements actor.Message.
func (GetIssuesMsg) MessageType() string { return "GetIssuesMsg" }

// UpdateIssueMsg updates the status of a review issue.
type UpdateIssueMsg struct {
	actor.BaseMessage

	ReviewID string
	IssueID  int64
	Status   string // open, fixed, wont_fix, duplicate.
}

// MessageType implements actor.Message.
func (UpdateIssueMsg) MessageType() string { return "UpdateIssueMsg" }

// =============================================================================
// Response messages
// =============================================================================

// CreateReviewResp is the response for a CreateReviewMsg.
type CreateReviewResp struct {
	ReviewID string
	ThreadID string
	State    string
	Error    error
}

// GetReviewResp is the response for a GetReviewMsg.
type GetReviewResp struct {
	ReviewID         string
	ThreadID         string
	State            string
	Branch           string
	BaseBranch       string
	ReviewType       string
	Iterations       int
	OpenIssues       int64
	IterationDetails []IterationDetail
	Error            error
}

// IterationDetail contains the full details of a review iteration.
type IterationDetail struct {
	IterationNum  int
	ReviewerID    string
	Decision      string
	Summary       string
	FilesReviewed int
	LinesAnalyzed int
	DurationMS    int64
	CostUSD       float64
	StartedAt     int64
	CompletedAt   int64
}

// ListReviewsResp is the response for a ListReviewsMsg.
type ListReviewsResp struct {
	Reviews []ReviewSummary
	Error   error
}

// ReviewSummary is a lightweight review representation for listings.
type ReviewSummary struct {
	ReviewID    string
	ThreadID    string
	RequesterID int64
	Branch      string
	State       string
	ReviewType  string
	CreatedAt   int64
}

// ResubmitResp is the response for a ResubmitMsg.
type ResubmitResp struct {
	ReviewID string
	NewState string
	Error    error
}

// CancelReviewResp is the response for a CancelReviewMsg.
type CancelReviewResp struct {
	Error error
}

// DeleteReviewResp is the response for a DeleteReviewMsg.
type DeleteReviewResp struct {
	Error error
}

// GetIssuesResp is the response for a GetIssuesMsg.
type GetIssuesResp struct {
	Issues []IssueSummary
	Error  error
}

// IssueSummary is a lightweight issue representation for listings.
type IssueSummary struct {
	ID           int64
	ReviewID     string
	IterationNum int
	IssueType    string
	Severity     string
	FilePath     string
	LineStart    int
	Title        string
	Status       string
}

// UpdateIssueResp is the response for an UpdateIssueMsg.
type UpdateIssueResp struct {
	Error error
}
