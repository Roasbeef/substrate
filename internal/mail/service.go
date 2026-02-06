package mail

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
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

// ServiceConfig holds configuration for the mail service.
type ServiceConfig struct {
	// Store is the storage backend for persisting messages.
	Store store.Storage

	// NotificationHub is the optional notification hub actor reference. When
	// set, the service notifies recipients via the hub after sending messages
	// using fire-and-forget Tell semantics for optimal performance.
	NotificationHub NotificationActorRef
}

// Service is the mail service actor behavior.
type Service struct {
	store    store.Storage
	notifHub NotificationActorRef
}

// NewService creates a new mail service with the given configuration.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		store:    cfg.Store,
		notifHub: cfg.NotificationHub,
	}
}

// NewServiceWithStore creates a new mail service with just a store (no
// notifications). This is a convenience constructor for simple use cases.
func NewServiceWithStore(s store.Storage) *Service {
	return &Service{store: s}
}

// SetNotificationHub sets the notification hub actor reference. This allows
// deferred initialization when the hub is spawned after the mail service.
func (s *Service) SetNotificationHub(hub NotificationActorRef) {
	s.notifHub = hub
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (s *Service) Receive(ctx context.Context,
	msg MailRequest,
) fn.Result[MailResponse] {
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
	req SendMailRequest,
) SendMailResponse {
	var response SendMailResponse

	// Variables to capture data for notification after successful transaction.
	var recipientIDs []int64
	var senderName string
	var msgCreatedAt time.Time

	err := s.store.WithTx(ctx, func(ctx context.Context,
		txStore store.Storage,
	) error {
		// Generate thread ID if not provided.
		threadID := req.ThreadID
		if threadID == "" {
			threadID = uuid.New().String()
		}
		response.ThreadID = threadID

		// Resolve recipient agent IDs.
		for _, name := range req.RecipientNames {
			agent, err := txStore.GetAgentByName(ctx, name)
			if err != nil {
				return fmt.Errorf("recipient %q not found: %w",
					name, err)
			}
			recipientIDs = append(recipientIDs, agent.ID)
		}

		// Get or create the sender's inbox topic for direct messages.
		sender, err := txStore.GetAgent(ctx, req.SenderID)
		if err != nil {
			return fmt.Errorf("sender not found: %w", err)
		}
		senderName = sender.Name

		// For direct messages, use the first recipient's inbox topic.
		var topicID int64
		if len(recipientIDs) > 0 {
			recipient, err := txStore.GetAgent(ctx, recipientIDs[0])
			if err != nil {
				return fmt.Errorf("failed to get recipient: %w",
					err)
			}

			topic, err := txStore.GetOrCreateAgentInboxTopic(
				ctx, recipient.Name,
			)
			if err != nil {
				return fmt.Errorf("failed to get inbox topic: "+
					"%w", err)
			}
			topicID = topic.ID
		} else if req.TopicName != "" {
			// Use the specified topic for pub/sub.
			topic, err := txStore.GetTopicByName(ctx, req.TopicName)
			if err != nil {
				return fmt.Errorf("topic %q not found: %w",
					req.TopicName, err)
			}
			topicID = topic.ID
		} else {
			return fmt.Errorf("no recipients or topic specified")
		}

		// Get the next log offset for the topic.
		logOffset, err := txStore.NextLogOffset(ctx, topicID)
		if err != nil {
			return fmt.Errorf("failed to get next offset: %w", err)
		}

		// Create the message.
		msg, err := txStore.CreateMessage(ctx, store.CreateMessageParams{
			ThreadID:    threadID,
			TopicID:     topicID,
			LogOffset:   logOffset,
			SenderID:    sender.ID,
			Subject:     req.Subject,
			Body:        req.Body,
			Priority:    string(req.Priority),
			DeadlineAt:  req.Deadline,
			Attachments: req.Attachments,
		})
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}
		response.MessageID = msg.ID
		msgCreatedAt = msg.CreatedAt

		// Create recipient entries.
		for _, recipientID := range recipientIDs {
			err := txStore.CreateMessageRecipient(
				ctx, msg.ID, recipientID,
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
		return response
	}

	// After successful transaction, notify recipients via notification hub actor.
	// Use fire-and-forget Tell for optimal performance - we don't need to wait
	// for the notification to be delivered.
	if s.notifHub != nil && len(recipientIDs) > 0 {
		notifMsg := InboxMessage{
			ID:         response.MessageID,
			ThreadID:   response.ThreadID,
			SenderID:   req.SenderID,
			SenderName: senderName,
			Subject:    req.Subject,
			Body:       req.Body,
			Priority:   req.Priority,
			State:      StateUnreadStr.String(),
			CreatedAt:  msgCreatedAt,
			Deadline:   req.Deadline,
		}

		// Notify each recipient via the hub actor using fire-and-forget.
		for _, recipientID := range recipientIDs {
			s.notifHub.Tell(ctx, NotifyAgentMsg{
				AgentID: recipientID,
				Message: notifMsg,
			})
		}

		// Also notify agent_id=0 for global inbox viewers (web UI without
		// a specific agent selected). This ensures real-time updates work
		// for the default inbox view.
		s.notifHub.Tell(ctx, NotifyAgentMsg{
			AgentID: 0,
			Message: notifMsg,
		})
	}

	return response
}

// handleFetchInbox processes a FetchInboxRequest.
func (s *Service) handleFetchInbox(ctx context.Context,
	req FetchInboxRequest,
) FetchInboxResponse {
	var response FetchInboxResponse

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	// Handle sender name prefix filter (for aggregate views like CodeReviewer).
	if req.SenderNamePrefix != "" {
		msgs, err := s.store.GetMessagesBySenderNamePrefix(
			ctx, req.SenderNamePrefix, limit,
		)
		if err != nil {
			response.Error = fmt.Errorf(
				"failed to fetch by sender prefix: %w", err,
			)
			return response
		}

		for _, m := range msgs {
			response.Messages = append(
				response.Messages, storeInboxToMail(m),
			)
		}
		return response
	}

	// Handle sent messages view.
	if req.SentOnly {
		// If AgentID is provided, get sent messages for that agent.
		// Otherwise, get all sent messages globally.
		if req.AgentID != 0 {
			msgs, err := s.store.GetSentMessages(ctx, req.AgentID, limit)
			if err != nil {
				response.Error = fmt.Errorf("failed to fetch sent: %w", err)
				return response
			}

			// Get agent info for sender details.
			agent, err := s.store.GetAgent(ctx, req.AgentID)
			if err != nil {
				response.Error = fmt.Errorf("failed to get agent: %w", err)
				return response
			}

			for _, m := range msgs {
				response.Messages = append(response.Messages, InboxMessage{
					ID:               m.ID,
					ThreadID:         m.ThreadID,
					TopicID:          m.TopicID,
					SenderID:         m.SenderID,
					SenderName:       agent.Name,
					SenderProjectKey: agent.ProjectKey,
					SenderGitBranch:  agent.GitBranch,
					Subject:          m.Subject,
					Body:             m.Body,
					Priority:         Priority(m.Priority),
					CreatedAt:        m.CreatedAt,
					Deadline:         m.DeadlineAt,
					State:            "read", // Sent messages are always "read".
				})
			}
		} else {
			msgs, err := s.store.GetAllSentMessages(ctx, limit)
			if err != nil {
				response.Error = fmt.Errorf("failed to fetch all sent: %w", err)
				return response
			}

			for _, m := range msgs {
				response.Messages = append(
					response.Messages, storeInboxToMail(m),
				)
			}
		}
		return response
	}

	if req.UnreadOnly {
		msgs, err := s.store.GetUnreadMessages(ctx, req.AgentID, limit)
		if err != nil {
			response.Error = fmt.Errorf("failed to fetch unread: "+
				"%w", err)
			return response
		}

		// Convert to InboxMessage format.
		for _, m := range msgs {
			response.Messages = append(
				response.Messages, storeInboxToMail(m),
			)
		}

		return response
	}

	// Use global inbox query when no specific agent is selected (AgentID=0).
	var msgs []store.InboxMessage
	var err error
	if req.AgentID == 0 {
		msgs, err = s.store.GetAllInboxMessages(ctx, limit, 0)
	} else {
		msgs, err = s.store.GetInboxMessages(ctx, req.AgentID, limit)
	}
	if err != nil {
		response.Error = fmt.Errorf("failed to fetch inbox: %w", err)
		return response
	}

	for _, m := range msgs {
		response.Messages = append(response.Messages, storeInboxToMail(m))
	}

	return response
}

// handleReadMessage processes a ReadMessageRequest. The read-and-mark-read
// operation is wrapped in a transaction to ensure atomic state transition.
func (s *Service) handleReadMessage(ctx context.Context,
	req ReadMessageRequest,
) ReadMessageResponse {
	var response ReadMessageResponse

	err := s.store.WithTx(ctx, func(ctx context.Context,
		txStore store.Storage,
	) error {
		// Get the message.
		msg, err := txStore.GetMessage(ctx, req.MessageID)
		if err != nil {
			return fmt.Errorf("message not found: %w", err)
		}

		// Get the recipient state.
		recipient, err := txStore.GetMessageRecipient(
			ctx, req.MessageID, req.AgentID,
		)
		if err != nil {
			return fmt.Errorf("not a recipient: %w", err)
		}

		// Mark as read if currently unread.
		if recipient.State == StateUnreadStr.String() {
			err = txStore.MarkMessageRead(ctx, req.MessageID, req.AgentID)
			if err != nil {
				return fmt.Errorf("failed to mark read: %w", err)
			}
			recipient.State = StateReadStr.String()
			now := time.Now()
			recipient.ReadAt = &now
		}

		// Build response.
		inboxMsg := InboxMessage{
			ID:           msg.ID,
			ThreadID:     msg.ThreadID,
			TopicID:      msg.TopicID,
			SenderID:     msg.SenderID,
			Subject:      msg.Subject,
			Body:         msg.Body,
			Priority:     Priority(msg.Priority),
			State:        recipient.State,
			CreatedAt:    msg.CreatedAt,
			Deadline:     msg.DeadlineAt,
			SnoozedUntil: recipient.SnoozedUntil,
			ReadAt:       recipient.ReadAt,
			AckedAt:      recipient.AckedAt,
		}

		response.Message = &inboxMsg
		return nil
	})
	if err != nil {
		response.Error = err
	}
	return response
}

// handleUpdateState processes an UpdateStateRequest.
func (s *Service) handleUpdateState(ctx context.Context,
	req UpdateStateRequest,
) UpdateStateResponse {
	var response UpdateStateResponse

	if req.NewState == "snoozed" {
		if req.SnoozedUntil == nil {
			response.Error = fmt.Errorf(
				"snoozed_until required for snooze",
			)
			return response
		}

		err := s.store.SnoozeMessage(
			ctx, req.MessageID, req.AgentID, *req.SnoozedUntil,
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to snooze: %w", err)
			return response
		}
	} else if req.NewState == StateReadStr.String() {
		// Use MarkMessageRead for read transitions so the
		// read_at timestamp is properly set in the DB.
		err := s.store.MarkMessageRead(
			ctx, req.MessageID, req.AgentID,
		)
		if err != nil {
			response.Error = fmt.Errorf(
				"failed to mark read: %w", err,
			)
			return response
		}
	} else {
		err := s.store.UpdateRecipientState(
			ctx, req.MessageID, req.AgentID, req.NewState,
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
	req AckMessageRequest,
) AckMessageResponse {
	var response AckMessageResponse

	err := s.store.AckMessage(ctx, req.MessageID, req.AgentID)
	if err != nil {
		response.Error = fmt.Errorf("failed to ack: %w", err)
		return response
	}

	response.Success = true
	return response
}

// handleGetStatus processes a GetStatusRequest.
func (s *Service) handleGetStatus(ctx context.Context,
	req GetStatusRequest,
) GetStatusResponse {
	var response GetStatusResponse

	agent, err := s.store.GetAgent(ctx, req.AgentID)
	if err != nil {
		response.Error = fmt.Errorf("agent not found: %w", err)
		return response
	}

	unreadCount, err := s.store.CountUnreadByAgent(ctx, req.AgentID)
	if err != nil {
		response.Error = fmt.Errorf("failed to count unread: %w", err)
		return response
	}

	urgentCount, err := s.store.CountUnreadUrgentByAgent(ctx, req.AgentID)
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

// handlePollChanges processes a PollChangesRequest. It checks both direct
// inbox messages (unread) and subscribed topic messages since the given
// offsets. Messages are deduplicated by ID.
func (s *Service) handlePollChanges(ctx context.Context,
	req PollChangesRequest,
) PollChangesResponse {
	var response PollChangesResponse
	response.NewOffsets = make(map[int64]int64)

	// Track seen message IDs to avoid duplicates (a message can appear
	// in both inbox and topic subscription).
	seenMsgIDs := make(map[int64]bool)

	// First, check for unread direct messages in the agent's inbox.
	unreadMsgs, err := s.store.GetUnreadMessages(ctx, req.AgentID, 100)
	if err != nil {
		response.Error = fmt.Errorf("failed to get unread messages: %w",
			err)
		return response
	}

	// Add unread direct messages to response.
	for _, msg := range unreadMsgs {
		seenMsgIDs[msg.ID] = true
		response.NewMessages = append(
			response.NewMessages, storeInboxToMail(msg),
		)
	}

	// Then check subscribed topics for new messages since the given offsets.
	topics, err := s.store.ListSubscriptionsByAgent(ctx, req.AgentID)
	if err != nil {
		response.Error = fmt.Errorf("failed to list subscriptions: %w",
			err)
		return response
	}

	// Poll each topic for new messages since the given offset.
	for _, topic := range topics {
		sinceOffset := req.SinceOffsets[topic.ID]

		msgs, err := s.store.GetMessagesSinceOffset(
			ctx, topic.ID, sinceOffset, 100,
		)
		if err != nil {
			response.Error = fmt.Errorf("failed to poll topic %d: "+
				"%w", topic.ID, err)
			return response
		}

		for _, msg := range msgs {
			// Track offset even if message was already seen.
			if msg.LogOffset > response.NewOffsets[topic.ID] {
				response.NewOffsets[topic.ID] = msg.LogOffset
			}

			// Skip if already added from inbox.
			if seenMsgIDs[msg.ID] {
				continue
			}
			seenMsgIDs[msg.ID] = true

			response.NewMessages = append(
				response.NewMessages, storeMessageToMail(msg),
			)
		}
	}

	return response
}

// storeInboxToMail converts a store.InboxMessage to mail.InboxMessage.
func storeInboxToMail(m store.InboxMessage) InboxMessage {
	return InboxMessage{
		ID:               m.ID,
		ThreadID:         m.ThreadID,
		TopicID:          m.TopicID,
		SenderID:         m.SenderID,
		SenderName:       m.SenderName,
		SenderProjectKey: m.SenderProjectKey,
		SenderGitBranch:  m.SenderGitBranch,
		Subject:          m.Subject,
		Body:             m.Body,
		Priority:         Priority(m.Priority),
		State:            m.State,
		CreatedAt:        m.CreatedAt,
		Deadline:         m.DeadlineAt,
		SnoozedUntil:     m.SnoozedUntil,
		ReadAt:           m.ReadAt,
		AckedAt:          m.AckedAt,
	}
}

// storeMessageToMail converts a store.Message to mail.InboxMessage.
func storeMessageToMail(m store.Message) InboxMessage {
	return InboxMessage{
		ID:        m.ID,
		ThreadID:  m.ThreadID,
		TopicID:   m.TopicID,
		SenderID:  m.SenderID,
		Subject:   m.Subject,
		Body:      m.Body,
		Priority:  Priority(m.Priority),
		CreatedAt: m.CreatedAt,
		Deadline:  m.DeadlineAt,
	}
}

// storeInboxMessageToMail converts a store.InboxMessage to mail.InboxMessage.
// This variant includes sender information (name, project, branch).
func storeInboxMessageToMail(m store.InboxMessage) InboxMessage {
	return InboxMessage{
		ID:               m.ID,
		ThreadID:         m.ThreadID,
		TopicID:          m.TopicID,
		SenderID:         m.SenderID,
		SenderName:       m.SenderName,
		SenderProjectKey: m.SenderProjectKey,
		SenderGitBranch:  m.SenderGitBranch,
		Subject:          m.Subject,
		Body:             m.Body,
		Priority:         Priority(m.Priority),
		CreatedAt:        m.CreatedAt,
		Deadline:         m.DeadlineAt,
		State:            m.State,
		SnoozedUntil:     m.SnoozedUntil,
		ReadAt:           m.ReadAt,
		AckedAt:          m.AckedAt,
	}
}

// handlePublish processes a PublishRequest.
func (s *Service) handlePublish(ctx context.Context,
	req PublishRequest,
) PublishResponse {
	var response PublishResponse

	err := s.store.WithTx(ctx, func(ctx context.Context,
		txStore store.Storage,
	) error {
		// Get the topic.
		topic, err := txStore.GetTopicByName(ctx, req.TopicName)
		if err != nil {
			return fmt.Errorf("topic %q not found: %w",
				req.TopicName, err)
		}

		// Generate thread ID.
		threadID := uuid.New().String()

		// Get the next log offset.
		logOffset, err := txStore.NextLogOffset(ctx, topic.ID)
		if err != nil {
			return fmt.Errorf("failed to get next offset: %w", err)
		}

		// Create the message.
		msg, err := txStore.CreateMessage(ctx, store.CreateMessageParams{
			ThreadID:  threadID,
			TopicID:   topic.ID,
			LogOffset: logOffset,
			SenderID:  req.SenderID,
			Subject:   req.Subject,
			Body:      req.Body,
			Priority:  string(req.Priority),
		})
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}
		response.MessageID = msg.ID

		// Get all subscribers to the topic.
		subscribers, err := txStore.ListSubscriptionsByTopic(
			ctx, topic.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to get subscribers: %w", err)
		}

		// Create recipient entries for all subscribers.
		for _, sub := range subscribers {
			err := txStore.CreateMessageRecipient(
				ctx, msg.ID, sub.ID,
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
	// Get messages by thread ID with sender information.
	rows, err := s.store.GetMessagesByThreadWithSender(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, r := range rows {
		messages = append(messages, storeInboxMessageToMail(r))
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
	var topicID int64

	err := s.store.WithTx(ctx, func(ctx context.Context,
		txStore store.Storage,
	) error {
		// Get or create the topic.
		topic, err := txStore.GetOrCreateTopic(ctx, topicName, "broadcast")
		if err != nil {
			return fmt.Errorf("failed to get/create topic: %w", err)
		}

		// Create subscription.
		err = txStore.CreateSubscription(ctx, agentID, topic.ID)
		if err != nil {
			return fmt.Errorf("failed to create subscription: %w", err)
		}

		topicID = topic.ID
		return nil
	})

	return topicID, err
}

// Unsubscribe removes an agent's subscription to a topic.
func (s *Service) Unsubscribe(ctx context.Context, agentID int64, topicName string) error {
	return s.store.WithTx(ctx, func(ctx context.Context,
		txStore store.Storage,
	) error {
		topic, err := txStore.GetTopicByName(ctx, topicName)
		if err != nil {
			return fmt.Errorf("topic not found: %w", err)
		}

		err = txStore.DeleteSubscription(ctx, agentID, topic.ID)
		if err != nil {
			return fmt.Errorf("failed to delete subscription: %w", err)
		}

		return nil
	})
}

// TopicInfo represents basic topic information.
type TopicInfo struct {
	ID           int64
	Name         string
	TopicType    string
	CreatedAt    time.Time
	MessageCount int64
}

// ListTopics lists topics, optionally filtered by agent subscription.
func (s *Service) ListTopics(ctx context.Context, req ListTopicsRequest) ([]TopicInfo, error) {
	var topics []TopicInfo

	if req.SubscribedOnly && req.AgentID > 0 {
		rows, err := s.store.ListSubscriptionsByAgent(ctx, req.AgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}
		for _, r := range rows {
			topics = append(topics, TopicInfo{
				ID:        r.ID,
				Name:      r.Name,
				TopicType: r.TopicType,
				CreatedAt: r.CreatedAt,
			})
		}
	} else {
		rows, err := s.store.ListTopics(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list topics: %w", err)
		}
		for _, r := range rows {
			topics = append(topics, TopicInfo{
				ID:           r.ID,
				Name:         r.Name,
				TopicType:    r.TopicType,
				CreatedAt:    r.CreatedAt,
				MessageCount: r.MessageCount,
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

	if req.AgentID == 0 {
		// Global search across all messages.
		rows, err := s.store.SearchMessages(ctx, req.Query, limit)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		var messages []InboxMessage
		for _, r := range rows {
			messages = append(messages, storeInboxToMail(r))
		}
		return messages, nil
	}

	// Agent-specific search.
	rows, err := s.store.SearchMessagesForAgent(
		ctx, req.Query, req.AgentID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var messages []InboxMessage
	for _, r := range rows {
		messages = append(messages, storeMessageToMail(r))
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
				rows, err := s.store.GetInboxMessages(ctx, agentID, 10)
				if err != nil {
					continue
				}

				for _, r := range rows {
					if r.ID > lastID {
						msg := storeInboxToMail(r)
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
