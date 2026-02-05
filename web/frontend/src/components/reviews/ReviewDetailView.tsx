// ReviewDetailView component - shows full review details with issues list.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { IssueStatus, ReviewDetail, ReviewIterationDetail } from '@/types/api.js';
import { useReviewIssues, useReviewDiff, useUpdateIssueStatus, useCancelReview } from '@/hooks/useReviews.js';
import { ReviewStateBadge } from './ReviewStateBadge.js';
import { ReviewIssueCard } from './ReviewIssueCard.js';
import { DiffViewer } from './DiffViewer.js';
import { Spinner } from '@/components/ui/Spinner.js';
import { routes } from '@/lib/routes.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Decision badge styles.
const decisionStyles: Record<string, string> = {
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-red-100 text-red-800',
  changes_requested: 'bg-orange-100 text-orange-800',
};

// Format milliseconds into human-readable duration.
function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
}

// Format unix timestamp to locale string.
function formatTimestamp(ts: number): string {
  if (ts === 0) return '--';
  return new Date(ts * 1000).toLocaleString();
}

// Format cost in USD.
function formatCost(usd: number): string {
  if (usd === 0) return '--';
  return `$${usd.toFixed(4)}`;
}

export interface ReviewDetailViewProps {
  review: ReviewDetail;
}

export function ReviewDetailView({ review }: ReviewDetailViewProps) {
  const navigate = useNavigate();
  const { data: issues, isLoading: issuesLoading } = useReviewIssues(
    review.review_id,
  );
  const updateStatus = useUpdateIssueStatus();
  const cancelMutation = useCancelReview();
  const { data: diffData, isLoading: diffLoading } = useReviewDiff(
    review.review_id,
  );
  const [showDiff, setShowDiff] = useState(false);

  const handleStatusChange = (issueId: number, status: IssueStatus) => {
    updateStatus.mutate({
      reviewId: review.review_id,
      issueId,
      status,
    });
  };

  const handleCancel = () => {
    cancelMutation.mutate(
      { reviewId: review.review_id },
      {
        onSuccess: () => {
          navigate(routes.reviews);
        },
      },
    );
  };

  const isTerminal = review.state === 'approved'
    || review.state === 'rejected'
    || review.state === 'cancelled';

  // Count open issues.
  const openCount = issues?.filter((i) => i.status === 'open').length ?? 0;
  const fixedCount = issues?.filter((i) => i.status === 'fixed').length ?? 0;

  return (
    <div className="space-y-6">
      {/* Back navigation. */}
      <button
        type="button"
        onClick={() => navigate(routes.reviews)}
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
        Back to Reviews
      </button>

      {/* Review header card. */}
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-gray-900">
                {review.branch}
              </h2>
              <ReviewStateBadge state={review.state} />
            </div>
            {review.base_branch ? (
              <p className="mt-1 text-sm text-gray-500">
                into <code className="text-gray-700">{review.base_branch}</code>
              </p>
            ) : null}
          </div>

          {/* Actions. */}
          {!isTerminal ? (
            <button
              type="button"
              onClick={handleCancel}
              disabled={cancelMutation.isPending}
              className={cn(
                'rounded-lg border border-red-200 px-3 py-1.5 text-sm font-medium',
                'text-red-600 hover:bg-red-50',
                'disabled:opacity-50 disabled:cursor-not-allowed',
              )}
            >
              {cancelMutation.isPending ? 'Cancelling...' : 'Cancel Review'}
            </button>
          ) : null}
        </div>

        {/* Metadata grid. */}
        <div className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div>
            <dt className="text-xs font-medium text-gray-500">Type</dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {review.review_type}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">Iterations</dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {review.iterations}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">Open Issues</dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {openCount}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">Fixed Issues</dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">
              {fixedCount}
            </dd>
          </div>
        </div>

        {/* Review ID. */}
        <div className="mt-4 border-t border-gray-100 pt-3">
          <span className="text-xs text-gray-400">
            Review ID: {review.review_id}
          </span>
          {review.thread_id ? (
            <span className="ml-4 text-xs text-gray-400">
              Thread: {review.thread_id}
            </span>
          ) : null}
        </div>

        {/* Error display. */}
        {review.error ? (
          <div className="mt-3 rounded bg-red-50 p-3 text-sm text-red-700">
            {review.error}
          </div>
        ) : null}
      </div>

      {/* Iterations section. */}
      {review.iteration_details && review.iteration_details.length > 0 ? (
        <div>
          <h3 className="mb-3 text-base font-semibold text-gray-900">
            Iterations ({review.iteration_details.length})
          </h3>
          <div className="space-y-3">
            {review.iteration_details.map((iter) => (
              <IterationCard key={iter.iteration_num} iteration={iter} />
            ))}
          </div>
        </div>
      ) : null}

      {/* Diff section. */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-base font-semibold text-gray-900">
            Changes
          </h3>
          <button
            type="button"
            onClick={() => setShowDiff(!showDiff)}
            className={cn(
              'rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
              showDiff
                ? 'border-blue-200 bg-blue-50 text-blue-700'
                : 'border-gray-200 text-gray-600 hover:bg-gray-50',
            )}
          >
            {showDiff ? 'Hide diff' : 'Show diff'}
          </button>
        </div>

        {showDiff ? (
          diffLoading ? (
            <div className="flex justify-center py-8">
              <Spinner size="md" variant="primary" label="Loading diff..." />
            </div>
          ) : diffData?.error ? (
            <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
              {diffData.error}
            </div>
          ) : diffData?.patch ? (
            <div>
              {diffData.command ? (
                <p className="mb-2 text-xs text-gray-400">
                  <code>{diffData.command}</code>
                </p>
              ) : null}
              <DiffViewer patch={diffData.patch} />
            </div>
          ) : (
            <div className="rounded-lg border border-gray-200 bg-white p-8 text-center">
              <p className="text-sm text-gray-500">No diff available.</p>
            </div>
          )
        ) : null}
      </div>

      {/* Issues section. */}
      <div>
        <h3 className="mb-3 text-base font-semibold text-gray-900">
          Issues ({issues?.length ?? 0})
        </h3>

        {issuesLoading ? (
          <div className="flex justify-center py-8">
            <Spinner size="md" variant="primary" label="Loading issues..." />
          </div>
        ) : issues && issues.length > 0 ? (
          <div className="space-y-3">
            {issues.map((issue) => (
              <ReviewIssueCard
                key={issue.id}
                issue={issue}
                onStatusChange={handleStatusChange}
                isUpdating={updateStatus.isPending}
              />
            ))}
          </div>
        ) : (
          <div className="rounded-lg border border-gray-200 bg-white p-8 text-center">
            <p className="text-sm text-gray-500">
              {review.state === 'under_review'
                ? 'Review is in progress. Issues will appear here when the reviewer completes analysis.'
                : 'No issues found for this review.'}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// IterationCard displays details for a single review iteration.
function IterationCard({ iteration }: { iteration: ReviewIterationDetail }) {
  const [expanded, setExpanded] = useState(false);

  const decisionLabel = iteration.decision
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());

  const badgeStyle = decisionStyles[iteration.decision] ?? 'bg-gray-100 text-gray-600';

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      {/* Iteration header. */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-gray-900">
            Iteration {iteration.iteration_num}
          </span>
          <span
            className={cn(
              'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
              badgeStyle,
            )}
          >
            {decisionLabel}
          </span>
        </div>
        {iteration.reviewer_id ? (
          <span className="text-xs text-gray-500">
            by {iteration.reviewer_id}
          </span>
        ) : null}
      </div>

      {/* Metrics row. */}
      <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div>
          <dt className="text-xs text-gray-500">Files</dt>
          <dd className="text-sm font-medium text-gray-900">
            {iteration.files_reviewed}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-gray-500">Lines</dt>
          <dd className="text-sm font-medium text-gray-900">
            {iteration.lines_analyzed.toLocaleString()}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-gray-500">Duration</dt>
          <dd className="text-sm font-medium text-gray-900">
            {formatDuration(iteration.duration_ms)}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-gray-500">Cost</dt>
          <dd className="text-sm font-medium text-gray-900">
            {formatCost(iteration.cost_usd)}
          </dd>
        </div>
      </div>

      {/* Summary toggle. */}
      {iteration.summary ? (
        <>
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="mt-3 text-xs font-medium text-gray-500 hover:text-gray-700"
          >
            {expanded ? 'Hide summary' : 'Show summary'}
          </button>
          {expanded ? (
            <div className="mt-2 rounded bg-gray-50 p-3">
              <p className="text-sm text-gray-700 whitespace-pre-wrap">
                {iteration.summary}
              </p>
            </div>
          ) : null}
        </>
      ) : null}

      {/* Timestamps. */}
      {iteration.started_at > 0 || iteration.completed_at > 0 ? (
        <div className="mt-3 flex gap-4 border-t border-gray-100 pt-2">
          {iteration.started_at > 0 ? (
            <span className="text-xs text-gray-400">
              Started: {formatTimestamp(iteration.started_at)}
            </span>
          ) : null}
          {iteration.completed_at > 0 ? (
            <span className="text-xs text-gray-400">
              Completed: {formatTimestamp(iteration.completed_at)}
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
