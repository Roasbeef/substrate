// Unit tests for NotificationSettings component.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NotificationSettings } from '@/components/layout/NotificationSettings.js';

// Mock Notification API.
const mockRequestPermission = vi.fn();

describe('NotificationSettings', () => {
  const originalNotification = globalThis.Notification;
  const originalLocalStorage = globalThis.localStorage;

  let mockLocalStorage: Record<string, string>;

  beforeEach(() => {
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

    // Mock Notification API with default permission.
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
    (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
    });
  });

  describe('when notifications not supported', () => {
    it('shows unsupported message', () => {
      delete (globalThis as unknown as Record<string, unknown>).Notification;

      render(<NotificationSettings />);

      expect(
        screen.getByText('Notifications are not supported in your browser.'),
      ).toBeInTheDocument();
    });
  });

  describe('permission states', () => {
    it('shows Enable button when permission is default', () => {
      render(<NotificationSettings />);

      expect(screen.getByText('Browser Permission')).toBeInTheDocument();
      expect(screen.getByText('Permission has not been requested yet.')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Enable' })).toBeInTheDocument();
    });

    it('shows Enabled badge when permission is granted', () => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'granted';

      render(<NotificationSettings />);

      expect(screen.getByText('Notifications are enabled.')).toBeInTheDocument();
      expect(screen.getByText('Enabled')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Enable' })).not.toBeInTheDocument();
    });

    it('shows Blocked badge when permission is denied', () => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'denied';

      render(<NotificationSettings />);

      expect(screen.getByText('Notifications are blocked by your browser.')).toBeInTheDocument();
      expect(screen.getByText('Blocked')).toBeInTheDocument();
      expect(
        screen.getByText('To enable notifications, update your browser settings for this site.'),
      ).toBeInTheDocument();
    });

    it('requests permission when Enable button clicked', async () => {
      const user = userEvent.setup();

      render(<NotificationSettings />);

      await user.click(screen.getByRole('button', { name: 'Enable' }));

      expect(mockRequestPermission).toHaveBeenCalled();
    });
  });

  describe('preference toggles', () => {
    beforeEach(() => {
      // Set permission to granted so toggles are enabled.
      (globalThis.Notification as unknown as { permission: string }).permission = 'granted';
    });

    it('shows all preference toggles', () => {
      render(<NotificationSettings />);

      expect(screen.getByText('Enable notifications')).toBeInTheDocument();
      expect(screen.getByText('New message notifications')).toBeInTheDocument();
      expect(screen.getByText('Play sound')).toBeInTheDocument();
    });

    it('toggles enabled preference', async () => {
      const user = userEvent.setup();

      render(<NotificationSettings />);

      // Find the first toggle (Enable notifications).
      const toggles = screen.getAllByRole('switch');
      const enableToggle = toggles[0];

      expect(enableToggle).toHaveAttribute('aria-checked', 'true');

      await user.click(enableToggle!);

      await waitFor(() => {
        expect(enableToggle).toHaveAttribute('aria-checked', 'false');
      });

      // Check localStorage was updated.
      const saved = JSON.parse(mockLocalStorage['subtrate_notification_preferences'] ?? '{}');
      expect(saved.enabled).toBe(false);
    });

    it('toggles showNewMessages preference', async () => {
      const user = userEvent.setup();

      render(<NotificationSettings />);

      // Second toggle is showNewMessages.
      const toggles = screen.getAllByRole('switch');
      const messageToggle = toggles[1];

      expect(messageToggle).toHaveAttribute('aria-checked', 'true');

      await user.click(messageToggle!);

      await waitFor(() => {
        expect(messageToggle).toHaveAttribute('aria-checked', 'false');
      });
    });

    it('toggles playSound preference', async () => {
      const user = userEvent.setup();

      render(<NotificationSettings />);

      // Third toggle is playSound.
      const toggles = screen.getAllByRole('switch');
      const soundToggle = toggles[2];

      expect(soundToggle).toHaveAttribute('aria-checked', 'false');

      await user.click(soundToggle!);

      await waitFor(() => {
        expect(soundToggle).toHaveAttribute('aria-checked', 'true');
      });
    });

    it('disables toggles when master toggle is off', async () => {
      const user = userEvent.setup();

      render(<NotificationSettings />);

      const toggles = screen.getAllByRole('switch');
      const enableToggle = toggles[0];
      const messageToggle = toggles[1];
      const soundToggle = toggles[2];

      // Turn off the master toggle.
      await user.click(enableToggle!);

      await waitFor(() => {
        expect(messageToggle).toBeDisabled();
        expect(soundToggle).toBeDisabled();
      });
    });

    it('disables all toggles when permission is not granted', () => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'default';

      render(<NotificationSettings />);

      const toggles = screen.getAllByRole('switch');
      toggles.forEach((toggle) => {
        expect(toggle).toBeDisabled();
      });
    });
  });

  describe('loading saved preferences', () => {
    beforeEach(() => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'granted';
    });

    it('loads saved preferences from localStorage', () => {
      mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify({
        enabled: false,
        showNewMessages: false,
        playSound: true,
      });

      render(<NotificationSettings />);

      const toggles = screen.getAllByRole('switch');
      expect(toggles[0]).toHaveAttribute('aria-checked', 'false'); // enabled
      expect(toggles[1]).toHaveAttribute('aria-checked', 'false'); // showNewMessages
      expect(toggles[2]).toHaveAttribute('aria-checked', 'true'); // playSound
    });
  });

  it('applies custom className', () => {
    render(<NotificationSettings className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });
});
