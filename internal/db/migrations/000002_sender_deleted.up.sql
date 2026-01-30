-- Add deleted_by_sender column to messages table.
-- This allows senders to "delete" their sent messages from the Sent view
-- without affecting the recipients' copies.
ALTER TABLE messages ADD COLUMN deleted_by_sender INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_messages_deleted ON messages(deleted_by_sender);
