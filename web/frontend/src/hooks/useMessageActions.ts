// Hook for message actions with toast notifications and confirmation dialogs.

import { useCallback, useState } from 'react';
import {
  useToggleMessageStar,
  useArchiveMessage,
  useUnarchiveMessage,
  useSnoozeMessage,
  useMarkMessageRead,
  useAcknowledgeMessage,
} from './useMessages.js';
import {
  useReplyToThread,
  useArchiveThread,
  useUnarchiveThread,
  useMarkThreadUnread,
  useDeleteThread,
} from './useThreads.js';
import { useUIStore } from '@/stores/ui.js';

// Type for pending undo operations.
interface UndoOperation {
  id: number;
  type: 'archive' | 'snooze';
  messageId: number;
  timeoutId: ReturnType<typeof setTimeout>;
}

// Hook for message actions with integrated toast notifications.
export function useMessageActions() {
  const addToast = useUIStore((state) => state.addToast);
  const [pendingUndos, setPendingUndos] = useState<Map<number, UndoOperation>>(
    new Map(),
  );

  // Mutations.
  const toggleStarMutation = useToggleMessageStar();
  const archiveMutation = useArchiveMessage();
  const unarchiveMutation = useUnarchiveMessage();
  const snoozeMutation = useSnoozeMessage();
  const markReadMutation = useMarkMessageRead();
  const acknowledgeMutation = useAcknowledgeMessage();

  // Handle star toggle with toast.
  const handleStar = useCallback(
    async (messageId: number, starred: boolean) => {
      try {
        await toggleStarMutation.mutateAsync({ id: messageId, starred });
        // Don't show toast for star/unstar - it's too frequent.
      } catch {
        addToast({
          variant: 'error',
          message: starred ? 'Failed to star message' : 'Failed to unstar message',
        });
      }
    },
    [toggleStarMutation, addToast],
  );

  // Handle archive with undo toast.
  const handleArchive = useCallback(
    async (messageId: number) => {
      const undoId = Date.now();

      try {
        await archiveMutation.mutateAsync(messageId);

        // Setup undo timeout.
        const timeoutId = setTimeout(() => {
          setPendingUndos((prev) => {
            const next = new Map(prev);
            next.delete(undoId);
            return next;
          });
        }, 5000);

        setPendingUndos((prev) => {
          const next = new Map(prev);
          next.set(undoId, {
            id: undoId,
            type: 'archive',
            messageId,
            timeoutId,
          });
          return next;
        });

        addToast({
          variant: 'success',
          message: 'Message archived',
          action: {
            label: 'Undo',
            onClick: () => {
              // Clear the timeout.
              const undo = pendingUndos.get(undoId);
              if (undo) {
                clearTimeout(undo.timeoutId);
              }
              setPendingUndos((prev) => {
                const next = new Map(prev);
                next.delete(undoId);
                return next;
              });
              // Unarchive.
              unarchiveMutation.mutate(messageId);
            },
          },
        });
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to archive message',
        });
      }
    },
    [archiveMutation, unarchiveMutation, addToast, pendingUndos],
  );

  // Handle snooze with toast.
  const handleSnooze = useCallback(
    async (messageId: number, until: string) => {
      try {
        await snoozeMutation.mutateAsync({ id: messageId, until });
        addToast({
          variant: 'success',
          message: 'Message snoozed',
        });
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to snooze message',
        });
      }
    },
    [snoozeMutation, addToast],
  );

  // Handle mark as read.
  const handleMarkRead = useCallback(
    async (messageId: number) => {
      try {
        await markReadMutation.mutateAsync(messageId);
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to mark message as read',
        });
      }
    },
    [markReadMutation, addToast],
  );

  // Handle acknowledge with toast.
  const handleAcknowledge = useCallback(
    async (messageId: number) => {
      try {
        await acknowledgeMutation.mutateAsync(messageId);
        addToast({
          variant: 'success',
          message: 'Message acknowledged',
        });
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to acknowledge message',
        });
      }
    },
    [acknowledgeMutation, addToast],
  );

  // Bulk actions for selected messages.
  const handleBulkStar = useCallback(
    async (messageIds: number[], starred: boolean) => {
      const results = await Promise.allSettled(
        messageIds.map((id) =>
          toggleStarMutation.mutateAsync({ id, starred }),
        ),
      );

      const failures = results.filter((r) => r.status === 'rejected').length;
      if (failures > 0) {
        addToast({
          variant: 'error',
          message: `Failed to ${starred ? 'star' : 'unstar'} ${failures} message(s)`,
        });
      } else if (messageIds.length > 1) {
        addToast({
          variant: 'success',
          message: `${starred ? 'Starred' : 'Unstarred'} ${messageIds.length} messages`,
        });
      }
    },
    [toggleStarMutation, addToast],
  );

  const handleBulkArchive = useCallback(
    async (messageIds: number[]) => {
      const results = await Promise.allSettled(
        messageIds.map((id) => archiveMutation.mutateAsync(id)),
      );

      const failures = results.filter((r) => r.status === 'rejected').length;
      const successes = messageIds.length - failures;

      if (failures > 0) {
        addToast({
          variant: 'error',
          message: `Failed to archive ${failures} message(s)`,
        });
      }

      if (successes > 0) {
        addToast({
          variant: 'success',
          message: `Archived ${successes} message(s)`,
          action: {
            label: 'Undo',
            onClick: () => {
              messageIds.forEach((id) => unarchiveMutation.mutate(id));
            },
          },
        });
      }
    },
    [archiveMutation, unarchiveMutation, addToast],
  );

  const handleBulkMarkRead = useCallback(
    async (messageIds: number[]) => {
      const results = await Promise.allSettled(
        messageIds.map((id) => markReadMutation.mutateAsync(id)),
      );

      const failures = results.filter((r) => r.status === 'rejected').length;
      if (failures > 0) {
        addToast({
          variant: 'error',
          message: `Failed to mark ${failures} message(s) as read`,
        });
      } else if (messageIds.length > 1) {
        addToast({
          variant: 'success',
          message: `Marked ${messageIds.length} messages as read`,
        });
      }
    },
    [markReadMutation, addToast],
  );

  return {
    // Single actions.
    star: handleStar,
    archive: handleArchive,
    snooze: handleSnooze,
    markRead: handleMarkRead,
    acknowledge: handleAcknowledge,

    // Bulk actions.
    bulkStar: handleBulkStar,
    bulkArchive: handleBulkArchive,
    bulkMarkRead: handleBulkMarkRead,

    // Loading states.
    isStarring: toggleStarMutation.isPending,
    isArchiving: archiveMutation.isPending,
    isSnoozing: snoozeMutation.isPending,
    isMarkingRead: markReadMutation.isPending,
    isAcknowledging: acknowledgeMutation.isPending,
  };
}

// Hook for delete confirmation.
export interface DeleteConfirmation {
  isOpen: boolean;
  messageId: number | null;
  messageIds: number[] | null;
}

export function useDeleteConfirmation() {
  const [confirmation, setConfirmation] = useState<DeleteConfirmation>({
    isOpen: false,
    messageId: null,
    messageIds: null,
  });

  const openSingleDelete = useCallback((messageId: number) => {
    setConfirmation({
      isOpen: true,
      messageId,
      messageIds: null,
    });
  }, []);

  const openBulkDelete = useCallback((messageIds: number[]) => {
    setConfirmation({
      isOpen: true,
      messageId: null,
      messageIds,
    });
  }, []);

  const closeDelete = useCallback(() => {
    setConfirmation({
      isOpen: false,
      messageId: null,
      messageIds: null,
    });
  }, []);

  return {
    confirmation,
    openSingleDelete,
    openBulkDelete,
    closeDelete,
  };
}

// Hook for snooze date picker.
export interface SnoozeState {
  isOpen: boolean;
  messageId: number | null;
}

export function useSnoozeModal() {
  const [snoozeState, setSnoozeState] = useState<SnoozeState>({
    isOpen: false,
    messageId: null,
  });

  const openSnooze = useCallback((messageId: number) => {
    setSnoozeState({
      isOpen: true,
      messageId,
    });
  }, []);

  const closeSnooze = useCallback(() => {
    setSnoozeState({
      isOpen: false,
      messageId: null,
    });
  }, []);

  return {
    snoozeState,
    openSnooze,
    closeSnooze,
  };
}

// Snooze duration options.
export interface SnoozeDuration {
  label: string;
  getDate: () => Date;
}

export const snoozeDurations: SnoozeDuration[] = [
  {
    label: 'Later today',
    getDate: () => {
      const date = new Date();
      date.setHours(date.getHours() + 3);
      return date;
    },
  },
  {
    label: 'Tomorrow morning',
    getDate: () => {
      const date = new Date();
      date.setDate(date.getDate() + 1);
      date.setHours(9, 0, 0, 0);
      return date;
    },
  },
  {
    label: 'Next week',
    getDate: () => {
      const date = new Date();
      date.setDate(date.getDate() + 7);
      date.setHours(9, 0, 0, 0);
      return date;
    },
  },
  {
    label: 'Next month',
    getDate: () => {
      const date = new Date();
      date.setMonth(date.getMonth() + 1);
      date.setHours(9, 0, 0, 0);
      return date;
    },
  },
];

// Hook for thread actions with integrated toast notifications.
export function useThreadActions() {
  const addToast = useUIStore((state) => state.addToast);

  // Thread mutations.
  const replyMutation = useReplyToThread();
  const archiveMutation = useArchiveThread();
  const unarchiveMutation = useUnarchiveThread();
  const markUnreadMutation = useMarkThreadUnread();
  const deleteMutation = useDeleteThread();

  // Handle reply with toast.
  const handleReply = useCallback(
    async (threadId: string, body: string) => {
      try {
        await replyMutation.mutateAsync({ id: threadId, body });
        addToast({
          variant: 'success',
          message: 'Reply sent',
        });
        return true;
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to send reply',
        });
        return false;
      }
    },
    [replyMutation, addToast],
  );

  // Handle archive with undo toast.
  const handleArchive = useCallback(
    async (threadId: string, onComplete?: () => void) => {
      try {
        await archiveMutation.mutateAsync(threadId);
        addToast({
          variant: 'success',
          message: 'Thread archived',
          action: {
            label: 'Undo',
            onClick: () => {
              unarchiveMutation.mutate(threadId);
            },
          },
        });
        onComplete?.();
        return true;
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to archive thread',
        });
        return false;
      }
    },
    [archiveMutation, unarchiveMutation, addToast],
  );

  // Handle mark as unread with toast.
  const handleMarkUnread = useCallback(
    async (threadId: string, onComplete?: () => void) => {
      try {
        await markUnreadMutation.mutateAsync(threadId);
        addToast({
          variant: 'success',
          message: 'Thread marked as unread',
        });
        onComplete?.();
        return true;
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to mark thread as unread',
        });
        return false;
      }
    },
    [markUnreadMutation, addToast],
  );

  // Handle delete with toast.
  const handleDelete = useCallback(
    async (threadId: string, onComplete?: () => void) => {
      try {
        await deleteMutation.mutateAsync(threadId);
        addToast({
          variant: 'success',
          message: 'Thread deleted',
        });
        onComplete?.();
        return true;
      } catch {
        addToast({
          variant: 'error',
          message: 'Failed to delete thread',
        });
        return false;
      }
    },
    [deleteMutation, addToast],
  );

  return {
    // Actions.
    reply: handleReply,
    archive: handleArchive,
    markUnread: handleMarkUnread,
    delete: handleDelete,

    // Loading states.
    isReplying: replyMutation.isPending,
    isArchiving: archiveMutation.isPending,
    isMarkingUnread: markUnreadMutation.isPending,
    isDeleting: deleteMutation.isPending,

    // Any action in progress.
    isProcessing:
      replyMutation.isPending ||
      archiveMutation.isPending ||
      markUnreadMutation.isPending ||
      deleteMutation.isPending,
  };
}
