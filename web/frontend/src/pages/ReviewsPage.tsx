// ReviewsPage component - list and detail view for code reviews.

import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useReviews, useReview } from '@/hooks/useReviews.js';
import { ReviewListItem, ReviewDetailView } from '@/components/reviews/index.js';
import { Spinner } from '@/components/ui/Spinner.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// State filter options.
const stateFilters: Array<{ label: string; value: string }> = [
  { label: 'All', value: '' },
  { label: 'In Review', value: 'under_review' },
  { label: 'Changes Requested', value: 'changes_requested' },
  { label: 'Approved', value: 'approved' },
  { label: 'Rejected', value: 'rejected' },
  { label: 'Cancelled', value: 'cancelled' },
];

export default function ReviewsPage() {
  const { reviewId } = useParams<{ reviewId?: string }>();
  const [stateFilter, setStateFilter] = useState('');

  // Fetch review detail if an ID is in the URL.
  const {
    data: reviewDetail,
    isLoading: detailLoading,
    error: detailError,
  } = useReview(reviewId ?? '', reviewId !== undefined);

  // Fetch reviews list.
  // Build options, only including state when there is a filter.
  const listOptions = stateFilter
    ? { state: stateFilter, limit: 50 }
    : { limit: 50 };

  const {
    data: reviews,
    isLoading: listLoading,
    error: listError,
  } = useReviews(listOptions);

  // If we have a review ID in the URL, show the detail view.
  if (reviewId) {
    if (detailLoading) {
      return (
        <div className="flex h-full items-center justify-center p-6">
          <Spinner size="lg" variant="primary" label="Loading review..." />
        </div>
      );
    }

    if (detailError) {
      return (
        <div className="p-6">
          <div className="rounded-lg border border-[#B3372B]/30 bg-[#B3372B]/10 p-6 text-center">
            <p className="text-sm text-[var(--c-rust)]">
              Failed to load review: {detailError.message}
            </p>
          </div>
        </div>
      );
    }

    if (reviewDetail) {
      return (
        <div className="p-6">
          <ReviewDetailView review={reviewDetail} />
        </div>
      );
    }

    return null;
  }

  // Show the reviews list.
  return (
    <div className="p-6">
      {/* Page header. */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--c-ink)]">Code Reviews</h1>
        <p className="mt-1 text-sm text-[var(--c-mut)]">
          Track and manage automated code review requests.
        </p>
      </div>

      {/* State filter tabs. */}
      <div className="mb-4 flex gap-1 rounded-lg bg-[var(--c-fill)] p-1">
        {stateFilters.map((filter) => (
          <button
            key={filter.value}
            type="button"
            onClick={() => setStateFilter(filter.value)}
            className={cn(
              'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              stateFilter === filter.value
                ? 'bg-[var(--c-card)] text-[var(--c-ink)] shadow-sm'
                : 'text-[var(--c-mut)] hover:text-[var(--c-ink)]',
            )}
          >
            {filter.label}
          </button>
        ))}
      </div>

      {/* Reviews list. */}
      {listLoading ? (
        <div className="flex justify-center py-12">
          <Spinner size="lg" variant="primary" label="Loading reviews..." />
        </div>
      ) : listError ? (
        <div className="rounded-lg border border-[#B3372B]/30 bg-[#B3372B]/10 p-6 text-center">
          <p className="text-sm text-[var(--c-rust)]">
            Failed to load reviews: {listError.message}
          </p>
        </div>
      ) : reviews && reviews.length > 0 ? (
        <div className="space-y-2">
          {reviews.map((review) => (
            <ReviewListItem key={review.review_id} review={review} />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] p-12 text-center">
          <svg
            className="mx-auto h-12 w-12 text-[var(--c-faint)]"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            />
          </svg>
          <h3 className="mt-4 text-sm font-medium text-[var(--c-ink)]">No reviews</h3>
          <p className="mt-1 text-sm text-[var(--c-mut)]">
            {stateFilter
              ? `No reviews with state "${stateFilter.replace('_', ' ')}".`
              : 'No reviews have been created yet.'}
          </p>
          <p className="mt-3 text-xs text-[var(--c-faint)]">
            Use <code className="rounded bg-[var(--c-fill)] px-1">substrate review request</code> to request a code review.
          </p>
        </div>
      )}
    </div>
  );
}
