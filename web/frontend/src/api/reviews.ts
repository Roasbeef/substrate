// API functions for review-related operations.
// Uses grpc-gateway REST API directly.

import { get, post, patch, del } from './client.js';
import type {
  CreateReviewRequest,
  CreateReviewResponse,
  ReviewDetail,
  ReviewIssue,
  ReviewSummary,
} from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Gateway response format for ListReviews.
interface GatewayListReviewsResponse {
  reviews?: Array<{
    review_id?: string;
    thread_id?: string;
    requester_id?: string;
    branch?: string;
    state?: string;
    review_type?: string;
    created_at?: string;
  }>;
}

// Gateway response format for GetReview.
interface GatewayReviewDetailResponse {
  review_id?: string;
  thread_id?: string;
  state?: string;
  branch?: string;
  base_branch?: string;
  review_type?: string;
  iterations?: number;
  open_issues?: string;
  error?: string;
}

// Gateway response format for ListReviewIssues.
interface GatewayListReviewIssuesResponse {
  issues?: Array<{
    id?: string;
    review_id?: string;
    iteration_num?: number;
    issue_type?: string;
    severity?: string;
    file_path?: string;
    line_start?: number;
    line_end?: number;
    title?: string;
    description?: string;
    code_snippet?: string;
    suggestion?: string;
    claude_md_ref?: string;
    status?: string;
  }>;
}

// Review list filter options.
export interface ReviewListOptions {
  state?: string;
  requesterId?: number;
  limit?: number;
  offset?: number;
}

// Build query string from filter options.
function buildQueryString(options: ReviewListOptions): string {
  const params = new URLSearchParams();

  if (options.state !== undefined && options.state !== '') {
    params.set('state', options.state);
  }
  if (options.requesterId !== undefined) {
    params.set('requester_id', String(options.requesterId));
  }
  if (options.limit !== undefined) {
    params.set('limit', String(options.limit));
  }
  if (options.offset !== undefined) {
    params.set('offset', String(options.offset));
  }

  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

// Parse gateway response into ReviewSummary array.
function parseListReviewsResponse(
  response: GatewayListReviewsResponse,
): ReviewSummary[] {
  return (response.reviews ?? []).map((r): ReviewSummary => ({
    review_id: r.review_id ?? '',
    thread_id: r.thread_id ?? '',
    requester_id: toNumber(r.requester_id),
    branch: r.branch ?? '',
    state: (r.state ?? 'pending_review') as ReviewSummary['state'],
    review_type: (r.review_type ?? 'full') as ReviewSummary['review_type'],
    created_at: toNumber(r.created_at),
  }));
}

// Parse gateway response into ReviewDetail.
function parseReviewDetailResponse(
  response: GatewayReviewDetailResponse,
): ReviewDetail {
  const result: ReviewDetail = {
    review_id: response.review_id ?? '',
    thread_id: response.thread_id ?? '',
    state: (response.state ?? 'pending_review') as ReviewDetail['state'],
    branch: response.branch ?? '',
    base_branch: response.base_branch ?? '',
    review_type: (response.review_type ?? 'full') as ReviewDetail['review_type'],
    iterations: response.iterations ?? 0,
    open_issues: toNumber(response.open_issues),
  };
  if (response.error !== undefined) {
    result.error = response.error;
  }
  return result;
}

// Parse gateway response into ReviewIssue array.
function parseListIssuesResponse(
  response: GatewayListReviewIssuesResponse,
): ReviewIssue[] {
  return (response.issues ?? []).map((issue): ReviewIssue => ({
    id: toNumber(issue.id),
    review_id: issue.review_id ?? '',
    iteration_num: issue.iteration_num ?? 0,
    issue_type: (issue.issue_type ?? 'other') as ReviewIssue['issue_type'],
    severity: (issue.severity ?? 'minor') as ReviewIssue['severity'],
    file_path: issue.file_path ?? '',
    line_start: issue.line_start ?? 0,
    line_end: issue.line_end ?? 0,
    title: issue.title ?? '',
    description: issue.description ?? '',
    code_snippet: issue.code_snippet ?? '',
    suggestion: issue.suggestion ?? '',
    claude_md_ref: issue.claude_md_ref ?? '',
    status: (issue.status ?? 'open') as ReviewIssue['status'],
  }));
}

// Fetch reviews with optional filters.
export async function fetchReviews(
  options: ReviewListOptions = {},
  signal?: AbortSignal,
): Promise<ReviewSummary[]> {
  const query = buildQueryString(options);
  const response = await get<GatewayListReviewsResponse>(
    `/reviews${query}`,
    signal,
  );
  return parseListReviewsResponse(response);
}

// Fetch a single review by ID.
export async function fetchReview(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewDetail> {
  const response = await get<GatewayReviewDetailResponse>(
    `/reviews/${reviewId}`,
    signal,
  );
  return parseReviewDetailResponse(response);
}

// Create a new review request.
export function createReview(
  data: CreateReviewRequest,
): Promise<CreateReviewResponse> {
  return post<CreateReviewResponse>('/reviews', data);
}

// Resubmit a review with new commit SHA.
export function resubmitReview(
  reviewId: string,
  commitSha: string,
): Promise<CreateReviewResponse> {
  return post<CreateReviewResponse>(
    `/reviews/${reviewId}/resubmit`,
    { commit_sha: commitSha },
  );
}

// Cancel an active review.
export function cancelReview(
  reviewId: string,
  reason?: string,
): Promise<void> {
  return del<void>(
    `/reviews/${reviewId}${reason ? `?reason=${encodeURIComponent(reason)}` : ''}`,
  );
}

// Fetch issues for a specific review.
export async function fetchReviewIssues(
  reviewId: string,
  signal?: AbortSignal,
): Promise<ReviewIssue[]> {
  const response = await get<GatewayListReviewIssuesResponse>(
    `/reviews/${reviewId}/issues`,
    signal,
  );
  return parseListIssuesResponse(response);
}

// Update the status of a review issue.
export function updateIssueStatus(
  reviewId: string,
  issueId: number,
  status: string,
): Promise<void> {
  return patch<void>(
    `/reviews/${reviewId}/issues/${issueId}`,
    { status },
  );
}
