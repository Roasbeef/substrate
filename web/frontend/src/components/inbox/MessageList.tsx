// MessageList component - displays a list of messages with selection management.

import { useCallback, useMemo, useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { MessageRow, CompactMessageRow } from './MessageRow.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Empty state component.
function EmptyState({
  title,
  description,
  icon,
}: {
  title: string;
  description?: string;
  icon?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
      {icon ? (
        <div className="mb-4 text-gray-400">{icon}</div>
      ) : (
        <svg
          className="mb-4 h-12 w-12 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          />
        </svg>
      )}
      <h3 className="text-lg font-medium text-gray-900">{title}</h3>
      {description ? (
        <p className="mt-1 text-sm text-gray-500">{description}</p>
      ) : null}
    </div>
  );
}

// Loading skeleton for message rows.
function MessageSkeleton({ compact = false }: { compact?: boolean }) {
  if (compact) {
    return (
      <div className="flex items-center gap-3 border-b border-gray-100 px-3 py-2">
        <div className="h-6 w-6 animate-pulse rounded-full bg-gray-200" />
        <div className="flex-1">
          <div className="h-4 w-3/4 animate-pulse rounded bg-gray-200" />
        </div>
        <div className="h-3 w-12 animate-pulse rounded bg-gray-200" />
      </div>
    );
  }

  return (
    <div className="flex items-center gap-3 border-b border-gray-100 px-4 py-3">
      <div className="h-4 w-4 animate-pulse rounded bg-gray-200" />
      <div className="h-4 w-4 animate-pulse rounded bg-gray-200" />
      <div className="h-8 w-8 animate-pulse rounded-full bg-gray-200" />
      <div className="flex-1 space-y-2">
        <div className="flex items-center gap-2">
          <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
          <div className="h-4 w-12 animate-pulse rounded bg-gray-200" />
        </div>
        <div className="flex items-center gap-2">
          <div className="h-4 w-32 animate-pulse rounded bg-gray-200" />
          <div className="h-4 w-48 animate-pulse rounded bg-gray-200" />
        </div>
      </div>
      <div className="h-3 w-16 animate-pulse rounded bg-gray-200" />
    </div>
  );
}

// Props for MessageList component.
export interface MessageListProps {
  /** Array of messages to display. */
  messages: MessageWithRecipients[];
  /** IDs of currently selected messages. */
  selectedIds?: Set<number>;
  /** Handler for selection changes. */
  onSelectionChange?: (selectedIds: Set<number>) => void;
  /** Handler for clicking a message. */
  onMessageClick?: (message: MessageWithRecipients) => void;
  /** Handler for starring/unstarring a message. */
  onStar?: (id: number, starred: boolean) => void;
  /** Handler for archiving a message. */
  onArchive?: (id: number) => void;
  /** Handler for snoozing a message. */
  onSnooze?: (id: number) => void;
  /** Handler for deleting a message. */
  onDelete?: (id: number) => void;
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Number of skeleton rows to show when loading. */
  loadingRows?: number;
  /** Whether the list is empty. */
  isEmpty?: boolean;
  /** Empty state title. */
  emptyTitle?: string;
  /** Empty state description. */
  emptyDescription?: string;
  /** Whether to show checkboxes. */
  showCheckboxes?: boolean;
  /** Whether to use compact variant. */
  compact?: boolean;
  /** Additional class name. */
  className?: string;
}

export function MessageList({
  messages,
  selectedIds = new Set<number>(),
  onSelectionChange,
  onMessageClick,
  onStar,
  onArchive,
  onSnooze,
  onDelete,
  isLoading = false,
  loadingRows = 5,
  isEmpty = false,
  emptyTitle = 'No messages',
  emptyDescription = 'Your inbox is empty.',
  showCheckboxes = true,
  compact = false,
  className,
}: MessageListProps) {
  // Handle individual message selection.
  const handleMessageSelect = useCallback(
    (id: number, selected: boolean) => {
      if (!onSelectionChange) return;

      const newSelected = new Set(selectedIds);
      if (selected) {
        newSelected.add(id);
      } else {
        newSelected.delete(id);
      }
      onSelectionChange(newSelected);
    },
    [selectedIds, onSelectionChange],
  );

  // Show loading skeletons.
  if (isLoading) {
    return (
      <div className={cn('divide-y divide-gray-100', className)}>
        {Array.from({ length: loadingRows }, (_, i) => (
          <MessageSkeleton key={i} compact={compact} />
        ))}
      </div>
    );
  }

  // Show empty state.
  if (isEmpty || messages.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }

  // Render compact variant.
  if (compact) {
    return (
      <div className={cn('divide-y divide-gray-100', className)}>
        {messages.map((message) => (
          <CompactMessageRow
            key={message.id}
            message={message}
            onClick={onMessageClick ? () => onMessageClick(message) : undefined}
          />
        ))}
      </div>
    );
  }

  // Render full message list.
  return (
    <div className={cn('divide-y divide-gray-100', className)}>
      {messages.map((message) => (
        <MessageRow
          key={message.id}
          message={message}
          isSelected={selectedIds.has(message.id)}
          onSelect={
            onSelectionChange
              ? (selected) => handleMessageSelect(message.id, selected)
              : undefined
          }
          onClick={onMessageClick ? () => onMessageClick(message) : undefined}
          onStar={onStar ? (starred) => onStar(message.id, starred) : undefined}
          onArchive={onArchive ? () => onArchive(message.id) : undefined}
          onSnooze={onSnooze ? () => onSnooze(message.id) : undefined}
          onDelete={onDelete ? () => onDelete(message.id) : undefined}
          showCheckbox={showCheckboxes}
        />
      ))}
    </div>
  );
}

// Props for selection management hook.
interface UseMessageSelectionOptions {
  /** All available message IDs. */
  messageIds: number[];
}

// Hook for managing message selection state.
export function useMessageSelection({ messageIds }: UseMessageSelectionOptions) {
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());

  // Compute selection state.
  const selectionState = useMemo(() => {
    const selectedCount = selectedIds.size;
    const totalCount = messageIds.length;
    const allSelected = totalCount > 0 && selectedCount === totalCount;
    const isIndeterminate = selectedCount > 0 && selectedCount < totalCount;

    return {
      selectedIds,
      selectedCount,
      totalCount,
      allSelected,
      isIndeterminate,
      hasSelection: selectedCount > 0,
    };
  }, [selectedIds, messageIds.length]);

  // Select or deselect all messages.
  const selectAll = useCallback(
    (selected: boolean) => {
      if (selected) {
        setSelectedIds(new Set(messageIds));
      } else {
        setSelectedIds(new Set());
      }
    },
    [messageIds],
  );

  // Clear selection.
  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  // Toggle selection for a single message.
  const toggleSelection = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  // Set selection directly.
  const setSelection = useCallback((ids: Set<number>) => {
    setSelectedIds(ids);
  }, []);

  return {
    ...selectionState,
    selectAll,
    clearSelection,
    toggleSelection,
    setSelection,
  };
}

// Connected message list that integrates with useMessages hook.
export interface ConnectedMessageListProps
  extends Omit<
    MessageListProps,
    'messages' | 'isLoading' | 'isEmpty' | 'selectedIds' | 'onSelectionChange'
  > {
  /** Messages from useMessages hook. */
  data?: MessageWithRecipients[];
  /** Loading state from useMessages hook. */
  isLoading?: boolean;
  /** Error from useMessages hook. */
  error?: Error | null;
  /** Selection state from useMessageSelection hook. */
  selection?: ReturnType<typeof useMessageSelection>;
}

export function ConnectedMessageList({
  data,
  isLoading = false,
  error,
  selection,
  ...props
}: ConnectedMessageListProps) {
  // Handle error state.
  if (error) {
    return (
      <EmptyState
        title="Failed to load messages"
        description={error.message}
        icon={
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
        }
      />
    );
  }

  return (
    <MessageList
      messages={data ?? []}
      isLoading={isLoading}
      isEmpty={!isLoading && (!data || data.length === 0)}
      selectedIds={selection?.selectedIds}
      onSelectionChange={selection?.setSelection}
      {...props}
    />
  );
}
