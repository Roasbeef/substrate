-- Drop the pending operations table.
DROP TABLE IF EXISTS pending_operations;

-- Drop the idempotency key index.
DROP INDEX IF EXISTS idx_messages_idempotency_key;

-- NOTE: SQLite does not support DROP COLUMN, so the idempotency_key column
-- on the messages table will remain. This is safe as it is nullable and
-- has no effect on existing queries.
