// ReviewStateBadge component - displays review state with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { ReviewState } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for review states.
const stateStyles: Record<ReviewState, string> = {
  pending_review: 'bg-yellow-100 text-yellow-800',
  under_review: 'bg-blue-100 text-blue-800',
  changes_requested: 'bg-orange-100 text-orange-800',
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-red-100 text-red-800',
  cancelled: 'bg-gray-100 text-gray-600',
};

// Human-readable labels for review states.
const stateLabels: Record<ReviewState, string> = {
  pending_review: 'Pending',
  under_review: 'In Review',
  changes_requested: 'Changes Requested',
  approved: 'Approved',
  rejected: 'Rejected',
  cancelled: 'Cancelled',
};

export interface ReviewStateBadgeProps {
  state: ReviewState;
  className?: string;
}

export function ReviewStateBadge({ state, className }: ReviewStateBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        stateStyles[state] ?? 'bg-gray-100 text-gray-600',
        className,
      )}
    >
      {stateLabels[state] ?? state}
    </span>
  );
}
