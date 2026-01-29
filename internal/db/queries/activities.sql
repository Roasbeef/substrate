-- name: CreateActivity :one
INSERT INTO activities (agent_id, activity_type, description, metadata, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetActivity :one
SELECT * FROM activities WHERE id = ?;

-- name: ListRecentActivities :many
SELECT * FROM activities
ORDER BY created_at DESC
LIMIT ?;

-- name: ListActivitiesByAgent :many
SELECT * FROM activities
WHERE agent_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListActivitiesByType :many
SELECT * FROM activities
WHERE activity_type = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListActivitiesSince :many
SELECT * FROM activities
WHERE created_at > ?
ORDER BY created_at DESC
LIMIT ?;

-- name: CountActivitiesByAgentToday :one
SELECT COUNT(*) FROM activities
WHERE agent_id = ? AND created_at > ?;

-- name: DeleteOldActivities :exec
DELETE FROM activities
WHERE created_at < ?;
