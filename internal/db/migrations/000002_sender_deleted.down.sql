-- Remove deleted_by_sender column from messages table.
-- Note: SQLite doesn't support DROP COLUMN directly before 3.35.0
-- This migration assumes SQLite 3.35.0+ or uses a workaround.
ALTER TABLE messages DROP COLUMN deleted_by_sender;
