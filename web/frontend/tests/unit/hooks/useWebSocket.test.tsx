// Unit tests for useWebSocket hooks.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { server } from '../../mocks/server.js';
import {
  useConnectionState,
  useWebSocketClient,
  useWebSocketConnection,
  useWebSocketMessage,
  useUnreadCount,
  useAgentUpdates,
  useActivityUpdates,
  useNewMessages,
  useWebSocket,
  resetWebSocketHookState,
} from '@/hooks/useWebSocket.js';
import { resetWebSocketClient } from '@/api/websocket.js';

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

describe('useWebSocket hooks', () => {
  beforeEach(() => {
    mockWebSocketInstances = [];
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
  });

  afterEach(() => {
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
    resetWebSocketHookState();
  });

  function getLatestWS(): MockWebSocket {
    return mockWebSocketInstances[mockWebSocketInstances.length - 1];
  }

  describe('useConnectionState', () => {
    it('returns initial disconnected state', () => {
      const { result } = renderHook(() => useConnectionState());
      expect(result.current).toBe('disconnected');
    });

    it('updates when connection state changes', () => {
      const { result } = renderHook(() => useConnectionState());
      const { result: connectionResult } = renderHook(() => useWebSocketConnection());

      act(() => {
        connectionResult.current.connect();
      });

      expect(result.current).toBe('connecting');

      act(() => {
        getLatestWS().simulateOpen();
      });

      expect(result.current).toBe('connected');
    });
  });

  describe('useWebSocketClient', () => {
    it('returns WebSocket client instance', () => {
      const { result } = renderHook(() => useWebSocketClient());
      expect(result.current).toBeDefined();
      expect(typeof result.current.connect).toBe('function');
      expect(typeof result.current.disconnect).toBe('function');
    });

    it('returns same instance across renders', () => {
      const { result, rerender } = renderHook(() => useWebSocketClient());
      const firstClient = result.current;

      rerender();

      expect(result.current).toBe(firstClient);
    });
  });

  describe('useWebSocketConnection', () => {
    it('provides connect and disconnect functions', () => {
      const { result } = renderHook(() => useWebSocketConnection(false));

      expect(typeof result.current.connect).toBe('function');
      expect(typeof result.current.disconnect).toBe('function');
    });

    it('auto-connects when enabled', () => {
      renderHook(() => useWebSocketConnection(true));

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('does not auto-connect when disabled', () => {
      renderHook(() => useWebSocketConnection(false));

      expect(mockWebSocketInstances).toHaveLength(0);
    });

    it('provides current connection state', () => {
      const { result } = renderHook(() => useWebSocketConnection(false));

      expect(result.current.state).toBe('disconnected');

      act(() => {
        result.current.connect();
      });

      expect(result.current.state).toBe('connecting');
    });

    it('disconnects on manual call', () => {
      const { result } = renderHook(() => useWebSocketConnection(false));

      act(() => {
        result.current.connect();
      });

      act(() => {
        getLatestWS().simulateOpen();
      });

      expect(result.current.state).toBe('connected');

      act(() => {
        result.current.disconnect();
      });

      expect(result.current.state).toBe('disconnected');
    });
  });

  describe('useWebSocketMessage', () => {
    it('calls handler when message received', () => {
      const handler = vi.fn();
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));

      renderHook(() => useWebSocketMessage('unread_count', handler));

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      act(() => {
        getLatestWS().simulateMessage({ type: 'unread_count', payload: { count: 5 } });
      });

      expect(handler).toHaveBeenCalledWith({ count: 5 });
    });

    it('unsubscribes on unmount', () => {
      const handler = vi.fn();
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));

      const { unmount } = renderHook(() => useWebSocketMessage('unread_count', handler));

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      unmount();

      act(() => {
        getLatestWS().simulateMessage({ type: 'unread_count', payload: { count: 5 } });
      });

      expect(handler).not.toHaveBeenCalled();
    });
  });

  describe('useUnreadCount', () => {
    it('returns initial count of 0', () => {
      const { result } = renderHook(() => useUnreadCount());
      expect(result.current).toBe(0);
    });

    it('updates when unread_count message received', () => {
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));
      const { result } = renderHook(() => useUnreadCount());

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      act(() => {
        getLatestWS().simulateMessage({ type: 'unread_count', payload: { count: 10 } });
      });

      expect(result.current).toBe(10);
    });
  });

  describe('useAgentUpdates', () => {
    it('calls handler with agent updates', () => {
      const handler = vi.fn();
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));

      renderHook(() => useAgentUpdates(handler));

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      const payload = {
        agents: [{ id: 1, name: 'Agent1', status: 'active', last_active_at: '', seconds_since_heartbeat: 0 }],
        counts: { active: 1, busy: 0, idle: 0, offline: 0 },
      };

      act(() => {
        getLatestWS().simulateMessage({ type: 'agent_update', payload });
      });

      expect(handler).toHaveBeenCalledWith(payload);
    });
  });

  describe('useActivityUpdates', () => {
    it('calls handler with activity updates', () => {
      const handler = vi.fn();
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));

      renderHook(() => useActivityUpdates(handler));

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      const payload = [
        { id: 1, agent_id: 1, agent_name: 'Agent1', type: 'message_sent', description: 'Sent a message', created_at: '' },
      ];

      act(() => {
        getLatestWS().simulateMessage({ type: 'activity', payload });
      });

      expect(handler).toHaveBeenCalledWith(payload);
    });
  });

  describe('useNewMessages', () => {
    it('calls handler with new message', () => {
      const handler = vi.fn();
      const { result: connectionResult } = renderHook(() => useWebSocketConnection(false));

      renderHook(() => useNewMessages(handler));

      act(() => {
        connectionResult.current.connect();
        getLatestWS().simulateOpen();
      });

      const payload = {
        id: 1,
        sender_id: 1,
        sender_name: 'Agent1',
        subject: 'Test',
        body: 'Hello',
        priority: 'normal',
        created_at: '',
      };

      act(() => {
        getLatestWS().simulateMessage({ type: 'new_message', payload });
      });

      expect(handler).toHaveBeenCalledWith(payload);
    });
  });

  describe('useWebSocket', () => {
    it('returns all WebSocket state and methods', () => {
      const { result } = renderHook(() => useWebSocket({ autoConnect: false }));

      expect(result.current.state).toBe('disconnected');
      expect(result.current.unreadCount).toBe(0);
      expect(typeof result.current.connect).toBe('function');
      expect(typeof result.current.disconnect).toBe('function');
      expect(result.current.client).toBeDefined();
    });

    it('auto-connects by default', () => {
      renderHook(() => useWebSocket());

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('does not auto-connect when disabled', () => {
      renderHook(() => useWebSocket({ autoConnect: false }));

      expect(mockWebSocketInstances).toHaveLength(0);
    });

    it('updates unread count when message received', () => {
      const { result } = renderHook(() => useWebSocket({ autoConnect: false }));

      act(() => {
        result.current.connect();
        getLatestWS().simulateOpen();
      });

      act(() => {
        getLatestWS().simulateMessage({ type: 'unread_count', payload: { count: 7 } });
      });

      expect(result.current.unreadCount).toBe(7);
    });
  });
});
