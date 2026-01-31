// ActivityItem component - displays a single activity entry.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import type { Activity, ActivityType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Activity type icons and colors.
interface ActivityTypeConfig {
  icon: React.ReactNode;
  bgColor: string;
  iconColor: string;
  label: string;
}

// Mail icon.
function MailIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

// Check icon.
function CheckIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 13l4 4L19 7"
      />
    </svg>
  );
}

// Play icon.
function PlayIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

// Stop icon.
function StopIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z"
      />
    </svg>
  );
}

// User plus icon.
function UserPlusIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z"
      />
    </svg>
  );
}

// Heartbeat icon.
function HeartbeatIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
      />
    </svg>
  );
}

const activityTypeConfigs: Record<ActivityType, ActivityTypeConfig> = {
  message_sent: {
    icon: <MailIcon className="h-4 w-4" />,
    bgColor: 'bg-blue-100',
    iconColor: 'text-blue-600',
    label: 'Message sent',
  },
  message_read: {
    icon: <CheckIcon className="h-4 w-4" />,
    bgColor: 'bg-green-100',
    iconColor: 'text-green-600',
    label: 'Message read',
  },
  session_started: {
    icon: <PlayIcon className="h-4 w-4" />,
    bgColor: 'bg-purple-100',
    iconColor: 'text-purple-600',
    label: 'Session started',
  },
  session_completed: {
    icon: <StopIcon className="h-4 w-4" />,
    bgColor: 'bg-gray-100',
    iconColor: 'text-gray-600',
    label: 'Session completed',
  },
  agent_registered: {
    icon: <UserPlusIcon className="h-4 w-4" />,
    bgColor: 'bg-yellow-100',
    iconColor: 'text-yellow-600',
    label: 'Agent registered',
  },
  heartbeat: {
    icon: <HeartbeatIcon className="h-4 w-4" />,
    bgColor: 'bg-red-100',
    iconColor: 'text-red-500',
    label: 'Heartbeat',
  },
};

// Format relative time.
function formatRelativeTime(date: string): string {
  const now = Date.now();
  const then = new Date(date).getTime();
  const seconds = Math.floor((now - then) / 1000);

  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;

  return new Date(date).toLocaleDateString();
}

// Props for ActivityItem.
export interface ActivityItemProps {
  /** Activity data to display. */
  activity: Activity;
  /** Whether to show the agent avatar. */
  showAvatar?: boolean;
  /** Whether to show the full timestamp. */
  showFullTimestamp?: boolean;
  /** Additional class name. */
  className?: string;
}

export function ActivityItem({
  activity,
  showAvatar = true,
  showFullTimestamp = false,
  className,
}: ActivityItemProps) {
  const config = activityTypeConfigs[activity.type];

  return (
    <div className={cn('flex items-start gap-3 py-3', className)}>
      {/* Activity icon or avatar. */}
      {showAvatar ? (
        <div className="relative flex-shrink-0">
          <Avatar name={activity.agent_name} size="sm" />
          <span
            className={cn(
              'absolute -bottom-1 -right-1 flex h-5 w-5 items-center justify-center rounded-full',
              config.bgColor,
              config.iconColor,
            )}
          >
            {config.icon}
          </span>
        </div>
      ) : (
        <div
          className={cn(
            'flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full',
            config.bgColor,
            config.iconColor,
          )}
        >
          {config.icon}
        </div>
      )}

      {/* Content. */}
      <div className="min-w-0 flex-1">
        <p className="text-sm text-gray-900">
          <span className="font-medium">{activity.agent_name}</span>{' '}
          <span className="text-gray-600">{activity.description}</span>
        </p>
        <p className="mt-0.5 text-xs text-gray-500">
          {showFullTimestamp
            ? new Date(activity.created_at).toLocaleString()
            : formatRelativeTime(activity.created_at)}
        </p>
      </div>
    </div>
  );
}

// Skeleton for loading state.
export function ActivityItemSkeleton() {
  return (
    <div className="flex items-start gap-3 py-3">
      <div className="h-8 w-8 flex-shrink-0 animate-pulse rounded-full bg-gray-200" />
      <div className="min-w-0 flex-1 space-y-2">
        <div className="h-4 w-3/4 animate-pulse rounded bg-gray-200" />
        <div className="h-3 w-1/4 animate-pulse rounded bg-gray-200" />
      </div>
    </div>
  );
}
