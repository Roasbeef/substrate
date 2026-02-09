-- Drop plan review indexes and table.
DROP INDEX IF EXISTS idx_plan_reviews_created;
DROP INDEX IF EXISTS idx_plan_reviews_requester;
DROP INDEX IF EXISTS idx_plan_reviews_session;
DROP INDEX IF EXISTS idx_plan_reviews_thread;
DROP INDEX IF EXISTS idx_plan_reviews_message;
DROP INDEX IF EXISTS idx_plan_reviews_state;
DROP TABLE IF EXISTS plan_reviews;
