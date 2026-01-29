-- Agents table: Named entities that send/receive mail.
CREATE TABLE IF NOT EXISTS agents (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    project_key TEXT,
    git_branch TEXT,
    current_session_id TEXT,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_name ON agents(name);
CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project_key);

-- Topics table: Pub/sub channels for message routing.
CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    topic_type TEXT NOT NULL CHECK (topic_type IN ('direct', 'broadcast', 'queue')),
    retention_seconds INTEGER DEFAULT 604800,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);
CREATE INDEX IF NOT EXISTS idx_topics_type ON topics(topic_type);

-- Subscriptions: Agent to topic mappings.
CREATE TABLE IF NOT EXISTS subscriptions (
    id INTEGER PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    subscribed_at INTEGER NOT NULL,
    UNIQUE(agent_id, topic_id)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_agent ON subscriptions(agent_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_topic ON subscriptions(topic_id);

-- Messages table: Log-structured message storage.
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY,
    thread_id TEXT NOT NULL,
    topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    log_offset INTEGER NOT NULL,
    sender_id INTEGER NOT NULL REFERENCES agents(id),
    subject TEXT NOT NULL,
    body_md TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('urgent', 'normal', 'low')),
    deadline_at INTEGER,
    attachments TEXT,
    created_at INTEGER NOT NULL,
    UNIQUE(topic_id, log_offset)
);

CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_topic ON messages(topic_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_priority ON messages(priority);

-- Message recipients: Per-agent message state tracking.
CREATE TABLE IF NOT EXISTS message_recipients (
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'unread' CHECK (state IN ('unread', 'read', 'starred', 'snoozed', 'archived', 'trash')),
    snoozed_until INTEGER,
    read_at INTEGER,
    acked_at INTEGER,
    PRIMARY KEY(message_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_recipients_agent ON message_recipients(agent_id);
CREATE INDEX IF NOT EXISTS idx_recipients_state ON message_recipients(state);
CREATE INDEX IF NOT EXISTS idx_recipients_snoozed ON message_recipients(snoozed_until) WHERE snoozed_until IS NOT NULL;

-- Consumer offsets: Log-based consumption tracking.
CREATE TABLE IF NOT EXISTS consumer_offsets (
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    topic_id INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    last_offset INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY(agent_id, topic_id)
);

-- Session identities: Maps Claude Code sessions to agents.
CREATE TABLE IF NOT EXISTS session_identities (
    session_id TEXT PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_key TEXT,
    git_branch TEXT,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_identities_agent ON session_identities(agent_id);
CREATE INDEX IF NOT EXISTS idx_session_identities_project ON session_identities(project_key);

-- Full-text search for messages.
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    subject,
    body_md,
    content=messages,
    content_rowid=id
);

-- Triggers to keep FTS index in sync.
CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, subject, body_md) VALUES (new.id, new.subject, new.body_md);
END;

CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, subject, body_md) VALUES('delete', old.id, old.subject, old.body_md);
END;

CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, subject, body_md) VALUES('delete', old.id, old.subject, old.body_md);
    INSERT INTO messages_fts(rowid, subject, body_md) VALUES (new.id, new.subject, new.body_md);
END;

-- Activities table: Agent activity tracking for the dashboard feed.
CREATE TABLE IF NOT EXISTS activities (
    id INTEGER PRIMARY KEY,
    agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    activity_type TEXT NOT NULL CHECK (activity_type IN (
        'commit', 'message', 'session_start', 'session_complete',
        'decision', 'error', 'blocker', 'heartbeat', 'task_complete'
    )),
    description TEXT NOT NULL,
    metadata TEXT, -- JSON blob for additional context.
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activities_agent ON activities(agent_id);
CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(activity_type);
CREATE INDEX IF NOT EXISTS idx_activities_created ON activities(created_at DESC);
