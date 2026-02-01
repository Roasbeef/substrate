package subtraterpc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	if req.DeadlineAt > 0 {
		t := time.Unix(req.DeadlineAt, 0)
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

	// Use actor system if available, otherwise fall back to direct service call.
	var messageID int64
	var threadID string
	if s.mailRef != nil {
		resp, err := s.sendMailActor(ctx, sendReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to send mail: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to send mail: %v", resp.Error)
		}
		messageID = resp.MessageID
		threadID = resp.ThreadID
	} else {
		resp, err := s.mailSvc.Send(ctx, sendReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to send mail: %v", err)
		}
		messageID = resp.MessageID
		threadID = resp.ThreadID
	}

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

	return &SendMailResponse{
		MessageId: messageID,
		ThreadId:  threadID,
	}, nil
}

// FetchInbox retrieves messages from an agent's inbox.
func (s *Server) FetchInbox(ctx context.Context, req *FetchInboxRequest) (*FetchInboxResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

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

	fetchReq := mail.FetchInboxRequest{
		AgentID:     req.AgentId,
		Limit:       limit,
		UnreadOnly:  req.UnreadOnly,
		StateFilter: stateFilter,
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var msgs []mail.InboxMessage
	if s.mailRef != nil {
		resp, err := s.fetchInboxActor(ctx, fetchReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch inbox: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch inbox: %v", resp.Error)
		}
		msgs = resp.Messages
	} else {
		var err error
		msgs, err = s.mailSvc.FetchInbox(ctx, fetchReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch inbox: %v", err)
		}
	}

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

	// Use actor system if available, otherwise fall back to direct service call.
	var msg *mail.InboxMessage
	if s.mailRef != nil {
		resp, err := s.readMessageActor(ctx, req.AgentId, req.MessageId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to read message: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to read message: %v", resp.Error)
		}
		msg = resp.Message
	} else {
		var err error
		msg, err = s.mailSvc.ReadMessage(ctx, req.AgentId, req.MessageId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to read message: %v", err)
		}
	}

	return &ReadMessageResponse{
		Message: convertMessage(msg),
	}, nil
}

// ReadThread retrieves all messages in a thread.
func (s *Server) ReadThread(ctx context.Context, req *ReadThreadRequest) (*ReadThreadResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}

	msgs, err := s.mailSvc.ReadThread(ctx, req.AgentId, req.ThreadId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read thread: %v", err)
	}

	return &ReadThreadResponse{
		Messages: convertMessages(msgs),
	}, nil
}

// UpdateState changes the state of a message.
func (s *Server) UpdateState(ctx context.Context, req *UpdateStateRequest) (*UpdateStateResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
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
		if req.SnoozedUntil == 0 {
			return nil, status.Error(codes.InvalidArgument, "snoozed_until is required for STATE_SNOOZED")
		}
		t := time.Unix(req.SnoozedUntil, 0)
		snoozedUntil = &t
	}

	// Use actor system if available, otherwise fall back to direct service call.
	if s.mailRef != nil {
		resp, err := s.updateMessageStateActor(
			ctx, req.AgentId, req.MessageId, newState, snoozedUntil,
		)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update state: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to update state: %v", resp.Error)
		}
	} else {
		err := s.mailSvc.UpdateState(ctx, mail.UpdateStateRequest{
			AgentID:      req.AgentId,
			MessageID:    req.MessageId,
			NewState:     newState,
			SnoozedUntil: snoozedUntil,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update state: %v", err)
		}
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

	// Use actor system if available, otherwise fall back to direct service call.
	if s.mailRef != nil {
		resp, err := s.ackMessageActor(ctx, req.AgentId, req.MessageId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to ack message: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to ack message: %v", resp.Error)
		}
	} else {
		err := s.mailSvc.AckMessage(ctx, req.AgentId, req.MessageId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to ack message: %v", err)
		}
	}

	return &AckMessageResponse{Success: true}, nil
}

// GetStatus returns the mail status for an agent.
func (s *Server) GetStatus(ctx context.Context, req *GetStatusRequest) (*GetStatusResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var stat mail.AgentStatus
	if s.mailRef != nil {
		resp, err := s.getAgentStatusActor(ctx, req.AgentId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get status: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to get status: %v", resp.Error)
		}
		stat = resp.Status
	} else {
		var err error
		stat, err = s.mailSvc.GetStatus(ctx, req.AgentId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get status: %v", err)
		}
	}

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

	// Use actor system if available, otherwise fall back to direct service call.
	var newMessages []mail.InboxMessage
	var newOffsets map[int64]int64
	if s.mailRef != nil {
		resp, err := s.pollChangesActor(ctx, req.AgentId, req.SinceOffsets)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to poll changes: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to poll changes: %v", resp.Error)
		}
		newMessages = resp.NewMessages
		newOffsets = resp.NewOffsets
	} else {
		resp, err := s.mailSvc.PollChanges(ctx, mail.PollChangesRequest{
			AgentID:      req.AgentId,
			SinceOffsets: req.SinceOffsets,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to poll changes: %v", err)
		}
		newMessages = resp.NewMessages
		newOffsets = resp.NewOffsets
	}

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

	// Use actor system if available, otherwise fall back to direct service call.
	var messageID int64
	var recipientsCount int
	if s.mailRef != nil {
		resp, err := s.publishMessageActor(ctx, pubReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
		}
		if resp.Error != nil {
			return nil, status.Errorf(codes.Internal, "failed to publish: %v", resp.Error)
		}
		messageID = resp.MessageID
		recipientsCount = resp.RecipientsCount
	} else {
		resp, err := s.mailSvc.Publish(ctx, pubReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
		}
		messageID = resp.MessageID
		recipientsCount = resp.RecipientsCount
	}

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
			Id:        t.ID,
			Name:      t.Name,
			TopicType: t.TopicType,
			CreatedAt: t.CreatedAt.Unix(),
		}
	}

	return &ListTopicsResponse{Topics: protoTopics}, nil
}

// Search performs full-text search across messages.
func (s *Server) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req.AgentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int32(50)
	if req.Limit > 0 {
		limit = req.Limit
	}

	results, err := s.mailSvc.Search(ctx, mail.SearchRequest{
		AgentID: req.AgentId,
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
		Id:           m.ID,
		ThreadId:     m.ThreadID,
		TopicId:      m.TopicID,
		SenderId:     m.SenderID,
		SenderName:   m.SenderName,
		Subject:      m.Subject,
		Body:         m.Body,
		Priority:     priority,
		State:        state,
		CreatedAt:    m.CreatedAt.Unix(),
		DeadlineAt:   timeToUnix(m.Deadline),
		SnoozedUntil: timeToUnix(m.SnoozedUntil),
		ReadAt:       timeToUnix(m.ReadAt),
		AckedAt:      timeToUnix(m.AckedAt),
	}
}

func convertMessages(msgs []mail.InboxMessage) []*InboxMessage {
	result := make([]*InboxMessage, len(msgs))
	for i := range msgs {
		result[i] = convertMessage(&msgs[i])
	}
	return result
}

func timeToUnix(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return 0
	}
	return t.Unix()
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
			CreatedAt: a.CreatedAt,
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
			CreatedAt: a.CreatedAt,
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
			CreatedAt: a.CreatedAt,
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
