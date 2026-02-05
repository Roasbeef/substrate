// React hooks for review-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchReviews,
  fetchReview,
  fetchReviewIssues,
  createReview,
  resubmitReview,
  cancelReview,
  updateIssueStatus,
  type ReviewListOptions,
} from '@/api/reviews.js';
import type { CreateReviewRequest } from '@/types/api.js';

// Query keys for reviews.
export const reviewKeys = {
  all: ['reviews'] as const,
  lists: () => [...reviewKeys.all, 'list'] as const,
  list: (options: ReviewListOptions) =>
    [...reviewKeys.lists(), options] as const,
  details: () => [...reviewKeys.all, 'detail'] as const,
  detail: (id: string) => [...reviewKeys.details(), id] as const,
  issues: (reviewId: string) =>
    [...reviewKeys.all, 'issues', reviewId] as const,
};

// Hook for fetching a list of reviews.
export function useReviews(options: ReviewListOptions = {}) {
  return useQuery({
    queryKey: reviewKeys.list(options),
    queryFn: async ({ signal }) => {
      return fetchReviews(options, signal);
    },
  });
}

// Hook for fetching a single review.
export function useReview(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.detail(reviewId),
    queryFn: async ({ signal }) => {
      return fetchReview(reviewId, signal);
    },
    enabled: enabled && reviewId !== '',
  });
}

// Hook for fetching issues for a review.
export function useReviewIssues(reviewId: string, enabled = true) {
  return useQuery({
    queryKey: reviewKeys.issues(reviewId),
    queryFn: async ({ signal }) => {
      return fetchReviewIssues(reviewId, signal);
    },
    enabled: enabled && reviewId !== '',
  });
}

// Hook for creating a new review.
export function useCreateReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateReviewRequest) => createReview(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    },
  });
}

// Hook for resubmitting a review.
export function useResubmitReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ reviewId, commitSha }: {
      reviewId: string;
      commitSha: string;
    }) => resubmitReview(reviewId, commitSha),
    onSuccess: (_data, { reviewId }) => {
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(reviewId),
      });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    },
  });
}

// Hook for cancelling a review.
export function useCancelReview() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ reviewId, reason }: {
      reviewId: string;
      reason?: string;
    }) => cancelReview(reviewId, reason),
    onSuccess: (_data, { reviewId }) => {
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(reviewId),
      });
      void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
    },
  });
}

// Hook for updating an issue status.
export function useUpdateIssueStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ reviewId, issueId, status }: {
      reviewId: string;
      issueId: number;
      status: string;
    }) => updateIssueStatus(reviewId, issueId, status),
    onSuccess: (_data, { reviewId }) => {
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.issues(reviewId),
      });
      void queryClient.invalidateQueries({
        queryKey: reviewKeys.detail(reviewId),
      });
    },
  });
}
