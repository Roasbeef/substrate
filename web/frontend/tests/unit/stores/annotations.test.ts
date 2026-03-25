import { describe, it, expect, beforeEach } from 'vitest';
import { useAnnotationStore } from '@/stores/annotations.js';
import { PlanAnnotationType } from '@/types/annotations.js';

describe('useAnnotationStore', () => {
  beforeEach(() => {
    // Reset the store before each test.
    useAnnotationStore.getState().reset();
  });

  describe('plan annotations', () => {
    it('should add a plan annotation', () => {
      const store = useAnnotationStore.getState();
      store.setPlanReviewId('pr-001');

      const ann = store.addPlanAnnotation({
        blockId: 'block-0',
        type: PlanAnnotationType.COMMENT,
        text: 'needs work',
        originalText: 'vague section',
        startOffset: 0,
        endOffset: 13,
      });

      expect(ann.id).toMatch(/^[0-9a-f]{8}-/);
      expect(ann.type).toBe(PlanAnnotationType.COMMENT);
      expect(ann.text).toBe('needs work');

      const { planAnnotations } = useAnnotationStore.getState();
      expect(planAnnotations).toHaveLength(1);
      expect(planAnnotations[0]!.id).toBe(ann.id);
    });

    it('should update a plan annotation', () => {
      const store = useAnnotationStore.getState();
      const ann = store.addPlanAnnotation({
        blockId: 'block-0',
        type: PlanAnnotationType.COMMENT,
        text: 'original',
        originalText: 'text',
        startOffset: 0,
        endOffset: 4,
      });

      store.updatePlanAnnotation(ann.id, { text: 'updated' });

      const { planAnnotations } = useAnnotationStore.getState();
      expect(planAnnotations[0]!.text).toBe('updated');
    });

    it('should delete a plan annotation', () => {
      const store = useAnnotationStore.getState();
      const ann = store.addPlanAnnotation({
        blockId: 'block-0',
        type: PlanAnnotationType.DELETION,
        originalText: 'remove me',
        startOffset: 0,
        endOffset: 9,
      });

      store.deletePlanAnnotation(ann.id);

      const { planAnnotations } = useAnnotationStore.getState();
      expect(planAnnotations).toHaveLength(0);
    });

    it('should clear selection when selected annotation is deleted', () => {
      const store = useAnnotationStore.getState();
      const ann = store.addPlanAnnotation({
        blockId: 'block-0',
        type: PlanAnnotationType.COMMENT,
        text: 'test',
        originalText: 'text',
        startOffset: 0,
        endOffset: 4,
      });

      store.selectPlanAnnotation(ann.id);
      expect(useAnnotationStore.getState().selectedPlanAnnotationId).toBe(
        ann.id,
      );

      store.deletePlanAnnotation(ann.id);
      expect(
        useAnnotationStore.getState().selectedPlanAnnotationId,
      ).toBeNull();
    });

    it('should select and deselect annotations', () => {
      const store = useAnnotationStore.getState();
      store.selectPlanAnnotation('some-id');
      expect(
        useAnnotationStore.getState().selectedPlanAnnotationId,
      ).toBe('some-id');

      store.selectPlanAnnotation(null);
      expect(
        useAnnotationStore.getState().selectedPlanAnnotationId,
      ).toBeNull();
    });
  });

  describe('diff annotations', () => {
    it('should add a diff annotation', () => {
      const store = useAnnotationStore.getState();
      store.setDiffMessageId(42);

      const ann = store.addDiffAnnotation({
        type: 'comment',
        scope: 'line',
        filePath: 'main.go',
        lineStart: 10,
        lineEnd: 12,
        side: 'new',
        text: 'fragile logic',
      });

      expect(ann.id).toMatch(/^[0-9a-f]{8}-/);
      expect(ann.type).toBe('comment');
      expect(ann.filePath).toBe('main.go');

      const { diffAnnotations } = useAnnotationStore.getState();
      expect(diffAnnotations).toHaveLength(1);
    });

    it('should update a diff annotation', () => {
      const store = useAnnotationStore.getState();
      const ann = store.addDiffAnnotation({
        type: 'suggestion',
        scope: 'line',
        filePath: 'main.go',
        lineStart: 5,
        lineEnd: 5,
        side: 'new',
        text: 'refactor',
        suggestedCode: 'func better() {}',
      });

      store.updateDiffAnnotation(ann.id, {
        text: 'improved suggestion',
        suggestedCode: 'func optimal() {}',
      });

      const { diffAnnotations } = useAnnotationStore.getState();
      expect(diffAnnotations[0]!.text).toBe('improved suggestion');
      expect(diffAnnotations[0]!.suggestedCode).toBe(
        'func optimal() {}',
      );
    });

    it('should delete a diff annotation', () => {
      const store = useAnnotationStore.getState();
      const ann = store.addDiffAnnotation({
        type: 'concern',
        scope: 'line',
        filePath: 'main.go',
        lineStart: 1,
        lineEnd: 1,
        side: 'old',
        text: 'race condition',
      });

      store.deleteDiffAnnotation(ann.id);

      const { diffAnnotations } = useAnnotationStore.getState();
      expect(diffAnnotations).toHaveLength(0);
    });
  });

  describe('reset', () => {
    it('should clear all state', () => {
      const store = useAnnotationStore.getState();
      store.setPlanReviewId('pr-001');
      store.setDiffMessageId(42);
      store.addPlanAnnotation({
        blockId: 'b',
        type: PlanAnnotationType.COMMENT,
        text: 't',
        originalText: 'o',
        startOffset: 0,
        endOffset: 1,
      });
      store.addDiffAnnotation({
        type: 'comment',
        scope: 'line',
        filePath: 'f',
        lineStart: 1,
        lineEnd: 1,
        side: 'new',
        text: 't',
      });

      store.reset();

      const state = useAnnotationStore.getState();
      expect(state.planAnnotations).toHaveLength(0);
      expect(state.diffAnnotations).toHaveLength(0);
      expect(state.planReviewId).toBeNull();
      expect(state.diffMessageId).toBeNull();
      expect(state.selectedPlanAnnotationId).toBeNull();
      expect(state.selectedDiffAnnotationId).toBeNull();
    });
  });
});
