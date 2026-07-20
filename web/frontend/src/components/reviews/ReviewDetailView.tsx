// ReviewDetailView component - shows full review details with issues list.

import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { renderMarkdownToHtml } from '@/lib/markdown.js';
import type { IssueStatus, Message, ReviewDetail, ReviewIterationDetail } from '@/types/api.js';
import { useReviewIssues, useReviewDiff, useUpdateIssueStatus, useCancelReview } from '@/hooks/useReviews.js';
import { useThread } from '@/hooks/useThreads.js';
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
  approved: 'bg-[#178A5B]/10 text-[var(--c-green)]',
  rejected: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  changes_requested: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
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

  // Fetch the review thread to get the reviewer's full mail messages.
  const { data: threadData, isLoading: threadLoading } = useThread(
    review.thread_id,
    !!review.thread_id,
  );

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
        className="flex items-center gap-1 text-sm text-[var(--c-mut)] hover:text-[var(--c-ink)]"
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
      <div className="rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-[var(--c-ink)]">
                {review.branch}
              </h2>
              <ReviewStateBadge state={review.state} />
            </div>
            {review.base_branch ? (
              <p className="mt-1 text-sm text-[var(--c-mut)]">
                into <code className="text-[var(--c-ink)]">{review.base_branch}</code>
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
                'rounded-lg border border-[#B3372B]/30 px-3 py-1.5 text-sm font-medium',
                'text-[var(--c-rust)] hover:bg-[#B3372B]/10',
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
            <dt className="text-xs font-medium text-[var(--c-mut)]">Type</dt>
            <dd className="mt-1 text-sm font-medium text-[var(--c-ink)]">
              {review.review_type}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-[var(--c-mut)]">Iterations</dt>
            <dd className="mt-1 text-sm font-medium text-[var(--c-ink)]">
              {review.iterations}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-[var(--c-mut)]">Open Issues</dt>
            <dd className="mt-1 text-sm font-medium text-[var(--c-ink)]">
              {openCount}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-[var(--c-mut)]">Fixed Issues</dt>
            <dd className="mt-1 text-sm font-medium text-[var(--c-ink)]">
              {fixedCount}
            </dd>
          </div>
        </div>

        {/* Review ID. */}
        <div className="mt-4 border-t border-[var(--c-fill)] pt-3">
          <span className="text-xs text-[var(--c-faint)]">
            Review ID: {review.review_id}
          </span>
          {review.thread_id ? (
            <span className="ml-4 text-xs text-[var(--c-faint)]">
              Thread: {review.thread_id}
            </span>
          ) : null}
        </div>

        {/* Error display. */}
        {review.error ? (
          <div className="mt-3 rounded bg-[#B3372B]/10 p-3 text-sm text-[var(--c-rust)]">
            {review.error}
          </div>
        ) : null}
      </div>

      {/* Iterations section. */}
      {review.iteration_details && review.iteration_details.length > 0 ? (
        <div>
          <h3 className="mb-3 text-base font-semibold text-[var(--c-ink)]">
            Iterations ({review.iteration_details.length})
          </h3>
          <div className="space-y-3">
            {review.iteration_details.map((iter) => (
              <IterationCard
                key={iter.iteration_num}
                iteration={iter}
                threadMessages={threadData?.messages}
                threadLoading={threadLoading}
              />
            ))}
          </div>
        </div>
      ) : null}

      {/* Diff section. */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-base font-semibold text-[var(--c-ink)]">
            Changes
          </h3>
          <button
            type="button"
            onClick={() => setShowDiff(!showDiff)}
            className={cn(
              'rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
              showDiff
                ? 'border-[#33608D]/30 bg-[#33608D]/10 text-[var(--c-steel)]'
                : 'border-[var(--c-hair)] text-[var(--c-mut)] hover:bg-[var(--c-hover)]',
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
            <div className="rounded-lg border border-[#B3372B]/30 bg-[#B3372B]/10 p-4 text-sm text-[var(--c-rust)]">
              {diffData.error}
            </div>
          ) : diffData?.patch ? (
            <div>
              {diffData.command ? (
                <p className="mb-2 text-xs text-[var(--c-faint)]">
                  <code>{diffData.command}</code>
                </p>
              ) : null}
              <DiffViewer patch={diffData.patch} />
            </div>
          ) : (
            <div className="rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-8 text-center">
              <p className="text-sm text-[var(--c-mut)]">No diff available.</p>
            </div>
          )
        ) : null}
      </div>

      {/* Issues section. */}
      <div>
        <h3 className="mb-3 text-base font-semibold text-[var(--c-ink)]">
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
          <div className="rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-8 text-center">
            <p className="text-sm text-[var(--c-mut)]">
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

// renderMarkdown delegates to the shared markdown renderer that
// forces external links to open in a new tab.
const renderMarkdown = renderMarkdownToHtml;

// Match a thread message to an iteration by reviewer name and timing.
function findIterationMessage(
  messages: Message[] | undefined,
  iteration: ReviewIterationDetail,
): Message | undefined {
  if (!messages || messages.length === 0) return undefined;

  // Filter to messages from reviewer agents (sender name contains "reviewer").
  const reviewerMessages = messages.filter(
    (m) => m.sender_name.startsWith('reviewer-'),
  );

  // If only one reviewer message, return it for the first iteration.
  if (reviewerMessages.length === 1 && iteration.iteration_num === 1) {
    return reviewerMessages[0];
  }

  // For multiple iterations, match by order (iteration N = Nth reviewer message).
  if (iteration.iteration_num <= reviewerMessages.length) {
    return reviewerMessages[iteration.iteration_num - 1];
  }

  return undefined;
}

// IterationCard displays details for a single review iteration.
function IterationCard({
  iteration,
  threadMessages,
  threadLoading,
}: {
  iteration: ReviewIterationDetail;
  threadMessages?: Message[] | undefined;
  threadLoading?: boolean | undefined;
}) {
  const [expanded, setExpanded] = useState(false);
  const [showReview, setShowReview] = useState(false);

  const decisionLabel = iteration.decision
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());

  const badgeStyle = decisionStyles[iteration.decision] ?? 'bg-[#6B7280]/10 text-[var(--c-mut)]';

  // Find the reviewer's mail message for this iteration.
  const reviewMessage = useMemo(
    () => findIterationMessage(threadMessages, iteration),
    [threadMessages, iteration],
  );

  // Render the review message body as HTML.
  const renderedReviewBody = useMemo(
    () => (reviewMessage ? renderMarkdown(reviewMessage.body) : ''),
    [reviewMessage],
  );

  return (
    <div className="rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-4">
      {/* Iteration header. */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-[var(--c-ink)]">
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
          <span className="text-xs text-[var(--c-mut)]">
            by {iteration.reviewer_id}
          </span>
        ) : null}
      </div>

      {/* Metrics row. */}
      <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div>
          <dt className="text-xs text-[var(--c-mut)]">Files</dt>
          <dd className="text-sm font-medium text-[var(--c-ink)]">
            {iteration.files_reviewed}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-[var(--c-mut)]">Lines</dt>
          <dd className="text-sm font-medium text-[var(--c-ink)]">
            {iteration.lines_analyzed.toLocaleString()}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-[var(--c-mut)]">Duration</dt>
          <dd className="text-sm font-medium text-[var(--c-ink)]">
            {formatDuration(iteration.duration_ms)}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-[var(--c-mut)]">Cost</dt>
          <dd className="text-sm font-medium text-[var(--c-ink)]">
            {formatCost(iteration.cost_usd)}
          </dd>
        </div>
      </div>

      {/* Action buttons row. */}
      <div className="mt-3 flex items-center gap-3">
        {/* Summary toggle. */}
        {iteration.summary ? (
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="text-xs font-medium text-[var(--c-mut)] hover:text-[var(--c-ink)]"
          >
            {expanded ? 'Hide summary' : 'Show summary'}
          </button>
        ) : null}

        {/* Full review toggle. */}
        {threadLoading ? (
          <span className="text-xs text-[var(--c-faint)]">Loading review...</span>
        ) : reviewMessage ? (
          <button
            type="button"
            onClick={() => setShowReview(!showReview)}
            className={cn(
              'text-xs font-medium transition-colors',
              showReview
                ? 'text-[var(--c-steel)] hover:text-[var(--c-ink)]'
                : 'text-[var(--c-steel)] hover:text-[var(--c-ink)]',
            )}
          >
            {showReview ? 'Hide full review' : 'View full review'}
          </button>
        ) : null}
      </div>

      {/* Summary content. */}
      {expanded && iteration.summary ? (
        <div className="mt-2 rounded bg-[var(--c-hover)] p-3">
          <p className="text-sm text-[var(--c-ink)] whitespace-pre-wrap">
            {iteration.summary}
          </p>
        </div>
      ) : null}

      {/* Full review mail content. */}
      {showReview && reviewMessage ? (
        <div className="mt-2 rounded-lg border border-[#33608D]/20 bg-[#33608D]/5 p-4">
          <div className="mb-2 flex items-center gap-2 text-xs text-[var(--c-mut)]">
            <svg
              className="h-3.5 w-3.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
            <span>
              From <span className="font-medium text-[var(--c-ink)]">{reviewMessage.sender_name}</span>
              {' — '}
              {reviewMessage.subject}
            </span>
          </div>
          <div
            className="prose prose-sm max-w-none text-[var(--c-ink)]"
            dangerouslySetInnerHTML={{ __html: renderedReviewBody }}
          />
        </div>
      ) : null}

      {/* Timestamps. */}
      {iteration.started_at > 0 || iteration.completed_at > 0 ? (
        <div className="mt-3 flex gap-4 border-t border-[var(--c-fill)] pt-2">
          {iteration.started_at > 0 ? (
            <span className="text-xs text-[var(--c-faint)]">
              Started: {formatTimestamp(iteration.started_at)}
            </span>
          ) : null}
          {iteration.completed_at > 0 ? (
            <span className="text-xs text-[var(--c-faint)]">
              Completed: {formatTimestamp(iteration.completed_at)}
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
