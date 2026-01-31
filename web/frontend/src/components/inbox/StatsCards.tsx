// StatsCards component - displays message statistics in card format.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Icon components for stats.
function InboxIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
      />
    </svg>
  );
}

function StarIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"
      />
    </svg>
  );
}

function UrgentIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
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
  );
}

function CheckIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

// Single stat card.
export interface StatCardProps {
  /** Label for the stat. */
  label: string;
  /** Numeric value to display. */
  value: number;
  /** Optional icon element. */
  icon?: React.ReactNode;
  /** Color variant for the card. */
  variant?: 'default' | 'blue' | 'yellow' | 'red' | 'green';
  /** Click handler. */
  onClick?: () => void;
  /** Whether the card is clickable. */
  clickable?: boolean;
  /** Additional class name. */
  className?: string;
}

// Variant styles for cards.
const variantStyles = {
  default: {
    bg: 'bg-white',
    icon: 'bg-gray-100 text-gray-600',
    text: 'text-gray-900',
  },
  blue: {
    bg: 'bg-blue-50',
    icon: 'bg-blue-100 text-blue-600',
    text: 'text-blue-900',
  },
  yellow: {
    bg: 'bg-yellow-50',
    icon: 'bg-yellow-100 text-yellow-600',
    text: 'text-yellow-900',
  },
  red: {
    bg: 'bg-red-50',
    icon: 'bg-red-100 text-red-600',
    text: 'text-red-900',
  },
  green: {
    bg: 'bg-green-50',
    icon: 'bg-green-100 text-green-600',
    text: 'text-green-900',
  },
};

export function StatCard({
  label,
  value,
  icon,
  variant = 'default',
  onClick,
  clickable = false,
  className,
}: StatCardProps) {
  const styles = variantStyles[variant];
  const isClickable = clickable || onClick !== undefined;

  const content = (
    <>
      {icon ? (
        <div
          className={cn(
            'flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg',
            styles.icon,
          )}
        >
          {icon}
        </div>
      ) : null}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-gray-500 truncate">{label}</p>
        <p className={cn('text-2xl font-semibold', styles.text)}>{value}</p>
      </div>
    </>
  );

  if (isClickable) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={cn(
          'flex items-center gap-4 rounded-lg border border-gray-200 p-4 text-left transition-colors',
          styles.bg,
          'hover:border-gray-300 hover:shadow-sm',
          'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
          className,
        )}
      >
        {content}
      </button>
    );
  }

  return (
    <div
      className={cn(
        'flex items-center gap-4 rounded-lg border border-gray-200 p-4',
        styles.bg,
        className,
      )}
    >
      {content}
    </div>
  );
}

// Stats for inbox.
export interface InboxStatsProps {
  /** Number of unread messages. */
  unread: number;
  /** Number of starred messages. */
  starred: number;
  /** Number of urgent messages. */
  urgent: number;
  /** Number of acknowledged messages (completed today). */
  acknowledged: number;
  /** Handler for clicking a stat card. */
  onStatClick?: (stat: 'unread' | 'starred' | 'urgent' | 'acknowledged') => void;
  /** Loading state. */
  isLoading?: boolean;
  /** Additional class name for the container. */
  className?: string;
}

export function InboxStats({
  unread,
  starred,
  urgent,
  acknowledged,
  onStatClick,
  isLoading = false,
  className,
}: InboxStatsProps) {
  if (isLoading) {
    return (
      <div className={cn('grid grid-cols-2 gap-4 md:grid-cols-4', className)}>
        {[1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className="h-20 animate-pulse rounded-lg border border-gray-200 bg-gray-100"
          />
        ))}
      </div>
    );
  }

  return (
    <div className={cn('grid grid-cols-2 gap-4 md:grid-cols-4', className)}>
      <StatCard
        label="Unread"
        value={unread}
        icon={<InboxIcon />}
        variant="blue"
        clickable={onStatClick !== undefined}
        onClick={() => onStatClick?.('unread')}
      />
      <StatCard
        label="Starred"
        value={starred}
        icon={<StarIcon />}
        variant="yellow"
        clickable={onStatClick !== undefined}
        onClick={() => onStatClick?.('starred')}
      />
      <StatCard
        label="Urgent"
        value={urgent}
        icon={<UrgentIcon />}
        variant="red"
        clickable={onStatClick !== undefined}
        onClick={() => onStatClick?.('urgent')}
      />
      <StatCard
        label="Completed"
        value={acknowledged}
        icon={<CheckIcon />}
        variant="green"
        clickable={onStatClick !== undefined}
        onClick={() => onStatClick?.('acknowledged')}
      />
    </div>
  );
}
