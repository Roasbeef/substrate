// Hook for real-time message updates via WebSocket.

import { useCallback, useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useNewMessages,
  useUnreadCount,
  useWebSocketConnection,
  type NewMessagePayload,
} from '@/hooks/useWebSocket.js';
import { messageKeys } from '@/hooks/useMessages.js';
import type { MessageWithRecipients, APIResponse } from '@/types/api.js';

// Type alias for message list response.
type MessageListResponse = APIResponse<MessageWithRecipients[]>;

// Interface for real-time message state.
export interface RealtimeMessageState {
  unreadCount: number;
  isConnected: boolean;
  connect: () => void;
  disconnect: () => void;
}

// Hook to enable real-time updates for messages.
export function useMessagesRealtime(): RealtimeMessageState {
  const queryClient = useQueryClient();
  const unreadCount = useUnreadCount();
  const { state, connect, disconnect } = useWebSocketConnection();
  const isConnected = state === 'connected';

  // Track if we've added a message to prevent duplicate handling.
  const processedIds = useRef(new Set<number>());

  // Handle new message received via WebSocket.
  const handleNewMessage = useCallback(
    (payload: NewMessagePayload) => {
      // Skip if we've already processed this message.
      if (processedIds.current.has(payload.id)) {
        return;
      }
      processedIds.current.add(payload.id);

      // Clean up old IDs periodically (keep last 100).
      if (processedIds.current.size > 100) {
        const ids = Array.from(processedIds.current);
        processedIds.current = new Set(ids.slice(-50));
      }

      // Convert WebSocket payload to message format for cache update.
      // The message is created with minimal data; a refetch will get full details.
      const newMessage: MessageWithRecipients = {
        id: payload.id,
        sender_id: payload.sender_id,
        sender_name: payload.sender_name,
        subject: payload.subject,
        body: payload.body,
        priority: payload.priority as MessageWithRecipients['priority'],
        created_at: payload.created_at,
        recipients: [
          {
            message_id: payload.id,
            agent_id: 0, // Will be filled by actual recipient data.
            agent_name: '', // Will be filled by actual recipient data.
            state: 'unread',
            is_starred: false,
            is_archived: false,
          },
        ],
      };

      // Update all message list queries by prepending the new message.
      queryClient.setQueriesData<MessageListResponse>(
        { queryKey: messageKeys.lists() },
        (oldData) => {
          if (!oldData) return oldData;

          // Check if message already exists in the list.
          const exists = oldData.data.some((m) => m.id === payload.id);
          if (exists) return oldData;

          return {
            ...oldData,
            data: [newMessage, ...oldData.data],
            ...(oldData.meta && {
              meta: {
                ...oldData.meta,
                total: oldData.meta.total + 1,
              },
            }),
          };
        },
      );

      // Invalidate queries to ensure fresh data.
      void queryClient.invalidateQueries({
        queryKey: messageKeys.lists(),
        exact: false,
        refetchType: 'none', // Don't refetch immediately, just mark as stale.
      });
    },
    [queryClient],
  );

  // Subscribe to new message events.
  useNewMessages(handleNewMessage);

  // Clean up processed IDs on unmount.
  useEffect(() => {
    return () => {
      processedIds.current.clear();
    };
  }, []);

  return {
    unreadCount,
    isConnected,
    connect,
    disconnect,
  };
}

// Hook to get real-time unread count only (lighter weight).
export function useRealtimeUnreadCount(): number {
  // This just wraps useUnreadCount but ensures WebSocket is connected.
  useWebSocketConnection();
  return useUnreadCount();
}
