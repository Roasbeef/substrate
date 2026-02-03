// Unit tests for useMessagesRealtime hook.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import {
  useMessagesRealtime,
  useRealtimeUnreadCount,
} from '@/hooks/useMessagesRealtime.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import type { ReactNode } from 'react';

// Disable MSW for these tests.
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

// Query client for testing.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
}

// Wrapper with QueryClientProvider.
function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe('useMessagesRealtime', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    mockWebSocketInstances = [];
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient = createTestQueryClient();
  });

  afterEach(() => {
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient.clear();
  });

  function getLatestWS(): MockWebSocket {
    return mockWebSocketInstances[mockWebSocketInstances.length - 1];
  }

  describe('useMessagesRealtime', () => {
    it('returns initial state', () => {
      const { result } = renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.unreadCount).toBe(0);
      expect(result.current.isConnected).toBe(false);
      expect(typeof result.current.connect).toBe('function');
      expect(typeof result.current.disconnect).toBe('function');
    });

    it('auto-connects WebSocket', () => {
      renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('updates isConnected when WebSocket connects', async () => {
      const { result } = renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current.isConnected).toBe(false);

      act(() => {
        getLatestWS().simulateOpen();
      });

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });
    });

    it('updates unread count when message received', async () => {
      const { result } = renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      act(() => {
        getLatestWS().simulateMessage({
          type: 'unread_count',
          payload: { count: 5 },
        });
      });

      await waitFor(() => {
        expect(result.current.unreadCount).toBe(5);
      });
    });

    it('handles new_message event', async () => {
      // Pre-populate the cache with initial messages.
      queryClient.setQueryData(['messages', 'list', {}], {
        data: [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Agent1',
            subject: 'Existing message',
            body: 'Hello',
            priority: 'normal',
            created_at: '2024-01-01T00:00:00Z',
            recipients: [
              {
                message_id: 1,
                agent_id: 2,
                agent_name: 'User',
                state: 'read',
                is_starred: false,
                is_archived: false,
              },
            ],
          },
        ],
      });

      const { result } = renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      // Simulate receiving a new message.
      act(() => {
        getLatestWS().simulateMessage({
          type: 'new_message',
          payload: {
            id: 2,
            sender_id: 3,
            sender_name: 'Agent3',
            subject: 'New message',
            body: 'Hi there!',
            priority: 'normal',
            created_at: '2024-01-01T01:00:00Z',
          },
        });
      });

      // Check that the cache was updated with the new message.
      await waitFor(() => {
        const cachedData = queryClient.getQueryData(['messages', 'list', {}]) as {
          data: Array<{ id: number }>;
        };
        expect(cachedData?.data).toHaveLength(2);
        expect(cachedData?.data[0]?.id).toBe(2); // New message at the front.
      });
    });

    it('does not duplicate messages in cache', async () => {
      // Pre-populate the cache with initial messages.
      queryClient.setQueryData(['messages', 'list', {}], {
        data: [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Agent1',
            subject: 'Existing message',
            body: 'Hello',
            priority: 'normal',
            created_at: '2024-01-01T00:00:00Z',
            recipients: [],
          },
        ],
      });

      renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      // Send the same message twice.
      const messagePayload = {
        type: 'new_message',
        payload: {
          id: 2,
          sender_id: 3,
          sender_name: 'Agent3',
          subject: 'New message',
          body: 'Hi there!',
          priority: 'normal',
          created_at: '2024-01-01T01:00:00Z',
        },
      };

      act(() => {
        getLatestWS().simulateMessage(messagePayload);
      });

      act(() => {
        getLatestWS().simulateMessage(messagePayload);
      });

      // Should still only have 2 messages (1 existing + 1 new).
      await waitFor(() => {
        const cachedData = queryClient.getQueryData(['messages', 'list', {}]) as {
          data: Array<{ id: number }>;
        };
        expect(cachedData?.data).toHaveLength(2);
      });
    });

    it('provides connect and disconnect functions', async () => {
      const { result } = renderHook(() => useMessagesRealtime(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      await waitFor(() => {
        expect(result.current.isConnected).toBe(true);
      });

      act(() => {
        result.current.disconnect();
      });

      await waitFor(() => {
        expect(result.current.isConnected).toBe(false);
      });
    });
  });

  describe('useRealtimeUnreadCount', () => {
    it('returns initial count of 0', () => {
      const { result } = renderHook(() => useRealtimeUnreadCount(), {
        wrapper: createWrapper(queryClient),
      });

      expect(result.current).toBe(0);
    });

    it('auto-connects WebSocket', () => {
      renderHook(() => useRealtimeUnreadCount(), {
        wrapper: createWrapper(queryClient),
      });

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('updates when unread_count message received', async () => {
      const { result } = renderHook(() => useRealtimeUnreadCount(), {
        wrapper: createWrapper(queryClient),
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      act(() => {
        getLatestWS().simulateMessage({
          type: 'unread_count',
          payload: { count: 10 },
        });
      });

      await waitFor(() => {
        expect(result.current).toBe(10);
      });
    });
  });
});
