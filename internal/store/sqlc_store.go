package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// nullInt64ToTime converts a sql.NullInt64 (unix timestamp) to *time.Time.
func nullInt64ToTime(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.Unix(n.Int64, 0)
	return &t
}

// QueryStore is the interface that defines all the sqlc query methods needed
// for substrate storage operations. This maps directly to sqlc.Querier
// methods.
type QueryStore interface {
	// Message operations.
	CreateMessage(
		ctx context.Context, arg sqlc.CreateMessageParams,
	) (sqlc.Message, error)
	GetMessage(ctx context.Context, id int64) (sqlc.Message, error)
	GetMessagesByThread(
		ctx context.Context, threadID string,
	) ([]sqlc.Message, error)
	GetMessagesByThreadWithSender(
		ctx context.Context, threadID string,
	) ([]sqlc.GetMessagesByThreadWithSenderRow, error)
	GetInboxMessages(
		ctx context.Context, arg sqlc.GetInboxMessagesParams,
	) ([]sqlc.GetInboxMessagesRow, error)
	GetUnreadMessages(
		ctx context.Context, arg sqlc.GetUnreadMessagesParams,
	) ([]sqlc.GetUnreadMessagesRow, error)
	GetArchivedMessages(
		ctx context.Context, arg sqlc.GetArchivedMessagesParams,
	) ([]sqlc.GetArchivedMessagesRow, error)
	UpdateRecipientState(
		ctx context.Context, arg sqlc.UpdateRecipientStateParams,
	) (int64, error)
	UpdateRecipientAcked(
		ctx context.Context, arg sqlc.UpdateRecipientAckedParams,
	) error
	UpdateRecipientSnoozed(
		ctx context.Context, arg sqlc.UpdateRecipientSnoozedParams,
	) error
	CreateMessageRecipient(
		ctx context.Context, arg sqlc.CreateMessageRecipientParams,
	) error
	GetMessageRecipient(
		ctx context.Context, arg sqlc.GetMessageRecipientParams,
	) (sqlc.MessageRecipient, error)
	CountUnreadByAgent(ctx context.Context, agentID int64) (int64, error)
	CountUnreadUrgentByAgent(
		ctx context.Context, agentID int64,
	) (int64, error)
	GetMessagesSinceOffset(
		ctx context.Context, arg sqlc.GetMessagesSinceOffsetParams,
	) ([]sqlc.Message, error)
	GetMaxLogOffset(ctx context.Context, topicID int64) (interface{}, error)
	GetAllInboxMessagesPaginated(
		ctx context.Context, arg sqlc.GetAllInboxMessagesPaginatedParams,
	) ([]sqlc.GetAllInboxMessagesPaginatedRow, error)
	GetMessageRecipients(
		ctx context.Context, messageID int64,
	) ([]sqlc.MessageRecipient, error)
	GetMessageRecipientsWithAgentsBulk(
		ctx context.Context, messageIDs []int64,
	) ([]sqlc.GetMessageRecipientsWithAgentsBulkRow, error)
	SearchMessages(
		ctx context.Context, arg sqlc.SearchMessagesParams,
	) ([]sqlc.SearchMessagesRow, error)
	GetMessagesByTopic(ctx context.Context, topicID int64) ([]sqlc.Message, error)

	// Agent operations.
	CreateAgent(
		ctx context.Context, arg sqlc.CreateAgentParams,
	) (sqlc.Agent, error)
	GetAgent(ctx context.Context, id int64) (sqlc.Agent, error)
	GetAgentByName(ctx context.Context, name string) (sqlc.Agent, error)
	GetAgentBySessionID(
		ctx context.Context, sessionID string,
	) (sqlc.Agent, error)
	ListAgents(ctx context.Context) ([]sqlc.Agent, error)
	ListAgentsByProject(
		ctx context.Context, projectKey sql.NullString,
	) ([]sqlc.Agent, error)
	UpdateAgentLastActive(
		ctx context.Context, arg sqlc.UpdateAgentLastActiveParams,
	) error
	UpdateAgentSession(
		ctx context.Context, arg sqlc.UpdateAgentSessionParams,
	) error
	UpdateAgentName(
		ctx context.Context, arg sqlc.UpdateAgentNameParams,
	) error
	DeleteAgent(ctx context.Context, id int64) error

	// Topic operations.
	CreateTopic(
		ctx context.Context, arg sqlc.CreateTopicParams,
	) (sqlc.Topic, error)
	GetTopic(ctx context.Context, id int64) (sqlc.Topic, error)
	GetTopicByName(ctx context.Context, name string) (sqlc.Topic, error)
	GetOrCreateAgentInboxTopic(
		ctx context.Context, arg sqlc.GetOrCreateAgentInboxTopicParams,
	) (sqlc.Topic, error)
	GetOrCreateTopic(
		ctx context.Context, arg sqlc.GetOrCreateTopicParams,
	) (sqlc.Topic, error)
	ListTopics(ctx context.Context) ([]sqlc.Topic, error)
	ListTopicsWithMessageCount(
		ctx context.Context,
	) ([]sqlc.ListTopicsWithMessageCountRow, error)
	ListTopicsByType(
		ctx context.Context, topicType string,
	) ([]sqlc.Topic, error)
	CreateSubscription(
		ctx context.Context, arg sqlc.CreateSubscriptionParams,
	) error
	DeleteSubscription(
		ctx context.Context, arg sqlc.DeleteSubscriptionParams,
	) error
	ListSubscriptionsByAgent(
		ctx context.Context, agentID int64,
	) ([]sqlc.Topic, error)
	ListSubscriptionsByTopic(
		ctx context.Context, topicID int64,
	) ([]sqlc.Agent, error)

	// Activity operations.
	CreateActivity(
		ctx context.Context, arg sqlc.CreateActivityParams,
	) (sqlc.Activity, error)
	ListRecentActivities(
		ctx context.Context, limit int64,
	) ([]sqlc.Activity, error)
	ListActivitiesByAgent(
		ctx context.Context, arg sqlc.ListActivitiesByAgentParams,
	) ([]sqlc.Activity, error)
	ListActivitiesSince(
		ctx context.Context, arg sqlc.ListActivitiesSinceParams,
	) ([]sqlc.Activity, error)
	DeleteOldActivities(ctx context.Context, createdAt int64) error

	// Session identity operations.
	CreateSessionIdentity(
		ctx context.Context, arg sqlc.CreateSessionIdentityParams,
	) error
	GetSessionIdentity(
		ctx context.Context, sessionID string,
	) (sqlc.SessionIdentity, error)
	DeleteSessionIdentity(ctx context.Context, sessionID string) error
	ListSessionIdentitiesByAgent(
		ctx context.Context, agentID int64,
	) ([]sqlc.SessionIdentity, error)
	UpdateSessionIdentityLastActive(
		ctx context.Context, arg sqlc.UpdateSessionIdentityLastActiveParams,
	) error

	// Sent message operations.
	GetSentMessages(
		ctx context.Context, arg sqlc.GetSentMessagesParams,
	) ([]sqlc.GetSentMessagesRow, error)
	GetAllSentMessages(
		ctx context.Context, limit int64,
	) ([]sqlc.GetAllSentMessagesRow, error)

	// Review operations.
	CreateReview(
		ctx context.Context, arg sqlc.CreateReviewParams,
	) (sqlc.Review, error)
	GetReview(
		ctx context.Context, reviewID string,
	) (sqlc.Review, error)
	ListReviews(
		ctx context.Context, arg sqlc.ListReviewsParams,
	) ([]sqlc.Review, error)
	ListReviewsByState(
		ctx context.Context, arg sqlc.ListReviewsByStateParams,
	) ([]sqlc.Review, error)
	ListReviewsByRequester(
		ctx context.Context, arg sqlc.ListReviewsByRequesterParams,
	) ([]sqlc.Review, error)
	ListActiveReviews(ctx context.Context) ([]sqlc.Review, error)
	UpdateReviewState(
		ctx context.Context, arg sqlc.UpdateReviewStateParams,
	) error
	UpdateReviewCompleted(
		ctx context.Context, arg sqlc.UpdateReviewCompletedParams,
	) error

	// Review iteration operations.
	CreateReviewIteration(
		ctx context.Context, arg sqlc.CreateReviewIterationParams,
	) (sqlc.ReviewIteration, error)
	GetReviewIterations(
		ctx context.Context, reviewID string,
	) ([]sqlc.ReviewIteration, error)

	// Review issue operations.
	CreateReviewIssue(
		ctx context.Context, arg sqlc.CreateReviewIssueParams,
	) (sqlc.ReviewIssue, error)
	GetReviewIssues(
		ctx context.Context, reviewID string,
	) ([]sqlc.ReviewIssue, error)
	GetOpenReviewIssues(
		ctx context.Context, reviewID string,
	) ([]sqlc.ReviewIssue, error)
	UpdateReviewIssueStatus(
		ctx context.Context, arg sqlc.UpdateReviewIssueStatusParams,
	) error
	CountOpenIssues(
		ctx context.Context, reviewID string,
	) (int64, error)
}

// BatchedQueryStore is a version of QueryStore that's capable of batched
// database operations with automatic retry on serialization errors.
type BatchedQueryStore interface {
	QueryStore

	db.BatchedTx[QueryStore]
}

// SqlcStore provides the Storage interface backed by sqlc queries with
// automatic transaction retry on SQLite busy/locked errors.
type SqlcStore struct {
	db    BatchedQueryStore
	sqlDB *sql.DB
}

// NewSqlcStore creates a new SqlcStore wrapping the given database connection.
func NewSqlcStore(sqlDB *sql.DB) *SqlcStore {
	baseDB := db.NewBaseDB(sqlDB)

	// Create query creator function for transaction executor.
	createQuery := func(tx *sql.Tx) QueryStore {
		return sqlc.New(tx)
	}

	executor := db.NewTransactionExecutor(baseDB, createQuery, nil)

	return &SqlcStore{
		db:    executor,
		sqlDB: sqlDB,
	}
}

// DB returns the underlying database connection for raw SQL queries like FTS5.
func (s *SqlcStore) DB() *sql.DB {
	return s.sqlDB
}

// Close closes the underlying database connection.
func (s *SqlcStore) Close() error {
	return s.sqlDB.Close()
}

// Ensure SqlcStore implements Storage.
var _ Storage = (*SqlcStore)(nil)

// FromDB creates a Storage implementation from a *sql.DB connection.
func FromDB(sqlDB *sql.DB) Storage {
	return NewSqlcStore(sqlDB)
}

// =============================================================================
// Transaction support
// =============================================================================

// StorageTxOptions defines transaction options for Storage operations.
type StorageTxOptions struct {
	readOnly bool
}

// ReadOnly returns true if this is a read-only transaction.
func (o *StorageTxOptions) ReadOnly() bool {
	return o.readOnly
}

// NewReadTx creates read-only transaction options.
func NewReadTx() *StorageTxOptions {
	return &StorageTxOptions{readOnly: true}
}

// NewWriteTx creates read-write transaction options.
func NewWriteTx() *StorageTxOptions {
	return &StorageTxOptions{readOnly: false}
}

// WithTx executes the given function within a database transaction.
func (s *SqlcStore) WithTx(ctx context.Context,
	fn func(ctx context.Context, store Storage) error,
) error {
	var writeTxOpts StorageTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(q QueryStore) error {
		// Create a txStore that wraps this transaction's queries.
		txStore := &txSqlcStore{
			queries: q,
			sqlDB:   s.sqlDB,
		}
		return fn(ctx, txStore)
	})
}

// WithReadTx executes the given function within a read-only database
// transaction. This ensures consistent snapshot reads across multiple queries.
func (s *SqlcStore) WithReadTx(ctx context.Context,
	fn func(ctx context.Context, store Storage) error,
) error {
	readTxOpts := NewReadTx()
	return s.db.ExecTx(ctx, readTxOpts, func(q QueryStore) error {
		// Create a txStore that wraps this transaction's queries.
		txStore := &txSqlcStore{
			queries: q,
			sqlDB:   s.sqlDB,
		}
		return fn(ctx, txStore)
	})
}

// txSqlcStore is a Storage implementation for use within a transaction.
type txSqlcStore struct {
	queries QueryStore
	sqlDB   *sql.DB
}

// WithTx for txSqlcStore returns an error since nested transactions are not
// supported.
func (s *txSqlcStore) WithTx(ctx context.Context,
	fn func(ctx context.Context, store Storage) error,
) error {
	return fmt.Errorf("nested transactions not supported: already within " +
		"a transaction context")
}

// WithReadTx for txSqlcStore returns an error since nested transactions are
// not supported.
func (s *txSqlcStore) WithReadTx(ctx context.Context,
	fn func(ctx context.Context, store Storage) error,
) error {
	return fmt.Errorf("nested transactions not supported: already within " +
		"a transaction context")
}

// Close is a no-op for transaction stores.
func (s *txSqlcStore) Close() error {
	return nil
}

// =============================================================================
// MessageStore implementation
// =============================================================================

// CreateMessage creates a new message in the database.
func (s *SqlcStore) CreateMessage(ctx context.Context,
	params CreateMessageParams,
) (Message, error) {
	msg, err := s.db.CreateMessage(ctx, sqlc.CreateMessageParams{
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
	})
	if err != nil {
		return Message{}, err
	}
	return MessageFromSqlc(msg), nil
}

// GetMessage retrieves a message by its ID.
func (s *SqlcStore) GetMessage(ctx context.Context, id int64) (Message, error) {
	msg, err := s.db.GetMessage(ctx, id)
	if err != nil {
		return Message{}, err
	}
	return MessageFromSqlc(msg), nil
}

// GetMessagesByThread retrieves all messages in a thread.
func (s *SqlcStore) GetMessagesByThread(ctx context.Context,
	threadID string,
) ([]Message, error) {
	rows, err := s.db.GetMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(rows))
	for i, row := range rows {
		messages[i] = MessageFromSqlc(row)
	}
	return messages, nil
}

// GetMessagesByThreadWithSender retrieves all messages in a thread with sender
// information (name, project, branch).
func (s *SqlcStore) GetMessagesByThreadWithSender(ctx context.Context,
	threadID string,
) ([]InboxMessage, error) {
	rows, err := s.db.GetMessagesByThreadWithSender(ctx, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]InboxMessage, len(rows))
	for i, row := range rows {
		messages[i] = InboxMessage{
			Message: Message{
				ID:              row.ID,
				ThreadID:        row.ThreadID,
				TopicID:         row.TopicID,
				LogOffset:       row.LogOffset,
				SenderID:        row.SenderID,
				Subject:         row.Subject,
				Body:            row.BodyMd,
				Priority:        row.Priority,
				DeadlineAt:      nullInt64ToTime(row.DeadlineAt),
				Attachments:     row.Attachments.String,
				CreatedAt:       time.Unix(row.CreatedAt, 0),
				DeletedBySender: row.DeletedBySender != 0,
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
		}
	}
	return messages, nil
}

// GetInboxMessages retrieves inbox messages for an agent.
func (s *SqlcStore) GetInboxMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.db.GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertInboxRows(rows), nil
}

// GetUnreadMessages retrieves unread messages for an agent.
func (s *SqlcStore) GetUnreadMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.db.GetUnreadMessages(ctx, sqlc.GetUnreadMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertUnreadRows(rows), nil
}

// GetArchivedMessages retrieves archived messages for an agent.
func (s *SqlcStore) GetArchivedMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.db.GetArchivedMessages(ctx, sqlc.GetArchivedMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertArchivedRows(rows), nil
}

// UpdateRecipientState updates the state of a message for a recipient.
func (s *SqlcStore) UpdateRecipientState(ctx context.Context, messageID,
	agentID int64, state string,
) error {
	_, err := s.db.UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     state,
		MessageID: messageID,
		AgentID:   agentID,
	})
	return err
}

// MarkMessageRead marks a message as read for a recipient.
func (s *SqlcStore) MarkMessageRead(ctx context.Context, messageID,
	agentID int64,
) error {
	now := time.Now().Unix()
	_, err := s.db.UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     "read",
		Column2:   "read",
		ReadAt:    sql.NullInt64{Int64: now, Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	return err
}

// AckMessage acknowledges a message for a recipient.
func (s *SqlcStore) AckMessage(ctx context.Context, messageID,
	agentID int64,
) error {
	return s.db.UpdateRecipientAcked(ctx, sqlc.UpdateRecipientAckedParams{
		AckedAt:   sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
}

// SnoozeMessage snoozes a message until the given time.
func (s *SqlcStore) SnoozeMessage(ctx context.Context, messageID,
	agentID int64, until time.Time,
) error {
	return s.db.UpdateRecipientSnoozed(ctx, sqlc.UpdateRecipientSnoozedParams{
		SnoozedUntil: sql.NullInt64{Int64: until.Unix(), Valid: true},
		MessageID:    messageID,
		AgentID:      agentID,
	})
}

// CreateMessageRecipient creates a recipient entry for a message.
func (s *SqlcStore) CreateMessageRecipient(ctx context.Context, messageID,
	agentID int64,
) error {
	return s.db.CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: messageID,
		AgentID:   agentID,
	})
}

// GetMessageRecipient retrieves the recipient state for a message.
func (s *SqlcStore) GetMessageRecipient(ctx context.Context, messageID,
	agentID int64,
) (MessageRecipient, error) {
	row, err := s.db.GetMessageRecipient(ctx, sqlc.GetMessageRecipientParams{
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		return MessageRecipient{}, err
	}
	return MessageRecipientFromSqlc(row), nil
}

// CountUnreadByAgent counts unread messages for an agent.
func (s *SqlcStore) CountUnreadByAgent(ctx context.Context,
	agentID int64,
) (int64, error) {
	return s.db.CountUnreadByAgent(ctx, agentID)
}

// CountUnreadUrgentByAgent counts urgent unread messages for an agent.
func (s *SqlcStore) CountUnreadUrgentByAgent(ctx context.Context,
	agentID int64,
) (int64, error) {
	return s.db.CountUnreadUrgentByAgent(ctx, agentID)
}

// GetMessagesSinceOffset retrieves messages after a given log offset.
func (s *SqlcStore) GetMessagesSinceOffset(ctx context.Context, topicID,
	offset int64, limit int,
) ([]Message, error) {
	rows, err := s.db.GetMessagesSinceOffset(ctx, sqlc.GetMessagesSinceOffsetParams{
		TopicID:   topicID,
		LogOffset: offset,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(rows))
	for i, row := range rows {
		messages[i] = MessageFromSqlc(row)
	}
	return messages, nil
}

// NextLogOffset returns the next available log offset for a topic.
func (s *SqlcStore) NextLogOffset(ctx context.Context,
	topicID int64,
) (int64, error) {
	result, err := s.db.GetMaxLogOffset(ctx, topicID)
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

// SearchMessagesForAgent performs full-text search for messages visible to
// a specific agent.
func (s *SqlcStore) SearchMessagesForAgent(ctx context.Context, query string,
	agentID int64, limit int,
) ([]Message, error) {
	// Use raw SQL for FTS5 search.
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT m.id, m.thread_id, m.topic_id, m.log_offset, m.sender_id,
		       m.subject, m.body_md, m.priority, m.deadline_at,
		       m.attachments, m.created_at
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		JOIN message_recipients mr ON m.id = mr.message_id
		WHERE messages_fts MATCH ? AND mr.agent_id = ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var deadlineAt sql.NullInt64
		var attachments sql.NullString
		var createdAt int64

		err := rows.Scan(
			&msg.ID, &msg.ThreadID, &msg.TopicID, &msg.LogOffset,
			&msg.SenderID, &msg.Subject, &msg.Body, &msg.Priority,
			&deadlineAt, &attachments, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		if deadlineAt.Valid {
			t := time.Unix(deadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		msg.Attachments = attachments.String
		msg.CreatedAt = time.Unix(createdAt, 0)

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return messages, nil
}

// GetAllInboxMessages retrieves inbox messages across all agents (global view).
func (s *SqlcStore) GetAllInboxMessages(ctx context.Context, limit,
	offset int) ([]InboxMessage, error) {

	rows, err := s.db.GetAllInboxMessagesPaginated(ctx, sqlc.GetAllInboxMessagesPaginatedParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
			State:            row.State,
			SnoozedUntil:     nullInt64ToTime(row.SnoozedUntil),
			ReadAt:           nullInt64ToTime(row.ReadAt),
			AckedAt:          nullInt64ToTime(row.AckedAt),
		})
	}
	return messages, nil
}

// GetMessageRecipients retrieves all recipients for a message.
func (s *SqlcStore) GetMessageRecipients(ctx context.Context,
	messageID int64) ([]MessageRecipientWithAgent, error) {

	rows, err := s.db.GetMessageRecipients(ctx, messageID)
	if err != nil {
		return nil, err
	}

	recipients := make([]MessageRecipientWithAgent, 0, len(rows))
	for _, row := range rows {
		// Need to fetch agent name separately or use bulk query.
		agent, _ := s.db.GetAgent(ctx, row.AgentID)
		recipients = append(recipients, MessageRecipientWithAgent{
			MessageRecipient: MessageRecipient{
				MessageID:    row.MessageID,
				AgentID:      row.AgentID,
				State:        row.State,
				SnoozedUntil: nullInt64ToTime(row.SnoozedUntil),
				ReadAt:       nullInt64ToTime(row.ReadAt),
				AckedAt:      nullInt64ToTime(row.AckedAt),
			},
			AgentName: agent.Name,
		})
	}
	return recipients, nil
}

// GetMessageRecipientsBulk retrieves recipients for multiple messages.
func (s *SqlcStore) GetMessageRecipientsBulk(ctx context.Context,
	messageIDs []int64) (map[int64][]MessageRecipientWithAgent, error) {

	rows, err := s.db.GetMessageRecipientsWithAgentsBulk(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]MessageRecipientWithAgent)
	for _, row := range rows {
		rec := MessageRecipientWithAgent{
			MessageRecipient: MessageRecipient{
				MessageID:    row.MessageID,
				AgentID:      row.AgentID,
				State:        row.State,
				SnoozedUntil: nullInt64ToTime(row.SnoozedUntil),
				ReadAt:       nullInt64ToTime(row.ReadAt),
				AckedAt:      nullInt64ToTime(row.AckedAt),
			},
			AgentName: row.AgentName.String,
		}
		result[row.MessageID] = append(result[row.MessageID], rec)
	}
	return result, nil
}

// SearchMessages performs global search across all messages.
func (s *SqlcStore) SearchMessages(ctx context.Context, query string,
	limit int) ([]InboxMessage, error) {

	// Escape LIKE wildcards.
	escapedQuery := "%" + query + "%"
	rows, err := s.db.SearchMessages(ctx, sqlc.SearchMessagesParams{
		Subject: escapedQuery,
		BodyMd:  escapedQuery,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
		})
	}
	return messages, nil
}

// GetMessagesByTopic retrieves all messages for a topic.
func (s *SqlcStore) GetMessagesByTopic(ctx context.Context,
	topicID int64) ([]Message, error) {

	rows, err := s.db.GetMessagesByTopic(ctx, topicID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, MessageFromSqlc(row))
	}
	return messages, nil
}

// GetSentMessages retrieves messages sent by a specific agent.
func (s *SqlcStore) GetSentMessages(ctx context.Context, senderID int64,
	limit int) ([]Message, error) {

	rows, err := s.db.GetSentMessages(ctx, sqlc.GetSentMessagesParams{
		SenderID: senderID,
		Limit:    int64(limit),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		msg := Message{
			ID:              row.ID,
			ThreadID:        row.ThreadID,
			TopicID:         row.TopicID,
			LogOffset:       row.LogOffset,
			SenderID:        row.SenderID,
			Subject:         row.Subject,
			Body:            row.BodyMd,
			Priority:        row.Priority,
			CreatedAt:       time.Unix(row.CreatedAt, 0),
			DeletedBySender: row.DeletedBySender == 1,
		}
		if row.DeadlineAt.Valid {
			t := time.Unix(row.DeadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		if row.Attachments.Valid {
			msg.Attachments = row.Attachments.String
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetAllSentMessages retrieves all sent messages across all agents.
func (s *SqlcStore) GetAllSentMessages(ctx context.Context,
	limit int) ([]InboxMessage, error) {

	rows, err := s.db.GetAllSentMessages(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
		})
	}
	return messages, nil
}

// =============================================================================
// AgentStore implementation
// =============================================================================

// CreateAgent creates a new agent in the database.
func (s *SqlcStore) CreateAgent(ctx context.Context,
	params CreateAgentParams,
) (Agent, error) {
	now := time.Now().Unix()
	agent, err := s.db.CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:         params.Name,
		ProjectKey:   ToSqlcNullString(params.ProjectKey),
		GitBranch:    ToSqlcNullString(params.GitBranch),
		CreatedAt:    now,
		LastActiveAt: now,
	})
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgent retrieves an agent by its ID.
func (s *SqlcStore) GetAgent(ctx context.Context, id int64) (Agent, error) {
	agent, err := s.db.GetAgent(ctx, id)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgentByName retrieves an agent by its name.
func (s *SqlcStore) GetAgentByName(ctx context.Context,
	name string,
) (Agent, error) {
	agent, err := s.db.GetAgentByName(ctx, name)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgentBySessionID retrieves an agent by session ID.
func (s *SqlcStore) GetAgentBySessionID(ctx context.Context,
	sessionID string,
) (Agent, error) {
	agent, err := s.db.GetAgentBySessionID(ctx, sessionID)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// ListAgents lists all agents.
func (s *SqlcStore) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.db.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// ListAgentsByProject lists agents for a specific project.
func (s *SqlcStore) ListAgentsByProject(ctx context.Context,
	projectKey string,
) ([]Agent, error) {
	rows, err := s.db.ListAgentsByProject(ctx, ToSqlcNullString(projectKey))
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// UpdateLastActive updates the last active timestamp for an agent.
func (s *SqlcStore) UpdateLastActive(ctx context.Context, id int64,
	ts time.Time,
) error {
	return s.db.UpdateAgentLastActive(ctx, sqlc.UpdateAgentLastActiveParams{
		LastActiveAt: ts.Unix(),
		ID:           id,
	})
}

// UpdateSession updates the session ID for an agent.
func (s *SqlcStore) UpdateSession(ctx context.Context, id int64,
	sessionID string,
) error {
	return s.db.UpdateAgentSession(ctx, sqlc.UpdateAgentSessionParams{
		CurrentSessionID: ToSqlcNullString(sessionID),
		ID:               id,
	})
}

// UpdateAgentName updates an agent's display name.
func (s *SqlcStore) UpdateAgentName(ctx context.Context, id int64,
	name string) error {

	return s.db.UpdateAgentName(ctx, sqlc.UpdateAgentNameParams{
		Name: name,
		ID:   id,
	})
}

// DeleteAgent deletes an agent by its ID.
func (s *SqlcStore) DeleteAgent(ctx context.Context, id int64) error {
	return s.db.DeleteAgent(ctx, id)
}

// =============================================================================
// TopicStore implementation
// =============================================================================

// CreateTopic creates a new topic.
func (s *SqlcStore) CreateTopic(ctx context.Context,
	params CreateTopicParams,
) (Topic, error) {
	topic, err := s.db.CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      params.Name,
		TopicType: params.TopicType,
		RetentionSeconds: sql.NullInt64{
			Int64: params.RetentionSeconds,
			Valid: params.RetentionSeconds > 0,
		},
	})
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetTopic retrieves a topic by its ID.
func (s *SqlcStore) GetTopic(ctx context.Context, id int64) (Topic, error) {
	topic, err := s.db.GetTopic(ctx, id)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetTopicByName retrieves a topic by its name.
func (s *SqlcStore) GetTopicByName(ctx context.Context,
	name string,
) (Topic, error) {
	topic, err := s.db.GetTopicByName(ctx, name)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetOrCreateAgentInboxTopic gets or creates an agent's inbox topic.
func (s *SqlcStore) GetOrCreateAgentInboxTopic(ctx context.Context,
	agentName string,
) (Topic, error) {
	topic, err := s.db.GetOrCreateAgentInboxTopic(
		ctx, sqlc.GetOrCreateAgentInboxTopicParams{
			Column1:   sql.NullString{String: agentName, Valid: true},
			CreatedAt: time.Now().Unix(),
		},
	)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetOrCreateTopic gets or creates a topic by name.
func (s *SqlcStore) GetOrCreateTopic(ctx context.Context, name,
	topicType string,
) (Topic, error) {
	topic, err := s.db.GetOrCreateTopic(ctx, sqlc.GetOrCreateTopicParams{
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// ListTopics lists all topics with their message counts.
func (s *SqlcStore) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := s.db.ListTopicsWithMessageCount(ctx)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicWithCountFromSqlc(row)
	}
	return topics, nil
}

// ListTopicsByType lists topics of a specific type.
func (s *SqlcStore) ListTopicsByType(ctx context.Context,
	topicType string,
) ([]Topic, error) {
	rows, err := s.db.ListTopicsByType(ctx, topicType)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicFromSqlc(row)
	}
	return topics, nil
}

// CreateSubscription subscribes an agent to a topic.
func (s *SqlcStore) CreateSubscription(ctx context.Context, agentID,
	topicID int64,
) error {
	return s.db.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID: agentID,
		TopicID: topicID,
	})
}

// DeleteSubscription unsubscribes an agent from a topic.
func (s *SqlcStore) DeleteSubscription(ctx context.Context, agentID,
	topicID int64,
) error {
	return s.db.DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
		AgentID: agentID,
		TopicID: topicID,
	})
}

// ListSubscriptionsByAgent lists topics an agent is subscribed to.
func (s *SqlcStore) ListSubscriptionsByAgent(ctx context.Context,
	agentID int64,
) ([]Topic, error) {
	rows, err := s.db.ListSubscriptionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicFromSqlc(row)
	}
	return topics, nil
}

// ListSubscriptionsByTopic lists agents subscribed to a topic.
func (s *SqlcStore) ListSubscriptionsByTopic(ctx context.Context,
	topicID int64,
) ([]Agent, error) {
	rows, err := s.db.ListSubscriptionsByTopic(ctx, topicID)
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// =============================================================================
// ActivityStore implementation
// =============================================================================

// CreateActivity records a new activity event.
func (s *SqlcStore) CreateActivity(ctx context.Context,
	params CreateActivityParams,
) error {
	_, err := s.db.CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      params.AgentID,
		ActivityType: params.ActivityType,
		Description:  params.Description,
		Metadata:     ToSqlcNullString(params.Metadata),
	})
	return err
}

// ListRecentActivities lists the most recent activities.
func (s *SqlcStore) ListRecentActivities(ctx context.Context,
	limit int,
) ([]Activity, error) {
	rows, err := s.db.ListRecentActivities(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// ListActivitiesByAgent lists activities for a specific agent.
func (s *SqlcStore) ListActivitiesByAgent(ctx context.Context, agentID int64,
	limit int,
) ([]Activity, error) {
	rows, err := s.db.ListActivitiesByAgent(ctx, sqlc.ListActivitiesByAgentParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// ListActivitiesSince lists activities since a given timestamp.
func (s *SqlcStore) ListActivitiesSince(ctx context.Context, since time.Time,
	limit int,
) ([]Activity, error) {
	rows, err := s.db.ListActivitiesSince(ctx, sqlc.ListActivitiesSinceParams{
		CreatedAt: since.Unix(),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// DeleteOldActivities removes activities older than a given time.
func (s *SqlcStore) DeleteOldActivities(ctx context.Context,
	olderThan time.Time,
) error {
	return s.db.DeleteOldActivities(ctx, olderThan.Unix())
}

// =============================================================================
// SessionStore implementation
// =============================================================================

// CreateSessionIdentity creates a new session identity mapping.
func (s *SqlcStore) CreateSessionIdentity(ctx context.Context,
	params CreateSessionIdentityParams,
) error {
	return s.db.CreateSessionIdentity(ctx, sqlc.CreateSessionIdentityParams{
		SessionID:  params.SessionID,
		AgentID:    params.AgentID,
		ProjectKey: ToSqlcNullString(params.ProjectKey),
		GitBranch:  ToSqlcNullString(params.GitBranch),
	})
}

// GetSessionIdentity retrieves a session identity by session ID.
func (s *SqlcStore) GetSessionIdentity(ctx context.Context,
	sessionID string,
) (SessionIdentity, error) {
	row, err := s.db.GetSessionIdentity(ctx, sessionID)
	if err != nil {
		return SessionIdentity{}, err
	}
	return SessionIdentityFromSqlc(row), nil
}

// DeleteSessionIdentity removes a session identity.
func (s *SqlcStore) DeleteSessionIdentity(ctx context.Context,
	sessionID string,
) error {
	return s.db.DeleteSessionIdentity(ctx, sessionID)
}

// ListSessionIdentitiesByAgent lists session identities for an agent.
func (s *SqlcStore) ListSessionIdentitiesByAgent(ctx context.Context,
	agentID int64,
) ([]SessionIdentity, error) {
	rows, err := s.db.ListSessionIdentitiesByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	sessions := make([]SessionIdentity, len(rows))
	for i, row := range rows {
		sessions[i] = SessionIdentityFromSqlc(row)
	}
	return sessions, nil
}

// UpdateSessionIdentityLastActive updates the last active timestamp.
func (s *SqlcStore) UpdateSessionIdentityLastActive(ctx context.Context,
	sessionID string, ts time.Time,
) error {
	return s.db.UpdateSessionIdentityLastActive(
		ctx, sqlc.UpdateSessionIdentityLastActiveParams{
			LastActiveAt: ts.Unix(),
			SessionID:    sessionID,
		},
	)
}

// =============================================================================
// txSqlcStore implementations (same methods, delegate to queries)
// =============================================================================

// CreateMessage creates a new message in the database.
func (s *txSqlcStore) CreateMessage(ctx context.Context,
	params CreateMessageParams,
) (Message, error) {
	msg, err := s.queries.CreateMessage(ctx, sqlc.CreateMessageParams{
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
	})
	if err != nil {
		return Message{}, err
	}
	return MessageFromSqlc(msg), nil
}

// GetMessage retrieves a message by its ID.
func (s *txSqlcStore) GetMessage(ctx context.Context,
	id int64,
) (Message, error) {
	msg, err := s.queries.GetMessage(ctx, id)
	if err != nil {
		return Message{}, err
	}
	return MessageFromSqlc(msg), nil
}

// GetMessagesByThread retrieves all messages in a thread.
func (s *txSqlcStore) GetMessagesByThread(ctx context.Context,
	threadID string,
) ([]Message, error) {
	rows, err := s.queries.GetMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(rows))
	for i, row := range rows {
		messages[i] = MessageFromSqlc(row)
	}
	return messages, nil
}

// GetMessagesByThreadWithSender retrieves all messages in a thread with sender
// information (name, project, branch).
func (s *txSqlcStore) GetMessagesByThreadWithSender(ctx context.Context,
	threadID string,
) ([]InboxMessage, error) {
	rows, err := s.queries.GetMessagesByThreadWithSender(ctx, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]InboxMessage, len(rows))
	for i, row := range rows {
		messages[i] = InboxMessage{
			Message: Message{
				ID:              row.ID,
				ThreadID:        row.ThreadID,
				TopicID:         row.TopicID,
				LogOffset:       row.LogOffset,
				SenderID:        row.SenderID,
				Subject:         row.Subject,
				Body:            row.BodyMd,
				Priority:        row.Priority,
				DeadlineAt:      nullInt64ToTime(row.DeadlineAt),
				Attachments:     row.Attachments.String,
				CreatedAt:       time.Unix(row.CreatedAt, 0),
				DeletedBySender: row.DeletedBySender != 0,
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
		}
	}
	return messages, nil
}

// GetInboxMessages retrieves inbox messages for an agent.
func (s *txSqlcStore) GetInboxMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.queries.GetInboxMessages(ctx, sqlc.GetInboxMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertInboxRows(rows), nil
}

// GetUnreadMessages retrieves unread messages for an agent.
func (s *txSqlcStore) GetUnreadMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.queries.GetUnreadMessages(ctx, sqlc.GetUnreadMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertUnreadRows(rows), nil
}

// GetArchivedMessages retrieves archived messages for an agent.
func (s *txSqlcStore) GetArchivedMessages(ctx context.Context, agentID int64,
	limit int,
) ([]InboxMessage, error) {
	rows, err := s.queries.GetArchivedMessages(ctx, sqlc.GetArchivedMessagesParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return convertArchivedRows(rows), nil
}

// UpdateRecipientState updates the state of a message for a recipient.
func (s *txSqlcStore) UpdateRecipientState(ctx context.Context, messageID,
	agentID int64, state string,
) error {
	_, err := s.queries.UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     state,
		MessageID: messageID,
		AgentID:   agentID,
	})
	return err
}

// MarkMessageRead marks a message as read for a recipient.
func (s *txSqlcStore) MarkMessageRead(ctx context.Context, messageID,
	agentID int64,
) error {
	now := time.Now().Unix()
	_, err := s.queries.UpdateRecipientState(ctx, sqlc.UpdateRecipientStateParams{
		State:     "read",
		Column2:   "read",
		ReadAt:    sql.NullInt64{Int64: now, Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
	return err
}

// AckMessage acknowledges a message for a recipient.
func (s *txSqlcStore) AckMessage(ctx context.Context, messageID,
	agentID int64,
) error {
	return s.queries.UpdateRecipientAcked(ctx, sqlc.UpdateRecipientAckedParams{
		AckedAt:   sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		MessageID: messageID,
		AgentID:   agentID,
	})
}

// SnoozeMessage snoozes a message until the given time.
func (s *txSqlcStore) SnoozeMessage(ctx context.Context, messageID,
	agentID int64, until time.Time,
) error {
	return s.queries.UpdateRecipientSnoozed(ctx, sqlc.UpdateRecipientSnoozedParams{
		SnoozedUntil: sql.NullInt64{Int64: until.Unix(), Valid: true},
		MessageID:    messageID,
		AgentID:      agentID,
	})
}

// CreateMessageRecipient creates a recipient entry for a message.
func (s *txSqlcStore) CreateMessageRecipient(ctx context.Context, messageID,
	agentID int64,
) error {
	return s.queries.CreateMessageRecipient(ctx, sqlc.CreateMessageRecipientParams{
		MessageID: messageID,
		AgentID:   agentID,
	})
}

// GetMessageRecipient retrieves the recipient state for a message.
func (s *txSqlcStore) GetMessageRecipient(ctx context.Context, messageID,
	agentID int64,
) (MessageRecipient, error) {
	row, err := s.queries.GetMessageRecipient(ctx, sqlc.GetMessageRecipientParams{
		MessageID: messageID,
		AgentID:   agentID,
	})
	if err != nil {
		return MessageRecipient{}, err
	}
	return MessageRecipientFromSqlc(row), nil
}

// CountUnreadByAgent counts unread messages for an agent.
func (s *txSqlcStore) CountUnreadByAgent(ctx context.Context,
	agentID int64,
) (int64, error) {
	return s.queries.CountUnreadByAgent(ctx, agentID)
}

// CountUnreadUrgentByAgent counts urgent unread messages for an agent.
func (s *txSqlcStore) CountUnreadUrgentByAgent(ctx context.Context,
	agentID int64,
) (int64, error) {
	return s.queries.CountUnreadUrgentByAgent(ctx, agentID)
}

// GetMessagesSinceOffset retrieves messages after a given log offset.
func (s *txSqlcStore) GetMessagesSinceOffset(ctx context.Context, topicID,
	offset int64, limit int,
) ([]Message, error) {
	rows, err := s.queries.GetMessagesSinceOffset(ctx, sqlc.GetMessagesSinceOffsetParams{
		TopicID:   topicID,
		LogOffset: offset,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(rows))
	for i, row := range rows {
		messages[i] = MessageFromSqlc(row)
	}
	return messages, nil
}

// NextLogOffset returns the next available log offset for a topic.
func (s *txSqlcStore) NextLogOffset(ctx context.Context,
	topicID int64,
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

// SearchMessagesForAgent performs full-text search for messages visible to
// a specific agent.
func (s *txSqlcStore) SearchMessagesForAgent(ctx context.Context, query string,
	agentID int64, limit int,
) ([]Message, error) {
	// Use raw SQL for FTS5 search.
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT m.id, m.thread_id, m.topic_id, m.log_offset, m.sender_id,
		       m.subject, m.body_md, m.priority, m.deadline_at,
		       m.attachments, m.created_at
		FROM messages m
		JOIN messages_fts fts ON m.id = fts.rowid
		JOIN message_recipients mr ON m.id = mr.message_id
		WHERE messages_fts MATCH ? AND mr.agent_id = ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var deadlineAt sql.NullInt64
		var attachments sql.NullString
		var createdAt int64

		err := rows.Scan(
			&msg.ID, &msg.ThreadID, &msg.TopicID, &msg.LogOffset,
			&msg.SenderID, &msg.Subject, &msg.Body, &msg.Priority,
			&deadlineAt, &attachments, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		if deadlineAt.Valid {
			t := time.Unix(deadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		msg.Attachments = attachments.String
		msg.CreatedAt = time.Unix(createdAt, 0)

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return messages, nil
}

// CreateAgent creates a new agent in the database.
func (s *txSqlcStore) CreateAgent(ctx context.Context,
	params CreateAgentParams,
) (Agent, error) {
	agent, err := s.queries.CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:       params.Name,
		ProjectKey: ToSqlcNullString(params.ProjectKey),
		GitBranch:  ToSqlcNullString(params.GitBranch),
	})
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgent retrieves an agent by its ID.
func (s *txSqlcStore) GetAgent(ctx context.Context, id int64) (Agent, error) {
	agent, err := s.queries.GetAgent(ctx, id)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgentByName retrieves an agent by its name.
func (s *txSqlcStore) GetAgentByName(ctx context.Context,
	name string,
) (Agent, error) {
	agent, err := s.queries.GetAgentByName(ctx, name)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// GetAgentBySessionID retrieves an agent by session ID.
func (s *txSqlcStore) GetAgentBySessionID(ctx context.Context,
	sessionID string,
) (Agent, error) {
	agent, err := s.queries.GetAgentBySessionID(ctx, sessionID)
	if err != nil {
		return Agent{}, err
	}
	return AgentFromSqlc(agent), nil
}

// ListAgents lists all agents.
func (s *txSqlcStore) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.queries.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// ListAgentsByProject lists agents for a specific project.
func (s *txSqlcStore) ListAgentsByProject(ctx context.Context,
	projectKey string,
) ([]Agent, error) {
	rows, err := s.queries.ListAgentsByProject(ctx, ToSqlcNullString(projectKey))
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// UpdateLastActive updates the last active timestamp for an agent.
func (s *txSqlcStore) UpdateLastActive(ctx context.Context, id int64,
	ts time.Time,
) error {
	return s.queries.UpdateAgentLastActive(ctx, sqlc.UpdateAgentLastActiveParams{
		LastActiveAt: ts.Unix(),
		ID:           id,
	})
}

// UpdateSession updates the session ID for an agent.
func (s *txSqlcStore) UpdateSession(ctx context.Context, id int64,
	sessionID string,
) error {
	return s.queries.UpdateAgentSession(ctx, sqlc.UpdateAgentSessionParams{
		CurrentSessionID: ToSqlcNullString(sessionID),
		ID:               id,
	})
}

// DeleteAgent deletes an agent by its ID.
func (s *txSqlcStore) DeleteAgent(ctx context.Context, id int64) error {
	return s.queries.DeleteAgent(ctx, id)
}

// CreateTopic creates a new topic.
func (s *txSqlcStore) CreateTopic(ctx context.Context,
	params CreateTopicParams,
) (Topic, error) {
	topic, err := s.queries.CreateTopic(ctx, sqlc.CreateTopicParams{
		Name:      params.Name,
		TopicType: params.TopicType,
		RetentionSeconds: sql.NullInt64{
			Int64: params.RetentionSeconds,
			Valid: params.RetentionSeconds > 0,
		},
	})
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetTopic retrieves a topic by its ID.
func (s *txSqlcStore) GetTopic(ctx context.Context, id int64) (Topic, error) {
	topic, err := s.queries.GetTopic(ctx, id)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetTopicByName retrieves a topic by its name.
func (s *txSqlcStore) GetTopicByName(ctx context.Context,
	name string,
) (Topic, error) {
	topic, err := s.queries.GetTopicByName(ctx, name)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetOrCreateAgentInboxTopic gets or creates an agent's inbox topic.
func (s *txSqlcStore) GetOrCreateAgentInboxTopic(ctx context.Context,
	agentName string,
) (Topic, error) {
	topic, err := s.queries.GetOrCreateAgentInboxTopic(
		ctx, sqlc.GetOrCreateAgentInboxTopicParams{
			Column1:   sql.NullString{String: agentName, Valid: true},
			CreatedAt: time.Now().Unix(),
		},
	)
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// GetOrCreateTopic gets or creates a topic by name.
func (s *txSqlcStore) GetOrCreateTopic(ctx context.Context, name,
	topicType string,
) (Topic, error) {
	topic, err := s.queries.GetOrCreateTopic(ctx, sqlc.GetOrCreateTopicParams{
		Name:      name,
		TopicType: topicType,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return Topic{}, err
	}
	return TopicFromSqlc(topic), nil
}

// ListTopics lists all topics with their message counts.
func (s *txSqlcStore) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := s.queries.ListTopicsWithMessageCount(ctx)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicWithCountFromSqlc(row)
	}
	return topics, nil
}

// ListTopicsByType lists topics of a specific type.
func (s *txSqlcStore) ListTopicsByType(ctx context.Context,
	topicType string,
) ([]Topic, error) {
	rows, err := s.queries.ListTopicsByType(ctx, topicType)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicFromSqlc(row)
	}
	return topics, nil
}

// CreateSubscription subscribes an agent to a topic.
func (s *txSqlcStore) CreateSubscription(ctx context.Context, agentID,
	topicID int64,
) error {
	return s.queries.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		AgentID: agentID,
		TopicID: topicID,
	})
}

// DeleteSubscription unsubscribes an agent from a topic.
func (s *txSqlcStore) DeleteSubscription(ctx context.Context, agentID,
	topicID int64,
) error {
	return s.queries.DeleteSubscription(ctx, sqlc.DeleteSubscriptionParams{
		AgentID: agentID,
		TopicID: topicID,
	})
}

// ListSubscriptionsByAgent lists topics an agent is subscribed to.
func (s *txSqlcStore) ListSubscriptionsByAgent(ctx context.Context,
	agentID int64,
) ([]Topic, error) {
	rows, err := s.queries.ListSubscriptionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		topics[i] = TopicFromSqlc(row)
	}
	return topics, nil
}

// ListSubscriptionsByTopic lists agents subscribed to a topic.
func (s *txSqlcStore) ListSubscriptionsByTopic(ctx context.Context,
	topicID int64,
) ([]Agent, error) {
	rows, err := s.queries.ListSubscriptionsByTopic(ctx, topicID)
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, len(rows))
	for i, row := range rows {
		agents[i] = AgentFromSqlc(row)
	}
	return agents, nil
}

// CreateActivity records a new activity event.
func (s *txSqlcStore) CreateActivity(ctx context.Context,
	params CreateActivityParams,
) error {
	_, err := s.queries.CreateActivity(ctx, sqlc.CreateActivityParams{
		AgentID:      params.AgentID,
		ActivityType: params.ActivityType,
		Description:  params.Description,
		Metadata:     ToSqlcNullString(params.Metadata),
	})
	return err
}

// ListRecentActivities lists the most recent activities.
func (s *txSqlcStore) ListRecentActivities(ctx context.Context,
	limit int,
) ([]Activity, error) {
	rows, err := s.queries.ListRecentActivities(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// ListActivitiesByAgent lists activities for a specific agent.
func (s *txSqlcStore) ListActivitiesByAgent(ctx context.Context, agentID int64,
	limit int,
) ([]Activity, error) {
	rows, err := s.queries.ListActivitiesByAgent(ctx, sqlc.ListActivitiesByAgentParams{
		AgentID: agentID,
		Limit:   int64(limit),
	})
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// ListActivitiesSince lists activities since a given timestamp.
func (s *txSqlcStore) ListActivitiesSince(ctx context.Context, since time.Time,
	limit int,
) ([]Activity, error) {
	rows, err := s.queries.ListActivitiesSince(ctx, sqlc.ListActivitiesSinceParams{
		CreatedAt: since.Unix(),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	activities := make([]Activity, len(rows))
	for i, row := range rows {
		activities[i] = ActivityFromSqlc(row)
	}
	return activities, nil
}

// DeleteOldActivities removes activities older than a given time.
func (s *txSqlcStore) DeleteOldActivities(ctx context.Context,
	olderThan time.Time,
) error {
	return s.queries.DeleteOldActivities(ctx, olderThan.Unix())
}

// CreateSessionIdentity creates a new session identity mapping.
func (s *txSqlcStore) CreateSessionIdentity(ctx context.Context,
	params CreateSessionIdentityParams,
) error {
	return s.queries.CreateSessionIdentity(ctx, sqlc.CreateSessionIdentityParams{
		SessionID:  params.SessionID,
		AgentID:    params.AgentID,
		ProjectKey: ToSqlcNullString(params.ProjectKey),
		GitBranch:  ToSqlcNullString(params.GitBranch),
	})
}

// GetSessionIdentity retrieves a session identity by session ID.
func (s *txSqlcStore) GetSessionIdentity(ctx context.Context,
	sessionID string,
) (SessionIdentity, error) {
	row, err := s.queries.GetSessionIdentity(ctx, sessionID)
	if err != nil {
		return SessionIdentity{}, err
	}
	return SessionIdentityFromSqlc(row), nil
}

// DeleteSessionIdentity removes a session identity.
func (s *txSqlcStore) DeleteSessionIdentity(ctx context.Context,
	sessionID string,
) error {
	return s.queries.DeleteSessionIdentity(ctx, sessionID)
}

// ListSessionIdentitiesByAgent lists session identities for an agent.
func (s *txSqlcStore) ListSessionIdentitiesByAgent(ctx context.Context,
	agentID int64,
) ([]SessionIdentity, error) {
	rows, err := s.queries.ListSessionIdentitiesByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	sessions := make([]SessionIdentity, len(rows))
	for i, row := range rows {
		sessions[i] = SessionIdentityFromSqlc(row)
	}
	return sessions, nil
}

// UpdateSessionIdentityLastActive updates the last active timestamp.
func (s *txSqlcStore) UpdateSessionIdentityLastActive(ctx context.Context,
	sessionID string, ts time.Time,
) error {
	return s.queries.UpdateSessionIdentityLastActive(
		ctx, sqlc.UpdateSessionIdentityLastActiveParams{
			LastActiveAt: ts.Unix(),
			SessionID:    sessionID,
		},
	)
}

// GetAllInboxMessages retrieves inbox messages across all agents (global view).
func (s *txSqlcStore) GetAllInboxMessages(ctx context.Context, limit,
	offset int) ([]InboxMessage, error) {

	rows, err := s.queries.GetAllInboxMessagesPaginated(
		ctx, sqlc.GetAllInboxMessagesPaginatedParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		},
	)
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
			State:            row.State,
			SnoozedUntil:     nullInt64ToTime(row.SnoozedUntil),
			ReadAt:           nullInt64ToTime(row.ReadAt),
			AckedAt:          nullInt64ToTime(row.AckedAt),
		})
	}
	return messages, nil
}

// GetMessageRecipients retrieves all recipients for a message.
func (s *txSqlcStore) GetMessageRecipients(ctx context.Context,
	messageID int64) ([]MessageRecipientWithAgent, error) {

	rows, err := s.queries.GetMessageRecipients(ctx, messageID)
	if err != nil {
		return nil, err
	}

	recipients := make([]MessageRecipientWithAgent, 0, len(rows))
	for _, row := range rows {
		// Fetch agent name within transaction.
		agent, _ := s.queries.GetAgent(ctx, row.AgentID)
		recipients = append(recipients, MessageRecipientWithAgent{
			MessageRecipient: MessageRecipient{
				MessageID:    row.MessageID,
				AgentID:      row.AgentID,
				State:        row.State,
				SnoozedUntil: nullInt64ToTime(row.SnoozedUntil),
				ReadAt:       nullInt64ToTime(row.ReadAt),
				AckedAt:      nullInt64ToTime(row.AckedAt),
			},
			AgentName: agent.Name,
		})
	}
	return recipients, nil
}

// GetMessageRecipientsBulk retrieves recipients for multiple messages.
func (s *txSqlcStore) GetMessageRecipientsBulk(ctx context.Context,
	messageIDs []int64) (map[int64][]MessageRecipientWithAgent, error) {

	rows, err := s.queries.GetMessageRecipientsWithAgentsBulk(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]MessageRecipientWithAgent)
	for _, row := range rows {
		rec := MessageRecipientWithAgent{
			MessageRecipient: MessageRecipient{
				MessageID:    row.MessageID,
				AgentID:      row.AgentID,
				State:        row.State,
				SnoozedUntil: nullInt64ToTime(row.SnoozedUntil),
				ReadAt:       nullInt64ToTime(row.ReadAt),
				AckedAt:      nullInt64ToTime(row.AckedAt),
			},
			AgentName: row.AgentName.String,
		}
		result[row.MessageID] = append(result[row.MessageID], rec)
	}
	return result, nil
}

// SearchMessages performs global search across all messages.
func (s *txSqlcStore) SearchMessages(ctx context.Context, query string,
	limit int) ([]InboxMessage, error) {

	escapedQuery := "%" + query + "%"
	rows, err := s.queries.SearchMessages(ctx, sqlc.SearchMessagesParams{
		Subject: escapedQuery,
		BodyMd:  escapedQuery,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName: row.SenderName.String,
		})
	}
	return messages, nil
}

// GetMessagesByTopic retrieves all messages for a topic.
func (s *txSqlcStore) GetMessagesByTopic(ctx context.Context,
	topicID int64) ([]Message, error) {

	rows, err := s.queries.GetMessagesByTopic(ctx, topicID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, MessageFromSqlc(row))
	}
	return messages, nil
}

// GetSentMessages retrieves messages sent by a specific agent.
func (s *txSqlcStore) GetSentMessages(ctx context.Context, senderID int64,
	limit int) ([]Message, error) {

	rows, err := s.queries.GetSentMessages(ctx, sqlc.GetSentMessagesParams{
		SenderID: senderID,
		Limit:    int64(limit),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		msg := Message{
			ID:              row.ID,
			ThreadID:        row.ThreadID,
			TopicID:         row.TopicID,
			LogOffset:       row.LogOffset,
			SenderID:        row.SenderID,
			Subject:         row.Subject,
			Body:            row.BodyMd,
			Priority:        row.Priority,
			CreatedAt:       time.Unix(row.CreatedAt, 0),
			DeletedBySender: row.DeletedBySender == 1,
		}
		if row.DeadlineAt.Valid {
			t := time.Unix(row.DeadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		if row.Attachments.Valid {
			msg.Attachments = row.Attachments.String
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetAllSentMessages retrieves all sent messages across all agents.
func (s *txSqlcStore) GetAllSentMessages(ctx context.Context,
	limit int) ([]InboxMessage, error) {

	rows, err := s.queries.GetAllSentMessages(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	messages := make([]InboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
		})
	}
	return messages, nil
}

// UpdateAgentName updates an agent's display name.
func (s *txSqlcStore) UpdateAgentName(ctx context.Context, id int64,
	name string) error {

	return s.queries.UpdateAgentName(ctx, sqlc.UpdateAgentNameParams{
		Name: name,
		ID:   id,
	})
}

// =============================================================================
// Helper functions for row conversion
// =============================================================================

// convertInboxRows converts sqlc inbox rows to domain InboxMessage.
func convertInboxRows(rows []sqlc.GetInboxMessagesRow) []InboxMessage {
	messages := make([]InboxMessage, len(rows))
	for i, row := range rows {
		msg := InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
			State:            row.State,
		}
		if row.DeadlineAt.Valid {
			t := time.Unix(row.DeadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		if row.Attachments.Valid {
			msg.Attachments = row.Attachments.String
		}
		if row.SnoozedUntil.Valid {
			t := time.Unix(row.SnoozedUntil.Int64, 0)
			msg.SnoozedUntil = &t
		}
		if row.ReadAt.Valid {
			t := time.Unix(row.ReadAt.Int64, 0)
			msg.ReadAt = &t
		}
		if row.AckedAt.Valid {
			t := time.Unix(row.AckedAt.Int64, 0)
			msg.AckedAt = &t
		}
		messages[i] = msg
	}
	return messages
}

// convertUnreadRows converts sqlc unread rows to domain InboxMessage.
func convertUnreadRows(rows []sqlc.GetUnreadMessagesRow) []InboxMessage {
	messages := make([]InboxMessage, len(rows))
	for i, row := range rows {
		msg := InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			SenderName:       row.SenderName.String,
			SenderProjectKey: row.SenderProjectKey.String,
			SenderGitBranch:  row.SenderGitBranch.String,
			State:            row.State,
		}
		if row.DeadlineAt.Valid {
			t := time.Unix(row.DeadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		if row.Attachments.Valid {
			msg.Attachments = row.Attachments.String
		}
		messages[i] = msg
	}
	return messages
}

// convertArchivedRows converts sqlc archived rows to domain InboxMessage.
// Note: GetArchivedMessages query doesn't include sender_name.
func convertArchivedRows(rows []sqlc.GetArchivedMessagesRow) []InboxMessage {
	messages := make([]InboxMessage, len(rows))
	for i, row := range rows {
		msg := InboxMessage{
			Message: Message{
				ID:        row.ID,
				ThreadID:  row.ThreadID,
				TopicID:   row.TopicID,
				LogOffset: row.LogOffset,
				SenderID:  row.SenderID,
				Subject:   row.Subject,
				Body:      row.BodyMd,
				Priority:  row.Priority,
				CreatedAt: time.Unix(row.CreatedAt, 0),
			},
			State: row.State,
		}
		if row.DeadlineAt.Valid {
			t := time.Unix(row.DeadlineAt.Int64, 0)
			msg.DeadlineAt = &t
		}
		if row.Attachments.Valid {
			msg.Attachments = row.Attachments.String
		}
		if row.AckedAt.Valid {
			t := time.Unix(row.AckedAt.Int64, 0)
			msg.AckedAt = &t
		}
		if row.SnoozedUntil.Valid {
			t := time.Unix(row.SnoozedUntil.Int64, 0)
			msg.SnoozedUntil = &t
		}
		if row.ReadAt.Valid {
			t := time.Unix(row.ReadAt.Int64, 0)
			msg.ReadAt = &t
		}
		messages[i] = msg
	}
	return messages
}
