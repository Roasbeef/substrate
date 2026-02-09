// React hooks for plan review queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchPlanReviews,
  fetchPlanReview,
  fetchPlanReviewByThread,
  updatePlanReviewStatus,
  type PlanReviewListOptions,
} from '@/api/plan-reviews.js';
import type { PlanReviewState } from '@/types/api.js';
import { useUIStore } from '@/stores/ui.js';

// Query keys for plan reviews.
export const planReviewKeys = {
  all: ['planReviews'] as const,
  lists: () => [...planReviewKeys.all, 'list'] as const,
  list: (options: PlanReviewListOptions) =>
    [...planReviewKeys.lists(), options] as const,
  details: () => [...planReviewKeys.all, 'detail'] as const,
  detail: (id: string) => [...planReviewKeys.details(), id] as const,
  byThread: (threadId: string) =>
    [...planReviewKeys.all, 'byThread', threadId] as const,
};

// Hook for fetching a list of plan reviews.
export function usePlanReviews(options: PlanReviewListOptions = {}) {
  return useQuery({
    queryKey: planReviewKeys.list(options),
    queryFn: async ({ signal }) => {
      return fetchPlanReviews(options, signal);
    },
  });
}

// Hook for fetching a single plan review.
export function usePlanReview(planReviewId: string, enabled = true) {
  return useQuery({
    queryKey: planReviewKeys.detail(planReviewId),
    queryFn: async ({ signal }) => {
      return fetchPlanReview(planReviewId, signal);
    },
    enabled: enabled && planReviewId !== '',
    // Poll pending reviews every 10 seconds for real-time updates.
    refetchInterval: (query) => {
      const data = query.state.data;
      if (data && data.state === 'pending') {
        return 10_000;
      }
      return false;
    },
  });
}

// Hook for fetching a plan review by thread ID.
export function usePlanReviewByThread(threadId: string, enabled = true) {
  return useQuery({
    queryKey: planReviewKeys.byThread(threadId),
    queryFn: async ({ signal }) => {
      return fetchPlanReviewByThread(threadId, signal);
    },
    enabled: enabled && threadId !== '',
    // Suppress 404 errors for threads without plan reviews.
    retry: false,
  });
}

// Hook for updating plan review status (approve/reject/request changes).
export function useUpdatePlanReviewStatus() {
  const queryClient = useQueryClient();
  const addToast = useUIStore((state) => state.addToast);

  return useMutation({
    mutationFn: ({ planReviewId, state, comment }: {
      planReviewId: string;
      state: PlanReviewState;
      comment?: string;
    }) => updatePlanReviewStatus(planReviewId, state, comment),
    onSuccess: (data, { state }) => {
      // Invalidate related queries.
      void queryClient.invalidateQueries({
        queryKey: planReviewKeys.detail(data.plan_review_id),
      });
      void queryClient.invalidateQueries({
        queryKey: planReviewKeys.lists(),
      });

      // Show success toast.
      const labels: Record<string, string> = {
        approved: 'Plan approved',
        rejected: 'Plan rejected',
        changes_requested: 'Changes requested',
      };
      addToast({
        variant: 'success',
        message: labels[state] ?? `Plan updated to ${state}`,
      });
    },
    onError: (error) => {
      addToast({
        variant: 'error',
        message: `Failed to update plan: ${error.message}`,
      });
    },
  });
}
