package mail

import (
	"context"
	"fmt"
	"time"
)

// ThreadFSM manages the state machine for a message recipient pair. It
// processes events, emits outbox events for side effects, and tracks state
// transitions.
type ThreadFSM struct {
	env   *ThreadEnvironment
	state ThreadState
}

// NewThreadFSM creates a new thread FSM with the given initial state and
// environment.
func NewThreadFSM(state ThreadState, env *ThreadEnvironment) *ThreadFSM {
	return &ThreadFSM{
		env:   env,
		state: state,
	}
}

// NewThreadFSMFromDB creates a thread FSM by loading state from the database
// representation.
func NewThreadFSMFromDB(agentID, messageID int64, threadID string,
	stateStr string, snoozedUntil *time.Time, readAt *time.Time,
	ackedAt *time.Time, trashRetention time.Duration,
) *ThreadFSM {
	if trashRetention == 0 {
		trashRetention = DefaultTrashRetention
	}

	env := &ThreadEnvironment{
		AgentID:        agentID,
		MessageID:      messageID,
		ThreadID:       threadID,
		TrashRetention: trashRetention,
	}

	// Reconstruct state with additional data.
	var state ThreadState
	switch RecipientState(stateStr) {
	case StateUnreadStr:
		state = &StateUnread{}

	case StateReadStr:
		readTime := time.Time{}
		if readAt != nil {
			readTime = *readAt
		}
		state = &StateRead{ReadAt: readTime, AckedAt: ackedAt}

	case StateStarredStr:
		readTime := time.Time{}
		if readAt != nil {
			readTime = *readAt
		}
		state = &StateStarred{ReadAt: readTime, AckedAt: ackedAt}

	case StateSnoozedStr:
		snoozed := time.Time{}
		if snoozedUntil != nil {
			snoozed = *snoozedUntil
		}
		readTime := time.Time{}
		if readAt != nil {
			readTime = *readAt
		}
		state = &StateSnoozed{
			SnoozedUntil: snoozed,
			WasRead:      readAt != nil,
			ReadAt:       readTime,
			AckedAt:      ackedAt,
		}

	case StateArchivedStr:
		readTime := time.Time{}
		if readAt != nil {
			readTime = *readAt
		}
		state = &StateArchived{ReadAt: readTime, AckedAt: ackedAt}

	case StateTrashStr:
		// Default purge time if not tracked elsewhere.
		purgeAt := time.Now().Add(trashRetention)
		state = &StateTrash{PurgeAt: purgeAt}

	default:
		state = &StateUnread{}
	}

	return &ThreadFSM{env: env, state: state}
}

// State returns the current state of the FSM.
func (f *ThreadFSM) State() ThreadState {
	return f.state
}

// StateString returns the string representation of the current state.
func (f *ThreadFSM) StateString() string {
	return f.state.String()
}

// IsTerminal returns true if the FSM is in a terminal state.
func (f *ThreadFSM) IsTerminal() bool {
	return f.state.IsTerminal()
}

// ProcessEvent processes an event and returns the outbox events that should
// be dispatched to external actors.
func (f *ThreadFSM) ProcessEvent(ctx context.Context,
	event ThreadEvent,
) ([]ThreadOutboxEvent, error) {
	transition, err := f.state.ProcessEvent(ctx, event, f.env)
	if err != nil {
		return nil, fmt.Errorf("process event %T: %w", event, err)
	}

	// Update state.
	f.state = transition.NextState

	return transition.OutboxEvents, nil
}

// Resume sends a ResumeThreadEvent to re-emit any pending outbox events.
// This should be called after loading an FSM from the database to ensure
// scheduled operations are re-registered.
func (f *ThreadFSM) Resume(ctx context.Context) ([]ThreadOutboxEvent, error) {
	return f.ProcessEvent(ctx, ResumeThreadEvent{
		AgentID:   f.env.AgentID,
		MessageID: f.env.MessageID,
	})
}

// ThreadFSMManager manages multiple thread FSMs and provides a higher-level
// API for message state operations.
type ThreadFSMManager struct {
	trashRetention time.Duration
}

// NewThreadFSMManager creates a new thread FSM manager with the given
// configuration.
func NewThreadFSMManager(trashRetention time.Duration) *ThreadFSMManager {
	if trashRetention == 0 {
		trashRetention = DefaultTrashRetention
	}
	return &ThreadFSMManager{trashRetention: trashRetention}
}

// CreateFSM creates a new FSM for a message recipient pair with the initial
// unread state.
func (m *ThreadFSMManager) CreateFSM(agentID, messageID int64,
	threadID string,
) *ThreadFSM {
	return NewThreadFSM(&StateUnread{}, &ThreadEnvironment{
		AgentID:        agentID,
		MessageID:      messageID,
		ThreadID:       threadID,
		TrashRetention: m.trashRetention,
	})
}

// LoadFSM loads an FSM from database state.
func (m *ThreadFSMManager) LoadFSM(agentID, messageID int64, threadID string,
	stateStr string, snoozedUntil *time.Time, readAt *time.Time,
	ackedAt *time.Time,
) *ThreadFSM {
	return NewThreadFSMFromDB(
		agentID, messageID, threadID, stateStr,
		snoozedUntil, readAt, ackedAt, m.trashRetention,
	)
}

// MarkRead creates a ReadEvent and returns the outbox events.
func (m *ThreadFSMManager) MarkRead(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, ReadEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
		ReadAt:    time.Now(),
	})
}

// Star creates a StarEvent and returns the outbox events.
func (m *ThreadFSMManager) Star(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, StarEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Unstar creates an UnstarEvent and returns the outbox events.
func (m *ThreadFSMManager) Unstar(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, UnstarEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Snooze creates a SnoozeEvent and returns the outbox events.
func (m *ThreadFSMManager) Snooze(ctx context.Context, fsm *ThreadFSM,
	until time.Time,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, SnoozeEvent{
		AgentID:      fsm.env.AgentID,
		MessageID:    fsm.env.MessageID,
		SnoozedUntil: until,
	})
}

// Wake creates a WakeEvent and returns the outbox events.
func (m *ThreadFSMManager) Wake(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, WakeEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Archive creates an ArchiveEvent and returns the outbox events.
func (m *ThreadFSMManager) Archive(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, ArchiveEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Unarchive creates an UnarchiveEvent and returns the outbox events.
func (m *ThreadFSMManager) Unarchive(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, UnarchiveEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Trash creates a TrashEvent and returns the outbox events.
func (m *ThreadFSMManager) Trash(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, TrashEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Restore creates a RestoreEvent and returns the outbox events.
func (m *ThreadFSMManager) Restore(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, RestoreEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
	})
}

// Ack creates an AckEvent and returns the outbox events.
func (m *ThreadFSMManager) Ack(ctx context.Context,
	fsm *ThreadFSM,
) ([]ThreadOutboxEvent, error) {
	return fsm.ProcessEvent(ctx, AckEvent{
		AgentID:   fsm.env.AgentID,
		MessageID: fsm.env.MessageID,
		AckedAt:   time.Now(),
	})
}
