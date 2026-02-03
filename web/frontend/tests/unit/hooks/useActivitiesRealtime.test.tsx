// Unit tests for useActivitiesRealtime hook.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { useActivitiesRealtime } from '@/hooks/useActivitiesRealtime.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import { activityKeys } from '@/hooks/useActivities.js';
import type { ReactNode } from 'react';
import type { Activity, APIResponse } from '@/types/api.js';

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

describe('useActivitiesRealtime', () => {
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

  it('returns initial state', () => {
    const { result } = renderHook(() => useActivitiesRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    expect(result.current.isConnected).toBe(false);
    expect(typeof result.current.connect).toBe('function');
    expect(typeof result.current.disconnect).toBe('function');
  });

  it('auto-connects WebSocket', () => {
    renderHook(() => useActivitiesRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    expect(mockWebSocketInstances).toHaveLength(1);
  });

  it('updates isConnected when WebSocket connects', async () => {
    const { result } = renderHook(() => useActivitiesRealtime(), {
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

  it('updates activity cache when activity message received', async () => {
    // Pre-populate the cache with initial activities.
    const initialActivities: Activity[] = [
      {
        id: 1,
        agent_id: 1,
        agent_name: 'Agent1',
        type: 'message_sent',
        description: 'Sent a message',
        created_at: '2024-01-01T00:00:00Z',
      },
    ];
    queryClient.setQueryData(activityKeys.list({}), {
      data: initialActivities,
      meta: { total: 1, page: 1, page_size: 20 },
    });

    renderHook(() => useActivitiesRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      getLatestWS().simulateOpen();
    });

    // Simulate receiving new activities.
    const newActivities = [
      {
        id: 2,
        agent_id: 2,
        agent_name: 'Agent2',
        type: 'session_started',
        description: 'Started a new session',
        created_at: '2024-01-01T01:00:00Z',
      },
    ];

    act(() => {
      getLatestWS().simulateMessage({
        type: 'activity',
        payload: newActivities,
      });
    });

    // Check that the cache was updated.
    await waitFor(() => {
      const cachedData = queryClient.getQueryData<APIResponse<Activity[]>>(
        activityKeys.list({}),
      );
      expect(cachedData?.data).toHaveLength(2);
      expect(cachedData?.data[0]?.id).toBe(2); // New activity at the front.
    });
  });

  it('does not duplicate activities in cache', async () => {
    // Pre-populate the cache with initial activities.
    const initialActivities: Activity[] = [
      {
        id: 1,
        agent_id: 1,
        agent_name: 'Agent1',
        type: 'message_sent',
        description: 'Sent a message',
        created_at: '2024-01-01T00:00:00Z',
      },
    ];
    queryClient.setQueryData(activityKeys.list({}), {
      data: initialActivities,
      meta: { total: 1, page: 1, page_size: 20 },
    });

    renderHook(() => useActivitiesRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      getLatestWS().simulateOpen();
    });

    // Send the same activity twice.
    const newActivity = [
      {
        id: 2,
        agent_id: 2,
        agent_name: 'Agent2',
        type: 'session_started',
        description: 'Started a new session',
        created_at: '2024-01-01T01:00:00Z',
      },
    ];

    act(() => {
      getLatestWS().simulateMessage({
        type: 'activity',
        payload: newActivity,
      });
    });

    act(() => {
      getLatestWS().simulateMessage({
        type: 'activity',
        payload: newActivity,
      });
    });

    // Should still only have 2 activities (1 initial + 1 new).
    await waitFor(() => {
      const cachedData = queryClient.getQueryData<APIResponse<Activity[]>>(
        activityKeys.list({}),
      );
      expect(cachedData?.data).toHaveLength(2);
    });
  });

  it('provides connect and disconnect functions', async () => {
    const { result } = renderHook(() => useActivitiesRealtime(), {
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
