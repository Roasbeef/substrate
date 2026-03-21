// Annotation helpers for table of contents and section-level counting.
// Ported from plannotator's annotationHelpers.ts.

import type { Block, PlanAnnotation } from '@/types/annotations.js';

// TocItem represents a single entry in the hierarchical table of contents.
export interface TocItem {
  id: string;
  content: string;
  level: number;
  order: number;
  children: TocItem[];
  annotationCount: number;
}

// getAnnotationCountBySection calculates annotation counts per heading
// section. A section includes all blocks from a heading until the next
// heading of the same or higher level.
export function getAnnotationCountBySection(
  blocks: Block[],
  annotations: PlanAnnotation[],
): Map<string, number> {
  const counts = new Map<string, number>();

  const headings = blocks.filter(
    (b) => b.type === 'heading' && (b.level ?? 0) <= 3,
  );

  if (headings.length === 0) return counts;

  for (let i = 0; i < headings.length; i++) {
    const heading = headings[i]!;
    const currentLevel = heading.level ?? 1;
    const startLine = heading.startLine;

    // Find the end of this section (next heading of same or higher level).
    let endLine = Infinity;
    for (let j = i + 1; j < headings.length; j++) {
      const nextHeading = headings[j]!;
      const nextLevel = nextHeading.level ?? 1;
      if (nextLevel <= currentLevel) {
        endLine = nextHeading.startLine;
        break;
      }
    }

    // Count annotations in blocks within this section.
    let count = 0;
    for (const block of blocks) {
      if (block.startLine >= startLine && block.startLine < endLine) {
        const blockAnnotations = annotations.filter(
          (a) => a.blockId === block.id,
        );
        count += blockAnnotations.length;
      }
    }

    counts.set(heading.id, count);
  }

  return counts;
}

// buildTocHierarchy constructs a hierarchical TOC tree from a flat blocks
// array and annotation counts per section.
export function buildTocHierarchy(
  blocks: Block[],
  annotationCounts: Map<string, number>,
): TocItem[] {
  const headings = blocks
    .filter((b) => b.type === 'heading' && (b.level ?? 0) <= 3)
    .sort((a, b) => a.order - b.order);

  const root: TocItem[] = [];
  const stack: TocItem[] = [];

  for (const heading of headings) {
    const item: TocItem = {
      id: heading.id,
      content: heading.content,
      level: heading.level ?? 1,
      order: heading.order,
      children: [],
      annotationCount: annotationCounts.get(heading.id) ?? 0,
    };

    // Find the correct parent based on heading level.
    while (stack.length > 0) {
      const top = stack[stack.length - 1]!;
      if (top.level >= item.level) {
        stack.pop();
      } else {
        break;
      }
    }

    if (stack.length === 0) {
      root.push(item);
    } else {
      stack[stack.length - 1]!.children.push(item);
    }

    stack.push(item);
  }

  return root;
}
