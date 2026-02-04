// InboxPage component - main inbox view with messages, filters, and actions.

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router-dom';
import {
  InboxStats,
  CategoryTabs,
  FilterBar,
  MessageList,
  useMessageSelection,
  ThreadView,
  type InboxCategory,
  type FilterType,
  type TabItem,
} from '@/components/inbox/index.js';
import {
  useMessages,
  useToggleMessageStar,
  useArchiveMessage,
  useSnoozeMessage,
  useMarkMessageRead,
  useDeleteMessage,
} from '@/hooks/useMessages.js';
import { useAuthStore } from '@/stores/auth.js';
import { useUIStore } from '@/stores/ui.js';
import {
  useThread,
  useReplyToThread,
  useArchiveThread,
  useDeleteThread,
  useMarkThreadUnread,
} from '@/hooks/useThreads.js';
import { useMessagesRealtime } from '@/hooks/useMessagesRealtime.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Inbox page state.
interface InboxState {
  category: InboxCategory;
  filter: FilterType;
  senderFilter: string | null;
}

// Route filter type for sidebar navigation.
type RouteFilter = 'inbox' | 'starred' | 'snoozed' | 'sent' | 'archive';

// Get route filter from pathname.
function getRouteFilter(pathname: string): RouteFilter {
  if (pathname.startsWith('/starred')) return 'starred';
  if (pathname.startsWith('/snoozed')) return 'snoozed';
  if (pathname.startsWith('/sent')) return 'sent';
  if (pathname.startsWith('/archive')) return 'archive';
  return 'inbox';
}

export default function InboxPage() {
  // Get current location for route-based filtering.
  const location = useLocation();
  const params = useParams<{ threadId?: string }>();
  const routeFilter = getRouteFilter(location.pathname);

  // Get current agent and aggregate selection from auth store for filtering.
  const currentAgent = useAuthStore((state) => state.currentAgent);
  const selectedAgentIds = useAuthStore((state) => state.selectedAgentIds);
  const selectedAggregate = useAuthStore((state) => state.selectedAggregate);

  // Page state.
  const [state, setState] = useState<InboxState>({
    category: 'primary',
    filter: 'all',
    senderFilter: null,
  });

  // Reset filter when route changes.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset filter on route change
    setState((prev) => ({ ...prev, filter: 'all' }));
  }, [routeFilter]);

  // Thread view state.
  const [selectedThreadId, setSelectedThreadId] = useState<string | null>(null);

  // Check for pending thread from notification click.
  const pendingThreadId = useUIStore((state) => state.pendingThreadId);
  const clearPendingThread = useUIStore((state) => state.clearPendingThread);

  // Open pending thread if one is set (from notification click).
  useEffect(() => {
    if (pendingThreadId) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- sync with store
      setSelectedThreadId(pendingThreadId);
      clearPendingThread();
    }
  }, [pendingThreadId, clearPendingThread]);

  // Open thread from URL params (from search navigation).
  useEffect(() => {
    if (params.threadId) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- sync with URL params
      setSelectedThreadId(params.threadId);
    }
  }, [params.threadId]);

  // Extract agent ID for stable dependency.
  const currentAgentId = currentAgent?.id;

  // Determine if we're filtering by aggregate (multiple agents).
  const isAggregateFilter = selectedAggregate !== null && selectedAgentIds.length > 1;

  // Build query options from route and state.
  const queryOptions = useMemo(() => {
    const options: Record<string, string | boolean | number | undefined> = {};

    // Apply route-based category filter.
    if (routeFilter !== 'inbox') {
      options.category = routeFilter;
    }

    // Apply additional state filter (only if on inbox page).
    if (routeFilter === 'inbox') {
      if (state.filter === 'unread') {
        options.filter = 'unread';
      } else if (state.filter === 'starred') {
        options.filter = 'starred';
      }
    }

    // Apply category (would need backend support).
    if (state.category !== 'primary') {
      options.category = state.category;
    }

    // Apply agent filter if a single agent is selected (not aggregate).
    if (currentAgentId !== undefined && !isAggregateFilter) {
      options.agentId = currentAgentId;
    }

    // For aggregates, filter by sender name prefix on the backend.
    if (isAggregateFilter && selectedAggregate === 'CodeReviewer') {
      options.senderNamePrefix = 'reviewer-';
    }

    return options;
  }, [state.category, state.filter, routeFilter, currentAgentId, isAggregateFilter, selectedAggregate]);

  // Fetch messages.
  const {
    data: messagesResponse,
    isLoading,
    error,
    refetch,
  } = useMessages(queryOptions);

  // Extract messages array from response with stable reference.
  // For aggregate selections, filter by sender_id matching any of the selected agent IDs.
  const allMessages = useMemo(() => {
    const messages = messagesResponse?.data ?? [];

    // If aggregate is selected, filter to messages FROM those agents.
    if (isAggregateFilter && selectedAgentIds.length > 0) {
      return messages.filter((m) => selectedAgentIds.includes(m.sender_id));
    }

    return messages;
  }, [messagesResponse?.data, isAggregateFilter, selectedAgentIds]);

  // Get unique senders for filter dropdown.
  const uniqueSenders = useMemo(() => {
    const senders = new Set<string>();
    allMessages.forEach((m) => senders.add(m.sender_name));
    return Array.from(senders).sort();
  }, [allMessages]);

  // Apply sender filter.
  const messages = useMemo(() => {
    if (!state.senderFilter) return allMessages;
    return allMessages.filter((m) => m.sender_name === state.senderFilter);
  }, [allMessages, state.senderFilter]);

  // Selection state.
  const messageIds = useMemo(
    () => messages.map((m) => m.id),
    [messages],
  );
  const selection = useMessageSelection({ messageIds });

  // Message mutations.
  const toggleStar = useToggleMessageStar();
  const archiveMessage = useArchiveMessage();
  const snoozeMessage = useSnoozeMessage();
  const markRead = useMarkMessageRead();
  const deleteMessage = useDeleteMessage();

  // Thread query and mutations.
  const {
    data: threadData,
    isLoading: threadLoading,
    error: threadError,
  } = useThread(selectedThreadId ?? '', selectedThreadId !== null);
  const replyToThread = useReplyToThread();
  const archiveThread = useArchiveThread();
  const deleteThread = useDeleteThread();
  const markThreadUnread = useMarkThreadUnread();

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

  // Compute category counts for tabs.
  const categoryTabs = useMemo((): TabItem[] => {
    const allMessages = messagesResponse?.data;
    if (!allMessages) {
      return [
        { id: 'primary', label: 'Primary', count: 0 },
        { id: 'agents', label: 'Agents', count: 0 },
        { id: 'topics', label: 'Topics', count: 0 },
      ];
    }

    // Primary = messages from User (human sender).
    // Agents = messages from other agents.
    // Topics = would need topic_id field (not yet in API, count 0 for now).
    const primaryCount = allMessages.filter(
      (m) => m.sender_name === 'User',
    ).length;
    const agentsCount = allMessages.filter(
      (m) => m.sender_name !== 'User',
    ).length;

    return [
      { id: 'primary', label: 'Primary', count: primaryCount },
      { id: 'agents', label: 'Agents', count: agentsCount },
      { id: 'topics', label: 'Topics', count: 0 },
    ];
  }, [messagesResponse]);

  // Handle category change.
  const handleCategoryChange = useCallback((category: InboxCategory) => {
    setState((prev) => ({ ...prev, category }));
  }, []);

  // Handle filter change.
  const handleFilterChange = useCallback((filter: FilterType) => {
    setState((prev) => ({ ...prev, filter }));
  }, []);

  // Handle sender filter change.
  const handleSenderFilterChange = useCallback((sender: string | null) => {
    setState((prev) => ({ ...prev, senderFilter: sender }));
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

  // Handle message click - open thread view.
  const handleMessageClick = useCallback((message: MessageWithRecipients) => {
    // Mark as read when opened.
    if (message.recipients[0]?.state === 'unread') {
      markRead.mutate(message.id);
    }
    // Open thread view with the message's thread_id (or message id as string fallback).
    const threadId = message.thread_id ?? String(message.id);
    setSelectedThreadId(threadId);
  }, [markRead]);

  // Handle closing thread view.
  const handleCloseThread = useCallback(() => {
    setSelectedThreadId(null);
  }, []);

  // Handle thread reply.
  const handleThreadReply = useCallback((body: string) => {
    if (selectedThreadId !== null) {
      replyToThread.mutate({ id: selectedThreadId, body });
    }
  }, [selectedThreadId, replyToThread]);

  // Handle thread archive.
  const handleThreadArchive = useCallback(() => {
    if (selectedThreadId !== null) {
      archiveThread.mutate(selectedThreadId);
      setSelectedThreadId(null);
    }
  }, [selectedThreadId, archiveThread]);

  // Handle thread delete.
  const handleThreadDelete = useCallback(() => {
    if (selectedThreadId !== null) {
      deleteThread.mutate(selectedThreadId);
      setSelectedThreadId(null);
    }
  }, [selectedThreadId, deleteThread]);

  // Handle mark thread as unread.
  const handleThreadMarkUnread = useCallback(() => {
    if (selectedThreadId !== null) {
      markThreadUnread.mutate(selectedThreadId);
      setSelectedThreadId(null);
    }
  }, [selectedThreadId, markThreadUnread]);

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

  // Handle delete.
  // For CodeReviewer aggregate view, mark as deleted by sender since we're
  // filtering by sender_name_prefix.
  const handleDelete = useCallback(
    (id: number) => {
      // Would show confirmation dialog in real app.
      const markSenderDeleted = selectedAggregate === 'CodeReviewer';
      deleteMessage.mutate({ id, markSenderDeleted });
    },
    [deleteMessage, selectedAggregate],
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
    const markSenderDeleted = selectedAggregate === 'CodeReviewer';
    selection.selectedIds.forEach((id) => {
      deleteMessage.mutate({ id, markSenderDeleted });
    });
    selection.clearSelection();
  }, [selection, deleteMessage, selectedAggregate]);

  // Mutation loading state (includes thread mutations).
  const isActionLoading =
    toggleStar.isPending ||
    archiveMessage.isPending ||
    snoozeMessage.isPending ||
    markRead.isPending ||
    deleteMessage.isPending ||
    replyToThread.isPending ||
    archiveThread.isPending ||
    deleteThread.isPending ||
    markThreadUnread.isPending;

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
          tabs={categoryTabs}
          disabled={isLoading}
        />
      </div>

      {/* Sender filter (when multiple senders exist). */}
      {uniqueSenders.length > 1 && (
        <div className="flex items-center gap-2 border-b border-gray-200 bg-gray-50 px-6 py-2">
          <label htmlFor="sender-filter" className="text-sm font-medium text-gray-700">
            Filter by sender:
          </label>
          <select
            id="sender-filter"
            value={state.senderFilter ?? ''}
            onChange={(e) => handleSenderFilterChange(e.target.value || null)}
            className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          >
            <option value="">All senders</option>
            {uniqueSenders.map((sender) => (
              <option key={sender} value={sender}>
                {sender}
              </option>
            ))}
          </select>
          {state.senderFilter && (
            <button
              type="button"
              onClick={() => handleSenderFilterChange(null)}
              className="text-sm text-blue-600 hover:text-blue-700"
            >
              Clear filter
            </button>
          )}
        </div>
      )}

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

      {/* Thread view modal. */}
      <ThreadView
        isOpen={selectedThreadId !== null}
        onClose={handleCloseThread}
        {...(threadData !== undefined && { thread: threadData })}
        isLoading={threadLoading}
        {...(threadError !== null && { error: threadError })}
        onReply={handleThreadReply}
        onArchive={handleThreadArchive}
        onDelete={handleThreadDelete}
        onMarkUnread={handleThreadMarkUnread}
        isActionLoading={
          replyToThread.isPending ||
          archiveThread.isPending ||
          deleteThread.isPending ||
          markThreadUnread.isPending
        }
      />

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
