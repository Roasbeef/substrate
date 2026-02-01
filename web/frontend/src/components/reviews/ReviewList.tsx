// ReviewList component - displays a filterable list of reviews.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useReviews, useReviewStats } from '@/hooks/useReviews.js';
import { useReviewListRealtime } from '@/hooks/useReviewsRealtime.js';
import { ReviewRow, CompactReviewRow } from './ReviewRow.js';
import { Skeleton } from '@/components/ui/Skeleton.js';
import { Button } from '@/components/ui/Button.js';
import type { Review, ReviewState, ListReviewsParams } from '@/types/reviews.js';

// Combine clsx and tailwind-merge.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Empty state icon.
function EmptyIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.5}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
      />
    </svg>
  );
}

// Refresh icon.
function RefreshIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
      />
    </svg>
  );
}

// Filter buttons for review states.
const stateFilters: Array<{ value: ReviewState | 'all'; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'pending_review', label: 'Pending' },
  { value: 'under_review', label: 'In Review' },
  { value: 'changes_requested', label: 'Changes' },
  { value: 'approved', label: 'Approved' },
];

interface FilterTabProps {
  label: string;
  isActive: boolean;
  onClick: () => void;
  count?: number;
}

function FilterTab({ label, isActive, onClick, count }: FilterTabProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'px-3 py-1.5 text-sm font-medium rounded-md transition-colors',
        isActive
          ? 'bg-blue-100 text-blue-700'
          : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
      )}
    >
      {label}
      {count !== undefined && count > 0 && (
        <span
          className={cn(
            'ml-1.5 px-1.5 py-0.5 text-xs rounded-full',
            isActive ? 'bg-blue-200 text-blue-800' : 'bg-gray-200 text-gray-600',
          )}
        >
          {count}
        </span>
      )}
    </button>
  );
}

export interface ReviewListProps {
  /** Pre-filter to a specific requester. */
  requesterId?: number;
  /** Callback when a review is clicked. */
  onSelectReview?: (review: Review) => void;
  /** Show filter tabs. */
  showFilters?: boolean;
  /** Compact mode for sidebars. */
  compact?: boolean;
  /** Maximum number of reviews to show. */
  limit?: number;
  /** Additional class name. */
  className?: string;
}

export function ReviewList({
  requesterId,
  onSelectReview,
  showFilters = true,
  compact = false,
  limit = 50,
  className,
}: ReviewListProps) {
  const [activeFilter, setActiveFilter] = useState<ReviewState | 'all'>('all');

  // Build query params.
  const params: ListReviewsParams = {
    limit,
  };
  if (activeFilter !== 'all') {
    params.filter = activeFilter;
  }
  if (requesterId) {
    params.requester_id = requesterId;
  }

  // Fetch reviews and stats.
  const { data, isLoading, error, refetch } = useReviews(params);
  const { data: stats } = useReviewStats();

  // Subscribe to real-time updates.
  useReviewListRealtime();

  // Loading state.
  if (isLoading) {
    return (
      <div className={cn('space-y-2 p-4', className)}>
        {[...Array(5)].map((_, i) => (
          <Skeleton key={i} className="h-16 rounded-lg" />
        ))}
      </div>
    );
  }

  // Error state.
  if (error) {
    return (
      <div className={cn('p-4 text-center', className)}>
        <p className="text-sm text-red-600 mb-2">Failed to load reviews</p>
        <Button size="sm" variant="secondary" onClick={() => refetch()}>
          <RefreshIcon className="h-4 w-4 mr-1" />
          Retry
        </Button>
      </div>
    );
  }

  const reviews = data?.reviews ?? [];

  // Get counts from stats.
  const pendingCount = stats?.pending ?? 0;
  const inProgressCount = stats?.in_progress ?? 0;
  const changesCount = stats?.changes_requested ?? 0;
  const approvedCount = stats?.approved ?? 0;

  return (
    <div className={cn('flex flex-col', className)}>
      {/* Filter tabs */}
      {showFilters && (
        <div className="flex items-center gap-1 px-4 py-2 border-b border-gray-200 overflow-x-auto">
          <FilterTab
            label="All"
            isActive={activeFilter === 'all'}
            onClick={() => setActiveFilter('all')}
          />
          <FilterTab
            label="Pending"
            isActive={activeFilter === 'pending_review'}
            onClick={() => setActiveFilter('pending_review')}
            count={pendingCount}
          />
          <FilterTab
            label="In Review"
            isActive={activeFilter === 'under_review'}
            onClick={() => setActiveFilter('under_review')}
            count={inProgressCount}
          />
          <FilterTab
            label="Changes"
            isActive={activeFilter === 'changes_requested'}
            onClick={() => setActiveFilter('changes_requested')}
            count={changesCount}
          />
          <FilterTab
            label="Approved"
            isActive={activeFilter === 'approved'}
            onClick={() => setActiveFilter('approved')}
            count={approvedCount}
          />
        </div>
      )}

      {/* Review list */}
      {reviews.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-gray-500">
          <EmptyIcon className="h-12 w-12 mb-2" />
          <p className="text-sm">No reviews found</p>
          {activeFilter !== 'all' && (
            <button
              type="button"
              onClick={() => setActiveFilter('all')}
              className="mt-1 text-sm text-blue-600 hover:underline"
            >
              Show all reviews
            </button>
          )}
        </div>
      ) : compact ? (
        <div className="flex-1 overflow-y-auto">
          {reviews.map((review) => (
            <CompactReviewRow
              key={review.review_id}
              review={review}
              onClick={onSelectReview ? () => onSelectReview(review) : undefined}
            />
          ))}
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto">
          {reviews.map((review) => (
            <ReviewRow
              key={review.review_id}
              review={review}
              onClick={onSelectReview ? () => onSelectReview(review) : undefined}
            />
          ))}
        </div>
      )}
    </div>
  );
}
