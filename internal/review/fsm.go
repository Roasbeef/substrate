package review

import (
	"fmt"
	"sync"
	"time"
)

// ReviewEvent triggers state transitions.
type ReviewEvent interface {
	reviewEventMarker()
}

// Event types for the review FSM.
type (
	// SubmitForReviewEvent is sent when a review is requested.
	SubmitForReviewEvent struct {
		RequesterID int64
	}

	// StartReviewEvent is sent when a reviewer starts reviewing.
	StartReviewEvent struct {
		ReviewerID string
	}

	// RequestChangesEvent is sent when a reviewer requests changes.
	RequestChangesEvent struct {
		ReviewerID string
		Issues     []ReviewIssue
	}

	// ResubmitEvent is sent when the author pushes fixes.
	ResubmitEvent struct {
		NewCommitSHA string
	}

	// ApproveEvent is sent when a reviewer approves.
	ApproveEvent struct {
		ReviewerID string
	}

	// RejectEvent is sent when a review is rejected.
	RejectEvent struct {
		Reason string
	}

	// CancelEvent is sent when a review is cancelled.
	CancelEvent struct {
		Reason string
	}
)

// Event marker implementations.
func (SubmitForReviewEvent) reviewEventMarker() {}
func (StartReviewEvent) reviewEventMarker()     {}
func (RequestChangesEvent) reviewEventMarker()  {}
func (ResubmitEvent) reviewEventMarker()        {}
func (ApproveEvent) reviewEventMarker()         {}
func (RejectEvent) reviewEventMarker()          {}
func (CancelEvent) reviewEventMarker()          {}

// ReviewFSM manages review state transitions.
type ReviewFSM struct {
	mu sync.RWMutex

	// Review identifiers
	ReviewID string
	ThreadID string

	// Current state
	CurrentState ReviewState

	// History for debugging/UI
	Transitions []StateTransition

	// Multi-reviewer tracking
	ReviewerStates map[string]ReviewerState

	// Config for consensus
	Config *MultiReviewConfig
}

// NewReviewFSM creates a new review FSM.
func NewReviewFSM(reviewID, threadID string, config *MultiReviewConfig) *ReviewFSM {
	return &ReviewFSM{
		ReviewID:       reviewID,
		ThreadID:       threadID,
		CurrentState:   StateNew,
		Transitions:    make([]StateTransition, 0),
		ReviewerStates: make(map[string]ReviewerState),
		Config:         config,
	}
}

// State returns the current state.
func (fsm *ReviewFSM) State() ReviewState {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.CurrentState
}

// ProcessEvent handles a review event and returns the new state.
func (fsm *ReviewFSM) ProcessEvent(event ReviewEvent) (ReviewState, error) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	oldState := fsm.CurrentState
	var newState ReviewState
	var eventName string

	switch e := event.(type) {
	case SubmitForReviewEvent:
		eventName = "submit_for_review"
		if fsm.CurrentState != StateNew {
			return fsm.CurrentState, fmt.Errorf(
				"cannot submit for review from state %s", fsm.CurrentState,
			)
		}
		newState = StatePendingReview

	case StartReviewEvent:
		eventName = "start_review"
		if fsm.CurrentState != StatePendingReview &&
			fsm.CurrentState != StateReReview {
			return fsm.CurrentState, fmt.Errorf(
				"cannot start review from state %s", fsm.CurrentState,
			)
		}
		newState = StateUnderReview
		fsm.ReviewerStates[e.ReviewerID] = ReviewerState{
			ReviewerID: e.ReviewerID,
			ReviewedAt: time.Now(),
		}

	case RequestChangesEvent:
		eventName = "request_changes"
		if fsm.CurrentState != StateUnderReview {
			return fsm.CurrentState, fmt.Errorf(
				"cannot request changes from state %s", fsm.CurrentState,
			)
		}
		newState = StateChangesRequested
		if state, ok := fsm.ReviewerStates[e.ReviewerID]; ok {
			state.Decision = DecisionRequestChanges
			state.Issues = e.Issues
			state.ReviewedAt = time.Now()
			fsm.ReviewerStates[e.ReviewerID] = state
		}

	case ResubmitEvent:
		eventName = "resubmit"
		if fsm.CurrentState != StateChangesRequested {
			return fsm.CurrentState, fmt.Errorf(
				"cannot resubmit from state %s", fsm.CurrentState,
			)
		}
		newState = StateReReview

	case ApproveEvent:
		eventName = "approve"
		if fsm.CurrentState != StateUnderReview {
			return fsm.CurrentState, fmt.Errorf(
				"cannot approve from state %s", fsm.CurrentState,
			)
		}

		// Update reviewer state
		if state, ok := fsm.ReviewerStates[e.ReviewerID]; ok {
			state.Decision = DecisionApprove
			state.ReviewedAt = time.Now()
			fsm.ReviewerStates[e.ReviewerID] = state
		} else {
			fsm.ReviewerStates[e.ReviewerID] = ReviewerState{
				ReviewerID: e.ReviewerID,
				Decision:   DecisionApprove,
				ReviewedAt: time.Now(),
			}
		}

		// Check if we have consensus for approval
		if fsm.hasConsensus() {
			newState = StateApproved
		} else {
			// Stay in under_review, waiting for more approvals
			newState = StateUnderReview
		}

	case RejectEvent:
		eventName = "reject"
		newState = StateRejected

	case CancelEvent:
		eventName = "cancel"
		newState = StateCancelled

	default:
		return fsm.CurrentState, fmt.Errorf("unknown event type: %T", event)
	}

	// Record transition
	fsm.Transitions = append(fsm.Transitions, StateTransition{
		FromState: oldState,
		ToState:   newState,
		Event:     eventName,
		Timestamp: time.Now(),
	})

	fsm.CurrentState = newState
	return newState, nil
}

// hasConsensus checks if enough reviewers have approved.
func (fsm *ReviewFSM) hasConsensus() bool {
	if fsm.Config == nil {
		// Without config, single approval is enough
		for _, state := range fsm.ReviewerStates {
			if state.Decision == DecisionApprove {
				return true
			}
		}
		return false
	}

	approvals := 0
	for _, state := range fsm.ReviewerStates {
		if state.Decision == DecisionApprove {
			approvals++
		}
		// Check for blocking issues
		if fsm.Config.BlockOnCritical {
			for _, issue := range state.Issues {
				if issue.Severity == SeverityCritical {
					return false
				}
			}
		}
	}

	if fsm.Config.RequireAll {
		return approvals >= len(fsm.Config.Reviewers)
	}
	return approvals >= fsm.Config.MinApprovals
}

// GetReviewerDecisions returns a summary of all reviewer decisions.
func (fsm *ReviewFSM) GetReviewerDecisions() map[string]ReviewDecision {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	decisions := make(map[string]ReviewDecision)
	for id, state := range fsm.ReviewerStates {
		decisions[id] = state.Decision
	}
	return decisions
}

// GetTransitionHistory returns the state transition history.
func (fsm *ReviewFSM) GetTransitionHistory() []StateTransition {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	history := make([]StateTransition, len(fsm.Transitions))
	copy(history, fsm.Transitions)
	return history
}

// HasCriticalIssues checks if any reviewer found critical issues.
func (fsm *ReviewFSM) HasCriticalIssues() bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	for _, state := range fsm.ReviewerStates {
		for _, issue := range state.Issues {
			if issue.Severity == SeverityCritical {
				return true
			}
		}
	}
	return false
}

// OpenIssueCount returns the total number of open issues.
func (fsm *ReviewFSM) OpenIssueCount() int {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	count := 0
	for _, state := range fsm.ReviewerStates {
		count += len(state.Issues)
	}
	return count
}
