// Reviews API client.

import { get, post, patch, del } from './client.js';
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
  ResubmitReviewRequest,
  UpdateIssueStatusRequest,
  ReviewListResponse,
  ReviewDetailResponse,
  IssueStatus,
} from '@/types/reviews.js';

// Build query string from params.
function buildQueryString(params: Record<string, unknown>): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null) {
      searchParams.set(key, String(value));
    }
  }
  const query = searchParams.toString();
  return query ? `?${query}` : '';
}

// List reviews with optional filters.
export async function listReviews(
  params?: ListReviewsParams,
  signal?: AbortSignal,
): Promise<ReviewListResponse> {
  const query = params ? buildQueryString(params) : '';
  return get<ReviewListResponse>(`/reviews${query}`, signal);
}

// Get a single review by ID with full details.
export async function getReview(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewDetailResponse> {
  return get<ReviewDetailResponse>(`/reviews/${reviewId}`, signal);
}

// Create a new review request.
export async function createReview(
  request: CreateReviewRequest,
  signal?: AbortSignal,
): Promise<Review> {
  return post<Review>('/reviews', request, signal);
}

// Re-request review after changes.
export async function resubmitReview(
  reviewId: string,
  request: ResubmitReviewRequest,
  signal?: AbortSignal,
): Promise<Review> {
  return post<Review>(`/reviews/${reviewId}/resubmit`, request, signal);
}

// Cancel a review.
export async function cancelReview(
  reviewId: string,
  signal?: AbortSignal,
): Promise<void> {
  return del<void>(`/reviews/${reviewId}`, signal);
}

// Get all iterations for a review.
export async function listReviewIterations(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewIteration[]> {
  return get<ReviewIteration[]>(`/reviews/${reviewId}/iterations`, signal);
}

// Get all issues for a review.
export async function listReviewIssues(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewIssue[]> {
  return get<ReviewIssue[]>(`/reviews/${reviewId}/issues`, signal);
}

// Get open issues for a review.
export async function listOpenReviewIssues(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewIssue[]> {
  return get<ReviewIssue[]>(`/reviews/${reviewId}/issues?status=open`, signal);
}

// Update issue status.
export async function updateIssueStatus(
  reviewId: string,
  issueId: number,
  status: IssueStatus,
  signal?: AbortSignal,
): Promise<ReviewIssue> {
  const request: UpdateIssueStatusRequest = { status };
  return patch<ReviewIssue>(
    `/reviews/${reviewId}/issues/${issueId}`,
    request,
    signal,
  );
}

// Get review statistics.
export async function getReviewStats(
  signal?: AbortSignal,
): Promise<ReviewStats> {
  return get<ReviewStats>('/reviews/stats', signal);
}

// Get diff for a specific file in a review.
export async function getFileDiff(
  reviewId: string,
  filePath: string,
  signal?: AbortSignal,
): Promise<FileDiff> {
  const encodedPath = encodeURIComponent(filePath);
  return get<FileDiff>(`/reviews/${reviewId}/diff?file=${encodedPath}`, signal);
}

// Get the full patch for a review.
export async function getReviewPatch(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewPatch> {
  return get<ReviewPatch>(`/reviews/${reviewId}/patch`, signal);
}

// List reviews by state (convenience methods).
export async function listPendingReviews(
  limit?: number,
  signal?: AbortSignal,
): Promise<ReviewListResponse> {
  return listReviews({ filter: 'pending_review', limit }, signal);
}

export async function listActiveReviews(
  limit?: number,
  signal?: AbortSignal,
): Promise<ReviewListResponse> {
  return listReviews({ filter: 'under_review', limit }, signal);
}

export async function listApprovedReviews(
  limit?: number,
  signal?: AbortSignal,
): Promise<ReviewListResponse> {
  return listReviews({ filter: 'approved', limit }, signal);
}

// List reviews requested by a specific user.
export async function listMyReviews(
  requesterId: number,
  limit?: number,
  signal?: AbortSignal,
): Promise<ReviewListResponse> {
  return listReviews({ requester_id: requesterId, limit }, signal);
}

// Reviews API namespace export.
export const reviewsApi = {
  list: listReviews,
  get: getReview,
  create: createReview,
  resubmit: resubmitReview,
  cancel: cancelReview,
  listIterations: listReviewIterations,
  listIssues: listReviewIssues,
  listOpenIssues: listOpenReviewIssues,
  updateIssueStatus,
  getStats: getReviewStats,
  getFileDiff,
  getPatch: getReviewPatch,
  listPending: listPendingReviews,
  listActive: listActiveReviews,
  listApproved: listApprovedReviews,
  listMine: listMyReviews,
};
