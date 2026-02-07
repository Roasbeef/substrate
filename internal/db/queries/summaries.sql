-- name: CreateAgentSummary :one
INSERT INTO agent_summaries (agent_id, summary, delta, transcript_hash, cost_usd, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetLatestAgentSummary :one
SELECT * FROM agent_summaries
WHERE agent_id = ?
ORDER BY created_at DESC
LIMIT 1;

-- name: GetAgentSummaryHistory :many
SELECT * FROM agent_summaries
WHERE agent_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: DeleteOldAgentSummaries :exec
DELETE FROM agent_summaries WHERE created_at < ?;
