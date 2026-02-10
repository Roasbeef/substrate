package review

import (
	"context"
	"fmt"
)

// ReviewState is the sealed interface for all review states. Each state
// handles incoming events and returns state transitions with optional outbox
// events for side effects.
type ReviewState interface {
	// ProcessEvent handles an incoming event and returns the next state
	// along with any outbox events to emit.
	ProcessEvent(ctx context.Context, event ReviewEvent,
		env *ReviewEnvironment) (*ReviewTransition, error)

	// IsTerminal returns true if this is a terminal state.
	IsTerminal() bool

	// String returns a human-readable name for the state.
	String() string

	// isReviewState seals the interface.
	isReviewState()
}

// ReviewTransition represents the result of processing an event.
type ReviewTransition struct {
	NextState    ReviewState
	OutboxEvents []ReviewOutboxEvent
}

// ReviewEnvironment provides context for state transitions.
type ReviewEnvironment struct {
	ReviewID    string
	ThreadID    string
	RepoPath    string
	RequesterID int64
}

// Compile-time verification that all concrete states implement ReviewState.
var (
	_ ReviewState = (*StateNew)(nil)
	_ ReviewState = (*StatePendingReview)(nil)
	_ ReviewState = (*StateUnderReview)(nil)
	_ ReviewState = (*StateChangesRequested)(nil)
	_ ReviewState = (*StateReReview)(nil)
	_ ReviewState = (*StateApproved)(nil)
	_ ReviewState = (*StateRejected)(nil)
	_ ReviewState = (*StateCancelled)(nil)
)

// =============================================================================
// StateNew: Initial state when a review is first created.
// =============================================================================

// StateNew is the initial state before a review request has been submitted.
type StateNew struct{}

// ProcessEvent handles events in the New state.
func (s *StateNew) ProcessEvent(_ context.Context, event ReviewEvent,
	env *ReviewEnvironment,
) (*ReviewTransition, error) {
	switch e := event.(type) {
	case SubmitForReviewEvent:
		return &ReviewTransition{
			NextState: &StatePendingReview{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "pending_review",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "new",
					NewState: "pending_review",
				},
				SpawnReviewerAgent{
					ReviewID:  env.ReviewID,
					ThreadID:  env.ThreadID,
					RepoPath:  env.RepoPath,
					Requester: e.RequesterID,
				},
				RecordActivity{
					AgentID:      e.RequesterID,
					ActivityType: "review_requested",
					Description:  "Requested code review",
					ReviewID:     env.ReviewID,
				},
			},
		}, nil

	case CancelEvent:
		return &ReviewTransition{
			NextState: &StateCancelled{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "cancelled",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "new",
					NewState: "cancelled",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unexpected event %T in state New", event,
		)
	}
}

func (s *StateNew) IsTerminal() bool { return false }
func (s *StateNew) String() string   { return "new" }
func (s *StateNew) isReviewState()   {}

// =============================================================================
// StatePendingReview: Waiting for a reviewer agent to start.
// =============================================================================

// StatePendingReview indicates the review request has been submitted and a
// reviewer agent is being spawned.
type StatePendingReview struct{}

// ProcessEvent handles events in the PendingReview state.
func (s *StatePendingReview) ProcessEvent(_ context.Context,
	event ReviewEvent, env *ReviewEnvironment,
) (*ReviewTransition, error) {
	switch e := event.(type) {
	case StartReviewEvent:
		return &ReviewTransition{
			NextState: &StateUnderReview{ReviewerID: e.ReviewerID},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "under_review",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "pending_review",
					NewState: "under_review",
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_started",
					Description: fmt.Sprintf(
						"Reviewer %s started review",
						e.ReviewerID,
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case CancelEvent:
		return &ReviewTransition{
			NextState: &StateCancelled{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "cancelled",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "pending_review",
					NewState: "cancelled",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unexpected event %T in state PendingReview", event,
		)
	}
}

func (s *StatePendingReview) IsTerminal() bool { return false }
func (s *StatePendingReview) String() string   { return "pending_review" }
func (s *StatePendingReview) isReviewState()   {}

// =============================================================================
// StateUnderReview: A reviewer agent is actively analyzing the code.
// =============================================================================

// StateUnderReview indicates a reviewer is actively analyzing the code.
type StateUnderReview struct {
	ReviewerID string
}

// ProcessEvent handles events in the UnderReview state.
func (s *StateUnderReview) ProcessEvent(_ context.Context,
	event ReviewEvent, env *ReviewEnvironment,
) (*ReviewTransition, error) {
	switch e := event.(type) {
	case ApproveEvent:
		return &ReviewTransition{
			NextState: &StateApproved{ReviewerID: e.ReviewerID},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "approved",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "under_review",
					NewState: "approved",
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_approved",
					Description: fmt.Sprintf(
						"Review approved by %s",
						e.ReviewerID,
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case RequestChangesEvent:
		return &ReviewTransition{
			NextState: &StateChangesRequested{
				ReviewerID: e.ReviewerID,
			},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "changes_requested",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "under_review",
					NewState: "changes_requested",
				},
				CreateReviewIssues{
					ReviewID: env.ReviewID,
					Issues:   e.Issues,
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_completed",
					Description: fmt.Sprintf(
						"Reviewer %s requested changes (%d issues)",
						e.ReviewerID, len(e.Issues),
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case RejectEvent:
		return &ReviewTransition{
			NextState: &StateRejected{
				ReviewerID: e.ReviewerID,
				Reason:     e.Reason,
			},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "rejected",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "under_review",
					NewState: "rejected",
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_rejected",
					Description: fmt.Sprintf(
						"Review rejected by %s: %s",
						e.ReviewerID, e.Reason,
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case CancelEvent:
		return &ReviewTransition{
			NextState: &StateCancelled{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "cancelled",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "under_review",
					NewState: "cancelled",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unexpected event %T in state UnderReview", event,
		)
	}
}

func (s *StateUnderReview) IsTerminal() bool { return false }
func (s *StateUnderReview) String() string   { return "under_review" }
func (s *StateUnderReview) isReviewState()   {}

// =============================================================================
// StateChangesRequested: Reviewer found issues, waiting for author to fix.
// =============================================================================

// StateChangesRequested indicates the reviewer found issues and the author
// needs to push fixes and resubmit.
type StateChangesRequested struct {
	ReviewerID string
}

// ProcessEvent handles events in the ChangesRequested state.
func (s *StateChangesRequested) ProcessEvent(_ context.Context,
	event ReviewEvent, env *ReviewEnvironment,
) (*ReviewTransition, error) {
	switch e := event.(type) {
	case ResubmitEvent:
		_ = e
		return &ReviewTransition{
			NextState: &StateReReview{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "re_review",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "changes_requested",
					NewState: "re_review",
				},
				SpawnReviewerAgent{
					ReviewID:  env.ReviewID,
					ThreadID:  env.ThreadID,
					RepoPath:  env.RepoPath,
					Requester: env.RequesterID,
				},
			},
		}, nil

	case ApproveEvent:
		// The reviewer updated its decision to approve after
		// processing follow-up messages during back-and-forth.
		return &ReviewTransition{
			NextState: &StateApproved{
				ReviewerID: e.ReviewerID,
			},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "approved",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "changes_requested",
					NewState: "approved",
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_approved",
					Description: fmt.Sprintf(
						"Review approved by %s "+
							"(after changes requested)",
						e.ReviewerID,
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case RejectEvent:
		// The reviewer updated its decision to reject after
		// processing follow-up messages.
		return &ReviewTransition{
			NextState: &StateRejected{
				ReviewerID: e.ReviewerID,
				Reason:     e.Reason,
			},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "rejected",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "changes_requested",
					NewState: "rejected",
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_rejected",
					Description: fmt.Sprintf(
						"Review rejected by %s: %s",
						e.ReviewerID, e.Reason,
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case RequestChangesEvent:
		// The reviewer issued another round of change requests
		// during the same back-and-forth conversation.
		return &ReviewTransition{
			NextState: &StateChangesRequested{
				ReviewerID: e.ReviewerID,
			},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "changes_requested",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "changes_requested",
					NewState: "changes_requested",
				},
				CreateReviewIssues{
					ReviewID: env.ReviewID,
					Issues:   e.Issues,
				},
				RecordActivity{
					AgentID:      env.RequesterID,
					ActivityType: "review_completed",
					Description: fmt.Sprintf(
						"Reviewer %s requested "+
							"additional changes "+
							"(%d issues)",
						e.ReviewerID, len(e.Issues),
					),
					ReviewID: env.ReviewID,
				},
			},
		}, nil

	case CancelEvent:
		return &ReviewTransition{
			NextState: &StateCancelled{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "cancelled",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "changes_requested",
					NewState: "cancelled",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unexpected event %T in state ChangesRequested", event,
		)
	}
}

func (s *StateChangesRequested) IsTerminal() bool { return false }
func (s *StateChangesRequested) String() string   { return "changes_requested" }
func (s *StateChangesRequested) isReviewState()   {}

// =============================================================================
// StateReReview: Author resubmitted, reviewer re-analyzing.
// =============================================================================

// StateReReview indicates the author has pushed fixes and a new review round
// is in progress.
type StateReReview struct{}

// ProcessEvent handles events in the ReReview state. Transitions are the same
// as UnderReview since the reviewer is doing the same type of analysis.
func (s *StateReReview) ProcessEvent(_ context.Context,
	event ReviewEvent, env *ReviewEnvironment,
) (*ReviewTransition, error) {
	switch e := event.(type) {
	case StartReviewEvent:
		return &ReviewTransition{
			NextState: &StateUnderReview{ReviewerID: e.ReviewerID},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "under_review",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "re_review",
					NewState: "under_review",
				},
			},
		}, nil

	case CancelEvent:
		return &ReviewTransition{
			NextState: &StateCancelled{},
			OutboxEvents: []ReviewOutboxEvent{
				PersistReviewState{
					ReviewID: env.ReviewID,
					NewState: "cancelled",
				},
				NotifyReviewStateChange{
					ReviewID: env.ReviewID,
					OldState: "re_review",
					NewState: "cancelled",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unexpected event %T in state ReReview", event,
		)
	}
}

func (s *StateReReview) IsTerminal() bool { return false }
func (s *StateReReview) String() string   { return "re_review" }
func (s *StateReReview) isReviewState()   {}

// =============================================================================
// Terminal states: Approved, Rejected, Cancelled.
// =============================================================================

// StateApproved indicates the review has been approved.
type StateApproved struct {
	ReviewerID string
}

// ProcessEvent returns an error since Approved is a terminal state.
func (s *StateApproved) ProcessEvent(_ context.Context,
	event ReviewEvent, _ *ReviewEnvironment,
) (*ReviewTransition, error) {
	return nil, fmt.Errorf(
		"review is in terminal state Approved, cannot process %T",
		event,
	)
}

func (s *StateApproved) IsTerminal() bool { return true }
func (s *StateApproved) String() string   { return "approved" }
func (s *StateApproved) isReviewState()   {}

// StateRejected indicates the review has been permanently rejected.
type StateRejected struct {
	ReviewerID string
	Reason     string
}

// ProcessEvent returns an error since Rejected is a terminal state.
func (s *StateRejected) ProcessEvent(_ context.Context,
	event ReviewEvent, _ *ReviewEnvironment,
) (*ReviewTransition, error) {
	return nil, fmt.Errorf(
		"review is in terminal state Rejected, cannot process %T",
		event,
	)
}

func (s *StateRejected) IsTerminal() bool { return true }
func (s *StateRejected) String() string   { return "rejected" }
func (s *StateRejected) isReviewState()   {}

// StateCancelled indicates the review has been cancelled.
type StateCancelled struct{}

// ProcessEvent returns an error since Cancelled is a terminal state.
func (s *StateCancelled) ProcessEvent(_ context.Context,
	event ReviewEvent, _ *ReviewEnvironment,
) (*ReviewTransition, error) {
	return nil, fmt.Errorf(
		"review is in terminal state Cancelled, cannot process %T",
		event,
	)
}

func (s *StateCancelled) IsTerminal() bool { return true }
func (s *StateCancelled) String() string   { return "cancelled" }
func (s *StateCancelled) isReviewState()   {}

// StateFromString reconstructs a ReviewState from its string representation.
// Used when loading review state from the database.
func StateFromString(s string) ReviewState {
	switch s {
	case "new":
		return &StateNew{}
	case "pending_review":
		return &StatePendingReview{}
	case "under_review":
		return &StateUnderReview{}
	case "changes_requested":
		return &StateChangesRequested{}
	case "re_review":
		return &StateReReview{}
	case "approved":
		return &StateApproved{}
	case "rejected":
		return &StateRejected{}
	case "cancelled":
		return &StateCancelled{}
	default:
		return &StateNew{}
	}
}
