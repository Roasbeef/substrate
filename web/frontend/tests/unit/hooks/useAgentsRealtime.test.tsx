// Unit tests for useAgentsRealtime hook.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { useAgentsRealtime } from '@/hooks/useAgentsRealtime.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import { agentKeys } from '@/hooks/useAgents.js';
import type { ReactNode } from 'react';
import type { AgentsStatusResponse } from '@/types/api.js';

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

describe('useAgentsRealtime', () => {
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
    const { result } = renderHook(() => useAgentsRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    expect(result.current.isConnected).toBe(false);
    expect(typeof result.current.connect).toBe('function');
    expect(typeof result.current.disconnect).toBe('function');
  });

  it('auto-connects WebSocket', () => {
    renderHook(() => useAgentsRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    expect(mockWebSocketInstances).toHaveLength(1);
  });

  it('updates isConnected when WebSocket connects', async () => {
    const { result } = renderHook(() => useAgentsRealtime(), {
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

  it('updates agent status cache when agent_update message received', async () => {
    // Pre-populate the cache with initial agents.
    const initialData: AgentsStatusResponse = {
      agents: [
        {
          id: 1,
          name: 'Agent1',
          status: 'active',
          last_active_at: '2024-01-01T00:00:00Z',
          seconds_since_heartbeat: 10,
        },
      ],
      counts: {
        active: 1,
        busy: 0,
        idle: 0,
        offline: 0,
      },
    };
    queryClient.setQueryData(agentKeys.status(), initialData);

    renderHook(() => useAgentsRealtime(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      getLatestWS().simulateOpen();
    });

    // Simulate receiving an agent update.
    const updatePayload = {
      agents: [
        {
          id: 1,
          name: 'Agent1',
          status: 'busy',
          last_active_at: '2024-01-01T01:00:00Z',
          seconds_since_heartbeat: 5,
        },
        {
          id: 2,
          name: 'Agent2',
          status: 'active',
          last_active_at: '2024-01-01T01:00:00Z',
          seconds_since_heartbeat: 0,
        },
      ],
      counts: {
        active: 1,
        busy: 1,
        idle: 0,
        offline: 0,
      },
    };

    act(() => {
      getLatestWS().simulateMessage({
        type: 'agent_update',
        payload: updatePayload,
      });
    });

    // Check that the cache was updated.
    await waitFor(() => {
      const cachedData = queryClient.getQueryData<AgentsStatusResponse>(
        agentKeys.status(),
      );
      expect(cachedData?.agents).toHaveLength(2);
      expect(cachedData?.agents[0]?.status).toBe('busy');
      expect(cachedData?.counts.busy).toBe(1);
    });
  });

  it('provides connect and disconnect functions', async () => {
    const { result } = renderHook(() => useAgentsRealtime(), {
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
