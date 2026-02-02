package mail

import (
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// Priority represents the priority level of a message.
type Priority string

const (
	// PriorityUrgent indicates a high-priority message that should be
	// processed immediately.
	PriorityUrgent Priority = "urgent"

	// PriorityNormal indicates a standard priority message.
	PriorityNormal Priority = "normal"

	// PriorityLow indicates a low-priority message that can be deferred.
	PriorityLow Priority = "low"
)

// RecipientState represents the state of a message for a recipient.
type RecipientState string

const (
	// StateUnreadStr indicates the message has not been read.
	StateUnreadStr RecipientState = "unread"

	// StateReadStr indicates the message has been read.
	StateReadStr RecipientState = "read"

	// StateStarredStr indicates the message has been starred/favorited.
	StateStarredStr RecipientState = "starred"

	// StateSnoozedStr indicates the message has been snoozed until later.
	StateSnoozedStr RecipientState = "snoozed"

	// StateArchivedStr indicates the message has been archived.
	StateArchivedStr RecipientState = "archived"

	// StateTrashStr indicates the message has been moved to trash.
	StateTrashStr RecipientState = "trash"
)

// String returns the string representation of the state.
func (s RecipientState) String() string {
	return string(s)
}

// IsValid returns true if the state is a recognized value.
func (s RecipientState) IsValid() bool {
	switch s {
	case StateUnreadStr, StateReadStr, StateStarredStr,
		StateSnoozedStr, StateArchivedStr, StateTrashStr:
		return true
	default:
		return false
	}
}

// MessageAction represents an action that can be performed on a message.
type MessageAction string

const (
	// ActionStar stars a message.
	ActionStar MessageAction = "star"

	// ActionArchive archives a message.
	ActionArchive MessageAction = "archive"

	// ActionSnooze snoozes a message.
	ActionSnooze MessageAction = "snooze"

	// ActionAck acknowledges a message.
	ActionAck MessageAction = "ack"

	// ActionRead marks a message as read.
	ActionRead MessageAction = "read"

	// ActionUnread marks a message as unread.
	ActionUnread MessageAction = "unread"

	// ActionDelete deletes a message (moves to trash).
	ActionDelete MessageAction = "delete"
)

// String returns the string representation of the action.
func (a MessageAction) String() string {
	return string(a)
}

// IsValid returns true if the action is a recognized value.
func (a MessageAction) IsValid() bool {
	switch a {
	case ActionStar, ActionArchive, ActionSnooze,
		ActionAck, ActionRead, ActionUnread, ActionDelete:
		return true
	default:
		return false
	}
}

// SendMailRequest is an actor message requesting to send mail.
type SendMailRequest struct {
	actor.BaseMessage

	// SenderID is the database ID of the sending agent.
	SenderID int64

	// RecipientNames is a list of agent names to receive the message.
	RecipientNames []string

	// TopicName is the topic to publish to (optional, for pub/sub).
	TopicName string

	// Subject is the message subject line.
	Subject string

	// Body is the message body in markdown format.
	Body string

	// Priority indicates the urgency of the message.
	Priority Priority

	// Deadline is an optional deadline for acknowledgment.
	Deadline *time.Time

	// ThreadID is an optional thread ID for threading messages together.
	// If empty, a new thread ID will be generated.
	ThreadID string

	// Attachments is optional JSON-encoded attachment data.
	Attachments string
}

// MessageType implements actor.Message.
func (SendMailRequest) MessageType() string { return "SendMailRequest" }

// SendMailResponse is the response to a SendMailRequest.
type SendMailResponse struct {
	// MessageID is the database ID of the created message.
	MessageID int64

	// ThreadID is the thread ID of the message.
	ThreadID string

	// Error is any error that occurred during sending.
	Error error
}

// FetchInboxRequest is an actor message requesting inbox messages.
type FetchInboxRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the agent.
	AgentID int64

	// Limit is the maximum number of messages to return.
	Limit int

	// UnreadOnly filters to only unread messages.
	UnreadOnly bool

	// IncludeArchived includes archived messages.
	IncludeArchived bool

	// StateFilter filters messages by state (e.g., "unread", "starred").
	// If nil, no state filtering is applied.
	StateFilter *string
}

// MessageType implements actor.Message.
func (FetchInboxRequest) MessageType() string { return "FetchInboxRequest" }

// InboxMessage represents a message in the inbox with its state.
type InboxMessage struct {
	ID           int64
	ThreadID     string
	TopicID      int64
	SenderID     int64
	SenderName   string
	Subject      string
	Body         string
	Priority     Priority
	Deadline     *time.Time
	State        string
	SnoozedUntil *time.Time
	ReadAt       *time.Time
	AckedAt      *time.Time
	CreatedAt    time.Time
}

// FetchInboxResponse is the response to a FetchInboxRequest.
type FetchInboxResponse struct {
	Messages []InboxMessage
	Error    error
}

// ReadMessageRequest is an actor message requesting to read a message.
type ReadMessageRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the requesting agent.
	AgentID int64

	// MessageID is the database ID of the message to read.
	MessageID int64
}

// MessageType implements actor.Message.
func (ReadMessageRequest) MessageType() string { return "ReadMessageRequest" }

// ReadMessageResponse is the response to a ReadMessageRequest.
type ReadMessageResponse struct {
	Message *InboxMessage
	Error   error
}

// UpdateStateRequest is an actor message to update message state.
type UpdateStateRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the agent.
	AgentID int64

	// MessageID is the database ID of the message.
	MessageID int64

	// NewState is the target state (read, starred, snoozed, archived, trash).
	NewState string

	// SnoozedUntil is required when NewState is "snoozed".
	SnoozedUntil *time.Time
}

// MessageType implements actor.Message.
func (UpdateStateRequest) MessageType() string { return "UpdateStateRequest" }

// UpdateStateResponse is the response to an UpdateStateRequest.
type UpdateStateResponse struct {
	Success bool
	Error   error
}

// AckMessageRequest is an actor message to acknowledge receipt.
type AckMessageRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the agent.
	AgentID int64

	// MessageID is the database ID of the message.
	MessageID int64
}

// MessageType implements actor.Message.
func (AckMessageRequest) MessageType() string { return "AckMessageRequest" }

// AckMessageResponse is the response to an AckMessageRequest.
type AckMessageResponse struct {
	Success bool
	Error   error
}

// GetStatusRequest is an actor message requesting agent status.
type GetStatusRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the agent.
	AgentID int64
}

// MessageType implements actor.Message.
func (GetStatusRequest) MessageType() string { return "GetStatusRequest" }

// AgentStatus represents the mail status for an agent.
type AgentStatus struct {
	AgentID      int64
	AgentName    string
	UnreadCount  int64
	UrgentCount  int64
	StarredCount int64
	SnoozedCount int64
}

// GetStatusResponse is the response to a GetStatusRequest.
type GetStatusResponse struct {
	Status AgentStatus
	Error  error
}

// PollChangesRequest is an actor message for polling new messages.
type PollChangesRequest struct {
	actor.BaseMessage

	// AgentID is the database ID of the agent.
	AgentID int64

	// SinceOffset is the last seen log offset per topic.
	SinceOffsets map[int64]int64
}

// MessageType implements actor.Message.
func (PollChangesRequest) MessageType() string { return "PollChangesRequest" }

// PollChangesResponse is the response to a PollChangesRequest.
type PollChangesResponse struct {
	// NewMessages contains messages since the provided offsets.
	NewMessages []InboxMessage

	// NewOffsets contains the latest offset per topic.
	NewOffsets map[int64]int64

	Error error
}

// PublishRequest is an actor message for publishing to a topic.
type PublishRequest struct {
	actor.BaseMessage

	// SenderID is the database ID of the sending agent.
	SenderID int64

	// TopicName is the topic to publish to.
	TopicName string

	// Subject is the message subject line.
	Subject string

	// Body is the message body in markdown format.
	Body string

	// Priority indicates the urgency of the message.
	Priority Priority
}

// MessageType implements actor.Message.
func (PublishRequest) MessageType() string { return "PublishRequest" }

// PublishResponse is the response to a PublishRequest.
type PublishResponse struct {
	// MessageID is the database ID of the created message.
	MessageID int64

	// RecipientsCount is the number of agents that received the message.
	RecipientsCount int

	Error error
}
