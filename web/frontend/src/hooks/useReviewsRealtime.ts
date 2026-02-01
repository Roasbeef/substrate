// React hook for real-time review updates via WebSocket.

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWebSocket } from './useWebSocket.js';
import { reviewKeys } from './useReviews.js';
import type { ReviewWebSocketEvent } from '@/types/reviews.js';

// Hook for subscribing to real-time review updates.
export function useReviewsRealtime(reviewId?: string) {
  const queryClient = useQueryClient();
  const { lastMessage } = useWebSocket();

  useEffect(() => {
    if (!lastMessage) return;

    // Try to parse as ReviewWebSocketEvent.
    let event: ReviewWebSocketEvent | null = null;
    try {
      const parsed = lastMessage as unknown;
      if (
        typeof parsed === 'object' &&
        parsed !== null &&
        'type' in parsed &&
        'payload' in parsed
      ) {
        event = parsed as ReviewWebSocketEvent;
      }
    } catch {
      return;
    }

    if (!event) return;

    const { type, payload } = event;

    switch (type) {
      case 'review_created':
        // Invalidate lists to show the new review.
        void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
        void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
        break;

      case 'review_updated':
        // Invalidate the specific review.
        if (payload.review_id) {
          void queryClient.invalidateQueries({
            queryKey: reviewKeys.detail(payload.review_id),
          });
        }
        // Always invalidate lists.
        void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
        void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
        break;

      case 'review_iteration_added':
        // Invalidate iterations and details if it's the review we're watching.
        if (payload.review_id === reviewId || !reviewId) {
          if (payload.review_id) {
            void queryClient.invalidateQueries({
              queryKey: reviewKeys.detail(payload.review_id),
            });
            void queryClient.invalidateQueries({
              queryKey: reviewKeys.iterations(payload.review_id),
            });
          }
        }
        break;

      case 'issue_resolved':
        // Invalidate issues for the review.
        if (payload.review_id === reviewId || !reviewId) {
          if (payload.review_id) {
            void queryClient.invalidateQueries({
              queryKey: reviewKeys.issues(payload.review_id),
            });
            void queryClient.invalidateQueries({
              queryKey: reviewKeys.openIssues(payload.review_id),
            });
          }
        }
        break;

      case 'review_approved':
      case 'review_rejected':
        // Invalidate everything for this review.
        if (payload.review_id) {
          void queryClient.invalidateQueries({
            queryKey: reviewKeys.detail(payload.review_id),
          });
        }
        void queryClient.invalidateQueries({ queryKey: reviewKeys.lists() });
        void queryClient.invalidateQueries({ queryKey: reviewKeys.stats() });
        break;
    }
  }, [lastMessage, reviewId, queryClient]);
}

// Hook for subscribing to updates for a specific review.
export function useReviewRealtime(reviewId: string) {
  return useReviewsRealtime(reviewId);
}

// Hook for subscribing to review list updates only.
export function useReviewListRealtime() {
  return useReviewsRealtime();
}
