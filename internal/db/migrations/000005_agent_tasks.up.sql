-- Migration 000005: Add task tracking tables for Claude Code task integration
-- Tasks persist to ~/.claude/tasks/{listID}/ as JSON files; we mirror them here

-- Table for tracking registered task lists
CREATE TABLE task_lists (
    id INTEGER PRIMARY KEY,
    list_id TEXT NOT NULL UNIQUE,           -- Claude's list ID (from CLAUDE_CODE_TASK_LIST_ID)
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    watch_path TEXT NOT NULL,               -- Full path to ~/.claude/tasks/{list_id}/
    created_at INTEGER NOT NULL,
    last_synced_at INTEGER                  -- Last time we synced from files
);

CREATE INDEX idx_task_lists_agent ON task_lists(agent_id);

-- Table for individual tasks
CREATE TABLE agent_tasks (
    id INTEGER PRIMARY KEY,

    -- Ownership
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    list_id TEXT NOT NULL,                  -- References task_lists.list_id

    -- Claude's task identification
    claude_task_id TEXT NOT NULL,           -- ID from Claude (e.g., "task-123")

    -- Task content (from TaskCreate)
    subject TEXT NOT NULL,                  -- Imperative title
    description TEXT,                       -- Detailed requirements
    active_form TEXT,                       -- Present-tense spinner text
    metadata TEXT,                          -- JSON blob for arbitrary KV pairs

    -- Lifecycle
    status TEXT NOT NULL DEFAULT 'pending', -- pending, in_progress, completed, deleted
    owner TEXT,                             -- Assigned agent name

    -- Dependencies (stored as JSON arrays of task IDs)
    blocked_by TEXT DEFAULT '[]',           -- Tasks blocking this one
    blocks TEXT DEFAULT '[]',               -- Tasks this one blocks

    -- Timestamps
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    started_at INTEGER,                     -- When moved to in_progress
    completed_at INTEGER,                   -- When moved to completed

    -- File tracking
    file_path TEXT,                         -- Path to the JSON file
    file_mtime INTEGER,                     -- File modification time for change detection

    UNIQUE(list_id, claude_task_id)
);

-- Indexes for common queries
CREATE INDEX idx_agent_tasks_agent_status ON agent_tasks(agent_id, status);
CREATE INDEX idx_agent_tasks_list ON agent_tasks(list_id);
CREATE INDEX idx_agent_tasks_status ON agent_tasks(status) WHERE status NOT IN ('completed', 'deleted');
CREATE INDEX idx_agent_tasks_owner ON agent_tasks(owner) WHERE owner IS NOT NULL;

-- View for available tasks (pending, no owner, no blockers)
CREATE VIEW available_tasks AS
SELECT * FROM agent_tasks
WHERE status = 'pending'
  AND (owner IS NULL OR owner = '')
  AND (blocked_by IS NULL OR blocked_by = '[]');
