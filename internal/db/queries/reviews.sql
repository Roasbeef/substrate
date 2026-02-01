-- name: CreateReview :one
INSERT INTO reviews (
    review_id, thread_id, requester_id, pr_number, branch, base_branch,
    commit_sha, repo_path, review_type, priority, state, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReview :one
SELECT * FROM reviews WHERE review_id = ?;

-- name: GetReviewByID :one
SELECT * FROM reviews WHERE id = ?;

-- name: GetReviewByThread :one
SELECT * FROM reviews WHERE thread_id = ?;

-- name: ListReviews :many
SELECT * FROM reviews
ORDER BY created_at DESC
LIMIT ?;

-- name: ListReviewsByRequester :many
SELECT * FROM reviews
WHERE requester_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListReviewsByState :many
SELECT * FROM reviews
WHERE state = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListReviewsByBranch :many
SELECT * FROM reviews
WHERE branch = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListPendingReviews :many
SELECT * FROM reviews
WHERE state IN ('new', 'pending_review', 're_review')
ORDER BY
    CASE priority
        WHEN 'urgent' THEN 1
        WHEN 'normal' THEN 2
        WHEN 'low' THEN 3
    END,
    created_at ASC
LIMIT ?;

-- name: ListActiveReviews :many
SELECT * FROM reviews
WHERE state NOT IN ('approved', 'rejected', 'cancelled')
ORDER BY updated_at DESC
LIMIT ?;

-- name: UpdateReviewState :exec
UPDATE reviews
SET state = ?, updated_at = ?
WHERE review_id = ?;

-- name: UpdateReviewCommit :exec
UPDATE reviews
SET commit_sha = ?, updated_at = ?
WHERE review_id = ?;

-- name: CompleteReview :exec
UPDATE reviews
SET state = ?, completed_at = ?, updated_at = ?
WHERE review_id = ?;

-- name: DeleteReview :exec
DELETE FROM reviews WHERE review_id = ?;

-- name: CountReviewsByState :one
SELECT COUNT(*) FROM reviews WHERE state = ?;

-- name: CountOpenReviewsByRequester :one
SELECT COUNT(*) FROM reviews
WHERE requester_id = ?
  AND state NOT IN ('approved', 'rejected', 'cancelled');

-- Review Iterations queries

-- name: CreateReviewIteration :one
INSERT INTO review_iterations (
    review_id, iteration_num, reviewer_id, reviewer_session_id, decision,
    summary, issues_json, suggestions_json, files_reviewed, lines_analyzed,
    duration_ms, cost_usd, started_at, completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReviewIteration :one
SELECT * FROM review_iterations
WHERE review_id = ? AND iteration_num = ? AND reviewer_id = ?;

-- name: GetLatestReviewIteration :one
SELECT * FROM review_iterations
WHERE review_id = ?
ORDER BY iteration_num DESC, started_at DESC
LIMIT 1;

-- name: ListReviewIterations :many
SELECT * FROM review_iterations
WHERE review_id = ?
ORDER BY iteration_num ASC, started_at ASC;

-- name: ListReviewIterationsByReviewer :many
SELECT * FROM review_iterations
WHERE reviewer_id = ?
ORDER BY started_at DESC
LIMIT ?;

-- name: GetIterationCount :one
SELECT COALESCE(MAX(iteration_num), 0) as max_iteration
FROM review_iterations
WHERE review_id = ?;

-- name: UpdateReviewIterationComplete :exec
UPDATE review_iterations
SET completed_at = ?
WHERE id = ?;

-- name: CountIterationsByDecision :one
SELECT
    SUM(CASE WHEN decision = 'approve' THEN 1 ELSE 0 END) as approvals,
    SUM(CASE WHEN decision = 'request_changes' THEN 1 ELSE 0 END) as change_requests,
    SUM(CASE WHEN decision = 'comment' THEN 1 ELSE 0 END) as comments
FROM review_iterations
WHERE review_id = ? AND iteration_num = ?;

-- name: GetReviewerDecisions :many
SELECT reviewer_id, decision
FROM review_iterations
WHERE review_id = ? AND iteration_num = ?;

-- Review Issues queries

-- name: CreateReviewIssue :one
INSERT INTO review_issues (
    review_id, iteration_num, issue_type, severity, file_path, line_start,
    line_end, title, description, code_snippet, suggestion, claude_md_ref,
    status, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReviewIssue :one
SELECT * FROM review_issues WHERE id = ?;

-- name: ListReviewIssues :many
SELECT * FROM review_issues
WHERE review_id = ?
ORDER BY
    CASE severity
        WHEN 'critical' THEN 1
        WHEN 'high' THEN 2
        WHEN 'medium' THEN 3
        WHEN 'low' THEN 4
    END,
    file_path, line_start;

-- name: ListReviewIssuesByIteration :many
SELECT * FROM review_issues
WHERE review_id = ? AND iteration_num = ?
ORDER BY severity, file_path, line_start;

-- name: ListOpenReviewIssues :many
SELECT * FROM review_issues
WHERE review_id = ? AND status = 'open'
ORDER BY severity, file_path, line_start;

-- name: ListReviewIssuesByFile :many
SELECT * FROM review_issues
WHERE review_id = ? AND file_path = ?
ORDER BY line_start;

-- name: ListReviewIssuesBySeverity :many
SELECT * FROM review_issues
WHERE review_id = ? AND severity = ?
ORDER BY file_path, line_start;

-- name: UpdateReviewIssueStatus :exec
UPDATE review_issues
SET status = ?, resolved_at = ?, resolved_in_iteration = ?
WHERE id = ?;

-- name: ResolveIssue :exec
UPDATE review_issues
SET status = 'fixed', resolved_at = ?, resolved_in_iteration = ?
WHERE id = ?;

-- name: BulkResolveIssues :exec
UPDATE review_issues
SET status = 'fixed', resolved_at = ?, resolved_in_iteration = ?
WHERE review_id = ? AND id IN (sqlc.slice('issue_ids'));

-- name: CountOpenIssues :one
SELECT COUNT(*) FROM review_issues
WHERE review_id = ? AND status = 'open';

-- name: CountIssuesBySeverity :many
SELECT severity, COUNT(*) as count
FROM review_issues
WHERE review_id = ?
GROUP BY severity;

-- name: CountIssuesByType :many
SELECT issue_type, COUNT(*) as count
FROM review_issues
WHERE review_id = ?
GROUP BY issue_type;

-- name: DeleteReviewIssues :exec
DELETE FROM review_issues WHERE review_id = ?;

-- Aggregate queries for dashboard

-- name: GetReviewStats :one
SELECT
    COUNT(*) as total_reviews,
    SUM(CASE WHEN state = 'approved' THEN 1 ELSE 0 END) as approved,
    SUM(CASE WHEN state IN ('new', 'pending_review', 're_review') THEN 1 ELSE 0 END) as pending,
    SUM(CASE WHEN state = 'under_review' THEN 1 ELSE 0 END) as in_progress,
    SUM(CASE WHEN state = 'changes_requested' THEN 1 ELSE 0 END) as changes_requested
FROM reviews;

-- name: GetReviewStatsForRequester :one
SELECT
    COUNT(*) as total_reviews,
    SUM(CASE WHEN state = 'approved' THEN 1 ELSE 0 END) as approved,
    SUM(CASE WHEN state IN ('new', 'pending_review', 're_review') THEN 1 ELSE 0 END) as pending,
    SUM(CASE WHEN state = 'changes_requested' THEN 1 ELSE 0 END) as changes_requested
FROM reviews
WHERE requester_id = ?;

-- name: GetReviewerStats :one
SELECT
    COUNT(DISTINCT review_id) as reviews_performed,
    SUM(CASE WHEN decision = 'approve' THEN 1 ELSE 0 END) as approvals,
    SUM(CASE WHEN decision = 'request_changes' THEN 1 ELSE 0 END) as change_requests,
    AVG(duration_ms) as avg_duration_ms,
    SUM(cost_usd) as total_cost
FROM review_iterations
WHERE reviewer_id = ?;

-- name: GetAverageIterationsToApproval :one
SELECT AVG(iteration_count) as avg_iterations
FROM (
    SELECT review_id, MAX(iteration_num) as iteration_count
    FROM review_iterations
    WHERE review_id IN (SELECT review_id FROM reviews WHERE state = 'approved')
    GROUP BY review_id
);
