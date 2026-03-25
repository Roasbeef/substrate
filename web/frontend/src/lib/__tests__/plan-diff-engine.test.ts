import { describe, it, expect } from 'vitest';
import { computePlanDiff } from '../plan-diff-engine.js';

describe('computePlanDiff', () => {
  it('should detect additions', () => {
    const result = computePlanDiff('line1\n', 'line1\nline2\n');
    expect(result.stats.additions).toBeGreaterThan(0);
    expect(result.blocks.some((b) => b.type === 'added')).toBe(true);
  });

  it('should detect deletions', () => {
    const result = computePlanDiff('line1\nline2\n', 'line1\n');
    expect(result.stats.deletions).toBeGreaterThan(0);
    expect(result.blocks.some((b) => b.type === 'removed')).toBe(true);
  });

  it('should detect modifications (adjacent remove+add)', () => {
    const result = computePlanDiff('old text\n', 'new text\n');
    expect(result.stats.modifications).toBeGreaterThan(0);
    expect(result.blocks.some((b) => b.type === 'modified')).toBe(true);
  });

  it('should detect unchanged content', () => {
    const result = computePlanDiff(
      'same\nchanged\n',
      'same\nmodified\n',
    );
    expect(result.blocks.some((b) => b.type === 'unchanged')).toBe(true);
  });

  it('should return empty diff for identical content', () => {
    const result = computePlanDiff('same content\n', 'same content\n');
    expect(result.stats.additions).toBe(0);
    expect(result.stats.deletions).toBe(0);
    expect(result.stats.modifications).toBe(0);
    expect(result.blocks.every((b) => b.type === 'unchanged')).toBe(true);
  });

  it('should handle empty inputs', () => {
    const result = computePlanDiff('', '');
    expect(result.blocks).toHaveLength(0);
  });

  it('should handle adding content to empty', () => {
    const result = computePlanDiff('', 'new content\n');
    expect(result.stats.additions).toBeGreaterThan(0);
  });

  it('should handle removing all content', () => {
    const result = computePlanDiff('old content\n', '');
    expect(result.stats.deletions).toBeGreaterThan(0);
  });

  it('modified blocks should have oldContent', () => {
    const result = computePlanDiff('before\n', 'after\n');
    const modified = result.blocks.find((b) => b.type === 'modified');
    expect(modified).toBeDefined();
    expect(modified!.oldContent).toBeDefined();
    expect(modified!.content).toContain('after');
  });
});
