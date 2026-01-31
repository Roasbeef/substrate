// React hook for thread-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchThread,
  replyToThread,
  archiveThread,
  unarchiveThread,
  markThreadUnread,
  deleteThread,
} from '@/api/threads.js';
import { messageKeys } from './useMessages.js';
import type { ThreadWithMessages } from '@/types/api.js';

// Query keys for threads.
export const threadKeys = {
  all: ['threads'] as const,
  details: () => [...threadKeys.all, 'detail'] as const,
  detail: (id: number) => [...threadKeys.details(), id] as const,
};

// Hook for fetching a single thread with all its messages.
export function useThread(id: number, enabled = true) {
  return useQuery({
    queryKey: threadKeys.detail(id),
    queryFn: async ({ signal }) => {
      return fetchThread(id, signal);
    },
    enabled,
  });
}

// Hook for replying to a thread.
export function useReplyToThread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: string }) =>
      replyToThread(id, body),
    onSuccess: (_data, { id }) => {
      // Invalidate the thread to refetch with the new message.
      void queryClient.invalidateQueries({ queryKey: threadKeys.detail(id) });
      // Also invalidate message lists since a new message was created.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for archiving a thread.
export function useArchiveThread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => archiveThread(id),
    onMutate: async (id) => {
      // Cancel outgoing refetches.
      await queryClient.cancelQueries({ queryKey: threadKeys.detail(id) });

      // Snapshot previous value.
      const previousThread = queryClient.getQueryData<ThreadWithMessages>(
        threadKeys.detail(id),
      );

      return { previousThread };
    },
    onError: (_err, id, context) => {
      // Rollback on error.
      if (context?.previousThread) {
        queryClient.setQueryData(
          threadKeys.detail(id),
          context.previousThread,
        );
      }
    },
    onSettled: () => {
      // Invalidate lists to ensure they reflect the change.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for marking a thread as unread.
export function useMarkThreadUnread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => markThreadUnread(id),
    onSuccess: () => {
      // Invalidate message lists to refresh unread counts.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for unarchiving a thread.
export function useUnarchiveThread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => unarchiveThread(id),
    onSettled: () => {
      // Invalidate lists to ensure they reflect the change.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Hook for deleting a thread.
export function useDeleteThread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => deleteThread(id),
    onSuccess: (_data, id) => {
      // Remove the thread from the cache.
      queryClient.removeQueries({ queryKey: threadKeys.detail(id) });
      // Invalidate message lists to reflect the deletion.
      void queryClient.invalidateQueries({ queryKey: messageKeys.lists() });
    },
  });
}

// Convenience hook that returns all thread-related mutations.
export function useThreadMutations() {
  const replyMutation = useReplyToThread();
  const archiveMutation = useArchiveThread();
  const unarchiveMutation = useUnarchiveThread();
  const markUnreadMutation = useMarkThreadUnread();
  const deleteMutation = useDeleteThread();

  return {
    reply: replyMutation,
    archive: archiveMutation,
    unarchive: unarchiveMutation,
    markUnread: markUnreadMutation,
    delete: deleteMutation,
  };
}
