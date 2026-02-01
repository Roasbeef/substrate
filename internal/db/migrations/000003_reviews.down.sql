-- Drop indexes first
DROP INDEX IF EXISTS idx_review_issues_file;
DROP INDEX IF EXISTS idx_review_issues_severity;
DROP INDEX IF EXISTS idx_review_issues_status;
DROP INDEX IF EXISTS idx_review_issues_review;
DROP INDEX IF EXISTS idx_review_iterations_decision;
DROP INDEX IF EXISTS idx_review_iterations_review;
DROP INDEX IF EXISTS idx_reviews_branch;
DROP INDEX IF EXISTS idx_reviews_created;
DROP INDEX IF EXISTS idx_reviews_thread;
DROP INDEX IF EXISTS idx_reviews_requester;
DROP INDEX IF EXISTS idx_reviews_state;

-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS review_issues;
DROP TABLE IF EXISTS review_iterations;
DROP TABLE IF EXISTS reviews;
