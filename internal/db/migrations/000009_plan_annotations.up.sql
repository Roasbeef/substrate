-- Plan annotations table: stores inline annotations on plan review content.
CREATE TABLE plan_annotations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_review_id TEXT NOT NULL REFERENCES plan_reviews(plan_review_id) ON DELETE CASCADE,
    annotation_id TEXT NOT NULL UNIQUE,
    block_id TEXT NOT NULL,
    annotation_type TEXT NOT NULL
        CHECK (annotation_type IN (
            'DELETION', 'INSERTION', 'REPLACEMENT',
            'COMMENT', 'GLOBAL_COMMENT'
        )),
    text TEXT,
    original_text TEXT NOT NULL,
    start_offset INTEGER NOT NULL DEFAULT 0,
    end_offset INTEGER NOT NULL DEFAULT 0,
    diff_context TEXT
        CHECK (diff_context IS NULL OR diff_context IN (
            'added', 'removed', 'modified'
        )),
    created_by INTEGER REFERENCES agents(id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_plan_annotations_review
    ON plan_annotations(plan_review_id);
CREATE INDEX idx_plan_annotations_block
    ON plan_annotations(plan_review_id, block_id);

-- Diff annotations table: stores line-level annotations on code diffs.
CREATE TABLE diff_annotations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    annotation_id TEXT NOT NULL UNIQUE,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    annotation_type TEXT NOT NULL
        CHECK (annotation_type IN ('comment', 'suggestion', 'concern')),
    scope TEXT NOT NULL DEFAULT 'line'
        CHECK (scope IN ('line', 'file')),
    file_path TEXT NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER NOT NULL,
    side TEXT NOT NULL DEFAULT 'new'
        CHECK (side IN ('old', 'new')),
    text TEXT,
    suggested_code TEXT,
    original_code TEXT,
    created_by INTEGER REFERENCES agents(id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_diff_annotations_message
    ON diff_annotations(message_id);
CREATE INDEX idx_diff_annotations_file
    ON diff_annotations(message_id, file_path);
