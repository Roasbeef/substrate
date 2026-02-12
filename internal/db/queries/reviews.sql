-- name: CreateReview :one
INSERT INTO reviews (
    review_id, thread_id, requester_id,
    pr_number, branch, base_branch, commit_sha, repo_path, remote_url,
    review_type, priority, state,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReview :one
SELECT * FROM reviews WHERE review_id = ?;

-- name: ResolveReviewID :one
SELECT review_id FROM reviews WHERE review_id LIKE ? || '%' LIMIT 1;

-- name: GetReviewByID :one
SELECT * FROM reviews WHERE id = ?;

-- name: ListReviews :many
SELECT * FROM reviews
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListReviewsByState :many
SELECT * FROM reviews
WHERE state = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListReviewsByRequester :many
SELECT * FROM reviews
WHERE requester_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListActiveReviews :many
-- Returns reviews that are in non-terminal states (for restart recovery).
SELECT * FROM reviews
WHERE state NOT IN ('approved', 'rejected', 'cancelled')
ORDER BY created_at DESC;

-- name: UpdateReviewState :exec
UPDATE reviews SET state = ?, updated_at = ? WHERE review_id = ?;

-- name: UpdateReviewCompleted :exec
UPDATE reviews SET state = ?, updated_at = ?, completed_at = ? WHERE review_id = ?;

-- name: CountReviewsByState :one
SELECT COUNT(*) FROM reviews WHERE state = ?;

-- name: CountReviewsByRequester :one
SELECT COUNT(*) FROM reviews WHERE requester_id = ?;

-- name: CreateReviewIteration :one
INSERT INTO review_iterations (
    review_id, iteration_num, reviewer_id, reviewer_session_id,
    decision, summary, issues_json, suggestions_json,
    files_reviewed, lines_analyzed, duration_ms, cost_usd,
    started_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReviewIterations :many
SELECT * FROM review_iterations
WHERE review_id = ?
ORDER BY iteration_num ASC;

-- name: GetReviewIteration :one
SELECT * FROM review_iterations
WHERE review_id = ? AND iteration_num = ? AND reviewer_id = ?;

-- name: GetLatestIteration :one
SELECT * FROM review_iterations
WHERE review_id = ?
ORDER BY iteration_num DESC, completed_at DESC
LIMIT 1;

-- name: CreateReviewIssue :one
INSERT INTO review_issues (
    review_id, iteration_num,
    issue_type, severity,
    file_path, line_start, line_end,
    title, description, code_snippet, suggestion, claude_md_ref,
    status, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetReviewIssues :many
SELECT * FROM review_issues
WHERE review_id = ?
ORDER BY severity ASC, created_at ASC;

-- name: GetReviewIssuesByIteration :many
SELECT * FROM review_issues
WHERE review_id = ? AND iteration_num = ?
ORDER BY severity ASC, created_at ASC;

-- name: GetReviewIssue :one
SELECT * FROM review_issues WHERE id = ?;

-- name: GetOpenReviewIssues :many
SELECT * FROM review_issues
WHERE review_id = ? AND status = 'open'
ORDER BY severity ASC, created_at ASC;

-- name: UpdateReviewIssueStatus :exec
UPDATE review_issues
SET status = ?, resolved_at = ?, resolved_in_iteration = ?
WHERE id = ?;

-- name: CountReviewIssuesByStatus :one
SELECT COUNT(*) FROM review_issues
WHERE review_id = ? AND status = ?;

-- name: CountOpenIssues :one
SELECT COUNT(*) FROM review_issues
WHERE review_id = ? AND status = 'open';

-- name: DeleteReviewIssues :exec
DELETE FROM review_issues WHERE review_id = ?;

-- name: DeleteReviewIterations :exec
DELETE FROM review_iterations WHERE review_id = ?;

-- name: DeleteReview :exec
DELETE FROM reviews WHERE review_id = ?;
