package subtraterpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RegisterTaskList registers a new task list for an agent.
func (s *Server) RegisterTaskList(
	ctx context.Context, req *RegisterTaskListRequest,
) (*RegisterTaskListResponse, error) {
	if req.ListId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "list_id is required",
		)
	}
	if req.AgentId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "agent_id is required",
		)
	}

	tl, err := s.taskStore.CreateTaskList(
		ctx, store.CreateTaskListParams{
			ListID:    req.ListId,
			AgentID:   req.AgentId,
			WatchPath: req.WatchPath,
		},
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to create task list: %v", err,
		)
	}

	return &RegisterTaskListResponse{
		TaskList: taskListToProto(tl),
	}, nil
}

// GetTaskList retrieves a task list by ID.
func (s *Server) GetTaskList(
	ctx context.Context, req *GetTaskListRequest,
) (*GetTaskListResponse, error) {
	if req.ListId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "list_id is required",
		)
	}

	tl, err := s.taskStore.GetTaskList(ctx, req.ListId)
	if err != nil {
		return nil, status.Errorf(
			codes.NotFound, "task list not found: %v", err,
		)
	}

	return &GetTaskListResponse{
		TaskList: taskListToProto(tl),
	}, nil
}

// ListTaskLists lists registered task lists.
func (s *Server) ListTaskLists(
	ctx context.Context, req *ListTaskListsRequest,
) (*ListTaskListsResponse, error) {
	var (
		lists []store.TaskList
		err   error
	)

	if req.AgentId > 0 {
		lists, err = s.taskStore.ListTaskListsByAgent(
			ctx, req.AgentId,
		)
	} else {
		lists, err = s.taskStore.ListTaskLists(ctx)
	}
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to list task lists: %v", err,
		)
	}

	protos := make([]*TaskListProto, len(lists))
	for i, tl := range lists {
		protos[i] = taskListToProto(tl)
	}

	return &ListTaskListsResponse{TaskLists: protos}, nil
}

// UnregisterTaskList removes a task list registration.
func (s *Server) UnregisterTaskList(
	ctx context.Context, req *UnregisterTaskListRequest,
) (*UnregisterTaskListResponse, error) {
	if req.ListId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "list_id is required",
		)
	}

	// Delete tasks first, then the list.
	if err := s.taskStore.DeleteTasksByList(
		ctx, req.ListId,
	); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to delete tasks for list: %v", err,
		)
	}

	if err := s.taskStore.DeleteTaskList(
		ctx, req.ListId,
	); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to delete task list: %v", err,
		)
	}

	return &UnregisterTaskListResponse{}, nil
}

// UpsertTask creates or updates a task.
func (s *Server) UpsertTask(
	ctx context.Context, req *UpsertTaskRequest,
) (*UpsertTaskResponse, error) {
	if req.ListId == "" || req.ClaudeTaskId == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"list_id and claude_task_id are required",
		)
	}

	blockedByJSON := marshalStringSlice(req.BlockedBy)
	blocksJSON := marshalStringSlice(req.Blocks)

	task, err := s.taskStore.UpsertTask(
		ctx, store.UpsertTaskParams{
			AgentID:      req.AgentId,
			ListID:       req.ListId,
			ClaudeTaskID: req.ClaudeTaskId,
			Subject:      req.Subject,
			Description:  req.Description,
			ActiveForm:   req.ActiveForm,
			Metadata:     req.MetadataJson,
			Status:       taskStatusToString(req.Status),
			Owner:        req.Owner,
			BlockedBy:    blockedByJSON,
			Blocks:       blocksJSON,
		},
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to upsert task: %v", err,
		)
	}

	return &UpsertTaskResponse{Task: taskToProto(task)}, nil
}

// GetTask retrieves a task by Claude task ID.
func (s *Server) GetTask(
	ctx context.Context, req *GetTaskProtoRequest,
) (*GetTaskResponse, error) {
	if req.ListId == "" || req.ClaudeTaskId == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"list_id and claude_task_id are required",
		)
	}

	task, err := s.taskStore.GetTaskByClaudeID(
		ctx, req.ListId, req.ClaudeTaskId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.NotFound, "task not found: %v", err,
		)
	}

	return &GetTaskResponse{Task: taskToProto(task)}, nil
}

// ListTasks lists tasks with optional filters.
func (s *Server) ListTasks(
	ctx context.Context, req *ListTasksRequest,
) (*ListTasksResponse, error) {
	var (
		tasks []store.Task
		err   error
	)

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	switch {
	case req.AvailableOnly && req.AgentId > 0:
		tasks, err = s.taskStore.ListAvailableTasks(
			ctx, req.AgentId,
		)
	case req.ActiveOnly && req.AgentId > 0:
		tasks, err = s.taskStore.ListActiveTasksByAgent(
			ctx, req.AgentId,
		)
	case req.ListId != "":
		tasks, err = s.taskStore.ListTasksByList(
			ctx, req.ListId,
		)
	case req.Status != TaskStatus_TASK_STATUS_UNSPECIFIED &&
		req.AgentId == 0:

		tasks, err = s.taskStore.ListTasksByStatus(
			ctx, taskStatusToString(req.Status), limit, offset,
		)
	case req.AgentId > 0:
		tasks, err = s.taskStore.ListTasksByAgentWithLimit(
			ctx, req.AgentId, limit, offset,
		)
	default:
		tasks, err = s.taskStore.ListAllTasks(
			ctx, limit, offset,
		)
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to list tasks: %v", err,
		)
	}

	protos := make([]*TaskProto, len(tasks))
	for i, t := range tasks {
		protos[i] = taskToProto(t)
	}

	return &ListTasksResponse{Tasks: protos}, nil
}

// UpdateTaskStatus updates a task's status.
func (s *Server) UpdateTaskStatus(
	ctx context.Context, req *UpdateTaskStatusRequest,
) (*UpdateTaskStatusResponse, error) {
	if req.ListId == "" || req.ClaudeTaskId == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"list_id and claude_task_id are required",
		)
	}

	err := s.taskStore.UpdateTaskStatus(
		ctx, req.ListId, req.ClaudeTaskId,
		taskStatusToString(req.Status), time.Now(),
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update task status: %v", err,
		)
	}

	return &UpdateTaskStatusResponse{}, nil
}

// UpdateTaskOwner assigns an owner to a task.
func (s *Server) UpdateTaskOwner(
	ctx context.Context, req *UpdateTaskOwnerRequest,
) (*UpdateTaskOwnerResponse, error) {
	if req.ListId == "" || req.ClaudeTaskId == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"list_id and claude_task_id are required",
		)
	}

	err := s.taskStore.UpdateTaskOwner(
		ctx, req.ListId, req.ClaudeTaskId,
		req.Owner, time.Now(),
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update task owner: %v", err,
		)
	}

	return &UpdateTaskOwnerResponse{}, nil
}

// DeleteTask deletes a task.
func (s *Server) DeleteTask(
	ctx context.Context, req *DeleteTaskRequest,
) (*DeleteTaskResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "id is required",
		)
	}

	err := s.taskStore.DeleteTask(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to delete task: %v", err,
		)
	}

	return &DeleteTaskResponse{}, nil
}

// GetTaskStats retrieves task statistics.
func (s *Server) GetTaskStats(
	ctx context.Context, req *GetTaskStatsRequest,
) (*GetTaskStatsResponse, error) {
	todaySince := todayStart()

	var (
		stats store.TaskStats
		err   error
	)

	switch {
	case req.ListId != "":
		stats, err = s.taskStore.GetTaskStatsByList(
			ctx, req.ListId, todaySince,
		)
	case req.AgentId > 0:
		stats, err = s.taskStore.GetTaskStatsByAgent(
			ctx, req.AgentId, todaySince,
		)
	default:
		stats, err = s.taskStore.GetAllTaskStats(
			ctx, todaySince,
		)
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to get task stats: %v", err,
		)
	}

	return &GetTaskStatsResponse{
		Stats: &TaskStatsProto{
			PendingCount:    stats.PendingCount,
			InProgressCount: stats.InProgressCount,
			CompletedCount:  stats.CompletedCount,
			BlockedCount:    stats.BlockedCount,
			AvailableCount:  stats.AvailableCount,
			CompletedToday:  stats.CompletedToday,
		},
	}, nil
}

// GetAllAgentTaskStats retrieves task statistics grouped by agent.
func (s *Server) GetAllAgentTaskStats(
	ctx context.Context, req *GetAllAgentTaskStatsRequest,
) (*GetAllAgentTaskStatsResponse, error) {
	todaySince := todayStart()

	agentStats, err := s.taskStore.GetAllAgentTaskStats(
		ctx, todaySince,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get agent task stats: %v", err,
		)
	}

	protos := make([]*AgentTaskStatsProto, len(agentStats))
	for i, as := range agentStats {
		protos[i] = &AgentTaskStatsProto{
			AgentId:         as.AgentID,
			PendingCount:    as.PendingCount,
			InProgressCount: as.InProgressCount,
			BlockedCount:    as.BlockedCount,
			CompletedToday:  as.CompletedToday,
		}
	}

	return &GetAllAgentTaskStatsResponse{Stats: protos}, nil
}

// SyncTaskList triggers a sync of a task list.
func (s *Server) SyncTaskList(
	ctx context.Context, req *SyncTaskListRequest,
) (*SyncTaskListResponse, error) {
	if req.ListId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "list_id is required",
		)
	}

	err := s.taskStore.UpdateTaskListSyncTime(
		ctx, req.ListId, time.Now(),
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update sync time: %v", err,
		)
	}

	return &SyncTaskListResponse{}, nil
}

// PruneOldTasks removes old completed/deleted tasks.
func (s *Server) PruneOldTasks(
	ctx context.Context, req *PruneOldTasksRequest,
) (*PruneOldTasksResponse, error) {
	// Default to 7 days if no cutoff specified.
	olderThan := time.Now().Add(-7 * 24 * time.Hour)
	if req.OlderThan != nil {
		olderThan = req.OlderThan.AsTime()
	}

	err := s.taskStore.PruneOldTasks(ctx, olderThan)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to prune old tasks: %v", err,
		)
	}

	return &PruneOldTasksResponse{}, nil
}

// =============================================================================
// Conversion helpers
// =============================================================================

// taskListToProto converts a store TaskList to a proto TaskListProto.
func taskListToProto(tl store.TaskList) *TaskListProto {
	p := &TaskListProto{
		Id:        tl.ID,
		ListId:    tl.ListID,
		AgentId:   tl.AgentID,
		WatchPath: tl.WatchPath,
		CreatedAt: timestamppb.New(tl.CreatedAt),
	}
	if tl.LastSyncedAt != nil {
		p.LastSyncedAt = timestamppb.New(*tl.LastSyncedAt)
	}
	return p
}

// taskToProto converts a store Task to a proto TaskProto.
func taskToProto(t store.Task) *TaskProto {
	p := &TaskProto{
		Id:           t.ID,
		AgentId:      t.AgentID,
		ListId:       t.ListID,
		ClaudeTaskId: t.ClaudeTaskID,
		Subject:      t.Subject,
		Description:  t.Description,
		ActiveForm:   t.ActiveForm,
		MetadataJson: t.Metadata,
		Status:       stringToTaskStatus(t.Status),
		Owner:        t.Owner,
		CreatedAt:    timestamppb.New(t.CreatedAt),
		UpdatedAt:    timestamppb.New(t.UpdatedAt),
	}

	// Parse blocked_by JSON.
	if t.BlockedBy != "" && t.BlockedBy != "[]" {
		var blockedBy []string
		if json.Unmarshal([]byte(t.BlockedBy), &blockedBy) == nil {
			p.BlockedBy = blockedBy
		}
	}

	// Parse blocks JSON.
	if t.Blocks != "" && t.Blocks != "[]" {
		var blocks []string
		if json.Unmarshal([]byte(t.Blocks), &blocks) == nil {
			p.Blocks = blocks
		}
	}

	if t.StartedAt != nil {
		p.StartedAt = timestamppb.New(*t.StartedAt)
	}
	if t.CompletedAt != nil {
		p.CompletedAt = timestamppb.New(*t.CompletedAt)
	}

	return p
}

// taskStatusToString converts a proto TaskStatus to a string.
func taskStatusToString(s TaskStatus) string {
	switch s {
	case TaskStatus_TASK_STATUS_PENDING:
		return "pending"
	case TaskStatus_TASK_STATUS_IN_PROGRESS:
		return "in_progress"
	case TaskStatus_TASK_STATUS_COMPLETED:
		return "completed"
	default:
		return "pending"
	}
}

// stringToTaskStatus converts a string to a proto TaskStatus.
func stringToTaskStatus(s string) TaskStatus {
	switch s {
	case "pending":
		return TaskStatus_TASK_STATUS_PENDING
	case "in_progress":
		return TaskStatus_TASK_STATUS_IN_PROGRESS
	case "completed":
		return TaskStatus_TASK_STATUS_COMPLETED
	default:
		return TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

// marshalStringSlice converts a string slice to JSON.
func marshalStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// todayStart returns the start of today in UTC.
func todayStart() time.Time {
	now := time.Now().UTC()
	return time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0, time.UTC,
	)
}
