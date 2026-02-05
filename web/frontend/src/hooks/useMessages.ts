// React hook for message-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchMessages,
  fetchMessage,
  sendMessage,
  toggleMessageStar,
  archiveMessage,
  unarchiveMessage,
  snoozeMessage,
  markMessageRead,
  acknowledgeMessage,
  deleteMessage,
  type MessageListOptions,
} from '@/api/messages.js';
import type {
  MessageWithRecipients,
  SendMessageRequest,
} from '@/types/api.js';

// Query keys for messages.
export const messageKeys = {
  all: ['messages'] as const,
  lists: () => [...messageKeys.all, 'list'] as const,
  list: (options: MessageListOptions) =>
    [...messageKeys.lists(), options] as const,
  details: () => [...messageKeys.all, 'detail'] as const,
  detail: (id: number) => [...messageKeys.details(), id] as const,
};

// Hook for fetching a list of messages.
export function useMessages(options: MessageListOptions = {}) {
  return useQuery({
    queryKey: messageKeys.list(options),
    queryFn: async ({ signal }) => {
      const response = await fetchMessages(options, signal);
      return response;
    },
  });
}

// Hook for fetching a single message.
export function useMessage(id: number, enabled = true) {
  return useQuery({
    queryKey: messageKeys.detail(id),
    queryFn: async ({ signal }) => {
      return fetchMessage(id, signal);
    },
    enabled,
  });
}

// Hook for sending a new message.
export function useSendMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SendMessageRequest) => sendMessage(data),
    onSuccess: () => {
      // Invalidate message lists to refetch.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for starring/unstarring a message.
export function useToggleMessageStar() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, starred }: { id: number; starred: boolean }) =>
      toggleMessageStar(id, starred),
    onMutate: async ({ id, starred }) => {
      // Cancel outgoing refetches.
      await queryClient.cancelQueries({ queryKey: messageKeys.detail(id) });

      // Snapshot previous value.
      const previousMessage = queryClient.getQueryData<MessageWithRecipients>(
        messageKeys.detail(id),
      );

      // Optimistically update the cache.
      if (previousMessage) {
        queryClient.setQueryData<MessageWithRecipients>(
          messageKeys.detail(id),
          {
            ...previousMessage,
            recipients: previousMessage.recipients.map((r) => ({
              ...r,
              is_starred: starred,
            })),
          },
        );
      }

      return { previousMessage };
    },
    onError: (_err, { id }, context) => {
      // Rollback on error.
      if (context?.previousMessage) {
        queryClient.setQueryData(
          messageKeys.detail(id),
          context.previousMessage,
        );
      }
    },
    onSettled: () => {
      // Invalidate lists to ensure they reflect the change.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for archiving a message.
export function useArchiveMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => archiveMessage(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: messageKeys.detail(id) });

      const previousMessage = queryClient.getQueryData<MessageWithRecipients>(
        messageKeys.detail(id),
      );

      if (previousMessage) {
        queryClient.setQueryData<MessageWithRecipients>(
          messageKeys.detail(id),
          {
            ...previousMessage,
            recipients: previousMessage.recipients.map((r) => ({
              ...r,
              is_archived: true,
            })),
          },
        );
      }

      return { previousMessage };
    },
    onError: (_err, id, context) => {
      if (context?.previousMessage) {
        queryClient.setQueryData(
          messageKeys.detail(id),
          context.previousMessage,
        );
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for unarchiving a message.
export function useUnarchiveMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => unarchiveMessage(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for snoozing a message.
export function useSnoozeMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, until }: { id: number; until: string }) =>
      snoozeMessage(id, until),
    onMutate: async ({ id, until }) => {
      await queryClient.cancelQueries({ queryKey: messageKeys.detail(id) });

      const previousMessage = queryClient.getQueryData<MessageWithRecipients>(
        messageKeys.detail(id),
      );

      if (previousMessage) {
        queryClient.setQueryData<MessageWithRecipients>(
          messageKeys.detail(id),
          {
            ...previousMessage,
            recipients: previousMessage.recipients.map((r) => ({
              ...r,
              snoozed_until: until,
            })),
          },
        );
      }

      return { previousMessage };
    },
    onError: (_err, { id }, context) => {
      if (context?.previousMessage) {
        queryClient.setQueryData(
          messageKeys.detail(id),
          context.previousMessage,
        );
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for marking a message as read.
export function useMarkMessageRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => markMessageRead(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: messageKeys.detail(id) });

      const previousMessage = queryClient.getQueryData<MessageWithRecipients>(
        messageKeys.detail(id),
      );

      if (previousMessage) {
        queryClient.setQueryData<MessageWithRecipients>(
          messageKeys.detail(id),
          {
            ...previousMessage,
            recipients: previousMessage.recipients.map((r) => ({
              ...r,
              state: 'read',
              read_at: new Date().toISOString(),
            })),
          },
        );
      }

      return { previousMessage };
    },
    onError: (_err, id, context) => {
      if (context?.previousMessage) {
        queryClient.setQueryData(
          messageKeys.detail(id),
          context.previousMessage,
        );
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for acknowledging a message.
export function useAcknowledgeMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => acknowledgeMessage(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: messageKeys.detail(id) });

      const previousMessage = queryClient.getQueryData<MessageWithRecipients>(
        messageKeys.detail(id),
      );

      if (previousMessage) {
        queryClient.setQueryData<MessageWithRecipients>(
          messageKeys.detail(id),
          {
            ...previousMessage,
            recipients: previousMessage.recipients.map((r) => ({
              ...r,
              state: 'acknowledged',
              acknowledged_at: new Date().toISOString(),
            })),
          },
        );
      }

      return { previousMessage };
    },
    onError: (_err, id, context) => {
      if (context?.previousMessage) {
        queryClient.setQueryData(
          messageKeys.detail(id),
          context.previousMessage,
        );
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Delete message request parameters.
export interface DeleteMessageParams {
  id: number;
  // markSenderDeleted should be true when deleting from aggregate views like
  // CodeReviewer where messages are filtered by sender name.
  markSenderDeleted?: boolean;
}

// Hook for deleting a message.
export function useDeleteMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: DeleteMessageParams) =>
      deleteMessage(params.id, params.markSenderDeleted ?? false),
    onMutate: async (params) => {
      // Cancel outgoing refetches to prevent overwriting optimistic update.
      await queryClient.cancelQueries({ queryKey: messageKeys.lists() });

      // Snapshot the previous lists for rollback.
      const previousLists = queryClient.getQueriesData<{
        data: MessageWithRecipients[];
      }>({ queryKey: messageKeys.lists() });

      // Optimistically remove the message from all lists.
      queryClient.setQueriesData<{ data: MessageWithRecipients[] }>(
        { queryKey: messageKeys.lists() },
        (old) => {
          if (!old?.data) return old;
          return {
            ...old,
            data: old.data.filter((m) => m.id !== params.id),
          };
        },
      );

      // Also remove the detail cache.
      queryClient.removeQueries({ queryKey: messageKeys.detail(params.id) });

      return { previousLists };
    },
    onError: (_err, _params, context) => {
      // Rollback all lists on error.
      if (context?.previousLists) {
        for (const [queryKey, data] of context.previousLists) {
          queryClient.setQueryData(queryKey, data);
        }
      }
    },
    onSettled: () => {
      // Always refetch to ensure consistency.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Convenience hook that returns all message-related mutations.
export function useMessageMutations() {
  const sendMessageMutation = useSendMessage();
  const toggleStarMutation = useToggleMessageStar();
  const archiveMutation = useArchiveMessage();
  const unarchiveMutation = useUnarchiveMessage();
  const snoozeMutation = useSnoozeMessage();
  const markReadMutation = useMarkMessageRead();
  const acknowledgeMutation = useAcknowledgeMessage();
  const deleteMutation = useDeleteMessage();

  return {
    sendMessage: sendMessageMutation,
    toggleStar: toggleStarMutation,
    archive: archiveMutation,
    unarchive: unarchiveMutation,
    snooze: snoozeMutation,
    markRead: markReadMutation,
    acknowledge: acknowledgeMutation,
    delete: deleteMutation,
  };
}
