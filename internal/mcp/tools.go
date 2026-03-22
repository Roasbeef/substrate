package mcp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/roasbeef/subtrate/internal/mail"
)

// requireAgentID validates that agent_id is non-zero. MCP clients that
// omit agent_id send 0, which would create messages from a phantom sender
// or operate on the wrong agent's data.
func requireAgentID(agentID int64) error {
	if agentID == 0 {
		return fmt.Errorf(
			"agent_id is required and must be non-zero",
		)
	}

	return nil
}

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

// handleSendMail sends a message to one or more agent recipients.
func (s *Server) handleSendMail(ctx context.Context,
	req *mcp.CallToolRequest, args SendMailArgs,
) (*mcp.CallToolResult, SendMailResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, SendMailResult{}, err
	}

	priority := mail.Priority(args.Priority)
	if priority == "" {
		priority = mail.PriorityNormal
	}

	resp, err := s.backend.SendMail(ctx, mail.SendMailRequest{
		SenderID:       args.AgentID,
		RecipientNames: args.Recipients,
		Subject:        args.Subject,
		Body:           args.Body,
		Priority:       priority,
		ThreadID:       args.ThreadID,
	})
	if err != nil {
		return nil, SendMailResult{}, err
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

// handleFetchInbox retrieves inbox messages for an agent with pagination.
func (s *Server) handleFetchInbox(ctx context.Context,
	req *mcp.CallToolRequest, args FetchInboxArgs,
) (*mcp.CallToolResult, FetchInboxResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, FetchInboxResult{}, err
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 50
	}

	resp, err := s.backend.FetchInbox(ctx, mail.FetchInboxRequest{
		AgentID:    args.AgentID,
		Limit:      limit,
		UnreadOnly: args.UnreadOnly,
	})
	if err != nil {
		return nil, FetchInboxResult{}, err
	}

	messages := make([]InboxMessageResult, 0, len(resp.Messages))
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

// handleReadMessage reads a specific message and marks it as read.
func (s *Server) handleReadMessage(ctx context.Context,
	req *mcp.CallToolRequest, args ReadMessageArgs,
) (*mcp.CallToolResult, ReadMessageResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, ReadMessageResult{}, err
	}

	resp, err := s.backend.ReadMessage(ctx, args.AgentID, args.MessageID)
	if err != nil {
		return nil, ReadMessageResult{}, err
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

// handleAckMessage acknowledges receipt of a message.
func (s *Server) handleAckMessage(ctx context.Context,
	req *mcp.CallToolRequest, args AckMessageArgs,
) (*mcp.CallToolResult, AckMessageResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, AckMessageResult{}, err
	}

	resp, err := s.backend.AckMessage(ctx, args.AgentID, args.MessageID)
	if err != nil {
		return nil, AckMessageResult{}, err
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

// handleMarkRead marks a message as read for the agent.
func (s *Server) handleMarkRead(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs,
) (*mcp.CallToolResult, StateChangeResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, StateChangeResult{}, err
	}

	resp, err := s.backend.UpdateState(
		ctx, args.AgentID, args.MessageID,
		mail.StateReadStr.String(), nil,
	)
	if err != nil {
		return nil, StateChangeResult{}, err
	}

	return nil, StateChangeResult{Success: resp.Success}, nil
}

// handleStarMessage stars or unstars a message for later reference.
func (s *Server) handleStarMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs,
) (*mcp.CallToolResult, StateChangeResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, StateChangeResult{}, err
	}

	resp, err := s.backend.UpdateState(
		ctx, args.AgentID, args.MessageID,
		mail.StateStarredStr.String(), nil,
	)
	if err != nil {
		return nil, StateChangeResult{}, err
	}

	return nil, StateChangeResult{Success: resp.Success}, nil
}

// SnoozeArgs are the arguments for the snooze_message tool.
type SnoozeArgs struct {
	AgentID      int64  `json:"agent_id" jsonschema:"ID of the agent"`
	MessageID    int64  `json:"message_id" jsonschema:"ID of the message"`
	SnoozedUntil string `json:"snoozed_until" jsonschema:"RFC3339 timestamp when the message should reappear"`
}

// handleSnoozeMessage snoozes a message until a specified time.
func (s *Server) handleSnoozeMessage(ctx context.Context,
	req *mcp.CallToolRequest, args SnoozeArgs,
) (*mcp.CallToolResult, StateChangeResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, StateChangeResult{}, err
	}

	t, err := time.Parse(time.RFC3339, args.SnoozedUntil)
	if err != nil {
		return nil, StateChangeResult{},
			fmt.Errorf("invalid snoozed_until format: %w", err)
	}

	resp, err := s.backend.UpdateState(
		ctx, args.AgentID, args.MessageID,
		mail.StateSnoozedStr.String(), &t,
	)
	if err != nil {
		return nil, StateChangeResult{}, err
	}

	return nil, StateChangeResult{Success: resp.Success}, nil
}

// handleArchiveMessage archives a message to remove it from the inbox.
func (s *Server) handleArchiveMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs,
) (*mcp.CallToolResult, StateChangeResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, StateChangeResult{}, err
	}

	resp, err := s.backend.UpdateState(
		ctx, args.AgentID, args.MessageID,
		mail.StateArchivedStr.String(), nil,
	)
	if err != nil {
		return nil, StateChangeResult{}, err
	}

	return nil, StateChangeResult{Success: resp.Success}, nil
}

// handleTrashMessage moves a message to the trash.
func (s *Server) handleTrashMessage(ctx context.Context,
	req *mcp.CallToolRequest, args StateChangeArgs,
) (*mcp.CallToolResult, StateChangeResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, StateChangeResult{}, err
	}

	resp, err := s.backend.UpdateState(
		ctx, args.AgentID, args.MessageID,
		mail.StateTrashStr.String(), nil,
	)
	if err != nil {
		return nil, StateChangeResult{}, err
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

// handleSubscribe subscribes an agent to a topic by name.
func (s *Server) handleSubscribe(ctx context.Context,
	req *mcp.CallToolRequest, args SubscribeArgs,
) (*mcp.CallToolResult, SubscribeResult, error) {
	err := s.backend.CreateSubscription(
		ctx, args.AgentID, args.TopicName,
	)
	if err != nil {
		return nil, SubscribeResult{},
			fmt.Errorf("failed to subscribe: %w", err)
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

// handleUnsubscribe removes an agent's subscription to a topic.
func (s *Server) handleUnsubscribe(ctx context.Context,
	req *mcp.CallToolRequest, args UnsubscribeArgs,
) (*mcp.CallToolResult, SubscribeResult, error) {
	err := s.backend.DeleteSubscription(
		ctx, args.AgentID, args.TopicName,
	)
	if err != nil {
		return nil, SubscribeResult{},
			fmt.Errorf("failed to unsubscribe: %w", err)
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

// handleListTopics lists available topics, optionally filtered by subscription.
func (s *Server) handleListTopics(ctx context.Context,
	req *mcp.CallToolRequest, args ListTopicsArgs,
) (*mcp.CallToolResult, ListTopicsResult, error) {
	topicsResult := make([]TopicInfo, 0)

	if args.SubscribedOnly && args.AgentID > 0 {
		subs, err := s.backend.ListSubscriptionsByAgent(
			ctx, args.AgentID,
		)
		if err != nil {
			return nil, ListTopicsResult{},
				fmt.Errorf("failed to list subscriptions: %w", err)
		}

		for _, topic := range subs {
			topicsResult = append(topicsResult, TopicInfo{
				ID:   topic.ID,
				Name: topic.Name,
				Type: topic.TopicType,
			})
		}
	} else {
		topics, err := s.backend.ListTopics(ctx)
		if err != nil {
			return nil, ListTopicsResult{},
				fmt.Errorf("failed to list topics: %w", err)
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

// handlePublish publishes a message to a named topic.
func (s *Server) handlePublish(ctx context.Context,
	req *mcp.CallToolRequest, args PublishArgs,
) (*mcp.CallToolResult, PublishResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, PublishResult{}, err
	}

	priority := mail.Priority(args.Priority)
	if priority == "" {
		priority = mail.PriorityNormal
	}

	resp, err := s.backend.Publish(ctx, mail.PublishRequest{
		SenderID:  args.AgentID,
		TopicName: args.TopicName,
		Subject:   args.Subject,
		Body:      args.Body,
		Priority:  priority,
	})
	if err != nil {
		return nil, PublishResult{}, err
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

// handleSearch performs full-text search across messages for an agent.
func (s *Server) handleSearch(ctx context.Context,
	req *mcp.CallToolRequest, args SearchArgs,
) (*mcp.CallToolResult, SearchResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, SearchResult{}, err
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}

	messages, err := s.backend.SearchMessages(
		ctx, args.Query, args.AgentID, limit,
	)
	if err != nil {
		return nil, SearchResult{},
			fmt.Errorf("search failed: %w", err)
	}

	results := make([]InboxMessageResult, 0, len(messages))
	for _, m := range messages {
		results = append(results, InboxMessageResult{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Priority:  m.Priority,
			State:     "",
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

// handleGetStatus returns the mail status summary for an agent.
func (s *Server) handleGetStatus(ctx context.Context,
	req *mcp.CallToolRequest, args GetStatusArgs,
) (*mcp.CallToolResult, GetStatusResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, GetStatusResult{}, err
	}

	resp, err := s.backend.GetStatus(ctx, args.AgentID)
	if err != nil {
		return nil, GetStatusResult{}, err
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
	AgentID      int64            `json:"agent_id" jsonschema:"ID of the agent"`
	SinceOffsets map[string]int64 `json:"since_offsets,omitempty" jsonschema:"Last seen offset per topic ID (keys are topic IDs as strings)"`
}

// PollChangesResult is the result of the poll_changes tool.
type PollChangesResult struct {
	NewMessages []InboxMessageResult `json:"new_messages"`
	NewOffsets  map[string]int64     `json:"new_offsets"`
}

// handlePollChanges polls for new messages since the given topic offsets.
func (s *Server) handlePollChanges(ctx context.Context,
	req *mcp.CallToolRequest, args PollChangesArgs,
) (*mcp.CallToolResult, PollChangesResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, PollChangesResult{}, err
	}

	// Convert string-keyed map to int64-keyed map.
	sinceOffsets := make(map[int64]int64)
	for k, v := range args.SinceOffsets {
		topicID, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, PollChangesResult{},
				fmt.Errorf("invalid topic ID %q: %w", k, err)
		}
		sinceOffsets[topicID] = v
	}

	resp, err := s.backend.PollChanges(
		ctx, args.AgentID, sinceOffsets,
	)
	if err != nil {
		return nil, PollChangesResult{}, err
	}

	messages := make([]InboxMessageResult, 0, len(resp.NewMessages))
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

// handleRegisterAgent creates a new agent with the given name.
func (s *Server) handleRegisterAgent(ctx context.Context,
	req *mcp.CallToolRequest, args RegisterAgentArgs,
) (*mcp.CallToolResult, RegisterAgentResult, error) {
	agent, err := s.backend.RegisterAgent(
		ctx, args.Name, args.ProjectKey, args.GitBranch,
	)
	if err != nil {
		return nil, RegisterAgentResult{},
			fmt.Errorf("failed to register agent: %w", err)
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

// handleWhoAmI returns the current agent identity by ID.
func (s *Server) handleWhoAmI(ctx context.Context,
	req *mcp.CallToolRequest, args WhoAmIArgs,
) (*mcp.CallToolResult, WhoAmIResult, error) {
	agent, err := s.backend.GetAgent(ctx, args.AgentID)
	if err != nil {
		return nil, WhoAmIResult{},
			fmt.Errorf("agent not found: %w", err)
	}

	return nil, WhoAmIResult{
		AgentID:    agent.ID,
		AgentName:  agent.Name,
		ProjectKey: agent.ProjectKey,
	}, nil
}

// ListAgentsArgs are the arguments for the list_agents tool.
type ListAgentsArgs struct{}

// AgentInfo is information about a registered agent.
type AgentInfo struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ProjectKey string `json:"project_key,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
}

// ListAgentsResult is the result of the list_agents tool.
type ListAgentsResult struct {
	Agents []AgentInfo `json:"agents"`
}

// handleListAgents returns all registered agents.
func (s *Server) handleListAgents(ctx context.Context,
	req *mcp.CallToolRequest, args ListAgentsArgs,
) (*mcp.CallToolResult, ListAgentsResult, error) {
	agents, err := s.backend.ListAgents(ctx)
	if err != nil {
		return nil, ListAgentsResult{},
			fmt.Errorf("failed to list agents: %w", err)
	}

	result := make([]AgentInfo, 0, len(agents))
	for _, a := range agents {
		result = append(result, AgentInfo{
			ID:         a.ID,
			Name:       a.Name,
			ProjectKey: a.ProjectKey,
			GitBranch:  a.GitBranch,
		})
	}

	return nil, ListAgentsResult{Agents: result}, nil
}

// GetAgentByNameArgs are the arguments for the get_agent_by_name tool.
type GetAgentByNameArgs struct {
	Name string `json:"name" jsonschema:"Name of the agent to look up"`
}

// handleGetAgentByName looks up an agent by name instead of ID.
func (s *Server) handleGetAgentByName(ctx context.Context,
	req *mcp.CallToolRequest, args GetAgentByNameArgs,
) (*mcp.CallToolResult, WhoAmIResult, error) {
	agent, err := s.backend.GetAgentByName(ctx, args.Name)
	if err != nil {
		return nil, WhoAmIResult{},
			fmt.Errorf("agent %q not found: %w", args.Name, err)
	}

	return nil, WhoAmIResult{
		AgentID:    agent.ID,
		AgentName:  agent.Name,
		ProjectKey: agent.ProjectKey,
	}, nil
}

// HeartbeatArgs are the arguments for the heartbeat tool.
type HeartbeatArgs struct {
	AgentID int64 `json:"agent_id" jsonschema:"ID of the agent sending the heartbeat"`
}

// HeartbeatResult is the result of the heartbeat tool.
type HeartbeatResult struct {
	Success bool `json:"success"`
}

// handleHeartbeat records a liveness signal for an agent.
func (s *Server) handleHeartbeat(ctx context.Context,
	req *mcp.CallToolRequest, args HeartbeatArgs,
) (*mcp.CallToolResult, HeartbeatResult, error) {
	if err := requireAgentID(args.AgentID); err != nil {
		return nil, HeartbeatResult{}, err
	}

	err := s.backend.Heartbeat(ctx, args.AgentID)
	if err != nil {
		return nil, HeartbeatResult{},
			fmt.Errorf("heartbeat failed: %w", err)
	}

	return nil, HeartbeatResult{Success: true}, nil
}

// ReadThreadArgs are the arguments for the read_thread tool.
type ReadThreadArgs struct {
	ThreadID string `json:"thread_id" jsonschema:"Thread ID to read all messages from"`
}

// ReadThreadResult is the result of the read_thread tool.
type ReadThreadResult struct {
	Messages []InboxMessageResult `json:"messages"`
}

// handleReadThread returns all messages in a conversation thread.
func (s *Server) handleReadThread(ctx context.Context,
	req *mcp.CallToolRequest, args ReadThreadArgs,
) (*mcp.CallToolResult, ReadThreadResult, error) {
	msgs, err := s.backend.ReadThread(ctx, args.ThreadID)
	if err != nil {
		return nil, ReadThreadResult{},
			fmt.Errorf("failed to read thread: %w", err)
	}

	result := make([]InboxMessageResult, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, InboxMessageResult{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Body:      m.Body,
			Priority:  string(m.Priority),
			State:     m.State,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, ReadThreadResult{Messages: result}, nil
}
