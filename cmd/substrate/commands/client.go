package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/roasbeef/subtrate/internal/agent"
	subtraterpc "github.com/roasbeef/subtrate/internal/api/grpc"
	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
	"github.com/roasbeef/subtrate/internal/mail"
	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ReviewInfo contains information about a code review.
type ReviewInfo struct {
	ReviewID    string
	ThreadID    string
	RequesterID int64
	PRNumber    *int64
	Branch      string
	BaseBranch  string
	CommitSHA   string
	RepoPath    string
	ReviewType  string
	Priority    string
	State       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// ReviewIterationInfo contains information about a review iteration.
type ReviewIterationInfo struct {
	ID                int64
	ReviewID          string
	IterationNum      int
	ReviewerID        string
	ReviewerSessionID string
	Decision          string
	Summary           string
	FilesReviewed     int
	LinesAnalyzed     int
	DurationMS        int64
	CostUSD           float64
	StartedAt         time.Time
	CompletedAt       *time.Time
}

// ReviewIssueInfo contains information about a review issue.
type ReviewIssueInfo struct {
	ID           int64
	ReviewID     string
	IterationNum int
	IssueType    string
	Severity     string
	FilePath     string
	LineStart    int
	LineEnd      *int
	Title        string
	Description  string
	CodeSnippet  string
	Suggestion   string
	ClaudeMDRef  string
	Status       string
	CreatedAt    time.Time
}

// ReviewStatsInfo contains aggregate review statistics.
type ReviewStatsInfo struct {
	TotalReviews     int64
	Approved         int64
	Pending          int64
	InProgress       int64
	ChangesRequested int64
}

const (
	// defaultGRPCAddr is the default address for the substrated daemon.
	defaultGRPCAddr = "localhost:10009"

	// grpcConnectTimeout is the timeout for connecting to the daemon.
	grpcConnectTimeout = 2 * time.Second
)

// Client provides an interface for CLI operations. It can be backed by either
// a gRPC connection to substrated or direct database access.
type Client struct {
	// When using gRPC mode.
	conn        *grpc.ClientConn
	mailClient  subtraterpc.MailClient
	agentClient subtraterpc.AgentClient

	// When using direct DB mode.
	store       *db.Store
	mailService *mail.Service
	registry    *agent.Registry
	identityMgr *agent.IdentityManager

	// mode indicates the connection mode.
	mode ClientMode

	// grpcAddr is the address used for gRPC connection.
	grpcAddr string
}

// ClientMode indicates how the client is connected.
type ClientMode int

const (
	// ModeGRPC indicates the client is connected via gRPC.
	ModeGRPC ClientMode = iota

	// ModeDirect indicates the client is using direct database access.
	ModeDirect
)

// String returns a human-readable string for the client mode.
func (m ClientMode) String() string {
	switch m {
	case ModeGRPC:
		return "gRPC"
	case ModeDirect:
		return "direct"
	default:
		return "unknown"
	}
}

// getClient returns a Client that tries to connect to the daemon via gRPC
// first, then falls back to direct database access if the daemon is not
// running.
func getClient() (*Client, error) {
	addr := grpcAddr
	if addr == "" {
		addr = defaultGRPCAddr
	}

	// Try gRPC connection first.
	client, err := tryGRPCConnection(addr)
	if err == nil {
		return client, nil
	}

	// If gRPC failed, fall back to direct database access.
	if verbose {
		fmt.Fprintf(os.Stderr, "Note: daemon not running at %s, using direct database access\n", addr)
	}

	return getDirectClient()
}

// tryGRPCConnection attempts to connect to the daemon via gRPC.
func tryGRPCConnection(addr string) (*Client, error) {
	// Use grpc.NewClient (non-blocking) and verify connectivity manually.
	// This replaces deprecated grpc.DialContext with grpc.WithBlock.
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Verify the connection is actually working by making a test call.
	// We use a short timeout to check if the daemon is running.
	ctx, cancel := context.WithTimeout(context.Background(), grpcConnectTimeout)
	defer cancel()

	agentClient := subtraterpc.NewAgentClient(conn)

	// Make a lightweight call to verify connectivity.
	_, err = agentClient.ListAgents(ctx, &subtraterpc.ListAgentsRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("daemon not responding: %w", err)
	}

	return &Client{
		conn:        conn,
		mailClient:  subtraterpc.NewMailClient(conn),
		agentClient: agentClient,
		mode:        ModeGRPC,
		grpcAddr:    addr,
	}, nil
}

// getDirectClient creates a client that directly accesses the database.
func getDirectClient() (*Client, error) {
	dbStore, err := getStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	registry := agent.NewRegistry(dbStore)
	identityMgr, err := agent.NewIdentityManager(dbStore, registry)
	if err != nil {
		dbStore.Close()
		return nil, fmt.Errorf("failed to create identity manager: %w", err)
	}

	return &Client{
		store:       dbStore,
		mailService: mail.NewService(store.FromDB(dbStore.DB())),
		registry:    registry,
		identityMgr: identityMgr,
		mode:        ModeDirect,
	}, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	if c.store != nil {
		return c.store.Close()
	}
	return nil
}

// Mode returns the connection mode of the client.
func (c *Client) Mode() ClientMode {
	return c.mode
}

// EnsureIdentity creates or retrieves an agent identity for a session.
func (c *Client) EnsureIdentity(
	ctx context.Context, sessionID, projectDir, gitBranch string,
) (*agent.IdentityFile, error) {

	if c.mode == ModeGRPC {
		resp, err := c.agentClient.EnsureIdentity(ctx, &subtraterpc.EnsureIdentityRequest{
			SessionId:  sessionID,
			ProjectDir: projectDir,
			GitBranch:  gitBranch,
		})
		if err != nil {
			return nil, err
		}
		return &agent.IdentityFile{
			AgentID:   resp.AgentId,
			AgentName: resp.AgentName,
		}, nil
	}

	return c.identityMgr.EnsureIdentity(ctx, sessionID, projectDir, gitBranch)
}

// GetAgentByName retrieves an agent by name.
func (c *Client) GetAgentByName(ctx context.Context, name string) (*sqlc.Agent, error) {
	if c.mode == ModeGRPC {
		resp, err := c.agentClient.GetAgent(ctx, &subtraterpc.GetAgentRequest{
			Name: name,
		})
		if err != nil {
			return nil, err
		}
		return &sqlc.Agent{
			ID:   resp.Id,
			Name: resp.Name,
		}, nil
	}

	return c.registry.GetAgentByName(ctx, name)
}

// DeleteAgent removes an agent by ID.
func (c *Client) DeleteAgent(ctx context.Context, id int64) error {
	if c.mode == ModeGRPC {
		_, err := c.agentClient.DeleteAgent(
			ctx, &subtraterpc.DeleteAgentRequest{Id: id},
		)
		return err
	}

	return c.registry.DeleteAgent(ctx, id)
}

// FetchInbox retrieves messages from an agent's inbox.
func (c *Client) FetchInbox(ctx context.Context, req mail.FetchInboxRequest) ([]mail.InboxMessage, error) {
	if c.mode == ModeGRPC {
		grpcReq := &subtraterpc.FetchInboxRequest{
			AgentId:    req.AgentID,
			Limit:      int32(req.Limit),
			UnreadOnly: req.UnreadOnly,
		}

		resp, err := c.mailClient.FetchInbox(ctx, grpcReq)
		if err != nil {
			return nil, err
		}

		return convertProtoMessagesToMail(resp.Messages), nil
	}

	result := c.mailService.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return nil, err
	}
	resp := val.(mail.FetchInboxResponse)
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Messages, nil
}

// ReadMessage retrieves a single message by ID and marks it as read.
func (c *Client) ReadMessage(ctx context.Context, agentID, messageID int64) (*mail.InboxMessage, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.ReadMessage(ctx, &subtraterpc.ReadMessageRequest{
			AgentId:   agentID,
			MessageId: messageID,
		})
		if err != nil {
			return nil, err
		}
		if resp.Message == nil {
			return nil, fmt.Errorf("message not found")
		}
		msg := convertProtoMessageToMail(resp.Message)
		return &msg, nil
	}

	result := c.mailService.Receive(ctx, mail.ReadMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
	val, err := result.Unpack()
	if err != nil {
		return nil, err
	}
	resp := val.(mail.ReadMessageResponse)
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Message, nil
}

// ReadThread retrieves all messages in a thread.
func (c *Client) ReadThread(ctx context.Context, agentID int64, threadID string) ([]mail.InboxMessage, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.ReadThread(ctx, &subtraterpc.ReadThreadRequest{
			AgentId:  agentID,
			ThreadId: threadID,
		})
		if err != nil {
			return nil, err
		}
		return convertProtoMessagesToMail(resp.Messages), nil
	}

	// Direct mode: use database query directly.
	messages, err := c.store.Queries().GetMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	result := make([]mail.InboxMessage, 0, len(messages))
	for _, m := range messages {
		msg := mail.InboxMessage{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			TopicID:   m.TopicID,
			SenderID:  m.SenderID,
			Subject:   m.Subject,
			Body:      m.BodyMd,
			Priority:  mail.Priority(m.Priority),
			CreatedAt: time.Unix(m.CreatedAt, 0),
		}
		if m.DeadlineAt.Valid {
			t := time.Unix(m.DeadlineAt.Int64, 0)
			msg.Deadline = &t
		}
		result = append(result, msg)
	}
	return result, nil
}

// SendMail sends a new message.
func (c *Client) SendMail(ctx context.Context, req mail.SendMailRequest) (int64, string, error) {
	if c.mode == ModeGRPC {
		grpcReq := &subtraterpc.SendMailRequest{
			SenderId:       req.SenderID,
			RecipientNames: req.RecipientNames,
			TopicName:      req.TopicName,
			ThreadId:       req.ThreadID,
			Subject:        req.Subject,
			Body:           req.Body,
			Priority:       convertPriorityToProto(req.Priority),
		}
		if req.Deadline != nil {
			grpcReq.DeadlineAt = req.Deadline.Unix()
		}

		resp, err := c.mailClient.SendMail(ctx, grpcReq)
		if err != nil {
			return 0, "", err
		}
		return resp.MessageId, resp.ThreadId, nil
	}

	result := c.mailService.Receive(ctx, req)
	val, err := result.Unpack()
	if err != nil {
		return 0, "", err
	}
	resp := val.(mail.SendMailResponse)
	if resp.Error != nil {
		return 0, "", resp.Error
	}
	return resp.MessageID, resp.ThreadID, nil
}

// UpdateState changes the state of a message.
func (c *Client) UpdateState(ctx context.Context, agentID, messageID int64, state string, snoozedUntil *time.Time) error {
	if c.mode == ModeGRPC {
		grpcReq := &subtraterpc.UpdateStateRequest{
			AgentId:   agentID,
			MessageId: messageID,
			NewState:  convertStateToProto(state),
		}
		if snoozedUntil != nil {
			grpcReq.SnoozedUntil = snoozedUntil.Unix()
		}

		_, err := c.mailClient.UpdateState(ctx, grpcReq)
		return err
	}

	result := c.mailService.Receive(ctx, mail.UpdateStateRequest{
		AgentID:      agentID,
		MessageID:    messageID,
		NewState:     state,
		SnoozedUntil: snoozedUntil,
	})
	val, err := result.Unpack()
	if err != nil {
		return err
	}
	resp := val.(mail.UpdateStateResponse)
	return resp.Error
}

// AckMessage acknowledges receipt of a message.
func (c *Client) AckMessage(ctx context.Context, agentID, messageID int64) error {
	if c.mode == ModeGRPC {
		_, err := c.mailClient.AckMessage(ctx, &subtraterpc.AckMessageRequest{
			AgentId:   agentID,
			MessageId: messageID,
		})
		return err
	}

	result := c.mailService.Receive(ctx, mail.AckMessageRequest{
		AgentID:   agentID,
		MessageID: messageID,
	})
	val, err := result.Unpack()
	if err != nil {
		return err
	}
	resp := val.(mail.AckMessageResponse)
	return resp.Error
}

// GetStatus returns the mail status for an agent.
func (c *Client) GetStatus(ctx context.Context, agentID int64) (*mail.AgentStatus, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.GetStatus(ctx, &subtraterpc.GetStatusRequest{
			AgentId: agentID,
		})
		if err != nil {
			return nil, err
		}
		return &mail.AgentStatus{
			AgentID:      resp.AgentId,
			AgentName:    resp.AgentName,
			UnreadCount:  resp.UnreadCount,
			UrgentCount:  resp.UrgentCount,
			StarredCount: resp.StarredCount,
			SnoozedCount: resp.SnoozedCount,
		}, nil
	}

	result := c.mailService.Receive(ctx, mail.GetStatusRequest{
		AgentID: agentID,
	})
	val, err := result.Unpack()
	if err != nil {
		return nil, err
	}
	resp := val.(mail.GetStatusResponse)
	if resp.Error != nil {
		return nil, resp.Error
	}
	status := resp.Status
	return &status, nil
}

// HasUnackedStatusTo checks if there are any unacked status messages from
// sender to recipient. Used for deduplication in status-update command.
func (c *Client) HasUnackedStatusTo(
	ctx context.Context, senderID, recipientID int64,
) (bool, error) {

	if c.mode == ModeGRPC {
		resp, err := c.mailClient.HasUnackedStatusTo(
			ctx, &subtraterpc.HasUnackedStatusToRequest{
				SenderId:    senderID,
				RecipientId: recipientID,
			},
		)
		if err != nil {
			return false, err
		}
		return resp.HasPending, nil
	}

	count, err := c.store.Queries().HasUnackedStatusToAgent(
		ctx, sqlc.HasUnackedStatusToAgentParams{
			SenderID: senderID,
			AgentID:  recipientID,
		},
	)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// PollChanges checks for new messages since given offsets.
func (c *Client) PollChanges(ctx context.Context, agentID int64, sinceOffsets map[int64]int64) ([]mail.InboxMessage, map[int64]int64, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.PollChanges(ctx, &subtraterpc.PollChangesRequest{
			AgentId:      agentID,
			SinceOffsets: sinceOffsets,
		})
		if err != nil {
			return nil, nil, err
		}
		return convertProtoMessagesToMail(resp.NewMessages), resp.NewOffsets, nil
	}

	result := c.mailService.Receive(ctx, mail.PollChangesRequest{
		AgentID:      agentID,
		SinceOffsets: sinceOffsets,
	})
	val, err := result.Unpack()
	if err != nil {
		return nil, nil, err
	}
	resp := val.(mail.PollChangesResponse)
	if resp.Error != nil {
		return nil, nil, resp.Error
	}
	return resp.NewMessages, resp.NewOffsets, nil
}

// Search performs full-text search across messages.
func (c *Client) Search(ctx context.Context, agentID int64, query string, topicID int64, limit int) ([]mail.InboxMessage, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.Search(ctx, &subtraterpc.SearchRequest{
			AgentId: agentID,
			Query:   query,
			TopicId: topicID,
			Limit:   int32(limit),
		})
		if err != nil {
			return nil, err
		}
		return convertProtoMessagesToMail(resp.Results), nil
	}

	// Direct mode: use database search directly.
	if topicID > 0 {
		// Search within topic not implemented in direct mode yet.
		return nil, fmt.Errorf("topic-scoped search requires daemon connection")
	}

	results, err := c.store.SearchMessagesForAgent(ctx, query, agentID, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]mail.InboxMessage, 0, len(results))
	for _, r := range results {
		messages = append(messages, mail.InboxMessage{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  mail.Priority(r.Priority),
			CreatedAt: time.Unix(r.CreatedAt, 0),
		})
	}
	return messages, nil
}

// Publish sends a message to a topic.
func (c *Client) Publish(ctx context.Context, senderID int64, topicName, subject, body string, priority mail.Priority) (int64, int, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.Publish(ctx, &subtraterpc.PublishRequest{
			SenderId:  senderID,
			TopicName: topicName,
			Subject:   subject,
			Body:      body,
			Priority:  convertPriorityToProto(priority),
		})
		if err != nil {
			return 0, 0, err
		}
		return resp.MessageId, int(resp.RecipientsCount), nil
	}

	result := c.mailService.Receive(ctx, mail.PublishRequest{
		SenderID:  senderID,
		TopicName: topicName,
		Subject:   subject,
		Body:      body,
		Priority:  priority,
	})
	val, err := result.Unpack()
	if err != nil {
		return 0, 0, err
	}
	resp := val.(mail.PublishResponse)
	if resp.Error != nil {
		return 0, 0, resp.Error
	}
	return resp.MessageID, resp.RecipientsCount, nil
}

// RegisterAgent creates a new agent.
func (c *Client) RegisterAgent(
	ctx context.Context, name, projectKey, gitBranch string,
) (int64, string, error) {

	if c.mode == ModeGRPC {
		// Note: gRPC RegisterAgentRequest doesn't include git_branch yet.
		// The branch will be set on first heartbeat/identity call.
		resp, err := c.agentClient.RegisterAgent(ctx, &subtraterpc.RegisterAgentRequest{
			Name:       name,
			ProjectKey: projectKey,
		})
		if err != nil {
			return 0, "", err
		}
		return resp.AgentId, resp.Name, nil
	}

	ag, err := c.registry.RegisterAgent(ctx, name, projectKey, gitBranch)
	if err != nil {
		return 0, "", err
	}
	return ag.ID, ag.Name, nil
}

// ListAgents lists all registered agents.
func (c *Client) ListAgents(ctx context.Context) ([]sqlc.Agent, error) {
	if c.mode == ModeGRPC {
		resp, err := c.agentClient.ListAgents(ctx, &subtraterpc.ListAgentsRequest{})
		if err != nil {
			return nil, err
		}
		agents := make([]sqlc.Agent, len(resp.Agents))
		for i, a := range resp.Agents {
			agents[i] = sqlc.Agent{
				ID:   a.Id,
				Name: a.Name,
			}
		}
		return agents, nil
	}

	return c.registry.ListAgents(ctx)
}

// UpdateHeartbeat updates the last active time for an agent.
func (c *Client) UpdateHeartbeat(ctx context.Context, agentID int64) error {
	if c.mode == ModeGRPC {
		// Heartbeat is implicit in gRPC calls; the daemon tracks activity.
		// For now, we can use GetStatus as a heartbeat.
		_, err := c.mailClient.GetStatus(ctx, &subtraterpc.GetStatusRequest{
			AgentId: agentID,
		})
		return err
	}

	return c.registry.UpdateLastActive(ctx, agentID)
}

// GetAgent retrieves an agent by ID.
func (c *Client) GetAgent(ctx context.Context, agentID int64) (*sqlc.Agent, error) {
	if c.mode == ModeGRPC {
		resp, err := c.agentClient.GetAgent(ctx, &subtraterpc.GetAgentRequest{
			AgentId: agentID,
		})
		if err != nil {
			return nil, err
		}
		return &sqlc.Agent{
			ID:           resp.Id,
			Name:         resp.Name,
			LastActiveAt: resp.LastActiveAt,
		}, nil
	}

	ag, err := c.store.Queries().GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return &ag, nil
}

// RestoreIdentity restores a previously saved identity for a session.
func (c *Client) RestoreIdentity(ctx context.Context, sessionID string) (*agent.IdentityFile, error) {
	if c.mode == ModeGRPC {
		// Use EnsureIdentity which will restore if exists.
		// Note: empty git_branch is fine for restore - existing value is kept.
		return c.EnsureIdentity(ctx, sessionID, "", "")
	}

	return c.identityMgr.RestoreIdentity(ctx, sessionID)
}

// SaveIdentity persists the current identity state.
func (c *Client) SaveIdentity(ctx context.Context, identity *agent.IdentityFile) error {
	if c.mode == ModeGRPC {
		// In gRPC mode, the daemon handles persistence automatically.
		return nil
	}

	return c.identityMgr.SaveIdentity(ctx, identity)
}

// SetProjectDefault sets the default agent for a project.
func (c *Client) SetProjectDefault(ctx context.Context, projectDir, agentName string) error {
	if c.mode == ModeGRPC {
		// SetProjectDefault requires direct database access.
		return fmt.Errorf("set-default requires direct mode (daemon not running)")
	}

	return c.identityMgr.SetProjectDefault(ctx, projectDir, agentName)
}

// ListConsumerOffsets returns consumer offsets for an agent.
func (c *Client) ListConsumerOffsets(ctx context.Context, agentID int64) (map[string]int64, error) {
	if c.mode == ModeGRPC {
		// Not implemented in gRPC yet.
		return nil, fmt.Errorf("list consumer offsets requires direct mode")
	}

	offsets, err := c.store.Queries().ListConsumerOffsetsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, o := range offsets {
		result[o.TopicName] = o.LastOffset
	}
	return result, nil
}

// Subscribe subscribes an agent to a topic.
func (c *Client) Subscribe(ctx context.Context, agentID int64, topicName string) error {
	if c.mode == ModeGRPC {
		_, err := c.mailClient.Subscribe(ctx, &subtraterpc.SubscribeRequest{
			AgentId:   agentID,
			TopicName: topicName,
		})
		return err
	}

	// Get the topic.
	topic, err := c.store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found", topicName)
	}

	// Check if already subscribed.
	_, err = c.store.Queries().GetSubscription(ctx, sqlc.GetSubscriptionParams{
		AgentID: agentID,
		TopicID: topic.ID,
	})
	if err == nil {
		// Already subscribed.
		return nil
	}

	// Create subscription.
	return c.store.Queries().CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      agentID,
		TopicID:      topic.ID,
		SubscribedAt: time.Now().Unix(),
	})
}

// Unsubscribe removes an agent's subscription to a topic.
func (c *Client) Unsubscribe(ctx context.Context, agentID int64, topicName string) error {
	if c.mode == ModeGRPC {
		_, err := c.mailClient.Unsubscribe(ctx, &subtraterpc.UnsubscribeRequest{
			AgentId:   agentID,
			TopicName: topicName,
		})
		return err
	}

	// Get the topic.
	topic, err := c.store.Queries().GetTopicByName(ctx, topicName)
	if err != nil {
		return fmt.Errorf("topic %q not found", topicName)
	}

	// Delete subscription.
	return c.store.Queries().DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
		AgentID: agentID,
		TopicID: topic.ID,
	})
}

// TopicInfo represents information about a topic.
type TopicInfo struct {
	ID               int64
	Name             string
	Type             string
	RetentionSeconds int64
	SubscriberCount  int64
}

// ListTopics returns all topics.
func (c *Client) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	if c.mode == ModeGRPC {
		resp, err := c.mailClient.ListTopics(ctx, &subtraterpc.ListTopicsRequest{})
		if err != nil {
			return nil, err
		}
		topics := make([]TopicInfo, len(resp.Topics))
		for i, t := range resp.Topics {
			topics[i] = TopicInfo{
				ID:   t.Id,
				Name: t.Name,
				Type: t.TopicType,
				// RetentionSeconds and SubscriberCount not in proto.
			}
		}
		return topics, nil
	}

	dbTopics, err := c.store.Queries().ListTopics(ctx)
	if err != nil {
		return nil, err
	}

	topics := make([]TopicInfo, len(dbTopics))
	for i, t := range dbTopics {
		var retention int64
		if t.RetentionSeconds.Valid {
			retention = t.RetentionSeconds.Int64
		}

		count, _ := c.store.Queries().CountSubscribersByTopic(ctx, t.ID)

		topics[i] = TopicInfo{
			ID:               t.ID,
			Name:             t.Name,
			Type:             t.TopicType,
			RetentionSeconds: retention,
			SubscriberCount:  count,
		}
	}
	return topics, nil
}

// SubscriptionInfo represents a subscription entry.
type SubscriptionInfo struct {
	TopicID          int64
	TopicName        string
	TopicType        string
	RetentionSeconds int64
}

// ListSubscriptionsByAgent returns topics an agent is subscribed to.
func (c *Client) ListSubscriptionsByAgent(ctx context.Context, agentID int64) ([]SubscriptionInfo, error) {
	if c.mode == ModeGRPC {
		// Use ListTopics with subscribed_only to get agent's subscriptions.
		resp, err := c.mailClient.ListTopics(ctx, &subtraterpc.ListTopicsRequest{
			AgentId:        agentID,
			SubscribedOnly: true,
		})
		if err != nil {
			return nil, err
		}
		subs := make([]SubscriptionInfo, len(resp.Topics))
		for i, t := range resp.Topics {
			subs[i] = SubscriptionInfo{
				TopicID:   t.Id,
				TopicName: t.Name,
				TopicType: t.TopicType,
			}
		}
		return subs, nil
	}

	dbSubs, err := c.store.Queries().ListSubscriptionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	subs := make([]SubscriptionInfo, len(dbSubs))
	for i, s := range dbSubs {
		var retention int64
		if s.RetentionSeconds.Valid {
			retention = s.RetentionSeconds.Int64
		}
		subs[i] = SubscriptionInfo{
			TopicID:          s.ID,
			TopicName:        s.Name,
			TopicType:        s.TopicType,
			RetentionSeconds: retention,
		}
	}
	return subs, nil
}

// =============================================================================
// Review methods
// =============================================================================

// GetReview retrieves a review by its review ID.
func (c *Client) GetReview(ctx context.Context, reviewID string) (*ReviewInfo, error) {
	if c.mode == ModeGRPC {
		// Review gRPC endpoints not implemented yet.
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	review, err := c.store.Queries().GetReview(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	return convertSqlcReview(review), nil
}

// ListReviews lists all reviews with a limit.
func (c *Client) ListReviews(ctx context.Context, limit int) ([]ReviewInfo, error) {
	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	reviews, err := c.store.Queries().ListReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return convertSqlcReviews(reviews), nil
}

// ListReviewsByRequester lists reviews by a specific requester.
func (c *Client) ListReviewsByRequester(
	ctx context.Context, requesterID int64, limit int,
) ([]ReviewInfo, error) {

	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	reviews, err := c.store.Queries().ListReviewsByRequester(
		ctx, sqlc.ListReviewsByRequesterParams{
			RequesterID: requesterID,
			Limit:       int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	return convertSqlcReviews(reviews), nil
}

// ListReviewsByState lists reviews by state.
func (c *Client) ListReviewsByState(
	ctx context.Context, state string, limit int,
) ([]ReviewInfo, error) {

	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	reviews, err := c.store.Queries().ListReviewsByState(
		ctx, sqlc.ListReviewsByStateParams{
			State: state,
			Limit: int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	return convertSqlcReviews(reviews), nil
}

// ListPendingReviews lists reviews pending review.
func (c *Client) ListPendingReviews(ctx context.Context, limit int) ([]ReviewInfo, error) {
	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	reviews, err := c.store.Queries().ListPendingReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return convertSqlcReviews(reviews), nil
}

// ListActiveReviews lists active (not completed) reviews.
func (c *Client) ListActiveReviews(ctx context.Context, limit int) ([]ReviewInfo, error) {
	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	reviews, err := c.store.Queries().ListActiveReviews(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return convertSqlcReviews(reviews), nil
}

// GetReviewStats retrieves aggregate review statistics.
func (c *Client) GetReviewStats(ctx context.Context) (*ReviewStatsInfo, error) {
	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	stats, err := c.store.Queries().GetReviewStats(ctx)
	if err != nil {
		return nil, err
	}
	return &ReviewStatsInfo{
		TotalReviews:     stats.TotalReviews,
		Approved:         stats.Approved,
		Pending:          stats.Pending,
		InProgress:       stats.InProgress,
		ChangesRequested: stats.ChangesRequested,
	}, nil
}

// ListReviewIterations lists all iterations for a review.
func (c *Client) ListReviewIterations(
	ctx context.Context, reviewID string,
) ([]ReviewIterationInfo, error) {

	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	iters, err := c.store.Queries().ListReviewIterations(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	return convertSqlcReviewIterations(iters), nil
}

// ListReviewIssues lists all issues for a review.
func (c *Client) ListReviewIssues(ctx context.Context, reviewID string) ([]ReviewIssueInfo, error) {
	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	issues, err := c.store.Queries().ListReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	return convertSqlcReviewIssues(issues), nil
}

// ListOpenReviewIssues lists open issues for a review.
func (c *Client) ListOpenReviewIssues(
	ctx context.Context, reviewID string,
) ([]ReviewIssueInfo, error) {

	if c.mode == ModeGRPC {
		return nil, fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	issues, err := c.store.Queries().ListOpenReviewIssues(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	return convertSqlcReviewIssues(issues), nil
}

// CancelReview cancels an active review.
func (c *Client) CancelReview(ctx context.Context, reviewID string) error {
	if c.mode == ModeGRPC {
		return fmt.Errorf("review operations require direct mode (daemon not running)")
	}

	return c.store.Queries().UpdateReviewState(ctx, sqlc.UpdateReviewStateParams{
		State:     "cancelled",
		UpdatedAt: time.Now().Unix(),
		ReviewID:  reviewID,
	})
}

// Helper functions for review conversion.

func convertSqlcReview(r sqlc.Review) *ReviewInfo {
	review := &ReviewInfo{
		ReviewID:    r.ReviewID,
		ThreadID:    r.ThreadID,
		RequesterID: r.RequesterID,
		Branch:      r.Branch,
		BaseBranch:  r.BaseBranch,
		CommitSHA:   r.CommitSha,
		RepoPath:    r.RepoPath,
		ReviewType:  r.ReviewType,
		Priority:    r.Priority,
		State:       r.State,
		CreatedAt:   time.Unix(r.CreatedAt, 0),
		UpdatedAt:   time.Unix(r.UpdatedAt, 0),
	}
	if r.PrNumber.Valid {
		prNum := r.PrNumber.Int64
		review.PRNumber = &prNum
	}
	if r.CompletedAt.Valid {
		t := time.Unix(r.CompletedAt.Int64, 0)
		review.CompletedAt = &t
	}
	return review
}

func convertSqlcReviews(reviews []sqlc.Review) []ReviewInfo {
	result := make([]ReviewInfo, len(reviews))
	for i, r := range reviews {
		result[i] = *convertSqlcReview(r)
	}
	return result
}

func convertSqlcReviewIterations(iters []sqlc.ReviewIteration) []ReviewIterationInfo {
	result := make([]ReviewIterationInfo, len(iters))
	for i, iter := range iters {
		info := ReviewIterationInfo{
			ID:                iter.ID,
			ReviewID:          iter.ReviewID,
			IterationNum:      int(iter.IterationNum),
			ReviewerID:        iter.ReviewerID,
			ReviewerSessionID: iter.ReviewerSessionID.String,
			Decision:          iter.Decision,
			Summary:           iter.Summary,
			FilesReviewed:     int(iter.FilesReviewed),
			LinesAnalyzed:     int(iter.LinesAnalyzed),
			DurationMS:        iter.DurationMs,
			CostUSD:           iter.CostUsd,
			StartedAt:         time.Unix(iter.StartedAt, 0),
		}
		if iter.CompletedAt.Valid {
			t := time.Unix(iter.CompletedAt.Int64, 0)
			info.CompletedAt = &t
		}
		result[i] = info
	}
	return result
}

func convertSqlcReviewIssues(issues []sqlc.ReviewIssue) []ReviewIssueInfo {
	result := make([]ReviewIssueInfo, len(issues))
	for i, issue := range issues {
		info := ReviewIssueInfo{
			ID:           issue.ID,
			ReviewID:     issue.ReviewID,
			IterationNum: int(issue.IterationNum),
			IssueType:    issue.IssueType,
			Severity:     issue.Severity,
			FilePath:     issue.FilePath,
			LineStart:    int(issue.LineStart),
			Title:        issue.Title,
			Description:  issue.Description,
			CodeSnippet:  issue.CodeSnippet.String,
			Suggestion:   issue.Suggestion.String,
			ClaudeMDRef:  issue.ClaudeMdRef.String,
			Status:       issue.Status,
			CreatedAt:    time.Unix(issue.CreatedAt, 0),
		}
		if issue.LineEnd.Valid {
			lineEnd := int(issue.LineEnd.Int64)
			info.LineEnd = &lineEnd
		}
		result[i] = info
	}
	return result
}

// Helper functions for proto conversion.

func convertProtoMessagesToMail(msgs []*subtraterpc.InboxMessage) []mail.InboxMessage {
	result := make([]mail.InboxMessage, len(msgs))
	for i, m := range msgs {
		result[i] = convertProtoMessageToMail(m)
	}
	return result
}

func convertProtoMessageToMail(m *subtraterpc.InboxMessage) mail.InboxMessage {
	msg := mail.InboxMessage{
		ID:         m.Id,
		ThreadID:   m.ThreadId,
		TopicID:    m.TopicId,
		SenderID:   m.SenderId,
		SenderName: m.SenderName,
		Subject:    m.Subject,
		Body:       m.Body,
		Priority:   convertProtoToPriority(m.Priority),
		State:      convertProtoToState(m.State),
		CreatedAt:  time.Unix(m.CreatedAt, 0),
	}

	if m.DeadlineAt > 0 {
		t := time.Unix(m.DeadlineAt, 0)
		msg.Deadline = &t
	}
	if m.SnoozedUntil > 0 {
		t := time.Unix(m.SnoozedUntil, 0)
		msg.SnoozedUntil = &t
	}
	if m.ReadAt > 0 {
		t := time.Unix(m.ReadAt, 0)
		msg.ReadAt = &t
	}
	if m.AckedAt > 0 {
		t := time.Unix(m.AckedAt, 0)
		msg.AckedAt = &t
	}

	return msg
}

func convertPriorityToProto(p mail.Priority) subtraterpc.Priority {
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

func convertProtoToPriority(p subtraterpc.Priority) mail.Priority {
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

func convertStateToProto(s string) subtraterpc.MessageState {
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

func convertProtoToState(s subtraterpc.MessageState) string {
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
