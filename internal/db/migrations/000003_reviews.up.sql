-- Review requests table
CREATE TABLE reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL UNIQUE,           -- UUID
    thread_id TEXT NOT NULL,                  -- Links to message thread
    requester_id INTEGER NOT NULL REFERENCES agents(id),

    -- PR Information
    pr_number INTEGER,
    branch TEXT NOT NULL,
    base_branch TEXT NOT NULL DEFAULT 'main',
    commit_sha TEXT NOT NULL,
    repo_path TEXT NOT NULL,

    -- Configuration
    review_type TEXT NOT NULL DEFAULT 'full' CHECK (review_type IN ('full', 'incremental', 'security', 'performance')),
    priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('urgent', 'normal', 'low')),

    -- State
    state TEXT NOT NULL DEFAULT 'new' CHECK (state IN ('new', 'pending_review', 'under_review', 'changes_requested', 're_review', 'approved', 'rejected', 'cancelled')),

    -- Timestamps
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
);

-- Review iterations (each round of review)
CREATE TABLE review_iterations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL REFERENCES reviews(review_id) ON DELETE CASCADE,
    iteration_num INTEGER NOT NULL,

    -- Reviewer info
    reviewer_id TEXT NOT NULL,                -- Reviewer persona name
    reviewer_session_id TEXT,                 -- Claude session ID for this review

    -- Results
    decision TEXT NOT NULL CHECK (decision IN ('approve', 'request_changes', 'comment')),
    summary TEXT NOT NULL,
    issues_json TEXT,                         -- JSON array of ReviewIssue
    suggestions_json TEXT,                    -- JSON array of Suggestion

    -- Metrics
    files_reviewed INTEGER NOT NULL DEFAULT 0,
    lines_analyzed INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,

    -- Timestamps
    started_at INTEGER NOT NULL,
    completed_at INTEGER,

    UNIQUE(review_id, iteration_num, reviewer_id)
);

-- Review issues (denormalized for querying)
CREATE TABLE review_issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id TEXT NOT NULL REFERENCES reviews(review_id) ON DELETE CASCADE,
    iteration_num INTEGER NOT NULL,

    issue_type TEXT NOT NULL CHECK (issue_type IN ('bug', 'security', 'claude_md_violation', 'logic_error')),
    severity TEXT NOT NULL CHECK (severity IN ('critical', 'high', 'medium', 'low')),

    file_path TEXT NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER,

    title TEXT NOT NULL,
    description TEXT NOT NULL,
    code_snippet TEXT,
    suggestion TEXT,
    claude_md_ref TEXT,

    -- Resolution tracking
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'fixed', 'wont_fix', 'duplicate')),
    resolved_at INTEGER,
    resolved_in_iteration INTEGER,

    created_at INTEGER NOT NULL
);

-- Indexes for common queries
CREATE INDEX idx_reviews_state ON reviews(state);
CREATE INDEX idx_reviews_requester ON reviews(requester_id);
CREATE INDEX idx_reviews_thread ON reviews(thread_id);
CREATE INDEX idx_reviews_created ON reviews(created_at DESC);
CREATE INDEX idx_reviews_branch ON reviews(branch);
CREATE INDEX idx_review_iterations_review ON review_iterations(review_id);
CREATE INDEX idx_review_iterations_decision ON review_iterations(decision);
CREATE INDEX idx_review_issues_review ON review_issues(review_id);
CREATE INDEX idx_review_issues_status ON review_issues(status);
CREATE INDEX idx_review_issues_severity ON review_issues(severity);
CREATE INDEX idx_review_issues_file ON review_issues(file_path);
