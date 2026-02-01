// React hook for review-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  listReviews,
  getReview,
  createReview,
  resubmitReview,
  cancelReview,
  listReviewIterations,
  listReviewIssues,
  listOpenReviewIssues,
  updateIssueStatus,
  getReviewStats,
  getFileDiff,
  getReviewPatch,
} from '@/api/reviews.js';
import type {
  Review,
  ReviewWithDetails,
  ReviewIteration,
  ReviewIssue,
  ReviewStats,
  FileDiff,
  ReviewPatch,
  ListReviewsParams,
  CreateReviewRequest,
  IssueStatus,
  ReviewDetailResponse,
} from '@/types/reviews.js';

// Query keys for reviews.
export const reviewKeys = {
  all: ['reviews'] as const,
  lists: () => [...reviewKeys.all, 'list'] as const,
  list: (params?: ListReviewsParams) => [...reviewKeys.lists(), params] as const,
  details: () => [...reviewKeys.all, 'detail'] as const,
  detail: (id: string) => [...reviewKeys.details(), id] as const,
  iterations: (id: string) => [...reviewKeys.detail(id), 'iterations'] as const,
  issues: (id: string) => [...reviewKeys.detail(id), 'issues'] as const,
  openIssues: (id: string) => [...reviewKeys.detail(id), 'openIssues'] as const,
  diff: (id: string, file: string) =>
    [...reviewKeys.detail(id), 'diff', file] as const,
  patch: (id: string) => [...reviewKeys.detail(id), 'patch'] as const,
  stats: () => [...reviewKeys.all, 'stats'] as const,
};

// Hook for fetching a list of reviews.
export function useReviews(params?: ListReviewsParams) {
  return useQuery({
    queryKey: reviewKeys.list(params),
    queryFn: async ({ signal }) => {
      return listReviews(params, signal);
    },
  });
}

// Hook for fetching a single review with details.
export function useReview(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.detail(reviewId),
    queryFn: async ({ signal }) => {
      return getReview(reviewId, signal);
    },
    enabled: enabled && !!reviewId,
  });
}

// Hook for fetching review iterations.
export function useReviewIterations(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.iterations(reviewId),
    queryFn: async ({ signal }) => {
      return listReviewIterations(reviewId, signal);
    },
    enabled: enabled && !!reviewId,
  });
}

// Hook for fetching review issues.
export function useReviewIssues(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.issues(reviewId),
    queryFn: async ({ signal }) => {
      return listReviewIssues(reviewId, signal);
    },
    enabled: enabled && !!reviewId,
  });
}

// Hook for fetching open review issues only.
export function useOpenReviewIssues(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.openIssues(reviewId),
    queryFn: async ({ signal }) => {
      return listOpenReviewIssues(reviewId, signal);
    },
    enabled: enabled && !!reviewId,
  });
}

// Hook for fetching review statistics.
export function useReviewStats() {
  return useQuery({
    queryKey: reviewKeys.stats(),
    queryFn: async ({ signal }) => {
      return getReviewStats(signal);
    },
  });
}

// Hook for fetching a file diff.
export function useFileDiff(reviewId: string, filePath: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.diff(reviewId, filePath),
    queryFn: async ({ signal }) => {
      return getFileDiff(reviewId, filePath, signal);
    },
    enabled: enabled && !!reviewId && !!filePath,
  });
}

// Hook for fetching the review patch.
export function useReviewPatch(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.patch(reviewId),
    queryFn: async ({ signal }) => {
      return getReviewPatch(reviewId, signal);
    },
    enabled: enabled && !!reviewId,
  });
}

// Hook for creating a new review.
export function useCreateReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateReviewRequest) => createReview(data),
    onSuccess: () => {
      // Invalidate review lists to refetch.
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
    },
  });
}

// Hook for resubmitting a review after changes.
export function useResubmitReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      reviewId,
      commitSha,
    }: {
      reviewId: string;
      commitSha: string;
    }) => resubmitReview(reviewId, { commit_sha: commitSha }),
    onSuccess: (_data, { reviewId }) => {
      // Invalidate the specific review and lists.
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(reviewId),
      });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
    },
  });
}

// Hook for cancelling a review.
export function useCancelReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (reviewId: string) => cancelReview(reviewId),
    onMutate: async (reviewId) => {
      // Cancel outgoing refetches.
      await queryClient.cancelQueries({
        queryKey: reviewKeys.detail(reviewId),
      });

      // Snapshot previous value.
      const previousReview = queryClient.getQueryData<ReviewDetailResponse>(
        reviewKeys.detail(reviewId),
      );

      // Optimistically update to cancelled state.
      if (previousReview) {
        queryClient.setQueryData<ReviewDetailResponse>(
          reviewKeys.detail(reviewId),
          {
            ...previousReview,
            state: 'cancelled',
          },
        );
      }

      return { previousReview };
    },
    onError: (_err, reviewId, context) => {
      // Rollback on error.
      if (context?.previousReview) {
        queryClient.setQueryData(
          reviewKeys.detail(reviewId),
          context.previousReview,
        );
      }
    },
    onSettled: () => {
      // Invalidate lists to ensure they reflect the change.
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
    },
  });
}

// Hook for updating issue status.
export function useUpdateIssueStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      reviewId,
      issueId,
      status,
    }: {
      reviewId: string;
      issueId: number;
      status: IssueStatus;
    }) => updateIssueStatus(reviewId, issueId, status),
    onMutate: async ({ reviewId, issueId, status }) => {
      // Cancel outgoing refetches.
      await queryClient.cancelQueries({
        queryKey: reviewKeys.issues(reviewId),
      });
      await queryClient.cancelQueries({
        queryKey: reviewKeys.openIssues(reviewId),
      });

      // Snapshot previous values.
      const previousIssues = queryClient.getQueryData<ReviewIssue[]>(
        reviewKeys.issues(reviewId),
      );
      const previousOpenIssues = queryClient.getQueryData<ReviewIssue[]>(
        reviewKeys.openIssues(reviewId),
      );

      // Optimistically update the cache.
      if (previousIssues) {
        queryClient.setQueryData<ReviewIssue[]>(
          reviewKeys.issues(reviewId),
          previousIssues.map((issue) =>
            issue.id === issueId
              ? {
                  ...issue,
                  status,
                  resolved_at:
                    status === 'fixed' ? new Date().toISOString() : undefined,
                }
              : issue,
          ),
        );
      }

      if (previousOpenIssues) {
        queryClient.setQueryData<ReviewIssue[]>(
          reviewKeys.openIssues(reviewId),
          status === 'open'
            ? previousOpenIssues
            : previousOpenIssues.filter((issue) => issue.id !== issueId),
        );
      }

      return { previousIssues, previousOpenIssues };
    },
    onError: (_err, { reviewId }, context) => {
      // Rollback on error.
      if (context?.previousIssues) {
        queryClient.setQueryData(
          reviewKeys.issues(reviewId),
          context.previousIssues,
        );
      }
      if (context?.previousOpenIssues) {
        queryClient.setQueryData(
          reviewKeys.openIssues(reviewId),
          context.previousOpenIssues,
        );
      }
    },
    onSettled: (_data, _err, { reviewId }) => {
      // Invalidate to ensure consistency.
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(reviewId),
      });
    },
  });
}

// Convenience hook that returns all review-related mutations.
export function useReviewMutations() {
  const createMutation = useCreateReview();
  const resubmitMutation = useResubmitReview();
  const cancelMutation = useCancelReview();
  const updateIssueMutation = useUpdateIssueStatus();

  return {
    create: createMutation,
    resubmit: resubmitMutation,
    cancel: cancelMutation,
    updateIssueStatus: updateIssueMutation,
  };
}
