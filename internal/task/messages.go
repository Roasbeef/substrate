// Package task provides an actor-based service for managing Claude Code tasks
// that are tracked by the Subtrate system.
package task

import (
	"time"

	"github.com/roasbeef/subtrate/internal/baselib/actor"
)

// TaskServiceKey is the service key for the task service actor.
var TaskServiceKey = actor.NewServiceKey[TaskRequest, TaskResponse](
	"task-service",
)

// TaskRequest is the sealed interface for task service requests.
type TaskRequest interface {
	actor.Message
	isTaskRequest()
}

// TaskResponse is the sealed interface for task service responses.
type TaskResponse interface {
	isTaskResponse()
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
	BlockedBy    []string
	Blocks       []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
}

// TaskList represents a registered task list.
type TaskList struct {
	ID           int64
	ListID       string
	AgentID      int64
	WatchPath    string
	CreatedAt    time.Time
	LastSyncedAt *time.Time
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
	AgentName       string
	PendingCount    int64
	InProgressCount int64
	BlockedCount    int64
	CompletedToday  int64
}

// =============================================================================
// TaskList registration messages
// =============================================================================

// RegisterTaskListRequest registers a new task list for an agent.
type RegisterTaskListRequest struct {
	actor.BaseMessage

	ListID    string
	AgentID   int64
	WatchPath string
}

// MessageType implements actor.Message.
func (RegisterTaskListRequest) MessageType() string { return "RegisterTaskListRequest" }
func (RegisterTaskListRequest) isTaskRequest()      {}

// RegisterTaskListResponse is the response to RegisterTaskListRequest.
type RegisterTaskListResponse struct {
	TaskList TaskList
	Error    error
}

func (RegisterTaskListResponse) isTaskResponse() {}

// GetTaskListRequest retrieves a task list by its ID.
type GetTaskListRequest struct {
	actor.BaseMessage

	ListID string
}

// MessageType implements actor.Message.
func (GetTaskListRequest) MessageType() string { return "GetTaskListRequest" }
func (GetTaskListRequest) isTaskRequest()      {}

// GetTaskListResponse is the response to GetTaskListRequest.
type GetTaskListResponse struct {
	TaskList TaskList
	Error    error
}

func (GetTaskListResponse) isTaskResponse() {}

// ListTaskListsRequest lists all registered task lists.
type ListTaskListsRequest struct {
	actor.BaseMessage

	// AgentID filters by agent if non-zero.
	AgentID int64
}

// MessageType implements actor.Message.
func (ListTaskListsRequest) MessageType() string { return "ListTaskListsRequest" }
func (ListTaskListsRequest) isTaskRequest()      {}

// ListTaskListsResponse is the response to ListTaskListsRequest.
type ListTaskListsResponse struct {
	TaskLists []TaskList
	Error     error
}

func (ListTaskListsResponse) isTaskResponse() {}

// UnregisterTaskListRequest removes a task list registration.
type UnregisterTaskListRequest struct {
	actor.BaseMessage

	ListID string
}

// MessageType implements actor.Message.
func (UnregisterTaskListRequest) MessageType() string { return "UnregisterTaskListRequest" }
func (UnregisterTaskListRequest) isTaskRequest()      {}

// UnregisterTaskListResponse is the response to UnregisterTaskListRequest.
type UnregisterTaskListResponse struct {
	Error error
}

func (UnregisterTaskListResponse) isTaskResponse() {}

// =============================================================================
// Task CRUD messages
// =============================================================================

// UpsertTaskRequest creates or updates a task.
type UpsertTaskRequest struct {
	actor.BaseMessage

	AgentID      int64
	ListID       string
	ClaudeTaskID string
	Subject      string
	Description  string
	ActiveForm   string
	Metadata     string
	Status       string
	Owner        string
	BlockedBy    []string
	Blocks       []string
	FilePath     string
	FileMtime    int64
}

// MessageType implements actor.Message.
func (UpsertTaskRequest) MessageType() string { return "UpsertTaskRequest" }
func (UpsertTaskRequest) isTaskRequest()      {}

// UpsertTaskResponse is the response to UpsertTaskRequest.
type UpsertTaskResponse struct {
	Task  Task
	Error error
}

func (UpsertTaskResponse) isTaskResponse() {}

// GetTaskRequest retrieves a task by Claude task ID.
type GetTaskRequest struct {
	actor.BaseMessage

	ListID       string
	ClaudeTaskID string
}

// MessageType implements actor.Message.
func (GetTaskRequest) MessageType() string { return "GetTaskRequest" }
func (GetTaskRequest) isTaskRequest()      {}

// GetTaskResponse is the response to GetTaskRequest.
type GetTaskResponse struct {
	Task  Task
	Error error
}

func (GetTaskResponse) isTaskResponse() {}

// UpdateTaskStatusRequest updates a task's status.
type UpdateTaskStatusRequest struct {
	actor.BaseMessage

	ListID       string
	ClaudeTaskID string
	Status       string
}

// MessageType implements actor.Message.
func (UpdateTaskStatusRequest) MessageType() string { return "UpdateTaskStatusRequest" }
func (UpdateTaskStatusRequest) isTaskRequest()      {}

// UpdateTaskStatusResponse is the response to UpdateTaskStatusRequest.
type UpdateTaskStatusResponse struct {
	Error error
}

func (UpdateTaskStatusResponse) isTaskResponse() {}

// UpdateTaskOwnerRequest assigns an owner to a task.
type UpdateTaskOwnerRequest struct {
	actor.BaseMessage

	ListID       string
	ClaudeTaskID string
	Owner        string
}

// MessageType implements actor.Message.
func (UpdateTaskOwnerRequest) MessageType() string { return "UpdateTaskOwnerRequest" }
func (UpdateTaskOwnerRequest) isTaskRequest()      {}

// UpdateTaskOwnerResponse is the response to UpdateTaskOwnerRequest.
type UpdateTaskOwnerResponse struct {
	Error error
}

func (UpdateTaskOwnerResponse) isTaskResponse() {}

// DeleteTaskRequest deletes a task.
type DeleteTaskRequest struct {
	actor.BaseMessage

	ID int64
}

// MessageType implements actor.Message.
func (DeleteTaskRequest) MessageType() string { return "DeleteTaskRequest" }
func (DeleteTaskRequest) isTaskRequest()      {}

// DeleteTaskResponse is the response to DeleteTaskRequest.
type DeleteTaskResponse struct {
	Error error
}

func (DeleteTaskResponse) isTaskResponse() {}

// =============================================================================
// Task listing messages
// =============================================================================

// ListTasksRequest lists tasks with optional filters.
type ListTasksRequest struct {
	actor.BaseMessage

	// AgentID filters by agent if non-zero.
	AgentID int64

	// ListID filters by task list if non-empty.
	ListID string

	// Status filters by status if non-empty.
	Status string

	// ActiveOnly returns only pending and in_progress tasks.
	ActiveOnly bool

	// AvailableOnly returns only tasks that can be started.
	AvailableOnly bool

	// Limit is the maximum number of tasks to return.
	Limit int

	// Offset for pagination.
	Offset int
}

// MessageType implements actor.Message.
func (ListTasksRequest) MessageType() string { return "ListTasksRequest" }
func (ListTasksRequest) isTaskRequest()      {}

// ListTasksResponse is the response to ListTasksRequest.
type ListTasksResponse struct {
	Tasks []Task
	Error error
}

func (ListTasksResponse) isTaskResponse() {}

// =============================================================================
// Statistics messages
// =============================================================================

// GetTaskStatsRequest retrieves task statistics.
type GetTaskStatsRequest struct {
	actor.BaseMessage

	// AgentID filters by agent if non-zero.
	AgentID int64

	// ListID filters by list if non-empty.
	ListID string

	// TodaySince is the start of "today" for completed_today count.
	TodaySince time.Time
}

// MessageType implements actor.Message.
func (GetTaskStatsRequest) MessageType() string { return "GetTaskStatsRequest" }
func (GetTaskStatsRequest) isTaskRequest()      {}

// GetTaskStatsResponse is the response to GetTaskStatsRequest.
type GetTaskStatsResponse struct {
	Stats TaskStats
	Error error
}

func (GetTaskStatsResponse) isTaskResponse() {}

// GetAllAgentTaskStatsRequest retrieves task statistics grouped by agent.
type GetAllAgentTaskStatsRequest struct {
	actor.BaseMessage

	TodaySince time.Time
}

// MessageType implements actor.Message.
func (GetAllAgentTaskStatsRequest) MessageType() string { return "GetAllAgentTaskStatsRequest" }
func (GetAllAgentTaskStatsRequest) isTaskRequest()      {}

// GetAllAgentTaskStatsResponse is the response to GetAllAgentTaskStatsRequest.
type GetAllAgentTaskStatsResponse struct {
	Stats []AgentTaskStats
	Error error
}

func (GetAllAgentTaskStatsResponse) isTaskResponse() {}

// =============================================================================
// Sync and cleanup messages
// =============================================================================

// SyncTaskListRequest triggers a sync of a task list from files.
type SyncTaskListRequest struct {
	actor.BaseMessage

	ListID string
}

// MessageType implements actor.Message.
func (SyncTaskListRequest) MessageType() string { return "SyncTaskListRequest" }
func (SyncTaskListRequest) isTaskRequest()      {}

// SyncTaskListResponse is the response to SyncTaskListRequest.
type SyncTaskListResponse struct {
	TasksUpdated int
	TasksDeleted int
	Error        error
}

func (SyncTaskListResponse) isTaskResponse() {}

// PruneOldTasksRequest removes old completed/deleted tasks.
type PruneOldTasksRequest struct {
	actor.BaseMessage

	OlderThan time.Time
}

// MessageType implements actor.Message.
func (PruneOldTasksRequest) MessageType() string { return "PruneOldTasksRequest" }
func (PruneOldTasksRequest) isTaskRequest()      {}

// PruneOldTasksResponse is the response to PruneOldTasksRequest.
type PruneOldTasksResponse struct {
	Error error
}

func (PruneOldTasksResponse) isTaskResponse() {}
