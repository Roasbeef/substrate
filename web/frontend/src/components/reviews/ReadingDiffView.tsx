// ReadingDiffView renders a patch abridged down to the rows worth reading,
// with every elided region expandable back in place.
//
// Rendering is delegated to DiffViewer rather than hand-rolled, so a reading
// diff gets the same syntax highlighting, file sidebar, and unified/split
// toggle as a full one. That matters beyond consistency: the reader is
// switching between the two views constantly, and a reading diff that looked
// different would read as a different kind of artifact rather than the same
// patch with less of it.
//
// Expansion works by splicing, not by re-fetching. A reading diff is a
// projection of its input, so the server can say exactly which original lines
// it hid; opening a region rebuilds the patch with those lines restored and
// hands the result back to the same renderer.

import { lazy, Suspense, useCallback, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import {
  fetchReadingDiff,
  type ReadingDiff,
  type ReadingDiffSegment,
} from '@/api/reading-diff.js';
import { Spinner } from '@/components/ui/Spinner.js';

const DiffViewer = lazy(() =>
  import('./DiffViewer.js').then((m) => ({ default: m.DiffViewer })),
);

// autoAbridgeMaxBytes is the largest patch abridged without being asked.
//
// Abridging runs a model over the whole patch, so cost and latency scale with
// size: a few hundred lines returns in seconds, while a forty-file branch diff
// can occupy the model for minutes and may burn a retry before it lands. Above
// this threshold the reader gets the full diff immediately and an explicit
// button, because silently making someone wait minutes for a view they did not
// ask for is worse than showing them the patch they already have.
const autoAbridgeMaxBytes = 60 * 1024;

export interface ReadingDiffViewProps {
  // The raw unified patch to abridge.
  patch: string;
  // Repository path, letting the planner inspect surrounding source.
  repoPath?: string | undefined;
  // Called when the reader switches to the unabridged patch.
  onShowFull?: (() => void) | undefined;
}

// Split a patch into physical lines without a trailing phantom entry, matching
// the server's line numbering so segment coordinates line up.
function splitLines(text: string): string[] {
  const lines = text.split('\n');
  if (lines.length > 0 && lines[lines.length - 1] === '') {
    lines.pop();
  }
  return lines;
}

// A region the abridgement hid, addressed in original patch coordinates.
interface ElidedRegion {
  key: string;
  startLine: number;
  endLine: number;
  lineCount: number;
  // A fold already shows an ellipsis row standing in for these lines, so the
  // reader has a visual cue; a plain removal leaves no trace at all.
  folded: boolean;
}

// Walk the segment map to produce the patch to display and the list of regions
// the reader can open.
//
// Each segment consumes a known number of rows from the abridged text: a kept
// run contributes its own rows, and a fold contributes exactly one ellipsis
// row. Tracking that cursor is what keeps a splice aligned; the server
// guarantees one folded segment per fold so the accounting stays simple.
function buildView(
  raw: string,
  data: ReadingDiff,
  expanded: Set<string>,
): { displayPatch: string; regions: ElidedRegion[] } {
  const rawLines = splitLines(raw);
  const readingLines = splitLines(data.reading_diff);

  const out: string[] = [];
  const regions: ElidedRegion[] = [];
  let cursor = 0;

  const regionKey = (seg: ReadingDiffSegment) =>
    `${seg.kind}-${seg.start_line}-${seg.end_line}`;

  for (const seg of data.segments) {
    const count = seg.end_line - seg.start_line + 1;

    if (seg.kind === 'kept') {
      out.push(...readingLines.slice(cursor, cursor + count));
      cursor += count;
      continue;
    }

    const key = regionKey(seg);
    const isOpen = expanded.has(key);

    regions.push({
      key,
      startLine: seg.start_line,
      endLine: seg.end_line,
      lineCount: count,
      folded: seg.kind === 'folded',
    });

    if (seg.kind === 'folded') {
      if (isOpen) {
        out.push(...rawLines.slice(seg.start_line - 1, seg.end_line));
      } else {
        out.push(...readingLines.slice(cursor, cursor + 1));
      }
      cursor += 1;
      continue;
    }

    if (isOpen) {
      out.push(...rawLines.slice(seg.start_line - 1, seg.end_line));
    }
  }

  return { displayPatch: out.join('\n') + '\n', regions };
}

export function ReadingDiffView({
  patch,
  repoPath,
  onShowFull,
}: ReadingDiffViewProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const oversized = patch.length > autoAbridgeMaxBytes;
  const [requested, setRequested] = useState(!oversized);

  const { data, isLoading, error } = useQuery({
    queryKey: ['reading-diff', patch, repoPath ?? ''],
    queryFn: ({ signal }) => fetchReadingDiff(patch, repoPath, signal),
    enabled: requested,
    // Abridging is expensive and content-addressed, so there is never a reason
    // to refetch the same patch within a session.
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });

  const toggle = useCallback((key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  const view = useMemo(
    () => (data ? buildView(patch, data, expanded) : null),
    [patch, data, expanded],
  );

  const allOpen =
    view !== null &&
    view.regions.length > 0 &&
    view.regions.every((r) => expanded.has(r.key));

  const toggleAll = useCallback(() => {
    if (!view) return;
    setExpanded(allOpen ? new Set() : new Set(view.regions.map((r) => r.key)));
  }, [view, allOpen]);

  // A large patch is not abridged until asked. Show it in full meanwhile, so
  // the reader is never staring at a spinner instead of the diff they opened.
  if (!requested) {
    return (
      <div data-testid="reading-diff-optional">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-[var(--c-hair)] px-3 py-2">
          <span className="text-[12px] text-[var(--c-ink-3)]">
            {Math.round(patch.length / 1024)} KB of diff. Abridging one this
            large can take a few minutes.
          </span>
          <button
            type="button"
            data-testid="reading-diff-request"
            onClick={() => setRequested(true)}
            className="ml-auto text-[12px] font-medium text-[var(--c-steel)] hover:underline"
          >
            Abridge it
          </button>
        </div>
        <Suspense
          fallback={
            <div className="flex justify-center p-4">
              <Spinner size="sm" />
            </div>
          }
        >
          <DiffViewer patch={patch} initialStyle="unified" />
        </Suspense>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="p-4 text-[13px] text-[var(--c-ink-3)]">
        <div className="flex items-center gap-2">
          <Spinner size="sm" />
          <span>
            Abridging {Math.round(patch.length / 1024)} KB of diff. A model is
            reading the whole patch, which can take a few minutes.
          </span>
        </div>
        <button
          type="button"
          onClick={() => {
            setRequested(false);
            if (onShowFull) onShowFull();
          }}
          className="mt-1.5 font-medium text-[var(--c-steel)] hover:underline"
        >
          Stop waiting and show the full diff
        </button>
      </div>
    );
  }

  if (error || !data || !view) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    return (
      <div className="p-3 text-[13px]">
        <p className="text-[var(--c-rust)]">
          Could not build a reading diff: {message}
        </p>
        {onShowFull && (
          <button
            type="button"
            onClick={onShowFull}
            className="mt-1 font-medium text-[var(--c-steel)] hover:underline"
          >
            Show the full diff
          </button>
        )}
      </div>
    );
  }

  return (
    <div data-testid="reading-diff">
      <div className="border-b border-[var(--c-hair)] px-3 py-2">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="text-[13px] font-medium text-[var(--c-ink-1)]">
            {data.summary}
          </span>
          <span
            data-testid="reading-diff-elision"
            className="text-[12px] text-[var(--c-ink-3)]"
          >
            {data.elision}
          </span>
          {onShowFull && (
            <button
              type="button"
              onClick={onShowFull}
              className="ml-auto text-[12px] font-medium text-[var(--c-steel)] hover:underline"
            >
              Full diff
            </button>
          )}
        </div>

        {view.regions.length > 0 && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            <span className="text-[11px] uppercase tracking-wide text-[var(--c-ink-3)]">
              elided
            </span>
            {view.regions.map((region) => {
              const isOpen = expanded.has(region.key);
              return (
                <button
                  key={region.key}
                  type="button"
                  data-testid="reading-diff-expand"
                  onClick={() => toggle(region.key)}
                  title={`Original lines ${region.startLine}–${region.endLine}`}
                  className={`rounded border px-1.5 py-0.5 text-[11px] transition-colors ${
                    isOpen
                      ? 'border-[var(--c-steel)] text-[var(--c-steel)]'
                      : 'border-[var(--c-hair)] text-[var(--c-ink-3)] hover:border-[var(--c-steel)]'
                  }`}
                >
                  {isOpen ? '▾' : '▸'} {region.lineCount}
                  {region.folded ? ' folded' : ''}
                </button>
              );
            })}
            <button
              type="button"
              data-testid="reading-diff-expand-all"
              onClick={toggleAll}
              className="ml-1 text-[11px] font-medium text-[var(--c-steel)] hover:underline"
            >
              {allOpen ? 'collapse all' : 'expand all'}
            </button>
          </div>
        )}
      </div>

      <Suspense
        fallback={
          <div className="flex justify-center p-4">
            <Spinner size="sm" />
          </div>
        }
      >
        <DiffViewer patch={view.displayPatch} initialStyle="unified" />
      </Suspense>
    </div>
  );
}
