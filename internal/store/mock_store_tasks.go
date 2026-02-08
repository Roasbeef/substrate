package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TaskStore implementation for MockStore.

// taskKey generates a unique key for task lookups by list and claude ID.
func taskKey(listID, claudeTaskID string) string {
	return listID + ":" + claudeTaskID
}

// CreateTaskList registers a new task list for watching.
func (m *MockStore) CreateTaskList(
	ctx context.Context, params CreateTaskListParams,
) (TaskList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.taskLists == nil {
		m.taskLists = make(map[string]TaskList)
	}

	if _, exists := m.taskLists[params.ListID]; exists {
		return TaskList{}, fmt.Errorf(
			"task list %s already exists", params.ListID,
		)
	}

	tl := TaskList{
		ID:        m.nextTaskListID,
		ListID:    params.ListID,
		AgentID:   params.AgentID,
		WatchPath: params.WatchPath,
		CreatedAt: time.Now(),
	}
	m.nextTaskListID++
	m.taskLists[params.ListID] = tl

	return tl, nil
}

// GetTaskList retrieves a task list by its list ID.
func (m *MockStore) GetTaskList(
	ctx context.Context, listID string,
) (TaskList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tl, ok := m.taskLists[listID]
	if !ok {
		return TaskList{}, sql.ErrNoRows
	}
	return tl, nil
}

// GetTaskListByID retrieves a task list by its database ID.
func (m *MockStore) GetTaskListByID(
	ctx context.Context, id int64,
) (TaskList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, tl := range m.taskLists {
		if tl.ID == id {
			return tl, nil
		}
	}
	return TaskList{}, sql.ErrNoRows
}

// ListTaskLists lists all registered task lists.
func (m *MockStore) ListTaskLists(
	ctx context.Context,
) ([]TaskList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]TaskList, 0, len(m.taskLists))
	for _, tl := range m.taskLists {
		result = append(result, tl)
	}
	return result, nil
}

// ListTaskListsByAgent lists task lists for a specific agent.
func (m *MockStore) ListTaskListsByAgent(
	ctx context.Context, agentID int64,
) ([]TaskList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []TaskList
	for _, tl := range m.taskLists {
		if tl.AgentID == agentID {
			result = append(result, tl)
		}
	}
	return result, nil
}

// UpdateTaskListSyncTime updates the last sync timestamp.
func (m *MockStore) UpdateTaskListSyncTime(
	ctx context.Context, listID string, syncTime time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tl, ok := m.taskLists[listID]
	if !ok {
		return sql.ErrNoRows
	}
	tl.LastSyncedAt = &syncTime
	m.taskLists[listID] = tl
	return nil
}

// DeleteTaskList removes a task list.
func (m *MockStore) DeleteTaskList(
	ctx context.Context, listID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.taskLists, listID)
	return nil
}

// CreateTask creates a new task.
func (m *MockStore) CreateTask(
	ctx context.Context, params CreateTaskParams,
) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	task := Task{
		ID:           m.nextTaskID,
		AgentID:      params.AgentID,
		ListID:       params.ListID,
		ClaudeTaskID: params.ClaudeTaskID,
		Subject:      params.Subject,
		Description:  params.Description,
		ActiveForm:   params.ActiveForm,
		Metadata:     params.Metadata,
		Status:       params.Status,
		Owner:        params.Owner,
		BlockedBy:    params.BlockedBy,
		Blocks:       params.Blocks,
		CreatedAt:    now,
		UpdatedAt:    now,
		FilePath:     params.FilePath,
		FileMtime:    params.FileMtime,
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.BlockedBy == "" {
		task.BlockedBy = "[]"
	}
	if task.Blocks == "" {
		task.Blocks = "[]"
	}

	m.nextTaskID++

	if m.tasks == nil {
		m.tasks = make(map[int64]Task)
	}
	if m.tasksByKey == nil {
		m.tasksByKey = make(map[string]int64)
	}

	m.tasks[task.ID] = task
	m.tasksByKey[taskKey(params.ListID, params.ClaudeTaskID)] = task.ID

	return task, nil
}

// UpsertTask creates or updates a task.
func (m *MockStore) UpsertTask(
	ctx context.Context, params UpsertTaskParams,
) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tasks == nil {
		m.tasks = make(map[int64]Task)
	}
	if m.tasksByKey == nil {
		m.tasksByKey = make(map[string]int64)
	}

	key := taskKey(params.ListID, params.ClaudeTaskID)
	now := time.Now()

	if id, exists := m.tasksByKey[key]; exists {
		task := m.tasks[id]
		task.Subject = params.Subject
		task.Description = params.Description
		task.ActiveForm = params.ActiveForm
		task.Metadata = params.Metadata
		task.Status = params.Status
		task.Owner = params.Owner
		task.BlockedBy = params.BlockedBy
		task.Blocks = params.Blocks
		task.UpdatedAt = now
		task.StartedAt = params.StartedAt
		task.CompletedAt = params.CompletedAt
		task.FilePath = params.FilePath
		task.FileMtime = params.FileMtime
		m.tasks[id] = task
		return task, nil
	}

	task := Task{
		ID:           m.nextTaskID,
		AgentID:      params.AgentID,
		ListID:       params.ListID,
		ClaudeTaskID: params.ClaudeTaskID,
		Subject:      params.Subject,
		Description:  params.Description,
		ActiveForm:   params.ActiveForm,
		Metadata:     params.Metadata,
		Status:       params.Status,
		Owner:        params.Owner,
		BlockedBy:    params.BlockedBy,
		Blocks:       params.Blocks,
		CreatedAt:    now,
		UpdatedAt:    now,
		StartedAt:    params.StartedAt,
		CompletedAt:  params.CompletedAt,
		FilePath:     params.FilePath,
		FileMtime:    params.FileMtime,
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.BlockedBy == "" {
		task.BlockedBy = "[]"
	}
	if task.Blocks == "" {
		task.Blocks = "[]"
	}

	m.nextTaskID++
	m.tasks[task.ID] = task
	m.tasksByKey[key] = task.ID

	return task, nil
}

// GetTask retrieves a task by its database ID.
func (m *MockStore) GetTask(
	ctx context.Context, id int64,
) (Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return Task{}, sql.ErrNoRows
	}
	return task, nil
}

// GetTaskByClaudeID retrieves a task by its Claude task ID within a list.
func (m *MockStore) GetTaskByClaudeID(
	ctx context.Context, listID, claudeTaskID string,
) (Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := taskKey(listID, claudeTaskID)
	id, ok := m.tasksByKey[key]
	if !ok {
		return Task{}, sql.ErrNoRows
	}
	return m.tasks[id], nil
}

// ListTasksByAgent lists all tasks for an agent.
func (m *MockStore) ListTasksByAgent(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID {
			result = append(result, task)
		}
	}
	return result, nil
}

// ListTasksByAgentWithLimit lists tasks with pagination.
func (m *MockStore) ListTasksByAgentWithLimit(
	ctx context.Context, agentID int64, limit, offset int,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID {
			all = append(all, task)
		}
	}

	if offset >= len(all) {
		return nil, nil
	}
	end := min(offset+limit, len(all))
	return all[offset:end], nil
}

// ListActiveTasksByAgent lists pending and in_progress tasks.
func (m *MockStore) ListActiveTasksByAgent(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID &&
			(task.Status == "pending" || task.Status == "in_progress") {

			result = append(result, task)
		}
	}
	return result, nil
}

// ListTasksByList lists all tasks for a specific list.
func (m *MockStore) ListTasksByList(
	ctx context.Context, listID string,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.ListID == listID {
			result = append(result, task)
		}
	}
	return result, nil
}

// ListInProgressTasks lists tasks currently in progress.
func (m *MockStore) ListInProgressTasks(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID && task.Status == "in_progress" {
			result = append(result, task)
		}
	}
	return result, nil
}

// ListPendingTasks lists pending tasks.
func (m *MockStore) ListPendingTasks(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID && task.Status == "pending" {
			result = append(result, task)
		}
	}
	return result, nil
}

// ListBlockedTasks lists tasks with blockers.
func (m *MockStore) ListBlockedTasks(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID && task.Status == "pending" &&
			task.BlockedBy != "" && task.BlockedBy != "[]" {

			result = append(result, task)
		}
	}
	return result, nil
}

// ListAvailableTasks lists tasks that can be started.
func (m *MockStore) ListAvailableTasks(
	ctx context.Context, agentID int64,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID && task.Status == "pending" &&
			(task.Owner == "") &&
			(task.BlockedBy == "" || task.BlockedBy == "[]") {

			result = append(result, task)
		}
	}
	return result, nil
}

// ListRecentCompletedTasks lists recently completed tasks.
func (m *MockStore) ListRecentCompletedTasks(
	ctx context.Context, agentID int64, since time.Time, limit int,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.AgentID == agentID && task.Status == "completed" &&
			task.CompletedAt != nil &&
			task.CompletedAt.After(since) {

			result = append(result, task)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// ListAllTasks lists all tasks with pagination.
func (m *MockStore) ListAllTasks(
	ctx context.Context, limit, offset int,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Task
	for _, task := range m.tasks {
		all = append(all, task)
	}

	if offset >= len(all) {
		return nil, nil
	}
	end := min(offset+limit, len(all))
	return all[offset:end], nil
}

// ListTasksByStatus lists tasks by status with pagination.
func (m *MockStore) ListTasksByStatus(
	ctx context.Context, status string, limit, offset int,
) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matching []Task
	for _, task := range m.tasks {
		if task.Status == status {
			matching = append(matching, task)
		}
	}

	if offset >= len(matching) {
		return nil, nil
	}
	end := min(offset+limit, len(matching))
	return matching[offset:end], nil
}

// UpdateTaskStatus updates a task's status with timestamp handling.
func (m *MockStore) UpdateTaskStatus(
	ctx context.Context, listID, claudeTaskID, status string,
	now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := taskKey(listID, claudeTaskID)
	id, ok := m.tasksByKey[key]
	if !ok {
		return sql.ErrNoRows
	}

	task := m.tasks[id]
	task.Status = status
	task.UpdatedAt = now

	if status == "in_progress" && task.StartedAt == nil {
		task.StartedAt = &now
	}
	if status == "completed" {
		task.CompletedAt = &now
	}

	m.tasks[id] = task
	return nil
}

// UpdateTaskOwner assigns an owner to a task.
func (m *MockStore) UpdateTaskOwner(
	ctx context.Context, listID, claudeTaskID, owner string,
	now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := taskKey(listID, claudeTaskID)
	id, ok := m.tasksByKey[key]
	if !ok {
		return sql.ErrNoRows
	}

	task := m.tasks[id]
	task.Owner = owner
	task.UpdatedAt = now
	m.tasks[id] = task
	return nil
}

// GetTaskStatsByAgent returns task statistics for an agent.
func (m *MockStore) GetTaskStatsByAgent(
	ctx context.Context, agentID int64, todaySince time.Time,
) (TaskStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats TaskStats
	for _, task := range m.tasks {
		if task.AgentID != agentID {
			continue
		}
		m.accumulateTaskStats(&stats, task, todaySince)
	}
	return stats, nil
}

// GetTaskStatsByList returns task statistics for a list.
func (m *MockStore) GetTaskStatsByList(
	ctx context.Context, listID string, todaySince time.Time,
) (TaskStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats TaskStats
	for _, task := range m.tasks {
		if task.ListID != listID {
			continue
		}
		m.accumulateTaskStats(&stats, task, todaySince)
	}
	return stats, nil
}

// GetAllTaskStats returns global task statistics.
func (m *MockStore) GetAllTaskStats(
	ctx context.Context, todaySince time.Time,
) (TaskStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats TaskStats
	for _, task := range m.tasks {
		m.accumulateTaskStats(&stats, task, todaySince)
	}
	return stats, nil
}

// accumulateTaskStats adds a task's contribution to the stats.
func (m *MockStore) accumulateTaskStats(
	stats *TaskStats, task Task, todaySince time.Time,
) {
	switch task.Status {
	case "pending":
		stats.PendingCount++
		if task.BlockedBy != "" && task.BlockedBy != "[]" {
			stats.BlockedCount++
		} else if task.Owner == "" {
			stats.AvailableCount++
		}
	case "in_progress":
		stats.InProgressCount++
	case "completed":
		stats.CompletedCount++
		if task.CompletedAt != nil &&
			task.CompletedAt.After(todaySince) {

			stats.CompletedToday++
		}
	}
}

// GetAllAgentTaskStats returns task statistics grouped by agent.
func (m *MockStore) GetAllAgentTaskStats(
	ctx context.Context, todaySince time.Time,
) ([]AgentTaskStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentStats := make(map[int64]*AgentTaskStats)
	for _, task := range m.tasks {
		s, ok := agentStats[task.AgentID]
		if !ok {
			s = &AgentTaskStats{AgentID: task.AgentID}
			agentStats[task.AgentID] = s
		}

		switch task.Status {
		case "pending":
			s.PendingCount++
			if task.BlockedBy != "" && task.BlockedBy != "[]" {
				s.BlockedCount++
			}
		case "in_progress":
			s.InProgressCount++
		case "completed":
			if task.CompletedAt != nil &&
				task.CompletedAt.After(todaySince) {

				s.CompletedToday++
			}
		}
	}

	result := make([]AgentTaskStats, 0, len(agentStats))
	for _, s := range agentStats {
		result = append(result, *s)
	}
	return result, nil
}

// CountTasksByList counts tasks in a list.
func (m *MockStore) CountTasksByList(
	ctx context.Context, listID string,
) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, task := range m.tasks {
		if task.ListID == listID {
			count++
		}
	}
	return count, nil
}

// DeleteTask deletes a task by ID.
func (m *MockStore) DeleteTask(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return sql.ErrNoRows
	}

	delete(m.tasksByKey, taskKey(task.ListID, task.ClaudeTaskID))
	delete(m.tasks, id)
	return nil
}

// DeleteTasksByList deletes all tasks in a list.
func (m *MockStore) DeleteTasksByList(
	ctx context.Context, listID string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, task := range m.tasks {
		if task.ListID == listID {
			delete(m.tasksByKey, taskKey(task.ListID, task.ClaudeTaskID))
			delete(m.tasks, id)
		}
	}
	return nil
}

// MarkTasksDeletedByList marks tasks as deleted if not in active list.
func (m *MockStore) MarkTasksDeletedByList(
	ctx context.Context, listID string, activeIDs []string,
	now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeSet := make(map[string]bool, len(activeIDs))
	for _, id := range activeIDs {
		activeSet[id] = true
	}

	for id, task := range m.tasks {
		if task.ListID == listID && !activeSet[task.ClaudeTaskID] {
			task.Status = "deleted"
			task.UpdatedAt = now
			m.tasks[id] = task
		}
	}
	return nil
}

// PruneOldTasks removes old completed/deleted tasks.
func (m *MockStore) PruneOldTasks(
	ctx context.Context, olderThan time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, task := range m.tasks {
		if (task.Status == "completed" || task.Status == "deleted") &&
			task.UpdatedAt.Before(olderThan) {

			delete(m.tasksByKey, taskKey(task.ListID, task.ClaudeTaskID))
			delete(m.tasks, id)
		}
	}
	return nil
}
