// API functions for plan review operations.
// Uses grpc-gateway REST API directly.

import { get, patch } from './client.js';
import type { PlanReview, PlanReviewState } from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Gateway response format for a single plan review.
interface GatewayPlanReviewProto {
  id?: string;
  plan_review_id?: string;
  thread_id?: string;
  requester_id?: string;
  reviewer_name?: string;
  plan_path?: string;
  plan_title?: string;
  plan_summary?: string;
  state?: string;
  reviewer_comment?: string;
  reviewed_by?: string;
  session_id?: string;
  message_id?: string;
  created_at?: string;
  updated_at?: string;
  reviewed_at?: string;
}

// Gateway response format for ListPlanReviews.
interface GatewayListPlanReviewsResponse {
  plan_reviews?: GatewayPlanReviewProto[];
}

// Plan review list filter options.
export interface PlanReviewListOptions {
  state?: string;
  requesterId?: number;
  limit?: number;
  offset?: number;
}

// Build query string from filter options.
function buildQueryString(options: PlanReviewListOptions): string {
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

// Parse a gateway plan review proto into a typed PlanReview.
function parsePlanReview(r: GatewayPlanReviewProto): PlanReview {
  return {
    id: toNumber(r.id),
    plan_review_id: r.plan_review_id ?? '',
    thread_id: r.thread_id ?? '',
    requester_id: toNumber(r.requester_id),
    reviewer_name: r.reviewer_name ?? '',
    plan_path: r.plan_path ?? '',
    plan_title: r.plan_title ?? '',
    plan_summary: r.plan_summary ?? '',
    state: (r.state ?? 'pending') as PlanReviewState,
    reviewer_comment: r.reviewer_comment ?? '',
    reviewed_by: toNumber(r.reviewed_by),
    session_id: r.session_id ?? '',
    message_id: toNumber(r.message_id),
    created_at: toNumber(r.created_at),
    updated_at: toNumber(r.updated_at),
    reviewed_at: toNumber(r.reviewed_at),
  };
}

// Fetch plan reviews with optional filters.
export async function fetchPlanReviews(
  options: PlanReviewListOptions = {},
  signal?: AbortSignal,
): Promise<PlanReview[]> {
  const query = buildQueryString(options);
  const response = await get<GatewayListPlanReviewsResponse>(
    `/plan-reviews${query}`,
    signal,
  );
  return (response.plan_reviews ?? []).map(parsePlanReview);
}

// Fetch a single plan review by ID.
export async function fetchPlanReview(
  planReviewId: string,
  signal?: AbortSignal,
): Promise<PlanReview> {
  const response = await get<GatewayPlanReviewProto>(
    `/plan-reviews/${planReviewId}`,
    signal,
  );
  return parsePlanReview(response);
}

// Fetch a plan review by thread ID.
export async function fetchPlanReviewByThread(
  threadId: string,
  signal?: AbortSignal,
): Promise<PlanReview> {
  const response = await get<GatewayPlanReviewProto>(
    `/plan-reviews/by-thread/${threadId}`,
    signal,
  );
  return parsePlanReview(response);
}

// Update plan review status (approve, reject, or request changes).
export async function updatePlanReviewStatus(
  planReviewId: string,
  state: PlanReviewState,
  reviewerComment?: string,
): Promise<PlanReview> {
  const body: Record<string, unknown> = { state };
  if (reviewerComment !== undefined) {
    body.reviewer_comment = reviewerComment;
  }
  const response = await patch<GatewayPlanReviewProto>(
    `/plan-reviews/${planReviewId}`,
    body,
  );
  return parsePlanReview(response);
}
