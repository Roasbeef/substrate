import { describe, it, expect } from 'vitest';
import {
  getAnnotationCountBySection,
  buildTocHierarchy,
} from '../annotation-helpers.js';
import type { Block, PlanAnnotation } from '@/types/annotations.js';
import { PlanAnnotationType } from '@/types/annotations.js';

function makeBlock(overrides: Partial<Block> & { id: string }): Block {
  return {
    type: 'paragraph',
    content: 'text',
    order: 0,
    startLine: 1,
    ...overrides,
  };
}

function makeAnnotation(
  overrides: Partial<PlanAnnotation> & { id: string; blockId: string },
): PlanAnnotation {
  return {
    type: PlanAnnotationType.COMMENT,
    originalText: 'text',
    startOffset: 0,
    endOffset: 4,
    createdAt: Date.now(),
    updatedAt: Date.now(),
    ...overrides,
  };
}

describe('getAnnotationCountBySection', () => {
  it('should count annotations per heading section', () => {
    const blocks: Block[] = [
      makeBlock({ id: 'h1', type: 'heading', level: 1, startLine: 1, order: 1 }),
      makeBlock({ id: 'p1', startLine: 2, order: 2 }),
      makeBlock({ id: 'h2', type: 'heading', level: 2, startLine: 3, order: 3 }),
      makeBlock({ id: 'p2', startLine: 4, order: 4 }),
    ];
    const annotations: PlanAnnotation[] = [
      makeAnnotation({ id: 'a1', blockId: 'p1' }),
      makeAnnotation({ id: 'a2', blockId: 'p2' }),
      makeAnnotation({ id: 'a3', blockId: 'p2' }),
    ];

    const counts = getAnnotationCountBySection(blocks, annotations);
    // h1 section includes h2 subsection (hierarchical), so all 3 annotations.
    expect(counts.get('h1')).toBe(3);
    expect(counts.get('h2')).toBe(2);
  });

  it('should return empty map when no headings', () => {
    const blocks: Block[] = [
      makeBlock({ id: 'p1', startLine: 1, order: 1 }),
    ];
    const counts = getAnnotationCountBySection(blocks, []);
    expect(counts.size).toBe(0);
  });
});

describe('buildTocHierarchy', () => {
  it('should build a flat list for same-level headings', () => {
    const blocks: Block[] = [
      makeBlock({ id: 'h1', type: 'heading', level: 1, order: 1 }),
      makeBlock({ id: 'h2', type: 'heading', level: 1, order: 2 }),
    ];
    const counts = new Map<string, number>();
    const toc = buildTocHierarchy(blocks, counts);
    expect(toc).toHaveLength(2);
    expect(toc[0]!.children).toHaveLength(0);
  });

  it('should nest child headings under parents', () => {
    const blocks: Block[] = [
      makeBlock({ id: 'h1', type: 'heading', level: 1, order: 1, content: 'Top' }),
      makeBlock({ id: 'h2', type: 'heading', level: 2, order: 2, content: 'Sub' }),
      makeBlock({ id: 'h3', type: 'heading', level: 3, order: 3, content: 'SubSub' }),
    ];
    const counts = new Map<string, number>();
    const toc = buildTocHierarchy(blocks, counts);
    expect(toc).toHaveLength(1);
    expect(toc[0]!.children).toHaveLength(1);
    expect(toc[0]!.children[0]!.children).toHaveLength(1);
  });

  it('should include annotation counts', () => {
    const blocks: Block[] = [
      makeBlock({ id: 'h1', type: 'heading', level: 1, order: 1 }),
    ];
    const counts = new Map([['h1', 5]]);
    const toc = buildTocHierarchy(blocks, counts);
    expect(toc[0]!.annotationCount).toBe(5);
  });
});
