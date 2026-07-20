// PlanStateBadge component - displays plan review state with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { PlanReviewState } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for plan review states.
const stateStyles: Record<PlanReviewState, string> = {
  pending: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
  approved: 'bg-[#178A5B]/10 text-[var(--c-green)]',
  rejected: 'bg-[#B3372B]/10 text-[var(--c-rust)]',
  changes_requested: 'bg-[#C98A1B]/12 text-[var(--c-amber)]',
};

// Human-readable labels for plan review states.
const stateLabels: Record<PlanReviewState, string> = {
  pending: 'Pending Review',
  approved: 'Approved',
  rejected: 'Rejected',
  changes_requested: 'Changes Requested',
};

export interface PlanStateBadgeProps {
  state: PlanReviewState;
  className?: string;
}

export function PlanStateBadge({ state, className }: PlanStateBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        stateStyles[state] ?? 'bg-[#6B7280]/8 text-[var(--c-mut)]',
        className,
      )}
    >
      {stateLabels[state] ?? state}
    </span>
  );
}
