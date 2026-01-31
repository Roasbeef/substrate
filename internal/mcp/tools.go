package mcp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/mail"
)

// SendMailArgs are the arguments for the send_mail tool.
type SendMailArgs struct {
	// AgentID is the sending agent's ID.
	AgentID int64 `json:"agent_id" jsonschema:"ID of the sending agent"`

	// Recipients is a list of recipient agent names.
	Recipients []string `json:"recipients" jsonschema:"List of recipient agent names"`

	// Subject is the message subject line.
	Subject string `json:"subject" jsonschema:"Message subject line"`

	// Body is the message body in markdown format.
	Body string `json:"body" jsonschema:"Message body in markdown format"`

	// Priority is the message priority (urgent, normal, low).
	Priority string `json:"priority,omitempty" jsonschema:"Priority: urgent, normal, or low,default=normal"`

	// ThreadID is an optional thread ID for threading messages.
	ThreadID string `json:"thread_id,omitempty" jsonschema:"Optional thread ID for threading related messages"`
}

// SendMailResult is the result of the send_mail tool.
type SendMailResult struct {
	MessageID int64  `json:"message_id"`
	ThreadID  string `json:"thread_id"`
}

func (s *Server) handleSendMail(ctx context.Context,
	req *mcp.CallToolRequest, args SendMailArgs) (*mcp.CallToolResult, SendMailResult, error) {

	priority := mail.Priority(args.Priority)
	if priority == "" {
		priority = mail.PriorityNormal
	}

	mailReq := mail.SendMailRequest{
		SenderID:       args.AgentID,
		RecipientNames: args.Recipients,
		Subject:        args.Subject,
		Body:           args.Body,
		Priority:       priority,
		ThreadID:       args.ThreadID,
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.SendMailResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.sendMailActor(ctx, mailReq)
		if err != nil {
			return nil, SendMailResult{}, err
		}
	} else {
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, SendMailResult{}, err
		}
		resp = val.(mail.SendMailResponse)
	}

	if resp.Error != nil {
		return nil, SendMailResult{}, resp.Error
	}

	return nil, SendMailResult{
		MessageID: resp.MessageID,
		ThreadID:  resp.ThreadID,
	}, nil
}

// FetchInboxArgs are the arguments for the fetch_inbox tool.
type FetchInboxArgs struct {
	AgentID    int64 `json:"agent_id" jsonschema:"ID of the agent to fetch inbox for"`
	Limit      int   `json:"limit,omitempty" jsonschema:"Maximum number of messages to return,default=50"`
	UnreadOnly bool  `json:"unread_only,omitempty" jsonschema:"Only return unread messages"`
}

// FetchInboxResult is the result of the fetch_inbox tool.
type FetchInboxResult struct {
	Messages []InboxMessageResult `json:"messages"`
}

// InboxMessageResult is a message in the inbox.
type InboxMessageResult struct {
	ID        int64  `json:"id"`
	ThreadID  string `json:"thread_id"`
	SenderID  int64  `json:"sender_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body,omitempty"`
	Priority  string `json:"priority"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) handleFetchInbox(ctx context.Context,
	req *mcp.CallToolRequest, args FetchInboxArgs) (*mcp.CallToolResult, FetchInboxResult, error) {

	limit := args.Limit
	if limit <= 0 {
		limit = 50
	}

	mailReq := mail.FetchInboxRequest{
		AgentID:    args.AgentID,
		Limit:      limit,
		UnreadOnly: args.UnreadOnly,
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.FetchInboxResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.fetchInboxActor(ctx, mailReq)
		if err != nil {
			return nil, FetchInboxResult{}, err
		}
	} else {
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, FetchInboxResult{}, err
		}
		resp = val.(mail.FetchInboxResponse)
	}

	if resp.Error != nil {
		return nil, FetchInboxResult{}, resp.Error
	}

	var messages []InboxMessageResult
	for _, m := range resp.Messages {
		messages = append(messages, InboxMessageResult{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Priority:  string(m.Priority),
			State:     m.State,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, FetchInboxResult{Messages: messages}, nil
}

// ReadMessageArgs are the arguments for the read_message tool.
type ReadMessageArgs struct {
	AgentID   int64 `json:"agent_id" jsonschema:"ID of the requesting agent"`
	MessageID int64 `json:"message_id" jsonschema:"ID of the message to read"`
}

// ReadMessageResult is the result of the read_message tool.
type ReadMessageResult struct {
	ID        int64  `json:"id"`
	ThreadID  string `json:"thread_id"`
	SenderID  int64  `json:"sender_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Priority  string `json:"priority"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) handleReadMessage(ctx context.Context,
	req *mcp.CallToolRequest, args ReadMessageArgs) (*mcp.CallToolResult, ReadMessageResult, error) {

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.ReadMessageResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.readMessageActor(ctx, args.AgentID, args.MessageID)
		if err != nil {
			return nil, ReadMessageResult{}, err
		}
	} else {
		mailReq := mail.ReadMessageRequest{
			AgentID:   args.AgentID,
			MessageID: args.MessageID,
		}
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, ReadMessageResult{}, err
		}
		resp = val.(mail.ReadMessageResponse)
	}

	if resp.Error != nil {
		return nil, ReadMessageResult{}, resp.Error
	}

	if resp.Message == nil {
		return nil, ReadMessageResult{}, fmt.Errorf("message not found")
	}

	m := resp.Message
	return nil, ReadMessageResult{
		ID:        m.ID,
		ThreadID:  m.ThreadID,
		SenderID:  m.SenderID,
		Subject:   m.Subject,
		Body:      m.Body,
		Priority:  string(m.Priority),
		State:     m.State,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}, nil
}

// AckMessageArgs are the arguments for the ack_message tool.
type AckMessageArgs struct {
	AgentID   int64 `json:"agent_id" jsonschema:"ID of the agent"`
	MessageID int64 `json:"message_id" jsonschema:"ID of the message to acknowledge"`
}

// AckMessageResult is the result of the ack_message tool.
type AckMessageResult struct {
	Success bool `json:"success"`
}

func (s *Server) handleAckMessage(ctx context.Context,
	req *mcp.CallToolRequest, args AckMessageArgs) (*mcp.CallToolResult, AckMessageResult, error) {

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.AckMessageResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.ackMessageActor(ctx, args.AgentID, args.MessageID)
		if err != nil {
			return nil, AckMessageResult{}, err
		}
	} else {
		mailReq := mail.AckMessageRequest{
			AgentID:   args.AgentID,
			MessageID: args.MessageID,
		}
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, AckMessageResult{}, err
		}
		resp = val.(mail.AckMessageResponse)
	}

	if resp.Error != nil {
		return nil, AckMessageResult{}, resp.Error
	}

	return nil, AckMessageResult{Success: resp.Success}, nil
}

// StateChangeArgs are common arguments for state change tools.
type StateChangeArgs struct {
	AgentID   int64 `json:"agent_id" jsonschema:"ID of the agent"`
	MessageID int64 `json:"message_id" jsonschema:"ID of the message"`
}

// StateChangeResult is the result of state change tools.
type StateChangeResult struct {
	Success bool `json:"success"`
}

func (s *Server) handleMarkRead(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs) (*mcp.CallToolResult, StateChangeResult, error) {

	return s.updateState(ctx, args.AgentID, args.MessageID, "read", nil)
}

func (s *Server) handleStarMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs) (*mcp.CallToolResult, StateChangeResult, error) {

	return s.updateState(ctx, args.AgentID, args.MessageID, "starred", nil)
}

// SnoozeArgs are the arguments for the snooze_message tool.
type SnoozeArgs struct {
	AgentID      int64  `json:"agent_id" jsonschema:"ID of the agent"`
	MessageID    int64  `json:"message_id" jsonschema:"ID of the message"`
	SnoozedUntil string `json:"snoozed_until" jsonschema:"RFC3339 timestamp when the message should reappear"`
}

func (s *Server) handleSnoozeMessage(ctx context.Context,
	req *mcp.CallToolRequest, args SnoozeArgs) (*mcp.CallToolResult, StateChangeResult, error) {

	t, err := time.Parse(time.RFC3339, args.SnoozedUntil)
	if err != nil {
		return nil, StateChangeResult{}, fmt.Errorf("invalid snoozed_until format: %w", err)
	}

	return s.updateState(ctx, args.AgentID, args.MessageID, "snoozed", &t)
}

func (s *Server) handleArchiveMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs) (*mcp.CallToolResult, StateChangeResult, error) {

	return s.updateState(ctx, args.AgentID, args.MessageID, "archived", nil)
}

func (s *Server) handleTrashMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs) (*mcp.CallToolResult, StateChangeResult, error) {

	return s.updateState(ctx, args.AgentID, args.MessageID, "trash", nil)
}

func (s *Server) updateState(ctx context.Context, agentID, messageID int64,
	newState string, snoozedUntil *time.Time) (*mcp.CallToolResult, StateChangeResult, error) {

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.UpdateStateResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.updateMessageStateActor(ctx, agentID, messageID, newState, snoozedUntil)
		if err != nil {
			return nil, StateChangeResult{}, err
		}
	} else {
		mailReq := mail.UpdateStateRequest{
			AgentID:      agentID,
			MessageID:    messageID,
			NewState:     newState,
			SnoozedUntil: snoozedUntil,
		}
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, StateChangeResult{}, err
		}
		resp = val.(mail.UpdateStateResponse)
	}

	if resp.Error != nil {
		return nil, StateChangeResult{}, resp.Error
	}

	return nil, StateChangeResult{Success: resp.Success}, nil
}

// SubscribeArgs are the arguments for the subscribe tool.
type SubscribeArgs struct {
	AgentID   int64  `json:"agent_id" jsonschema:"ID of the agent"`
	TopicName string `json:"topic_name" jsonschema:"Name of the topic to subscribe to"`
}

// SubscribeResult is the result of the subscribe tool.
type SubscribeResult struct {
	Success   bool   `json:"success"`
	TopicName string `json:"topic_name"`
}

func (s *Server) handleSubscribe(ctx context.Context,
	req *mcp.CallToolRequest, args SubscribeArgs) (*mcp.CallToolResult, SubscribeResult, error) {

	// Get the topic.
	topic, err := s.storage.GetTopicByName(ctx, args.TopicName)
	if err != nil {
		return nil, SubscribeResult{}, fmt.Errorf("topic %q not found: %w", args.TopicName, err)
	}

	// Create subscription.
	err = s.storage.CreateSubscription(ctx, args.AgentID, topic.ID)
	if err != nil {
		return nil, SubscribeResult{}, fmt.Errorf("failed to subscribe: %w", err)
	}

	return nil, SubscribeResult{
		Success:   true,
		TopicName: args.TopicName,
	}, nil
}

// UnsubscribeArgs are the arguments for the unsubscribe tool.
type UnsubscribeArgs struct {
	AgentID   int64  `json:"agent_id" jsonschema:"ID of the agent"`
	TopicName string `json:"topic_name" jsonschema:"Name of the topic to unsubscribe from"`
}

func (s *Server) handleUnsubscribe(ctx context.Context,
	req *mcp.CallToolRequest, args UnsubscribeArgs) (*mcp.CallToolResult, SubscribeResult, error) {

	// Get the topic.
	topic, err := s.storage.GetTopicByName(ctx, args.TopicName)
	if err != nil {
		return nil, SubscribeResult{}, fmt.Errorf("topic %q not found: %w", args.TopicName, err)
	}

	// Delete subscription.
	err = s.storage.DeleteSubscription(ctx, args.AgentID, topic.ID)
	if err != nil {
		return nil, SubscribeResult{}, fmt.Errorf("failed to unsubscribe: %w", err)
	}

	return nil, SubscribeResult{
		Success:   true,
		TopicName: args.TopicName,
	}, nil
}

// ListTopicsArgs are the arguments for the list_topics tool.
type ListTopicsArgs struct {
	AgentID        int64 `json:"agent_id,omitempty" jsonschema:"If set, only list topics the agent is subscribed to"`
	SubscribedOnly bool  `json:"subscribed_only,omitempty" jsonschema:"Only show topics this agent is subscribed to"`
}

// TopicInfo is information about a topic.
type TopicInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListTopicsResult is the result of the list_topics tool.
type ListTopicsResult struct {
	Topics []TopicInfo `json:"topics"`
}

func (s *Server) handleListTopics(ctx context.Context,
	req *mcp.CallToolRequest, args ListTopicsArgs) (*mcp.CallToolResult, ListTopicsResult, error) {

	var topicsResult []TopicInfo

	if args.SubscribedOnly && args.AgentID > 0 {
		subs, err := s.storage.ListSubscriptionsByAgent(ctx, args.AgentID)
		if err != nil {
			return nil, ListTopicsResult{}, fmt.Errorf("failed to list subscriptions: %w", err)
		}

		for _, topic := range subs {
			topicsResult = append(topicsResult, TopicInfo{
				ID:   topic.ID,
				Name: topic.Name,
				Type: topic.TopicType,
			})
		}
	} else {
		topics, err := s.storage.ListTopics(ctx)
		if err != nil {
			return nil, ListTopicsResult{}, fmt.Errorf("failed to list topics: %w", err)
		}

		for _, topic := range topics {
			topicsResult = append(topicsResult, TopicInfo{
				ID:   topic.ID,
				Name: topic.Name,
				Type: topic.TopicType,
			})
		}
	}

	return nil, ListTopicsResult{Topics: topicsResult}, nil
}

// PublishArgs are the arguments for the publish tool.
type PublishArgs struct {
	AgentID   int64  `json:"agent_id" jsonschema:"ID of the sending agent"`
	TopicName string `json:"topic_name" jsonschema:"Name of the topic to publish to"`
	Subject   string `json:"subject" jsonschema:"Message subject line"`
	Body      string `json:"body" jsonschema:"Message body in markdown format"`
	Priority  string `json:"priority,omitempty" jsonschema:"Priority: urgent, normal, or low,default=normal"`
}

// PublishResult is the result of the publish tool.
type PublishResult struct {
	MessageID       int64 `json:"message_id"`
	RecipientsCount int   `json:"recipients_count"`
}

func (s *Server) handlePublish(ctx context.Context,
	req *mcp.CallToolRequest, args PublishArgs) (*mcp.CallToolResult, PublishResult, error) {

	priority := mail.Priority(args.Priority)
	if priority == "" {
		priority = mail.PriorityNormal
	}

	pubReq := mail.PublishRequest{
		SenderID:  args.AgentID,
		TopicName: args.TopicName,
		Subject:   args.Subject,
		Body:      args.Body,
		Priority:  priority,
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.PublishResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.publishMessageActor(ctx, pubReq)
		if err != nil {
			return nil, PublishResult{}, err
		}
	} else {
		result := s.mailSvc.Receive(ctx, pubReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, PublishResult{}, err
		}
		resp = val.(mail.PublishResponse)
	}

	if resp.Error != nil {
		return nil, PublishResult{}, resp.Error
	}

	return nil, PublishResult{
		MessageID:       resp.MessageID,
		RecipientsCount: resp.RecipientsCount,
	}, nil
}

// SearchArgs are the arguments for the search tool.
type SearchArgs struct {
	AgentID int64  `json:"agent_id" jsonschema:"ID of the agent to search for"`
	Query   string `json:"query" jsonschema:"Search query string"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of results,default=20"`
}

// SearchResult is the result of the search tool.
type SearchResult struct {
	Results []InboxMessageResult `json:"results"`
}

func (s *Server) handleSearch(ctx context.Context,
	req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchResult, error) {

	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}

	messages, err := s.storage.SearchMessagesForAgent(
		ctx, args.Query, args.AgentID, limit,
	)
	if err != nil {
		return nil, SearchResult{}, fmt.Errorf("search failed: %w", err)
	}

	var results []InboxMessageResult
	for _, m := range messages {
		results = append(results, InboxMessageResult{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Priority:  m.Priority,
			State:     "", // Search returns store.Message which has no state.
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, SearchResult{Results: results}, nil
}

// GetStatusArgs are the arguments for the get_status tool.
type GetStatusArgs struct {
	AgentID int64 `json:"agent_id" jsonschema:"ID of the agent"`
}

// GetStatusResult is the result of the get_status tool.
type GetStatusResult struct {
	AgentID     int64  `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	UnreadCount int64  `json:"unread_count"`
	UrgentCount int64  `json:"urgent_count"`
}

func (s *Server) handleGetStatus(ctx context.Context,
	req *mcp.CallToolRequest, args GetStatusArgs) (*mcp.CallToolResult, GetStatusResult, error) {

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.GetStatusResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.getAgentStatusActor(ctx, args.AgentID)
		if err != nil {
			return nil, GetStatusResult{}, err
		}
	} else {
		mailReq := mail.GetStatusRequest{
			AgentID: args.AgentID,
		}
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, GetStatusResult{}, err
		}
		resp = val.(mail.GetStatusResponse)
	}

	if resp.Error != nil {
		return nil, GetStatusResult{}, resp.Error
	}

	return nil, GetStatusResult{
		AgentID:     resp.Status.AgentID,
		AgentName:   resp.Status.AgentName,
		UnreadCount: resp.Status.UnreadCount,
		UrgentCount: resp.Status.UrgentCount,
	}, nil
}

// PollChangesArgs are the arguments for the poll_changes tool.
type PollChangesArgs struct {
	AgentID      int64             `json:"agent_id" jsonschema:"ID of the agent"`
	SinceOffsets map[string]int64  `json:"since_offsets,omitempty" jsonschema:"Last seen offset per topic ID (keys are topic IDs as strings)"`
}

// PollChangesResult is the result of the poll_changes tool.
type PollChangesResult struct {
	NewMessages []InboxMessageResult `json:"new_messages"`
	NewOffsets  map[string]int64     `json:"new_offsets"`
}

func (s *Server) handlePollChanges(ctx context.Context,
	req *mcp.CallToolRequest, args PollChangesArgs) (*mcp.CallToolResult, PollChangesResult, error) {

	// Convert string-keyed map to int64-keyed map for the mail service.
	sinceOffsets := make(map[int64]int64)
	for k, v := range args.SinceOffsets {
		topicID, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, PollChangesResult{}, fmt.Errorf(
				"invalid topic ID %q: %w", k, err,
			)
		}
		sinceOffsets[topicID] = v
	}

	// Use actor system if available, otherwise fall back to direct service call.
	var resp mail.PollChangesResponse
	if s.hasMailActor() {
		var err error
		resp, err = s.pollChangesActor(ctx, args.AgentID, sinceOffsets)
		if err != nil {
			return nil, PollChangesResult{}, err
		}
	} else {
		mailReq := mail.PollChangesRequest{
			AgentID:      args.AgentID,
			SinceOffsets: sinceOffsets,
		}
		result := s.mailSvc.Receive(ctx, mailReq)
		val, err := result.Unpack()
		if err != nil {
			return nil, PollChangesResult{}, err
		}
		resp = val.(mail.PollChangesResponse)
	}

	if resp.Error != nil {
		return nil, PollChangesResult{}, resp.Error
	}

	var messages []InboxMessageResult
	for _, m := range resp.NewMessages {
		messages = append(messages, InboxMessageResult{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Priority:  string(m.Priority),
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	// Convert int64-keyed map to string-keyed map for JSON.
	newOffsets := make(map[string]int64)
	for k, v := range resp.NewOffsets {
		newOffsets[strconv.FormatInt(k, 10)] = v
	}

	return nil, PollChangesResult{
		NewMessages: messages,
		NewOffsets:  newOffsets,
	}, nil
}

// RegisterAgentArgs are the arguments for the register_agent tool.
type RegisterAgentArgs struct {
	Name       string `json:"name" jsonschema:"Name for the new agent"`
	ProjectKey string `json:"project_key,omitempty" jsonschema:"Optional project key to bind the agent to"`
	GitBranch  string `json:"git_branch,omitempty" jsonschema:"Optional git branch name for the agent"`
}

// RegisterAgentResult is the result of the register_agent tool.
type RegisterAgentResult struct {
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name"`
}

func (s *Server) handleRegisterAgent(ctx context.Context,
	req *mcp.CallToolRequest, args RegisterAgentArgs) (*mcp.CallToolResult, RegisterAgentResult, error) {

	agent, err := s.registry.RegisterAgent(
		ctx, args.Name, args.ProjectKey, args.GitBranch,
	)
	if err != nil {
		return nil, RegisterAgentResult{}, fmt.Errorf("failed to register agent: %w", err)
	}

	return nil, RegisterAgentResult{
		AgentID:   agent.ID,
		AgentName: agent.Name,
	}, nil
}

// WhoAmIArgs are the arguments for the whoami tool.
type WhoAmIArgs struct {
	AgentID int64 `json:"agent_id" jsonschema:"ID of the agent to look up"`
}

// WhoAmIResult is the result of the whoami tool.
type WhoAmIResult struct {
	AgentID    int64  `json:"agent_id"`
	AgentName  string `json:"agent_name"`
	ProjectKey string `json:"project_key,omitempty"`
}

func (s *Server) handleWhoAmI(ctx context.Context,
	req *mcp.CallToolRequest, args WhoAmIArgs) (*mcp.CallToolResult, WhoAmIResult, error) {

	agent, err := s.storage.GetAgent(ctx, args.AgentID)
	if err != nil {
		return nil, WhoAmIResult{}, fmt.Errorf("agent not found: %w", err)
	}

	return nil, WhoAmIResult{
		AgentID:    agent.ID,
		AgentName:  agent.Name,
		ProjectKey: agent.ProjectKey,
	}, nil
}
