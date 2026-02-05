package review

// ReviewOutboxEvent is the sealed interface for events emitted by the review
// FSM to external actors. These events trigger side effects like database
// persistence, notifications, and agent spawning.
type ReviewOutboxEvent interface {
	// isReviewOutboxEvent seals the interface to prevent external
	// implementations.
	isReviewOutboxEvent()
}

// Ensure all outbox event types implement ReviewOutboxEvent.
func (PersistReviewState) isReviewOutboxEvent()      {}
func (NotifyReviewStateChange) isReviewOutboxEvent() {}
func (SpawnReviewerAgent) isReviewOutboxEvent()      {}
func (CreateReviewIteration) isReviewOutboxEvent()   {}
func (CreateReviewIssues) isReviewOutboxEvent()      {}
func (RecordActivity) isReviewOutboxEvent()          {}

// PersistReviewState requests database persistence of the review state.
type PersistReviewState struct {
	ReviewID string
	NewState string
}

// NotifyReviewStateChange notifies subscribers of a review state change
// for real-time UI updates via WebSocket.
type NotifyReviewStateChange struct {
	ReviewID string
	OldState string
	NewState string
}

// SpawnReviewerAgent requests the sub-actor to create a Claude Agent SDK
// instance for performing the review.
type SpawnReviewerAgent struct {
	ReviewID  string
	ThreadID  string
	RepoPath  string
	Requester int64
}

// CreateReviewIteration requests persistence of a review iteration result.
type CreateReviewIteration struct {
	ReviewID     string
	IterationNum int
	ReviewerID   string
	Decision     string
	Summary      string
	Issues       []ReviewIssueSummary
}

// CreateReviewIssues requests persistence of individual review issues.
type CreateReviewIssues struct {
	ReviewID string
	Issues   []ReviewIssueSummary
}

// RecordActivity requests an activity entry for the review event.
type RecordActivity struct {
	AgentID      int64
	ActivityType string
	Description  string
	ReviewID     string
}
