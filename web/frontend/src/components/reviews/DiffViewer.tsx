// DiffViewer component - renders unified/split diff views using @pierre/diffs.
// Supports inline and fullscreen modes with a sticky toolbar and file sidebar.

import {
  useState, useMemo, useCallback, useEffect, useRef,
} from 'react';
import { FileDiff } from '@pierre/diffs/react';
import { parsePatchFiles } from '@pierre/diffs';

export interface DiffViewerProps {
  // The raw unified diff / patch string (git diff output).
  patch: string;
  // Optional CSS class name.
  className?: string;
  // Initial diff style, defaults to 'unified'.
  initialStyle?: 'unified' | 'split';
}

// Extract just the filename from a path (e.g. "b/internal/review/service.go" -> "service.go").
function basename(path: string): string {
  const parts = path.split('/');
  return parts[parts.length - 1] ?? path;
}

// Extract the directory portion of a path (e.g. "internal/review/service.go" -> "internal/review").
function dirname(path: string): string {
  // Strip leading a/ or b/ prefix from git diff paths.
  const clean = path.replace(/^[ab]\//, '');
  const lastSlash = clean.lastIndexOf('/');
  return lastSlash === -1 ? '' : clean.slice(0, lastSlash);
}

// Clean git diff path prefix.
function cleanPath(path: string): string {
  return path.replace(/^[ab]\//, '');
}

// Count additions and deletions for a single file diff.
function countFileChanges(fileDiff: { hunks?: Array<{ lines?: string[] }> }): {
  additions: number;
  deletions: number;
} {
  let additions = 0;
  let deletions = 0;

  if (fileDiff.hunks) {
    for (const hunk of fileDiff.hunks) {
      if (hunk.lines) {
        for (const line of hunk.lines) {
          if (line.startsWith('+') && !line.startsWith('+++')) {
            additions++;
          } else if (line.startsWith('-') && !line.startsWith('---')) {
            deletions++;
          }
        }
      }
    }
  }

  return { additions, deletions };
}

// DiffViewer parses a multi-file patch and renders each file diff.
export function DiffViewer({
  patch,
  className,
  initialStyle = 'unified',
}: DiffViewerProps) {
  const [diffStyle, setDiffStyle] = useState<'unified' | 'split'>(
    initialStyle,
  );
  const [copied, setCopied] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [activeFileIdx, setActiveFileIdx] = useState(0);

  // Refs for scroll-to-file navigation.
  const fileRefs = useRef<Map<number, HTMLDivElement>>(new Map());
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const options = useMemo(() => ({
    diffStyle,
    theme: 'pierre-dark' as const,
    diffIndicators: 'classic' as const,
    disableLineNumbers: false,
  }), [diffStyle]);

  // Parse the full patch into individual file diffs.
  const fileDiffs = useMemo(() => {
    const parsed = parsePatchFiles(patch);
    return parsed.flatMap((p) => p.files);
  }, [patch]);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(patch).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [patch]);

  // Toggle fullscreen mode.
  const toggleFullscreen = useCallback(() => {
    setFullscreen((prev) => !prev);
  }, []);

  // Scroll to a specific file diff by index.
  const scrollToFile = useCallback((idx: number) => {
    const el = fileRefs.current.get(idx);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      setActiveFileIdx(idx);
    }
  }, []);

  // Track which file is currently visible via IntersectionObserver.
  useEffect(() => {
    if (!fullscreen || !scrollContainerRef.current) return;

    const observer = new IntersectionObserver(
      (entries) => {
        // Find the topmost visible file diff.
        for (const entry of entries) {
          if (entry.isIntersecting) {
            const idx = Number(entry.target.getAttribute('data-file-idx'));
            if (!isNaN(idx)) {
              setActiveFileIdx(idx);
              break;
            }
          }
        }
      },
      {
        root: scrollContainerRef.current,
        rootMargin: '-10% 0px -80% 0px',
        threshold: 0,
      },
    );

    // Observe all file diff elements.
    for (const [, el] of fileRefs.current) {
      observer.observe(el);
    }

    return () => observer.disconnect();
  }, [fullscreen, fileDiffs]);

  // Close fullscreen on Escape key.
  useEffect(() => {
    if (!fullscreen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setFullscreen(false);
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    // Prevent body scroll while fullscreen.
    document.body.style.overflow = 'hidden';

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = '';
    };
  }, [fullscreen]);

  // Shared toolbar rendered in both inline and fullscreen modes.
  const toolbar = (
    <div
      className={`sticky top-0 z-10 flex items-center justify-between border-b px-3 py-2 ${
        fullscreen
          ? 'border-gray-700 bg-gray-900'
          : 'border-gray-200 bg-white'
      }`}
    >
      <div className="flex items-center gap-3">
        {/* Sidebar toggle (fullscreen only). */}
        {fullscreen ? (
          <button
            type="button"
            onClick={() => setSidebarOpen((prev) => !prev)}
            className="rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-800 hover:text-gray-200"
            title={sidebarOpen ? 'Hide file list' : 'Show file list'}
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
        ) : null}

        <div className={`flex items-center gap-1 rounded-lg p-0.5 ${
          fullscreen ? 'bg-gray-800' : 'bg-gray-100'
        }`}>
          <button
            type="button"
            onClick={() => setDiffStyle('unified')}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              diffStyle === 'unified'
                ? fullscreen
                  ? 'bg-gray-700 text-gray-100 shadow-sm'
                  : 'bg-white text-gray-900 shadow-sm'
                : fullscreen
                  ? 'text-gray-400 hover:text-gray-200'
                  : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            Unified
          </button>
          <button
            type="button"
            onClick={() => setDiffStyle('split')}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              diffStyle === 'split'
                ? fullscreen
                  ? 'bg-gray-700 text-gray-100 shadow-sm'
                  : 'bg-white text-gray-900 shadow-sm'
                : fullscreen
                  ? 'text-gray-400 hover:text-gray-200'
                  : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            Split
          </button>
        </div>
        <span className={`text-xs ${fullscreen ? 'text-gray-400' : 'text-gray-500'}`}>
          {fileDiffs.length} file{fileDiffs.length !== 1 ? 's' : ''}
        </span>
      </div>

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={handleCopy}
          className={`flex items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors ${
            fullscreen
              ? 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
              : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700'
          }`}
        >
          {copied ? (
            <>
              <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
              Copied
            </>
          ) : (
            <>
              <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              Copy patch
            </>
          )}
        </button>

        {/* Fullscreen toggle. */}
        <button
          type="button"
          onClick={toggleFullscreen}
          className={`flex items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors ${
            fullscreen
              ? 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
              : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700'
          }`}
          title={fullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen'}
        >
          {fullscreen ? (
            <>
              <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
              Exit
            </>
          ) : (
            <>
              <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
              Fullscreen
            </>
          )}
        </button>
      </div>
    </div>
  );

  // File diffs list with data attributes for intersection observer.
  const fileDiffList = (
    <div className={`space-y-2 ${fullscreen ? 'p-4' : ''}`}>
      {fileDiffs.map((fileDiff, idx) => (
        <div
          key={fileDiff.name ?? fileDiff.prevName ?? idx}
          ref={(el) => {
            if (el) {
              fileRefs.current.set(idx, el);
            } else {
              fileRefs.current.delete(idx);
            }
          }}
          data-file-idx={idx}
          className="overflow-hidden rounded-lg border border-gray-700"
        >
          <FileDiff fileDiff={fileDiff} options={options} />
        </div>
      ))}
    </div>
  );

  // File sidebar for fullscreen mode.
  const sidebar = (
    <div className="flex h-full w-64 flex-shrink-0 flex-col border-r border-gray-700 bg-gray-950">
      <div className="border-b border-gray-700 px-3 py-2">
        <span className="text-xs font-medium uppercase tracking-wider text-gray-500">
          Changed Files
        </span>
      </div>
      <nav className="min-h-0 flex-1 overflow-y-auto py-1">
        {fileDiffs.map((fileDiff, idx) => {
          const name = fileDiff.name ?? fileDiff.prevName ?? `file-${idx}`;
          const dir = dirname(name);
          const file = basename(cleanPath(name));
          const { additions, deletions } = countFileChanges(
            fileDiff as { hunks?: Array<{ lines?: string[] }> },
          );
          const isActive = idx === activeFileIdx;

          return (
            <button
              key={name}
              type="button"
              onClick={() => scrollToFile(idx)}
              className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors ${
                isActive
                  ? 'bg-gray-800 text-gray-100'
                  : 'text-gray-400 hover:bg-gray-900 hover:text-gray-200'
              }`}
              title={cleanPath(name)}
            >
              {/* File icon. */}
              <svg className="h-3.5 w-3.5 flex-shrink-0 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{file}</div>
                {dir ? (
                  <div className="truncate text-[10px] text-gray-600">{dir}</div>
                ) : null}
              </div>
              {/* Change indicators. */}
              <div className="flex flex-shrink-0 items-center gap-1 text-[10px]">
                {additions > 0 ? (
                  <span className="text-green-500">+{additions}</span>
                ) : null}
                {deletions > 0 ? (
                  <span className="text-red-400">-{deletions}</span>
                ) : null}
              </div>
            </button>
          );
        })}
      </nav>
    </div>
  );

  // Fullscreen overlay — fixed position covering the entire viewport.
  if (fullscreen) {
    return (
      <div className="fixed inset-0 z-50 flex flex-col bg-gray-900">
        {toolbar}
        <div className="flex min-h-0 flex-1">
          {/* Sidebar (collapsible). */}
          {sidebarOpen ? sidebar : null}
          {/* Main diff content. */}
          <div
            className="min-h-0 min-w-0 flex-1 overflow-y-auto"
            ref={scrollContainerRef}
          >
            {fileDiffList}
          </div>
        </div>
      </div>
    );
  }

  // Inline mode — sticky toolbar within the nearest scroll container.
  return (
    <div className={className}>
      {toolbar}
      {fileDiffList}
    </div>
  );
}
