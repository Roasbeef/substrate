-- name: GetReadingDiffByKey :one
SELECT * FROM reading_diffs
WHERE cache_key = ?;

-- name: CreateReadingDiff :one
INSERT INTO reading_diffs (
    cache_key, reading_diff, summary, segments, stats,
    raw_changed, visible_changed, raw_files, visible_files,
    model, rubric_hash, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(cache_key) DO UPDATE SET
    reading_diff = excluded.reading_diff,
    summary = excluded.summary,
    segments = excluded.segments,
    stats = excluded.stats,
    raw_changed = excluded.raw_changed,
    visible_changed = excluded.visible_changed,
    raw_files = excluded.raw_files,
    visible_files = excluded.visible_files,
    model = excluded.model,
    rubric_hash = excluded.rubric_hash,
    created_at = excluded.created_at
RETURNING *;

-- name: DeleteReadingDiffsBefore :exec
DELETE FROM reading_diffs
WHERE created_at < ?;

-- name: CountReadingDiffs :one
SELECT COUNT(*) FROM reading_diffs;
