// Prompt component for requesting notification permission.

import { useCallback, useEffect, useState } from 'react';
import { useNotifications } from '@/hooks/useNotifications.js';
import { Button } from '@/components/ui/Button.js';

// Storage key for tracking if the prompt has been dismissed.
const DISMISSED_KEY = 'subtrate_notification_prompt_dismissed';

// Props for NotificationPrompt component.
export interface NotificationPromptProps {
  className?: string;
}

// Check if the prompt has been dismissed.
function isPromptDismissed(): boolean {
  try {
    return localStorage.getItem(DISMISSED_KEY) === 'true';
  } catch {
    return false;
  }
}

// Mark the prompt as dismissed.
function dismissPrompt(): void {
  try {
    localStorage.setItem(DISMISSED_KEY, 'true');
  } catch {
    // Ignore storage errors.
  }
}

// Clear the dismissed state (for testing or reset).
export function resetPromptDismissed(): void {
  try {
    localStorage.removeItem(DISMISSED_KEY);
  } catch {
    // Ignore storage errors.
  }
}

// Component that prompts users to enable notifications.
export function NotificationPrompt({ className = '' }: NotificationPromptProps): JSX.Element | null {
  const { isSupported, permission, requestPermission } = useNotifications();
  const [isVisible, setIsVisible] = useState(false);
  const [isRequesting, setIsRequesting] = useState(false);

  // Check if we should show the prompt.
  useEffect(() => {
    // Don't show if:
    // - Notifications not supported
    // - Permission already granted or denied
    // - Prompt was previously dismissed
    if (!isSupported || permission !== 'default' || isPromptDismissed()) {
      setIsVisible(false);
      return;
    }

    // Show the prompt after a short delay to avoid being intrusive.
    const timer = setTimeout(() => {
      setIsVisible(true);
    }, 2000);

    return () => clearTimeout(timer);
  }, [isSupported, permission]);

  // Handle enable button click.
  const handleEnable = useCallback(async () => {
    setIsRequesting(true);
    try {
      const result = await requestPermission();
      if (result === 'granted' || result === 'denied') {
        setIsVisible(false);
      }
    } finally {
      setIsRequesting(false);
    }
  }, [requestPermission]);

  // Handle dismiss button click.
  const handleDismiss = useCallback(() => {
    dismissPrompt();
    setIsVisible(false);
  }, []);

  // Don't render if not visible.
  if (!isVisible) {
    return null;
  }

  return (
    <div
      className={`fixed bottom-4 right-4 z-50 max-w-sm rounded-lg border border-gray-200 bg-white p-4 shadow-lg dark:border-gray-700 dark:bg-gray-800 ${className}`}
      role="alert"
      aria-live="polite"
    >
      <div className="flex items-start gap-3">
        {/* Bell icon. */}
        <div className="flex-shrink-0 rounded-full bg-indigo-100 p-2 dark:bg-indigo-900">
          <svg
            className="h-5 w-5 text-indigo-600 dark:text-indigo-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
            />
          </svg>
        </div>

        {/* Content. */}
        <div className="flex-1">
          <h3 className="text-sm font-medium text-gray-900 dark:text-gray-100">
            Enable notifications
          </h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Get notified when you receive new messages from agents.
          </p>

          {/* Buttons. */}
          <div className="mt-3 flex gap-2">
            <Button
              size="sm"
              onClick={handleEnable}
              disabled={isRequesting}
              aria-busy={isRequesting}
            >
              {isRequesting ? 'Enabling...' : 'Enable'}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleDismiss}
              disabled={isRequesting}
            >
              Not now
            </Button>
          </div>
        </div>

        {/* Close button. */}
        <button
          type="button"
          className="flex-shrink-0 rounded-lg p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          onClick={handleDismiss}
          aria-label="Dismiss notification prompt"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
    </div>
  );
}
