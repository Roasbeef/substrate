// PlanDiffView renders a visual diff between two plan versions, showing
// added, removed, modified, and unchanged blocks with color-coded styling.

import { useMemo } from 'react';
import { computePlanDiff } from '@/lib/plan-diff-engine.js';
import type { PlanDiffResult } from '@/types/annotations.js';

export interface PlanDiffViewProps {
  oldText: string;
  newText: string;
}

export function PlanDiffView({ oldText, newText }: PlanDiffViewProps) {
  const diff: PlanDiffResult = useMemo(
    () => computePlanDiff(oldText, newText),
    [oldText, newText],
  );

  return (
    <div>
      {/* Stats header. */}
      <div className="mb-4 flex items-center gap-3">
        <span className="text-xs font-medium text-gray-500">
          Plan Changes:
        </span>
        {diff.stats.additions > 0 && (
          <span className="rounded bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">
            +{diff.stats.additions} lines
          </span>
        )}
        {diff.stats.deletions > 0 && (
          <span className="rounded bg-red-100 px-2 py-0.5 text-xs font-medium text-red-800">
            -{diff.stats.deletions} lines
          </span>
        )}
        {diff.stats.modifications > 0 && (
          <span className="rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800">
            ~{diff.stats.modifications} modified
          </span>
        )}
      </div>

      {/* Diff blocks. */}
      <div className="space-y-0">
        {diff.blocks.map((block, idx) => {
          switch (block.type) {
            case 'added':
              return (
                <div
                  key={idx}
                  className="border-l-4 border-green-400 bg-green-50 px-4 py-2"
                >
                  <pre className="whitespace-pre-wrap text-sm text-green-900 font-mono">
                    {block.content}
                  </pre>
                </div>
              );

            case 'removed':
              return (
                <div
                  key={idx}
                  className="border-l-4 border-red-400 bg-red-50 px-4 py-2"
                >
                  <pre className="whitespace-pre-wrap text-sm text-red-900 line-through font-mono">
                    {block.content}
                  </pre>
                </div>
              );

            case 'modified':
              return (
                <div
                  key={idx}
                  className="border-l-4 border-blue-400 bg-blue-50/50 px-4 py-2"
                >
                  {block.oldContent && (
                    <div className="mb-2">
                      <span className="text-[10px] font-semibold uppercase tracking-wider text-red-600">
                        Before
                      </span>
                      <pre className="mt-0.5 whitespace-pre-wrap text-sm text-red-800 line-through font-mono">
                        {block.oldContent}
                      </pre>
                    </div>
                  )}
                  <div>
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-green-600">
                      After
                    </span>
                    <pre className="mt-0.5 whitespace-pre-wrap text-sm text-green-800 font-mono">
                      {block.content}
                    </pre>
                  </div>
                </div>
              );

            case 'unchanged':
              // Collapse large unchanged blocks.
              if (block.lines > 6) {
                return (
                  <details key={idx} className="px-4 py-1">
                    <summary className="cursor-pointer text-xs text-gray-400 hover:text-gray-600">
                      {block.lines} unchanged lines
                    </summary>
                    <pre className="mt-1 whitespace-pre-wrap text-sm text-gray-500 font-mono">
                      {block.content}
                    </pre>
                  </details>
                );
              }
              return (
                <div key={idx} className="px-4 py-1">
                  <pre className="whitespace-pre-wrap text-sm text-gray-500 font-mono">
                    {block.content}
                  </pre>
                </div>
              );

            default:
              return null;
          }
        })}
      </div>
    </div>
  );
}
