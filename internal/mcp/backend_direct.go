package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/actorutil"
	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// DirectBackend implements Backend using direct database access and an
// optional actor system reference. When a MailActorRef is configured,
// mail operations are routed through the actor system for proper
// concurrency control. Otherwise, they fall back to direct service calls.
type DirectBackend struct {
	storage  store.Storage
	mailSvc  *mail.Service
	mailRef  mail.MailActorRef
	registry *agent.Registry
}

// DirectBackendConfig holds configuration for a DirectBackend.
type DirectBackendConfig struct {
	// Storage is the domain storage layer.
	Storage store.Storage

	// MailSvc is the mail service for direct calls.
	MailSvc *mail.Service

	// MailRef is an optional actor reference for mail operations.
	MailRef mail.MailActorRef

	// Registry is the agent registry.
	Registry *agent.Registry
}

// NewDirectBackend creates a new DirectBackend with the given config.
func NewDirectBackend(cfg DirectBackendConfig) *DirectBackend {
	return &DirectBackend{
		storage:  cfg.Storage,
		mailSvc:  cfg.MailSvc,
		mailRef:  cfg.MailRef,
		registry: cfg.Registry,
	}
}

// hasActor returns true if a mail actor ref is configured.
func (b *DirectBackend) hasActor() bool {
	return b.mailRef != nil
}

// SendMail sends a message via the actor system or direct service.
func (b *DirectBackend) SendMail(ctx context.Context,
	req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.SendMailResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.SendMailResponse{}, err
	}

	resp := val.(mail.SendMailResponse)
	if resp.Error != nil {
		return mail.SendMailResponse{}, resp.Error
	}

	return resp, nil
}

// FetchInbox retrieves inbox messages via the actor system or direct service.
func (b *DirectBackend) FetchInbox(ctx context.Context,
	req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.FetchInboxResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.FetchInboxResponse{}, err
	}

	resp := val.(mail.FetchInboxResponse)
	if resp.Error != nil {
		return mail.FetchInboxResponse{}, resp.Error
	}

	return resp, nil
}

// ReadMessage reads a message and marks it as read.
func (b *DirectBackend) ReadMessage(ctx context.Context,
	agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	req := mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	}

	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.ReadMessageResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.ReadMessageResponse{}, err
	}

	resp := val.(mail.ReadMessageResponse)
	if resp.Error != nil {
		return mail.ReadMessageResponse{}, resp.Error
	}

	return resp, nil
}

// AckMessage acknowledges receipt of a message.
func (b *DirectBackend) AckMessage(ctx context.Context,
	agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	req := mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	}

	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.AckMessageResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.AckMessageResponse{}, err
	}

	resp := val.(mail.AckMessageResponse)
	if resp.Error != nil {
		return mail.AckMessageResponse{}, resp.Error
	}

	return resp, nil
}

// UpdateState changes the state of a message.
func (b *DirectBackend) UpdateState(ctx context.Context,
	agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	req := mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    messageID,
		NewState:     newState,
		SnoozedUntil: snoozedUntil,
	}

	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.UpdateStateResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.UpdateStateResponse{}, err
	}

	resp := val.(mail.UpdateStateResponse)
	if resp.Error != nil {
		return mail.UpdateStateResponse{}, resp.Error
	}

	return resp, nil
}

// GetStatus returns the mail status summary for an agent.
func (b *DirectBackend) GetStatus(ctx context.Context,
	agentID int64,
) (mail.GetStatusResponse, error) {
	req := mail.GetStatusRequest{AgentID: agentID}

	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.GetStatusResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.GetStatusResponse{}, err
	}

	resp := val.(mail.GetStatusResponse)
	if resp.Error != nil {
		return mail.GetStatusResponse{}, resp.Error
	}

	return resp, nil
}

// PollChanges checks for new messages since given offsets.
func (b *DirectBackend) PollChanges(ctx context.Context,
	agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	req := mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	}

	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.PollChangesResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.PollChangesResponse{}, err
	}

	resp := val.(mail.PollChangesResponse)
	if resp.Error != nil {
		return mail.PollChangesResponse{}, resp.Error
	}

	return resp, nil
}

// Publish sends a message to a topic.
func (b *DirectBackend) Publish(ctx context.Context,
	req mail.PublishRequest,
) (mail.PublishResponse, error) {
	if b.hasActor() {
		return actorutil.AskAwaitTyped[
			mail.MailRequest, mail.MailResponse,
			mail.PublishResponse,
		](ctx, b.mailRef, req)
	}

	result := b.mailSvc.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return mail.PublishResponse{}, err
	}

	resp := val.(mail.PublishResponse)
	if resp.Error != nil {
		return mail.PublishResponse{}, resp.Error
	}

	return resp, nil
}

// GetTopicByName retrieves a topic by its name.
func (b *DirectBackend) GetTopicByName(ctx context.Context,
	name string,
) (store.Topic, error) {
	return b.storage.GetTopicByName(ctx, name)
}

// ListTopics returns all available topics.
func (b *DirectBackend) ListTopics(
	ctx context.Context,
) ([]store.Topic, error) {
	return b.storage.ListTopics(ctx)
}

// ListSubscriptionsByAgent returns topics an agent is subscribed to.
func (b *DirectBackend) ListSubscriptionsByAgent(ctx context.Context,
	agentID int64,
) ([]store.Topic, error) {
	return b.storage.ListSubscriptionsByAgent(ctx, agentID)
}

// CreateSubscription subscribes an agent to a topic by name.
func (b *DirectBackend) CreateSubscription(ctx context.Context,
	agentID int64, topicName string,
) error {
	topic, err := b.storage.GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found: %w", topicName, err)
	}

	return b.storage.CreateSubscription(ctx, agentID, topic.ID)
}

// DeleteSubscription removes an agent's subscription by topic name.
func (b *DirectBackend) DeleteSubscription(ctx context.Context,
	agentID int64, topicName string,
) error {
	topic, err := b.storage.GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found: %w", topicName, err)
	}

	return b.storage.DeleteSubscription(ctx, agentID, topic.ID)
}

// SearchMessages performs full-text search across messages for an agent.
func (b *DirectBackend) SearchMessages(ctx context.Context,
	query string, agentID int64, limit int,
) ([]store.Message, error) {
	return b.storage.SearchMessagesForAgent(ctx, query, agentID, limit)
}

// RegisterAgent creates a new agent with the given name.
func (b *DirectBackend) RegisterAgent(ctx context.Context,
	name, projectKey, gitBranch string,
) (store.Agent, error) {
	ag, err := b.registry.RegisterAgent(
		ctx, name, projectKey, gitBranch,
	)
	if err != nil {
		return store.Agent{}, err
	}

	return store.AgentFromSqlc(*ag), nil
}

// GetAgent retrieves an agent by ID.
func (b *DirectBackend) GetAgent(ctx context.Context,
	agentID int64,
) (store.Agent, error) {
	return b.storage.GetAgent(ctx, agentID)
}

// Compile-time interface check.
var _ Backend = (*DirectBackend)(nil)
