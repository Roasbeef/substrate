// PlanDetailView component — full plan viewer with inline annotation support.
// Uses a 3-column layout: [TOC | BlockViewer | AnnotationSidebar] when in
// review mode (pending state).

import { useState, useMemo, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { PlanReview } from '@/types/api.js';
import { useUpdatePlanReviewStatus } from '@/hooks/usePlanReviews.js';
import { useThread } from '@/hooks/useThreads.js';
import { PlanStateBadge } from './PlanStateBadge.js';
import { routes } from '@/lib/routes.js';

// Annotation imports.
import { parseMarkdownToBlocks } from '@/lib/markdown-parser.js';
import {
  getAnnotationCountBySection,
  buildTocHierarchy,
} from '@/lib/annotation-helpers.js';
import {
  exportPlanAnnotations,
  wrapFeedbackForAgent,
} from '@/lib/feedback-export.js';
import { useAnnotationStore } from '@/stores/annotations.js';
import { fetchPlanAnnotations } from '@/api/annotations.js';
import { BlockViewer } from './BlockViewer.js';
import { AnnotationSidebar } from './AnnotationSidebar.js';
import { PlanToc } from './PlanToc.js';
import { PlanDiffView } from './PlanDiffView.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format unix timestamp to locale string.
function formatTimestamp(ts: number): string {
  if (ts === 0) return '--';
  return new Date(ts * 1000).toLocaleString();
}

export interface PlanDetailViewProps {
  planReview: PlanReview;
}

export function PlanDetailView({ planReview }: PlanDetailViewProps) {
  const navigate = useNavigate();
  const updateStatus = useUpdatePlanReviewStatus();
  const [additionalComment, setAdditionalComment] = useState('');
  const [showDiff, setShowDiff] = useState(false);

  // Annotation store.
  const {
    planAnnotations,
    setPlanReviewId,
    loadPlanAnnotations,
    loadPlanDraft,
    reset: resetAnnotations,
  } = useAnnotationStore();

  // Initialize annotation store: fetch from server, fall back to draft.
  useEffect(() => {
    const prId = planReview.plan_review_id;
    setPlanReviewId(prId);

    // Try server first, fall back to localStorage draft.
    fetchPlanAnnotations(prId)
      .then((serverAnnotations) => {
        if (serverAnnotations.length > 0) {
          loadPlanAnnotations(serverAnnotations);
        } else {
          loadPlanDraft(prId);
        }
      })
      .catch(() => {
        loadPlanDraft(prId);
      });

    return () => {
      resetAnnotations();
    };
  }, [
    planReview.plan_review_id, setPlanReviewId, loadPlanAnnotations,
    loadPlanDraft, resetAnnotations,
  ]);

  // Fetch the plan thread to get the full plan body from the mail message.
  const { data: threadData, isLoading: threadLoading } = useThread(
    planReview.thread_id,
    !!planReview.thread_id,
  );

  // Extract the plan body from the first message in the thread.
  const planBody = useMemo(() => {
    if (!threadData?.messages || threadData.messages.length === 0) {
      return planReview.plan_summary || 'No plan content available.';
    }
    const firstMsg = threadData.messages[0];
    return firstMsg !== undefined
      ? firstMsg.body
      : 'No plan content available.';
  }, [threadData, planReview.plan_summary]);

  // Extract the previous plan version (for plan diff).
  const previousPlanBody = useMemo(() => {
    if (!threadData?.messages || threadData.messages.length < 2) {
      return null;
    }
    // Messages are ordered — second-to-last is the previous version.
    const prevMsg = threadData.messages[threadData.messages.length - 2];
    return prevMsg !== undefined ? prevMsg.body : null;
  }, [threadData]);

  const hasMultipleVersions = previousPlanBody !== null;
  const versionCount = threadData?.messages?.length ?? 1;

  // Parse plan body into blocks for annotation targeting.
  const blocks = useMemo(
    () => parseMarkdownToBlocks(planBody),
    [planBody],
  );

  // Build table of contents from blocks and annotation counts.
  const annotationCounts = useMemo(
    () => getAnnotationCountBySection(blocks, planAnnotations),
    [blocks, planAnnotations],
  );
  const tocItems = useMemo(
    () => buildTocHierarchy(blocks, annotationCounts),
    [blocks, annotationCounts],
  );

  const isPending = planReview.state === 'pending';

  // Scroll to a block by ID.
  const handleScrollToBlock = useCallback((blockId: string) => {
    const el = document.querySelector(`[data-block-id="${blockId}"]`);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, []);

  // Handle review actions.
  const handleAction = (
    action: 'approved' | 'rejected' | 'changes_requested',
  ) => {
    let comment = additionalComment.trim();

    // For deny actions with annotations, auto-generate structured feedback.
    if (
      (action === 'changes_requested' || action === 'rejected') &&
      planAnnotations.length > 0
    ) {
      const annotationFeedback = exportPlanAnnotations(
        blocks, planAnnotations,
      );
      const wrappedFeedback = wrapFeedbackForAgent(annotationFeedback);

      // Append any additional freeform comment.
      comment = comment
        ? `${wrappedFeedback}\n\n---\n\nAdditional notes:\n${comment}`
        : wrappedFeedback;
    }

    updateStatus.mutate({
      planReviewId: planReview.plan_review_id,
      state: action,
      ...(comment !== '' ? { comment } : {}),
    }, {
      onSuccess: () => {
        setAdditionalComment('');
      },
    });
  };

  const title =
    planReview.plan_title || planReview.plan_path || 'Untitled Plan';

  return (
    <div className="space-y-6">
      {/* Back navigation. */}
      <button
        type="button"
        onClick={() => navigate(routes.plans)}
        className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700"
      >
        <svg
          className="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M15 19l-7-7 7-7"
          />
        </svg>
        Back to Plans
      </button>

      {/* Plan header card. */}
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-gray-900">
                {title}
              </h2>
              <PlanStateBadge state={planReview.state} />
            </div>
            {planReview.plan_path ? (
              <p className="mt-1 text-sm text-gray-500">
                <code className="text-gray-600">
                  {planReview.plan_path}
                </code>
              </p>
            ) : null}
          </div>
        </div>

        {/* Metadata grid. */}
        <div className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div>
            <dt className="text-xs font-medium text-gray-500">
              Reviewer
            </dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {planReview.reviewer_name || '--'}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              Created
            </dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {formatTimestamp(planReview.created_at)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              Updated
            </dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {formatTimestamp(planReview.updated_at)}
            </dd>
          </div>
          {planReview.reviewed_at > 0 ? (
            <div>
              <dt className="text-xs font-medium text-gray-500">
                Reviewed
              </dt>
              <dd className="mt-1 text-sm font-medium text-gray-900">
                {formatTimestamp(planReview.reviewed_at)}
              </dd>
            </div>
          ) : (
            <div>
              <dt className="text-xs font-medium text-gray-500">
                Session
              </dt>
              <dd className="mt-1 text-sm font-medium text-gray-900">
                {planReview.session_id
                  ? planReview.session_id.slice(0, 12)
                  : '--'}
              </dd>
            </div>
          )}
        </div>

        {/* IDs row. */}
        <div className="mt-4 border-t border-gray-100 pt-3">
          <span className="text-xs text-gray-400">
            ID: {planReview.plan_review_id}
          </span>
          {planReview.thread_id ? (
            <span className="ml-4 text-xs text-gray-400">
              Thread: {planReview.thread_id}
            </span>
          ) : null}
        </div>
      </div>

      {/* AI Summary section (if available and distinct from body). */}
      {planReview.plan_summary &&
      planReview.plan_summary !== planBody ? (
        <div className="rounded-lg border border-blue-100 bg-blue-50/50 p-4">
          <h3 className="mb-2 text-sm font-semibold text-blue-900">
            AI Summary
          </h3>
          <p className="text-sm text-blue-800 whitespace-pre-wrap">
            {planReview.plan_summary}
          </p>
        </div>
      ) : null}

      {/* Reviewer comment (when resolved). */}
      {planReview.reviewer_comment ? (
        <div
          className={cn(
            'rounded-lg border p-4',
            planReview.state === 'approved'
              ? 'border-green-100 bg-green-50/50'
              : planReview.state === 'rejected'
                ? 'border-red-100 bg-red-50/50'
                : 'border-yellow-100 bg-yellow-50/50',
          )}
        >
          <h3
            className={cn(
              'mb-2 text-sm font-semibold',
              planReview.state === 'approved'
                ? 'text-green-900'
                : planReview.state === 'rejected'
                  ? 'text-red-900'
                  : 'text-yellow-900',
            )}
          >
            Reviewer Comment
          </h3>
          <p
            className={cn(
              'text-sm whitespace-pre-wrap',
              planReview.state === 'approved'
                ? 'text-green-800'
                : planReview.state === 'rejected'
                  ? 'text-red-800'
                  : 'text-yellow-800',
            )}
          >
            {planReview.reviewer_comment}
          </p>
        </div>
      ) : null}

      {/* Plan content — 3-column layout when pending, single column otherwise. */}
      {threadLoading ? (
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <div className="flex items-center gap-2 py-8 text-sm text-gray-500">
            <svg
              className="h-4 w-4 animate-spin"
              viewBox="0 0 24 24"
              fill="none"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
            Loading plan content...
          </div>
        </div>
      ) : isPending ? (
        // Three-column annotation layout.
        <div className="flex gap-4">
          {/* Left: Table of Contents. */}
          {tocItems.length > 0 && (
            <div className="hidden w-48 flex-shrink-0 lg:block">
              <div className="sticky top-4 rounded-lg border border-gray-200 bg-white py-3">
                <PlanToc
                  items={tocItems}
                  onScrollToBlock={handleScrollToBlock}
                />
              </div>
            </div>
          )}

          {/* Center: Plan content with block-based rendering. */}
          <div className="min-w-0 flex-1">
            <div className="rounded-lg border border-gray-200 bg-white p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <h3 className="text-base font-semibold text-gray-900">
                    Plan Content
                  </h3>
                  {hasMultipleVersions && (
                    <span className="text-xs text-gray-400">
                      v{versionCount}
                    </span>
                  )}
                  {hasMultipleVersions && (
                    <button
                      type="button"
                      onClick={() => setShowDiff(!showDiff)}
                      className={cn(
                        'rounded-md px-2 py-1 text-xs font-medium transition-colors',
                        showDiff
                          ? 'bg-blue-100 text-blue-800'
                          : 'text-gray-500 hover:bg-gray-100',
                      )}
                    >
                      {showDiff ? 'Hide Diff' : 'Show Changes'}
                    </button>
                  )}
                </div>
                {planAnnotations.length > 0 && (
                  <span className="rounded-full bg-yellow-100 px-2.5 py-0.5 text-xs font-medium text-yellow-800">
                    {planAnnotations.length} annotation
                    {planAnnotations.length !== 1 ? 's' : ''}
                  </span>
                )}
              </div>
              {showDiff && previousPlanBody ? (
                <PlanDiffView
                  oldText={previousPlanBody}
                  newText={planBody}
                />
              ) : (
                <BlockViewer
                  blocks={blocks}
                  annotations={planAnnotations}
                />
              )}
            </div>
          </div>

          {/* Right: Annotation Sidebar. */}
          <div className="hidden w-72 flex-shrink-0 xl:block">
            <div className="sticky top-4 max-h-[calc(100vh-6rem)] rounded-lg border border-gray-200 bg-white">
              <AnnotationSidebar
                annotations={planAnnotations}
                onScrollToBlock={handleScrollToBlock}
              />
            </div>
          </div>
        </div>
      ) : (
        // Read-only single-column layout.
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-4 text-base font-semibold text-gray-900">
            Plan Content
          </h3>
          <BlockViewer
            blocks={blocks}
            annotations={planAnnotations}
            readOnly
          />
        </div>
      )}

      {/* Action buttons (only shown when pending). */}
      {isPending ? (
        <div className="rounded-lg border border-gray-200 bg-white p-6">
          <h3 className="mb-3 text-base font-semibold text-gray-900">
            Review Actions
          </h3>

          {/* Annotation summary. */}
          {planAnnotations.length > 0 && (
            <div className="mb-3 rounded-lg border border-yellow-200 bg-yellow-50 px-3 py-2 text-xs text-yellow-800">
              {planAnnotations.length} annotation
              {planAnnotations.length !== 1 ? 's' : ''} will be
              included as structured feedback when requesting changes
              or rejecting.
            </div>
          )}

          {/* Additional comment textarea. */}
          <textarea
            value={additionalComment}
            onChange={(e) => setAdditionalComment(e.target.value)}
            placeholder="Add additional notes (optional)..."
            rows={3}
            className={cn(
              'w-full rounded-lg border border-gray-300 px-3 py-2 text-sm',
              'placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
              'resize-none',
            )}
          />

          {/* Action buttons. */}
          <div className="mt-3 flex items-center gap-3">
            <button
              type="button"
              onClick={() => handleAction('approved')}
              disabled={updateStatus.isPending}
              className={cn(
                'rounded-lg px-4 py-2 text-sm font-medium text-white',
                'bg-green-600 hover:bg-green-700',
                'disabled:opacity-50 disabled:cursor-not-allowed',
                'focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2',
              )}
            >
              {updateStatus.isPending ? 'Updating...' : 'Approve'}
            </button>

            <button
              type="button"
              onClick={() => handleAction('changes_requested')}
              disabled={updateStatus.isPending}
              className={cn(
                'rounded-lg border border-yellow-300 px-4 py-2 text-sm font-medium',
                'text-yellow-700 hover:bg-yellow-50',
                'disabled:opacity-50 disabled:cursor-not-allowed',
                'focus:outline-none focus:ring-2 focus:ring-yellow-500 focus:ring-offset-2',
              )}
            >
              Request Changes
              {planAnnotations.length > 0 &&
                ` (${planAnnotations.length})`}
            </button>

            <button
              type="button"
              onClick={() => handleAction('rejected')}
              disabled={updateStatus.isPending}
              className={cn(
                'rounded-lg border border-red-300 px-4 py-2 text-sm font-medium',
                'text-red-700 hover:bg-red-50',
                'disabled:opacity-50 disabled:cursor-not-allowed',
                'focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2',
              )}
            >
              Reject
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
