// ThreadView component - modal view for a full thread conversation.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Modal } from '@/components/ui/Modal.js';
import { Button } from '@/components/ui/Button.js';
import { Textarea } from '@/components/ui/Input.js';
import { Spinner } from '@/components/ui/Spinner.js';
import { ThreadMessage, DeadlineBanner } from './ThreadMessage.js';
import type { ThreadWithMessages } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Icon components.
function ArrowLeftIcon({ className }: { className?: string }) {
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
        d="M10 19l-7-7m0 0l7-7m-7 7h18"
      />
    </svg>
  );
}

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

function EnvelopeIcon({ className }: { className?: string }) {
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
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

function ChevronUpIcon({ className }: { className?: string }) {
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
        d="M5 15l7-7 7 7"
      />
    </svg>
  );
}

function ChevronDownIcon({ className }: { className?: string }) {
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
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

function ExpandIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
      />
    </svg>
  );
}

function CollapseIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 9V4.5M9 9H4.5M9 9L3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5l5.25 5.25"
      />
    </svg>
  );
}

// Toolbar action button.
interface ToolbarButtonProps {
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  disabled?: boolean;
  className?: string;
}

function ToolbarButton({
  onClick,
  icon,
  label,
  disabled = false,
  className,
}: ToolbarButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'flex items-center justify-center rounded-md p-2',
        'text-gray-500 hover:bg-gray-100 hover:text-gray-700',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        disabled ? 'cursor-not-allowed opacity-50' : '',
        className,
      )}
      title={label}
      aria-label={label}
    >
      {icon}
    </button>
  );
}

// Props for ThreadView component.
export interface ThreadViewProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** The thread data to display. */
  thread?: ThreadWithMessages;
  /** Whether the thread is loading. */
  isLoading?: boolean;
  /** Error loading thread. */
  error?: Error | null;
  /** Handler for archiving the thread. */
  onArchive?: () => void;
  /** Handler for deleting the thread. */
  onDelete?: () => void;
  /** Handler for marking as unread. */
  onMarkUnread?: () => void;
  /** Handler for replying. */
  onReply?: (body: string) => void;
  /** Handler for acknowledging a deadline. */
  onAcknowledge?: () => void;
  /** Handler for navigating to previous thread. */
  onPrevious?: () => void;
  /** Handler for navigating to next thread. */
  onNext?: () => void;
  /** Whether there is a previous thread. */
  hasPrevious?: boolean;
  /** Whether there is a next thread. */
  hasNext?: boolean;
  /** Whether actions are loading. */
  isActionLoading?: boolean;
  /** Deadline for the thread (if any). */
  deadline?: string;
}

export function ThreadView({
  isOpen,
  onClose,
  thread,
  isLoading = false,
  error,
  onArchive,
  onDelete,
  onMarkUnread,
  onReply,
  onAcknowledge,
  onPrevious,
  onNext,
  hasPrevious = false,
  hasNext = false,
  isActionLoading = false,
  deadline,
}: ThreadViewProps) {
  // State for reply input.
  const [replyText, setReplyText] = useState('');
  const [isReplying, setIsReplying] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  // State for focused message index.
  const [focusedIndex, setFocusedIndex] = useState<number | null>(null);

  // Ref for messages container to scroll.
  const messagesRef = useRef<HTMLDivElement>(null);
  const messageRefs = useRef<Map<number, HTMLDivElement>>(new Map());

  // Messages in chronological order.
  const messages = useMemo(
    () => thread?.messages ?? [],
    [thread?.messages],
  );

  // Scroll to bottom when thread loads.
  useEffect(() => {
    if (messages.length > 0 && messagesRef.current) {
      messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
      setFocusedIndex(messages.length - 1);
    }
  }, [messages.length]);

  // Scroll to focused message.
  useEffect(() => {
    if (focusedIndex !== null && focusedIndex < messages.length) {
      const messageEl = messageRefs.current.get(focusedIndex);
      messageEl?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [focusedIndex, messages.length]);

  // Handle keyboard navigation.
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        setFocusedIndex((prev) => {
          const next = (prev ?? -1) + 1;
          return next < messages.length ? next : prev;
        });
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        setFocusedIndex((prev) => {
          const next = (prev ?? messages.length) - 1;
          return next >= 0 ? next : prev;
        });
      } else if (e.key === 'Escape') {
        onClose();
      }
    },
    [messages.length, onClose],
  );

  // Handle reply submission.
  const handleReply = useCallback(async () => {
    if (!replyText.trim() || !onReply) return;

    setIsReplying(true);
    try {
      onReply(replyText.trim());
      setReplyText('');
    } finally {
      setIsReplying(false);
    }
  }, [replyText, onReply]);

  // Handle reply on Enter (with Ctrl/Cmd).
  const handleReplyKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        void handleReply();
      }
    },
    [handleReply],
  );

  // Check if deadline is past.
  const isDeadlinePast = useMemo(() => {
    if (!deadline) return false;
    return new Date(deadline) < new Date();
  }, [deadline]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="3xl"
      className="flex h-[80vh] flex-col overflow-hidden"
      rawContent
      showCloseButton={false}
    >
      {/* Toolbar. */}
      <div className="flex flex-shrink-0 items-center justify-between border-b border-gray-200 bg-gray-50 px-4 py-2">
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={onClose}
            className={cn(
              'flex items-center gap-2 rounded-lg px-3 py-1.5',
              'text-sm font-medium text-gray-700',
              'hover:bg-gray-200 transition-colors',
              'focus:outline-none focus:ring-2 focus:ring-blue-500',
            )}
          >
            <ArrowLeftIcon className="h-4 w-4" />
            <span>Back to inbox</span>
          </button>

          <div className="mx-3 h-5 w-px bg-gray-300" />

          {onArchive ? (
            <ToolbarButton
              onClick={onArchive}
              icon={<ArchiveIcon />}
              label="Archive"
              disabled={isActionLoading}
            />
          ) : null}

          {onDelete ? (
            <ToolbarButton
              onClick={onDelete}
              icon={<TrashIcon />}
              label="Delete"
              disabled={isActionLoading}
            />
          ) : null}

          {onMarkUnread ? (
            <ToolbarButton
              onClick={onMarkUnread}
              icon={<EnvelopeIcon />}
              label="Mark as unread"
              disabled={isActionLoading}
            />
          ) : null}
        </div>

        <div className="flex items-center gap-1">
          <ToolbarButton
            onClick={onPrevious ?? (() => {})}
            icon={<ChevronUpIcon />}
            label="Previous thread"
            disabled={!hasPrevious || isActionLoading}
          />
          <ToolbarButton
            onClick={onNext ?? (() => {})}
            icon={<ChevronDownIcon />}
            label="Next thread"
            disabled={!hasNext || isActionLoading}
          />
          <div className="ml-2 h-5 w-px bg-gray-300" />
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-200 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Close"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Content. */}
      <div
        className="min-h-0 flex-1 overflow-y-auto p-4"
        ref={messagesRef}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role="log"
        aria-label="Thread messages"
      >
        {isLoading ? (
          <div className="flex h-full items-center justify-center">
            <Spinner size="lg" />
          </div>
        ) : error ? (
          <div className="flex h-full flex-col items-center justify-center text-center">
            <svg
              className="h-12 w-12 text-red-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <p className="mt-2 text-gray-500">{error.message}</p>
          </div>
        ) : thread ? (
          <div className="space-y-4">
            {/* Subject. */}
            <h1 className="text-xl font-bold text-gray-900">{thread.subject}</h1>

            {/* Deadline banner. */}
            {deadline ? (
              <DeadlineBanner
                deadline={deadline}
                isPast={isDeadlinePast}
                {...(onAcknowledge && { onAcknowledge })}
                isLoading={isActionLoading}
              />
            ) : null}

            {/* Messages. */}
            {messages.map((message, index) => (
              <div
                key={message.id}
                ref={(el) => {
                  if (el) {
                    messageRefs.current.set(index, el);
                  } else {
                    messageRefs.current.delete(index);
                  }
                }}
              >
                <ThreadMessage
                  message={message}
                  isFirst={index === 0}
                  isFocused={focusedIndex === index}
                />
              </div>
            ))}
          </div>
        ) : null}
      </div>

      {/* Reply box. */}
      {thread && onReply ? (
        <div
          className={cn(
            'border-t border-gray-200 p-4',
            isExpanded && 'flex-1 flex flex-col min-h-[300px]',
          )}
        >
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm text-gray-500">
              Press Ctrl+Enter to send
            </span>
            <button
              type="button"
              onClick={() => setIsExpanded(!isExpanded)}
              className={cn(
                'flex items-center gap-1 rounded px-2 py-1 text-xs',
                'text-gray-500 hover:bg-gray-100 hover:text-gray-700',
                'transition-colors',
              )}
              title={isExpanded ? 'Collapse' : 'Expand'}
            >
              {isExpanded ? (
                <>
                  <CollapseIcon />
                  <span>Collapse</span>
                </>
              ) : (
                <>
                  <ExpandIcon />
                  <span>Expand</span>
                </>
              )}
            </button>
          </div>
          <Textarea
            value={replyText}
            onChange={(e) => setReplyText(e.target.value)}
            onKeyDown={handleReplyKeyDown}
            placeholder="Write a reply..."
            rows={isExpanded ? 12 : 6}
            className={cn(
              'w-full',
              isExpanded && 'flex-1 min-h-[200px]',
            )}
          />
          <div className="mt-3 flex justify-end">
            <Button
              onClick={() => void handleReply()}
              disabled={!replyText.trim() || isReplying || isActionLoading}
              isLoading={isReplying}
            >
              Reply
            </Button>
          </div>
        </div>
      ) : null}
    </Modal>
  );
}

// Hook for managing thread view state.
export interface UseThreadViewOptions {
  /** Initial thread ID. */
  initialThreadId?: number | null;
  /** Callback when thread changes. */
  onThreadChange?: (threadId: number | null) => void;
}

export function useThreadView(options: UseThreadViewOptions = {}) {
  const { initialThreadId = null, onThreadChange } = options;

  const [threadId, setThreadId] = useState<number | null>(initialThreadId);

  const open = useCallback(
    (id: number) => {
      setThreadId(id);
      onThreadChange?.(id);
    },
    [onThreadChange],
  );

  const close = useCallback(() => {
    setThreadId(null);
    onThreadChange?.(null);
  }, [onThreadChange]);

  return {
    threadId,
    isOpen: threadId !== null,
    open,
    close,
  };
}
