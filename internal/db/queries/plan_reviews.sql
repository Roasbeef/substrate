-- name: CreatePlanReview :one
INSERT INTO plan_reviews (
    plan_review_id, message_id, thread_id,
    requester_id, reviewer_name,
    plan_path, plan_title, plan_summary,
    state, session_id,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPlanReview :one
SELECT * FROM plan_reviews WHERE plan_review_id = ?;

-- name: GetPlanReviewByID :one
SELECT * FROM plan_reviews WHERE id = ?;

-- name: GetPlanReviewByMessage :one
SELECT * FROM plan_reviews WHERE message_id = ?;

-- name: GetPlanReviewByThread :one
SELECT * FROM plan_reviews
WHERE thread_id = ?
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetPlanReviewBySession :one
SELECT * FROM plan_reviews
WHERE session_id = ? AND state = 'pending'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListPlanReviews :many
SELECT * FROM plan_reviews
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListPlanReviewsByState :many
SELECT * FROM plan_reviews
WHERE state = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListPlanReviewsByRequester :many
SELECT * FROM plan_reviews
WHERE requester_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: UpdatePlanReviewState :exec
UPDATE plan_reviews
SET state = ?,
    reviewer_comment = ?,
    reviewed_by = ?,
    updated_at = ?,
    reviewed_at = ?
WHERE plan_review_id = ?;

-- name: DeletePlanReview :exec
DELETE FROM plan_reviews WHERE plan_review_id = ?;
