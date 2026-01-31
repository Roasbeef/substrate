package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// SqlcStore implements the Store interface using sqlc-generated queries.
type SqlcStore struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewSqlcStore creates a new SqlcStore wrapping the given database connection.
func NewSqlcStore(db *sql.DB) *SqlcStore {
	return &SqlcStore{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Close closes the underlying database connection.
func (s *SqlcStore) Close() error {
	return s.db.Close()
}

// WithTx executes the given function within a database transaction.
func (s *SqlcStore) WithTx(
	ctx context.Context,
	fn func(ctx context.Context, store Store) error,
) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new store instance bound to this transaction.
	txStore := &SqlcStore{
		db:      s.db,
		queries: sqlc.New(tx),
	}

	// Execute the callback.
	if err := fn(ctx, txStore); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf(
				"tx error: %w, rollback error: %v", err, rbErr,
			)
		}
		return err
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// MessageStore implementation.

// CreateMessage creates a new message in the database.
func (s *SqlcStore) CreateMessage(
	ctx context.Context, params CreateMessageParams,
) (Message, error) {

	sqlcParams := sqlc.CreateMessageParams{
		ThreadID:    params.ThreadID,
		TopicID:     params.TopicID,
		LogOffset:   params.LogOffset,
		SenderID:    params.SenderID,
		Subject:     params.Subject,
		BodyMd:      params.Body,
		Priority:    params.Priority,
		DeadlineAt:  ToSqlcNullInt64(params.DeadlineAt),
		Attachments: ToSqlcNullString(params.Attachments),
		CreatedAt:   time.Now().Unix(),
	}

	m, err := s.queries.CreateMessage(ctx, sqlcParams)
	if err != nil {
		return Message{}, fmt.Errorf("failed to create message: %w", err)
	}

	return MessageFromSqlc(m), nil
}

// GetMessage retrieves a message by its ID.
func (s *SqlcStore) GetMessage(ctx context.Context, id int64) (Message, error) {
	m, err := s.queries.GetMessage(ctx, id)
	if err != nil {
		return Message{}, fmt.Errorf("failed to get message: %w", err)
	}
	return MessageFromSqlc(m), nil
}

// GetMessagesByThread retrieves all messages in a thread.
func (s *SqlcStore) GetMessagesByThread(
	ctx context.Context, threadID string,
) ([]Message, error) {

	rows, err := s.queries.GetMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	messages := make([]Message, len(rows))
	for i, r := range rows {
		messages[i] = MessageFromSqlc(r)
	}
	return messages, nil
}

// GetInboxMessages retrieves inbox messages for an agent.
func (s *SqlcStore) GetInboxMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {

	rows, err := s.queries.GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get inbox messages: %w", err)
	}

	messages := make([]InboxMessage, len(rows))
	for i, r := range rows {
		messages[i] = inboxMessageFromRow(r)
	}
	return messages, nil
}

// GetUnreadMessages retrieves unread messages for an agent.
func (s *SqlcStore) GetUnreadMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {

	rows, err := s.queries.GetUnreadMessages(ctx, sqlc.GetUnreadMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get unread messages: %w", err)
	}

	messages := make([]InboxMessage, len(rows))
	for i, r := range rows {
		messages[i] = inboxMessageFromUnreadRow(r)
	}
	return messages, nil
}

// GetArchivedMessages retrieves archived messages for an agent.
func (s *SqlcStore) GetArchivedMessages(
	ctx context.Context, agentID int64, limit int,
) ([]InboxMessage, error) {

	rows, err := s.queries.GetArchivedMessages(
		ctx, sqlc.GetArchivedMessagesParams{
			AgentID: agentID,
			Limit:   int64(limit),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get archived messages: %w", err)
	}

	messages := make([]InboxMessage, len(rows))
	for i, r := range rows {
		messages[i] = inboxMessageFromArchivedRow(r)
	}
	return messages, nil
}

// UpdateRecipientState updates the state of a message for a recipient.
func (s *SqlcStore) UpdateRecipientState(
	ctx context.Context, messageID, agentID int64, state string,
) error {

	now := time.Now().Unix()
	_, err := s.queries.UpdateRecipientState(
		ctx, sqlc.UpdateRecipientStateParams{
			State:     state,
			Column2:   state,
			ReadAt:    sql.NullInt64{Int64: now, Valid: true},
			MessageID: messageID,
			AgentID:   agentID,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update recipient state: %w", err)
	}
	return nil
}

// MarkMessageRead marks a message as read for a recipient.
func (s *SqlcStore) MarkMessageRead(
	ctx context.Context, messageID, agentID int64,
) error {

	return s.UpdateRecipientState(ctx, messageID, agentID, "read")
}

// AckMessage acknowledges a message for a recipient.
func (s *SqlcStore) AckMessage(
	ctx context.Context, messageID, agentID int64,
) error {

	err := s.queries.UpdateRecipientAcked(
		ctx, sqlc.UpdateRecipientAckedParams{
			AckedAt: sql.NullInt64{
				Int64: time.Now().Unix(),
				Valid: true,
			},
			MessageID: messageID,
			AgentID:   agentID,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}
	return nil
}

// CreateMessageRecipient creates a recipient entry for a message.
func (s *SqlcStore) CreateMessageRecipient(
	ctx context.Context, messageID, agentID int64,
) error {

	err := s.queries.CreateMessageRecipient(
		ctx, sqlc.CreateMessageRecipientParams{
			MessageID: messageID,
			AgentID:   agentID,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create recipient: %w", err)
	}
	return nil
}

// GetMessageRecipient retrieves the recipient state for a message.
func (s *SqlcStore) GetMessageRecipient(
	ctx context.Context, messageID, agentID int64,
) (MessageRecipient, error) {

	r, err := s.queries.GetMessageRecipient(
		ctx, sqlc.GetMessageRecipientParams{
			MessageID: messageID,
			AgentID:   agentID,
		},
	)
	if err != nil {
		return MessageRecipient{}, fmt.Errorf(
			"failed to get recipient: %w", err,
		)
	}
	return MessageRecipientFromSqlc(r), nil
}

// CountUnreadByAgent counts unread messages for an agent.
func (s *SqlcStore) CountUnreadByAgent(
	ctx context.Context, agentID int64,
) (int64, error) {

	count, err := s.queries.CountUnreadByAgent(ctx, agentID)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread: %w", err)
	}
	return count, nil
}

// CountUnreadUrgentByAgent counts urgent unread messages for an agent.
func (s *SqlcStore) CountUnreadUrgentByAgent(
	ctx context.Context, agentID int64,
) (int64, error) {

	count, err := s.queries.CountUnreadUrgentByAgent(ctx, agentID)
	if err != nil {
		return 0, fmt.Errorf("failed to count urgent: %w", err)
	}
	return count, nil
}

// GetMessagesSinceOffset retrieves messages after a given log offset.
func (s *SqlcStore) GetMessagesSinceOffset(
	ctx context.Context, topicID, offset int64, limit int,
) ([]Message, error) {

	rows, err := s.queries.GetMessagesSinceOffset(
		ctx, sqlc.GetMessagesSinceOffsetParams{
			TopicID:   topicID,
			LogOffset: offset,
			Limit:     int64(limit),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages since offset: %w", err)
	}

	messages := make([]Message, len(rows))
	for i, r := range rows {
		messages[i] = MessageFromSqlc(r)
	}
	return messages, nil
}

// NextLogOffset returns the next available log offset for a topic.
func (s *SqlcStore) NextLogOffset(
	ctx context.Context, topicID int64,
) (int64, error) {

	result, err := s.queries.GetMaxLogOffset(ctx, topicID)
	if err != nil {
		return 0, fmt.Errorf("failed to get max log offset: %w", err)
	}

	var maxOffset int64
	switch v := result.(type) {
	case int64:
		maxOffset = v
	case int:
		maxOffset = int64(v)
	case nil:
		maxOffset = 0
	default:
		return 0, fmt.Errorf("unexpected type for max offset: %T", result)
	}

	return maxOffset + 1, nil
}

// AgentStore implementation.

// CreateAgent creates a new agent in the database.
func (s *SqlcStore) CreateAgent(
	ctx context.Context, params CreateAgentParams,
) (Agent, error) {

	a, err := s.queries.CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:       params.Name,
		ProjectKey: ToSqlcNullString(params.ProjectKey),
		GitBranch:  ToSqlcNullString(params.GitBranch),
		CreatedAt:  time.Now().Unix(),
	})
	if err != nil {
		return Agent{}, fmt.Errorf("failed to create agent: %w", err)
	}
	return AgentFromSqlc(a), nil
}

// GetAgent retrieves an agent by its ID.
func (s *SqlcStore) GetAgent(ctx context.Context, id int64) (Agent, error) {
	a, err := s.queries.GetAgent(ctx, id)
	if err != nil {
		return Agent{}, fmt.Errorf("failed to get agent: %w", err)
	}
	return AgentFromSqlc(a), nil
}

// GetAgentByName retrieves an agent by its name.
func (s *SqlcStore) GetAgentByName(
	ctx context.Context, name string,
) (Agent, error) {

	a, err := s.queries.GetAgentByName(ctx, name)
	if err != nil {
		return Agent{}, fmt.Errorf("failed to get agent by name: %w", err)
	}
	return AgentFromSqlc(a), nil
}

// GetAgentBySessionID retrieves an agent by session ID.
func (s *SqlcStore) GetAgentBySessionID(
	ctx context.Context, sessionID string,
) (Agent, error) {

	a, err := s.queries.GetAgentBySessionID(ctx, sessionID)
	if err != nil {
		return Agent{}, fmt.Errorf(
			"failed to get agent by session: %w", err,
		)
	}
	return AgentFromSqlc(a), nil
}

// ListAgents lists all agents.
func (s *SqlcStore) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.queries.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agents := make([]Agent, len(rows))
	for i, r := range rows {
		agents[i] = AgentFromSqlc(r)
	}
	return agents, nil
}

// ListAgentsByProject lists agents for a specific project.
func (s *SqlcStore) ListAgentsByProject(
	ctx context.Context, projectKey string,
) ([]Agent, error) {

	rows, err := s.queries.ListAgentsByProject(
		ctx, ToSqlcNullString(projectKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents by project: %w", err)
	}

	agents := make([]Agent, len(rows))
	for i, r := range rows {
		agents[i] = AgentFromSqlc(r)
	}
	return agents, nil
}

// UpdateLastActive updates the last active timestamp for an agent.
func (s *SqlcStore) UpdateLastActive(
	ctx context.Context, id int64, ts time.Time,
) error {

	err := s.queries.UpdateAgentLastActive(
		ctx, sqlc.UpdateAgentLastActiveParams{
			ID:           id,
			LastActiveAt: ts.Unix(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update last active: %w", err)
	}
	return nil
}

// UpdateSession updates the session ID for an agent.
func (s *SqlcStore) UpdateSession(
	ctx context.Context, id int64, sessionID string,
) error {

	err := s.queries.UpdateAgentSession(ctx, sqlc.UpdateAgentSessionParams{
		ID:               id,
		CurrentSessionID: ToSqlcNullString(sessionID),
	})
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

// DeleteAgent deletes an agent by its ID.
func (s *SqlcStore) DeleteAgent(ctx context.Context, id int64) error {
	err := s.queries.DeleteAgent(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}
	return nil
}

// TopicStore implementation.

// CreateTopic creates a new topic.
func (s *SqlcStore) CreateTopic(
	ctx context.Context, params CreateTopicParams,
) (Topic, error) {

	t, err := s.queries.CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      params.Name,
		TopicType: params.TopicType,
		RetentionSeconds: sql.NullInt64{
			Int64: params.RetentionSeconds,
			Valid: params.RetentionSeconds > 0,
		},
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return Topic{}, fmt.Errorf("failed to create topic: %w", err)
	}
	return TopicFromSqlc(t), nil
}

// GetTopic retrieves a topic by its ID.
func (s *SqlcStore) GetTopic(ctx context.Context, id int64) (Topic, error) {
	t, err := s.queries.GetTopic(ctx, id)
	if err != nil {
		return Topic{}, fmt.Errorf("failed to get topic: %w", err)
	}
	return TopicFromSqlc(t), nil
}

// GetTopicByName retrieves a topic by its name.
func (s *SqlcStore) GetTopicByName(
	ctx context.Context, name string,
) (Topic, error) {

	t, err := s.queries.GetTopicByName(ctx, name)
	if err != nil {
		return Topic{}, fmt.Errorf("failed to get topic by name: %w", err)
	}
	return TopicFromSqlc(t), nil
}

// GetOrCreateAgentInboxTopic gets or creates an agent's inbox topic.
func (s *SqlcStore) GetOrCreateAgentInboxTopic(
	ctx context.Context, agentName string,
) (Topic, error) {

	t, err := s.queries.GetOrCreateAgentInboxTopic(
		ctx, sqlc.GetOrCreateAgentInboxTopicParams{
			Column1:   ToSqlcNullString(agentName),
			CreatedAt: time.Now().Unix(),
		},
	)
	if err != nil {
		return Topic{}, fmt.Errorf(
			"failed to get/create inbox topic: %w", err,
		)
	}
	return TopicFromSqlc(t), nil
}

// GetOrCreateTopic gets or creates a topic by name.
func (s *SqlcStore) GetOrCreateTopic(
	ctx context.Context, name, topicType string,
) (Topic, error) {

	t, err := s.queries.GetOrCreateTopic(ctx, sqlc.GetOrCreateTopicParams{
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return Topic{}, fmt.Errorf("failed to get/create topic: %w", err)
	}
	return TopicFromSqlc(t), nil
}

// ListTopics lists all topics.
func (s *SqlcStore) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := s.queries.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	topics := make([]Topic, len(rows))
	for i, r := range rows {
		topics[i] = TopicFromSqlc(r)
	}
	return topics, nil
}

// ListTopicsByType lists topics of a specific type.
func (s *SqlcStore) ListTopicsByType(
	ctx context.Context, topicType string,
) ([]Topic, error) {

	rows, err := s.queries.ListTopicsByType(ctx, topicType)
	if err != nil {
		return nil, fmt.Errorf("failed to list topics by type: %w", err)
	}

	topics := make([]Topic, len(rows))
	for i, r := range rows {
		topics[i] = TopicFromSqlc(r)
	}
	return topics, nil
}

// CreateSubscription subscribes an agent to a topic.
func (s *SqlcStore) CreateSubscription(
	ctx context.Context, agentID, topicID int64,
) error {

	err := s.queries.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID:      agentID,
		TopicID:      topicID,
		SubscribedAt: time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	return nil
}

// DeleteSubscription unsubscribes an agent from a topic.
func (s *SqlcStore) DeleteSubscription(
	ctx context.Context, agentID, topicID int64,
) error {

	err := s.queries.DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
		AgentID: agentID,
		TopicID: topicID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	return nil
}

// ListSubscriptionsByAgent lists topics an agent is subscribed to.
func (s *SqlcStore) ListSubscriptionsByAgent(
	ctx context.Context, agentID int64,
) ([]Topic, error) {

	rows, err := s.queries.ListSubscriptionsByAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}

	topics := make([]Topic, len(rows))
	for i, r := range rows {
		topics[i] = TopicFromSqlc(r)
	}
	return topics, nil
}

// ListSubscriptionsByTopic lists agents subscribed to a topic.
func (s *SqlcStore) ListSubscriptionsByTopic(
	ctx context.Context, topicID int64,
) ([]Agent, error) {

	rows, err := s.queries.ListSubscriptionsByTopic(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscribers: %w", err)
	}

	agents := make([]Agent, len(rows))
	for i, r := range rows {
		agents[i] = AgentFromSqlc(r)
	}
	return agents, nil
}

// ActivityStore implementation.

// CreateActivity records a new activity event.
func (s *SqlcStore) CreateActivity(
	ctx context.Context, params CreateActivityParams,
) error {

	_, err := s.queries.CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      params.AgentID,
		ActivityType: params.ActivityType,
		Description:  params.Description,
		Metadata:     ToSqlcNullString(params.Metadata),
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}
	return nil
}

// ListRecentActivities lists the most recent activities.
func (s *SqlcStore) ListRecentActivities(
	ctx context.Context, limit int,
) ([]Activity, error) {

	rows, err := s.queries.ListRecentActivities(ctx, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list recent activities: %w", err)
	}

	activities := make([]Activity, len(rows))
	for i, r := range rows {
		activities[i] = ActivityFromSqlc(r)
	}
	return activities, nil
}

// ListActivitiesByAgent lists activities for a specific agent.
func (s *SqlcStore) ListActivitiesByAgent(
	ctx context.Context, agentID int64, limit int,
) ([]Activity, error) {

	rows, err := s.queries.ListActivitiesByAgent(
		ctx, sqlc.ListActivitiesByAgentParams{
			AgentID: agentID,
			Limit:   int64(limit),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent activities: %w", err)
	}

	activities := make([]Activity, len(rows))
	for i, r := range rows {
		activities[i] = ActivityFromSqlc(r)
	}
	return activities, nil
}

// ListActivitiesSince lists activities since a given timestamp.
func (s *SqlcStore) ListActivitiesSince(
	ctx context.Context, since time.Time, limit int,
) ([]Activity, error) {

	rows, err := s.queries.ListActivitiesSince(
		ctx, sqlc.ListActivitiesSinceParams{
			CreatedAt: since.Unix(),
			Limit:     int64(limit),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list activities since: %w", err)
	}

	activities := make([]Activity, len(rows))
	for i, r := range rows {
		activities[i] = ActivityFromSqlc(r)
	}
	return activities, nil
}

// DeleteOldActivities removes activities older than a given time.
func (s *SqlcStore) DeleteOldActivities(
	ctx context.Context, olderThan time.Time,
) error {

	err := s.queries.DeleteOldActivities(ctx, olderThan.Unix())
	if err != nil {
		return fmt.Errorf("failed to delete old activities: %w", err)
	}
	return nil
}

// SessionStore implementation.

// CreateSessionIdentity creates a new session identity mapping.
func (s *SqlcStore) CreateSessionIdentity(
	ctx context.Context, params CreateSessionIdentityParams,
) error {

	err := s.queries.CreateSessionIdentity(
		ctx, sqlc.CreateSessionIdentityParams{
			SessionID:    params.SessionID,
			AgentID:      params.AgentID,
			ProjectKey:   ToSqlcNullString(params.ProjectKey),
			GitBranch:    ToSqlcNullString(params.GitBranch),
			CreatedAt:    time.Now().Unix(),
			LastActiveAt: time.Now().Unix(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create session identity: %w", err)
	}
	return nil
}

// GetSessionIdentity retrieves a session identity by session ID.
func (s *SqlcStore) GetSessionIdentity(
	ctx context.Context, sessionID string,
) (SessionIdentity, error) {

	si, err := s.queries.GetSessionIdentity(ctx, sessionID)
	if err != nil {
		return SessionIdentity{}, fmt.Errorf(
			"failed to get session identity: %w", err,
		)
	}
	return SessionIdentityFromSqlc(si), nil
}

// DeleteSessionIdentity removes a session identity.
func (s *SqlcStore) DeleteSessionIdentity(
	ctx context.Context, sessionID string,
) error {

	err := s.queries.DeleteSessionIdentity(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session identity: %w", err)
	}
	return nil
}

// ListSessionIdentitiesByAgent lists session identities for an agent.
func (s *SqlcStore) ListSessionIdentitiesByAgent(
	ctx context.Context, agentID int64,
) ([]SessionIdentity, error) {

	rows, err := s.queries.ListSessionIdentitiesByAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to list session identities: %w", err,
		)
	}

	identities := make([]SessionIdentity, len(rows))
	for i, r := range rows {
		identities[i] = SessionIdentityFromSqlc(r)
	}
	return identities, nil
}

// UpdateSessionIdentityLastActive updates the last active timestamp.
func (s *SqlcStore) UpdateSessionIdentityLastActive(
	ctx context.Context, sessionID string, ts time.Time,
) error {

	err := s.queries.UpdateSessionIdentityLastActive(
		ctx, sqlc.UpdateSessionIdentityLastActiveParams{
			SessionID:    sessionID,
			LastActiveAt: ts.Unix(),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to update session identity last active: %w", err,
		)
	}
	return nil
}

// Helper functions for converting sqlc row types.

func inboxMessageFromRow(r sqlc.GetInboxMessagesRow) InboxMessage {
	msg := InboxMessage{
		Message: Message{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  r.Priority,
			CreatedAt: time.Unix(r.CreatedAt, 0),
		},
		SenderName: r.SenderName.String,
		State:      r.State,
	}
	if r.DeadlineAt.Valid {
		t := time.Unix(r.DeadlineAt.Int64, 0)
		msg.DeadlineAt = &t
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

func inboxMessageFromUnreadRow(r sqlc.GetUnreadMessagesRow) InboxMessage {
	msg := InboxMessage{
		Message: Message{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  r.Priority,
			CreatedAt: time.Unix(r.CreatedAt, 0),
		},
		SenderName: r.SenderName.String,
		State:      r.State,
	}
	if r.DeadlineAt.Valid {
		t := time.Unix(r.DeadlineAt.Int64, 0)
		msg.DeadlineAt = &t
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

func inboxMessageFromArchivedRow(r sqlc.GetArchivedMessagesRow) InboxMessage {
	msg := InboxMessage{
		Message: Message{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			TopicID:   r.TopicID,
			SenderID:  r.SenderID,
			Subject:   r.Subject,
			Body:      r.BodyMd,
			Priority:  r.Priority,
			CreatedAt: time.Unix(r.CreatedAt, 0),
		},
		// Note: GetArchivedMessages query doesn't include sender name.
		SenderName: "",
		State:      r.State,
	}
	if r.DeadlineAt.Valid {
		t := time.Unix(r.DeadlineAt.Int64, 0)
		msg.DeadlineAt = &t
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

// Ensure SqlcStore implements Store.
var _ Store = (*SqlcStore)(nil)
