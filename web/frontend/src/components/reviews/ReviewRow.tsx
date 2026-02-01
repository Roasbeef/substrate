// ReviewRow component - a single row in the reviews list.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { ReviewStateBadge, SeverityBadge } from './ReviewBadges.js';
import type { Review } from '@/types/reviews.js';

// Combine clsx and tailwind-merge.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Git branch icon.
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

// PR icon.
function PullRequestIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 7a2 2 0 11-4 0 2 2 0 014 0zM7 17a2 2 0 11-4 0 2 2 0 014 0zM21 7a2 2 0 11-4 0 2 2 0 014 0zM5 7v10M19 7v10"
      />
    </svg>
  );
}

// External link icon.
function ExternalLinkIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
      />
    </svg>
  );
}

// Format relative time.
function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  });
}

// Truncate branch name if too long.
function truncateBranch(branch: string, maxLength = 30): string {
  if (branch.length <= maxLength) return branch;
  return branch.slice(0, maxLength - 3) + '...';
}

export interface ReviewRowProps {
  review: Review;
  onClick?: () => void;
  showRequester?: boolean;
  className?: string;
}

export function ReviewRow({
  review,
  onClick,
  showRequester = true,
  className,
}: ReviewRowProps) {
  // Check if review is in active state.
  const isActive = ['under_review', 'pending_review', 'changes_requested', 're_review'].includes(
    review.state,
  );

  return (
    <div
      className={cn(
        'flex items-center gap-3 border-b border-gray-100 px-4 py-3 transition-colors',
        isActive ? 'bg-blue-50/30' : 'bg-white',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
    >
      {/* Requester avatar */}
      {showRequester && (
        <Avatar
          name={review.requester_name ?? `Agent ${review.requester_id}`}
          size="sm"
          className="flex-shrink-0"
        />
      )}

      {/* Branch info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <BranchIcon className="h-4 w-4 text-gray-400 flex-shrink-0" />
          <span className="font-mono text-sm font-medium text-gray-900 truncate">
            {truncateBranch(review.branch)}
          </span>

          {/* PR number if available */}
          {review.pr_number && (
            <span className="text-sm text-gray-500 flex items-center gap-1">
              <PullRequestIcon className="h-3.5 w-3.5" />#{review.pr_number}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-gray-500">
            {review.base_branch}
          </span>
          <span className="text-xs text-gray-400">|</span>
          <span className="text-xs text-gray-500">
            {review.review_type}
          </span>
          {review.priority !== 'normal' && (
            <>
              <span className="text-xs text-gray-400">|</span>
              <span
                className={cn(
                  'text-xs font-medium',
                  review.priority === 'urgent' ? 'text-red-600' : 'text-gray-600',
                )}
              >
                {review.priority}
              </span>
            </>
          )}
        </div>
      </div>

      {/* State badge */}
      <ReviewStateBadge state={review.state} />

      {/* Timestamp */}
      <span className="flex-shrink-0 text-xs text-gray-500">
        {formatRelativeTime(review.created_at)}
      </span>

      {/* External link if PR */}
      {review.pr_number && (
        <a
          href={`https://github.com/${review.repo_path}/pull/${review.pr_number}`}
          target="_blank"
          rel="noopener noreferrer"
          className="p-1 text-gray-400 hover:text-gray-600"
          onClick={(e) => e.stopPropagation()}
          aria-label="Open PR in GitHub"
        >
          <ExternalLinkIcon className="h-4 w-4" />
        </a>
      )}
    </div>
  );
}

// Compact variant for sidebars.
export function CompactReviewRow({
  review,
  onClick,
  className,
}: {
  review: Review;
  onClick?: () => void;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 px-3 py-2 border-b border-gray-100 transition-colors',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      <BranchIcon className="h-3.5 w-3.5 text-gray-400 flex-shrink-0" />
      <span className="font-mono text-xs text-gray-700 truncate flex-1">
        {truncateBranch(review.branch, 20)}
      </span>
      <ReviewStateBadge state={review.state} size="sm" showDot={false} />
    </div>
  );
}
