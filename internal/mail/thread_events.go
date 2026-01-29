package mail

import "time"

// ThreadEvent is the sealed interface for all events that can be sent to a
// thread state machine. Events drive state transitions in the FSM.
type ThreadEvent interface {
	// isThreadEvent is an unexported method that seals this interface,
	// preventing external packages from implementing new event types.
	isThreadEvent()
}

// Ensure all event types implement ThreadEvent.
func (ReadEvent) isThreadEvent()       {}
func (StarEvent) isThreadEvent()       {}
func (UnstarEvent) isThreadEvent()     {}
func (SnoozeEvent) isThreadEvent()     {}
func (WakeEvent) isThreadEvent()       {}
func (ArchiveEvent) isThreadEvent()    {}
func (UnarchiveEvent) isThreadEvent()  {}
func (TrashEvent) isThreadEvent()      {}
func (RestoreEvent) isThreadEvent()    {}
func (AckEvent) isThreadEvent()        {}
func (ResumeThreadEvent) isThreadEvent() {}

// ReadEvent marks a message as read by the recipient.
type ReadEvent struct {
	AgentID   int64
	MessageID int64
	ReadAt    time.Time
}

// StarEvent stars a message for the recipient.
type StarEvent struct {
	AgentID   int64
	MessageID int64
}

// UnstarEvent removes the star from a message.
type UnstarEvent struct {
	AgentID   int64
	MessageID int64
}

// SnoozeEvent snoozes a message until a specified time.
type SnoozeEvent struct {
	AgentID      int64
	MessageID    int64
	SnoozedUntil time.Time
}

// WakeEvent wakes a snoozed message, returning it to unread state.
type WakeEvent struct {
	AgentID   int64
	MessageID int64
}

// ArchiveEvent archives a message, removing it from the inbox.
type ArchiveEvent struct {
	AgentID   int64
	MessageID int64
}

// UnarchiveEvent restores an archived message to the inbox.
type UnarchiveEvent struct {
	AgentID   int64
	MessageID int64
}

// TrashEvent moves a message to trash.
type TrashEvent struct {
	AgentID   int64
	MessageID int64
}

// RestoreEvent restores a message from trash to inbox.
type RestoreEvent struct {
	AgentID   int64
	MessageID int64
}

// AckEvent acknowledges receipt of a message with a deadline.
type AckEvent struct {
	AgentID   int64
	MessageID int64
	AckedAt   time.Time
}

// ResumeThreadEvent is sent when resuming a thread state machine after
// restart. States should re-emit any pending outbox events.
type ResumeThreadEvent struct {
	AgentID   int64
	MessageID int64
}
