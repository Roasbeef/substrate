// Component for managing notification settings.

import { useCallback, type JSX } from 'react';
import { useNotifications } from '@/hooks/useNotifications.js';
import { Button } from '@/components/ui/Button.js';

// Props for NotificationSettings component.
export interface NotificationSettingsProps {
  className?: string;
}

// Component for managing notification preferences.
export function NotificationSettings({ className = '' }: NotificationSettingsProps): JSX.Element {
  const {
    isSupported,
    permission,
    preferences,
    requestPermission,
    updatePreferences,
    showNotification,
  } = useNotifications();

  // Handle test notification.
  const handleTestNotification = useCallback(() => {
    showNotification('Test Notification', {
      body: 'This is a test notification from Subtrate. Notifications are working correctly!',
      tag: 'test-notification',
    });
  }, [showNotification]);

  // Handle permission request.
  const handleRequestPermission = useCallback(async () => {
    await requestPermission();
  }, [requestPermission]);

  // Handle toggle for enabled.
  const handleToggleEnabled = useCallback(() => {
    updatePreferences({ enabled: !preferences.enabled });
  }, [preferences.enabled, updatePreferences]);

  // Handle toggle for showNewMessages.
  const handleToggleShowNewMessages = useCallback(() => {
    updatePreferences({ showNewMessages: !preferences.showNewMessages });
  }, [preferences.showNewMessages, updatePreferences]);

  // Handle toggle for playSound.
  const handleTogglePlaySound = useCallback(() => {
    updatePreferences({ playSound: !preferences.playSound });
  }, [preferences.playSound, updatePreferences]);

  // Show unsupported message if notifications aren't available.
  if (!isSupported) {
    return (
      <div className={`rounded-lg border border-yellow-200 bg-yellow-50 p-4 dark:border-yellow-800 dark:bg-yellow-900/20 ${className}`}>
        <div className="flex items-center gap-2">
          <svg
            className="h-5 w-5 text-yellow-600 dark:text-yellow-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <p className="text-sm text-yellow-700 dark:text-yellow-300">
            Notifications are not supported in your browser.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className={`space-y-6 ${className}`}>
      {/* Permission status. */}
      <div className="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100">
              Browser Permission
            </h4>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {permission === 'granted' && 'Notifications are enabled.'}
              {permission === 'denied' && 'Notifications are blocked by your browser.'}
              {permission === 'default' && 'Permission has not been requested yet.'}
            </p>
          </div>

          {permission === 'default' && (
            <Button size="sm" onClick={handleRequestPermission}>
              Enable
            </Button>
          )}

          {permission === 'granted' && (
            <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900 dark:text-green-200">
              <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                  clipRule="evenodd"
                />
              </svg>
              Enabled
            </span>
          )}

          {permission === 'denied' && (
            <span className="inline-flex items-center gap-1 rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-800 dark:bg-red-900 dark:text-red-200">
              <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                  clipRule="evenodd"
                />
              </svg>
              Blocked
            </span>
          )}
        </div>

        {permission === 'denied' && (
          <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            To enable notifications, update your browser settings for this site.
          </p>
        )}

        {/* Test notification button. */}
        {permission === 'granted' && preferences.enabled && (
          <div className="mt-4 flex items-center justify-between border-t border-gray-200 pt-4 dark:border-gray-700">
            <div>
              <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                Test Notifications
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Send a test notification to verify setup.
              </p>
            </div>
            <Button size="sm" variant="outline" onClick={handleTestNotification}>
              Send Test
            </Button>
          </div>
        )}
      </div>

      {/* Preferences section. */}
      <div className="space-y-4">
        <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100">
          Notification Preferences
        </h4>

        {/* Master toggle. */}
        <ToggleSetting
          label="Enable notifications"
          description="Receive desktop notifications from Subtrate."
          checked={preferences.enabled}
          onChange={handleToggleEnabled}
          disabled={permission !== 'granted'}
        />

        {/* New message toggle. */}
        <ToggleSetting
          label="New message notifications"
          description="Get notified when you receive new messages."
          checked={preferences.showNewMessages}
          onChange={handleToggleShowNewMessages}
          disabled={permission !== 'granted' || !preferences.enabled}
        />

        {/* Sound toggle. */}
        <ToggleSetting
          label="Play sound"
          description="Play a sound when a notification is shown."
          checked={preferences.playSound}
          onChange={handleTogglePlaySound}
          disabled={permission !== 'granted' || !preferences.enabled}
        />
      </div>
    </div>
  );
}

// Props for ToggleSetting component.
interface ToggleSettingProps {
  label: string;
  description: string;
  checked: boolean;
  onChange: () => void;
  disabled?: boolean;
}

// Individual toggle setting row.
function ToggleSetting({
  label,
  description,
  checked,
  onChange,
  disabled = false,
}: ToggleSettingProps): JSX.Element {
  return (
    <div className="flex items-center justify-between">
      <div className={disabled ? 'opacity-50' : ''}>
        <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
          {label}
        </p>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          {description}
        </p>
      </div>

      <button
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={onChange}
        className={`
          relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent
          transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2
          ${checked ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}
          ${disabled ? 'cursor-not-allowed opacity-50' : ''}
        `}
      >
        <span
          className={`
            pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0
            transition duration-200 ease-in-out
            ${checked ? 'translate-x-5' : 'translate-x-0'}
          `}
        />
      </button>
    </div>
  );
}
