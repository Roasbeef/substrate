package review

// ReviewEvent is the sealed interface for events that drive the review FSM.
// All event types must implement the unexported isReviewEvent() method.
type ReviewEvent interface {
	// isReviewEvent seals the interface to prevent external implementations.
	isReviewEvent()
}

// Ensure all event types implement ReviewEvent.
func (SubmitForReviewEvent) isReviewEvent() {}
func (StartReviewEvent) isReviewEvent()     {}
func (RequestChangesEvent) isReviewEvent()  {}
func (ResubmitEvent) isReviewEvent()        {}
func (ApproveEvent) isReviewEvent()         {}
func (RejectEvent) isReviewEvent()          {}
func (CancelEvent) isReviewEvent()          {}

// SubmitForReviewEvent is sent when an author requests a review.
type SubmitForReviewEvent struct {
	RequesterID int64
}

// StartReviewEvent is sent when a reviewer agent begins its analysis.
type StartReviewEvent struct {
	ReviewerID string
}

// RequestChangesEvent is sent when a reviewer requests changes to the code.
type RequestChangesEvent struct {
	ReviewerID string
	Issues     []ReviewIssueSummary
}

// ReviewIssueSummary is a lightweight issue reference used in events.
type ReviewIssueSummary struct {
	Title    string
	Severity string
}

// ResubmitEvent is sent when an author pushes fixes and re-requests review.
type ResubmitEvent struct {
	NewCommitSHA string
}

// ApproveEvent is sent when a reviewer approves the changes.
type ApproveEvent struct {
	ReviewerID string
}

// RejectEvent is sent when a reviewer permanently rejects the changes.
type RejectEvent struct {
	ReviewerID string
	Reason     string
}

// CancelEvent is sent when a review is cancelled by the requester.
type CancelEvent struct {
	Reason string
}
