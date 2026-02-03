// Hook for browser notification integration.

import { useCallback, useState } from 'react';
import { useNewMessages, useWebSocketConnection, type NewMessagePayload } from '@/hooks/useWebSocket.js';

// Notification permission state.
export type NotificationPermissionState = 'default' | 'granted' | 'denied';

// Notification preferences stored in localStorage.
export interface NotificationPreferences {
  enabled: boolean;
  showNewMessages: boolean;
  playSound: boolean;
}

// Default preferences.
const DEFAULT_PREFERENCES: NotificationPreferences = {
  enabled: true,
  showNewMessages: true,
  playSound: false,
};

// LocalStorage key for preferences.
const PREFERENCES_KEY = 'subtrate_notification_preferences';

// Load preferences from localStorage.
function loadPreferences(): NotificationPreferences {
  try {
    const stored = localStorage.getItem(PREFERENCES_KEY);
    if (stored) {
      return { ...DEFAULT_PREFERENCES, ...JSON.parse(stored) };
    }
  } catch {
    // Ignore parse errors.
  }
  return DEFAULT_PREFERENCES;
}

// Save preferences to localStorage.
function savePreferences(prefs: NotificationPreferences): void {
  try {
    localStorage.setItem(PREFERENCES_KEY, JSON.stringify(prefs));
  } catch {
    // Ignore storage errors.
  }
}

// Extended notification options with click handler.
export interface ExtendedNotificationOptions extends NotificationOptions {
  onClick?: () => void;
}

// Interface for notification state.
export interface NotificationState {
  permission: NotificationPermissionState;
  isSupported: boolean;
  preferences: NotificationPreferences;
  requestPermission: () => Promise<NotificationPermissionState>;
  updatePreferences: (updates: Partial<NotificationPreferences>) => void;
  showNotification: (title: string, options?: ExtendedNotificationOptions) => void;
}

// Hook to manage browser notifications.
export function useNotifications(): NotificationState {
  // Check if notifications are supported.
  const isSupported = typeof Notification !== 'undefined';

  // Use lazy initialization to avoid setState in effect.
  const [permission, setPermission] = useState<NotificationPermissionState>(() =>
    isSupported ? Notification.permission : 'default',
  );
  const [preferences, setPreferences] = useState<NotificationPreferences>(loadPreferences);

  // Request notification permission.
  const requestPermission = useCallback(async (): Promise<NotificationPermissionState> => {
    if (!isSupported) {
      return 'denied';
    }

    try {
      const result = await Notification.requestPermission();
      setPermission(result);
      return result;
    } catch {
      return 'denied';
    }
  }, [isSupported]);

  // Update preferences.
  const updatePreferences = useCallback((updates: Partial<NotificationPreferences>) => {
    setPreferences((prev) => {
      const newPrefs = { ...prev, ...updates };
      savePreferences(newPrefs);
      return newPrefs;
    });
  }, []);

  // Show a notification.
  const showNotification = useCallback(
    (title: string, options?: ExtendedNotificationOptions) => {
      if (!isSupported) {
        console.warn('Notifications not supported in this browser');
        return;
      }
      if (permission !== 'granted') {
        console.warn('Notification permission not granted:', permission);
        return;
      }
      if (!preferences.enabled) {
        console.warn('Notifications disabled in preferences');
        return;
      }

      try {
        // Extract onClick before passing to Notification constructor.
        const { onClick, ...notificationOptions } = options ?? {};

        // Create notification without icon if it might be missing.
        const notification = new Notification(title, {
          ...notificationOptions,
        });
        console.log('Notification created:', title);

        // Handle click to navigate to thread and focus window.
        notification.onclick = () => {
          window.focus();
          notification.close();
          onClick?.();
        };

        // Auto-close after 5 seconds.
        setTimeout(() => notification.close(), 5000);
      } catch (error) {
        console.error('Failed to create notification:', error);
      }
    },
    [isSupported, permission, preferences.enabled],
  );

  return {
    permission,
    isSupported,
    preferences,
    requestPermission,
    updatePreferences,
    showNotification,
  };
}

// Options for the new message notifications hook.
export interface NewMessageNotificationsOptions {
  onThreadClick?: (threadId: string) => void;
}

// Hook to automatically show notifications for new messages.
export function useNewMessageNotifications(
  options: NewMessageNotificationsOptions = {},
): void {
  const { onThreadClick } = options;
  const { permission, isSupported, preferences, showNotification } = useNotifications();
  const { state } = useWebSocketConnection();

  // Handle new message.
  const handleNewMessage = useCallback(
    (payload: NewMessagePayload) => {
      // Only show notifications when connected, permission granted, and enabled.
      if (
        !isSupported ||
        permission !== 'granted' ||
        !preferences.enabled ||
        !preferences.showNewMessages ||
        state !== 'connected'
      ) {
        return;
      }

      // Show the notification with click handler.
      showNotification(`New message from ${payload.sender_name}`, {
        body: payload.subject,
        tag: `message-${payload.id}`,
        onClick: () => {
          // Navigate to the thread when notification is clicked.
          if (onThreadClick && payload.thread_id) {
            onThreadClick(payload.thread_id);
          }
        },
      });
    },
    [isSupported, permission, preferences, state, showNotification, onThreadClick],
  );

  // Subscribe to new messages.
  useNewMessages(handleNewMessage);
}
