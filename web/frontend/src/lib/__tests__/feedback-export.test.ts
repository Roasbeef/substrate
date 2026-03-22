import { describe, it, expect } from 'vitest';
import {
  exportPlanAnnotations,
  exportDiffAnnotations,
  wrapFeedbackForAgent,
} from '../feedback-export.js';
import type { Block, PlanAnnotation, DiffAnnotation } from '@/types/annotations.js';
import { PlanAnnotationType } from '@/types/annotations.js';

function makeBlock(id: string, order: number): Block {
  return {
    id,
    type: 'paragraph',
    content: 'text',
    order,
    startLine: order,
  };
}

describe('exportPlanAnnotations', () => {
  it('should return no-changes message for empty annotations', () => {
    const result = exportPlanAnnotations([], []);
    expect(result).toBe('No changes detected.');
  });

  it('should format DELETION annotations', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.DELETION,
      originalText: 'remove me',
      startOffset: 0,
      endOffset: 9,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).toContain('Remove this');
    expect(result).toContain('remove me');
  });

  it('should format COMMENT annotations', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.COMMENT,
      text: 'needs more detail',
      originalText: 'vague section',
      startOffset: 0,
      endOffset: 13,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).toContain('Feedback on');
    expect(result).toContain('needs more detail');
  });

  it('should format REPLACEMENT annotations', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.REPLACEMENT,
      text: 'new text',
      originalText: 'old text',
      startOffset: 0,
      endOffset: 8,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).toContain('Change this');
    expect(result).toContain('old text');
    expect(result).toContain('new text');
  });

  it('should include diff context label when present', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.COMMENT,
      text: 'test',
      originalText: 'text',
      diffContext: 'added',
      startOffset: 0,
      endOffset: 4,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).toContain('[In diff content]');
  });
});

describe('exportDiffAnnotations', () => {
  it('should return no-feedback message for empty annotations', () => {
    const result = exportDiffAnnotations([]);
    expect(result).toContain('No feedback provided');
  });

  it('should group annotations by file', () => {
    const annotations: DiffAnnotation[] = [
      {
        id: 'a1',
        type: 'comment',
        scope: 'line',
        filePath: 'main.go',
        lineStart: 10,
        lineEnd: 10,
        side: 'new',
        text: 'looks good',
        createdAt: Date.now(),
        updatedAt: Date.now(),
      },
      {
        id: 'a2',
        type: 'suggestion',
        scope: 'line',
        filePath: 'utils.go',
        lineStart: 5,
        lineEnd: 8,
        side: 'new',
        text: 'refactor this',
        suggestedCode: 'func better() {}',
        createdAt: Date.now(),
        updatedAt: Date.now(),
      },
    ];
    const result = exportDiffAnnotations(annotations);
    expect(result).toContain('## main.go');
    expect(result).toContain('## utils.go');
    expect(result).toContain('looks good');
    expect(result).toContain('Suggested code');
    expect(result).toContain('func better() {}');
  });

  it('should handle file-level comments', () => {
    const annotations: DiffAnnotation[] = [{
      id: 'a1',
      type: 'comment',
      scope: 'file',
      filePath: 'main.go',
      lineStart: 0,
      lineEnd: 0,
      side: 'new',
      text: 'overall this file needs refactoring',
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportDiffAnnotations(annotations);
    expect(result).toContain('File Comment');
    expect(result).toContain('overall this file needs refactoring');
  });
});

describe('security: code fence escaping', () => {
  it('should escape triple backticks in plan annotations', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.DELETION,
      originalText: 'line with ``` backticks',
      startOffset: 0,
      endOffset: 22,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    // Should not contain raw triple backticks inside code fences.
    const fenceContent = result.split('```\n')[1]!;
    expect(fenceContent).not.toContain('```');
  });

  it('should escape quadruple backticks', () => {
    const blocks = [makeBlock('b1', 1)];
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.REPLACEMENT,
      text: 'new text with ```` four backticks',
      originalText: 'old text',
      startOffset: 0,
      endOffset: 8,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).not.toMatch(/````/);
  });

  it('should sanitize file paths with newlines in diff export', () => {
    const annotations: DiffAnnotation[] = [{
      id: 'a1',
      type: 'comment',
      scope: 'line',
      filePath: 'main.go\n## Injected Heading',
      lineStart: 1,
      lineEnd: 1,
      side: 'new',
      text: 'test',
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportDiffAnnotations(annotations);
    // Newline in path should be replaced, not create a new heading.
    // The header "# Code Review Feedback" + one sanitized "## file" = 1 ##.
    const headings = result.split('\n').filter(l => l.startsWith('## '));
    expect(headings.length).toBe(1);
    // The injected heading should not appear.
    expect(result).not.toContain('## Injected Heading');
  });

  it('should truncate excessively long annotation text', () => {
    const blocks = [makeBlock('b1', 1)];
    const longText = 'x'.repeat(20000);
    const annotations: PlanAnnotation[] = [{
      id: 'a1',
      blockId: 'b1',
      type: PlanAnnotationType.COMMENT,
      text: longText,
      originalText: 'short',
      startOffset: 0,
      endOffset: 5,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }];
    const result = exportPlanAnnotations(blocks, annotations);
    expect(result).toContain('[...truncated]');
  });
});

describe('wrapFeedbackForAgent', () => {
  it('should include directive preamble', () => {
    const result = wrapFeedbackForAgent('Fix the bugs');
    expect(result).toContain('YOUR PLAN WAS NOT APPROVED');
    expect(result).toContain('You MUST revise the plan');
    expect(result).toContain('Fix the bugs');
  });

  it('should include the tool name', () => {
    const result = wrapFeedbackForAgent('test', 'CustomTool');
    expect(result).toContain('CustomTool');
  });

  it('should use default message when feedback is empty', () => {
    const result = wrapFeedbackForAgent('');
    expect(result).toContain('Plan changes requested');
  });
});
