-- name: CreatePlanAnnotation :one
INSERT INTO plan_annotations (
    plan_review_id, annotation_id, block_id, annotation_type,
    text, original_text, start_offset, end_offset,
    diff_context, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPlanAnnotation :one
SELECT * FROM plan_annotations WHERE annotation_id = ?;

-- name: ListPlanAnnotationsByReview :many
SELECT * FROM plan_annotations
WHERE plan_review_id = ?
ORDER BY id ASC;

-- name: UpdatePlanAnnotation :exec
UPDATE plan_annotations
SET text = ?, original_text = ?, start_offset = ?,
    end_offset = ?, diff_context = ?, updated_at = ?
WHERE annotation_id = ?;

-- name: DeletePlanAnnotation :exec
DELETE FROM plan_annotations WHERE annotation_id = ?;

-- name: DeletePlanAnnotationsByReview :exec
DELETE FROM plan_annotations WHERE plan_review_id = ?;

-- name: CreateDiffAnnotation :one
INSERT INTO diff_annotations (
    annotation_id, message_id, annotation_type, scope,
    file_path, line_start, line_end, side,
    text, suggested_code, original_code,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetDiffAnnotation :one
SELECT * FROM diff_annotations WHERE annotation_id = ?;

-- name: ListDiffAnnotationsByMessage :many
SELECT * FROM diff_annotations
WHERE message_id = ?
ORDER BY file_path ASC, line_start ASC;

-- name: UpdateDiffAnnotation :exec
UPDATE diff_annotations
SET text = ?, suggested_code = ?, original_code = ?, updated_at = ?
WHERE annotation_id = ?;

-- name: DeleteDiffAnnotation :exec
DELETE FROM diff_annotations WHERE annotation_id = ?;

-- name: DeleteDiffAnnotationsByMessage :exec
DELETE FROM diff_annotations WHERE message_id = ?;
