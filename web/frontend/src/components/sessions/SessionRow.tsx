// SessionRow component - single row in the session list table.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { Badge } from '@/components/ui/Badge.js';
import type { Session, SessionStatus } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Props for SessionRow.
export interface SessionRowProps {
  /** The session to display. */
  session: Session;
  /** Handler for clicking the row. */
  onClick?: () => void;
  /** Whether this row is selected. */
  isSelected?: boolean;
  /** Additional class name. */
  className?: string;
}

// Map session status to badge variant.
function getStatusVariant(status: SessionStatus): 'success' | 'warning' | 'default' {
  switch (status) {
    case 'active':
      return 'success';
    case 'completed':
      return 'default';
    case 'abandoned':
      return 'warning';
    default:
      return 'default';
  }
}

// Format duration from start time.
function formatDuration(startedAt: string, endedAt?: string): string {
  const start = new Date(startedAt);
  const end = endedAt ? new Date(endedAt) : new Date();
  const diffMs = end.getTime() - start.getTime();

  const minutes = Math.floor(diffMs / 60000);
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;

  if (hours > 0) {
    return `${hours}h ${remainingMinutes}m`;
  }
  return `${minutes}m`;
}

// Format relative time.
function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) {
    return `${diffDays}d ago`;
  }
  if (diffHours > 0) {
    return `${diffHours}h ago`;
  }
  if (diffMinutes > 0) {
    return `${diffMinutes}m ago`;
  }
  return 'just now';
}

export function SessionRow({
  session,
  onClick,
  isSelected = false,
  className,
}: SessionRowProps) {
  const duration = formatDuration(session.started_at, session.ended_at);
  const startedAgo = formatRelativeTime(session.started_at);

  return (
    <tr
      onClick={onClick}
      className={cn(
        'group border-b border-gray-100 transition-colors',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        isSelected ? 'bg-blue-50' : '',
        className,
      )}
    >
      {/* Agent. */}
      <td className="py-3 pl-4 pr-3">
        <div className="flex items-center gap-3">
          <Avatar
            name={session.agent_name}
            size="sm"
          />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-gray-900">
              {session.agent_name}
            </p>
            <p className="truncate text-xs text-gray-500">
              ID: {session.id}
            </p>
          </div>
        </div>
      </td>

      {/* Project/Branch. */}
      <td className="px-3 py-3">
        <div className="min-w-0">
          <p className="truncate text-sm text-gray-900">
            {session.project ?? '—'}
          </p>
          {session.branch ? (
            <p className="truncate text-xs text-gray-500">
              <span className="font-mono">{session.branch}</span>
            </p>
          ) : null}
        </div>
      </td>

      {/* Status. */}
      <td className="px-3 py-3">
        <Badge variant={getStatusVariant(session.status)}>
          {session.status}
        </Badge>
      </td>

      {/* Duration. */}
      <td className="px-3 py-3 text-sm text-gray-500">
        {duration}
      </td>

      {/* Started. */}
      <td className="px-3 py-3 text-sm text-gray-500">
        {startedAgo}
      </td>

      {/* Actions. */}
      <td className="py-3 pl-3 pr-4 text-right">
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onClick?.();
          }}
          className={cn(
            'text-sm text-blue-600 hover:text-blue-700',
            'opacity-0 transition-opacity group-hover:opacity-100',
          )}
        >
          View
        </button>
      </td>
    </tr>
  );
}

// Compact version for sidebars.
export interface CompactSessionRowProps {
  /** The session to display. */
  session: Session;
  /** Handler for clicking the row. */
  onClick?: () => void;
  /** Additional class name. */
  className?: string;
}

export function CompactSessionRow({
  session,
  onClick,
  className,
}: CompactSessionRowProps) {
  const duration = formatDuration(session.started_at, session.ended_at);

  return (
    <div
      onClick={onClick}
      className={cn(
        'flex items-center justify-between rounded-md px-3 py-2',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        className,
      )}
    >
      <div className="flex items-center gap-2 min-w-0">
        <Avatar name={session.agent_name} size="xs" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-gray-900">
            {session.agent_name}
          </p>
          {session.project ? (
            <p className="truncate text-xs text-gray-500">
              {session.project}
            </p>
          ) : null}
        </div>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        <Badge variant={getStatusVariant(session.status)} size="sm">
          {session.status}
        </Badge>
        <span className="text-xs text-gray-500">{duration}</span>
      </div>
    </div>
  );
}
