// PlanListItem component - a single row in the plan reviews list.

import { Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { PlanReview } from '@/types/api.js';
import { PlanStateBadge } from './PlanStateBadge.js';
import { routes } from '@/lib/routes.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

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

export interface PlanListItemProps {
  planReview: PlanReview;
  className?: string;
}

export function PlanListItem({ planReview, className }: PlanListItemProps) {
  const title = planReview.plan_title || planReview.plan_path || 'Untitled Plan';

  return (
    <Link
      to={routes.plan(planReview.plan_review_id)}
      className={cn(
        'flex items-center gap-4 rounded-lg border border-gray-200 bg-white px-4 py-3',
        'transition-all hover:border-gray-300 hover:shadow-sm',
        className,
      )}
    >
      {/* Plan title and metadata. */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium text-gray-900">
            {title}
          </span>
          <PlanStateBadge state={planReview.state} />
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-gray-500">
          <span>by {planReview.reviewer_name || 'Unknown'}</span>
          <span className="text-gray-300">|</span>
          <span>ID: {planReview.plan_review_id.slice(0, 8)}</span>
          {planReview.session_id ? (
            <>
              <span className="text-gray-300">|</span>
              <span>Session: {planReview.session_id.slice(0, 8)}</span>
            </>
          ) : null}
        </div>
      </div>

      {/* Timestamp. */}
      <div className="shrink-0 text-xs text-gray-400">
        {formatRelativeTime(planReview.created_at)}
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
