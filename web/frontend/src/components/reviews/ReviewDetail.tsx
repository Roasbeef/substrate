// ReviewDetail component - displays full review information with iterations and issues.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import {
  useReview,
  useReviewIterations,
  useOpenReviewIssues,
  useUpdateIssueStatus,
  useCancelReview,
  useResubmitReview,
} from '@/hooks/useReviews.js';
import { useReviewRealtime } from '@/hooks/useReviewsRealtime.js';
import { ReviewStateBadge, DecisionBadge } from './ReviewBadges.js';
import { IssueCard } from './IssueCard.js';
import { Button } from '@/components/ui/Button.js';
import { Skeleton } from '@/components/ui/Skeleton.js';
import type { ReviewIteration, ReviewIssue, IssueStatus } from '@/types/reviews.js';
import { isTerminalState, canResubmit, canCancel } from '@/types/reviews.js';

// Combine clsx and tailwind-merge.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format duration.
function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds}s`;
}

// Format cost.
function formatCost(usd: number): string {
  if (usd === 0) return '$0.00';
  if (usd < 0.01) return '<$0.01';
  return `$${usd.toFixed(2)}`;
}

// Format timestamp.
function formatTime(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

// Branch icon.
function BranchIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 18V6m10 12V6M7 18a2 2 0 100-4 2 2 0 000 4zm10 0a2 2 0 100-4 2 2 0 000 4zM7 6a2 2 0 100-4 2 2 0 000 4zm10 0a2 2 0 100-4 2 2 0 000 4z"
      />
    </svg>
  );
}

// Commit icon.
function CommitIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 12h.01M12 12h.01M16 12h.01M12 8V4m0 16v-4"
      />
      <circle cx="12" cy="12" r="3" strokeWidth={2} />
    </svg>
  );
}

// Iteration item component.
interface IterationItemProps {
  iteration: ReviewIteration;
  isLatest: boolean;
}

function IterationItem({ iteration, isLatest }: IterationItemProps) {
  const [expanded, setExpanded] = useState(isLatest);

  return (
    <div
      className={cn(
        'border rounded-lg overflow-hidden',
        isLatest ? 'border-blue-200 bg-blue-50/30' : 'border-gray-200',
      )}
    >
      {/* Iteration header */}
      <div
        className={cn(
          'px-4 py-3 flex items-center justify-between cursor-pointer',
          isLatest ? 'bg-blue-50' : 'bg-gray-50',
        )}
        onClick={() => setExpanded(!expanded)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setExpanded(!expanded);
          }
        }}
      >
        <div className="flex items-center gap-3">
          <span className="text-sm font-medium text-gray-700">
            Iteration {iteration.iteration_num}
          </span>
          <DecisionBadge decision={iteration.decision} size="sm" />
          <span className="text-xs text-gray-500">{iteration.reviewer_id}</span>
        </div>

        <div className="flex items-center gap-4 text-xs text-gray-500">
          <span>{formatTime(iteration.started_at)}</span>
          <span>{formatDuration(iteration.duration_ms)}</span>
          <span>{formatCost(iteration.cost_usd)}</span>
        </div>
      </div>

      {/* Iteration details */}
      {expanded && (
        <div className="px-4 py-3 border-t bg-white">
          <p className="text-sm text-gray-700 mb-3">{iteration.summary}</p>

          <div className="grid grid-cols-3 gap-4 text-xs text-gray-500 mb-3">
            <div>
              <span className="font-medium text-gray-700">Files: </span>
              {iteration.files_reviewed}
            </div>
            <div>
              <span className="font-medium text-gray-700">Lines: </span>
              {iteration.lines_analyzed}
            </div>
            <div>
              <span className="font-medium text-gray-700">Issues: </span>
              {iteration.issues?.length ?? 0}
            </div>
          </div>

          {/* Issues in this iteration */}
          {iteration.issues && iteration.issues.length > 0 && (
            <div className="space-y-2">
              <h4 className="text-xs font-medium text-gray-500 uppercase tracking-wider">
                Issues Found
              </h4>
              {iteration.issues.map((issue, idx) => (
                <IssueCard key={idx} issue={issue} compact />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export interface ReviewDetailProps {
  reviewId: string;
  onClose?: () => void;
  className?: string;
}

export function ReviewDetail({ reviewId, onClose, className }: ReviewDetailProps) {
  // Fetch review data.
  const { data: review, isLoading, error } = useReview(reviewId);
  const { data: iterations } = useReviewIterations(reviewId);
  const { data: openIssues } = useOpenReviewIssues(reviewId);

  // Subscribe to real-time updates.
  useReviewRealtime(reviewId);

  // Mutations.
  const updateIssueStatus = useUpdateIssueStatus();
  const cancelReview = useCancelReview();
  const resubmitReview = useResubmitReview();

  // Handle issue status change.
  const handleIssueStatusChange = (issueId: number, status: IssueStatus) => {
    updateIssueStatus.mutate({ reviewId, issueId, status });
  };

  // Loading state.
  if (isLoading) {
    return (
      <div className={cn('p-4 space-y-4', className)}>
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-20" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  // Error state.
  if (error || !review) {
    return (
      <div className={cn('p-4 text-center', className)}>
        <p className="text-sm text-red-600">Failed to load review</p>
      </div>
    );
  }

  const sortedIterations = [...(iterations ?? [])].sort(
    (a, b) => b.iteration_num - a.iteration_num,
  );

  return (
    <div className={cn('flex flex-col h-full', className)}>
      {/* Header */}
      <div className="flex-shrink-0 border-b border-gray-200 px-4 py-3">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <BranchIcon className="h-5 w-5 text-gray-400" />
            <h2 className="font-mono text-lg font-semibold text-gray-900">
              {review.branch}
            </h2>
            <ReviewStateBadge state={review.state} />
          </div>

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="p-1 text-gray-400 hover:text-gray-600"
              aria-label="Close"
            >
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          )}
        </div>

        <div className="flex items-center gap-4 text-sm text-gray-600">
          <span className="flex items-center gap-1">
            Base: <span className="font-mono">{review.base_branch}</span>
          </span>
          <span className="flex items-center gap-1">
            <CommitIcon className="h-4 w-4" />
            <span className="font-mono">{review.commit_sha.slice(0, 7)}</span>
          </span>
          <span className="text-gray-400">|</span>
          <span>{review.review_type}</span>
          {review.pr_number && (
            <>
              <span className="text-gray-400">|</span>
              <a
                href={`https://github.com/${review.repo_path}/pull/${review.pr_number}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:underline"
              >
                PR #{review.pr_number}
              </a>
            </>
          )}
        </div>
      </div>

      {/* Actions */}
      <div className="flex-shrink-0 px-4 py-2 border-b border-gray-100 bg-gray-50 flex items-center gap-2">
        {canResubmit(review.state) && (
          <Button
            size="sm"
            variant="primary"
            onClick={() => {
              // TODO: Get latest commit SHA
              resubmitReview.mutate({ reviewId, commitSha: review.commit_sha });
            }}
            disabled={resubmitReview.isPending}
          >
            Request Re-review
          </Button>
        )}
        {canCancel(review.state) && (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => cancelReview.mutate(reviewId)}
            disabled={cancelReview.isPending}
          >
            Cancel Review
          </Button>
        )}
        {isTerminalState(review.state) && (
          <span className="text-xs text-gray-500">
            Review completed at {formatTime(review.completed_at ?? review.updated_at)}
          </span>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Open Issues */}
        {openIssues && openIssues.length > 0 && (
          <section>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">
              Open Issues ({openIssues.length})
            </h3>
            <div className="space-y-2">
              {openIssues.map((issue) => (
                <IssueCard
                  key={issue.id}
                  issue={issue}
                  onStatusChange={(status) => handleIssueStatusChange(issue.id, status)}
                  isLoading={updateIssueStatus.isPending}
                />
              ))}
            </div>
          </section>
        )}

        {/* Iterations */}
        <section>
          <h3 className="text-sm font-semibold text-gray-700 mb-2">
            Review Iterations ({sortedIterations.length})
          </h3>
          {sortedIterations.length === 0 ? (
            <p className="text-sm text-gray-500">No iterations yet.</p>
          ) : (
            <div className="space-y-2">
              {sortedIterations.map((iteration, idx) => (
                <IterationItem
                  key={iteration.id}
                  iteration={iteration}
                  isLatest={idx === 0}
                />
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
