// Hook for showing in-app toast notifications when new messages arrive.

import { useCallback, useRef } from 'react';
import { useNewMessages, useWebSocketConnection, type NewMessagePayload } from '@/hooks/useWebSocket.js';
import { useUIStore } from '@/stores/ui.js';
import { useNavigate, useLocation } from 'react-router-dom';

// Priority levels for styling.
type MessagePriority = 'urgent' | 'normal' | 'low';

// Interface for toast notification preferences.
export interface MessageToastPreferences {
  enabled: boolean;
  showUrgentOnly: boolean;
}

// Default preferences.
const DEFAULT_PREFERENCES: MessageToastPreferences = {
  enabled: true,
  showUrgentOnly: false,
};

// LocalStorage key for preferences.
const PREFERENCES_KEY = 'subtrate_message_toast_preferences';

// Load preferences from localStorage.
function loadPreferences(): MessageToastPreferences {
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
export function saveMessageToastPreferences(prefs: MessageToastPreferences): void {
  try {
    localStorage.setItem(PREFERENCES_KEY, JSON.stringify(prefs));
  } catch {
    // Ignore storage errors.
  }
}

// Get the priority from message payload.
function getMessagePriority(priority: string): MessagePriority {
  const normalized = priority.toLowerCase();
  if (normalized === 'urgent' || normalized === 'high') {
    return 'urgent';
  }
  if (normalized === 'low') {
    return 'low';
  }
  return 'normal';
}

// Hook to show toast notifications for new messages.
// This provides immediate visual feedback in the UI when messages arrive.
export function useMessageToasts(): void {
  const addToast = useUIStore((state) => state.addToast);
  const setPendingThread = useUIStore((state) => state.setPendingThread);
  const { state } = useWebSocketConnection();
  const navigate = useNavigate();
  const location = useLocation();

  // Track shown message IDs to prevent duplicates.
  const shownIds = useRef(new Set<number>());

  // Handle new message.
  const handleNewMessage = useCallback(
    (payload: NewMessagePayload) => {
      // Only show toasts when connected.
      if (state !== 'connected') {
        return;
      }

      // Skip if we've already shown a toast for this message.
      if (shownIds.current.has(payload.id)) {
        return;
      }
      shownIds.current.add(payload.id);

      // Clean up old IDs periodically (keep last 50).
      if (shownIds.current.size > 100) {
        const ids = Array.from(shownIds.current);
        shownIds.current = new Set(ids.slice(-50));
      }

      // Load preferences.
      const prefs = loadPreferences();
      if (!prefs.enabled) {
        return;
      }

      const priority = getMessagePriority(payload.priority);

      // Skip non-urgent messages if showUrgentOnly is enabled.
      if (prefs.showUrgentOnly && priority !== 'urgent') {
        return;
      }

      // Determine toast variant based on priority.
      const variant = priority === 'urgent' ? 'warning' : 'info';

      // Truncate subject if too long.
      const maxSubjectLength = 50;
      const subject =
        payload.subject.length > maxSubjectLength
          ? payload.subject.substring(0, maxSubjectLength) + '...'
          : payload.subject;

      // Set duration based on priority (urgent messages stay longer).
      const duration = priority === 'urgent' ? 10000 : 5000;

      // Show the toast notification.
      addToast({
        variant,
        title: `New message from ${payload.sender_name}`,
        message: subject,
        duration,
        action: {
          label: 'View',
          onClick: () => {
            // Set the pending thread ID for InboxPage to pick up.
            setPendingThread(payload.thread_id);

            // If already on inbox, force navigation with thread param.
            // Otherwise, navigate to inbox and let useEffect handle it.
            if (location.pathname === '/inbox' || location.pathname === '/') {
              // Navigate with thread ID in URL to force InboxPage to open it.
              navigate(`/inbox/thread/${payload.thread_id}`);
            } else {
              navigate('/inbox');
            }
          },
        },
      });
    },
    [state, addToast, navigate, setPendingThread, location.pathname],
  );

  // Subscribe to new messages.
  useNewMessages(handleNewMessage);
}

// Hook to get and update message toast preferences.
export function useMessageToastPreferences(): {
  preferences: MessageToastPreferences;
  updatePreferences: (updates: Partial<MessageToastPreferences>) => void;
} {
  // Load preferences from localStorage.
  const preferences = loadPreferences();

  // Update preferences.
  const updatePreferences = useCallback((updates: Partial<MessageToastPreferences>) => {
    const newPrefs = { ...loadPreferences(), ...updates };
    saveMessageToastPreferences(newPrefs);
  }, []);

  return {
    preferences,
    updatePreferences,
  };
}
