package mail

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// MailServiceKey is the service key for the mail service actor.
var MailServiceKey = actor.NewServiceKey[MailRequest, MailResponse](
	"mail-service",
)

// MailRequest is the union type for all mail service requests.
type MailRequest interface {
	actor.Message
	isMailRequest()
}

// Ensure all request types implement MailRequest.
func (SendMailRequest) isMailRequest()    {}
func (FetchInboxRequest) isMailRequest()  {}
func (ReadMessageRequest) isMailRequest() {}
func (UpdateStateRequest) isMailRequest() {}
func (AckMessageRequest) isMailRequest()  {}
func (GetStatusRequest) isMailRequest()   {}
func (PollChangesRequest) isMailRequest() {}
func (PublishRequest) isMailRequest()     {}

// MailResponse is the union type for all mail service responses.
type MailResponse interface {
	isMailResponse()
}

// Ensure all response types implement MailResponse.
func (SendMailResponse) isMailResponse()    {}
func (FetchInboxResponse) isMailResponse()  {}
func (ReadMessageResponse) isMailResponse() {}
func (UpdateStateResponse) isMailResponse() {}
func (AckMessageResponse) isMailResponse()  {}
func (GetStatusResponse) isMailResponse()   {}
func (PollChangesResponse) isMailResponse() {}
func (PublishResponse) isMailResponse()     {}

// Service is the mail service actor behavior.
type Service struct {
	store *db.Store
}

// NewService creates a new mail service with the given database store.
func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (s *Service) Receive(ctx context.Context,
	msg MailRequest) fn.Result[MailResponse] {

	switch m := msg.(type) {
	case SendMailRequest:
		resp := s.handleSendMail(ctx, m)
		return fn.Ok[MailResponse](resp)

	case FetchInboxRequest:
		resp := s.handleFetchInbox(ctx, m)
		return fn.Ok[MailResponse](resp)

	case ReadMessageRequest:
		resp := s.handleReadMessage(ctx, m)
		return fn.Ok[MailResponse](resp)

	case UpdateStateRequest:
		resp := s.handleUpdateState(ctx, m)
		return fn.Ok[MailResponse](resp)

	case AckMessageRequest:
		resp := s.handleAckMessage(ctx, m)
		return fn.Ok[MailResponse](resp)

	case GetStatusRequest:
		resp := s.handleGetStatus(ctx, m)
		return fn.Ok[MailResponse](resp)

	case PollChangesRequest:
		resp := s.handlePollChanges(ctx, m)
		return fn.Ok[MailResponse](resp)

	case PublishRequest:
		resp := s.handlePublish(ctx, m)
		return fn.Ok[MailResponse](resp)

	default:
		return fn.Err[MailResponse](fmt.Errorf(
			"unknown message type: %T", msg,
		))
	}
}

// handleSendMail processes a SendMailRequest.
func (s *Service) handleSendMail(ctx context.Context,
	req SendMailRequest) SendMailResponse {

	var response SendMailResponse

	err := s.store.WithTx(ctx, func(ctx context.Context,
		q *sqlc.Queries) error {

		// Generate thread ID if not provided.
		threadID := req.ThreadID
		if threadID == "" {
			threadID = uuid.New().String()
		}
		response.ThreadID = threadID

		// Resolve recipient agent IDs.
		var recipientIDs []int64
		for _, name := range req.RecipientNames {
			agent, err := q.GetAgentByName(ctx, name)
			if err != nil {
				return fmt.Errorf("recipient %q not found: %w",
					name, err)
			}
			recipientIDs = append(recipientIDs, agent.ID)
		}

		// Get or create the sender's inbox topic for direct messages.
		sender, err := q.GetAgent(ctx, req.SenderID)
		if err != nil {
			return fmt.Errorf("sender not found: %w", err)
		}

		// For direct messages, use the first recipient's inbox topic.
		var topicID int64
		if len(recipientIDs) > 0 {
			recipient, err := q.GetAgent(ctx, recipientIDs[0])
			if err != nil {
				return fmt.Errorf("failed to get recipient: %w",
					err)
			}

			topic, err := q.GetOrCreateAgentInboxTopic(
				ctx, sqlc.GetOrCreateAgentInboxTopicParams{
					Column1: sql.NullString{
						String: recipient.Name,
						Valid:  true,
					},
					CreatedAt: time.Now().Unix(),
				},
			)
			if err != nil {
				return fmt.Errorf("failed to get inbox topic: "+
					"%w", err)
			}
			topicID = topic.ID
		} else if req.TopicName != "" {
			// Use the specified topic for pub/sub.
			topic, err := q.GetTopicByName(ctx, req.TopicName)
			if err != nil {
				return fmt.Errorf("topic %q not found: %w",
					req.TopicName, err)
			}
			topicID = topic.ID
		} else {
			return fmt.Errorf("no recipients or topic specified")
		}

		// Get the next log offset for the topic.
		logOffset, err := db.NextLogOffset(ctx, q, topicID)
		if err != nil {
			return fmt.Errorf("failed to get next offset: %w", err)
		}

		// Convert deadline to nullable int64.
		var deadlineAt sql.NullInt64
		if req.Deadline != nil {
			deadlineAt = sql.NullInt64{
				Int64: req.Deadline.Unix(),
				Valid: true,
			}
		}

		// Convert attachments to nullable string.
		var attachments sql.NullString
		if req.Attachments != "" {
			attachments = sql.NullString{
				String: req.Attachments,
				Valid:  true,
			}
		}

		// Create the message.
		msg, err := q.CreateMessage(ctx, sqlc.CreateMessageParams{
			ThreadID:    threadID,
			TopicID:     topicID,
			LogOffset:   logOffset,
			SenderID:    sender.ID,
			Subject:     req.Subject,
			BodyMd:      req.Body,
			Priority:    string(req.Priority),
			DeadlineAt:  deadlineAt,
			Attachments: attachments,
			CreatedAt:   time.Now().Unix(),
		})
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}
		response.MessageID = msg.ID

		// Create recipient entries.
		for _, recipientID := range recipientIDs {
			err := q.CreateMessageRecipient(
				ctx, sqlc.CreateMessageRecipientParams{
					MessageID: msg.ID,
					AgentID:   recipientID,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to create recipient "+
					"entry: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		response.Error = err
	}

	return response
}

// handleFetchInbox processes a FetchInboxRequest.
func (s *Service) handleFetchInbox(ctx context.Context,
	req FetchInboxRequest) FetchInboxResponse {

	var response FetchInboxResponse

	limit := int64(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	var rows []sqlc.GetInboxMessagesRow
	var err error

	if req.UnreadOnly {
		unreadRows, err := s.store.Queries().GetUnreadMessages(
			ctx, sqlc.GetUnreadMessagesParams{
				AgentID: req.AgentID,
				Limit:   limit,
			},
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to fetch unread: "+
				"%w", err)
			return response
		}

		// Convert to InboxMessage format.
		for _, r := range unreadRows {
			response.Messages = append(
				response.Messages, convertUnreadRow(r),
			)
		}

		return response
	}

	rows, err = s.store.Queries().GetInboxMessages(
		ctx, sqlc.GetInboxMessagesParams{
			AgentID: req.AgentID,
			Limit:   limit,
		},
	)
	if err != nil {
		response.Error = fmt.Errorf("failed to fetch inbox: %w", err)
		return response
	}

	for _, r := range rows {
		response.Messages = append(response.Messages, convertInboxRow(r))
	}

	return response
}

// handleReadMessage processes a ReadMessageRequest.
func (s *Service) handleReadMessage(ctx context.Context,
	req ReadMessageRequest) ReadMessageResponse {

	var response ReadMessageResponse

	// Get the message.
	msg, err := s.store.Queries().GetMessage(ctx, req.MessageID)
	if err != nil {
		response.Error = fmt.Errorf("message not found: %w", err)
		return response
	}

	// Get the recipient state.
	recipient, err := s.store.Queries().GetMessageRecipient(
		ctx, sqlc.GetMessageRecipientParams{
			MessageID: req.MessageID,
			AgentID:   req.AgentID,
		},
	)
	if err != nil {
		response.Error = fmt.Errorf("not a recipient: %w", err)
		return response
	}

	// Mark as read if currently unread.
	if recipient.State == "unread" {
		now := time.Now().Unix()
		err = s.store.Queries().UpdateRecipientState(
			ctx, sqlc.UpdateRecipientStateParams{
				State:     "read",
				Column2:   "read",
				ReadAt:    sql.NullInt64{Int64: now, Valid: true},
				MessageID: req.MessageID,
				AgentID:   req.AgentID,
			},
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to mark read: %w",
				err)
			return response
		}
		recipient.State = "read"
		recipient.ReadAt = sql.NullInt64{Int64: now, Valid: true}
	}

	// Build response.
	inboxMsg := InboxMessage{
		ID:        msg.ID,
		ThreadID:  msg.ThreadID,
		TopicID:   msg.TopicID,
		SenderID:  msg.SenderID,
		Subject:   msg.Subject,
		Body:      msg.BodyMd,
		Priority:  Priority(msg.Priority),
		State:     recipient.State,
		CreatedAt: time.Unix(msg.CreatedAt, 0),
	}

	if msg.DeadlineAt.Valid {
		t := time.Unix(msg.DeadlineAt.Int64, 0)
		inboxMsg.Deadline = &t
	}

	if recipient.SnoozedUntil.Valid {
		t := time.Unix(recipient.SnoozedUntil.Int64, 0)
		inboxMsg.SnoozedUntil = &t
	}

	if recipient.ReadAt.Valid {
		t := time.Unix(recipient.ReadAt.Int64, 0)
		inboxMsg.ReadAt = &t
	}

	if recipient.AckedAt.Valid {
		t := time.Unix(recipient.AckedAt.Int64, 0)
		inboxMsg.AckedAt = &t
	}

	response.Message = &inboxMsg
	return response
}

// handleUpdateState processes an UpdateStateRequest.
func (s *Service) handleUpdateState(ctx context.Context,
	req UpdateStateRequest) UpdateStateResponse {

	var response UpdateStateResponse

	if req.NewState == "snoozed" {
		if req.SnoozedUntil == nil {
			response.Error = fmt.Errorf(
				"snoozed_until required for snooze",
			)
			return response
		}

		err := s.store.Queries().UpdateRecipientSnoozed(
			ctx, sqlc.UpdateRecipientSnoozedParams{
				SnoozedUntil: sql.NullInt64{
					Int64: req.SnoozedUntil.Unix(),
					Valid: true,
				},
				MessageID: req.MessageID,
				AgentID:   req.AgentID,
			},
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to snooze: %w", err)
			return response
		}
	} else {
		now := time.Now().Unix()
		err := s.store.Queries().UpdateRecipientState(
			ctx, sqlc.UpdateRecipientStateParams{
				State:     req.NewState,
				Column2:   req.NewState,
				ReadAt:    sql.NullInt64{Int64: now, Valid: true},
				MessageID: req.MessageID,
				AgentID:   req.AgentID,
			},
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to update state: "+
				"%w", err)
			return response
		}
	}

	response.Success = true
	return response
}

// handleAckMessage processes an AckMessageRequest.
func (s *Service) handleAckMessage(ctx context.Context,
	req AckMessageRequest) AckMessageResponse {

	var response AckMessageResponse

	err := s.store.Queries().UpdateRecipientAcked(
		ctx, sqlc.UpdateRecipientAckedParams{
			AckedAt:   sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
			MessageID: req.MessageID,
			AgentID:   req.AgentID,
		},
	)
	if err != nil {
		response.Error = fmt.Errorf("failed to ack: %w", err)
		return response
	}

	response.Success = true
	return response
}

// handleGetStatus processes a GetStatusRequest.
func (s *Service) handleGetStatus(ctx context.Context,
	req GetStatusRequest) GetStatusResponse {

	var response GetStatusResponse

	agent, err := s.store.Queries().GetAgent(ctx, req.AgentID)
	if err != nil {
		response.Error = fmt.Errorf("agent not found: %w", err)
		return response
	}

	unreadCount, err := s.store.Queries().CountUnreadByAgent(
		ctx, req.AgentID,
	)
	if err != nil {
		response.Error = fmt.Errorf("failed to count unread: %w", err)
		return response
	}

	urgentCount, err := s.store.Queries().CountUnreadUrgentByAgent(
		ctx, req.AgentID,
	)
	if err != nil {
		response.Error = fmt.Errorf("failed to count urgent: %w", err)
		return response
	}

	response.Status = AgentStatus{
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		UnreadCount: unreadCount,
		UrgentCount: urgentCount,
	}

	return response
}

// handlePollChanges processes a PollChangesRequest.
func (s *Service) handlePollChanges(ctx context.Context,
	req PollChangesRequest) PollChangesResponse {

	var response PollChangesResponse
	response.NewOffsets = make(map[int64]int64)

	// Get subscribed topics.
	topics, err := s.store.Queries().ListSubscriptionsByAgent(
		ctx, req.AgentID,
	)
	if err != nil {
		response.Error = fmt.Errorf("failed to list subscriptions: %w",
			err)
		return response
	}

	// Poll each topic for new messages.
	for _, topic := range topics {
		sinceOffset := req.SinceOffsets[topic.ID]

		msgs, err := s.store.Queries().GetMessagesSinceOffset(
			ctx, sqlc.GetMessagesSinceOffsetParams{
				TopicID:   topic.ID,
				LogOffset: sinceOffset,
				Limit:     100,
			},
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to poll topic %d: "+
				"%w", topic.ID, err)
			return response
		}

		for _, msg := range msgs {
			inboxMsg := InboxMessage{
				ID:        msg.ID,
				ThreadID:  msg.ThreadID,
				TopicID:   msg.TopicID,
				SenderID:  msg.SenderID,
				Subject:   msg.Subject,
				Body:      msg.BodyMd,
				Priority:  Priority(msg.Priority),
				CreatedAt: time.Unix(msg.CreatedAt, 0),
			}

			if msg.DeadlineAt.Valid {
				t := time.Unix(msg.DeadlineAt.Int64, 0)
				inboxMsg.Deadline = &t
			}

			response.NewMessages = append(
				response.NewMessages, inboxMsg,
			)

			if msg.LogOffset > response.NewOffsets[topic.ID] {
				response.NewOffsets[topic.ID] = msg.LogOffset
			}
		}
	}

	return response
}

// convertInboxRow converts a database row to InboxMessage.
func convertInboxRow(r sqlc.GetInboxMessagesRow) InboxMessage {
	msg := InboxMessage{
		ID:        r.ID,
		ThreadID:  r.ThreadID,
		TopicID:   r.TopicID,
		SenderID:  r.SenderID,
		Subject:   r.Subject,
		Body:      r.BodyMd,
		Priority:  Priority(r.Priority),
		State:     r.State,
		CreatedAt: time.Unix(r.CreatedAt, 0),
	}

	if r.DeadlineAt.Valid {
		t := time.Unix(r.DeadlineAt.Int64, 0)
		msg.Deadline = &t
	}

	if r.SnoozedUntil.Valid {
		t := time.Unix(r.SnoozedUntil.Int64, 0)
		msg.SnoozedUntil = &t
	}

	if r.ReadAt.Valid {
		t := time.Unix(r.ReadAt.Int64, 0)
		msg.ReadAt = &t
	}

	if r.AckedAt.Valid {
		t := time.Unix(r.AckedAt.Int64, 0)
		msg.AckedAt = &t
	}

	return msg
}

// handlePublish processes a PublishRequest.
func (s *Service) handlePublish(ctx context.Context,
	req PublishRequest) PublishResponse {

	var response PublishResponse

	err := s.store.WithTx(ctx, func(ctx context.Context,
		q *sqlc.Queries) error {

		// Get the topic.
		topic, err := q.GetTopicByName(ctx, req.TopicName)
		if err != nil {
			return fmt.Errorf("topic %q not found: %w",
				req.TopicName, err)
		}

		// Generate thread ID.
		threadID := uuid.New().String()

		// Get the next log offset.
		logOffset, err := db.NextLogOffset(ctx, q, topic.ID)
		if err != nil {
			return fmt.Errorf("failed to get next offset: %w", err)
		}

		// Create the message.
		msg, err := q.CreateMessage(ctx, sqlc.CreateMessageParams{
			ThreadID:  threadID,
			TopicID:   topic.ID,
			LogOffset: logOffset,
			SenderID:  req.SenderID,
			Subject:   req.Subject,
			BodyMd:    req.Body,
			Priority:  string(req.Priority),
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}
		response.MessageID = msg.ID

		// Get all subscribers to the topic.
		subscribers, err := q.ListSubscriptionsByTopic(ctx, topic.ID)
		if err != nil {
			return fmt.Errorf("failed to get subscribers: %w", err)
		}

		// Create recipient entries for all subscribers.
		for _, sub := range subscribers {
			err := q.CreateMessageRecipient(
				ctx, sqlc.CreateMessageRecipientParams{
					MessageID: msg.ID,
					AgentID:   sub.ID,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to create recipient "+
					"entry: %w", err)
			}
			response.RecipientsCount++
		}

		return nil
	})

	if err != nil {
		response.Error = err
	}

	return response
}

// convertUnreadRow converts an unread row to InboxMessage.
func convertUnreadRow(r sqlc.GetUnreadMessagesRow) InboxMessage {
	msg := InboxMessage{
		ID:        r.ID,
		ThreadID:  r.ThreadID,
		TopicID:   r.TopicID,
		SenderID:  r.SenderID,
		Subject:   r.Subject,
		Body:      r.BodyMd,
		Priority:  Priority(r.Priority),
		State:     r.State,
		CreatedAt: time.Unix(r.CreatedAt, 0),
	}

	if r.DeadlineAt.Valid {
		t := time.Unix(r.DeadlineAt.Int64, 0)
		msg.Deadline = &t
	}

	if r.SnoozedUntil.Valid {
		t := time.Unix(r.SnoozedUntil.Int64, 0)
		msg.SnoozedUntil = &t
	}

	if r.ReadAt.Valid {
		t := time.Unix(r.ReadAt.Int64, 0)
		msg.ReadAt = &t
	}

	if r.AckedAt.Valid {
		t := time.Unix(r.AckedAt.Int64, 0)
		msg.AckedAt = &t
	}

	return msg
}

// =============================================================================
// Direct methods for gRPC server (bypass actor system for synchronous calls)
// =============================================================================

// Send sends a mail message synchronously.
func (s *Service) Send(ctx context.Context, req SendMailRequest) (SendMailResponse, error) {
	resp := s.handleSendMail(ctx, req)
	return resp, resp.Error
}

// FetchInbox fetches inbox messages synchronously.
func (s *Service) FetchInbox(ctx context.Context, req FetchInboxRequest) ([]InboxMessage, error) {
	resp := s.handleFetchInbox(ctx, req)
	return resp.Messages, resp.Error
}

// ReadMessage reads and marks a message as read synchronously.
func (s *Service) ReadMessage(ctx context.Context, agentID, messageID int64) (*InboxMessage, error) {
	resp := s.handleReadMessage(ctx, ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
	return resp.Message, resp.Error
}

// ReadThread reads all messages in a thread synchronously.
func (s *Service) ReadThread(ctx context.Context, agentID int64, threadID string) ([]InboxMessage, error) {
	// Get messages by thread ID.
	rows, err := s.store.Queries().GetMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	var messages []InboxMessage
	for _, r := range rows {
		msg := InboxMessage{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  Priority(r.Priority),
			CreatedAt: time.Unix(r.CreatedAt, 0),
		}
		if r.DeadlineAt.Valid {
			t := time.Unix(r.DeadlineAt.Int64, 0)
			msg.Deadline = &t
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// UpdateState updates the state of a message synchronously.
func (s *Service) UpdateState(ctx context.Context, req UpdateStateRequest) error {
	resp := s.handleUpdateState(ctx, req)
	return resp.Error
}

// AckMessage acknowledges a message synchronously.
func (s *Service) AckMessage(ctx context.Context, agentID, messageID int64) error {
	resp := s.handleAckMessage(ctx, AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
	return resp.Error
}

// GetStatus gets the mail status for an agent synchronously.
func (s *Service) GetStatus(ctx context.Context, agentID int64) (AgentStatus, error) {
	resp := s.handleGetStatus(ctx, GetStatusRequest{AgentID: agentID})
	return resp.Status, resp.Error
}

// PollChanges polls for new messages since given offsets synchronously.
func (s *Service) PollChanges(ctx context.Context, req PollChangesRequest) (PollChangesResponse, error) {
	resp := s.handlePollChanges(ctx, req)
	return resp, resp.Error
}

// Publish publishes a message to a topic synchronously.
func (s *Service) Publish(ctx context.Context, req PublishRequest) (PublishResponse, error) {
	resp := s.handlePublish(ctx, req)
	return resp, resp.Error
}

// Subscribe subscribes an agent to a topic.
func (s *Service) Subscribe(ctx context.Context, agentID int64, topicName string) (int64, error) {
	return s.store.WithTxResult(ctx, func(ctx context.Context, q *sqlc.Queries) (int64, error) {
		// Get or create the topic.
		topic, err := q.GetOrCreateTopic(ctx, sqlc.GetOrCreateTopicParams{
			Name:      topicName,
			TopicType: "broadcast",
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return 0, fmt.Errorf("failed to get/create topic: %w", err)
		}

		// Create subscription.
		err = q.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
			AgentID:      agentID,
			TopicID:      topic.ID,
			SubscribedAt: time.Now().Unix(),
		})
		if err != nil {
			return 0, fmt.Errorf("failed to create subscription: %w", err)
		}

		return topic.ID, nil
	})
}

// Unsubscribe removes an agent's subscription to a topic.
func (s *Service) Unsubscribe(ctx context.Context, agentID int64, topicName string) error {
	return s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		topic, err := q.GetTopicByName(ctx, topicName)
		if err != nil {
			return fmt.Errorf("topic not found: %w", err)
		}

		err = q.DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
			AgentID: agentID,
			TopicID: topic.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to delete subscription: %w", err)
		}

		return nil
	})
}

// TopicInfo represents basic topic information.
type TopicInfo struct {
	ID        int64
	Name      string
	TopicType string
	CreatedAt time.Time
}

// ListTopics lists topics, optionally filtered by agent subscription.
func (s *Service) ListTopics(ctx context.Context, req ListTopicsRequest) ([]TopicInfo, error) {
	var topics []TopicInfo

	if req.SubscribedOnly && req.AgentID > 0 {
		rows, err := s.store.Queries().ListSubscriptionsByAgent(ctx, req.AgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}
		for _, r := range rows {
			topics = append(topics, TopicInfo{
				ID:        r.ID,
				Name:      r.Name,
				TopicType: r.TopicType,
				CreatedAt: time.Unix(r.CreatedAt, 0),
			})
		}
	} else {
		rows, err := s.store.Queries().ListTopics(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list topics: %w", err)
		}
		for _, r := range rows {
			topics = append(topics, TopicInfo{
				ID:        r.ID,
				Name:      r.Name,
				TopicType: r.TopicType,
				CreatedAt: time.Unix(r.CreatedAt, 0),
			})
		}
	}

	return topics, nil
}

// ListTopicsRequest is the request for ListTopics.
type ListTopicsRequest struct {
	AgentID        int64
	SubscribedOnly bool
}

// SearchRequest is the request for Search.
type SearchRequest struct {
	AgentID int64
	Query   string
	TopicID int64
	Limit   int32
}

// Search performs full-text search across messages.
func (s *Service) Search(ctx context.Context, req SearchRequest) ([]InboxMessage, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.store.SearchMessagesForAgent(ctx, req.Query, req.AgentID, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var messages []InboxMessage
	for _, r := range rows {
		msg := InboxMessage{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  Priority(r.Priority),
			CreatedAt: time.Unix(r.CreatedAt, 0),
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// SubscribeInbox creates a streaming subscription to an agent's inbox.
// Returns a channel that receives new messages and a cancel function.
func (s *Service) SubscribeInbox(ctx context.Context, agentID int64) (<-chan InboxMessage, func(), error) {
	// Create a channel for messages.
	msgCh := make(chan InboxMessage, 100)

	// For now, this is a simple polling-based implementation.
	// A production implementation would use database notifications or a pub/sub system.
	done := make(chan struct{})

	go func() {
		defer close(msgCh)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var lastID int64

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Poll for new messages.
				rows, err := s.store.Queries().GetInboxMessages(
					ctx, sqlc.GetInboxMessagesParams{
						AgentID: agentID,
						Limit:   10,
					},
				)
				if err != nil {
					continue
				}

				for _, r := range rows {
					if r.ID > lastID {
						msg := convertInboxRow(r)
						select {
						case msgCh <- msg:
							lastID = r.ID
						default:
							// Channel full, skip.
						}
					}
				}
			}
		}
	}()

	cancel := func() {
		close(done)
	}

	return msgCh, cancel, nil
}
