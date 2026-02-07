-- name: CreateAgent :one
INSERT INTO agents (name, project_key, git_branch, current_session_id, created_at, last_active_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = ?;

-- name: GetAgentByName :one
SELECT * FROM agents WHERE name = ?;

-- name: GetAgentBySessionID :one
SELECT a.* FROM agents a
JOIN session_identities si ON a.id = si.agent_id
WHERE si.session_id = ?;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY last_active_at DESC;

-- name: ListAgentsByProject :many
SELECT * FROM agents WHERE project_key = ? ORDER BY last_active_at DESC;

-- name: UpdateAgentLastActive :exec
UPDATE agents SET last_active_at = ? WHERE id = ?;

-- name: UpdateAgentName :exec
UPDATE agents SET name = ? WHERE id = ?;

-- name: UpdateAgentSession :exec
UPDATE agents SET current_session_id = ?, last_active_at = ? WHERE id = ?;

-- name: UpdateAgentGitBranch :exec
UPDATE agents SET git_branch = ?, project_key = ?, last_active_at = ? WHERE id = ?;

-- name: SearchAgents :many
SELECT * FROM agents
WHERE name LIKE '%' || @query || '%'
   OR project_key LIKE '%' || @query || '%'
   OR git_branch LIKE '%' || @query || '%'
ORDER BY last_active_at DESC
LIMIT @max_results;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;

-- name: CreateSessionIdentity :exec
INSERT INTO session_identities (session_id, agent_id, project_key, git_branch, created_at, last_active_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (session_id) DO UPDATE SET
    agent_id = excluded.agent_id,
    project_key = excluded.project_key,
    git_branch = excluded.git_branch,
    last_active_at = excluded.last_active_at;

-- name: GetSessionIdentity :one
SELECT * FROM session_identities WHERE session_id = ?;

-- name: GetSessionIdentityByProject :one
SELECT * FROM session_identities WHERE project_key = ? ORDER BY last_active_at DESC LIMIT 1;

-- name: UpdateSessionIdentityLastActive :exec
UPDATE session_identities SET last_active_at = ? WHERE session_id = ?;

-- name: DeleteSessionIdentity :exec
DELETE FROM session_identities WHERE session_id = ?;

-- name: ListSessionIdentitiesByAgent :many
SELECT * FROM session_identities WHERE agent_id = ? ORDER BY last_active_at DESC;

-- name: UpdateAgentDiscoveryInfo :exec
UPDATE agents SET
    purpose = COALESCE(NULLIF(?, ''), purpose),
    working_dir = COALESCE(NULLIF(?, ''), working_dir),
    hostname = COALESCE(NULLIF(?, ''), hostname),
    last_active_at = ?
WHERE id = ?;

-- name: DiscoverAgents :many
SELECT
    a.*,
    COALESCE(
        (SELECT COUNT(*) FROM message_recipients mr
         WHERE mr.agent_id = a.id AND mr.state = 'unread'), 0
    ) AS unread_count
FROM agents a
ORDER BY a.last_active_at DESC;
