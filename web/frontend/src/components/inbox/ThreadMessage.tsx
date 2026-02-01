// ThreadMessage component - a single message within a thread view.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { PriorityBadge } from '@/components/ui/Badge.js';
import type { Message } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format date for message display.
function formatMessageDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  const isThisYear = date.getFullYear() === now.getFullYear();

  if (isToday) {
    return date.toLocaleTimeString(undefined, {
      hour: 'numeric',
      minute: '2-digit',
    });
  }

  if (isThisYear) {
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  }

  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

// Simple markdown-like rendering (basic bold, italic, code).
function renderMarkdown(text: string): React.ReactNode {
  // Split by code blocks first.
  const parts = text.split(/(`[^`]+`)/g);

  return parts.map((part, index) => {
    // Handle inline code.
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code
          key={index}
          className="rounded bg-gray-100 px-1 py-0.5 font-mono text-sm"
        >
          {part.slice(1, -1)}
        </code>
      );
    }

    // Handle bold (**text**).
    let content: React.ReactNode = part;
    content = part.split(/(\*\*[^*]+\*\*)/g).map((segment, i) => {
      if (segment.startsWith('**') && segment.endsWith('**')) {
        return <strong key={i}>{segment.slice(2, -2)}</strong>;
      }
      return segment;
    });

    return <span key={index}>{content}</span>;
  });
}

// Props for ThreadMessage component.
export interface ThreadMessageProps {
  /** The message to display. */
  message: Message;
  /** Whether this is the first (original) message in the thread. */
  isFirst?: boolean;
  /** Whether this message is currently focused. */
  isFocused?: boolean;
  /** Additional class name. */
  className?: string;
}

export function ThreadMessage({
  message,
  isFirst = false,
  isFocused = false,
  className,
}: ThreadMessageProps) {
  return (
    <div
      className={cn(
        'rounded-lg border bg-white p-4 transition-colors',
        isFocused ? 'border-blue-300 ring-2 ring-blue-100' : 'border-gray-200',
        className,
      )}
      role="article"
      aria-label={`Message from ${message.sender_name}`}
    >
      {/* Message header. */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <Avatar name={message.sender_name} size="md" />
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium text-gray-900">
                {message.sender_name}
              </span>
              {message.priority !== 'normal' ? (
                <PriorityBadge priority={message.priority} size="sm" />
              ) : null}
            </div>
            <span className="text-sm text-gray-500">
              {formatMessageDate(message.created_at)}
            </span>
          </div>
        </div>

        {/* Show subject only for first message. */}
        {isFirst ? null : (
          <span className="text-xs text-gray-400">Reply</span>
        )}
      </div>

      {/* Subject (only for first message). */}
      {isFirst ? (
        <h2 className="mt-3 text-lg font-semibold text-gray-900">
          {message.subject}
        </h2>
      ) : null}

      {/* Message body. */}
      <div className="mt-3 whitespace-pre-wrap text-gray-700">
        {renderMarkdown(message.body)}
      </div>
    </div>
  );
}

// Compact variant for collapsed messages.
export interface CompactThreadMessageProps {
  /** The message to display. */
  message: Message;
  /** Handler for clicking to expand. */
  onClick?: () => void;
  /** Additional class name. */
  className?: string;
}

export function CompactThreadMessage({
  message,
  onClick,
  className,
}: CompactThreadMessageProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 p-3 text-left',
        'hover:bg-gray-100 transition-colors',
        'focus:outline-none focus:ring-2 focus:ring-blue-500',
        className,
      )}
    >
      <Avatar name={message.sender_name} size="sm" />
      <div className="min-w-0 flex-1">
        <span className="truncate text-sm font-medium text-gray-900">
          {message.sender_name}
        </span>
        <span className="ml-2 truncate text-sm text-gray-500">
          {message.body.slice(0, 80)}
          {message.body.length > 80 ? '...' : ''}
        </span>
      </div>
      <span className="flex-shrink-0 text-xs text-gray-400">
        {formatMessageDate(message.created_at)}
      </span>
    </button>
  );
}

// Deadline banner for messages with deadlines.
export interface DeadlineBannerProps {
  /** The deadline date string. */
  deadline: string;
  /** Whether the deadline has passed. */
  isPast?: boolean;
  /** Handler for acknowledging. */
  onAcknowledge?: () => void;
  /** Whether the ack action is loading. */
  isLoading?: boolean;
  /** Additional class name. */
  className?: string;
}

export function DeadlineBanner({
  deadline,
  isPast = false,
  onAcknowledge,
  isLoading = false,
  className,
}: DeadlineBannerProps) {
  const date = new Date(deadline);
  const formattedDate = date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });

  return (
    <div
      className={cn(
        'flex items-center justify-between rounded-lg px-4 py-3',
        isPast ? 'bg-red-50' : 'bg-yellow-50',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <svg
          className={cn('h-5 w-5', isPast ? 'text-red-500' : 'text-yellow-500')}
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
        <span
          className={cn(
            'text-sm font-medium',
            isPast ? 'text-red-700' : 'text-yellow-700',
          )}
        >
          {isPast ? 'Deadline passed: ' : 'Deadline: '}
          {formattedDate}
        </span>
      </div>

      {onAcknowledge ? (
        <button
          type="button"
          onClick={onAcknowledge}
          disabled={isLoading}
          className={cn(
            'rounded px-3 py-1 text-sm font-medium transition-colors',
            'focus:outline-none focus:ring-2 focus:ring-offset-2',
            isPast
              ? 'bg-red-100 text-red-700 hover:bg-red-200 focus:ring-red-500'
              : 'bg-yellow-100 text-yellow-700 hover:bg-yellow-200 focus:ring-yellow-500',
            isLoading ? 'cursor-not-allowed opacity-50' : '',
          )}
        >
          {isLoading ? 'Acknowledging...' : 'Acknowledge'}
        </button>
      ) : null}
    </div>
  );
}
