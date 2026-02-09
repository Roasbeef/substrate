-- Task list registration queries

-- name: CreateTaskList :one
INSERT INTO task_lists (list_id, agent_id, watch_path, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetTaskList :one
SELECT * FROM task_lists WHERE list_id = ?;

-- name: GetTaskListByID :one
SELECT * FROM task_lists WHERE id = ?;

-- name: ListTaskLists :many
SELECT * FROM task_lists
ORDER BY created_at DESC;

-- name: ListTaskListsByAgent :many
SELECT * FROM task_lists
WHERE agent_id = ?
ORDER BY created_at DESC;

-- name: UpdateTaskListSyncTime :exec
UPDATE task_lists
SET last_synced_at = ?
WHERE list_id = ?;

-- name: DeleteTaskList :exec
DELETE FROM task_lists WHERE list_id = ?;

-- Task CRUD queries

-- name: CreateTask :one
INSERT INTO agent_tasks (
    agent_id, list_id, claude_task_id, subject, description,
    active_form, metadata, status, owner, blocked_by, blocks,
    created_at, updated_at, file_path, file_mtime
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTask :one
SELECT * FROM agent_tasks WHERE id = ?;

-- name: GetTaskByClaudeID :one
SELECT * FROM agent_tasks
WHERE list_id = ? AND claude_task_id = ?;

-- name: UpsertTask :one
INSERT INTO agent_tasks (
    agent_id, list_id, claude_task_id, subject, description,
    active_form, metadata, status, owner, blocked_by, blocks,
    created_at, updated_at, started_at, completed_at, file_path, file_mtime
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(list_id, claude_task_id) DO UPDATE SET
    subject = COALESCE(NULLIF(excluded.subject, ''), agent_tasks.subject),
    description = COALESCE(NULLIF(excluded.description, ''), agent_tasks.description),
    active_form = COALESCE(NULLIF(excluded.active_form, ''), agent_tasks.active_form),
    metadata = COALESCE(NULLIF(excluded.metadata, ''), agent_tasks.metadata),
    status = excluded.status,
    owner = COALESCE(NULLIF(excluded.owner, ''), agent_tasks.owner),
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
RETURNING *;

-- name: UpdateTaskStatus :exec
UPDATE agent_tasks
SET status = ?,
    updated_at = ?,
    started_at = CASE WHEN ? = 'in_progress' AND started_at IS NULL THEN ? ELSE started_at END,
    completed_at = CASE WHEN ? = 'completed' THEN ? ELSE completed_at END
WHERE list_id = ? AND claude_task_id = ?;

-- name: UpdateTaskOwner :exec
UPDATE agent_tasks
SET owner = ?, updated_at = ?
WHERE list_id = ? AND claude_task_id = ?;

-- Task listing queries

-- name: ListTasksByAgent :many
SELECT * FROM agent_tasks
WHERE agent_id = ?
ORDER BY
    CASE status
        WHEN 'in_progress' THEN 0
        WHEN 'pending' THEN 1
        ELSE 2
    END,
    updated_at DESC;

-- name: ListTasksByAgentWithLimit :many
SELECT * FROM agent_tasks
WHERE agent_id = ?
ORDER BY
    CASE status
        WHEN 'in_progress' THEN 0
        WHEN 'pending' THEN 1
        ELSE 2
    END,
    updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListActiveTasksByAgent :many
SELECT * FROM agent_tasks
WHERE agent_id = ? AND status IN ('pending', 'in_progress')
ORDER BY
    CASE status WHEN 'in_progress' THEN 0 ELSE 1 END,
    created_at ASC;

-- name: ListTasksByList :many
SELECT * FROM agent_tasks
WHERE list_id = ?
ORDER BY
    CASE status
        WHEN 'in_progress' THEN 0
        WHEN 'pending' THEN 1
        ELSE 2
    END,
    updated_at DESC;

-- name: ListInProgressTasks :many
SELECT * FROM agent_tasks
WHERE agent_id = ? AND status = 'in_progress'
ORDER BY started_at ASC;

-- name: ListPendingTasks :many
SELECT * FROM agent_tasks
WHERE agent_id = ? AND status = 'pending'
ORDER BY created_at ASC;

-- name: ListBlockedTasks :many
SELECT * FROM agent_tasks
WHERE agent_id = ?
  AND status = 'pending'
  AND blocked_by != '[]'
ORDER BY created_at ASC;

-- name: ListAvailableTasks :many
SELECT * FROM agent_tasks
WHERE agent_id = ?
  AND status = 'pending'
  AND (owner IS NULL OR owner = '')
  AND (blocked_by IS NULL OR blocked_by = '[]')
ORDER BY created_at ASC;

-- name: ListRecentCompletedTasks :many
SELECT * FROM agent_tasks
WHERE agent_id = ?
  AND status = 'completed'
  AND completed_at > ?
ORDER BY completed_at DESC
LIMIT ?;

-- name: ListAllTasks :many
SELECT * FROM agent_tasks
ORDER BY
    CASE status
        WHEN 'in_progress' THEN 0
        WHEN 'pending' THEN 1
        ELSE 2
    END,
    updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListTasksByStatus :many
SELECT * FROM agent_tasks
WHERE status = ?
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

-- Task statistics queries

-- name: GetTaskStatsByAgent :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND (owner IS NULL OR owner = '') AND (blocked_by IS NULL OR blocked_by = '[]')) as available_count,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
FROM agent_tasks
WHERE agent_id = ?;

-- name: GetTaskStatsByList :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND (owner IS NULL OR owner = '') AND (blocked_by IS NULL OR blocked_by = '[]')) as available_count,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
FROM agent_tasks
WHERE list_id = ?;

-- name: GetAllTaskStats :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
FROM agent_tasks;

-- name: GetAllAgentTaskStats :many
SELECT
    agent_id,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_count,
    COUNT(*) FILTER (WHERE status = 'pending' AND blocked_by != '[]') as blocked_count,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > ?) as completed_today
FROM agent_tasks
GROUP BY agent_id;

-- Task count query for list

-- name: CountTasksByList :one
SELECT COUNT(*) FROM agent_tasks WHERE list_id = ?;

-- Cleanup queries

-- name: DeleteTask :exec
DELETE FROM agent_tasks WHERE id = ?;

-- name: DeleteTasksByList :exec
DELETE FROM agent_tasks WHERE list_id = ?;

-- name: MarkTasksDeletedByList :exec
UPDATE agent_tasks
SET status = 'deleted', updated_at = ?
WHERE list_id = ?
  AND claude_task_id NOT IN (/*SLICE:active_ids*/?)
  AND status NOT IN ('completed', 'deleted');

-- name: PruneOldTasks :exec
DELETE FROM agent_tasks
WHERE status IN ('completed', 'deleted')
  AND completed_at < ?;
