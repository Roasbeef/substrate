// Integration tests for notification display with real-time updates.

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { NotificationPrompt, resetPromptDismissed } from '@/components/layout/NotificationPrompt.js';
import { NotificationSettings } from '@/components/layout/NotificationSettings.js';
import { useNewMessageNotifications } from '@/hooks/useNotifications.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import type { ReactNode } from 'react';

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
const mockNotification = vi.fn();
const mockRequestPermission = vi.fn();
const originalNotification = globalThis.Notification;
const originalLocalStorage = globalThis.localStorage;
let mockLocalStorage: Record<string, string>;

// Disable MSW for WebSocket tests.
beforeAll(() => {
  server.close();
});

afterAll(() => {
  server.listen();
});

// Create a test query client.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
        staleTime: 0,
      },
    },
  });
}

// Wrapper with notification hook.
function NotificationWrapper({
  children,
  queryClient,
}: {
  children: ReactNode;
  queryClient: QueryClient;
}) {
  useNewMessageNotifications();
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe('NotificationPrompt integration', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    mockWebSocketInstances = [];
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient = createTestQueryClient();

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
  });

  afterEach(() => {
    vi.useRealTimers();
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient.clear();
    (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
    });
    resetPromptDismissed();
  });

  it('shows prompt after delay', async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <NotificationPrompt />
      </QueryClientProvider>,
    );

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  it('enables notifications when user clicks Enable', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(
      <QueryClientProvider client={queryClient}>
        <NotificationPrompt />
      </QueryClientProvider>,
    );

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Enable' }));

    expect(mockRequestPermission).toHaveBeenCalled();

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('dismisses prompt and persists dismissal', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    render(
      <QueryClientProvider client={queryClient}>
        <NotificationPrompt />
      </QueryClientProvider>,
    );

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    await user.click(screen.getByRole('button', { name: 'Not now' }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    expect(mockLocalStorage['subtrate_notification_prompt_dismissed']).toBe('true');
  });
});

describe('NotificationSettings integration', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    mockWebSocketInstances = [];
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient = createTestQueryClient();

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

    mockRequestPermission.mockResolvedValue('granted');
    (globalThis as unknown as { Notification: unknown }).Notification = Object.assign(
      mockNotification,
      {
        permission: 'granted' as NotificationPermission,
        requestPermission: mockRequestPermission,
      },
    );
  });

  afterEach(() => {
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient.clear();
    (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
    });
  });

  it('shows enabled status when permission granted', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <NotificationSettings />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Notifications are enabled.')).toBeInTheDocument();
    expect(screen.getByText('Enabled')).toBeInTheDocument();
  });

  it('allows toggling notification preferences', async () => {
    const user = userEvent.setup();

    render(
      <QueryClientProvider client={queryClient}>
        <NotificationSettings />
      </QueryClientProvider>,
    );

    const toggles = screen.getAllByRole('switch');
    const enableToggle = toggles[0];

    expect(enableToggle).toHaveAttribute('aria-checked', 'true');

    await user.click(enableToggle!);

    await waitFor(() => {
      expect(enableToggle).toHaveAttribute('aria-checked', 'false');
    });

    const saved = JSON.parse(mockLocalStorage['subtrate_notification_preferences'] ?? '{}');
    expect(saved.enabled).toBe(false);
  });

  it('disables child toggles when master toggle is off', async () => {
    const user = userEvent.setup();

    render(
      <QueryClientProvider client={queryClient}>
        <NotificationSettings />
      </QueryClientProvider>,
    );

    const toggles = screen.getAllByRole('switch');
    const enableToggle = toggles[0];
    const messageToggle = toggles[1];
    const soundToggle = toggles[2];

    await user.click(enableToggle!);

    await waitFor(() => {
      expect(messageToggle).toBeDisabled();
      expect(soundToggle).toBeDisabled();
    });
  });
});

describe('Notification display on new message', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    mockWebSocketInstances = [];
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient = createTestQueryClient();

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

    mockRequestPermission.mockResolvedValue('granted');
    (globalThis as unknown as { Notification: unknown }).Notification = Object.assign(
      mockNotification,
      {
        permission: 'granted' as NotificationPermission,
        requestPermission: mockRequestPermission,
      },
    );
  });

  afterEach(() => {
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient.clear();
    (globalThis as unknown as { Notification: unknown }).Notification = originalNotification;
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
    });
  });

  function getLatestWS(): MockWebSocket {
    return mockWebSocketInstances[mockWebSocketInstances.length - 1];
  }

  it('shows notification when new message arrives and conditions are met', async () => {
    render(
      <NotificationWrapper queryClient={queryClient}>
        <div>App content</div>
      </NotificationWrapper>,
    );

    await act(async () => {
      getLatestWS().simulateOpen();
    });

    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Important Message',
          preview: 'Message preview',
          priority: 'urgent',
          created_at: new Date().toISOString(),
        },
      });
    });

    await waitFor(() => {
      expect(mockNotification).toHaveBeenCalledWith(
        'New message from TestAgent',
        expect.objectContaining({
          body: 'Important Message',
          tag: 'message-1',
        }),
      );
    });
  });

  it('does not show notification when disabled in preferences', async () => {
    mockLocalStorage['subtrate_notification_preferences'] = JSON.stringify({
      enabled: false,
      showNewMessages: true,
      playSound: false,
    });

    render(
      <NotificationWrapper queryClient={queryClient}>
        <div>App content</div>
      </NotificationWrapper>,
    );

    await act(async () => {
      getLatestWS().simulateOpen();
    });

    await act(async () => {
      getLatestWS().simulateMessage({
        type: 'new_message',
        payload: {
          id: 1,
          sender_name: 'TestAgent',
          subject: 'Important Message',
          preview: 'Preview',
          priority: 'normal',
          created_at: new Date().toISOString(),
        },
      });
    });

    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(mockNotification).not.toHaveBeenCalled();
  });
});
