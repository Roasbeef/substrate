// PlanStateBadge component - displays plan review state with color-coded badge.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { PlanReviewState } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Color mapping for plan review states.
const stateStyles: Record<PlanReviewState, string> = {
  pending: 'bg-amber-100 text-amber-800',
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-red-100 text-red-800',
  changes_requested: 'bg-yellow-100 text-yellow-800',
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
        stateStyles[state] ?? 'bg-gray-100 text-gray-600',
        className,
      )}
    >
      {stateLabels[state] ?? state}
    </span>
  );
}
