// ReviewDetailView component - shows full review details with issues list.

import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { IssueStatus, ReviewDetail } from '@/types/api.js';
import { useReviewIssues, useUpdateIssueStatus, useCancelReview } from '@/hooks/useReviews.js';
import { ReviewStateBadge } from './ReviewStateBadge.js';
import { ReviewIssueCard } from './ReviewIssueCard.js';
import { Spinner } from '@/components/ui/Spinner.js';
import { routes } from '@/lib/routes.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
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
