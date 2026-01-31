// Unit tests for NotificationPrompt component.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NotificationPrompt, resetPromptDismissed } from '@/components/layout/NotificationPrompt.js';

// Mock Notification API.
const mockRequestPermission = vi.fn();

describe('NotificationPrompt', () => {
  const originalNotification = globalThis.Notification;
  const originalLocalStorage = globalThis.localStorage;

  let mockLocalStorage: Record<string, string>;

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();

    // Mock localStorage.
    mockLocalStorage = {};
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: (key: string) => mockLocalStorage[key] ?? null,
        setItem: (key: string, value: string) => {
          mockLocalStorage[key] = value;
        },
        removeItem: (key: string) => {
          delete mockLocalStorage[key];
        },
        clear: () => {
          mockLocalStorage = {};
        },
      },
      writable: true,
    });

    // Mock Notification API.
    mockRequestPermission.mockResolvedValue('granted');
    (globalThis as unknown as { Notification: unknown }).Notification = Object.assign(
      vi.fn(),
      {
        permission: 'default' as NotificationPermission,
        requestPermission: mockRequestPermission,
      },
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
    });
    resetPromptDismissed();
  });

  it('shows prompt after delay when permission is default', async () => {
    render(<NotificationPrompt />);

    // Prompt should not be visible initially.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    // Advance timers past the 2 second delay.
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    // Now the prompt should be visible.
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText('Enable notifications')).toBeInTheDocument();
  });

  it('does not show prompt when permission is already granted', async () => {
    (globalThis.Notification as unknown as { permission: string }).permission = 'granted';

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not show prompt when permission is denied', async () => {
    (globalThis.Notification as unknown as { permission: string }).permission = 'denied';

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not show prompt when notifications are not supported', async () => {
    delete (globalThis as unknown as Record<string, unknown>).Notification;

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not show prompt when previously dismissed', async () => {
    mockLocalStorage['subtrate_notification_prompt_dismissed'] = 'true';

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('requests permission when Enable button is clicked', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    // Click Enable button.
    await user.click(screen.getByRole('button', { name: 'Enable' }));

    expect(mockRequestPermission).toHaveBeenCalled();
  });

  it('hides prompt after permission is granted', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    mockRequestPermission.mockResolvedValue('granted');

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Enable' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('hides prompt after permission is denied', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    mockRequestPermission.mockResolvedValue('denied');

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Enable' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('dismisses prompt and saves to localStorage when Not now clicked', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Not now' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    expect(mockLocalStorage['subtrate_notification_prompt_dismissed']).toBe('true');
  });

  it('dismisses prompt when close button clicked', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Dismiss notification prompt' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    expect(mockLocalStorage['subtrate_notification_prompt_dismissed']).toBe('true');
  });

  it('shows loading state while requesting permission', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    // Make request permission hang.
    let resolvePermission: (value: string) => void;
    mockRequestPermission.mockReturnValue(
      new Promise((resolve) => {
        resolvePermission = resolve;
      }),
    );

    render(<NotificationPrompt />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    // Click Enable button (don't await).
    await user.click(screen.getByRole('button', { name: 'Enable' }));

    // Should show loading state.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Enabling...' })).toBeInTheDocument();
    });

    // Resolve the permission.
    await act(async () => {
      resolvePermission!('granted');
    });

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('applies custom className', async () => {
    render(<NotificationPrompt className="custom-class" />);

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveClass('custom-class');
    });
  });
});
