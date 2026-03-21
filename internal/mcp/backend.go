package mcp

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// Backend abstracts the operations needed by MCP tool handlers. It decouples
// the tool handlers from their data source, allowing both direct DB access
// (via DirectBackend) and gRPC proxy (via GRPCBackend) implementations.
type Backend interface {
	// SendMail sends a message to one or more recipients.
	SendMail(ctx context.Context,
		req mail.SendMailRequest,
	) (mail.SendMailResponse, error)

	// FetchInbox retrieves inbox messages for an agent.
	FetchInbox(ctx context.Context,
		req mail.FetchInboxRequest,
	) (mail.FetchInboxResponse, error)

	// ReadMessage reads a specific message and marks it as read.
	ReadMessage(ctx context.Context,
		agentID, messageID int64,
	) (mail.ReadMessageResponse, error)

	// AckMessage acknowledges receipt of a message.
	AckMessage(ctx context.Context,
		agentID, messageID int64,
	) (mail.AckMessageResponse, error)

	// UpdateState changes the state of a message.
	UpdateState(ctx context.Context,
		agentID, messageID int64, newState string,
		snoozedUntil *time.Time,
	) (mail.UpdateStateResponse, error)

	// GetStatus returns the mail status summary for an agent.
	GetStatus(ctx context.Context,
		agentID int64,
	) (mail.GetStatusResponse, error)

	// PollChanges checks for new messages since the given offsets.
	PollChanges(ctx context.Context,
		agentID int64, sinceOffsets map[int64]int64,
	) (mail.PollChangesResponse, error)

	// Publish sends a message to a topic.
	Publish(ctx context.Context,
		req mail.PublishRequest,
	) (mail.PublishResponse, error)

	// GetTopicByName retrieves a topic by its name.
	GetTopicByName(ctx context.Context,
		name string,
	) (store.Topic, error)

	// ListTopics returns all available topics.
	ListTopics(ctx context.Context) ([]store.Topic, error)

	// ListSubscriptionsByAgent returns topics an agent is subscribed to.
	ListSubscriptionsByAgent(ctx context.Context,
		agentID int64,
	) ([]store.Topic, error)

	// CreateSubscription subscribes an agent to a topic.
	CreateSubscription(ctx context.Context,
		agentID, topicID int64,
	) error

	// DeleteSubscription removes an agent's subscription to a topic.
	DeleteSubscription(ctx context.Context,
		agentID, topicID int64,
	) error

	// SearchMessages performs full-text search across messages for an agent.
	SearchMessages(ctx context.Context,
		query string, agentID int64, limit int,
	) ([]store.Message, error)

	// RegisterAgent creates a new agent with the given name.
	RegisterAgent(ctx context.Context,
		name, projectKey, gitBranch string,
	) (*sqlc.Agent, error)

	// GetAgent retrieves an agent by ID.
	GetAgent(ctx context.Context,
		agentID int64,
	) (store.Agent, error)
}
