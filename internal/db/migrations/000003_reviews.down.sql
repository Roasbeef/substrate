-- Drop review tables in reverse order of dependencies.
DROP INDEX IF EXISTS idx_review_issues_severity;
DROP INDEX IF EXISTS idx_review_issues_status;
DROP INDEX IF EXISTS idx_review_issues_review;
DROP TABLE IF EXISTS review_issues;

DROP INDEX IF EXISTS idx_review_iterations_review;
DROP TABLE IF EXISTS review_iterations;

DROP INDEX IF EXISTS idx_reviews_created;
DROP INDEX IF EXISTS idx_reviews_thread;
DROP INDEX IF EXISTS idx_reviews_requester;
DROP INDEX IF EXISTS idx_reviews_state;
DROP TABLE IF EXISTS reviews;

DROP TRIGGER IF EXISTS check_activity_type_reviews;

-- SQLite does not support DROP COLUMN before version 3.35.0.
-- The metadata column on messages will remain after downgrade.
