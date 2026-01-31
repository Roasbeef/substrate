// Package activity provides an actor-based service for recording and querying
// activity events in the Subtrate system.
package activity

import (
	"time"

	"github.com/lightninglabs/darepo-client/baselib/actor"
)

// ActivityServiceKey is the service key for the activity service actor.
var ActivityServiceKey = actor.NewServiceKey[ActivityRequest, ActivityResponse](
	"activity-service",
)

// ActivityRequest is the sealed interface for activity service requests.
type ActivityRequest interface {
	actor.Message
	isActivityRequest()
}

// ActivityResponse is the sealed interface for activity service responses.
type ActivityResponse interface {
	isActivityResponse()
}

// RecordActivityRequest records a new activity event.
type RecordActivityRequest struct {
	actor.BaseMessage

	// AgentID is the agent that performed the activity.
	AgentID int64

	// ActivityType is the type of activity (e.g., "message_sent",
	// "message_read").
	ActivityType string

	// Description is a human-readable description of the activity.
	Description string

	// Metadata is optional JSON metadata about the activity.
	Metadata string
}

// MessageType implements actor.Message.
func (RecordActivityRequest) MessageType() string { return "RecordActivityRequest" }
func (RecordActivityRequest) isActivityRequest()  {}

// RecordActivityResponse is the response to a RecordActivityRequest.
type RecordActivityResponse struct {
	Error error
}

func (RecordActivityResponse) isActivityResponse() {}

// ListRecentRequest lists recent activities.
type ListRecentRequest struct {
	actor.BaseMessage

	// Limit is the maximum number of activities to return.
	Limit int
}

// MessageType implements actor.Message.
func (ListRecentRequest) MessageType() string { return "ListRecentRequest" }
func (ListRecentRequest) isActivityRequest()  {}

// Activity represents a single activity event.
type Activity struct {
	ID           int64
	AgentID      int64
	AgentName    string
	ActivityType string
	Description  string
	Metadata     string
	CreatedAt    time.Time
}

// ListRecentResponse is the response to a ListRecentRequest.
type ListRecentResponse struct {
	Activities []Activity
	Error      error
}

func (ListRecentResponse) isActivityResponse() {}

// ListByAgentRequest lists activities for a specific agent.
type ListByAgentRequest struct {
	actor.BaseMessage

	// AgentID is the agent to filter by.
	AgentID int64

	// Limit is the maximum number of activities to return.
	Limit int
}

// MessageType implements actor.Message.
func (ListByAgentRequest) MessageType() string { return "ListByAgentRequest" }
func (ListByAgentRequest) isActivityRequest()  {}

// ListByAgentResponse is the response to a ListByAgentRequest.
type ListByAgentResponse struct {
	Activities []Activity
	Error      error
}

func (ListByAgentResponse) isActivityResponse() {}

// ListSinceRequest lists activities since a given timestamp.
type ListSinceRequest struct {
	actor.BaseMessage

	// Since is the earliest timestamp to include.
	Since time.Time

	// Limit is the maximum number of activities to return.
	Limit int
}

// MessageType implements actor.Message.
func (ListSinceRequest) MessageType() string { return "ListSinceRequest" }
func (ListSinceRequest) isActivityRequest()  {}

// ListSinceResponse is the response to a ListSinceRequest.
type ListSinceResponse struct {
	Activities []Activity
	Error      error
}

func (ListSinceResponse) isActivityResponse() {}

// CleanupRequest removes old activities.
type CleanupRequest struct {
	actor.BaseMessage

	// OlderThan is the cutoff time for deletion.
	OlderThan time.Time
}

// MessageType implements actor.Message.
func (CleanupRequest) MessageType() string { return "CleanupRequest" }
func (CleanupRequest) isActivityRequest()  {}

// CleanupResponse is the response to a CleanupRequest.
type CleanupResponse struct {
	Error error
}

func (CleanupResponse) isActivityResponse() {}
