-- name: CreateTopic :one
INSERT INTO topics (name, topic_type, retention_seconds, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetTopic :one
SELECT * FROM topics WHERE id = ?;

-- name: GetTopicByName :one
SELECT * FROM topics WHERE name = ?;

-- name: ListTopics :many
SELECT * FROM topics ORDER BY name;

-- name: ListTopicsWithMessageCount :many
SELECT t.*, COUNT(m.id) as message_count
FROM topics t
LEFT JOIN messages m ON t.id = m.topic_id
GROUP BY t.id
ORDER BY t.name;

-- name: ListTopicsByType :many
SELECT * FROM topics WHERE topic_type = ? ORDER BY name;

-- name: UpdateTopicRetention :exec
UPDATE topics SET retention_seconds = ? WHERE id = ?;

-- name: DeleteTopic :exec
DELETE FROM topics WHERE id = ?;

-- name: CreateSubscription :exec
INSERT INTO subscriptions (agent_id, topic_id, subscribed_at)
VALUES (?, ?, ?)
ON CONFLICT (agent_id, topic_id) DO NOTHING;

-- name: DeleteSubscription :exec
DELETE FROM subscriptions WHERE agent_id = ? AND topic_id = ?;

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE agent_id = ? AND topic_id = ?;

-- name: ListSubscriptionsByAgent :many
SELECT t.* FROM topics t
JOIN subscriptions s ON t.id = s.topic_id
WHERE s.agent_id = ?
ORDER BY t.name;

-- name: ListSubscriptionsByTopic :many
SELECT a.* FROM agents a
JOIN subscriptions s ON a.id = s.agent_id
WHERE s.topic_id = ?
ORDER BY a.name;

-- name: CountSubscribersByTopic :one
SELECT COUNT(*) FROM subscriptions WHERE topic_id = ?;

-- name: GetOrCreateAgentInboxTopic :one
INSERT INTO topics (name, topic_type, retention_seconds, created_at)
VALUES ('agent/' || ? || '/inbox', 'direct', 604800, ?)
ON CONFLICT (name) DO UPDATE SET name = topics.name
RETURNING *;

-- name: GetOrCreateTopic :one
INSERT INTO topics (name, topic_type, retention_seconds, created_at)
VALUES (?, ?, 604800, ?)
ON CONFLICT (name) DO UPDATE SET name = topics.name
RETURNING *;

-- name: UpsertConsumerOffset :exec
INSERT INTO consumer_offsets (agent_id, topic_id, last_offset, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (agent_id, topic_id) DO UPDATE SET
    last_offset = excluded.last_offset,
    updated_at = excluded.updated_at;

-- name: GetConsumerOffset :one
SELECT last_offset FROM consumer_offsets
WHERE agent_id = ? AND topic_id = ?;

-- name: ListConsumerOffsetsByAgent :many
SELECT co.topic_id, co.last_offset, t.name as topic_name
FROM consumer_offsets co
JOIN topics t ON co.topic_id = t.id
WHERE co.agent_id = ?;
