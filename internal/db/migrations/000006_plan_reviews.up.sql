-- Plan reviews table: tracks plan mode approval workflow.
CREATE TABLE plan_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_review_id TEXT NOT NULL UNIQUE,      -- UUID for external reference.
    message_id INTEGER REFERENCES messages(id),
    thread_id TEXT NOT NULL,                  -- Links to message thread.

    requester_id INTEGER NOT NULL REFERENCES agents(id),
    reviewer_name TEXT NOT NULL DEFAULT 'User',

    -- Plan content metadata.
    plan_path TEXT NOT NULL,
    plan_title TEXT NOT NULL,
    plan_summary TEXT,

    -- State machine: pending -> approved/rejected/changes_requested.
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'approved', 'rejected', 'changes_requested')),

    -- Reviewer response.
    reviewer_comment TEXT,
    reviewed_by INTEGER REFERENCES agents(id),

    -- Session tracking for lookup by active session.
    session_id TEXT,

    -- Timestamps (Unix epoch seconds).
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    reviewed_at INTEGER
);

CREATE INDEX idx_plan_reviews_state ON plan_reviews(state);
CREATE INDEX idx_plan_reviews_message ON plan_reviews(message_id);
CREATE INDEX idx_plan_reviews_thread ON plan_reviews(thread_id);
CREATE INDEX idx_plan_reviews_session ON plan_reviews(session_id);
CREATE INDEX idx_plan_reviews_requester ON plan_reviews(requester_id);
CREATE INDEX idx_plan_reviews_created ON plan_reviews(created_at DESC);
