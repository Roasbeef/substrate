// API client for reading diffs: patches abridged down to the rows worth
// reading.

import { post } from './client.js';

// What happened to a run of lines in the original patch. A reading diff is a
// projection of its input, so every original line falls into exactly one of
// these and stays addressable.
export type SegmentKind = 'kept' | 'removed' | 'folded';

// A run of original patch lines and its fate. Coordinates are 1-based and
// inclusive, matching the physical lines of the raw patch.
export interface ReadingDiffSegment {
  kind: SegmentKind;
  start_line: number;
  end_line: number;
}

// Retention counters, computed by the server from the compiled result rather
// than reported by the model.
export interface ReadingDiffStats {
  raw_changed: number;
  visible_changed: number;
  removed_changed: number;
  folded_changed: number;
  fold_count: number;
  raw_files: number;
  visible_files: number;
}

// The abridged patch plus everything needed to render and expand it.
export interface ReadingDiff {
  reading_diff: string;
  summary: string;
  elision: string;
  segments: ReadingDiffSegment[];
  stats: ReadingDiffStats;
}

// Request an abridgement of a patch.
//
// The call can take minutes on a cache miss, because the server runs an agent
// over the whole patch to plan the elision. Results are cached by content, so
// a repeat request for the same patch returns immediately.
export function fetchReadingDiff(
  patch: string,
  repoPath?: string,
  signal?: AbortSignal,
): Promise<ReadingDiff> {
  return post<ReadingDiff>(
    '/reading-diff',
    repoPath ? { patch, repo_path: repoPath } : { patch },
    signal,
  );
}
