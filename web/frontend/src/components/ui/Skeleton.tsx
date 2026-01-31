// Skeleton loading placeholders for content that is loading.

import { cn } from '@/lib/utils';

interface SkeletonProps {
  className?: string;
  variant?: 'text' | 'circular' | 'rectangular';
  width?: string | number;
  height?: string | number;
  animation?: 'pulse' | 'wave' | 'none';
}

// Skeleton displays a placeholder while content is loading.
export function Skeleton({
  className,
  variant = 'text',
  width,
  height,
  animation = 'pulse',
}: SkeletonProps) {
  const style: React.CSSProperties = {
    width: typeof width === 'number' ? `${width}px` : width,
    height: typeof height === 'number' ? `${height}px` : height,
  };

  const variantClasses = {
    text: 'h-4 rounded',
    circular: 'rounded-full',
    rectangular: 'rounded-md',
  };

  const animationClasses = {
    pulse: 'animate-pulse',
    wave: 'animate-shimmer bg-gradient-to-r from-gray-200 via-gray-100 to-gray-200 bg-[length:200%_100%]',
    none: '',
  };

  return (
    <div
      className={cn(
        'bg-gray-200',
        variantClasses[variant],
        animationClasses[animation],
        className
      )}
      style={style}
      aria-hidden="true"
    />
  );
}

// SkeletonText displays a text placeholder with optional lines.
interface SkeletonTextProps {
  lines?: number;
  className?: string;
  lineHeight?: string;
}

export function SkeletonText({
  lines = 3,
  className,
  lineHeight = 'h-4',
}: SkeletonTextProps) {
  return (
    <div className={cn('space-y-2', className)}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          variant="text"
          className={cn(
            lineHeight,
            // Make last line shorter for natural appearance.
            i === lines - 1 && lines > 1 && 'w-3/4'
          )}
        />
      ))}
    </div>
  );
}

// MessageRowSkeleton displays a skeleton for a message row.
export function MessageRowSkeleton() {
  return (
    <div className="flex items-center gap-4 border-b border-gray-100 px-4 py-3">
      {/* Checkbox placeholder. */}
      <Skeleton variant="rectangular" width={20} height={20} />
      {/* Star placeholder. */}
      <Skeleton variant="rectangular" width={20} height={20} />
      {/* Avatar placeholder. */}
      <Skeleton variant="circular" width={32} height={32} />
      {/* Content placeholder. */}
      <div className="flex-1 space-y-2">
        <div className="flex items-center gap-2">
          <Skeleton variant="text" width={120} className="h-4" />
          <Skeleton variant="text" width={200} className="h-4" />
        </div>
        <Skeleton variant="text" width="60%" className="h-3" />
      </div>
      {/* Timestamp placeholder. */}
      <Skeleton variant="text" width={60} className="h-3" />
    </div>
  );
}

// MessageListSkeleton displays skeleton for a list of messages.
interface MessageListSkeletonProps {
  count?: number;
}

export function MessageListSkeleton({ count = 10 }: MessageListSkeletonProps) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({ length: count }).map((_, i) => (
        <MessageRowSkeleton key={i} />
      ))}
    </div>
  );
}

// AgentCardSkeleton displays a skeleton for an agent card.
export function AgentCardSkeleton() {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-start gap-3">
        {/* Avatar placeholder. */}
        <Skeleton variant="circular" width={48} height={48} />
        <div className="flex-1 space-y-2">
          {/* Name placeholder. */}
          <Skeleton variant="text" width="60%" className="h-5" />
          {/* Status placeholder. */}
          <Skeleton variant="rectangular" width={80} height={20} className="rounded-full" />
        </div>
      </div>
      <div className="mt-4 space-y-2">
        <Skeleton variant="text" width="80%" className="h-3" />
        <Skeleton variant="text" width="60%" className="h-3" />
      </div>
    </div>
  );
}

// AgentGridSkeleton displays skeleton for a grid of agent cards.
interface AgentGridSkeletonProps {
  count?: number;
}

export function AgentGridSkeleton({ count = 6 }: AgentGridSkeletonProps) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }).map((_, i) => (
        <AgentCardSkeleton key={i} />
      ))}
    </div>
  );
}

// SessionRowSkeleton displays a skeleton for a session row.
export function SessionRowSkeleton() {
  return (
    <div className="flex items-center gap-4 border-b border-gray-100 px-4 py-3">
      {/* Icon placeholder. */}
      <Skeleton variant="circular" width={40} height={40} />
      <div className="flex-1 space-y-2">
        <div className="flex items-center gap-2">
          <Skeleton variant="text" width={150} className="h-4" />
          <Skeleton variant="rectangular" width={60} height={20} className="rounded-full" />
        </div>
        <Skeleton variant="text" width="40%" className="h-3" />
      </div>
      <Skeleton variant="text" width={80} className="h-3" />
    </div>
  );
}

// SessionListSkeleton displays skeleton for a list of sessions.
interface SessionListSkeletonProps {
  count?: number;
}

export function SessionListSkeleton({ count = 5 }: SessionListSkeletonProps) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({ length: count }).map((_, i) => (
        <SessionRowSkeleton key={i} />
      ))}
    </div>
  );
}

// StatsCardSkeleton displays a skeleton for a stats card.
export function StatsCardSkeleton() {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-center gap-3">
        <Skeleton variant="circular" width={40} height={40} />
        <div className="space-y-2">
          <Skeleton variant="text" width={60} className="h-3" />
          <Skeleton variant="text" width={40} className="h-6" />
        </div>
      </div>
    </div>
  );
}

// DashboardStatsSkeleton displays skeleton for dashboard stats.
export function DashboardStatsSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
      <StatsCardSkeleton />
      <StatsCardSkeleton />
      <StatsCardSkeleton />
      <StatsCardSkeleton />
    </div>
  );
}

// ActivityItemSkeleton displays a skeleton for an activity item.
export function ActivityItemSkeleton() {
  return (
    <div className="flex gap-3 py-2">
      <Skeleton variant="circular" width={32} height={32} />
      <div className="flex-1 space-y-1">
        <Skeleton variant="text" width="80%" className="h-4" />
        <Skeleton variant="text" width="40%" className="h-3" />
      </div>
    </div>
  );
}

// ActivityFeedSkeleton displays skeleton for an activity feed.
interface ActivityFeedSkeletonProps {
  count?: number;
}

export function ActivityFeedSkeleton({ count = 5 }: ActivityFeedSkeletonProps) {
  return (
    <div className="divide-y divide-gray-50">
      {Array.from({ length: count }).map((_, i) => (
        <ActivityItemSkeleton key={i} />
      ))}
    </div>
  );
}

// ThreadSkeleton displays a skeleton for a thread view.
export function ThreadSkeleton() {
  return (
    <div className="space-y-4 p-4">
      {/* Header. */}
      <div className="border-b border-gray-200 pb-4">
        <Skeleton variant="text" width="70%" className="h-6 mb-2" />
        <div className="flex items-center gap-2">
          <Skeleton variant="circular" width={32} height={32} />
          <Skeleton variant="text" width={100} className="h-4" />
        </div>
      </div>
      {/* Messages. */}
      <div className="space-y-4">
        <ThreadMessageSkeleton />
        <ThreadMessageSkeleton />
      </div>
    </div>
  );
}

// ThreadMessageSkeleton displays a skeleton for a thread message.
export function ThreadMessageSkeleton() {
  return (
    <div className="rounded-lg border border-gray-200 p-4">
      <div className="mb-3 flex items-center gap-2">
        <Skeleton variant="circular" width={36} height={36} />
        <div className="space-y-1">
          <Skeleton variant="text" width={100} className="h-4" />
          <Skeleton variant="text" width={80} className="h-3" />
        </div>
      </div>
      <SkeletonText lines={3} />
    </div>
  );
}

// SearchResultsSkeleton displays skeleton for search results.
export function SearchResultsSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="rounded-lg border border-gray-200 p-3">
          <div className="flex items-center gap-2 mb-2">
            <Skeleton variant="rectangular" width={60} height={20} className="rounded" />
            <Skeleton variant="text" width={200} className="h-4" />
          </div>
          <Skeleton variant="text" width="90%" className="h-3" />
        </div>
      ))}
    </div>
  );
}

export default Skeleton;
