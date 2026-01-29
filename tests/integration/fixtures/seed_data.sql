-- Test seed data for integration tests.
-- Run this after migrations to populate test data.

-- Create test agents.
INSERT INTO agents (id, name, project_key, current_session_id, created_at, last_active_at)
VALUES
    (1, 'test-agent-1', '/test/project', NULL, strftime('%s', 'now'), strftime('%s', 'now')),
    (2, 'test-agent-2', '/test/project', NULL, strftime('%s', 'now'), strftime('%s', 'now')),
    (3, 'reviewer-agent', NULL, NULL, strftime('%s', 'now'), strftime('%s', 'now'));

-- Create test topics.
INSERT INTO topics (id, name, topic_type, retention_seconds, created_at)
VALUES
    (1, 'agent/test-agent-1/inbox', 'direct', 604800, strftime('%s', 'now')),
    (2, 'agent/test-agent-2/inbox', 'direct', 604800, strftime('%s', 'now')),
    (3, 'broadcast/all', 'broadcast', 604800, strftime('%s', 'now')),
    (4, 'project/test/notifications', 'broadcast', 604800, strftime('%s', 'now'));

-- Create subscriptions.
INSERT INTO subscriptions (agent_id, topic_id, subscribed_at)
VALUES
    (1, 1, strftime('%s', 'now')),
    (1, 3, strftime('%s', 'now')),
    (1, 4, strftime('%s', 'now')),
    (2, 2, strftime('%s', 'now')),
    (2, 3, strftime('%s', 'now')),
    (3, 3, strftime('%s', 'now'));

-- Create test messages.
INSERT INTO messages (id, thread_id, topic_id, log_offset, sender_id, subject, body_md, priority, deadline_at, attachments, created_at)
VALUES
    (1, 'thread-1', 1, 1, 2, 'Test Message 1', 'This is the first test message body.', 'normal', NULL, NULL, strftime('%s', 'now') - 3600),
    (2, 'thread-1', 1, 2, 1, 'Re: Test Message 1', 'This is a reply to the first message.', 'normal', NULL, NULL, strftime('%s', 'now') - 1800),
    (3, 'thread-2', 1, 3, 3, 'Urgent Review Needed', 'Please review PR #42 immediately.', 'urgent', strftime('%s', 'now') + 3600, NULL, strftime('%s', 'now') - 900),
    (4, 'thread-3', 3, 1, 1, 'Broadcast Announcement', 'This is a broadcast message to all agents.', 'normal', NULL, NULL, strftime('%s', 'now') - 300);

-- Create message recipients.
INSERT INTO message_recipients (message_id, agent_id, state, snoozed_until, read_at, acked_at)
VALUES
    (1, 1, 'unread', NULL, NULL, NULL),
    (2, 2, 'read', NULL, strftime('%s', 'now') - 1000, NULL),
    (3, 1, 'unread', NULL, NULL, NULL),
    (4, 1, 'read', NULL, strftime('%s', 'now') - 100, NULL),
    (4, 2, 'unread', NULL, NULL, NULL),
    (4, 3, 'unread', NULL, NULL, NULL);

-- Create consumer offsets.
INSERT INTO consumer_offsets (agent_id, topic_id, last_offset, updated_at)
VALUES
    (1, 1, 2, strftime('%s', 'now')),
    (2, 2, 0, strftime('%s', 'now'));

-- Create session identities.
INSERT INTO session_identities (session_id, agent_id, project_key, created_at, last_active_at)
VALUES
    ('test-session-001', 1, '/test/project', strftime('%s', 'now') - 7200, strftime('%s', 'now'));
