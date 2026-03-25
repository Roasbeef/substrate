// API functions for plan and diff annotation operations.
// Uses grpc-gateway REST API directly.

import { get, post, patch, del } from './client.js';
import type {
  PlanAnnotation,
  DiffAnnotation,
} from '@/types/annotations.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// =============================================================================
// Gateway response types
// =============================================================================

interface GatewayPlanAnnotationProto {
  id?: string;
  plan_review_id?: string;
  annotation_id?: string;
  block_id?: string;
  annotation_type?: string;
  text?: string;
  original_text?: string;
  start_offset?: string;
  end_offset?: string;
  diff_context?: string;
  created_at?: string;
  updated_at?: string;
}

interface GatewayDiffAnnotationProto {
  id?: string;
  annotation_id?: string;
  message_id?: string;
  annotation_type?: string;
  scope?: string;
  file_path?: string;
  line_start?: string;
  line_end?: string;
  side?: string;
  text?: string;
  suggested_code?: string;
  original_code?: string;
  created_at?: string;
  updated_at?: string;
}

interface GatewayListPlanAnnotationsResponse {
  annotations?: GatewayPlanAnnotationProto[];
}

interface GatewayListDiffAnnotationsResponse {
  annotations?: GatewayDiffAnnotationProto[];
}

// =============================================================================
// Parsers
// =============================================================================

// parsePlanAnnotation converts a gateway proto to a typed PlanAnnotation.
function parsePlanAnnotation(
  a: GatewayPlanAnnotationProto,
): PlanAnnotation {
  return {
    id: a.annotation_id ?? '',
    blockId: a.block_id ?? '',
    type: (a.annotation_type ?? 'COMMENT') as PlanAnnotation['type'],
    text: a.text,
    originalText: a.original_text ?? '',
    startOffset: toNumber(a.start_offset),
    endOffset: toNumber(a.end_offset),
    diffContext: a.diff_context as PlanAnnotation['diffContext'],
    createdAt: toNumber(a.created_at) * 1000,
    updatedAt: toNumber(a.updated_at) * 1000,
  };
}

// parseDiffAnnotation converts a gateway proto to a typed DiffAnnotation.
function parseDiffAnnotation(
  a: GatewayDiffAnnotationProto,
): DiffAnnotation {
  return {
    id: a.annotation_id ?? '',
    type: (a.annotation_type ?? 'comment') as DiffAnnotation['type'],
    scope: (a.scope ?? 'line') as DiffAnnotation['scope'],
    filePath: a.file_path ?? '',
    lineStart: toNumber(a.line_start),
    lineEnd: toNumber(a.line_end),
    side: (a.side ?? 'new') as DiffAnnotation['side'],
    text: a.text,
    suggestedCode: a.suggested_code,
    originalCode: a.original_code,
    createdAt: toNumber(a.created_at) * 1000,
    updatedAt: toNumber(a.updated_at) * 1000,
  };
}

// =============================================================================
// Plan Annotation API
// =============================================================================

// Fetch all annotations for a plan review.
export async function fetchPlanAnnotations(
  planReviewId: string,
  signal?: AbortSignal,
): Promise<PlanAnnotation[]> {
  const response = await get<GatewayListPlanAnnotationsResponse>(
    `/plan-reviews/${planReviewId}/annotations`,
    signal,
  );
  return (response.annotations ?? []).map(parsePlanAnnotation);
}

// Create a plan annotation.
export async function createPlanAnnotation(
  planReviewId: string,
  params: {
    annotationId: string;
    blockId: string;
    annotationType: string;
    text?: string;
    originalText: string;
    startOffset: number;
    endOffset: number;
    diffContext?: string;
  },
): Promise<PlanAnnotation> {
  const response = await post<GatewayPlanAnnotationProto>(
    `/plan-reviews/${planReviewId}/annotations`,
    {
      plan_review_id: planReviewId,
      annotation_id: params.annotationId,
      block_id: params.blockId,
      annotation_type: params.annotationType,
      text: params.text ?? '',
      original_text: params.originalText,
      start_offset: params.startOffset,
      end_offset: params.endOffset,
      diff_context: params.diffContext ?? '',
    },
  );
  return parsePlanAnnotation(response);
}

// Update a plan annotation.
export async function updatePlanAnnotation(
  annotationId: string,
  params: {
    text?: string;
    originalText?: string;
    startOffset?: number;
    endOffset?: number;
    diffContext?: string;
  },
): Promise<PlanAnnotation> {
  const response = await patch<GatewayPlanAnnotationProto>(
    `/annotations/plan/${annotationId}`,
    {
      annotation_id: annotationId,
      text: params.text ?? '',
      original_text: params.originalText ?? '',
      start_offset: params.startOffset ?? 0,
      end_offset: params.endOffset ?? 0,
      diff_context: params.diffContext ?? '',
    },
  );
  return parsePlanAnnotation(response);
}

// Delete a plan annotation.
export async function deletePlanAnnotation(
  annotationId: string,
): Promise<void> {
  await del(`/annotations/plan/${annotationId}`);
}

// =============================================================================
// Diff Annotation API
// =============================================================================

// Fetch all diff annotations for a message.
export async function fetchDiffAnnotations(
  messageId: number,
  signal?: AbortSignal,
): Promise<DiffAnnotation[]> {
  const response = await get<GatewayListDiffAnnotationsResponse>(
    `/messages/${messageId}/diff-annotations`,
    signal,
  );
  return (response.annotations ?? []).map(parseDiffAnnotation);
}

// Create a diff annotation.
export async function createDiffAnnotation(
  messageId: number,
  params: {
    annotationId: string;
    annotationType: string;
    scope: string;
    filePath: string;
    lineStart: number;
    lineEnd: number;
    side: string;
    text?: string;
    suggestedCode?: string;
    originalCode?: string;
  },
): Promise<DiffAnnotation> {
  const response = await post<GatewayDiffAnnotationProto>(
    `/messages/${messageId}/diff-annotations`,
    {
      annotation_id: params.annotationId,
      message_id: messageId,
      annotation_type: params.annotationType,
      scope: params.scope,
      file_path: params.filePath,
      line_start: params.lineStart,
      line_end: params.lineEnd,
      side: params.side,
      text: params.text ?? '',
      suggested_code: params.suggestedCode ?? '',
      original_code: params.originalCode ?? '',
    },
  );
  return parseDiffAnnotation(response);
}

// Update a diff annotation.
export async function updateDiffAnnotation(
  annotationId: string,
  params: {
    text?: string;
    suggestedCode?: string;
    originalCode?: string;
  },
): Promise<DiffAnnotation> {
  const response = await patch<GatewayDiffAnnotationProto>(
    `/annotations/diff/${annotationId}`,
    {
      annotation_id: annotationId,
      text: params.text ?? '',
      suggested_code: params.suggestedCode ?? '',
      original_code: params.originalCode ?? '',
    },
  );
  return parseDiffAnnotation(response);
}

// Delete a diff annotation.
export async function deleteDiffAnnotation(
  annotationId: string,
): Promise<void> {
  await del(`/annotations/diff/${annotationId}`);
}
