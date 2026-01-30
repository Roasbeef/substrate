-- name: CreateMessage :one
INSERT INTO messages (
    thread_id, topic_id, log_offset, sender_id, subject, body_md,
    priority, deadline_at, attachments, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = ?;

-- name: GetMessagesByThread :many
SELECT * FROM messages WHERE thread_id = ? ORDER BY created_at ASC;

-- name: GetMessagesByTopic :many
SELECT * FROM messages WHERE topic_id = ? ORDER BY log_offset ASC;

-- name: GetMessagesSinceOffset :many
SELECT * FROM messages
WHERE topic_id = ? AND log_offset > ?
ORDER BY log_offset ASC
LIMIT ?;

-- name: GetMaxLogOffset :one
SELECT COALESCE(MAX(log_offset), 0) FROM messages WHERE topic_id = ?;

-- name: ListMessagesByPriority :many
SELECT m.* FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ? AND m.priority = ?
ORDER BY m.created_at DESC
LIMIT ?;

-- name: DeleteMessage :exec
DELETE FROM messages WHERE id = ?;

-- name: DeleteMessagesByTopicOlderThan :execrows
DELETE FROM messages WHERE topic_id = ? AND created_at < ?;

-- name: CreateMessageRecipient :exec
INSERT INTO message_recipients (message_id, agent_id, state)
VALUES (?, ?, 'unread');

-- name: GetMessageRecipient :one
SELECT * FROM message_recipients
WHERE message_id = ? AND agent_id = ?;

-- name: GetMessageRecipients :many
SELECT * FROM message_recipients
WHERE message_id = ?;

-- name: UpdateRecipientState :exec
UPDATE message_recipients
SET state = ?, read_at = CASE WHEN ? = 'read' THEN ? ELSE read_at END
WHERE message_id = ? AND agent_id = ?;

-- name: UpdateRecipientAcked :exec
UPDATE message_recipients
SET acked_at = ?
WHERE message_id = ? AND agent_id = ?;

-- name: UpdateRecipientSnoozed :exec
UPDATE message_recipients
SET state = 'snoozed', snoozed_until = ?
WHERE message_id = ? AND agent_id = ?;

-- name: GetInboxMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state NOT IN ('archived', 'trash')
ORDER BY m.created_at DESC
LIMIT ?;

-- name: GetAllInboxMessages :many
-- Global inbox view: all messages across all agents, not archived or trashed.
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at, mr.agent_id as recipient_agent_id
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.state NOT IN ('archived', 'trash')
ORDER BY m.created_at DESC
LIMIT ?;

-- name: GetUnreadMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state = 'unread'
ORDER BY m.created_at DESC
LIMIT ?;

-- name: GetStarredMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state = 'starred'
ORDER BY m.created_at DESC
LIMIT ?;

-- name: GetSnoozedMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state = 'snoozed'
ORDER BY mr.snoozed_until ASC
LIMIT ?;

-- name: GetSnoozedMessagesReadyToWake :many
SELECT m.*, mr.agent_id as recipient_agent_id
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.state = 'snoozed'
    AND mr.snoozed_until <= ?
ORDER BY mr.snoozed_until ASC;

-- name: WakeSnoozedMessages :execrows
UPDATE message_recipients
SET state = 'unread', snoozed_until = NULL
WHERE state = 'snoozed' AND snoozed_until <= ?;

-- name: GetArchivedMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state = 'archived'
ORDER BY m.created_at DESC
LIMIT ?;

-- name: GetTrashMessages :many
SELECT m.*, mr.state, mr.snoozed_until, mr.read_at, mr.acked_at
FROM messages m
JOIN message_recipients mr ON m.id = mr.message_id
WHERE mr.agent_id = ?
    AND mr.state = 'trash'
ORDER BY m.created_at DESC
LIMIT ?;

-- name: CountUnreadByAgent :one
SELECT COUNT(*) FROM message_recipients
WHERE agent_id = ? AND state = 'unread';

-- name: CountUnreadUrgentByAgent :one
SELECT COUNT(*) FROM message_recipients mr
JOIN messages m ON mr.message_id = m.id
WHERE mr.agent_id = ? AND mr.state = 'unread' AND m.priority = 'urgent';

-- name: GetSentMessages :many
SELECT * FROM messages
WHERE sender_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: CountStarredByAgent :one
SELECT COUNT(*) FROM message_recipients
WHERE agent_id = ? AND state = 'starred';

-- name: CountSnoozedByAgent :one
SELECT COUNT(*) FROM message_recipients
WHERE agent_id = ? AND state = 'snoozed';

-- name: CountArchivedByAgent :one
SELECT COUNT(*) FROM message_recipients
WHERE agent_id = ? AND state = 'archived';

-- name: CountSentByAgent :one
SELECT COUNT(*) FROM messages
WHERE sender_id = ?;

-- Note: Full-text search queries using FTS5 are handled manually in Go code
-- since sqlc doesn't fully support FTS5 virtual tables.
