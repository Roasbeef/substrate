package mail

import "time"

// ThreadOutboxEvent is the sealed interface for events emitted by the thread
// FSM to external actors. These events trigger side effects like database
// persistence and notifications.
type ThreadOutboxEvent interface {
	// isThreadOutboxEvent seals the interface to prevent external
	// implementations.
	isThreadOutboxEvent()
}

// Ensure all outbox event types implement ThreadOutboxEvent.
func (PersistStateChange) isThreadOutboxEvent()  {}
func (NotifyStateChange) isThreadOutboxEvent()   {}
func (ScheduleWake) isThreadOutboxEvent()        {}
func (CancelScheduledWake) isThreadOutboxEvent() {}
func (SchedulePurge) isThreadOutboxEvent()       {}

// PersistStateChange requests that the new recipient state be persisted to
// the database.
type PersistStateChange struct {
	AgentID      int64
	MessageID    int64
	NewState     string
	ReadAt       *time.Time
	AckedAt      *time.Time
	SnoozedUntil *time.Time
}

// NotifyStateChange notifies subscribers that a message state changed. This
// can be used for real-time UI updates.
type NotifyStateChange struct {
	AgentID   int64
	MessageID int64
	ThreadID  string
	OldState  string
	NewState  string
}

// ScheduleWake schedules a wake event for a snoozed message at the specified
// time.
type ScheduleWake struct {
	AgentID   int64
	MessageID int64
	WakeAt    time.Time
}

// CancelScheduledWake cancels a previously scheduled wake event.
type CancelScheduledWake struct {
	AgentID   int64
	MessageID int64
}

// SchedulePurge schedules a message for permanent deletion from trash after
// the retention period.
type SchedulePurge struct {
	AgentID   int64
	MessageID int64
	PurgeAt   time.Time
}
