// PlansPage component - list and detail view for plan reviews.

import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { usePlanReviews, usePlanReview } from '@/hooks/usePlanReviews.js';
import { PlanListItem, PlanDetailView } from '@/components/plans/index.js';
import { Spinner } from '@/components/ui/Spinner.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// State filter options.
const stateFilters: Array<{ label: string; value: string }> = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Approved', value: 'approved' },
  { label: 'Rejected', value: 'rejected' },
  { label: 'Changes Requested', value: 'changes_requested' },
];

export default function PlansPage() {
  const { planReviewId } = useParams<{ planReviewId?: string }>();
  const [stateFilter, setStateFilter] = useState('');

  // Fetch plan review detail if an ID is in the URL.
  const {
    data: planDetail,
    isLoading: detailLoading,
    error: detailError,
  } = usePlanReview(planReviewId ?? '', planReviewId !== undefined);

  // Fetch plan reviews list.
  const listOptions = stateFilter
    ? { state: stateFilter, limit: 50 }
    : { limit: 50 };

  const {
    data: planReviews,
    isLoading: listLoading,
    error: listError,
  } = usePlanReviews(listOptions);

  // If we have a plan review ID in the URL, show the detail view.
  if (planReviewId) {
    if (detailLoading) {
      return (
        <div className="flex h-full items-center justify-center p-6">
          <Spinner size="lg" variant="primary" label="Loading plan..." />
        </div>
      );
    }

    if (detailError) {
      return (
        <div className="p-6">
          <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center">
            <p className="text-sm text-red-700">
              Failed to load plan review: {detailError.message}
            </p>
          </div>
        </div>
      );
    }

    if (planDetail) {
      return (
        <div className="p-6">
          <PlanDetailView planReview={planDetail} />
        </div>
      );
    }

    return null;
  }

  // Show the plan reviews list.
  return (
    <div className="p-6">
      {/* Page header. */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Plan Reviews</h1>
        <p className="mt-1 text-sm text-gray-500">
          Review and approve agent implementation plans before execution.
        </p>
      </div>

      {/* State filter tabs. */}
      <div className="mb-4 flex gap-1 rounded-lg bg-gray-100 p-1">
        {stateFilters.map((filter) => (
          <button
            key={filter.value}
            type="button"
            onClick={() => setStateFilter(filter.value)}
            className={cn(
              'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              stateFilter === filter.value
                ? 'bg-white text-gray-900 shadow-sm'
                : 'text-gray-600 hover:text-gray-900',
            )}
          >
            {filter.label}
          </button>
        ))}
      </div>

      {/* Plan reviews list. */}
      {listLoading ? (
        <div className="flex justify-center py-12">
          <Spinner size="lg" variant="primary" label="Loading plan reviews..." />
        </div>
      ) : listError ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center">
          <p className="text-sm text-red-700">
            Failed to load plan reviews: {listError.message}
          </p>
        </div>
      ) : planReviews && planReviews.length > 0 ? (
        <div className="space-y-2">
          {planReviews.map((pr) => (
            <PlanListItem key={pr.plan_review_id} planReview={pr} />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-gray-200 bg-white p-12 text-center">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
          <h3 className="mt-4 text-sm font-medium text-gray-900">No plan reviews</h3>
          <p className="mt-1 text-sm text-gray-500">
            {stateFilter
              ? `No plan reviews with state "${stateFilter.replace('_', ' ')}".`
              : 'No plan reviews have been submitted yet.'}
          </p>
          <p className="mt-3 text-xs text-gray-400">
            Plans are submitted automatically when agents call ExitPlanMode.
          </p>
        </div>
      )}
    </div>
  );
}
