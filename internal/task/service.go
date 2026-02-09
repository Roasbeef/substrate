package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/roasbeef/subtrate/internal/baselib/actor"
	"github.com/roasbeef/subtrate/internal/store"
)

// Service is the task service actor behavior.
type Service struct {
	store store.TaskStore
}

// ServiceConfig holds configuration for the task service.
type ServiceConfig struct {
	// Store is the task store implementation.
	Store store.TaskStore
}

// NewService creates a new task service with the given configuration.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		store: cfg.Store,
	}
}

// Receive implements actor.ActorBehavior by dispatching to type-specific
// handlers.
func (s *Service) Receive(
	ctx context.Context, msg TaskRequest,
) fn.Result[TaskResponse] {
	switch m := msg.(type) {
	// TaskList operations
	case RegisterTaskListRequest:
		resp := s.handleRegisterTaskList(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case GetTaskListRequest:
		resp := s.handleGetTaskList(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case ListTaskListsRequest:
		resp := s.handleListTaskLists(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case UnregisterTaskListRequest:
		resp := s.handleUnregisterTaskList(ctx, m)
		return fn.Ok[TaskResponse](resp)

	// Task CRUD operations
	case UpsertTaskRequest:
		resp := s.handleUpsertTask(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case GetTaskRequest:
		resp := s.handleGetTask(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case UpdateTaskStatusRequest:
		resp := s.handleUpdateTaskStatus(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case UpdateTaskOwnerRequest:
		resp := s.handleUpdateTaskOwner(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case DeleteTaskRequest:
		resp := s.handleDeleteTask(ctx, m)
		return fn.Ok[TaskResponse](resp)

	// Task listing
	case ListTasksRequest:
		resp := s.handleListTasks(ctx, m)
		return fn.Ok[TaskResponse](resp)

	// Statistics
	case GetTaskStatsRequest:
		resp := s.handleGetTaskStats(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case GetAllAgentTaskStatsRequest:
		resp := s.handleGetAllAgentTaskStats(ctx, m)
		return fn.Ok[TaskResponse](resp)

	// Sync and cleanup
	case SyncTaskListRequest:
		resp := s.handleSyncTaskList(ctx, m)
		return fn.Ok[TaskResponse](resp)

	case PruneOldTasksRequest:
		resp := s.handlePruneOldTasks(ctx, m)
		return fn.Ok[TaskResponse](resp)

	default:
		return fn.Err[TaskResponse](fmt.Errorf(
			"unknown message type: %T", msg,
		))
	}
}

// =============================================================================
// TaskList handlers
// =============================================================================

// handleRegisterTaskList processes a RegisterTaskListRequest.
func (s *Service) handleRegisterTaskList(
	ctx context.Context, req RegisterTaskListRequest,
) RegisterTaskListResponse {
	tl, err := s.store.CreateTaskList(ctx, store.CreateTaskListParams{
		ListID:    req.ListID,
		AgentID:   req.AgentID,
		WatchPath: req.WatchPath,
	})
	if err != nil {
		return RegisterTaskListResponse{Error: err}
	}

	return RegisterTaskListResponse{
		TaskList: convertTaskList(tl),
	}
}

// handleGetTaskList processes a GetTaskListRequest.
func (s *Service) handleGetTaskList(
	ctx context.Context, req GetTaskListRequest,
) GetTaskListResponse {
	tl, err := s.store.GetTaskList(ctx, req.ListID)
	if err != nil {
		return GetTaskListResponse{Error: err}
	}

	return GetTaskListResponse{
		TaskList: convertTaskList(tl),
	}
}

// handleListTaskLists processes a ListTaskListsRequest.
func (s *Service) handleListTaskLists(
	ctx context.Context, req ListTaskListsRequest,
) ListTaskListsResponse {
	var lists []store.TaskList
	var err error

	if req.AgentID > 0 {
		lists, err = s.store.ListTaskListsByAgent(ctx, req.AgentID)
	} else {
		lists, err = s.store.ListTaskLists(ctx)
	}

	if err != nil {
		return ListTaskListsResponse{Error: err}
	}

	return ListTaskListsResponse{
		TaskLists: convertTaskLists(lists),
	}
}

// handleUnregisterTaskList processes an UnregisterTaskListRequest.
func (s *Service) handleUnregisterTaskList(
	ctx context.Context, req UnregisterTaskListRequest,
) UnregisterTaskListResponse {
	// Delete all tasks in the list first.
	if err := s.store.DeleteTasksByList(ctx, req.ListID); err != nil {
		return UnregisterTaskListResponse{Error: err}
	}

	// Then delete the list itself.
	err := s.store.DeleteTaskList(ctx, req.ListID)
	return UnregisterTaskListResponse{Error: err}
}

// =============================================================================
// Task CRUD handlers
// =============================================================================

// handleUpsertTask processes an UpsertTaskRequest.
func (s *Service) handleUpsertTask(
	ctx context.Context, req UpsertTaskRequest,
) UpsertTaskResponse {
	blockedByJSON := "[]"
	blocksJSON := "[]"

	if len(req.BlockedBy) > 0 {
		if data, err := json.Marshal(req.BlockedBy); err == nil {
			blockedByJSON = string(data)
		}
	}
	if len(req.Blocks) > 0 {
		if data, err := json.Marshal(req.Blocks); err == nil {
			blocksJSON = string(data)
		}
	}

	task, err := s.store.UpsertTask(ctx, store.UpsertTaskParams{
		AgentID:      req.AgentID,
		ListID:       req.ListID,
		ClaudeTaskID: req.ClaudeTaskID,
		Subject:      req.Subject,
		Description:  req.Description,
		ActiveForm:   req.ActiveForm,
		Metadata:     req.Metadata,
		Status:       req.Status,
		Owner:        req.Owner,
		BlockedBy:    blockedByJSON,
		Blocks:       blocksJSON,
		FilePath:     req.FilePath,
		FileMtime:    req.FileMtime,
	})
	if err != nil {
		return UpsertTaskResponse{Error: err}
	}

	return UpsertTaskResponse{
		Task: convertTask(task),
	}
}

// handleGetTask processes a GetTaskRequest.
func (s *Service) handleGetTask(
	ctx context.Context, req GetTaskRequest,
) GetTaskResponse {
	task, err := s.store.GetTaskByClaudeID(ctx, req.ListID, req.ClaudeTaskID)
	if err != nil {
		return GetTaskResponse{Error: err}
	}

	return GetTaskResponse{
		Task: convertTask(task),
	}
}

// handleUpdateTaskStatus processes an UpdateTaskStatusRequest.
func (s *Service) handleUpdateTaskStatus(
	ctx context.Context, req UpdateTaskStatusRequest,
) UpdateTaskStatusResponse {
	err := s.store.UpdateTaskStatus(
		ctx, req.ListID, req.ClaudeTaskID, req.Status, time.Now(),
	)
	return UpdateTaskStatusResponse{Error: err}
}

// handleUpdateTaskOwner processes an UpdateTaskOwnerRequest.
func (s *Service) handleUpdateTaskOwner(
	ctx context.Context, req UpdateTaskOwnerRequest,
) UpdateTaskOwnerResponse {
	err := s.store.UpdateTaskOwner(
		ctx, req.ListID, req.ClaudeTaskID, req.Owner, time.Now(),
	)
	return UpdateTaskOwnerResponse{Error: err}
}

// handleDeleteTask processes a DeleteTaskRequest.
func (s *Service) handleDeleteTask(
	ctx context.Context, req DeleteTaskRequest,
) DeleteTaskResponse {
	err := s.store.DeleteTask(ctx, req.ID)
	return DeleteTaskResponse{Error: err}
}

// =============================================================================
// Task listing handlers
// =============================================================================

// handleListTasks processes a ListTasksRequest.
func (s *Service) handleListTasks(
	ctx context.Context, req ListTasksRequest,
) ListTasksResponse {
	var tasks []store.Task
	var err error

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	switch {
	case req.AvailableOnly && req.AgentID > 0:
		tasks, err = s.store.ListAvailableTasks(ctx, req.AgentID)

	case req.ActiveOnly && req.AgentID > 0:
		tasks, err = s.store.ListActiveTasksByAgent(ctx, req.AgentID)

	case req.ListID != "":
		tasks, err = s.store.ListTasksByList(ctx, req.ListID)

	case req.Status != "" && req.AgentID == 0:
		tasks, err = s.store.ListTasksByStatus(ctx, req.Status, limit, req.Offset)

	case req.AgentID > 0:
		tasks, err = s.store.ListTasksByAgentWithLimit(
			ctx, req.AgentID, limit, req.Offset,
		)

	default:
		tasks, err = s.store.ListAllTasks(ctx, limit, req.Offset)
	}

	if err != nil {
		return ListTasksResponse{Error: err}
	}

	return ListTasksResponse{
		Tasks: convertTasks(tasks),
	}
}

// =============================================================================
// Statistics handlers
// =============================================================================

// handleGetTaskStats processes a GetTaskStatsRequest.
func (s *Service) handleGetTaskStats(
	ctx context.Context, req GetTaskStatsRequest,
) GetTaskStatsResponse {
	var stats store.TaskStats
	var err error

	switch {
	case req.ListID != "":
		stats, err = s.store.GetTaskStatsByList(ctx, req.ListID, req.TodaySince)

	case req.AgentID > 0:
		stats, err = s.store.GetTaskStatsByAgent(ctx, req.AgentID, req.TodaySince)

	default:
		stats, err = s.store.GetAllTaskStats(ctx, req.TodaySince)
	}

	if err != nil {
		return GetTaskStatsResponse{Error: err}
	}

	return GetTaskStatsResponse{
		Stats: TaskStats{
			PendingCount:    stats.PendingCount,
			InProgressCount: stats.InProgressCount,
			CompletedCount:  stats.CompletedCount,
			BlockedCount:    stats.BlockedCount,
			AvailableCount:  stats.AvailableCount,
			CompletedToday:  stats.CompletedToday,
		},
	}
}

// handleGetAllAgentTaskStats processes a GetAllAgentTaskStatsRequest.
func (s *Service) handleGetAllAgentTaskStats(
	ctx context.Context, req GetAllAgentTaskStatsRequest,
) GetAllAgentTaskStatsResponse {
	stats, err := s.store.GetAllAgentTaskStats(ctx, req.TodaySince)
	if err != nil {
		return GetAllAgentTaskStatsResponse{Error: err}
	}

	result := make([]AgentTaskStats, len(stats))
	for i, s := range stats {
		result[i] = AgentTaskStats{
			AgentID:         s.AgentID,
			PendingCount:    s.PendingCount,
			InProgressCount: s.InProgressCount,
			BlockedCount:    s.BlockedCount,
			CompletedToday:  s.CompletedToday,
		}
	}

	return GetAllAgentTaskStatsResponse{
		Stats: result,
	}
}

// =============================================================================
// Sync and cleanup handlers
// =============================================================================

// handleSyncTaskList processes a SyncTaskListRequest.
// Note: Actual file watching and sync logic would be implemented here or in
// a separate file watcher component.
func (s *Service) handleSyncTaskList(
	ctx context.Context, req SyncTaskListRequest,
) SyncTaskListResponse {
	// Update sync time to indicate sync was requested.
	err := s.store.UpdateTaskListSyncTime(ctx, req.ListID, time.Now())
	if err != nil {
		return SyncTaskListResponse{Error: err}
	}

	// TODO: Implement file sync logic here or delegate to file watcher.
	// For now, just return success indicating sync was triggered.

	return SyncTaskListResponse{
		TasksUpdated: 0,
		TasksDeleted: 0,
	}
}

// handlePruneOldTasks processes a PruneOldTasksRequest.
func (s *Service) handlePruneOldTasks(
	ctx context.Context, req PruneOldTasksRequest,
) PruneOldTasksResponse {
	err := s.store.PruneOldTasks(ctx, req.OlderThan)
	return PruneOldTasksResponse{Error: err}
}

// =============================================================================
// Conversion functions
// =============================================================================

// convertTaskList converts a store TaskList to a service TaskList.
func convertTaskList(tl store.TaskList) TaskList {
	return TaskList{
		ID:           tl.ID,
		ListID:       tl.ListID,
		AgentID:      tl.AgentID,
		WatchPath:    tl.WatchPath,
		CreatedAt:    tl.CreatedAt,
		LastSyncedAt: tl.LastSyncedAt,
	}
}

// convertTaskLists converts a slice of store TaskLists.
func convertTaskLists(lists []store.TaskList) []TaskList {
	result := make([]TaskList, len(lists))
	for i, tl := range lists {
		result[i] = convertTaskList(tl)
	}
	return result
}

// convertTask converts a store Task to a service Task.
func convertTask(t store.Task) Task {
	task := Task{
		ID:           t.ID,
		AgentID:      t.AgentID,
		ListID:       t.ListID,
		ClaudeTaskID: t.ClaudeTaskID,
		Subject:      t.Subject,
		Description:  t.Description,
		ActiveForm:   t.ActiveForm,
		Metadata:     t.Metadata,
		Status:       t.Status,
		Owner:        t.Owner,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
		StartedAt:    t.StartedAt,
		CompletedAt:  t.CompletedAt,
	}

	// Parse blocked_by JSON array.
	if t.BlockedBy != "" && t.BlockedBy != "[]" {
		var blockedBy []string
		if err := json.Unmarshal([]byte(t.BlockedBy), &blockedBy); err == nil {
			task.BlockedBy = blockedBy
		}
	}

	// Parse blocks JSON array.
	if t.Blocks != "" && t.Blocks != "[]" {
		var blocks []string
		if err := json.Unmarshal([]byte(t.Blocks), &blocks); err == nil {
			task.Blocks = blocks
		}
	}

	return task
}

// convertTasks converts a slice of store Tasks.
func convertTasks(tasks []store.Task) []Task {
	result := make([]Task, len(tasks))
	for i, t := range tasks {
		result[i] = convertTask(t)
	}
	return result
}

// =============================================================================
// Actor creation
// =============================================================================

// TaskActorRef is the typed actor reference for the task service.
type TaskActorRef = actor.ActorRef[TaskRequest, TaskResponse]

// NewTaskActor creates a new task actor with the given configuration.
func NewTaskActor(cfg ServiceConfig) *actor.Actor[TaskRequest, TaskResponse] {
	svc := NewService(cfg)
	return actor.NewActor(actor.ActorConfig[TaskRequest, TaskResponse]{
		ID:          "task-service",
		Behavior:    svc,
		MailboxSize: 100,
	})
}

// Ensure Service implements ActorBehavior.
var _ actor.ActorBehavior[TaskRequest, TaskResponse] = (*Service)(nil)
