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
  full: 'bg-[#5B5BD6]/10 text-[var(--c-violet)]',
  incremental: 'bg-[#0E7490]/10 text-[var(--c-teal)]',
  security: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  performance: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
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
        'flex items-center gap-4 rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] px-4 py-3',
        'transition-all hover:border-[var(--c-ghost2)] hover:shadow-sm',
        className,
      )}
    >
      {/* Branch name as main identifier. */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium text-[var(--c-ink)]">
            {review.branch}
          </span>
          <ReviewStateBadge state={review.state} />
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-[var(--c-mut)]">
          <span
            className={cn(
              'rounded px-1.5 py-0.5 text-xs font-medium',
              typeStyles[review.review_type] ?? 'bg-[var(--c-fill)] text-[var(--c-mut)]',
            )}
          >
            {review.review_type}
          </span>
          <span className="text-[var(--c-hair)]">|</span>
          <span>ID: {review.review_id.slice(0, 8)}</span>
        </div>
      </div>

      {/* Timestamp. */}
      <div className="shrink-0 text-xs text-[var(--c-faint)]">
        {formatRelativeTime(review.created_at)}
      </div>

      {/* Chevron arrow. */}
      <svg
        className="h-4 w-4 shrink-0 text-[var(--c-faint)]"
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
