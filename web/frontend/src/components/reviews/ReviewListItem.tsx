// ReviewListItem component - a single row in the reviews list.

import { Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { ReviewSummary } from '@/types/api.js';
import { ReviewStateBadge } from './ReviewStateBadge.js';
import { routes } from '@/lib/routes.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Review type labels and styles.
const typeStyles: Record<string, string> = {
  full: 'bg-purple-50 text-purple-700',
  incremental: 'bg-cyan-50 text-cyan-700',
  security: 'bg-red-50 text-red-700',
  performance: 'bg-amber-50 text-amber-700',
};

// Format Unix timestamp to relative time.
function formatRelativeTime(unixTimestamp: number): string {
  if (unixTimestamp === 0) return '';

  const now = Date.now();
  const ts = unixTimestamp * 1000;
  const diff = now - ts;

  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;

  return new Date(ts).toLocaleDateString();
}

export interface ReviewListItemProps {
  review: ReviewSummary;
  className?: string;
}

export function ReviewListItem({ review, className }: ReviewListItemProps) {
  return (
    <Link
      to={routes.review(review.review_id)}
      className={cn(
        'flex items-center gap-4 rounded-lg border border-gray-200 bg-white px-4 py-3',
        'transition-all hover:border-gray-300 hover:shadow-sm',
        className,
      )}
    >
      {/* Branch name as main identifier. */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium text-gray-900">
            {review.branch}
          </span>
          <ReviewStateBadge state={review.state} />
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-gray-500">
          <span
            className={cn(
              'rounded px-1.5 py-0.5 text-xs font-medium',
              typeStyles[review.review_type] ?? 'bg-gray-50 text-gray-600',
            )}
          >
            {review.review_type}
          </span>
          <span className="text-gray-300">|</span>
          <span>ID: {review.review_id.slice(0, 8)}</span>
        </div>
      </div>

      {/* Timestamp. */}
      <div className="shrink-0 text-xs text-gray-400">
        {formatRelativeTime(review.created_at)}
      </div>

      {/* Chevron arrow. */}
      <svg
        className="h-4 w-4 shrink-0 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9 5l7 7-7 7"
        />
      </svg>
    </Link>
  );
}
