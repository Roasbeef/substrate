// Unit tests for useNotifications hook.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useNotifications, useNewMessageNotifications, type NotificationPreferences } from '@/hooks/useNotifications.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import { server } from '../../mocks/server.js';

// Disable MSW for WebSocket tests.
beforeAll(() => {
  server.close();
});

afterAll(() => {
  server.listen();
});

// Mock WebSocket class.
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url: string;
  readyState: number = MockWebSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    mockWebSocketInstances.push(this);
  }

  send(): void {}

  close(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close', { code: 1000 }));
  }

  simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }

  simulateMessage(data: unknown): void {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }));
  }
}

let mockWebSocketInstances: MockWebSocket[] = [];
const OriginalWebSocket = globalThis.WebSocket;

// Mock Notification API.
const mockNotification = vi.fn();
const mockRequestPermission = vi.fn();

const originalNotification = globalThis.Notification;
const originalLocalStorage = globalThis.localStorage;

let mockLocalStorage: Record<string, string>;

// Common setup for notification tests.
function setupNotificationMocks() {
  vi.clearAllMocks();
  mockWebSocketInstances = [];

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
    mockNotification,
    {
      permission: 'default' as NotificationPermission,
      requestPermission: mockRequestPermission,
    },
  );
}

function cleanupNotificationMocks() {
  (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
  Object.defineProperty(globalThis, 'localStorage', {
    value: originalLocalStorage,
    writable: true,
  });
}

describe('useNotifications', () => {
  beforeEach(() => {
    setupNotificationMocks();
  });

  afterEach(() => {
    cleanupNotificationMocks();
  });

  describe('initial state', () => {
    it('returns correct initial state', () => {
      const { result } = renderHook(() => useNotifications());

      expect(result.current.isSupported).toBe(true);
      expect(result.current.permission).toBe('default');
      expect(result.current.preferences.enabled).toBe(true);
      expect(result.current.preferences.showNewMessages).toBe(true);
      expect(result.current.preferences.playSound).toBe(false);
    });

    it('detects when notifications are not supported', () => {
      delete (globalThis as unknown as Record<string, unknown>).Notification;

      const { result } = renderHook(() => useNotifications());

      expect(result.current.isSupported).toBe(false);
    });

    it('loads saved preferences from localStorage', () => {
      const savedPrefs: NotificationPreferences = {
        enabled: false,
        showNewMessages: false,
        playSound: true,
      };
      mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify(savedPrefs);

      const { result } = renderHook(() => useNotifications());

      expect(result.current.preferences.enabled).toBe(false);
      expect(result.current.preferences.showNewMessages).toBe(false);
      expect(result.current.preferences.playSound).toBe(true);
    });
  });

  describe('requestPermission', () => {
    it('requests notification permission', async () => {
      const { result } = renderHook(() => useNotifications());

      let permission: string | undefined;
      await act(async () => {
        permission = await result.current.requestPermission();
      });

      expect(mockRequestPermission).toHaveBeenCalled();
      expect(permission).toBe('granted');
      expect(result.current.permission).toBe('granted');
    });

    it('returns denied when permission denied', async () => {
      mockRequestPermission.mockResolvedValue('denied');

      const { result } = renderHook(() => useNotifications());

      let permission: string | undefined;
      await act(async () => {
        permission = await result.current.requestPermission();
      });

      expect(permission).toBe('denied');
      expect(result.current.permission).toBe('denied');
    });

    it('returns denied when not supported', async () => {
      delete (globalThis as unknown as Record<string, unknown>).Notification;

      const { result } = renderHook(() => useNotifications());

      let permission: string | undefined;
      await act(async () => {
        permission = await result.current.requestPermission();
      });

      expect(permission).toBe('denied');
    });

    it('handles errors gracefully', async () => {
      mockRequestPermission.mockRejectedValue(new Error('Permission error'));

      const { result } = renderHook(() => useNotifications());

      let permission: string | undefined;
      await act(async () => {
        permission = await result.current.requestPermission();
      });

      expect(permission).toBe('denied');
    });
  });

  describe('updatePreferences', () => {
    it('updates preferences', () => {
      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.updatePreferences({ enabled: false });
      });

      expect(result.current.preferences.enabled).toBe(false);
      expect(result.current.preferences.showNewMessages).toBe(true); // Unchanged.
    });

    it('saves preferences to localStorage', () => {
      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.updatePreferences({ playSound: true });
      });

      const saved = JSON.parse(
        mockLocalStorage['subtrate_notification_preferences'] ?? '{}',
      );
      expect(saved.playSound).toBe(true);
    });

    it('can update multiple preferences at once', () => {
      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.updatePreferences({
          enabled: false,
          showNewMessages: false,
          playSound: true,
        });
      });

      expect(result.current.preferences.enabled).toBe(false);
      expect(result.current.preferences.showNewMessages).toBe(false);
      expect(result.current.preferences.playSound).toBe(true);
    });
  });

  describe('showNotification', () => {
    it('shows notification when permission granted and enabled', () => {
      // Set permission to granted.
      (globalThis.Notification as unknown as { permission: string }).permission = 'granted';

      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.showNotification('Test Title', { body: 'Test body' });
      });

      expect(mockNotification).toHaveBeenCalledWith('Test Title', expect.objectContaining({
        body: 'Test body',
      }));
    });

    it('does not show notification when permission not granted', () => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'denied';

      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.showNotification('Test Title');
      });

      expect(mockNotification).not.toHaveBeenCalled();
    });

    it('does not show notification when disabled in preferences', () => {
      (globalThis.Notification as unknown as { permission: string }).permission = 'granted';
      mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify({
        enabled: false,
        showNewMessages: true,
        playSound: false,
      });

      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.showNotification('Test Title');
      });

      expect(mockNotification).not.toHaveBeenCalled();
    });

    it('does not show notification when not supported', () => {
      delete (globalThis as unknown as Record<string, unknown>).Notification;

      const { result } = renderHook(() => useNotifications());

      act(() => {
        result.current.showNotification('Test Title');
      });

      expect(mockNotification).not.toHaveBeenCalled();
    });
  });
});

describe('useNewMessageNotifications', () => {
  function getLatestWS(): MockWebSocket {
    return mockWebSocketInstances[mockWebSocketInstances.length - 1];
  }

  beforeEach(() => {
    setupNotificationMocks();
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
  });

  afterEach(() => {
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    cleanupNotificationMocks();
  });

  it('shows notification when new message received', async () => {
    // Set permission to granted.
    (globalThis.Notification as unknown as { permission: string }).permission = 'granted';

    renderHook(() => useNewMessageNotifications());

    // Simulate WebSocket connection.
    await act(async () => {
      getLatestWS().simulateOpen();
    });

    // Simulate receiving a new message.
    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Test Subject',
          preview: 'Test preview',
          priority: 'normal',
          created_at: new Date().toISOString(),
        },
      });
    });

    await waitFor(() => {
      expect(mockNotification).toHaveBeenCalledWith(
        'New message from TestAgent',
        expect.objectContaining({
          body: 'Test Subject',
          tag: 'message-1',
        }),
      );
    });
  });

  it('does not show notification when not connected', async () => {
    // Set permission to granted.
    (globalThis.Notification as unknown as { permission: string }).permission = 'granted';

    renderHook(() => useNewMessageNotifications());

    // Don't simulate open - stay disconnected.

    // Try to simulate a message (won't work because not connected).
    act(() => {
      const ws = getLatestWS();
      if (ws?.onmessage) {
        ws.onmessage(new MessageEvent('message', {
          data: JSON.stringify({
            type: 'new_message',
            payload: {
              id: 1,
              sender_name: 'TestAgent',
              subject: 'Test Subject',
              preview: 'Test preview',
              priority: 'normal',
              created_at: new Date().toISOString(),
            },
          }),
        }));
      }
    });

    // Wait a bit to ensure no notification was triggered.
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockNotification).not.toHaveBeenCalled();
  });

  it('does not show notification when permission not granted', async () => {
    // Permission is default (not granted).
    renderHook(() => useNewMessageNotifications());

    await act(async () => {
      getLatestWS().simulateOpen();
    });

    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Test Subject',
          preview: 'Test preview',
          priority: 'normal',
          created_at: new Date().toISOString(),
        },
      });
    });

    // Wait a bit.
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockNotification).not.toHaveBeenCalled();
  });

  it('does not show notification when notifications disabled in preferences', async () => {
    // Set permission to granted but disable in preferences.
    (globalThis.Notification as unknown as { permission: string }).permission = 'granted';
    mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify({
      enabled: false,
      showNewMessages: true,
      playSound: false,
    });

    renderHook(() => useNewMessageNotifications());

    await act(async () => {
      getLatestWS().simulateOpen();
    });

    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Test Subject',
          preview: 'Test preview',
          priority: 'normal',
          created_at: new Date().toISOString(),
        },
      });
    });

    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockNotification).not.toHaveBeenCalled();
  });

  it('does not show notification when showNewMessages disabled', async () => {
    // Set permission to granted but disable showNewMessages.
    (globalThis.Notification as unknown as { permission: string }).permission = 'granted';
    mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify({
      enabled: true,
      showNewMessages: false,
      playSound: false,
    });

    renderHook(() => useNewMessageNotifications());

    await act(async () => {
      getLatestWS().simulateOpen();
    });

    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Test Subject',
          preview: 'Test preview',
          priority: 'normal',
          created_at: new Date().toISOString(),
        },
      });
    });

    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockNotification).not.toHaveBeenCalled();
  });
});
