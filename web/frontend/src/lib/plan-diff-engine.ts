// Plan diff engine for computing line-level diffs between plan versions.
// Ported from plannotator's planDiffEngine.ts.
// Wraps the `diff` library's diffLines() and groups adjacent add/remove
// changes into "modified" blocks for cleaner rendering.

import { diffLines, type Change } from 'diff';
import type {
  PlanDiffBlock,
  PlanDiffStats,
  PlanDiffResult,
} from '@/types/annotations.js';

// countLines returns the number of lines in a string, handling trailing
// newlines correctly.
function countLines(text: string): number {
  const lines = text.split('\n');
  if (lines.length > 0 && lines[lines.length - 1] === '') {
    return lines.length - 1;
  }
  return lines.length;
}

// computePlanDiff computes the diff between two plan versions. Groups
// consecutive remove+add changes into "modified" blocks for better
// rendering (showing what was replaced rather than separate remove and
// add blocks).
export function computePlanDiff(
  oldText: string,
  newText: string,
): PlanDiffResult {
  const changes: Change[] = diffLines(oldText, newText);

  const blocks: PlanDiffBlock[] = [];
  const stats: PlanDiffStats = {
    additions: 0,
    deletions: 0,
    modifications: 0,
  };

  for (let i = 0; i < changes.length; i++) {
    const change = changes[i]!;
    const next = changes[i + 1];

    if (change.removed && next?.added) {
      // Adjacent remove + add = modification.
      blocks.push({
        type: 'modified',
        content: next.value,
        oldContent: change.value,
        lines: countLines(next.value),
      });
      stats.modifications++;
      stats.additions += countLines(next.value);
      stats.deletions += countLines(change.value);
      i++;
    } else if (change.added) {
      blocks.push({
        type: 'added',
        content: change.value,
        lines: countLines(change.value),
      });
      stats.additions += countLines(change.value);
    } else if (change.removed) {
      blocks.push({
        type: 'removed',
        content: change.value,
        lines: countLines(change.value),
      });
      stats.deletions += countLines(change.value);
    } else {
      blocks.push({
        type: 'unchanged',
        content: change.value,
        lines: countLines(change.value),
      });
    }
  }

  return { blocks, stats };
}
