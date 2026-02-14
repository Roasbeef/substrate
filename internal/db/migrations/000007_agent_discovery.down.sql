-- Drop the compound index added by the up migration.
DROP INDEX IF EXISTS idx_recipients_agent_state;

-- SQLite doesn't support DROP COLUMN before 3.35.0, so we recreate the table.
CREATE TABLE agents_backup AS SELECT
    id, name, project_key, git_branch, current_session_id,
    created_at, last_active_at
FROM agents;

DROP TABLE agents;

CREATE TABLE agents (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    project_key TEXT,
    git_branch TEXT,
    current_session_id TEXT,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER NOT NULL
);

INSERT INTO agents SELECT * FROM agents_backup;
DROP TABLE agents_backup;
