package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// MessageStore handles message persistence operations.
type MessageStore interface {
	// CreateMessage creates a new message in the database.
	CreateMessage(ctx context.Context, params CreateMessageParams) (Message, error)

	// GetMessage retrieves a message by its ID.
	GetMessage(ctx context.Context, id int64) (Message, error)

	// GetMessagesByThread retrieves all messages in a thread.
	GetMessagesByThread(ctx context.Context, threadID string) ([]Message, error)

	// GetMessagesByThreadWithSender retrieves all messages in a thread with
	// sender information (name, project, branch).
	GetMessagesByThreadWithSender(
		ctx context.Context, threadID string,
	) ([]InboxMessage, error)

	// GetInboxMessages retrieves inbox messages for an agent.
	GetInboxMessages(
		ctx context.Context, agentID int64, limit int,
	) ([]InboxMessage, error)

	// GetUnreadMessages retrieves unread messages for an agent.
	GetUnreadMessages(
		ctx context.Context, agentID int64, limit int,
	) ([]InboxMessage, error)

	// GetArchivedMessages retrieves archived messages for an agent.
	GetArchivedMessages(
		ctx context.Context, agentID int64, limit int,
	) ([]InboxMessage, error)

	// UpdateRecipientState updates the state of a message for a recipient.
	UpdateRecipientState(
		ctx context.Context, messageID, agentID int64, state string,
	) error

	// MarkMessageRead marks a message as read for a recipient.
	MarkMessageRead(ctx context.Context, messageID, agentID int64) error

	// AckMessage acknowledges a message for a recipient.
	AckMessage(ctx context.Context, messageID, agentID int64) error

	// SnoozeMessage snoozes a message until the given time.
	SnoozeMessage(
		ctx context.Context, messageID, agentID int64, until time.Time,
	) error

	// CreateMessageRecipient creates a recipient entry for a message.
	CreateMessageRecipient(
		ctx context.Context, messageID, agentID int64,
	) error

	// GetMessageRecipient retrieves the recipient state for a message.
	GetMessageRecipient(
		ctx context.Context, messageID, agentID int64,
	) (MessageRecipient, error)

	// CountUnreadByAgent counts unread messages for an agent.
	CountUnreadByAgent(ctx context.Context, agentID int64) (int64, error)

	// CountUnreadUrgentByAgent counts urgent unread messages for an agent.
	CountUnreadUrgentByAgent(ctx context.Context, agentID int64) (int64, error)

	// GetMessagesSinceOffset retrieves messages after a given log offset.
	GetMessagesSinceOffset(
		ctx context.Context, topicID, offset int64, limit int,
	) ([]Message, error)

	// NextLogOffset returns the next available log offset for a topic.
	NextLogOffset(ctx context.Context, topicID int64) (int64, error)

	// SearchMessagesForAgent performs full-text search for messages visible to
	// a specific agent.
	SearchMessagesForAgent(
		ctx context.Context, query string, agentID int64, limit int,
	) ([]Message, error)

	// GetAllInboxMessages retrieves inbox messages across all agents (global view).
	GetAllInboxMessages(
		ctx context.Context, limit, offset int,
	) ([]InboxMessage, error)

	// GetMessageRecipients retrieves all recipients for a message.
	GetMessageRecipients(
		ctx context.Context, messageID int64,
	) ([]MessageRecipientWithAgent, error)

	// GetMessageRecipientsBulk retrieves recipients for multiple messages in a
	// single query. Returns a map of message ID to recipients.
	GetMessageRecipientsBulk(
		ctx context.Context, messageIDs []int64,
	) (map[int64][]MessageRecipientWithAgent, error)

	// SearchMessages performs global search across all messages.
	SearchMessages(
		ctx context.Context, query string, limit int,
	) ([]InboxMessage, error)

	// GetMessagesByTopic retrieves all messages for a topic.
	GetMessagesByTopic(ctx context.Context, topicID int64) ([]Message, error)

	// GetSentMessages retrieves messages sent by a specific agent.
	GetSentMessages(
		ctx context.Context, senderID int64, limit int,
	) ([]Message, error)

	// GetAllSentMessages retrieves all sent messages across all agents.
	GetAllSentMessages(ctx context.Context, limit int) ([]InboxMessage, error)

	// GetMessagesBySenderNamePrefix retrieves messages from agents whose name
	// starts with the given prefix (e.g., "reviewer-" for CodeReviewer aggregate).
	GetMessagesBySenderNamePrefix(
		ctx context.Context, prefix string, limit int,
	) ([]InboxMessage, error)

	// GetMessageByIdempotencyKey retrieves a message by its idempotency key.
	// Returns sql.ErrNoRows if no matching message is found.
	GetMessageByIdempotencyKey(
		ctx context.Context, key string,
	) (Message, error)
}

// AgentStore handles agent persistence operations.
type AgentStore interface {
	// CreateAgent creates a new agent in the database.
	CreateAgent(ctx context.Context, params CreateAgentParams) (Agent, error)

	// GetAgent retrieves an agent by its ID.
	GetAgent(ctx context.Context, id int64) (Agent, error)

	// GetAgentByName retrieves an agent by its name.
	GetAgentByName(ctx context.Context, name string) (Agent, error)

	// GetAgentBySessionID retrieves an agent by session ID.
	GetAgentBySessionID(ctx context.Context, sessionID string) (Agent, error)

	// ListAgents lists all agents.
	ListAgents(ctx context.Context) ([]Agent, error)

	// ListAgentsByProject lists agents for a specific project.
	ListAgentsByProject(
		ctx context.Context, projectKey string,
	) ([]Agent, error)

	// UpdateLastActive updates the last active timestamp for an agent.
	UpdateLastActive(ctx context.Context, id int64, ts time.Time) error

	// UpdateSession updates the session ID for an agent.
	UpdateSession(ctx context.Context, id int64, sessionID string) error

	// UpdateAgentName updates an agent's display name.
	UpdateAgentName(ctx context.Context, id int64, name string) error

	// SearchAgents searches agents by name, project_key, or git_branch.
	SearchAgents(
		ctx context.Context, query string, limit int,
	) ([]Agent, error)

	// DeleteAgent deletes an agent by its ID.
	DeleteAgent(ctx context.Context, id int64) error
}

// TopicStore handles topic and subscription persistence.
type TopicStore interface {
	// CreateTopic creates a new topic.
	CreateTopic(ctx context.Context, params CreateTopicParams) (Topic, error)

	// GetTopic retrieves a topic by its ID.
	GetTopic(ctx context.Context, id int64) (Topic, error)

	// GetTopicByName retrieves a topic by its name.
	GetTopicByName(ctx context.Context, name string) (Topic, error)

	// GetOrCreateAgentInboxTopic gets or creates an agent's inbox topic.
	GetOrCreateAgentInboxTopic(
		ctx context.Context, agentName string,
	) (Topic, error)

	// GetOrCreateTopic gets or creates a topic by name.
	GetOrCreateTopic(
		ctx context.Context, name, topicType string,
	) (Topic, error)

	// ListTopics lists all topics.
	ListTopics(ctx context.Context) ([]Topic, error)

	// ListTopicsByType lists topics of a specific type.
	ListTopicsByType(ctx context.Context, topicType string) ([]Topic, error)

	// CreateSubscription subscribes an agent to a topic.
	CreateSubscription(
		ctx context.Context, agentID, topicID int64,
	) error

	// DeleteSubscription unsubscribes an agent from a topic.
	DeleteSubscription(ctx context.Context, agentID, topicID int64) error

	// ListSubscriptionsByAgent lists topics an agent is subscribed to.
	ListSubscriptionsByAgent(ctx context.Context, agentID int64) ([]Topic, error)

	// ListSubscriptionsByTopic lists agents subscribed to a topic.
	ListSubscriptionsByTopic(ctx context.Context, topicID int64) ([]Agent, error)
}

// ActivityStore handles activity feed persistence.
type ActivityStore interface {
	// CreateActivity records a new activity event.
	CreateActivity(ctx context.Context, params CreateActivityParams) error

	// ListRecentActivities lists the most recent activities.
	ListRecentActivities(ctx context.Context, limit int) ([]Activity, error)

	// ListActivitiesByAgent lists activities for a specific agent.
	ListActivitiesByAgent(
		ctx context.Context, agentID int64, limit int,
	) ([]Activity, error)

	// ListActivitiesSince lists activities since a given timestamp.
	ListActivitiesSince(
		ctx context.Context, since time.Time, limit int,
	) ([]Activity, error)

	// DeleteOldActivities removes activities older than a given time.
	DeleteOldActivities(ctx context.Context, olderThan time.Time) error
}

// SessionStore handles session identity persistence.
type SessionStore interface {
	// CreateSessionIdentity creates a new session identity mapping.
	CreateSessionIdentity(
		ctx context.Context, params CreateSessionIdentityParams,
	) error

	// GetSessionIdentity retrieves a session identity by session ID.
	GetSessionIdentity(
		ctx context.Context, sessionID string,
	) (SessionIdentity, error)

	// DeleteSessionIdentity removes a session identity.
	DeleteSessionIdentity(ctx context.Context, sessionID string) error

	// ListSessionIdentitiesByAgent lists session identities for an agent.
	ListSessionIdentitiesByAgent(
		ctx context.Context, agentID int64,
	) ([]SessionIdentity, error)

	// UpdateSessionIdentityLastActive updates the last active timestamp.
	UpdateSessionIdentityLastActive(
		ctx context.Context, sessionID string, ts time.Time,
	) error
}

// TaskStore provides task tracking operations for Claude Code tasks.
type TaskStore interface {
	// CreateTaskList registers a new task list for watching.
	CreateTaskList(
		ctx context.Context, params CreateTaskListParams,
	) (TaskList, error)

	// GetTaskList retrieves a task list by its list ID.
	GetTaskList(ctx context.Context, listID string) (TaskList, error)

	// GetTaskListByID retrieves a task list by its database ID.
	GetTaskListByID(ctx context.Context, id int64) (TaskList, error)

	// ListTaskLists lists all registered task lists.
	ListTaskLists(ctx context.Context) ([]TaskList, error)

	// ListTaskListsByAgent lists task lists for a specific agent.
	ListTaskListsByAgent(ctx context.Context, agentID int64) ([]TaskList, error)

	// UpdateTaskListSyncTime updates the last sync timestamp.
	UpdateTaskListSyncTime(
		ctx context.Context, listID string, syncTime time.Time,
	) error

	// DeleteTaskList removes a task list and its tasks.
	DeleteTaskList(ctx context.Context, listID string) error

	// CreateTask creates a new task.
	CreateTask(ctx context.Context, params CreateTaskParams) (Task, error)

	// UpsertTask creates or updates a task.
	UpsertTask(ctx context.Context, params UpsertTaskParams) (Task, error)

	// GetTask retrieves a task by its database ID.
	GetTask(ctx context.Context, id int64) (Task, error)

	// GetTaskByClaudeID retrieves a task by its Claude task ID within a list.
	GetTaskByClaudeID(
		ctx context.Context, listID, claudeTaskID string,
	) (Task, error)

	// ListTasksByAgent lists all tasks for an agent.
	ListTasksByAgent(ctx context.Context, agentID int64) ([]Task, error)

	// ListTasksByAgentWithLimit lists tasks with pagination.
	ListTasksByAgentWithLimit(
		ctx context.Context, agentID int64, limit, offset int,
	) ([]Task, error)

	// ListActiveTasksByAgent lists pending and in_progress tasks.
	ListActiveTasksByAgent(ctx context.Context, agentID int64) ([]Task, error)

	// ListTasksByList lists all tasks for a specific list.
	ListTasksByList(ctx context.Context, listID string) ([]Task, error)

	// ListInProgressTasks lists tasks currently in progress.
	ListInProgressTasks(ctx context.Context, agentID int64) ([]Task, error)

	// ListPendingTasks lists pending tasks.
	ListPendingTasks(ctx context.Context, agentID int64) ([]Task, error)

	// ListBlockedTasks lists tasks with blockers.
	ListBlockedTasks(ctx context.Context, agentID int64) ([]Task, error)

	// ListAvailableTasks lists tasks that can be started (pending, no owner,
	// no blockers).
	ListAvailableTasks(ctx context.Context, agentID int64) ([]Task, error)

	// ListRecentCompletedTasks lists recently completed tasks.
	ListRecentCompletedTasks(
		ctx context.Context, agentID int64, since time.Time, limit int,
	) ([]Task, error)

	// ListAllTasks lists all tasks with pagination.
	ListAllTasks(ctx context.Context, limit, offset int) ([]Task, error)

	// ListTasksByStatus lists tasks by status with pagination.
	ListTasksByStatus(
		ctx context.Context, status string, limit, offset int,
	) ([]Task, error)

	// UpdateTaskStatus updates a task's status with timestamp handling.
	UpdateTaskStatus(
		ctx context.Context, listID, claudeTaskID, status string,
		now time.Time,
	) error

	// UpdateTaskOwner assigns an owner to a task.
	UpdateTaskOwner(
		ctx context.Context, listID, claudeTaskID, owner string,
		now time.Time,
	) error

	// GetTaskStatsByAgent returns task statistics for an agent.
	GetTaskStatsByAgent(
		ctx context.Context, agentID int64, todaySince time.Time,
	) (TaskStats, error)

	// GetTaskStatsByList returns task statistics for a list.
	GetTaskStatsByList(
		ctx context.Context, listID string, todaySince time.Time,
	) (TaskStats, error)

	// GetAllTaskStats returns global task statistics.
	GetAllTaskStats(ctx context.Context, todaySince time.Time) (TaskStats, error)

	// GetAllAgentTaskStats returns task statistics grouped by agent.
	GetAllAgentTaskStats(
		ctx context.Context, todaySince time.Time,
	) ([]AgentTaskStats, error)

	// CountTasksByList counts tasks in a list.
	CountTasksByList(ctx context.Context, listID string) (int64, error)

	// DeleteTask deletes a task by ID.
	DeleteTask(ctx context.Context, id int64) error

	// DeleteTasksByList deletes all tasks in a list.
	DeleteTasksByList(ctx context.Context, listID string) error

	// MarkTasksDeletedByList marks tasks as deleted if not in active list.
	MarkTasksDeletedByList(
		ctx context.Context, listID string, activeIDs []string, now time.Time,
	) error

	// PruneOldTasks removes old completed/deleted tasks.
	PruneOldTasks(ctx context.Context, olderThan time.Time) error
}

// ReviewStore provides review CRUD operations.
type ReviewStore interface {
	// CreateReview creates a new review record.
	CreateReview(
		ctx context.Context, params CreateReviewParams,
	) (Review, error)

	// GetReview retrieves a review by its UUID.
	GetReview(ctx context.Context, reviewID string) (Review, error)

	// ListReviews lists reviews ordered by creation time.
	ListReviews(
		ctx context.Context, limit, offset int,
	) ([]Review, error)

	// ListReviewsByState lists reviews matching the given state.
	ListReviewsByState(
		ctx context.Context, state string, limit int,
	) ([]Review, error)

	// ListReviewsByRequester lists reviews by the requesting agent.
	ListReviewsByRequester(
		ctx context.Context, requesterID int64, limit int,
	) ([]Review, error)

	// ListActiveReviews returns reviews in non-terminal states (for
	// restart recovery).
	ListActiveReviews(ctx context.Context) ([]Review, error)

	// UpdateReviewState updates the FSM state of a review.
	UpdateReviewState(
		ctx context.Context, reviewID, state string,
	) error

	// UpdateReviewCompleted marks a review as completed with a terminal
	// state.
	UpdateReviewCompleted(
		ctx context.Context, reviewID, state string,
	) error

	// CreateReviewIteration records a review iteration result.
	CreateReviewIteration(
		ctx context.Context, params CreateReviewIterationParams,
	) (ReviewIteration, error)

	// GetReviewIterations gets all iterations for a review.
	GetReviewIterations(
		ctx context.Context, reviewID string,
	) ([]ReviewIteration, error)

	// CreateReviewIssue records a specific issue found during review.
	CreateReviewIssue(
		ctx context.Context, params CreateReviewIssueParams,
	) (ReviewIssue, error)

	// GetReviewIssues gets all issues for a review.
	GetReviewIssues(
		ctx context.Context, reviewID string,
	) ([]ReviewIssue, error)

	// GetOpenReviewIssues gets open issues for a review.
	GetOpenReviewIssues(
		ctx context.Context, reviewID string,
	) ([]ReviewIssue, error)

	// UpdateReviewIssueStatus updates an issue's resolution status.
	UpdateReviewIssueStatus(
		ctx context.Context, issueID int64, status string,
		resolvedInIteration *int,
	) error

	// CountOpenIssues counts open issues for a review.
	CountOpenIssues(ctx context.Context, reviewID string) (int64, error)

	// DeleteReview deletes a review and its associated iterations and
	// issues.
	DeleteReview(ctx context.Context, reviewID string) error
}

// Storage combines all store interfaces for unified access.
type Storage interface {
	MessageStore
	AgentStore
	TopicStore
	ActivityStore
	SessionStore
	TaskStore
	ReviewStore

	// WithTx executes a function within a write database transaction.
	WithTx(ctx context.Context, fn func(ctx context.Context, s Storage) error) error

	// WithReadTx executes a function within a read-only database transaction.
	// This ensures consistent snapshot reads across multiple queries.
	WithReadTx(ctx context.Context, fn func(ctx context.Context, s Storage) error) error

	// Close closes the store and releases resources.
	Close() error
}

// Domain model types that abstract sqlc models.

// Message represents a mail message.
type Message struct {
	ID              int64
	ThreadID        string
	TopicID         int64
	LogOffset       int64
	SenderID        int64
	Subject         string
	Body            string
	Priority        string
	DeadlineAt      *time.Time
	Attachments     string
	CreatedAt       time.Time
	DeletedBySender bool
	IdempotencyKey  string
}

// MessageRecipient represents a message recipient's state.
type MessageRecipient struct {
	MessageID    int64
	AgentID      int64
	State        string
	SnoozedUntil *time.Time
	ReadAt       *time.Time
	AckedAt      *time.Time
}

// MessageRecipientWithAgent extends MessageRecipient with agent name.
type MessageRecipientWithAgent struct {
	MessageRecipient
	AgentName string
}

// InboxMessage represents a message in an agent's inbox with metadata.
type InboxMessage struct {
	Message
	SenderName       string
	SenderProjectKey string
	SenderGitBranch  string
	State            string
	SnoozedUntil     *time.Time
	ReadAt           *time.Time
	AckedAt          *time.Time
}

// Agent represents an agent in the system.
type Agent struct {
	ID               int64
	Name             string
	ProjectKey       string
	GitBranch        string
	CurrentSessionID string
	CreatedAt        time.Time
	LastActiveAt     time.Time
}

// Topic represents a message topic for pub/sub.
type Topic struct {
	ID               int64
	Name             string
	TopicType        string
	RetentionSeconds int64
	CreatedAt        time.Time
	MessageCount     int64
}

// Activity represents an activity event.
type Activity struct {
	ID           int64
	AgentID      int64
	ActivityType string
	Description  string
	Metadata     string
	CreatedAt    time.Time
}

// SessionIdentity maps a session ID to an agent.
type SessionIdentity struct {
	SessionID    string
	AgentID      int64
	ProjectKey   string
	GitBranch    string
	CreatedAt    time.Time
	LastActiveAt time.Time
}

// TaskList represents a registered task list for file watching.
type TaskList struct {
	ID           int64
	ListID       string
	AgentID      int64
	WatchPath    string
	CreatedAt    time.Time
	LastSyncedAt *time.Time
}

// Task represents a Claude Code task.
type Task struct {
	ID           int64
	AgentID      int64
	ListID       string
	ClaudeTaskID string
	Subject      string
	Description  string
	ActiveForm   string
	Metadata     string
	Status       string
	Owner        string
	BlockedBy    string // JSON array of task IDs
	Blocks       string // JSON array of task IDs
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	FilePath     string
	FileMtime    int64
}

// TaskStats contains aggregate task statistics.
type TaskStats struct {
	PendingCount    int64
	InProgressCount int64
	CompletedCount  int64
	BlockedCount    int64
	AvailableCount  int64
	CompletedToday  int64
}

// AgentTaskStats contains task statistics for a specific agent.
type AgentTaskStats struct {
	AgentID         int64
	PendingCount    int64
	InProgressCount int64
	BlockedCount    int64
	CompletedToday  int64
}

// Parameter types for create operations.

// CreateMessageParams contains parameters for creating a message.
type CreateMessageParams struct {
	ThreadID       string
	TopicID        int64
	LogOffset      int64
	SenderID       int64
	Subject        string
	Body           string
	Priority       string
	DeadlineAt     *time.Time
	Attachments    string
	IdempotencyKey string
}

// CreateAgentParams contains parameters for creating an agent.
type CreateAgentParams struct {
	Name       string
	ProjectKey string
	GitBranch  string
}

// CreateTopicParams contains parameters for creating a topic.
type CreateTopicParams struct {
	Name             string
	TopicType        string
	RetentionSeconds int64
}

// CreateActivityParams contains parameters for creating an activity.
type CreateActivityParams struct {
	AgentID      int64
	ActivityType string
	Description  string
	Metadata     string
}

// CreateSessionIdentityParams contains parameters for creating a session
// identity.
type CreateSessionIdentityParams struct {
	SessionID  string
	AgentID    int64
	ProjectKey string
	GitBranch  string
}

// CreateTaskListParams contains parameters for registering a task list.
type CreateTaskListParams struct {
	ListID    string
	AgentID   int64
	WatchPath string
}

// CreateTaskParams contains parameters for creating a task.
type CreateTaskParams struct {
	AgentID      int64
	ListID       string
	ClaudeTaskID string
	Subject      string
	Description  string
	ActiveForm   string
	Metadata     string
	Status       string
	Owner        string
	BlockedBy    string
	Blocks       string
	FilePath     string
	FileMtime    int64
}

// UpsertTaskParams contains parameters for upserting a task.
type UpsertTaskParams struct {
	AgentID      int64
	ListID       string
	ClaudeTaskID string
	Subject      string
	Description  string
	ActiveForm   string
	Metadata     string
	Status       string
	Owner        string
	BlockedBy    string
	Blocks       string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	FilePath     string
	FileMtime    int64
}

// Conversion functions from sqlc models.

// MessageFromSqlc converts a sqlc.Message to a store.Message.
func MessageFromSqlc(m sqlc.Message) Message {
	msg := Message{
		ID:              m.ID,
		ThreadID:        m.ThreadID,
		TopicID:         m.TopicID,
		LogOffset:       m.LogOffset,
		SenderID:        m.SenderID,
		Subject:         m.Subject,
		Body:            m.BodyMd,
		Priority:        m.Priority,
		CreatedAt:       time.Unix(m.CreatedAt, 0),
		DeletedBySender: m.DeletedBySender == 1,
	}
	if m.DeadlineAt.Valid {
		t := time.Unix(m.DeadlineAt.Int64, 0)
		msg.DeadlineAt = &t
	}
	if m.Attachments.Valid {
		msg.Attachments = m.Attachments.String
	}
	if m.IdempotencyKey.Valid {
		msg.IdempotencyKey = m.IdempotencyKey.String
	}
	return msg
}

// AgentFromSqlc converts a sqlc.Agent to a store.Agent.
func AgentFromSqlc(a sqlc.Agent) Agent {
	return Agent{
		ID:               a.ID,
		Name:             a.Name,
		ProjectKey:       a.ProjectKey.String,
		GitBranch:        a.GitBranch.String,
		CurrentSessionID: a.CurrentSessionID.String,
		CreatedAt:        time.Unix(a.CreatedAt, 0),
		LastActiveAt:     time.Unix(a.LastActiveAt, 0),
	}
}

// TopicFromSqlc converts a sqlc.Topic to a store.Topic.
func TopicFromSqlc(t sqlc.Topic) Topic {
	return Topic{
		ID:               t.ID,
		Name:             t.Name,
		TopicType:        t.TopicType,
		RetentionSeconds: t.RetentionSeconds.Int64,
		CreatedAt:        time.Unix(t.CreatedAt, 0),
	}
}

// TopicWithCountFromSqlc converts a sqlc.ListTopicsWithMessageCountRow to a
// store.Topic.
func TopicWithCountFromSqlc(t sqlc.ListTopicsWithMessageCountRow) Topic {
	return Topic{
		ID:               t.ID,
		Name:             t.Name,
		TopicType:        t.TopicType,
		RetentionSeconds: t.RetentionSeconds.Int64,
		CreatedAt:        time.Unix(t.CreatedAt, 0),
		MessageCount:     t.MessageCount,
	}
}

// ActivityFromSqlc converts a sqlc.Activity to a store.Activity.
func ActivityFromSqlc(a sqlc.Activity) Activity {
	return Activity{
		ID:           a.ID,
		AgentID:      a.AgentID,
		ActivityType: a.ActivityType,
		Description:  a.Description,
		Metadata:     a.Metadata.String,
		CreatedAt:    time.Unix(a.CreatedAt, 0),
	}
}

// SessionIdentityFromSqlc converts a sqlc.SessionIdentity to a store model.
func SessionIdentityFromSqlc(s sqlc.SessionIdentity) SessionIdentity {
	return SessionIdentity{
		SessionID:    s.SessionID,
		AgentID:      s.AgentID,
		ProjectKey:   s.ProjectKey.String,
		GitBranch:    s.GitBranch.String,
		CreatedAt:    time.Unix(s.CreatedAt, 0),
		LastActiveAt: time.Unix(s.LastActiveAt, 0),
	}
}

// MessageRecipientFromSqlc converts a sqlc.MessageRecipient to a store model.
func MessageRecipientFromSqlc(r sqlc.MessageRecipient) MessageRecipient {
	mr := MessageRecipient{
		MessageID: r.MessageID,
		AgentID:   r.AgentID,
		State:     r.State,
	}
	if r.SnoozedUntil.Valid {
		t := time.Unix(r.SnoozedUntil.Int64, 0)
		mr.SnoozedUntil = &t
	}
	if r.ReadAt.Valid {
		t := time.Unix(r.ReadAt.Int64, 0)
		mr.ReadAt = &t
	}
	if r.AckedAt.Valid {
		t := time.Unix(r.AckedAt.Int64, 0)
		mr.AckedAt = &t
	}
	return mr
}

// Review represents a code review record.
type Review struct {
	ID          int64
	ReviewID    string
	ThreadID    string
	RequesterID int64
	PRNumber    int
	Branch      string
	BaseBranch  string
	CommitSHA   string
	RepoPath    string
	RemoteURL   string
	ReviewType  string
	Priority    string
	State       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// ReviewIteration represents a single round of review.
type ReviewIteration struct {
	ID                int64
	ReviewID          string
	IterationNum      int
	ReviewerID        string
	ReviewerSessionID string
	Decision          string
	Summary           string
	IssuesJSON        string
	SuggestionsJSON   string
	FilesReviewed     int
	LinesAnalyzed     int
	DurationMS        int64
	CostUSD           float64
	StartedAt         time.Time
	CompletedAt       *time.Time
}

// ReviewIssue represents a specific issue found during review.
type ReviewIssue struct {
	ID                  int64
	ReviewID            string
	IterationNum        int
	IssueType           string
	Severity            string
	FilePath            string
	LineStart           int
	LineEnd             int
	Title               string
	Description         string
	CodeSnippet         string
	Suggestion          string
	ClaudeMDRef         string
	Status              string
	ResolvedAt          *time.Time
	ResolvedInIteration *int
	CreatedAt           time.Time
}

// CreateReviewParams contains parameters for creating a review.
type CreateReviewParams struct {
	ReviewID    string
	ThreadID    string
	RequesterID int64
	PRNumber    int
	Branch      string
	BaseBranch  string
	CommitSHA   string
	RepoPath    string
	RemoteURL   string
	ReviewType  string
	Priority    string
}

// CreateReviewIterationParams contains parameters for creating a review
// iteration.
type CreateReviewIterationParams struct {
	ReviewID          string
	IterationNum      int
	ReviewerID        string
	ReviewerSessionID string
	Decision          string
	Summary           string
	IssuesJSON        string
	SuggestionsJSON   string
	FilesReviewed     int
	LinesAnalyzed     int
	DurationMS        int64
	CostUSD           float64
	StartedAt         time.Time
	CompletedAt       *time.Time
}

// CreateReviewIssueParams contains parameters for creating a review issue.
type CreateReviewIssueParams struct {
	ReviewID     string
	IterationNum int
	IssueType    string
	Severity     string
	FilePath     string
	LineStart    int
	LineEnd      int
	Title        string
	Description  string
	CodeSnippet  string
	Suggestion   string
	ClaudeMDRef  string
}

// ReviewFromSqlc converts a sqlc.Review to a store.Review.
func ReviewFromSqlc(r sqlc.Review) Review {
	rev := Review{
		ID:          r.ID,
		ReviewID:    r.ReviewID,
		ThreadID:    r.ThreadID,
		RequesterID: r.RequesterID,
		Branch:      r.Branch,
		BaseBranch:  r.BaseBranch,
		CommitSHA:   r.CommitSha,
		RepoPath:    r.RepoPath,
		RemoteURL:   r.RemoteUrl.String,
		ReviewType:  r.ReviewType,
		Priority:    r.Priority,
		State:       r.State,
		CreatedAt:   time.Unix(r.CreatedAt, 0),
		UpdatedAt:   time.Unix(r.UpdatedAt, 0),
	}
	if r.PrNumber.Valid {
		rev.PRNumber = int(r.PrNumber.Int64)
	}
	if r.CompletedAt.Valid {
		t := time.Unix(r.CompletedAt.Int64, 0)
		rev.CompletedAt = &t
	}
	return rev
}

// ReviewIterationFromSqlc converts a sqlc.ReviewIteration to a store model.
func ReviewIterationFromSqlc(ri sqlc.ReviewIteration) ReviewIteration {
	iter := ReviewIteration{
		ID:                ri.ID,
		ReviewID:          ri.ReviewID,
		IterationNum:      int(ri.IterationNum),
		ReviewerID:        ri.ReviewerID,
		ReviewerSessionID: ri.ReviewerSessionID.String,
		Decision:          ri.Decision,
		Summary:           ri.Summary,
		IssuesJSON:        ri.IssuesJson.String,
		SuggestionsJSON:   ri.SuggestionsJson.String,
		FilesReviewed:     int(ri.FilesReviewed),
		LinesAnalyzed:     int(ri.LinesAnalyzed),
		DurationMS:        ri.DurationMs,
		CostUSD:           ri.CostUsd,
		StartedAt:         time.Unix(ri.StartedAt, 0),
	}
	if ri.CompletedAt.Valid {
		t := time.Unix(ri.CompletedAt.Int64, 0)
		iter.CompletedAt = &t
	}
	return iter
}

// ReviewIssueFromSqlc converts a sqlc.ReviewIssue to a store model.
func ReviewIssueFromSqlc(ri sqlc.ReviewIssue) ReviewIssue {
	issue := ReviewIssue{
		ID:           ri.ID,
		ReviewID:     ri.ReviewID,
		IterationNum: int(ri.IterationNum),
		IssueType:    ri.IssueType,
		Severity:     ri.Severity,
		FilePath:     ri.FilePath,
		LineStart:    int(ri.LineStart),
		Title:        ri.Title,
		Description:  ri.Description,
		CodeSnippet:  ri.CodeSnippet.String,
		Suggestion:   ri.Suggestion.String,
		ClaudeMDRef:  ri.ClaudeMdRef.String,
		Status:       ri.Status,
		CreatedAt:    time.Unix(ri.CreatedAt, 0),
	}
	if ri.LineEnd.Valid {
		issue.LineEnd = int(ri.LineEnd.Int64)
	}
	if ri.ResolvedAt.Valid {
		t := time.Unix(ri.ResolvedAt.Int64, 0)
		issue.ResolvedAt = &t
	}
	if ri.ResolvedInIteration.Valid {
		v := int(ri.ResolvedInIteration.Int64)
		issue.ResolvedInIteration = &v
	}
	return issue
}

// ToSqlcNullString converts a string to sql.NullString.
func ToSqlcNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ToSqlcNullInt64 converts an int64 pointer to sql.NullInt64.
func ToSqlcNullInt64(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// ToSqlcNullInt64FromInt converts an int pointer to sql.NullInt64.
func ToSqlcNullInt64FromInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// ToSqlcNullInt64Val converts an int to sql.NullInt64.
func ToSqlcNullInt64Val(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}
