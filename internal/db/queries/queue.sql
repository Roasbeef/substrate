-- name: EnqueueOperation :one
INSERT INTO pending_operations (
    idempotency_key, operation_type, payload_json, agent_name,
    session_id, created_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListPendingOperations :many
SELECT * FROM pending_operations
WHERE status = 'pending' ORDER BY created_at ASC;

-- name: DrainPendingOperations :many
UPDATE pending_operations SET status = 'delivering'
WHERE status = 'pending' RETURNING *;

-- name: MarkOperationDelivered :exec
UPDATE pending_operations SET status = 'delivered' WHERE id = ?;

-- name: MarkOperationFailed :exec
UPDATE pending_operations
SET status = 'pending', attempts = attempts + 1, last_error = ?
WHERE id = ?;

-- name: ClearAllOperations :exec
DELETE FROM pending_operations;

-- name: PurgeExpiredOperations :execrows
DELETE FROM pending_operations
WHERE expires_at < ? AND status IN ('pending', 'failed');

-- name: CountPendingOperations :one
SELECT COUNT(*) FROM pending_operations WHERE status = 'pending';

-- name: GetQueueStats :one
SELECT
    COUNT(CASE WHEN status = 'pending' THEN 1 END) AS pending_count,
    COUNT(CASE WHEN status = 'delivered' THEN 1 END) AS delivered_count,
    COUNT(CASE WHEN status = 'expired' THEN 1 END) AS expired_count,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed_count,
    MIN(CASE WHEN status = 'pending' THEN created_at END) AS oldest_pending
FROM pending_operations;
