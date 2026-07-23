// ReviewStateBadge component - displays review state with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { ReviewState } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for review states.
const stateStyles: Record<ReviewState, string> = {
  pending_review: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
  under_review: 'bg-[#33608D]/10 text-[var(--c-steel)]',
  changes_requested: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
  approved: 'bg-[#178A5B]/10 text-[var(--c-green)]',
  rejected: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  cancelled: 'bg-[#6B7280]/10 text-[var(--c-mut)]',
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
        stateStyles[state] ?? 'bg-[#6B7280]/10 text-[var(--c-mut)]',
        className,
      )}
    >
      {stateLabels[state] ?? state}
    </span>
  );
}
