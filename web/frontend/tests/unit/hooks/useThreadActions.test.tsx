// Unit tests for useThreadActions hook.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useThreadActions } from '@/hooks/useMessageActions.js';
import { useUIStore } from '@/stores/ui.js';
import * as threadsApi from '@/api/threads.js';
import type { ReactNode } from 'react';

// Mock the threads API.
vi.mock('@/api/threads.js', () => ({
  fetchThread: vi.fn(),
  replyToThread: vi.fn(),
  archiveThread: vi.fn(),
  unarchiveThread: vi.fn(),
  markThreadUnread: vi.fn(),
  deleteThread: vi.fn(),
}));

// Test wrapper with QueryClient.
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe('useThreadActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset UI store.
    useUIStore.setState({ toasts: [] });
    // Default mocks return success.
    vi.mocked(threadsApi.replyToThread).mockResolvedValue(undefined);
    vi.mocked(threadsApi.archiveThread).mockResolvedValue(undefined);
    vi.mocked(threadsApi.unarchiveThread).mockResolvedValue(undefined);
    vi.mocked(threadsApi.markThreadUnread).mockResolvedValue(undefined);
    vi.mocked(threadsApi.deleteThread).mockResolvedValue(undefined);
  });

  describe('reply', () => {
    it('calls replyToThread with correct params', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.reply(1, 'Test reply message');
      });

      expect(threadsApi.replyToThread).toHaveBeenCalledWith(1, 'Test reply message');
    });

    it('shows success toast on reply', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.reply(1, 'Test reply');
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Reply sent')).toBe(true);
    });

    it('returns true on success', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      let success = false;
      await act(async () => {
        success = await result.current.reply(1, 'Test reply');
      });

      expect(success).toBe(true);
    });

    it('shows error toast on failure', async () => {
      vi.mocked(threadsApi.replyToThread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.reply(1, 'Test reply');
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error' && t.message === 'Failed to send reply')).toBe(true);
    });

    it('returns false on failure', async () => {
      vi.mocked(threadsApi.replyToThread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      let success = true;
      await act(async () => {
        success = await result.current.reply(1, 'Test reply');
      });

      expect(success).toBe(false);
    });
  });

  describe('archive', () => {
    it('calls archiveThread with correct id', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      expect(threadsApi.archiveThread).toHaveBeenCalledWith(1);
    });

    it('shows success toast with undo action', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      const toasts = useUIStore.getState().toasts;
      const archiveToast = toasts.find((t) => t.message === 'Thread archived');
      expect(archiveToast).toBeDefined();
      expect(archiveToast?.variant).toBe('success');
      expect(archiveToast?.action?.label).toBe('Undo');
    });

    it('calls onComplete callback after success', async () => {
      const onComplete = vi.fn();
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1, onComplete);
      });

      expect(onComplete).toHaveBeenCalled();
    });

    it('shows error toast on failure', async () => {
      vi.mocked(threadsApi.archiveThread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error' && t.message === 'Failed to archive thread')).toBe(true);
    });

    it('does not call onComplete on failure', async () => {
      vi.mocked(threadsApi.archiveThread).mockRejectedValue(new Error('fail'));
      const onComplete = vi.fn();

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1, onComplete);
      });

      expect(onComplete).not.toHaveBeenCalled();
    });
  });

  describe('markUnread', () => {
    it('calls markThreadUnread with correct id', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markUnread(1);
      });

      expect(threadsApi.markThreadUnread).toHaveBeenCalledWith(1);
    });

    it('shows success toast', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markUnread(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Thread marked as unread')).toBe(true);
    });

    it('calls onComplete callback after success', async () => {
      const onComplete = vi.fn();
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markUnread(1, onComplete);
      });

      expect(onComplete).toHaveBeenCalled();
    });

    it('shows error toast on failure', async () => {
      vi.mocked(threadsApi.markThreadUnread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markUnread(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error' && t.message === 'Failed to mark thread as unread')).toBe(true);
    });
  });

  describe('delete', () => {
    it('calls deleteThread with correct id', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.delete(1);
      });

      expect(threadsApi.deleteThread).toHaveBeenCalledWith(1);
    });

    it('shows success toast', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.delete(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Thread deleted')).toBe(true);
    });

    it('calls onComplete callback after success', async () => {
      const onComplete = vi.fn();
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.delete(1, onComplete);
      });

      expect(onComplete).toHaveBeenCalled();
    });

    it('shows error toast on failure', async () => {
      vi.mocked(threadsApi.deleteThread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.delete(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error' && t.message === 'Failed to delete thread')).toBe(true);
    });

    it('returns false on failure', async () => {
      vi.mocked(threadsApi.deleteThread).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      let success = true;
      await act(async () => {
        success = await result.current.delete(1);
      });

      expect(success).toBe(false);
    });
  });

  describe('loading states', () => {
    it('isReplying is true during reply operation', async () => {
      let resolvePromise: () => void;
      vi.mocked(threadsApi.replyToThread).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isReplying).toBe(false);

      act(() => {
        void result.current.reply(1, 'Test reply');
      });

      await waitFor(() => {
        expect(result.current.isReplying).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isReplying).toBe(false);
      });
    });

    it('isArchiving is true during archive operation', async () => {
      let resolvePromise: () => void;
      vi.mocked(threadsApi.archiveThread).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isArchiving).toBe(false);

      act(() => {
        void result.current.archive(1);
      });

      await waitFor(() => {
        expect(result.current.isArchiving).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isArchiving).toBe(false);
      });
    });

    it('isMarkingUnread is true during markUnread operation', async () => {
      let resolvePromise: () => void;
      vi.mocked(threadsApi.markThreadUnread).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isMarkingUnread).toBe(false);

      act(() => {
        void result.current.markUnread(1);
      });

      await waitFor(() => {
        expect(result.current.isMarkingUnread).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isMarkingUnread).toBe(false);
      });
    });

    it('isDeleting is true during delete operation', async () => {
      let resolvePromise: () => void;
      vi.mocked(threadsApi.deleteThread).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isDeleting).toBe(false);

      act(() => {
        void result.current.delete(1);
      });

      await waitFor(() => {
        expect(result.current.isDeleting).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isDeleting).toBe(false);
      });
    });

    it('isProcessing is true when any operation is in progress', async () => {
      let resolvePromise: () => void;
      vi.mocked(threadsApi.replyToThread).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isProcessing).toBe(false);

      act(() => {
        void result.current.reply(1, 'Test');
      });

      await waitFor(() => {
        expect(result.current.isProcessing).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isProcessing).toBe(false);
      });
    });
  });

  describe('undo functionality', () => {
    it('undo action calls unarchiveThread', async () => {
      const { result } = renderHook(() => useThreadActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      const toasts = useUIStore.getState().toasts;
      const archiveToast = toasts.find((t) => t.message === 'Thread archived');
      expect(archiveToast?.action).toBeDefined();

      // Trigger the undo action.
      act(() => {
        archiveToast?.action?.onClick();
      });

      await waitFor(() => {
        expect(threadsApi.unarchiveThread).toHaveBeenCalledWith(1);
      });
    });
  });
});
