// ActivityFeed component - displays a list of activity items with infinite scroll.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { ActivityItem, ActivityItemSkeleton } from './ActivityItem.js';
import type { Activity } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Props for ActivityFeed.
export interface ActivityFeedProps {
  /** List of activities to display. */
  activities?: Activity[];
  /** Whether data is loading initially. */
  isLoading?: boolean;
  /** Whether more data is being fetched. */
  isFetchingMore?: boolean;
  /** Whether there are more items to load. */
  hasMore?: boolean;
  /** Handler for loading more items. */
  onLoadMore?: () => void;
  /** Maximum height for the feed. */
  maxHeight?: string;
  /** Whether to show avatars. */
  showAvatars?: boolean;
  /** Error message. */
  error?: Error | null;
  /** Handler for retrying after error. */
  onRetry?: () => void;
  /** Additional class name. */
  className?: string;
}

// Empty state component.
function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center">
      <div className="mb-3 rounded-full bg-gray-100 p-3">
        <svg
          className="h-6 w-6 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      <p className="text-sm text-gray-500">No recent activity</p>
    </div>
  );
}

// Error state component.
function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center">
      <div className="mb-3 rounded-full bg-red-100 p-3">
        <svg
          className="h-6 w-6 text-red-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
      <p className="mb-2 text-sm text-gray-700">Failed to load activities</p>
      <p className="mb-3 text-xs text-gray-500">{message}</p>
      {onRetry ? (
        <button
          onClick={onRetry}
          className="text-sm font-medium text-blue-600 hover:text-blue-700"
        >
          Try again
        </button>
      ) : null}
    </div>
  );
}

// Loading skeleton for initial load.
function LoadingSkeleton({ count = 5 }: { count?: number }) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({ length: count }, (_, i) => (
        <ActivityItemSkeleton key={i} />
      ))}
    </div>
  );
}

export function ActivityFeed({
  activities,
  isLoading = false,
  isFetchingMore = false,
  hasMore = false,
  onLoadMore,
  maxHeight,
  showAvatars = true,
  error,
  onRetry,
  className,
}: ActivityFeedProps) {
  // Show error state.
  if (error) {
    return (
      <div className={cn('rounded-lg border border-gray-200 bg-white', className)}>
        <ErrorState message={error.message} onRetry={onRetry} />
      </div>
    );
  }

  // Show loading skeleton.
  if (isLoading) {
    return (
      <div className={cn('rounded-lg border border-gray-200 bg-white p-4', className)}>
        <LoadingSkeleton />
      </div>
    );
  }

  // Show empty state.
  if (!activities || activities.length === 0) {
    return (
      <div className={cn('rounded-lg border border-gray-200 bg-white', className)}>
        <EmptyState />
      </div>
    );
  }

  return (
    <div
      className={cn('rounded-lg border border-gray-200 bg-white', className)}
      style={maxHeight ? { maxHeight } : undefined}
    >
      <div
        className={cn(
          'divide-y divide-gray-100 px-4',
          maxHeight ? 'overflow-y-auto' : '',
        )}
        style={maxHeight ? { maxHeight } : undefined}
      >
        {activities.map((activity) => (
          <ActivityItem
            key={activity.id}
            activity={activity}
            showAvatar={showAvatars}
          />
        ))}

        {/* Load more button. */}
        {hasMore && onLoadMore ? (
          <div className="py-3 text-center">
            <button
              onClick={onLoadMore}
              disabled={isFetchingMore}
              className="text-sm font-medium text-blue-600 hover:text-blue-700 disabled:text-gray-400"
            >
              {isFetchingMore ? 'Loading...' : 'Load more'}
            </button>
          </div>
        ) : null}

        {/* Loading indicator for fetching more. */}
        {isFetchingMore && !hasMore ? (
          <div className="py-3">
            <ActivityItemSkeleton />
          </div>
        ) : null}
      </div>
    </div>
  );
}

// Compact version for sidebars.
export interface CompactActivityFeedProps {
  /** List of activities to display. */
  activities?: Activity[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Maximum number of items to show. */
  limit?: number;
  /** Additional class name. */
  className?: string;
}

export function CompactActivityFeed({
  activities,
  isLoading = false,
  limit = 5,
  className,
}: CompactActivityFeedProps) {
  if (isLoading) {
    return (
      <div className={cn('space-y-2', className)}>
        {Array.from({ length: limit }, (_, i) => (
          <div key={i} className="flex items-center gap-2">
            <div className="h-6 w-6 animate-pulse rounded-full bg-gray-200" />
            <div className="h-4 flex-1 animate-pulse rounded bg-gray-200" />
          </div>
        ))}
      </div>
    );
  }

  if (!activities || activities.length === 0) {
    return (
      <p className={cn('text-sm text-gray-500', className)}>No recent activity</p>
    );
  }

  const displayedActivities = activities.slice(0, limit);

  return (
    <div className={cn('space-y-1', className)}>
      {displayedActivities.map((activity) => (
        <ActivityItem
          key={activity.id}
          activity={activity}
          showAvatar={false}
          className="py-1.5"
        />
      ))}
    </div>
  );
}
