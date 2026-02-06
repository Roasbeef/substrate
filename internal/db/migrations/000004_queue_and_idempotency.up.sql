-- Idempotency key for message deduplication.
ALTER TABLE messages ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_idempotency_key
    ON messages(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Local queue for offline CLI operations.
CREATE TABLE IF NOT EXISTS pending_operations (
    id INTEGER PRIMARY KEY,
    idempotency_key TEXT UNIQUE NOT NULL,
    operation_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    session_id TEXT,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_pending_status
    ON pending_operations(status);
CREATE INDEX IF NOT EXISTS idx_pending_expires
    ON pending_operations(expires_at);
