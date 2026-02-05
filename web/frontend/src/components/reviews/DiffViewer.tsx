// DiffViewer component - renders unified/split diff views using @pierre/diffs.

import { useState, useMemo, useCallback } from 'react';
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

  return (
    <div className={className}>
      {/* Toolbar. */}
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-1 rounded-lg bg-gray-100 p-0.5">
          <button
            type="button"
            onClick={() => setDiffStyle('unified')}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              diffStyle === 'unified'
                ? 'bg-white text-gray-900 shadow-sm'
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
                ? 'bg-white text-gray-900 shadow-sm'
                : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            Split
          </button>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">
            {fileDiffs.length} file{fileDiffs.length !== 1 ? 's' : ''}
          </span>
          <button
            type="button"
            onClick={handleCopy}
            className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-gray-500 hover:bg-gray-100 hover:text-gray-700"
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
        </div>
      </div>

      {/* File diffs. */}
      <div className="space-y-2">
        {fileDiffs.map((fileDiff, idx) => (
          <div
            key={fileDiff.name ?? fileDiff.prevName ?? idx}
            className="overflow-hidden rounded-lg border border-gray-700"
          >
            <FileDiff fileDiff={fileDiff} options={options} />
          </div>
        ))}
      </div>
    </div>
  );
}
