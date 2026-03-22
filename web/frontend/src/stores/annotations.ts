// Annotation store for managing plan and diff annotations using Zustand.
// Provides CRUD operations with localStorage draft caching and server sync.

import { create } from 'zustand';
import { devtools } from 'zustand/middleware';
import type { PlanAnnotation, DiffAnnotation } from '@/types/annotations.js';
import * as api from '@/api/annotations.js';

// Debounce timer IDs for draft persistence.
let planDraftTimer: ReturnType<typeof setTimeout> | null = null;
let diffDraftTimer: ReturnType<typeof setTimeout> | null = null;
const DRAFT_DEBOUNCE_MS = 300;

// draftKey returns the localStorage key for draft persistence.
function draftKey(kind: 'plan' | 'diff', id: string): string {
  return `annotation-draft-${kind}-${id}`;
}

interface AnnotationState {
  // Plan annotations for the currently viewed plan review.
  planAnnotations: PlanAnnotation[];
  selectedPlanAnnotationId: string | null;
  planReviewId: string | null;

  // Diff annotations for the currently viewed diff message.
  diffAnnotations: DiffAnnotation[];
  selectedDiffAnnotationId: string | null;
  diffMessageId: number | null;

  // Plan annotation actions.
  setPlanReviewId: (id: string) => void;
  loadPlanAnnotations: (annotations: PlanAnnotation[]) => void;
  addPlanAnnotation: (
    params: Omit<PlanAnnotation, 'id' | 'createdAt' | 'updatedAt'>,
  ) => PlanAnnotation;
  updatePlanAnnotation: (
    id: string, updates: Partial<PlanAnnotation>,
  ) => void;
  deletePlanAnnotation: (id: string) => void;
  selectPlanAnnotation: (id: string | null) => void;

  // Diff annotation actions.
  setDiffMessageId: (id: number) => void;
  loadDiffAnnotations: (annotations: DiffAnnotation[]) => void;
  addDiffAnnotation: (
    params: Omit<DiffAnnotation, 'id' | 'createdAt' | 'updatedAt'>,
  ) => DiffAnnotation;
  updateDiffAnnotation: (
    id: string, updates: Partial<DiffAnnotation>,
  ) => void;
  deleteDiffAnnotation: (id: string) => void;
  selectDiffAnnotation: (id: string | null) => void;

  // Draft persistence.
  savePlanDraft: () => void;
  loadPlanDraft: (planReviewId: string) => void;
  saveDiffDraft: () => void;
  loadDiffDraft: (messageId: number) => void;

  // Bulk reset.
  reset: () => void;
}

export const useAnnotationStore = create<AnnotationState>()(
  devtools(
    (set, get) => ({
      // Initial state.
      planAnnotations: [],
      selectedPlanAnnotationId: null,
      planReviewId: null,
      diffAnnotations: [],
      selectedDiffAnnotationId: null,
      diffMessageId: null,

      // Plan annotation actions.
      setPlanReviewId: (id) =>
        set({ planReviewId: id }, undefined, 'setPlanReviewId'),

      loadPlanAnnotations: (annotations) =>
        set(
          { planAnnotations: annotations },
          undefined,
          'loadPlanAnnotations',
        ),

      addPlanAnnotation: (params) => {
        const now = Date.now();
        const annotationId = crypto.randomUUID();
        const annotation: PlanAnnotation = {
          ...params,
          id: annotationId,
          createdAt: now,
          updatedAt: now,
        };
        set(
          (state) => ({
            planAnnotations: [...state.planAnnotations, annotation],
          }),
          undefined,
          'addPlanAnnotation',
        );
        // Sync to server in background.
        const { planReviewId } = get();
        if (planReviewId) {
          api.createPlanAnnotation(planReviewId, {
            annotationId,
            blockId: params.blockId,
            annotationType: params.type,
            ...(params.text !== undefined ? { text: params.text } : {}),
            originalText: params.originalText,
            startOffset: params.startOffset,
            endOffset: params.endOffset,
            ...(params.diffContext !== undefined ? { diffContext: params.diffContext } : {}),
          }).catch(() => {
            // Server sync failed; localStorage draft is the fallback.
          });
        }
        if (planDraftTimer) clearTimeout(planDraftTimer);
        planDraftTimer = setTimeout(() => get().savePlanDraft(), DRAFT_DEBOUNCE_MS);
        return annotation;
      },

      updatePlanAnnotation: (id, updates) => {
        set(
          (state) => ({
            planAnnotations: state.planAnnotations.map((a) =>
              a.id === id
                ? { ...a, ...updates, updatedAt: Date.now() }
                : a,
            ),
          }),
          undefined,
          'updatePlanAnnotation',
        );
        // Sync to server.
        const ann = get().planAnnotations.find((a) => a.id === id);
        if (ann) {
          api.updatePlanAnnotation(id, {
            ...(ann.text !== undefined ? { text: ann.text } : {}),
            originalText: ann.originalText,
            startOffset: ann.startOffset,
            endOffset: ann.endOffset,
            ...(ann.diffContext !== undefined ? { diffContext: ann.diffContext } : {}),
          }).catch(() => {});
        }
        if (planDraftTimer) clearTimeout(planDraftTimer);
        planDraftTimer = setTimeout(() => get().savePlanDraft(), DRAFT_DEBOUNCE_MS);
      },

      deletePlanAnnotation: (id) => {
        set(
          (state) => ({
            planAnnotations: state.planAnnotations.filter(
              (a) => a.id !== id,
            ),
            selectedPlanAnnotationId:
              state.selectedPlanAnnotationId === id
                ? null
                : state.selectedPlanAnnotationId,
          }),
          undefined,
          'deletePlanAnnotation',
        );
        // Sync to server.
        api.deletePlanAnnotation(id).catch(() => {});
        if (planDraftTimer) clearTimeout(planDraftTimer);
        planDraftTimer = setTimeout(() => get().savePlanDraft(), DRAFT_DEBOUNCE_MS);
      },

      selectPlanAnnotation: (id) =>
        set(
          { selectedPlanAnnotationId: id },
          undefined,
          'selectPlanAnnotation',
        ),

      // Diff annotation actions.
      setDiffMessageId: (id) =>
        set({ diffMessageId: id }, undefined, 'setDiffMessageId'),

      loadDiffAnnotations: (annotations) =>
        set(
          { diffAnnotations: annotations },
          undefined,
          'loadDiffAnnotations',
        ),

      addDiffAnnotation: (params) => {
        const now = Date.now();
        const annotationId = crypto.randomUUID();
        const annotation: DiffAnnotation = {
          ...params,
          id: annotationId,
          createdAt: now,
          updatedAt: now,
        };
        set(
          (state) => ({
            diffAnnotations: [...state.diffAnnotations, annotation],
          }),
          undefined,
          'addDiffAnnotation',
        );
        // Sync to server.
        const { diffMessageId } = get();
        if (diffMessageId) {
          api.createDiffAnnotation(diffMessageId, {
            annotationId,
            annotationType: params.type,
            scope: params.scope,
            filePath: params.filePath,
            lineStart: params.lineStart,
            lineEnd: params.lineEnd,
            side: params.side,
            ...(params.text !== undefined ? { text: params.text } : {}),
            ...(params.suggestedCode !== undefined ? { suggestedCode: params.suggestedCode } : {}),
            ...(params.originalCode !== undefined ? { originalCode: params.originalCode } : {}),
          }).catch(() => {});
        }
        if (diffDraftTimer) clearTimeout(diffDraftTimer);
        diffDraftTimer = setTimeout(() => get().saveDiffDraft(), DRAFT_DEBOUNCE_MS);
        return annotation;
      },

      updateDiffAnnotation: (id, updates) => {
        set(
          (state) => ({
            diffAnnotations: state.diffAnnotations.map((a) =>
              a.id === id
                ? { ...a, ...updates, updatedAt: Date.now() }
                : a,
            ),
          }),
          undefined,
          'updateDiffAnnotation',
        );
        // Sync to server.
        const ann = get().diffAnnotations.find((a) => a.id === id);
        if (ann) {
          api.updateDiffAnnotation(id, {
            ...(ann.text !== undefined ? { text: ann.text } : {}),
            ...(ann.suggestedCode !== undefined ? { suggestedCode: ann.suggestedCode } : {}),
            ...(ann.originalCode !== undefined ? { originalCode: ann.originalCode } : {}),
          }).catch(() => {});
        }
        if (diffDraftTimer) clearTimeout(diffDraftTimer);
        diffDraftTimer = setTimeout(() => get().saveDiffDraft(), DRAFT_DEBOUNCE_MS);
      },

      deleteDiffAnnotation: (id) => {
        set(
          (state) => ({
            diffAnnotations: state.diffAnnotations.filter(
              (a) => a.id !== id,
            ),
            selectedDiffAnnotationId:
              state.selectedDiffAnnotationId === id
                ? null
                : state.selectedDiffAnnotationId,
          }),
          undefined,
          'deleteDiffAnnotation',
        );
        api.deleteDiffAnnotation(id).catch(() => {});
        if (diffDraftTimer) clearTimeout(diffDraftTimer);
        diffDraftTimer = setTimeout(() => get().saveDiffDraft(), DRAFT_DEBOUNCE_MS);
      },

      selectDiffAnnotation: (id) =>
        set(
          { selectedDiffAnnotationId: id },
          undefined,
          'selectDiffAnnotation',
        ),

      // Draft persistence — localStorage cache as fallback.
      savePlanDraft: () => {
        const { planReviewId, planAnnotations } = get();
        if (!planReviewId) return;
        try {
          localStorage.setItem(
            draftKey('plan', planReviewId),
            JSON.stringify(planAnnotations),
          );
        } catch {
          // localStorage may be full or unavailable.
        }
      },

      loadPlanDraft: (planReviewId) => {
        try {
          const raw = localStorage.getItem(
            draftKey('plan', planReviewId),
          );
          if (raw) {
            const annotations = JSON.parse(raw) as PlanAnnotation[];
            set(
              { planAnnotations: annotations, planReviewId },
              undefined,
              'loadPlanDraft',
            );
          }
        } catch {
          // Corrupt data — ignore.
        }
      },

      saveDiffDraft: () => {
        const { diffMessageId, diffAnnotations } = get();
        if (!diffMessageId) return;
        try {
          localStorage.setItem(
            draftKey('diff', String(diffMessageId)),
            JSON.stringify(diffAnnotations),
          );
        } catch {
          // localStorage may be full or unavailable.
        }
      },

      loadDiffDraft: (messageId) => {
        try {
          const raw = localStorage.getItem(
            draftKey('diff', String(messageId)),
          );
          if (raw) {
            const annotations = JSON.parse(raw) as DiffAnnotation[];
            set(
              { diffAnnotations: annotations, diffMessageId: messageId },
              undefined,
              'loadDiffDraft',
            );
          }
        } catch {
          // Corrupt data — ignore.
        }
      },

      // Reset all annotation state.
      reset: () =>
        set(
          {
            planAnnotations: [],
            selectedPlanAnnotationId: null,
            planReviewId: null,
            diffAnnotations: [],
            selectedDiffAnnotationId: null,
            diffMessageId: null,
          },
          undefined,
          'reset',
        ),
    }),
    { name: 'annotation-store' },
  ),
);
