// InboxPage component - main inbox view with messages, filters, and actions.

import { useCallback, useMemo, useState } from 'react';
import {
  InboxStats,
  CategoryTabs,
  FilterBar,
  MessageList,
  useMessageSelection,
  type InboxCategory,
  type FilterType,
} from '@/components/inbox/index.js';
import {
  useMessages,
  useToggleMessageStar,
  useArchiveMessage,
  useSnoozeMessage,
  useMarkMessageRead,
} from '@/hooks/useMessages.js';
import { useMessagesRealtime } from '@/hooks/useMessagesRealtime.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Inbox page state.
interface InboxState {
  category: InboxCategory;
  filter: FilterType;
}

export default function InboxPage() {
  // Page state.
  const [state, setState] = useState<InboxState>({
    category: 'primary',
    filter: 'all',
  });

  // Build query options from state.
  const queryOptions = useMemo(() => {
    const options: Record<string, string | boolean | undefined> = {};

    // Apply filter.
    if (state.filter === 'unread') {
      options.state = 'unread';
    } else if (state.filter === 'starred') {
      options.starred = true;
    }

    // Apply category (would need backend support).
    if (state.category !== 'primary') {
      options.category = state.category;
    }

    return options;
  }, [state.category, state.filter]);

  // Fetch messages.
  const {
    data: messagesResponse,
    isLoading,
    error,
    refetch,
  } = useMessages(queryOptions);

  // Extract messages array from response.
  const messages = messagesResponse?.data ?? [];

  // Selection state.
  const messageIds = useMemo(
    () => messages.map((m) => m.id),
    [messages],
  );
  const selection = useMessageSelection({ messageIds });

  // Mutations.
  const toggleStar = useToggleMessageStar();
  const archiveMessage = useArchiveMessage();
  const snoozeMessage = useSnoozeMessage();
  const markRead = useMarkMessageRead();

  // Enable real-time updates via WebSocket.
  const { isConnected: wsConnected } = useMessagesRealtime();

  // Compute stats from messages.
  const stats = useMemo(() => {
    const allMessages = messagesResponse?.data;
    if (!allMessages) {
      return { unread: 0, starred: 0, urgent: 0, acknowledged: 0 };
    }

    return {
      unread: allMessages.filter(
        (m) => m.recipients[0]?.state === 'unread',
      ).length,
      starred: allMessages.filter(
        (m) => m.recipients[0]?.is_starred,
      ).length,
      urgent: allMessages.filter(
        (m) => m.priority === 'urgent',
      ).length,
      acknowledged: allMessages.filter(
        (m) => m.recipients[0]?.state === 'acknowledged',
      ).length,
    };
  }, [messagesResponse]);

  // Handle category change.
  const handleCategoryChange = useCallback((category: InboxCategory) => {
    setState((prev) => ({ ...prev, category }));
  }, []);

  // Handle filter change.
  const handleFilterChange = useCallback((filter: FilterType) => {
    setState((prev) => ({ ...prev, filter }));
  }, []);

  // Handle stat card click to set filter.
  const handleStatClick = useCallback(
    (stat: 'unread' | 'starred' | 'urgent' | 'acknowledged') => {
      if (stat === 'unread') {
        setState((prev) => ({ ...prev, filter: 'unread' }));
      } else if (stat === 'starred') {
        setState((prev) => ({ ...prev, filter: 'starred' }));
      }
      // urgent and acknowledged would need additional filter options.
    },
    [],
  );

  // Handle refresh.
  const handleRefresh = useCallback(() => {
    void refetch();
  }, [refetch]);

  // Handle message click.
  const handleMessageClick = useCallback((message: MessageWithRecipients) => {
    // Mark as read when opened.
    if (message.recipients[0]?.state === 'unread') {
      markRead.mutate(message.id);
    }
    // TODO: Open thread view modal.
    console.log('Open message:', message.id);
  }, [markRead]);

  // Handle star toggle.
  const handleStar = useCallback(
    (id: number, starred: boolean) => {
      toggleStar.mutate({ id, starred });
    },
    [toggleStar],
  );

  // Handle archive.
  const handleArchive = useCallback(
    (id: number) => {
      archiveMessage.mutate(id);
    },
    [archiveMessage],
  );

  // Handle snooze.
  const handleSnooze = useCallback(
    (id: number) => {
      // Snooze for 1 hour by default - would open a modal in real app.
      const until = new Date();
      until.setHours(until.getHours() + 1);
      snoozeMessage.mutate({ id, until: until.toISOString() });
    },
    [snoozeMessage],
  );

  // Handle delete (archive for now).
  const handleDelete = useCallback(
    (id: number) => {
      // Would show confirmation dialog in real app.
      archiveMessage.mutate(id);
    },
    [archiveMessage],
  );

  // Bulk actions.
  const handleBulkArchive = useCallback(() => {
    selection.selectedIds.forEach((id) => {
      archiveMessage.mutate(id);
    });
    selection.clearSelection();
  }, [selection, archiveMessage]);

  const handleBulkStar = useCallback(() => {
    selection.selectedIds.forEach((id) => {
      toggleStar.mutate({ id, starred: true });
    });
    selection.clearSelection();
  }, [selection, toggleStar]);

  const handleBulkDelete = useCallback(() => {
    selection.selectedIds.forEach((id) => {
      archiveMessage.mutate(id);
    });
    selection.clearSelection();
  }, [selection, archiveMessage]);

  // Mutation loading state.
  const isActionLoading =
    toggleStar.isPending ||
    archiveMessage.isPending ||
    snoozeMessage.isPending ||
    markRead.isPending;

  return (
    <div className="flex h-full flex-col">
      {/* Stats cards with real-time indicator. */}
      <div className="border-b border-gray-200 bg-gray-50 px-6 py-4">
        <div className="flex items-center justify-between">
          <InboxStats
            unread={stats.unread}
            starred={stats.starred}
            urgent={stats.urgent}
            acknowledged={stats.acknowledged}
            onStatClick={handleStatClick}
            isLoading={isLoading}
          />
          <div
            className="flex items-center gap-1.5 text-xs"
            title={wsConnected ? 'Real-time updates active' : 'Connecting...'}
          >
            <span
              className={`inline-block h-2 w-2 rounded-full ${
                wsConnected ? 'bg-green-400' : 'bg-yellow-400 animate-pulse'
              }`}
            />
            <span className="text-gray-500">
              {wsConnected ? 'Live' : 'Connecting'}
            </span>
          </div>
        </div>
      </div>

      {/* Category tabs. */}
      <div className="border-b border-gray-200 bg-white px-6">
        <CategoryTabs
          selected={state.category}
          onSelect={handleCategoryChange}
          disabled={isLoading}
        />
      </div>

      {/* Filter bar. */}
      <FilterBar
        filter={state.filter}
        onFilterChange={handleFilterChange}
        selectedCount={selection.selectedCount}
        totalCount={messages.length}
        allSelected={selection.allSelected}
        isIndeterminate={selection.isIndeterminate}
        onSelectAll={selection.selectAll}
        onRefresh={handleRefresh}
        {...(selection.hasSelection && { onArchive: handleBulkArchive })}
        {...(selection.hasSelection && { onStar: handleBulkStar })}
        {...(selection.hasSelection && { onDelete: handleBulkDelete })}
        isLoading={isLoading || isActionLoading}
      />

      {/* Message list. */}
      <div className="flex-1 overflow-auto">
        <MessageList
          messages={messages}
          selectedIds={selection.selectedIds}
          onSelectionChange={selection.setSelection}
          onMessageClick={handleMessageClick}
          onStar={handleStar}
          onArchive={handleArchive}
          onSnooze={handleSnooze}
          onDelete={handleDelete}
          isLoading={isLoading}
          isEmpty={!isLoading && messages.length === 0}
          emptyTitle={
            state.filter === 'unread'
              ? 'All caught up!'
              : state.filter === 'starred'
                ? 'No starred messages'
                : 'No messages'
          }
          emptyDescription={
            state.filter === 'unread'
              ? 'You have no unread messages.'
              : state.filter === 'starred'
                ? 'Star messages to find them here.'
                : 'Your inbox is empty.'
          }
        />
      </div>

      {/* Error toast (would use Toast component in real app). */}
      {error ? (
        <div className="fixed bottom-4 right-4 rounded-lg bg-red-50 p-4 shadow-lg">
          <div className="flex items-center gap-2">
            <svg
              className="h-5 w-5 text-red-400"
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
            <span className="text-sm font-medium text-red-800">
              Failed to load messages: {error.message}
            </span>
          </div>
        </div>
      ) : null}
    </div>
  );
}
