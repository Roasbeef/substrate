-- Reading diffs table: caches abridged "reading diff" renderings of a patch.
--
-- Abridging asks a model to plan the elision, which costs tokens and takes
-- minutes, so a result is reused whenever the same patch is viewed again. The
-- cache key covers every input that shapes the answer: the patch bytes, the
-- model, and a hash of the generation rubric. Editing the rubric or switching
-- models therefore misses the cache and recomputes, rather than serving output
-- the current code would no longer produce.
CREATE TABLE reading_diffs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- cache_key is sha256(rubric_hash || model || raw_diff), hex encoded.
    cache_key TEXT NOT NULL UNIQUE,

    -- reading_diff is the compiled, abridged patch text.
    reading_diff TEXT NOT NULL,

    -- summary is the generator's one-line description of the change.
    summary TEXT NOT NULL DEFAULT '',

    -- segments is the JSON segment map tiling the original patch lines, which
    -- lets the viewer expand an elided region back in place.
    segments TEXT NOT NULL DEFAULT '[]',

    -- Retention counters are stored as columns rather than only inside the
    -- stats blob so the elision manifest can be rendered without decoding it.
    raw_changed INTEGER NOT NULL DEFAULT 0,
    visible_changed INTEGER NOT NULL DEFAULT 0,
    raw_files INTEGER NOT NULL DEFAULT 0,
    visible_files INTEGER NOT NULL DEFAULT 0,

    -- model records which model produced the plan, for provenance in the UI.
    model TEXT NOT NULL DEFAULT '',

    -- rubric_hash records the protocol version the entry was produced under.
    rubric_hash TEXT NOT NULL DEFAULT '',

    created_at INTEGER NOT NULL
);

-- Lookups are always by cache key; the UNIQUE constraint already indexes it.
-- This index supports pruning the oldest entries when the cache is trimmed.
CREATE INDEX idx_reading_diffs_created ON reading_diffs(created_at);
