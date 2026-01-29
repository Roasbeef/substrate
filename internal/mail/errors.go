package mail

import "errors"

var (
	// ErrUnknownRequestType is returned when an actor receives an unknown
	// message type.
	ErrUnknownRequestType = errors.New("unknown request type")

	// ErrAgentNotFound is returned when an agent cannot be found.
	ErrAgentNotFound = errors.New("agent not found")

	// ErrMessageNotFound is returned when a message cannot be found.
	ErrMessageNotFound = errors.New("message not found")

	// ErrTopicNotFound is returned when a topic cannot be found.
	ErrTopicNotFound = errors.New("topic not found")

	// ErrUnauthorized is returned when an agent is not authorized to
	// perform an operation.
	ErrUnauthorized = errors.New("unauthorized")
)
