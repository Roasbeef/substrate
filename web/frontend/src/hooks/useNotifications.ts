// Hook for browser notification integration.

import { useCallback, useEffect, useState } from 'react';
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

// Interface for notification state.
export interface NotificationState {
  permission: NotificationPermissionState;
  isSupported: boolean;
  preferences: NotificationPreferences;
  requestPermission: () => Promise<NotificationPermissionState>;
  updatePreferences: (updates: Partial<NotificationPreferences>) => void;
  showNotification: (title: string, options?: NotificationOptions) => void;
}

// Hook to manage browser notifications.
export function useNotifications(): NotificationState {
  const [permission, setPermission] = useState<NotificationPermissionState>('default');
  const [preferences, setPreferences] = useState<NotificationPreferences>(DEFAULT_PREFERENCES);

  // Check if notifications are supported.
  const isSupported = typeof Notification !== 'undefined';

  // Initialize permission and preferences on mount.
  useEffect(() => {
    if (isSupported) {
      setPermission(Notification.permission);
    }
    setPreferences(loadPreferences());
  }, [isSupported]);

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
    (title: string, options?: NotificationOptions) => {
      if (!isSupported || permission !== 'granted' || !preferences.enabled) {
        return;
      }

      try {
        new Notification(title, {
          icon: '/static/icons/notification-icon.png',
          ...options,
        });
      } catch {
        // Ignore notification errors.
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

// Hook to automatically show notifications for new messages.
export function useNewMessageNotifications(): void {
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

      // Show the notification.
      showNotification(`New message from ${payload.sender_name}`, {
        body: payload.subject,
        tag: `message-${payload.id}`,
      });
    },
    [isSupported, permission, preferences, state, showNotification],
  );

  // Subscribe to new messages.
  useNewMessages(handleNewMessage);
}
