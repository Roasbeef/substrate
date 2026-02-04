// MessageRow component - a single row in the message list.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { PriorityBadge } from '@/components/ui/Badge.js';
import type { MessageWithRecipients, Message } from '@/types/api.js';
import { formatAgentDisplayName, getAgentContext } from '@/lib/utils.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Convert message sender to AgentLike format for display formatting.
function getSenderAsAgent(message: Message) {
  return {
    name: message.sender_name,
    project_key: message.sender_project_key,
    git_branch: message.sender_git_branch,
  };
}

// Star icon.
function StarIcon({
  filled = false,
  className,
}: {
  filled?: boolean;
  className?: string;
}) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill={filled ? 'currentColor' : 'none'}
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

// Archive icon.
function ArchiveIcon({ className }: { className?: string }) {
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
        d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"
      />
    </svg>
  );
}

// Clock icon for snooze.
function ClockIcon({ className }: { className?: string }) {
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
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

// Trash icon.
function TrashIcon({ className }: { className?: string }) {
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
        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
      />
    </svg>
  );
}

// Format relative time.
function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  });
}

// Truncate text with ellipsis.
function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength) + '...';
}

// Action button for message row.
interface ActionButtonProps {
  icon: React.ReactNode;
  label: string;
  onClick: (e: React.MouseEvent) => void;
  className?: string;
}

function ActionButton({ icon, label, onClick, className }: ActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600',
        'focus:outline-none focus:ring-2 focus:ring-blue-500',
        'opacity-0 group-hover:opacity-100 transition-opacity',
        className,
      )}
      title={label}
      aria-label={label}
    >
      {icon}
    </button>
  );
}

// MessageRow props.
export interface MessageRowProps {
  /** Message data. */
  message: MessageWithRecipients;
  /** Whether this message is selected. */
  isSelected?: boolean;
  /** Handler for selection change. */
  onSelect?: (selected: boolean) => void;
  /** Handler for clicking the message row. */
  onClick?: () => void;
  /** Handler for starring/unstarring. */
  onStar?: (starred: boolean) => void;
  /** Handler for archiving. */
  onArchive?: () => void;
  /** Handler for snoozing. */
  onSnooze?: () => void;
  /** Handler for deleting. */
  onDelete?: () => void;
  /** Whether to show checkbox. */
  showCheckbox?: boolean;
  /** Additional class name. */
  className?: string;
}

export function MessageRow({
  message,
  isSelected = false,
  onSelect,
  onClick,
  onStar,
  onArchive,
  onSnooze,
  onDelete,
  showCheckbox = true,
  className,
}: MessageRowProps) {
  // Get recipient state for current user (assuming first recipient for now).
  const recipientState = message.recipients[0];
  const isUnread = recipientState?.state === 'unread';
  const isStarred = recipientState?.is_starred ?? false;

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    e.stopPropagation();
    onSelect?.(e.target.checked);
  };

  const handleStarClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onStar?.(!isStarred);
  };

  const handleArchiveClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onArchive?.();
  };

  const handleSnoozeClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onSnooze?.();
  };

  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete?.();
  };

  return (
    <div
      data-testid="message-row"
      className={cn(
        'group flex items-center gap-3 border-b border-gray-100 px-4 py-2.5 transition-all duration-150',
        isUnread ? 'bg-white font-medium' : 'bg-white',
        isSelected ? 'bg-blue-50' : '',
        onClick ? 'cursor-pointer hover:shadow-sm hover:z-10 hover:relative' : '',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
    >
      {/* Unread indicator dot. */}
      <div className="w-2 flex-shrink-0">
        {isUnread ? (
          <span
            className="inline-block h-2 w-2 rounded-full bg-blue-500"
            aria-label="Unread message"
          />
        ) : null}
      </div>

      {/* Checkbox. */}
      {showCheckbox ? (
        <div className="flex-shrink-0">
          <input
            type="checkbox"
            checked={isSelected}
            onChange={handleCheckboxChange}
            onClick={(e) => e.stopPropagation()}
            className={cn(
              'h-[18px] w-[18px] rounded border-gray-300 text-blue-600',
              'focus:ring-2 focus:ring-blue-500 focus:ring-offset-0',
              'opacity-0 group-hover:opacity-100 transition-opacity',
              isSelected && 'opacity-100',
            )}
            aria-label={`Select message: ${message.subject}`}
          />
        </div>
      ) : null}

      {/* Star button. */}
      <button
        type="button"
        onClick={handleStarClick}
        className={cn(
          'flex-shrink-0 rounded p-0.5',
          isStarred ? 'text-yellow-500' : 'text-gray-300 hover:text-yellow-400',
          'focus:outline-none focus:ring-2 focus:ring-blue-500',
        )}
        aria-label={isStarred ? 'Unstar message' : 'Star message'}
      >
        <StarIcon filled={isStarred} className="h-5 w-5" />
      </button>

      {/* Sender name with project/branch context - fixed width for alignment. */}
      <div
        className="w-52 flex-shrink-0 truncate"
        title={formatAgentDisplayName(getSenderAsAgent(message))}
      >
        <span
          className={cn(
            'text-sm',
            isUnread ? 'font-semibold text-gray-900' : 'text-gray-700',
          )}
        >
          {message.sender_name}
        </span>
        {getAgentContext(getSenderAsAgent(message)) ? (
          <span className="ml-1 text-xs text-gray-400">
            @{getAgentContext(getSenderAsAgent(message))}
          </span>
        ) : null}
      </div>

      {/* Recipient names (To field). */}
      {message.recipients && message.recipients.length > 0 ? (
        <div className="w-32 flex-shrink-0 truncate text-sm text-gray-500">
          <span className="text-gray-400">→ </span>
          {message.recipients.map((r) => r.agent_name).join(', ')}
        </div>
      ) : null}

      {/* Subject and preview - flexible width. */}
      <div className="flex min-w-0 flex-1 items-baseline gap-1.5">
        {message.priority !== 'normal' ? (
          <PriorityBadge priority={message.priority} size="sm" />
        ) : null}
        <span
          data-testid="message-subject"
          className={cn(
            'truncate text-sm',
            isUnread ? 'font-semibold text-gray-900' : 'text-gray-800',
          )}
        >
          {message.subject}
        </span>
        <span className="hidden text-sm text-gray-400 sm:inline">-</span>
        <span className="hidden truncate text-sm text-gray-500 sm:inline">
          {truncate(message.body, 80)}
        </span>
      </div>

      {/* Actions (visible on hover). */}
      <div className="flex flex-shrink-0 items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
        {onArchive ? (
          <ActionButton
            icon={<ArchiveIcon className="h-4 w-4" />}
            label="Archive"
            onClick={handleArchiveClick}
            className="opacity-100"
          />
        ) : null}
        {onSnooze ? (
          <ActionButton
            icon={<ClockIcon className="h-4 w-4" />}
            label="Snooze"
            onClick={handleSnoozeClick}
            className="opacity-100"
          />
        ) : null}
        {onDelete ? (
          <ActionButton
            icon={<TrashIcon className="h-4 w-4" />}
            label="Delete"
            onClick={handleDeleteClick}
            className="opacity-100"
          />
        ) : null}
      </div>

      {/* Timestamp - right aligned. */}
      <span className={cn(
        'w-16 flex-shrink-0 text-right text-xs',
        isUnread ? 'font-semibold text-gray-900' : 'text-gray-500',
      )}>
        {formatRelativeTime(message.created_at)}
      </span>
    </div>
  );
}

// Compact variant for smaller displays.
export function CompactMessageRow({
  message,
  onClick,
  className,
}: {
  message: MessageWithRecipients;
  onClick?: () => void;
  className?: string;
}) {
  const recipientState = message.recipients[0];
  const isUnread = recipientState?.state === 'unread';
  const isStarred = recipientState?.is_starred ?? false;

  return (
    <div
      className={cn(
        'flex items-center gap-3 border-b border-gray-100 px-3 py-2 transition-colors',
        isUnread ? 'bg-blue-50/50' : 'bg-white',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      {isStarred ? (
        <StarIcon filled className="h-3 w-3 flex-shrink-0 text-yellow-500" />
      ) : null}
      <Avatar name={message.sender_name} size="xs" className="flex-shrink-0" />
      <div className="min-w-0 flex-1">
        <span
          className={cn(
            'block truncate text-sm',
            isUnread ? 'font-medium text-gray-900' : 'text-gray-700',
          )}
        >
          {message.subject}
        </span>
      </div>
      <span className="flex-shrink-0 text-xs text-gray-400">
        {formatRelativeTime(message.created_at)}
      </span>
    </div>
  );
}
