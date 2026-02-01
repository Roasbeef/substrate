package mail

import (
	"context"
	"fmt"
	"time"
)

// ThreadState is the sealed interface for all thread recipient states. Each
// state handles incoming events and returns state transitions with optional
// outbox events for side effects.
type ThreadState interface {
	// ProcessEvent handles an incoming event and returns the next state
	// along with any outbox events to emit.
	ProcessEvent(ctx context.Context, event ThreadEvent,
		env *ThreadEnvironment) (*ThreadTransition, error)

	// IsTerminal returns true if this is a terminal state that requires
	// no further processing.
	IsTerminal() bool

	// String returns a human-readable name for the state.
	String() string

	// isThreadState seals the interface to prevent external implementations.
	isThreadState()
}

// ThreadTransition represents the result of processing an event, containing
// the next state and any events to emit.
type ThreadTransition struct {
	NextState    ThreadState
	OutboxEvents []ThreadOutboxEvent
}

// ThreadEnvironment provides context for state transitions, including
// configuration and metadata.
type ThreadEnvironment struct {
	AgentID        int64
	MessageID      int64
	ThreadID       string
	TrashRetention time.Duration // How long to keep trashed messages.
}

// DefaultTrashRetention is the default duration to keep trashed messages
// before permanent deletion.
const DefaultTrashRetention = 30 * 24 * time.Hour // 30 days

// Ensure all state types implement ThreadState.
var (
	_ ThreadState = (*StateUnread)(nil)
	_ ThreadState = (*StateRead)(nil)
	_ ThreadState = (*StateStarred)(nil)
	_ ThreadState = (*StateSnoozed)(nil)
	_ ThreadState = (*StateArchived)(nil)
	_ ThreadState = (*StateTrash)(nil)
)

// StateUnread is the initial state for a new message recipient.
type StateUnread struct{}

func (*StateUnread) isThreadState()   {}
func (*StateUnread) IsTerminal() bool { return false }
func (*StateUnread) String() string   { return "unread" }

// ProcessEvent handles events in the unread state.
func (s *StateUnread) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch e := event.(type) {
	case ReadEvent:
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: e.ReadAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
					ReadAt:    &e.ReadAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "read",
				},
			},
		}, nil

	case StarEvent:
		return &ThreadTransition{
			NextState: &StateStarred{},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "starred",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "starred",
				},
			},
		}, nil

	case SnoozeEvent:
		return &ThreadTransition{
			NextState: &StateSnoozed{SnoozedUntil: e.SnoozedUntil},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:      env.AgentID,
					MessageID:    env.MessageID,
					NewState:     "snoozed",
					SnoozedUntil: &e.SnoozedUntil,
				},
				ScheduleWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					WakeAt:    e.SnoozedUntil,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "snoozed",
				},
			},
		}, nil

	case ArchiveEvent:
		return &ThreadTransition{
			NextState: &StateArchived{},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "archived",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "archived",
				},
			},
		}, nil

	case TrashEvent:
		purgeAt := time.Now().Add(env.TrashRetention)
		return &ThreadTransition{
			NextState: &StateTrash{PurgeAt: purgeAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "trash",
				},
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   purgeAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "trash",
				},
			},
		}, nil

	case AckEvent:
		// Ack from unread implicitly reads the message.
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: e.AckedAt, AckedAt: &e.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
					ReadAt:    &e.AckedAt,
					AckedAt:   &e.AckedAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "unread",
					NewState:  "read",
				},
			},
		}, nil

	case ResumeThreadEvent:
		// On resume, stay in unread state with no outbox events.
		return &ThreadTransition{NextState: s}, nil

	default:
		return nil, fmt.Errorf("unread: unexpected event: %T", event)
	}
}

// StateRead represents a message that has been read by the recipient.
type StateRead struct {
	ReadAt  time.Time
	AckedAt *time.Time
}

func (*StateRead) isThreadState()   {}
func (*StateRead) IsTerminal() bool { return false }
func (*StateRead) String() string   { return "read" }

// ProcessEvent handles events in the read state.
func (s *StateRead) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch e := event.(type) {
	case StarEvent:
		return &ThreadTransition{
			NextState: &StateStarred{ReadAt: s.ReadAt, AckedAt: s.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "starred",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "read",
					NewState:  "starred",
				},
			},
		}, nil

	case SnoozeEvent:
		return &ThreadTransition{
			NextState: &StateSnoozed{
				SnoozedUntil: e.SnoozedUntil,
				WasRead:      true,
				ReadAt:       s.ReadAt,
				AckedAt:      s.AckedAt,
			},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:      env.AgentID,
					MessageID:    env.MessageID,
					NewState:     "snoozed",
					SnoozedUntil: &e.SnoozedUntil,
				},
				ScheduleWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					WakeAt:    e.SnoozedUntil,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "read",
					NewState:  "snoozed",
				},
			},
		}, nil

	case ArchiveEvent:
		return &ThreadTransition{
			NextState: &StateArchived{ReadAt: s.ReadAt, AckedAt: s.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "archived",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "read",
					NewState:  "archived",
				},
			},
		}, nil

	case TrashEvent:
		purgeAt := time.Now().Add(env.TrashRetention)
		return &ThreadTransition{
			NextState: &StateTrash{PurgeAt: purgeAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "trash",
				},
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   purgeAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "read",
					NewState:  "trash",
				},
			},
		}, nil

	case AckEvent:
		// Update acked timestamp.
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: s.ReadAt, AckedAt: &e.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
					AckedAt:   &e.AckedAt,
				},
			},
		}, nil

	case ReadEvent:
		// Already read, no-op.
		return &ThreadTransition{NextState: s}, nil

	case ResumeThreadEvent:
		return &ThreadTransition{NextState: s}, nil

	default:
		return nil, fmt.Errorf("read: unexpected event: %T", event)
	}
}

// StateStarred represents a starred message.
type StateStarred struct {
	ReadAt  time.Time
	AckedAt *time.Time
}

func (*StateStarred) isThreadState()   {}
func (*StateStarred) IsTerminal() bool { return false }
func (*StateStarred) String() string   { return "starred" }

// ProcessEvent handles events in the starred state.
func (s *StateStarred) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch e := event.(type) {
	case UnstarEvent:
		// Return to read state.
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: s.ReadAt, AckedAt: s.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "starred",
					NewState:  "read",
				},
			},
		}, nil

	case ArchiveEvent:
		return &ThreadTransition{
			NextState: &StateArchived{
				ReadAt:     s.ReadAt,
				AckedAt:    s.AckedAt,
				WasStarred: true,
			},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "archived",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "starred",
					NewState:  "archived",
				},
			},
		}, nil

	case TrashEvent:
		purgeAt := time.Now().Add(env.TrashRetention)
		return &ThreadTransition{
			NextState: &StateTrash{PurgeAt: purgeAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "trash",
				},
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   purgeAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "starred",
					NewState:  "trash",
				},
			},
		}, nil

	case AckEvent:
		return &ThreadTransition{
			NextState: &StateStarred{ReadAt: s.ReadAt, AckedAt: &e.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "starred",
					AckedAt:   &e.AckedAt,
				},
			},
		}, nil

	case ReadEvent, StarEvent:
		// Already starred/read, no-op.
		return &ThreadTransition{NextState: s}, nil

	case ResumeThreadEvent:
		return &ThreadTransition{NextState: s}, nil

	default:
		return nil, fmt.Errorf("starred: unexpected event: %T", event)
	}
}

// StateSnoozed represents a snoozed message that will wake at a specified time.
type StateSnoozed struct {
	SnoozedUntil time.Time
	WasRead      bool
	ReadAt       time.Time
	AckedAt      *time.Time
}

func (*StateSnoozed) isThreadState()   {}
func (*StateSnoozed) IsTerminal() bool { return false }
func (*StateSnoozed) String() string   { return "snoozed" }

// ProcessEvent handles events in the snoozed state.
func (s *StateSnoozed) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch e := event.(type) {
	case WakeEvent:
		// Return to unread state to bring attention back.
		return &ThreadTransition{
			NextState: &StateUnread{},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "unread",
				},
				CancelScheduledWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "snoozed",
					NewState:  "unread",
				},
			},
		}, nil

	case ReadEvent:
		// Reading cancels snooze and marks as read.
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: e.ReadAt, AckedAt: s.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
					ReadAt:    &e.ReadAt,
				},
				CancelScheduledWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "snoozed",
					NewState:  "read",
				},
			},
		}, nil

	case SnoozeEvent:
		// Re-snooze with new time.
		return &ThreadTransition{
			NextState: &StateSnoozed{
				SnoozedUntil: e.SnoozedUntil,
				WasRead:      s.WasRead,
				ReadAt:       s.ReadAt,
				AckedAt:      s.AckedAt,
			},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:      env.AgentID,
					MessageID:    env.MessageID,
					NewState:     "snoozed",
					SnoozedUntil: &e.SnoozedUntil,
				},
				CancelScheduledWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
				},
				ScheduleWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					WakeAt:    e.SnoozedUntil,
				},
			},
		}, nil

	case TrashEvent:
		purgeAt := time.Now().Add(env.TrashRetention)
		return &ThreadTransition{
			NextState: &StateTrash{PurgeAt: purgeAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "trash",
				},
				CancelScheduledWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
				},
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   purgeAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "snoozed",
					NewState:  "trash",
				},
			},
		}, nil

	case ResumeThreadEvent:
		// On resume, re-schedule the wake event.
		return &ThreadTransition{
			NextState: s,
			OutboxEvents: []ThreadOutboxEvent{
				ScheduleWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					WakeAt:    s.SnoozedUntil,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("snoozed: unexpected event: %T", event)
	}
}

// StateArchived represents an archived message.
type StateArchived struct {
	ReadAt     time.Time
	AckedAt    *time.Time
	WasStarred bool
}

func (*StateArchived) isThreadState()   {}
func (*StateArchived) IsTerminal() bool { return false }
func (*StateArchived) String() string   { return "archived" }

// ProcessEvent handles events in the archived state.
func (s *StateArchived) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch event.(type) {
	case UnarchiveEvent:
		// Restore to read state.
		return &ThreadTransition{
			NextState: &StateRead{ReadAt: s.ReadAt, AckedAt: s.AckedAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "read",
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "archived",
					NewState:  "read",
				},
			},
		}, nil

	case TrashEvent:
		purgeAt := time.Now().Add(env.TrashRetention)
		return &ThreadTransition{
			NextState: &StateTrash{PurgeAt: purgeAt},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "trash",
				},
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   purgeAt,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "archived",
					NewState:  "trash",
				},
			},
		}, nil

	case ResumeThreadEvent:
		return &ThreadTransition{NextState: s}, nil

	default:
		return nil, fmt.Errorf("archived: unexpected event: %T", event)
	}
}

// StateTrash represents a trashed message pending permanent deletion.
type StateTrash struct {
	PurgeAt time.Time
}

func (*StateTrash) isThreadState()   {}
func (*StateTrash) IsTerminal() bool { return true }
func (*StateTrash) String() string   { return "trash" }

// ProcessEvent handles events in the trash state.
func (s *StateTrash) ProcessEvent(ctx context.Context, event ThreadEvent,
	env *ThreadEnvironment,
) (*ThreadTransition, error) {
	switch event.(type) {
	case RestoreEvent:
		// Restore to unread state.
		return &ThreadTransition{
			NextState: &StateUnread{},
			OutboxEvents: []ThreadOutboxEvent{
				PersistStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					NewState:  "unread",
				},
				CancelScheduledWake{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
				},
				NotifyStateChange{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					ThreadID:  env.ThreadID,
					OldState:  "trash",
					NewState:  "unread",
				},
			},
		}, nil

	case ResumeThreadEvent:
		// On resume, re-schedule purge.
		return &ThreadTransition{
			NextState: s,
			OutboxEvents: []ThreadOutboxEvent{
				SchedulePurge{
					AgentID:   env.AgentID,
					MessageID: env.MessageID,
					PurgeAt:   s.PurgeAt,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("trash: unexpected event: %T", event)
	}
}

// StateFromString converts a database state string to a ThreadState.
func StateFromString(state string) ThreadState {
	switch state {
	case "unread":
		return &StateUnread{}
	case "read":
		return &StateRead{}
	case "starred":
		return &StateStarred{}
	case "snoozed":
		return &StateSnoozed{}
	case "archived":
		return &StateArchived{}
	case "trash":
		return &StateTrash{}
	default:
		return &StateUnread{}
	}
}
