package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// TaskStore implementation for txSqlcStore (transaction context) using raw SQL
// =============================================================================

// CreateTaskList registers a new task list for watching.
func (s *txSqlcStore) CreateTaskList(ctx context.Context,
	params CreateTaskListParams,
) (TaskList, error) {
	now := time.Now().Unix()

	row := s.sqlDB.QueryRowContext(ctx, `
		INSERT INTO task_lists (list_id, agent_id, watch_path, created_at)
		VALUES (?, ?, ?, ?)
		RETURNING id, list_id, agent_id, watch_path, created_at, last_synced_at
	`, params.ListID, params.AgentID, params.WatchPath, now)

	return scanTaskList(row)
}

// GetTaskList retrieves a task list by its list ID.
func (s *txSqlcStore) GetTaskList(ctx context.Context,
	listID string,
) (TaskList, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT id, list_id, agent_id, watch_path, created_at, last_synced_at
		FROM task_lists WHERE list_id = ?
	`, listID)

	return scanTaskList(row)
}

// GetTaskListByID retrieves a task list by its database ID.
func (s *txSqlcStore) GetTaskListByID(ctx context.Context,
	id int64,
) (TaskList, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT id, list_id, agent_id, watch_path, created_at, last_synced_at
		FROM task_lists WHERE id = ?
	`, id)

	return scanTaskList(row)
}

// ListTaskLists lists all registered task lists.
func (s *txSqlcStore) ListTaskLists(ctx context.Context) ([]TaskList, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT id, list_id, agent_id, watch_path, created_at, last_synced_at
		FROM task_lists
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []TaskList
	for rows.Next() {
		tl, err := scanTaskList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, tl)
	}

	return lists, rows.Err()
}

// ListTaskListsByAgent lists task lists for a specific agent.
func (s *txSqlcStore) ListTaskListsByAgent(ctx context.Context,
	agentID int64,
) ([]TaskList, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT id, list_id, agent_id, watch_path, created_at, last_synced_at
		FROM task_lists
		WHERE agent_id = ?
		ORDER BY created_at DESC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []TaskList
	for rows.Next() {
		tl, err := scanTaskList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, tl)
	}

	return lists, rows.Err()
}

// UpdateTaskListSyncTime updates the last sync timestamp.
func (s *txSqlcStore) UpdateTaskListSyncTime(ctx context.Context,
	listID string, syncTime time.Time,
) error {
	_, err := s.sqlDB.ExecContext(ctx, `
		UPDATE task_lists
		SET last_synced_at = ?
		WHERE list_id = ?
	`, syncTime.Unix(), listID)
	return err
}

// DeleteTaskList removes a task list and its tasks.
func (s *txSqlcStore) DeleteTaskList(ctx context.Context, listID string) error {
	_, err := s.sqlDB.ExecContext(ctx,
		`DELETE FROM task_lists WHERE list_id = ?`, listID,
	)
	return err
}

// CreateTask creates a new task.
func (s *txSqlcStore) CreateTask(ctx context.Context,
	params CreateTaskParams,
) (Task, error) {
	now := time.Now().Unix()

	row := s.sqlDB.QueryRowContext(ctx, `
		INSERT INTO agent_tasks (
			agent_id, list_id, claude_task_id, subject, description,
			active_form, metadata, status, owner, blocked_by, blocks,
			created_at, updated_at, file_path, file_mtime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
	`, params.AgentID, params.ListID, params.ClaudeTaskID, params.Subject,
		ToSqlcNullString(params.Description), ToSqlcNullString(params.ActiveForm),
		ToSqlcNullString(params.Metadata), params.Status,
		ToSqlcNullString(params.Owner), ToSqlcNullString(params.BlockedBy),
		ToSqlcNullString(params.Blocks), now, now,
		ToSqlcNullString(params.FilePath), params.FileMtime)

	return scanTask(row)
}

// UpsertTask creates or updates a task.
func (s *txSqlcStore) UpsertTask(ctx context.Context,
	params UpsertTaskParams,
) (Task, error) {
	now := time.Now().Unix()

	var startedAtVal, completedAtVal sql.NullInt64
	if params.StartedAt != nil {
		startedAtVal = sql.NullInt64{Int64: params.StartedAt.Unix(), Valid: true}
	}
	if params.CompletedAt != nil {
		completedAtVal = sql.NullInt64{
			Int64: params.CompletedAt.Unix(),
			Valid: true,
		}
	}

	row := s.sqlDB.QueryRowContext(ctx, `
		INSERT INTO agent_tasks (
			agent_id, list_id, claude_task_id, subject, description,
			active_form, metadata, status, owner, blocked_by, blocks,
			created_at, updated_at, started_at, completed_at, file_path, file_mtime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(list_id, claude_task_id) DO UPDATE SET
			subject = excluded.subject,
			description = excluded.description,
			active_form = excluded.active_form,
			metadata = excluded.metadata,
			status = excluded.status,
			owner = excluded.owner,
			blocked_by = excluded.blocked_by,
			blocks = excluded.blocks,
			updated_at = excluded.updated_at,
			started_at = CASE
				WHEN excluded.status = 'in_progress' AND agent_tasks.started_at IS NULL
				THEN excluded.updated_at
				ELSE COALESCE(agent_tasks.started_at, excluded.started_at)
			END,
			completed_at = CASE
				WHEN excluded.status = 'completed' AND agent_tasks.completed_at IS NULL
				THEN excluded.updated_at
				ELSE COALESCE(agent_tasks.completed_at, excluded.completed_at)
			END,
			file_path = excluded.file_path,
			file_mtime = excluded.file_mtime
		RETURNING id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
	`, params.AgentID, params.ListID, params.ClaudeTaskID, params.Subject,
		ToSqlcNullString(params.Description), ToSqlcNullString(params.ActiveForm),
		ToSqlcNullString(params.Metadata), params.Status,
		ToSqlcNullString(params.Owner), ToSqlcNullString(params.BlockedBy),
		ToSqlcNullString(params.Blocks), now, now,
		startedAtVal, completedAtVal,
		ToSqlcNullString(params.FilePath), params.FileMtime)

	return scanTask(row)
}

// GetTask retrieves a task by its database ID.
func (s *txSqlcStore) GetTask(ctx context.Context, id int64) (Task, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks WHERE id = ?
	`, id)

	return scanTask(row)
}

// GetTaskByClaudeID retrieves a task by its Claude task ID within a list.
func (s *txSqlcStore) GetTaskByClaudeID(ctx context.Context,
	listID, claudeTaskID string,
) (Task, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks WHERE list_id = ? AND claude_task_id = ?
	`, listID, claudeTaskID)

	return scanTask(row)
}

// txListTasksWithQuery is a helper for task listing queries in transaction.
func (s *txSqlcStore) txListTasksWithQuery(ctx context.Context,
	query string, args ...any,
) ([]Task, error) {
	rows, err := s.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

// ListTasksByAgent lists all tasks for an agent.
func (s *txSqlcStore) ListTasksByAgent(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ?
		ORDER BY
			CASE status
				WHEN 'in_progress' THEN 0
				WHEN 'pending' THEN 1
				ELSE 2
			END,
			updated_at DESC
	`, agentID)
}

// ListTasksByAgentWithLimit lists tasks with pagination.
func (s *txSqlcStore) ListTasksByAgentWithLimit(ctx context.Context,
	agentID int64, limit, offset int,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ?
		ORDER BY
			CASE status
				WHEN 'in_progress' THEN 0
				WHEN 'pending' THEN 1
				ELSE 2
			END,
			updated_at DESC
		LIMIT ? OFFSET ?
	`, agentID, limit, offset)
}

// ListActiveTasksByAgent lists pending and in_progress tasks.
func (s *txSqlcStore) ListActiveTasksByAgent(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ? AND status IN ('pending', 'in_progress')
		ORDER BY
			CASE status WHEN 'in_progress' THEN 0 ELSE 1 END,
			created_at ASC
	`, agentID)
}

// ListTasksByList lists all tasks for a specific list.
func (s *txSqlcStore) ListTasksByList(ctx context.Context,
	listID string,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE list_id = ?
		ORDER BY
			CASE status
				WHEN 'in_progress' THEN 0
				WHEN 'pending' THEN 1
				ELSE 2
			END,
			updated_at DESC
	`, listID)
}

// ListInProgressTasks lists tasks currently in progress.
func (s *txSqlcStore) ListInProgressTasks(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ? AND status = 'in_progress'
		ORDER BY started_at ASC
	`, agentID)
}

// ListPendingTasks lists pending tasks.
func (s *txSqlcStore) ListPendingTasks(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, agentID)
}

// ListBlockedTasks lists tasks with blockers.
func (s *txSqlcStore) ListBlockedTasks(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ?
			AND status = 'pending'
			AND blocked_by != '[]'
		ORDER BY created_at ASC
	`, agentID)
}

// ListAvailableTasks lists tasks that can be started.
func (s *txSqlcStore) ListAvailableTasks(ctx context.Context,
	agentID int64,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ?
			AND status = 'pending'
			AND (owner IS NULL OR owner = '')
			AND (blocked_by IS NULL OR blocked_by = '[]')
		ORDER BY created_at ASC
	`, agentID)
}

// ListRecentCompletedTasks lists recently completed tasks.
func (s *txSqlcStore) ListRecentCompletedTasks(ctx context.Context,
	agentID int64, since time.Time, limit int,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE agent_id = ?
			AND status = 'completed'
			AND completed_at > ?
		ORDER BY completed_at DESC
		LIMIT ?
	`, agentID, since.Unix(), limit)
}

// ListAllTasks lists all tasks with pagination.
func (s *txSqlcStore) ListAllTasks(ctx context.Context,
	limit, offset int,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		ORDER BY
			CASE status
				WHEN 'in_progress' THEN 0
				WHEN 'pending' THEN 1
				ELSE 2
			END,
			updated_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
}

// ListTasksByStatus lists tasks by status with pagination.
func (s *txSqlcStore) ListTasksByStatus(ctx context.Context,
	status string, limit, offset int,
) ([]Task, error) {
	return s.txListTasksWithQuery(ctx, `
		SELECT id, agent_id, list_id, claude_task_id, subject,
			description, active_form, metadata, status, owner,
			blocked_by, blocks, created_at, updated_at,
			started_at, completed_at, file_path, file_mtime
		FROM agent_tasks
		WHERE status = ?
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, status, limit, offset)
}

// UpdateTaskStatus updates a task's status with timestamp handling.
func (s *txSqlcStore) UpdateTaskStatus(ctx context.Context,
	listID, claudeTaskID, status string, now time.Time,
) error {
	nowUnix := now.Unix()

	_, err := s.sqlDB.ExecContext(ctx, `
		UPDATE agent_tasks
		SET status = ?,
			updated_at = ?,
			started_at = CASE
				WHEN ? = 'in_progress' AND started_at IS NULL
				THEN ?
				ELSE started_at
			END,
			completed_at = CASE
				WHEN ? = 'completed'
				THEN ?
				ELSE completed_at
			END
		WHERE list_id = ? AND claude_task_id = ?
	`, status, nowUnix, status, nowUnix, status, nowUnix, listID, claudeTaskID)

	return err
}

// UpdateTaskOwner assigns an owner to a task.
func (s *txSqlcStore) UpdateTaskOwner(ctx context.Context,
	listID, claudeTaskID, owner string, now time.Time,
) error {
	_, err := s.sqlDB.ExecContext(ctx, `
		UPDATE agent_tasks
		SET owner = ?, updated_at = ?
		WHERE list_id = ? AND claude_task_id = ?
	`, ToSqlcNullString(owner), now.Unix(), listID, claudeTaskID)

	return err
}

// GetTaskStatsByAgent returns task statistics for an agent.
func (s *txSqlcStore) GetTaskStatsByAgent(ctx context.Context,
	agentID int64, todaySince time.Time,
) (TaskStats, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND (owner IS NULL OR owner = '') AND (blocked_by IS NULL OR blocked_by = '[]')) as available_count,
			COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
		FROM agent_tasks
		WHERE agent_id = ?
	`, todaySince.Unix(), agentID)

	var stats TaskStats
	err := row.Scan(
		&stats.PendingCount, &stats.InProgressCount, &stats.CompletedCount,
		&stats.BlockedCount, &stats.AvailableCount, &stats.CompletedToday,
	)
	return stats, err
}

// GetTaskStatsByList returns task statistics for a list.
func (s *txSqlcStore) GetTaskStatsByList(ctx context.Context,
	listID string, todaySince time.Time,
) (TaskStats, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND (owner IS NULL OR owner = '') AND (blocked_by IS NULL OR blocked_by = '[]')) as available_count,
			COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
		FROM agent_tasks
		WHERE list_id = ?
	`, todaySince.Unix(), listID)

	var stats TaskStats
	err := row.Scan(
		&stats.PendingCount, &stats.InProgressCount, &stats.CompletedCount,
		&stats.BlockedCount, &stats.AvailableCount, &stats.CompletedToday,
	)
	return stats, err
}

// GetAllTaskStats returns global task statistics.
func (s *txSqlcStore) GetAllTaskStats(ctx context.Context,
	todaySince time.Time,
) (TaskStats, error) {
	row := s.sqlDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
			COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
		FROM agent_tasks
	`, todaySince.Unix())

	var stats TaskStats
	err := row.Scan(
		&stats.PendingCount, &stats.InProgressCount, &stats.CompletedCount,
		&stats.BlockedCount, &stats.CompletedToday,
	)
	return stats, err
}

// GetAllAgentTaskStats returns task statistics grouped by agent.
func (s *txSqlcStore) GetAllAgentTaskStats(ctx context.Context,
	todaySince time.Time,
) ([]AgentTaskStats, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT
			agent_id,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
			COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
			COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
		FROM agent_tasks
		GROUP BY agent_id
	`, todaySince.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statsList []AgentTaskStats
	for rows.Next() {
		var stats AgentTaskStats
		err := rows.Scan(
			&stats.AgentID, &stats.PendingCount, &stats.InProgressCount,
			&stats.BlockedCount, &stats.CompletedToday,
		)
		if err != nil {
			return nil, err
		}
		statsList = append(statsList, stats)
	}

	return statsList, rows.Err()
}

// CountTasksByList counts tasks in a list.
func (s *txSqlcStore) CountTasksByList(ctx context.Context,
	listID string,
) (int64, error) {
	var count int64
	err := s.sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agent_tasks WHERE list_id = ?`, listID,
	).Scan(&count)
	return count, err
}

// DeleteTask deletes a task by ID.
func (s *txSqlcStore) DeleteTask(ctx context.Context, id int64) error {
	_, err := s.sqlDB.ExecContext(ctx, `DELETE FROM agent_tasks WHERE id = ?`, id)
	return err
}

// DeleteTasksByList deletes all tasks in a list.
func (s *txSqlcStore) DeleteTasksByList(ctx context.Context,
	listID string,
) error {
	_, err := s.sqlDB.ExecContext(ctx,
		`DELETE FROM agent_tasks WHERE list_id = ?`, listID,
	)
	return err
}

// MarkTasksDeletedByList marks tasks as deleted if not in active list.
func (s *txSqlcStore) MarkTasksDeletedByList(ctx context.Context,
	listID string, activeIDs []string, now time.Time,
) error {
	if len(activeIDs) == 0 {
		_, err := s.sqlDB.ExecContext(ctx, `
			UPDATE agent_tasks
			SET status = 'deleted', updated_at = ?
			WHERE list_id = ?
				AND status NOT IN ('completed', 'deleted')
		`, now.Unix(), listID)
		return err
	}

	placeholders := make([]string, len(activeIDs))
	args := make([]any, 0, len(activeIDs)+2)
	args = append(args, now.Unix(), listID)
	for i, id := range activeIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		UPDATE agent_tasks
		SET status = 'deleted', updated_at = ?
		WHERE list_id = ?
			AND claude_task_id NOT IN (%s)
			AND status NOT IN ('completed', 'deleted')
	`, strings.Join(placeholders, ","))

	_, err := s.sqlDB.ExecContext(ctx, query, args...)
	return err
}

// PruneOldTasks removes old completed/deleted tasks.
func (s *txSqlcStore) PruneOldTasks(ctx context.Context,
	olderThan time.Time,
) error {
	_, err := s.sqlDB.ExecContext(ctx, `
		DELETE FROM agent_tasks
		WHERE status IN ('completed', 'deleted')
			AND completed_at < ?
	`, olderThan.Unix())
	return err
}
