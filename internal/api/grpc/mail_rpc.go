package subtraterpc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/roasbeef/subtrate/internal/agent"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
)

// SendMail sends a new message to one or more recipients.
func (s *Server) SendMail(ctx context.Context, req *SendMailRequest) (*SendMailResponse, error) {
	if req.SenderId == 0 {
		return nil, status.Error(codes.InvalidArgument, "sender_id is required")
	}
	if len(req.RecipientNames) == 0 && req.TopicName == "" {
		return nil, status.Error(codes.InvalidArgument, "recipient_names or topic_name is required")
	}
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	// Convert priority.
	priority := mail.PriorityNormal
	switch req.Priority {
	case Priority_PRIORITY_LOW:
		priority = mail.PriorityLow
	case Priority_PRIORITY_URGENT:
		priority = mail.PriorityUrgent
	}

	// Build send request.
	var deadline *time.Time
	if req.DeadlineAt != nil && req.DeadlineAt.IsValid() {
		t := req.DeadlineAt.AsTime()
		deadline = &t
	}

	sendReq := mail.SendMailRequest{
		SenderID:       req.SenderId,
		RecipientNames: req.RecipientNames,
		TopicName:      req.TopicName,
		ThreadID:       req.ThreadId,
		Subject:        req.Subject,
		Body:           req.Body,
		Priority:       priority,
		Deadline:       deadline,
		Attachments:    req.AttachmentsJson,
	}

	// Send via the shared mail client (actor system).
	resp, err := s.sendMailActor(ctx, sendReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to send mail: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to send mail: %v", resp.Error)
	}
	messageID := resp.MessageID
	threadID := resp.ThreadID

	// Record activity for the message.
	recipientsList := ""
	if len(req.RecipientNames) > 0 {
		recipientsList = req.RecipientNames[0]
		if len(req.RecipientNames) > 1 {
			recipientsList += fmt.Sprintf(" (+%d more)", len(req.RecipientNames)-1)
		}
	} else if req.TopicName != "" {
		recipientsList = req.TopicName
	}
	// Record activity (ignore errors - non-critical).
	_, _ = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      req.SenderId,
		ActivityType: "message",
		Description:  fmt.Sprintf("Sent \"%s\" to %s", req.Subject, recipientsList),
		Metadata:     sql.NullString{},
		CreatedAt:    time.Now().Unix(),
	})

	// WebSocket notifications are now handled by the mail service actor via the
	// NotificationHub. When sendMailActor is called above, the mail service
	// notifies the hub which forwards to WebSocket clients through the
	// HubNotificationBridge.

	return &SendMailResponse{
		MessageId: messageID,
		ThreadId:  threadID,
	}, nil
}

// FetchInbox retrieves messages from an agent's inbox.
// If agent_id is not provided, uses the default "User" agent.
// If sender_ids is provided, returns messages sent by those agents.
func (s *Server) FetchInbox(ctx context.Context, req *FetchInboxRequest) (*FetchInboxResponse, error) {
	limit := 50
	if req.Limit > 0 {
		limit = int(req.Limit)
	}

	// Convert state filter.
	var stateFilter *string
	if req.StateFilter != MessageState_STATE_UNSPECIFIED {
		state := stateToString(req.StateFilter)
		stateFilter = &state
	}

	// If sender_name_prefix is provided, fetch messages from matching senders.
	if req.SenderNamePrefix != "" {
		fetchReq := mail.FetchInboxRequest{
			AgentID:          0, // Not filtering by recipient.
			Limit:            limit,
			UnreadOnly:       req.UnreadOnly,
			StateFilter:      stateFilter,
			SenderNamePrefix: req.SenderNamePrefix,
		}

		resp, err := s.fetchInboxActor(ctx, fetchReq)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal, "failed to fetch inbox: %v", err,
			)
		}
		if resp.Error != nil {
			return nil, status.Errorf(
				codes.Internal, "failed to fetch inbox: %v", resp.Error,
			)
		}

		return &FetchInboxResponse{
			Messages: convertMessages(resp.Messages),
		}, nil
	}

	// Standard inbox fetch. AgentID=0 means global view (all agents).
	fetchReq := mail.FetchInboxRequest{
		AgentID:     req.AgentId,
		Limit:       limit,
		UnreadOnly:  req.UnreadOnly,
		StateFilter: stateFilter,
		SentOnly:    req.SentOnly,
	}

	// Fetch via the shared mail client (actor system).
	resp, err := s.fetchInboxActor(ctx, fetchReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch inbox: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch inbox: %v", resp.Error)
	}
	msgs := resp.Messages

	return &FetchInboxResponse{
		Messages: convertMessages(msgs),
	}, nil
}

// ReadMessage retrieves a single message by ID and marks it as read.
func (s *Server) ReadMessage(ctx context.Context, req *ReadMessageRequest) (*ReadMessageResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.MessageId == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}

	// Read via the shared mail client (actor system).
	resp, err := s.readMessageActor(ctx, req.AgentId, req.MessageId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read message: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to read message: %v", resp.Error)
	}
	msg := resp.Message

	return &ReadMessageResponse{
		Message: convertMessage(msg),
	}, nil
}

// ReadThread retrieves all messages in a thread.
// If agent_id is not provided, uses the default "User" agent.
func (s *Server) ReadThread(ctx context.Context, req *ReadThreadRequest) (*ReadThreadResponse, error) {
	agentID := req.AgentId
	if agentID == 0 {
		// Use the default "User" agent for thread view.
		userAgent, err := s.store.Queries().GetAgentByName(ctx, "User")
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get User agent")
		}
		agentID = userAgent.ID
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	msgs, err := s.mailSvc.ReadThread(ctx, agentID, req.ThreadId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thread: %v", err)
	}

	return &ReadThreadResponse{
		Messages: convertMessages(msgs),
	}, nil
}

// UpdateState changes the state of a message.
func (s *Server) UpdateState(ctx context.Context, req *UpdateStateRequest) (*UpdateStateResponse, error) {
	agentID := req.AgentId
	if agentID == 0 {
		// Use the default "User" agent for web UI state changes.
		userAgent, err := s.store.Queries().GetAgentByName(
			ctx, "User",
		)
		if err != nil {
			return nil, status.Error(
				codes.Internal,
				"failed to get User agent",
			)
		}
		agentID = userAgent.ID
	}
	if req.MessageId == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.NewState == MessageState_STATE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "new_state is required")
	}

	newState := stateToString(req.NewState)
	var snoozedUntil *time.Time
	if req.NewState == MessageState_STATE_SNOOZED {
		if req.SnoozedUntil == nil || !req.SnoozedUntil.IsValid() {
			return nil, status.Error(codes.InvalidArgument, "snoozed_until is required for STATE_SNOOZED")
		}
		t := req.SnoozedUntil.AsTime()
		snoozedUntil = &t
	}

	// Update state via the shared mail client (actor system).
	resp, err := s.updateMessageStateActor(
		ctx, agentID, req.MessageId, newState, snoozedUntil,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update state: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to update state: %v", resp.Error)
	}

	return &UpdateStateResponse{Success: true}, nil
}

// AckMessage acknowledges receipt of a message.
func (s *Server) AckMessage(ctx context.Context, req *AckMessageRequest) (*AckMessageResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.MessageId == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}

	// Ack via the shared mail client (actor system).
	resp, err := s.ackMessageActor(ctx, req.AgentId, req.MessageId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ack message: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to ack message: %v", resp.Error)
	}

	return &AckMessageResponse{Success: true}, nil
}

// GetStatus returns the mail status for an agent.
func (s *Server) GetStatus(ctx context.Context, req *GetStatusRequest) (*GetStatusResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Get status via the shared mail client (actor system).
	resp, err := s.getAgentStatusActor(ctx, req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get status: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to get status: %v", resp.Error)
	}
	stat := resp.Status

	return &GetStatusResponse{
		AgentId:      stat.AgentID,
		AgentName:    stat.AgentName,
		UnreadCount:  stat.UnreadCount,
		UrgentCount:  stat.UrgentCount,
		StarredCount: stat.StarredCount,
		SnoozedCount: stat.SnoozedCount,
	}, nil
}

// PollChanges checks for new messages since given offsets.
func (s *Server) PollChanges(ctx context.Context, req *PollChangesRequest) (*PollChangesResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Poll changes via the shared mail client (actor system).
	resp, err := s.pollChangesActor(ctx, req.AgentId, req.SinceOffsets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to poll changes: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to poll changes: %v", resp.Error)
	}
	newMessages := resp.NewMessages
	newOffsets := resp.NewOffsets

	return &PollChangesResponse{
		NewMessages: convertMessages(newMessages),
		NewOffsets:  newOffsets,
	}, nil
}

// SubscribeInbox creates a stream of new inbox messages using the actor-based
// notification hub for event-driven delivery.
func (s *Server) SubscribeInbox(req *SubscribeInboxRequest, stream Mail_SubscribeInboxServer) error {
	if req.AgentId == 0 {
		return status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Create a buffered channel for message delivery.
	msgCh := make(chan mail.InboxMessage, 100)

	// Generate a unique subscriber ID for this stream.
	subscriberID := fmt.Sprintf("grpc-stream-%d-%d", req.AgentId, time.Now().UnixNano())

	// Subscribe to the notification hub.
	ctx := stream.Context()
	subFuture := s.notificationHub.Ask(ctx, mail.SubscribeAgentMsg{
		AgentID:      req.AgentId,
		SubscriberID: subscriberID,
		DeliveryChan: msgCh,
	})

	subResp := subFuture.Await(ctx)
	if _, err := subResp.Unpack(); err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}

	// Ensure we unsubscribe when the stream ends.
	defer func() {
		// Use a background context for cleanup since stream context may be cancelled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		unsubFuture := s.notificationHub.Ask(cleanupCtx, mail.UnsubscribeAgentMsg{
			AgentID:      req.AgentId,
			SubscriberID: subscriberID,
		})
		unsubFuture.Await(cleanupCtx)
		close(msgCh)
	}()

	// Stream messages to the client.
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-s.quit:
			return status.Error(codes.Unavailable, "server shutting down")
		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			if err := stream.Send(convertMessage(&msg)); err != nil {
				return err
			}
		}
	}
}

// Publish sends a message to a pub/sub topic.
func (s *Server) Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	if req.SenderId == 0 {
		return nil, status.Error(codes.InvalidArgument, "sender_id is required")
	}
	if req.TopicName == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_name is required")
	}

	// Convert priority.
	priority := mail.PriorityNormal
	switch req.Priority {
	case Priority_PRIORITY_LOW:
		priority = mail.PriorityLow
	case Priority_PRIORITY_URGENT:
		priority = mail.PriorityUrgent
	}

	pubReq := mail.PublishRequest{
		SenderID:  req.SenderId,
		TopicName: req.TopicName,
		Subject:   req.Subject,
		Body:      req.Body,
		Priority:  priority,
	}

	// Publish via the shared mail client (actor system).
	resp, err := s.publishMessageActor(ctx, pubReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish: %v", resp.Error)
	}
	messageID := resp.MessageID
	recipientsCount := resp.RecipientsCount

	return &PublishResponse{
		MessageId:       messageID,
		RecipientsCount: int32(recipientsCount),
	}, nil
}

// Subscribe subscribes an agent to a topic.
func (s *Server) Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.TopicName == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_name is required")
	}

	topicID, err := s.mailSvc.Subscribe(ctx, req.AgentId, req.TopicName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}

	return &SubscribeResponse{
		Success: true,
		TopicId: topicID,
	}, nil
}

// Unsubscribe removes an agent's subscription to a topic.
func (s *Server) Unsubscribe(ctx context.Context, req *UnsubscribeRequest) (*UnsubscribeResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.TopicName == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_name is required")
	}

	err := s.mailSvc.Unsubscribe(ctx, req.AgentId, req.TopicName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unsubscribe: %v", err)
	}

	return &UnsubscribeResponse{Success: true}, nil
}

// ListTopics lists available topics.
func (s *Server) ListTopics(ctx context.Context, req *ListTopicsRequest) (*ListTopicsResponse, error) {
	topics, err := s.mailSvc.ListTopics(ctx, mail.ListTopicsRequest{
		AgentID:        req.AgentId,
		SubscribedOnly: req.SubscribedOnly,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list topics: %v", err)
	}

	protoTopics := make([]*Topic, len(topics))
	for i, t := range topics {
		protoTopics[i] = &Topic{
			Id:           t.ID,
			Name:         t.Name,
			TopicType:    t.TopicType,
			CreatedAt:    timestamppb.New(t.CreatedAt),
			MessageCount: t.MessageCount,
		}
	}

	return &ListTopicsResponse{Topics: protoTopics}, nil
}

// Search performs full-text search across messages.
// If agent_id is 0, performs global search across all messages.
func (s *Server) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int32(50)
	if req.Limit > 0 {
		limit = req.Limit
	}

	results, err := s.mailSvc.Search(ctx, mail.SearchRequest{
		AgentID: req.AgentId, // 0 = global search
		Query:   req.Query,
		TopicID: req.TopicId,
		Limit:   limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search: %v", err)
	}

	return &SearchResponse{
		Results: convertMessages(results),
	}, nil
}

// HasUnackedStatusTo checks if there are unacked status messages from sender
// to recipient. Used for deduplication in status-update command.
func (s *Server) HasUnackedStatusTo(
	ctx context.Context, req *HasUnackedStatusToRequest,
) (*HasUnackedStatusToResponse, error) {
	if req.SenderId == 0 {
		return nil, status.Error(codes.InvalidArgument, "sender_id is required")
	}
	if req.RecipientId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "recipient_id is required",
		)
	}

	count, err := s.store.Queries().HasUnackedStatusToAgent(
		ctx, sqlc.HasUnackedStatusToAgentParams{
			SenderID: req.SenderId,
			AgentID:  req.RecipientId,
		},
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to check status messages: %v", err,
		)
	}

	return &HasUnackedStatusToResponse{
		HasPending: count > 0,
	}, nil
}

// ReplyToThread sends a reply message to an existing thread.
func (s *Server) ReplyToThread(
	ctx context.Context, req *ReplyToThreadRequest,
) (*ReplyToThreadResponse, error) {
	senderID := req.SenderId
	if senderID == 0 {
		// Use the default "User" agent.
		userAgent, err := s.store.Queries().GetAgentByName(ctx, "User")
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get User agent")
		}
		senderID = userAgent.ID
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}
	if req.Body == "" {
		return nil, status.Error(codes.InvalidArgument, "body is required")
	}

	// Get the original thread to find recipients and subject.
	msgs, err := s.mailSvc.ReadThread(ctx, senderID, req.ThreadId)
	if err != nil || len(msgs) == 0 {
		return nil, status.Errorf(codes.NotFound, "thread not found: %v", err)
	}

	// Get the original message to extract subject and recipients.
	origMsg := msgs[0]
	subject := origMsg.Subject
	if !hasPrefix(subject, "Re: ") {
		subject = "Re: " + subject
	}

	// Build recipient list - reply to the original sender.
	var recipientNames []string
	if origMsg.SenderID != senderID {
		// Look up the sender's name.
		sender, err := s.store.Queries().GetAgent(ctx, origMsg.SenderID)
		if err == nil {
			recipientNames = append(recipientNames, sender.Name)
		}
	}

	// Send the reply.
	sendReq := mail.SendMailRequest{
		SenderID:       senderID,
		RecipientNames: recipientNames,
		ThreadID:       req.ThreadId,
		Subject:        subject,
		Body:           req.Body,
		Priority:       mail.PriorityNormal,
	}

	resp, err := s.sendMailActor(ctx, sendReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to send reply: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to send reply: %v", resp.Error)
	}

	return &ReplyToThreadResponse{
		MessageId: resp.MessageID,
	}, nil
}

// hasPrefix checks if a string has a prefix (case-insensitive).
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ArchiveThread archives all messages in a thread for an agent.
func (s *Server) ArchiveThread(
	ctx context.Context, req *ArchiveThreadRequest,
) (*ArchiveThreadResponse, error) {
	agentID := req.AgentId
	if agentID == 0 {
		// Use the default "User" agent.
		userAgent, err := s.store.Queries().GetAgentByName(ctx, "User")
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get User agent")
		}
		agentID = userAgent.ID
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	// Get all messages in the thread.
	msgs, err := s.mailSvc.ReadThread(ctx, agentID, req.ThreadId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thread: %v", err)
	}

	// Archive each message.
	archivedCount := 0
	for _, msg := range msgs {
		resp, err := s.updateMessageStateActor(
			ctx, agentID, msg.ID, mail.StateArchivedStr.String(), nil,
		)
		if err != nil {
			continue
		}
		if resp.Error == nil {
			archivedCount++
		}
	}

	return &ArchiveThreadResponse{
		Success:          true,
		MessagesArchived: int32(archivedCount),
	}, nil
}

// DeleteThread moves all messages in a thread to trash.
func (s *Server) DeleteThread(
	ctx context.Context, req *DeleteThreadRequest,
) (*DeleteThreadResponse, error) {
	agentID := req.AgentId
	if agentID == 0 {
		// Use the default "User" agent.
		userAgent, err := s.store.Queries().GetAgentByName(ctx, "User")
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get User agent")
		}
		agentID = userAgent.ID
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	// Get all messages in the thread.
	msgs, err := s.mailSvc.ReadThread(ctx, agentID, req.ThreadId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thread: %v", err)
	}

	// Move each message to trash.
	deletedCount := 0
	for _, msg := range msgs {
		resp, err := s.updateMessageStateActor(
			ctx, agentID, msg.ID, mail.StateTrashStr.String(), nil,
		)
		if err != nil {
			continue
		}
		if resp.Error == nil {
			deletedCount++
		}
	}

	return &DeleteThreadResponse{
		Success:         true,
		MessagesDeleted: int32(deletedCount),
	}, nil
}

// MarkThreadUnread marks all messages in a thread as unread.
func (s *Server) MarkThreadUnread(
	ctx context.Context, req *MarkThreadUnreadRequest,
) (*MarkThreadUnreadResponse, error) {
	agentID := req.AgentId
	if agentID == 0 {
		// Use the default "User" agent.
		userAgent, err := s.store.Queries().GetAgentByName(ctx, "User")
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get User agent")
		}
		agentID = userAgent.ID
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	// Get all messages in the thread.
	msgs, err := s.mailSvc.ReadThread(ctx, agentID, req.ThreadId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thread: %v", err)
	}

	// Mark each message as unread.
	for _, msg := range msgs {
		_, _ = s.updateMessageStateActor(
			ctx, agentID, msg.ID, mail.StateUnreadStr.String(), nil,
		)
	}

	return &MarkThreadUnreadResponse{Success: true}, nil
}

// DeleteMessage moves a single message to trash.
func (s *Server) DeleteMessage(
	ctx context.Context, req *DeleteMessageRequest,
) (*DeleteMessageResponse, error) {
	if req.MessageId == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}

	// If mark_sender_deleted is true, mark the message as deleted from sender's
	// perspective. This is used for aggregate views like CodeReviewer where
	// messages are filtered by sender_name_prefix.
	if req.MarkSenderDeleted {
		// Look up the message to get the sender ID.
		msg, err := s.store.Queries().GetMessage(ctx, req.MessageId)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal, "failed to get message: %v", err,
			)
		}
		// Mark the message as deleted by sender.
		err = s.store.Queries().MarkMessageDeletedBySender(
			ctx, sqlc.MarkMessageDeletedBySenderParams{
				ID:       req.MessageId,
				SenderID: msg.SenderID,
			},
		)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal, "failed to mark message deleted by sender: %v",
				err,
			)
		}
		return &DeleteMessageResponse{Success: true}, nil
	}

	agentID := req.AgentId
	if agentID == 0 {
		// Global view: look up the actual recipient for this message.
		recipients, err := s.store.Queries().GetMessageRecipients(
			ctx, req.MessageId,
		)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal, "failed to get message recipients: %v", err,
			)
		}
		if len(recipients) == 0 {
			return nil, status.Error(
				codes.NotFound, "no recipients found for message",
			)
		}
		// Use the first recipient's agent ID.
		agentID = recipients[0].AgentID
	}

	// Move the message to trash.
	resp, err := s.updateMessageStateActor(
		ctx, agentID, req.MessageId, mail.StateTrashStr.String(), nil,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete message: %v", err)
	}
	if resp.Error != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to delete message: %v", resp.Error,
		)
	}

	return &DeleteMessageResponse{Success: true}, nil
}

// Helper functions.

func stateToString(s MessageState) string {
	switch s {
	case MessageState_STATE_UNREAD:
		return mail.StateUnreadStr.String()
	case MessageState_STATE_READ:
		return mail.StateReadStr.String()
	case MessageState_STATE_STARRED:
		return mail.StateStarredStr.String()
	case MessageState_STATE_SNOOZED:
		return mail.StateSnoozedStr.String()
	case MessageState_STATE_ARCHIVED:
		return mail.StateArchivedStr.String()
	case MessageState_STATE_TRASH:
		return mail.StateTrashStr.String()
	default:
		return mail.StateUnreadStr.String()
	}
}

func stringToState(s string) MessageState {
	switch mail.RecipientState(s) {
	case mail.StateUnreadStr:
		return MessageState_STATE_UNREAD
	case mail.StateReadStr:
		return MessageState_STATE_READ
	case mail.StateStarredStr:
		return MessageState_STATE_STARRED
	case mail.StateSnoozedStr:
		return MessageState_STATE_SNOOZED
	case mail.StateArchivedStr:
		return MessageState_STATE_ARCHIVED
	case mail.StateTrashStr:
		return MessageState_STATE_TRASH
	default:
		return MessageState_STATE_UNREAD
	}
}

func convertMessage(m *mail.InboxMessage) *InboxMessage {
	if m == nil {
		return nil
	}

	state := stringToState(m.State)
	priority := Priority_PRIORITY_NORMAL
	switch m.Priority {
	case mail.PriorityLow:
		priority = Priority_PRIORITY_LOW
	case mail.PriorityUrgent:
		priority = Priority_PRIORITY_URGENT
	}

	return &InboxMessage{
		Id:               m.ID,
		ThreadId:         m.ThreadID,
		TopicId:          m.TopicID,
		SenderId:         m.SenderID,
		SenderName:       m.SenderName,
		SenderProjectKey: m.SenderProjectKey,
		SenderGitBranch:  m.SenderGitBranch,
		Subject:          m.Subject,
		Body:             m.Body,
		Priority:         priority,
		State:            state,
		CreatedAt:        timestamppb.New(m.CreatedAt),
		DeadlineAt:       timeToTimestamp(m.Deadline),
		SnoozedUntil:     timeToTimestamp(m.SnoozedUntil),
		ReadAt:           timeToTimestamp(m.ReadAt),
		AcknowledgedAt:   timeToTimestamp(m.AckedAt),
	}
}

func convertMessages(msgs []mail.InboxMessage) []*InboxMessage {
	result := make([]*InboxMessage, len(msgs))
	for i := range msgs {
		result[i] = convertMessage(&msgs[i])
	}
	return result
}

// timeToTimestamp converts a *time.Time to a protobuf Timestamp.
// Returns nil if t is nil or zero (serializes as null in JSON).
func timeToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// int64ToTimestamp converts a Unix timestamp int64 to a protobuf Timestamp.
// Returns nil if ts is 0 (serializes as null in JSON).
func int64ToTimestamp(ts int64) *timestamppb.Timestamp {
	if ts == 0 {
		return nil
	}
	return timestamppb.New(time.Unix(ts, 0))
}

// Ensure we implement the interfaces.
var (
	_ MailServer  = (*Server)(nil)
	_ AgentServer = (*Server)(nil)
)

// =============================================================================
// Agent RPCs
// =============================================================================

// ListAgents lists all registered agents.
func (s *Server) ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error) {
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agents: %v", err)
	}

	resp := &ListAgentsResponse{
		Agents: make([]*GetAgentResponse, len(agents)),
	}
	for i, a := range agents {
		resp.Agents[i] = &GetAgentResponse{
			Id:        a.ID,
			Name:      a.Name,
			CreatedAt: int64ToTimestamp(a.CreatedAt),
		}
	}

	return resp, nil
}

// DeleteAgent removes an agent by ID.
func (s *Server) DeleteAgent(ctx context.Context, req *DeleteAgentRequest) (*DeleteAgentResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent id is required")
	}

	err := s.agentReg.DeleteAgent(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete agent: %v", err)
	}

	return &DeleteAgentResponse{Success: true}, nil
}

// RegisterAgent creates a new agent with the given name.
func (s *Server) RegisterAgent(ctx context.Context, req *RegisterAgentRequest) (*RegisterAgentResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Note: git_branch is not in the proto, will be set via EnsureIdentity.
	agent, err := s.agentReg.RegisterAgent(ctx, req.Name, req.ProjectKey, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register agent: %v", err)
	}

	return &RegisterAgentResponse{
		AgentId: agent.ID,
		Name:    agent.Name,
	}, nil
}

// GetAgent retrieves an agent by ID or name.
func (s *Server) GetAgent(ctx context.Context, req *GetAgentRequest) (*GetAgentResponse, error) {
	if req.AgentId != 0 {
		a, err := s.store.Queries().GetAgent(ctx, req.AgentId)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
		}
		return &GetAgentResponse{
			Id:        a.ID,
			Name:      a.Name,
			CreatedAt: int64ToTimestamp(a.CreatedAt),
		}, nil
	}

	if req.Name != "" {
		a, err := s.store.Queries().GetAgentByName(ctx, req.Name)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
		}
		return &GetAgentResponse{
			Id:        a.ID,
			Name:      a.Name,
			CreatedAt: int64ToTimestamp(a.CreatedAt),
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "agent_id or name is required")
}

// EnsureIdentity creates or retrieves an agent identity for a session.
func (s *Server) EnsureIdentity(ctx context.Context, req *EnsureIdentityRequest) (*EnsureIdentityResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	identity, err := s.identityMgr.EnsureIdentity(
		ctx, req.SessionId, req.ProjectDir, req.GitBranch,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ensure identity: %v", err)
	}

	return &EnsureIdentityResponse{
		AgentId:   identity.AgentID,
		AgentName: identity.AgentName,
		Created:   identity.CreatedAt.After(identity.LastActiveAt.Add(-1 * time.Second)),
	}, nil
}

// SaveIdentity persists an agent's current state.
func (s *Server) SaveIdentity(ctx context.Context, req *SaveIdentityRequest) (*SaveIdentityResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Convert consumer offsets from topic IDs to topic names.
	offsets := make(map[string]int64)
	for topicID, offset := range req.ConsumerOffsets {
		topic, err := s.store.Queries().GetTopic(ctx, topicID)
		if err != nil {
			// Skip topics that don't exist.
			continue
		}
		offsets[topic.Name] = offset
	}

	// Construct an IdentityFile from the request.
	identity := &agent.IdentityFile{
		SessionID:       req.SessionId,
		AgentID:         req.AgentId,
		ConsumerOffsets: offsets,
	}

	err := s.identityMgr.SaveIdentity(ctx, identity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save identity: %v", err)
	}

	return &SaveIdentityResponse{Success: true}, nil
}

// HealthCheck returns the health status of the server.
func (s *Server) HealthCheck(
	ctx context.Context, req *HealthCheckRequest,
) (*HealthCheckResponse, error) {
	return &HealthCheckResponse{
		Status: "ok",
		Time:   timestamppb.Now(),
	}, nil
}

// GetDashboardStats returns dashboard statistics.
func (s *Server) GetDashboardStats(
	ctx context.Context, req *GetDashboardStatsRequest,
) (*GetDashboardStatsResponse, error) {
	// TODO: Implement actual stats queries.
	return &GetDashboardStatsResponse{
		Stats: &DashboardStats{
			ActiveAgents:    0,
			RunningSessions: 0,
			PendingMessages: 0,
			CompletedToday:  0,
		},
	}, nil
}

// GetAgentsStatus returns all agents with their current status and counts.
func (s *Server) GetAgentsStatus(
	ctx context.Context, req *GetAgentsStatusRequest,
) (*GetAgentsStatusResponse, error) {
	// Get all agents with their computed status.
	agents, err := s.heartbeatMgr.ListAgentsWithStatus(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agents: %v", err)
	}

	// Get status counts.
	counts, err := s.heartbeatMgr.GetStatusCounts(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get status counts: %v", err)
	}

	// Convert to proto types.
	protoAgents := make([]*AgentWithStatus, len(agents))
	for i, aws := range agents {
		protoAgents[i] = &AgentWithStatus{
			Id:                    aws.Agent.ID,
			Name:                  aws.Agent.Name,
			ProjectKey:            aws.Agent.ProjectKey.String,
			GitBranch:             aws.Agent.GitBranch.String,
			Status:                agentStatusToProto(aws.Status),
			LastActiveAt:          timestamppb.New(aws.LastActive),
			SecondsSinceHeartbeat: int64(time.Since(aws.LastActive).Seconds()),
		}
		// Note: Session IDs are strings in the identity system, but the proto
		// uses int64. Proto session_id is 0 since we use Claude session strings.
	}

	return &GetAgentsStatusResponse{
		Agents: protoAgents,
		Counts: &AgentStatusCounts{
			Active:  int32(counts.Active),
			Busy:    int32(counts.Busy),
			Idle:    int32(counts.Idle),
			Offline: int32(counts.Offline),
		},
	}, nil
}

// Heartbeat records a heartbeat for an agent.
// Either agent_id or agent_name must be provided. If both are given,
// agent_id takes precedence.
func (s *Server) Heartbeat(
	ctx context.Context, req *HeartbeatRequest,
) (*HeartbeatResponse, error) {
	agentID := req.AgentId

	// If agent_id is not provided, look up by agent_name.
	if agentID == 0 {
		if req.AgentName == "" {
			return nil, status.Error(
				codes.InvalidArgument,
				"either agent_id or agent_name is required",
			)
		}

		agentRow, err := s.store.Queries().GetAgentByName(ctx, req.AgentName)
		if err != nil {
			return nil, status.Errorf(
				codes.NotFound, "agent not found: %s", req.AgentName,
			)
		}
		agentID = agentRow.ID
	}

	// Record the heartbeat.
	err := s.heartbeatMgr.RecordHeartbeat(ctx, agentID)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to record heartbeat: %v", err,
		)
	}

	// Start session tracking if session_id is provided.
	if req.SessionId != "" {
		s.heartbeatMgr.StartSession(agentID, req.SessionId)
	}

	return &HeartbeatResponse{Success: true}, nil
}

// agentStatusToProto converts internal agent status to proto enum.
func agentStatusToProto(s agent.AgentStatus) AgentStatus {
	switch s {
	case agent.StatusActive:
		return AgentStatus_AGENT_STATUS_ACTIVE
	case agent.StatusBusy:
		return AgentStatus_AGENT_STATUS_BUSY
	case agent.StatusIdle:
		return AgentStatus_AGENT_STATUS_IDLE
	case agent.StatusOffline:
		return AgentStatus_AGENT_STATUS_OFFLINE
	default:
		return AgentStatus_AGENT_STATUS_UNSPECIFIED
	}
}

// =============================================================================
// Session RPCs
// =============================================================================

// ListSessions lists sessions with optional filters.
func (s *Server) ListSessions(
	ctx context.Context, req *ListSessionsRequest,
) (*ListSessionsResponse, error) {
	// Get all session identities from the database.
	agents, err := s.store.Queries().ListAgents(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agents: %v", err)
	}

	var sessions []*SessionInfo
	limit := 50
	if req.Limit > 0 {
		limit = int(req.Limit)
	}

	for _, a := range agents {
		// Check if agent has an active session.
		sessionID := s.heartbeatMgr.GetActiveSessionID(a.ID)
		if sessionID == "" && req.ActiveOnly {
			continue
		}
		if sessionID == "" {
			continue // Skip agents without sessions.
		}

		// Build session info from agent data.
		session := &SessionInfo{
			Id:        a.ID, // Use agent ID as session ID.
			AgentId:   a.ID,
			AgentName: a.Name,
			Project:   a.ProjectKey.String,
			Branch:    a.GitBranch.String,
			StartedAt: int64ToTimestamp(a.LastActiveAt),
			Status:    SessionStatus_SESSION_STATUS_ACTIVE,
		}

		sessions = append(sessions, session)
		if len(sessions) >= limit {
			break
		}
	}

	return &ListSessionsResponse{Sessions: sessions}, nil
}

// GetSession retrieves a single session by ID.
func (s *Server) GetSession(
	ctx context.Context, req *GetSessionRequest,
) (*GetSessionResponse, error) {
	if req.SessionId == 0 {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Session ID maps to agent ID in our simplified model.
	agent, err := s.store.Queries().GetAgent(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "session not found: %v", err)
	}

	// Check if there's an active session.
	sessionID := s.heartbeatMgr.GetActiveSessionID(agent.ID)
	sessionStatus := SessionStatus_SESSION_STATUS_COMPLETED
	if sessionID != "" {
		sessionStatus = SessionStatus_SESSION_STATUS_ACTIVE
	}

	session := &SessionInfo{
		Id:        agent.ID,
		AgentId:   agent.ID,
		AgentName: agent.Name,
		Project:   agent.ProjectKey.String,
		Branch:    agent.GitBranch.String,
		StartedAt: int64ToTimestamp(agent.LastActiveAt),
		Status:    sessionStatus,
	}

	return &GetSessionResponse{Session: session}, nil
}

// StartSession starts a new session for an agent.
func (s *Server) StartSession(
	ctx context.Context, req *StartSessionRequest,
) (*StartSessionResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Get the agent.
	agent, err := s.store.Queries().GetAgent(ctx, req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
	}

	// Generate a session ID and mark the agent as having an active session.
	sessionID := fmt.Sprintf("session-%d-%d", req.AgentId, time.Now().Unix())
	s.heartbeatMgr.StartSession(req.AgentId, sessionID)

	// Record activity for session start.
	_, _ = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      req.AgentId,
		ActivityType: "session_start",
		Description:  fmt.Sprintf("Started session for %s", agent.Name),
		Metadata:     sql.NullString{},
		CreatedAt:    time.Now().Unix(),
	})

	session := &SessionInfo{
		Id:        agent.ID,
		AgentId:   agent.ID,
		AgentName: agent.Name,
		Project:   req.Project,
		Branch:    req.Branch,
		StartedAt: timestamppb.Now(),
		Status:    SessionStatus_SESSION_STATUS_ACTIVE,
	}

	return &StartSessionResponse{Session: session}, nil
}

// CompleteSession marks a session as completed.
func (s *Server) CompleteSession(
	ctx context.Context, req *CompleteSessionRequest,
) (*CompleteSessionResponse, error) {
	if req.SessionId == 0 {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Session ID maps to agent ID.
	s.heartbeatMgr.EndSession(req.SessionId)

	// Record activity for session completion.
	agent, err := s.store.Queries().GetAgent(ctx, req.SessionId)
	if err == nil {
		_, _ = s.store.Queries().CreateActivity(ctx, sqlc.CreateActivityParams{
			AgentID:      req.SessionId,
			ActivityType: "session_complete",
			Description:  fmt.Sprintf("Completed session for %s", agent.Name),
			Metadata:     sql.NullString{},
			CreatedAt:    time.Now().Unix(),
		})
	}

	return &CompleteSessionResponse{Success: true}, nil
}

// Ensure we implement the Session server interface.
var _ SessionServer = (*Server)(nil)

// =============================================================================
// Activity RPCs
// =============================================================================

// ListActivities lists activities with optional filters.
func (s *Server) ListActivities(
	ctx context.Context, req *ListActivitiesRequest,
) (*ListActivitiesResponse, error) {
	pageSize := int64(20)
	if req.PageSize > 0 {
		pageSize = int64(req.PageSize)
	}

	var activities []sqlc.Activity
	var err error

	// Apply filters based on request.
	if req.AgentId > 0 {
		activities, err = s.store.Queries().ListActivitiesByAgent(
			ctx, sqlc.ListActivitiesByAgentParams{
				AgentID: req.AgentId,
				Limit:   pageSize,
			},
		)
	} else if req.Type != ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		activityType := activityTypeToString(req.Type)
		activities, err = s.store.Queries().ListActivitiesByType(
			ctx, sqlc.ListActivitiesByTypeParams{
				ActivityType: activityType,
				Limit:        pageSize,
			},
		)
	} else {
		activities, err = s.store.Queries().ListRecentActivities(ctx, pageSize)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list activities: %v", err)
	}

	// Convert to proto types.
	protoActivities := make([]*ActivityInfo, len(activities))
	for i, a := range activities {
		// Get agent name for display.
		agentName := ""
		agent, err := s.store.Queries().GetAgent(ctx, a.AgentID)
		if err == nil {
			agentName = agent.Name
		}

		protoActivities[i] = &ActivityInfo{
			Id:           a.ID,
			AgentId:      a.AgentID,
			AgentName:    agentName,
			Type:         stringToActivityType(a.ActivityType),
			Description:  a.Description,
			CreatedAt:    int64ToTimestamp(a.CreatedAt),
			MetadataJson: a.Metadata.String,
		}
	}

	return &ListActivitiesResponse{
		Activities: protoActivities,
		Total:      int64(len(protoActivities)),
		Page:       req.Page,
		PageSize:   int32(pageSize),
	}, nil
}

// activityTypeToString converts proto enum to database string.
func activityTypeToString(t ActivityType) string {
	switch t {
	case ActivityType_ACTIVITY_TYPE_MESSAGE_SENT:
		return "message"
	case ActivityType_ACTIVITY_TYPE_MESSAGE_READ:
		return "message_read"
	case ActivityType_ACTIVITY_TYPE_SESSION_STARTED:
		return "session_start"
	case ActivityType_ACTIVITY_TYPE_SESSION_COMPLETED:
		return "session_complete"
	case ActivityType_ACTIVITY_TYPE_AGENT_REGISTERED:
		return "agent_registered"
	case ActivityType_ACTIVITY_TYPE_HEARTBEAT:
		return "heartbeat"
	default:
		return ""
	}
}

// stringToActivityType converts database string to proto enum.
func stringToActivityType(s string) ActivityType {
	switch s {
	case "message":
		return ActivityType_ACTIVITY_TYPE_MESSAGE_SENT
	case "message_read":
		return ActivityType_ACTIVITY_TYPE_MESSAGE_READ
	case "session_start":
		return ActivityType_ACTIVITY_TYPE_SESSION_STARTED
	case "session_complete":
		return ActivityType_ACTIVITY_TYPE_SESSION_COMPLETED
	case "agent_registered":
		return ActivityType_ACTIVITY_TYPE_AGENT_REGISTERED
	case "heartbeat":
		return ActivityType_ACTIVITY_TYPE_HEARTBEAT
	default:
		return ActivityType_ACTIVITY_TYPE_UNSPECIFIED
	}
}

// Ensure we implement the Activity server interface.
var _ ActivityServer = (*Server)(nil)
