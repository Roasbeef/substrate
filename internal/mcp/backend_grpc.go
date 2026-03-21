package mcp

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
)

// GRPCBackend implements Backend by proxying all operations through gRPC
// to a running substrated daemon. This is used by the `substrate mcp serve`
// command to expose an MCP server that delegates to the daemon.
type GRPCBackend struct {
	conn        *grpc.ClientConn
	mailClient  subtraterpc.MailClient
	agentClient subtraterpc.AgentClient
}

// NewGRPCBackend creates a new GRPCBackend from an established connection.
func NewGRPCBackend(conn *grpc.ClientConn) *GRPCBackend {
	return &GRPCBackend{
		conn:        conn,
		mailClient:  subtraterpc.NewMailClient(conn),
		agentClient: subtraterpc.NewAgentClient(conn),
	}
}

// SendMail sends a message via the gRPC daemon.
func (b *GRPCBackend) SendMail(ctx context.Context,
	req mail.SendMailRequest,
) (mail.SendMailResponse, error) {
	resp, err := b.mailClient.SendMail(ctx, &subtraterpc.SendMailRequest{
		SenderId:       req.SenderID,
		RecipientNames: req.RecipientNames,
		TopicName:      req.TopicName,
		ThreadId:       req.ThreadID,
		Subject:        req.Subject,
		Body:           req.Body,
		Priority:       priorityToProto(req.Priority),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return mail.SendMailResponse{}, err
	}

	return mail.SendMailResponse{
		MessageID: resp.MessageId,
		ThreadID:  resp.ThreadId,
	}, nil
}

// FetchInbox retrieves inbox messages via the gRPC daemon.
func (b *GRPCBackend) FetchInbox(ctx context.Context,
	req mail.FetchInboxRequest,
) (mail.FetchInboxResponse, error) {
	resp, err := b.mailClient.FetchInbox(
		ctx, &subtraterpc.FetchInboxRequest{
			AgentId:    req.AgentID,
			Limit:      int32(req.Limit),
			UnreadOnly: req.UnreadOnly,
		},
	)
	if err != nil {
		return mail.FetchInboxResponse{}, err
	}

	return mail.FetchInboxResponse{
		Messages: protoToInboxMessages(resp.Messages),
	}, nil
}

// ReadMessage reads a message and marks it as read via the gRPC daemon.
func (b *GRPCBackend) ReadMessage(ctx context.Context,
	agentID, messageID int64,
) (mail.ReadMessageResponse, error) {
	resp, err := b.mailClient.ReadMessage(
		ctx, &subtraterpc.ReadMessageRequest{
			AgentId:   agentID,
			MessageId: messageID,
		},
	)
	if err != nil {
		return mail.ReadMessageResponse{}, err
	}

	if resp.Message == nil {
		return mail.ReadMessageResponse{}, nil
	}

	msg := protoToInboxMessage(resp.Message)
	return mail.ReadMessageResponse{Message: &msg}, nil
}

// AckMessage acknowledges receipt of a message via the gRPC daemon.
func (b *GRPCBackend) AckMessage(ctx context.Context,
	agentID, messageID int64,
) (mail.AckMessageResponse, error) {
	_, err := b.mailClient.AckMessage(
		ctx, &subtraterpc.AckMessageRequest{
			AgentId:   agentID,
			MessageId: messageID,
		},
	)
	if err != nil {
		return mail.AckMessageResponse{}, err
	}

	return mail.AckMessageResponse{Success: true}, nil
}

// UpdateState changes the state of a message via the gRPC daemon.
func (b *GRPCBackend) UpdateState(ctx context.Context,
	agentID, messageID int64, newState string,
	snoozedUntil *time.Time,
) (mail.UpdateStateResponse, error) {
	grpcReq := &subtraterpc.UpdateStateRequest{
		AgentId:   agentID,
		MessageId: messageID,
		NewState:  stateToProto(newState),
	}
	if snoozedUntil != nil {
		grpcReq.SnoozedUntil = timestamppb.New(*snoozedUntil)
	}

	_, err := b.mailClient.UpdateState(ctx, grpcReq)
	if err != nil {
		return mail.UpdateStateResponse{}, err
	}

	return mail.UpdateStateResponse{Success: true}, nil
}

// GetStatus returns the mail status summary via the gRPC daemon.
func (b *GRPCBackend) GetStatus(ctx context.Context,
	agentID int64,
) (mail.GetStatusResponse, error) {
	resp, err := b.mailClient.GetStatus(
		ctx, &subtraterpc.GetStatusRequest{
			AgentId: agentID,
		},
	)
	if err != nil {
		return mail.GetStatusResponse{}, err
	}

	return mail.GetStatusResponse{
		Status: mail.AgentStatus{
			AgentID:      resp.AgentId,
			AgentName:    resp.AgentName,
			UnreadCount:  resp.UnreadCount,
			UrgentCount:  resp.UrgentCount,
			StarredCount: resp.StarredCount,
			SnoozedCount: resp.SnoozedCount,
		},
	}, nil
}

// PollChanges checks for new messages via the gRPC daemon.
func (b *GRPCBackend) PollChanges(ctx context.Context,
	agentID int64, sinceOffsets map[int64]int64,
) (mail.PollChangesResponse, error) {
	resp, err := b.mailClient.PollChanges(
		ctx, &subtraterpc.PollChangesRequest{
			AgentId:      agentID,
			SinceOffsets: sinceOffsets,
		},
	)
	if err != nil {
		return mail.PollChangesResponse{}, err
	}

	return mail.PollChangesResponse{
		NewMessages: protoToInboxMessages(resp.NewMessages),
		NewOffsets:  resp.NewOffsets,
	}, nil
}

// Publish sends a message to a topic via the gRPC daemon.
func (b *GRPCBackend) Publish(ctx context.Context,
	req mail.PublishRequest,
) (mail.PublishResponse, error) {
	resp, err := b.mailClient.Publish(
		ctx, &subtraterpc.PublishRequest{
			SenderId:  req.SenderID,
			TopicName: req.TopicName,
			Subject:   req.Subject,
			Body:      req.Body,
			Priority:  priorityToProto(req.Priority),
		},
	)
	if err != nil {
		return mail.PublishResponse{}, err
	}

	return mail.PublishResponse{
		MessageID:       resp.MessageId,
		RecipientsCount: int(resp.RecipientsCount),
	}, nil
}

// GetTopicByName retrieves a topic by name. Since there is no dedicated
// gRPC RPC for this, we list all topics and filter by name.
func (b *GRPCBackend) GetTopicByName(ctx context.Context,
	name string,
) (store.Topic, error) {
	resp, err := b.mailClient.ListTopics(
		ctx, &subtraterpc.ListTopicsRequest{},
	)
	if err != nil {
		return store.Topic{}, err
	}

	for _, t := range resp.Topics {
		if t.Name == name {
			return protoToStoreTopic(t), nil
		}
	}

	return store.Topic{}, fmt.Errorf("topic %q not found", name)
}

// ListTopics returns all available topics via the gRPC daemon.
func (b *GRPCBackend) ListTopics(
	ctx context.Context,
) ([]store.Topic, error) {
	resp, err := b.mailClient.ListTopics(
		ctx, &subtraterpc.ListTopicsRequest{},
	)
	if err != nil {
		return nil, err
	}

	topics := make([]store.Topic, len(resp.Topics))
	for i, t := range resp.Topics {
		topics[i] = protoToStoreTopic(t)
	}

	return topics, nil
}

// ListSubscriptionsByAgent returns subscribed topics via the gRPC daemon.
func (b *GRPCBackend) ListSubscriptionsByAgent(ctx context.Context,
	agentID int64,
) ([]store.Topic, error) {
	resp, err := b.mailClient.ListTopics(
		ctx, &subtraterpc.ListTopicsRequest{
			AgentId:        agentID,
			SubscribedOnly: true,
		},
	)
	if err != nil {
		return nil, err
	}

	topics := make([]store.Topic, len(resp.Topics))
	for i, t := range resp.Topics {
		topics[i] = protoToStoreTopic(t)
	}

	return topics, nil
}

// CreateSubscription subscribes an agent to a topic via the gRPC daemon.
func (b *GRPCBackend) CreateSubscription(ctx context.Context,
	agentID, topicID int64,
) error {
	// The gRPC Subscribe RPC takes a topic name, but we have an ID.
	// Look up the topic name first.
	resp, err := b.mailClient.GetTopic(
		ctx, &subtraterpc.GetTopicRequest{TopicId: topicID},
	)
	if err != nil {
		return fmt.Errorf("get topic %d: %w", topicID, err)
	}

	_, err = b.mailClient.Subscribe(
		ctx, &subtraterpc.SubscribeRequest{
			AgentId:   agentID,
			TopicName: resp.Topic.Name,
		},
	)

	return err
}

// DeleteSubscription removes a subscription via the gRPC daemon.
func (b *GRPCBackend) DeleteSubscription(ctx context.Context,
	agentID, topicID int64,
) error {
	// The gRPC Unsubscribe RPC takes a topic name, but we have an ID.
	resp, err := b.mailClient.GetTopic(
		ctx, &subtraterpc.GetTopicRequest{TopicId: topicID},
	)
	if err != nil {
		return fmt.Errorf("get topic %d: %w", topicID, err)
	}

	_, err = b.mailClient.Unsubscribe(
		ctx, &subtraterpc.UnsubscribeRequest{
			AgentId:   agentID,
			TopicName: resp.Topic.Name,
		},
	)

	return err
}

// SearchMessages searches messages via the gRPC daemon.
func (b *GRPCBackend) SearchMessages(ctx context.Context,
	query string, agentID int64, limit int,
) ([]store.Message, error) {
	resp, err := b.mailClient.Search(ctx, &subtraterpc.SearchRequest{
		AgentId: agentID,
		Query:   query,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]store.Message, len(resp.Results))
	for i, m := range resp.Results {
		messages[i] = store.Message{
			ID:       m.Id,
			ThreadID: m.ThreadId,
			TopicID:  m.TopicId,
			SenderID: m.SenderId,
			Subject:  m.Subject,
			Body:     m.Body,
			Priority: protoToPriorityString(m.Priority),
			CreatedAt: func() time.Time {
				if m.CreatedAt != nil {
					return m.CreatedAt.AsTime()
				}
				return time.Time{}
			}(),
		}
	}

	return messages, nil
}

// RegisterAgent creates a new agent via the gRPC daemon.
func (b *GRPCBackend) RegisterAgent(ctx context.Context,
	name, projectKey, gitBranch string,
) (*sqlc.Agent, error) {
	resp, err := b.agentClient.RegisterAgent(
		ctx, &subtraterpc.RegisterAgentRequest{
			Name:       name,
			ProjectKey: projectKey,
		},
	)
	if err != nil {
		return nil, err
	}

	return &sqlc.Agent{
		ID:   resp.AgentId,
		Name: resp.Name,
	}, nil
}

// GetAgent retrieves an agent by ID via the gRPC daemon.
func (b *GRPCBackend) GetAgent(ctx context.Context,
	agentID int64,
) (store.Agent, error) {
	resp, err := b.agentClient.GetAgent(
		ctx, &subtraterpc.GetAgentRequest{AgentId: agentID},
	)
	if err != nil {
		return store.Agent{}, err
	}

	ag := store.Agent{
		ID:         resp.Id,
		Name:       resp.Name,
		ProjectKey: resp.ProjectKey,
		GitBranch:  resp.GitBranch,
	}

	if resp.LastActiveAt != nil {
		ag.LastActiveAt = resp.LastActiveAt.AsTime()
	}

	return ag, nil
}

// protoToStoreTopic converts a proto Topic to a store.Topic.
func protoToStoreTopic(t *subtraterpc.Topic) store.Topic {
	topic := store.Topic{
		ID:           t.Id,
		Name:         t.Name,
		TopicType:    t.TopicType,
		MessageCount: t.MessageCount,
	}

	if t.CreatedAt != nil {
		topic.CreatedAt = t.CreatedAt.AsTime()
	}

	return topic
}

// protoToInboxMessages converts proto messages to mail.InboxMessage slice.
func protoToInboxMessages(
	msgs []*subtraterpc.InboxMessage,
) []mail.InboxMessage {
	result := make([]mail.InboxMessage, len(msgs))
	for i, m := range msgs {
		result[i] = protoToInboxMessage(m)
	}

	return result
}

// protoToInboxMessage converts a single proto message to mail.InboxMessage.
func protoToInboxMessage(m *subtraterpc.InboxMessage) mail.InboxMessage {
	msg := mail.InboxMessage{
		ID:         m.Id,
		ThreadID:   m.ThreadId,
		TopicID:    m.TopicId,
		SenderID:   m.SenderId,
		SenderName: m.SenderName,
		Subject:    m.Subject,
		Body:       m.Body,
		Priority:   protoToPriority(m.Priority),
		State:      protoToState(m.State),
	}

	if m.CreatedAt != nil {
		msg.CreatedAt = m.CreatedAt.AsTime()
	}
	if m.DeadlineAt != nil && m.DeadlineAt.IsValid() {
		t := m.DeadlineAt.AsTime()
		msg.Deadline = &t
	}
	if m.SnoozedUntil != nil && m.SnoozedUntil.IsValid() {
		t := m.SnoozedUntil.AsTime()
		msg.SnoozedUntil = &t
	}
	if m.ReadAt != nil && m.ReadAt.IsValid() {
		t := m.ReadAt.AsTime()
		msg.ReadAt = &t
	}
	if m.AcknowledgedAt != nil && m.AcknowledgedAt.IsValid() {
		t := m.AcknowledgedAt.AsTime()
		msg.AckedAt = &t
	}

	return msg
}

// priorityToProto converts a mail.Priority to a proto Priority.
func priorityToProto(p mail.Priority) subtraterpc.Priority {
	switch p {
	case mail.PriorityLow:
		return subtraterpc.Priority_PRIORITY_LOW
	case mail.PriorityNormal:
		return subtraterpc.Priority_PRIORITY_NORMAL
	case mail.PriorityUrgent:
		return subtraterpc.Priority_PRIORITY_URGENT
	default:
		return subtraterpc.Priority_PRIORITY_NORMAL
	}
}

// protoToPriority converts a proto Priority to a mail.Priority.
func protoToPriority(p subtraterpc.Priority) mail.Priority {
	switch p {
	case subtraterpc.Priority_PRIORITY_LOW:
		return mail.PriorityLow
	case subtraterpc.Priority_PRIORITY_NORMAL:
		return mail.PriorityNormal
	case subtraterpc.Priority_PRIORITY_URGENT:
		return mail.PriorityUrgent
	default:
		return mail.PriorityNormal
	}
}

// protoToPriorityString converts a proto Priority to a string.
func protoToPriorityString(p subtraterpc.Priority) string {
	return string(protoToPriority(p))
}

// stateToProto converts a state string to a proto MessageState.
func stateToProto(s string) subtraterpc.MessageState {
	switch s {
	case "unread":
		return subtraterpc.MessageState_STATE_UNREAD
	case "read":
		return subtraterpc.MessageState_STATE_READ
	case "starred":
		return subtraterpc.MessageState_STATE_STARRED
	case "snoozed":
		return subtraterpc.MessageState_STATE_SNOOZED
	case "archived":
		return subtraterpc.MessageState_STATE_ARCHIVED
	case "trash":
		return subtraterpc.MessageState_STATE_TRASH
	default:
		return subtraterpc.MessageState_STATE_UNSPECIFIED
	}
}

// protoToState converts a proto MessageState to a string.
func protoToState(s subtraterpc.MessageState) string {
	switch s {
	case subtraterpc.MessageState_STATE_UNREAD:
		return "unread"
	case subtraterpc.MessageState_STATE_READ:
		return "read"
	case subtraterpc.MessageState_STATE_STARRED:
		return "starred"
	case subtraterpc.MessageState_STATE_SNOOZED:
		return "snoozed"
	case subtraterpc.MessageState_STATE_ARCHIVED:
		return "archived"
	case subtraterpc.MessageState_STATE_TRASH:
		return "trash"
	default:
		return "unread"
	}
}

// Compile-time interface check.
var _ Backend = (*GRPCBackend)(nil)
