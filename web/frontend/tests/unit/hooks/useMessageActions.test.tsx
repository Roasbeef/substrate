// Unit tests for useMessageActions hook.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  useMessageActions,
  useDeleteConfirmation,
  useSnoozeModal,
  snoozeDurations,
} from '@/hooks/useMessageActions.js';
import { useUIStore } from '@/stores/ui.js';
import * as messagesApi from '@/api/messages.js';
import type { ReactNode } from 'react';

// Mock the messages API.
vi.mock('@/api/messages.js', () => ({
  toggleMessageStar: vi.fn(),
  archiveMessage: vi.fn(),
  unarchiveMessage: vi.fn(),
  snoozeMessage: vi.fn(),
  markMessageRead: vi.fn(),
  acknowledgeMessage: vi.fn(),
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

describe('useMessageActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset UI store.
    useUIStore.setState({ toasts: [] });
    // Default mocks return success.
    vi.mocked(messagesApi.toggleMessageStar).mockResolvedValue(undefined);
    vi.mocked(messagesApi.archiveMessage).mockResolvedValue(undefined);
    vi.mocked(messagesApi.unarchiveMessage).mockResolvedValue(undefined);
    vi.mocked(messagesApi.snoozeMessage).mockResolvedValue(undefined);
    vi.mocked(messagesApi.markMessageRead).mockResolvedValue(undefined);
    vi.mocked(messagesApi.acknowledgeMessage).mockResolvedValue(undefined);
  });

  describe('star', () => {
    it('calls toggleMessageStar with correct params', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.star(1, true);
      });

      expect(messagesApi.toggleMessageStar).toHaveBeenCalledWith(1, true);
    });

    it('shows error toast on failure', async () => {
      vi.mocked(messagesApi.toggleMessageStar).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.star(1, true);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error')).toBe(true);
    });
  });

  describe('archive', () => {
    it('calls archiveMessage and shows success toast', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      expect(messagesApi.archiveMessage).toHaveBeenCalledWith(1);

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Message archived')).toBe(true);
    });

    it('shows toast with undo action', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      const toasts = useUIStore.getState().toasts;
      const archiveToast = toasts.find((t) => t.message === 'Message archived');
      expect(archiveToast?.action?.label).toBe('Undo');
    });

    it('shows error toast on failure', async () => {
      vi.mocked(messagesApi.archiveMessage).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.archive(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error')).toBe(true);
    });
  });

  describe('snooze', () => {
    it('calls snoozeMessage and shows success toast', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      const until = new Date().toISOString();

      await act(async () => {
        await result.current.snooze(1, until);
      });

      expect(messagesApi.snoozeMessage).toHaveBeenCalledWith(1, until);

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Message snoozed')).toBe(true);
    });

    it('shows error toast on failure', async () => {
      vi.mocked(messagesApi.snoozeMessage).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.snooze(1, new Date().toISOString());
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error')).toBe(true);
    });
  });

  describe('markRead', () => {
    it('calls markMessageRead', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markRead(1);
      });

      expect(messagesApi.markMessageRead).toHaveBeenCalledWith(1);
    });

    it('shows error toast on failure', async () => {
      vi.mocked(messagesApi.markMessageRead).mockRejectedValue(new Error('fail'));

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.markRead(1);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error')).toBe(true);
    });
  });

  describe('acknowledge', () => {
    it('calls acknowledgeMessage and shows success toast', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.acknowledge(1);
      });

      expect(messagesApi.acknowledgeMessage).toHaveBeenCalledWith(1);

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'success' && t.message === 'Message acknowledged')).toBe(true);
    });
  });

  describe('bulk actions', () => {
    it('bulkStar calls toggleMessageStar for each message', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.bulkStar([1, 2, 3], true);
      });

      expect(messagesApi.toggleMessageStar).toHaveBeenCalledTimes(3);
      expect(messagesApi.toggleMessageStar).toHaveBeenCalledWith(1, true);
      expect(messagesApi.toggleMessageStar).toHaveBeenCalledWith(2, true);
      expect(messagesApi.toggleMessageStar).toHaveBeenCalledWith(3, true);
    });

    it('bulkArchive calls archiveMessage for each message', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.bulkArchive([1, 2]);
      });

      expect(messagesApi.archiveMessage).toHaveBeenCalledTimes(2);
    });

    it('bulkMarkRead calls markMessageRead for each message', async () => {
      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.bulkMarkRead([1, 2, 3]);
      });

      expect(messagesApi.markMessageRead).toHaveBeenCalledTimes(3);
    });

    it('bulkStar shows error toast for partial failures', async () => {
      vi.mocked(messagesApi.toggleMessageStar)
        .mockResolvedValueOnce(undefined)
        .mockRejectedValueOnce(new Error('fail'));

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await result.current.bulkStar([1, 2], true);
      });

      const toasts = useUIStore.getState().toasts;
      expect(toasts.some((t) => t.variant === 'error')).toBe(true);
    });
  });

  describe('loading states', () => {
    it('isStarring is true during star operation', async () => {
      let resolvePromise: () => void;
      vi.mocked(messagesApi.toggleMessageStar).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = () => resolve(undefined);
        }),
      );

      const { result } = renderHook(() => useMessageActions(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isStarring).toBe(false);

      act(() => {
        void result.current.star(1, true);
      });

      await waitFor(() => {
        expect(result.current.isStarring).toBe(true);
      });

      await act(async () => {
        resolvePromise!();
      });

      await waitFor(() => {
        expect(result.current.isStarring).toBe(false);
      });
    });
  });
});

describe('useDeleteConfirmation', () => {
  it('starts with closed state', () => {
    const { result } = renderHook(() => useDeleteConfirmation());

    expect(result.current.confirmation.isOpen).toBe(false);
    expect(result.current.confirmation.messageId).toBeNull();
    expect(result.current.confirmation.messageIds).toBeNull();
  });

  it('opens single delete confirmation', () => {
    const { result } = renderHook(() => useDeleteConfirmation());

    act(() => {
      result.current.openSingleDelete(123);
    });

    expect(result.current.confirmation.isOpen).toBe(true);
    expect(result.current.confirmation.messageId).toBe(123);
    expect(result.current.confirmation.messageIds).toBeNull();
  });

  it('opens bulk delete confirmation', () => {
    const { result } = renderHook(() => useDeleteConfirmation());

    act(() => {
      result.current.openBulkDelete([1, 2, 3]);
    });

    expect(result.current.confirmation.isOpen).toBe(true);
    expect(result.current.confirmation.messageId).toBeNull();
    expect(result.current.confirmation.messageIds).toEqual([1, 2, 3]);
  });

  it('closes confirmation', () => {
    const { result } = renderHook(() => useDeleteConfirmation());

    act(() => {
      result.current.openSingleDelete(123);
    });

    act(() => {
      result.current.closeDelete();
    });

    expect(result.current.confirmation.isOpen).toBe(false);
    expect(result.current.confirmation.messageId).toBeNull();
  });
});

describe('useSnoozeModal', () => {
  it('starts with closed state', () => {
    const { result } = renderHook(() => useSnoozeModal());

    expect(result.current.snoozeState.isOpen).toBe(false);
    expect(result.current.snoozeState.messageId).toBeNull();
  });

  it('opens snooze modal', () => {
    const { result } = renderHook(() => useSnoozeModal());

    act(() => {
      result.current.openSnooze(456);
    });

    expect(result.current.snoozeState.isOpen).toBe(true);
    expect(result.current.snoozeState.messageId).toBe(456);
  });

  it('closes snooze modal', () => {
    const { result } = renderHook(() => useSnoozeModal());

    act(() => {
      result.current.openSnooze(456);
    });

    act(() => {
      result.current.closeSnooze();
    });

    expect(result.current.snoozeState.isOpen).toBe(false);
    expect(result.current.snoozeState.messageId).toBeNull();
  });
});

describe('snoozeDurations', () => {
  it('has expected number of presets', () => {
    expect(snoozeDurations.length).toBe(4);
  });

  it('each duration has label and getDate', () => {
    snoozeDurations.forEach((duration) => {
      expect(duration.label).toBeDefined();
      expect(typeof duration.getDate).toBe('function');
    });
  });

  it('getDate returns future date for each duration', () => {
    const now = new Date();
    snoozeDurations.forEach((duration) => {
      const date = duration.getDate();
      expect(date.getTime()).toBeGreaterThan(now.getTime());
    });
  });

  it('later today is within 24 hours', () => {
    const now = new Date();
    const laterToday = snoozeDurations.find((d) => d.label === 'Later today');
    expect(laterToday).toBeDefined();

    const date = laterToday!.getDate();
    const diffHours = (date.getTime() - now.getTime()) / (1000 * 60 * 60);
    expect(diffHours).toBeLessThanOrEqual(24);
    expect(diffHours).toBeGreaterThan(0);
  });

  it('tomorrow morning is next day at 9am', () => {
    const tomorrowMorning = snoozeDurations.find((d) => d.label === 'Tomorrow morning');
    expect(tomorrowMorning).toBeDefined();

    const date = tomorrowMorning!.getDate();
    expect(date.getHours()).toBe(9);
    expect(date.getMinutes()).toBe(0);
  });

  it('next week is about 7 days away', () => {
    const now = new Date();
    const nextWeek = snoozeDurations.find((d) => d.label === 'Next week');
    expect(nextWeek).toBeDefined();

    const date = nextWeek!.getDate();
    const diffDays = (date.getTime() - now.getTime()) / (1000 * 60 * 60 * 24);
    expect(diffDays).toBeGreaterThanOrEqual(6.5);
    expect(diffDays).toBeLessThanOrEqual(8);
  });

  it('next month is about 30 days away', () => {
    const now = new Date();
    const nextMonth = snoozeDurations.find((d) => d.label === 'Next month');
    expect(nextMonth).toBeDefined();

    const date = nextMonth!.getDate();
    const diffDays = (date.getTime() - now.getTime()) / (1000 * 60 * 60 * 24);
    expect(diffDays).toBeGreaterThanOrEqual(27);
    expect(diffDays).toBeLessThanOrEqual(32);
  });
});
