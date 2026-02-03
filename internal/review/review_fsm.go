package review

import (
	"context"
	"fmt"
)

// ReviewFSM manages review state transitions using the ProcessEvent pattern.
// Mirrors the ThreadFSM pattern from internal/mail/thread_fsm.go.
type ReviewFSM struct {
	state ReviewState
	env   *ReviewEnvironment
}

// NewReviewFSM creates a new review FSM starting in the New state.
func NewReviewFSM(reviewID, threadID, repoPath string,
	requesterID int64,
) *ReviewFSM {
	return &ReviewFSM{
		state: &StateNew{},
		env: &ReviewEnvironment{
			ReviewID:    reviewID,
			ThreadID:    threadID,
			RepoPath:    repoPath,
			RequesterID: requesterID,
		},
	}
}

// NewReviewFSMFromDB creates a review FSM from a persisted state string.
// Used when recovering active reviews on restart.
func NewReviewFSMFromDB(reviewID, threadID, repoPath string,
	requesterID int64, stateStr string,
) *ReviewFSM {
	return &ReviewFSM{
		state: StateFromString(stateStr),
		env: &ReviewEnvironment{
			ReviewID:    reviewID,
			ThreadID:    threadID,
			RepoPath:    repoPath,
			RequesterID: requesterID,
		},
	}
}

// ProcessEvent processes an event and returns the outbox events that should
// be dispatched to external actors.
func (f *ReviewFSM) ProcessEvent(ctx context.Context,
	event ReviewEvent,
) ([]ReviewOutboxEvent, error) {
	transition, err := f.state.ProcessEvent(ctx, event, f.env)
	if err != nil {
		return nil, fmt.Errorf("process event %T: %w", event, err)
	}

	// Update state.
	f.state = transition.NextState

	return transition.OutboxEvents, nil
}

// CurrentState returns a string representation of the current state.
func (f *ReviewFSM) CurrentState() string {
	return f.state.String()
}

// State returns the current ReviewState.
func (f *ReviewFSM) State() ReviewState {
	return f.state
}

// IsTerminal returns true if the review has reached a terminal state.
func (f *ReviewFSM) IsTerminal() bool {
	return f.state.IsTerminal()
}

// Environment returns the FSM's environment.
func (f *ReviewFSM) Environment() *ReviewEnvironment {
	return f.env
}
