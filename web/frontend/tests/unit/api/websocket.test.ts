// Unit tests for WebSocket client.
// These tests use a mock WebSocket implementation that doesn't conflict with MSW.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  WebSocketClient,
  createWebSocketClient,
  getWebSocketClient,
  resetWebSocketClient,
  type WSMessage,
} from '@/api/websocket.js';
import { server } from '../../mocks/server.js';

// Disable MSW for these tests since we're testing WebSocket mocking.
beforeAll(() => {
  server.close();
});

afterAll(() => {
  server.listen();
});

// Mock WebSocket class that we can control.
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
  sentMessages: string[] = [];
  closeCalled = false;
  closeCode?: number;
  closeReason?: string;

  constructor(url: string) {
    this.url = url;
    mockWebSocketInstances.push(this);
  }

  send(data: string): void {
    this.sentMessages.push(data);
  }

  close(code?: number, reason?: string): void {
    this.closeCalled = true;
    this.closeCode = code;
    this.closeReason = reason;
    this.readyState = MockWebSocket.CLOSED;
  }

  // Helper to simulate connection open.
  simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }

  // Helper to simulate connection close.
  simulateClose(code = 1000, reason = ''): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close', { code, reason }));
  }

  // Helper to simulate receiving a message.
  simulateMessage(data: unknown): void {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }));
  }

  // Helper to simulate an error.
  simulateError(): void {
    this.onerror?.(new Event('error'));
  }
}

// Store mock instances.
let mockWebSocketInstances: MockWebSocket[] = [];

// Save original WebSocket.
const OriginalWebSocket = globalThis.WebSocket;

describe('WebSocketClient', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockWebSocketInstances = [];
    // Replace global WebSocket with mock.
    globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    vi.useRealTimers();
    // Restore original WebSocket.
    globalThis.WebSocket = OriginalWebSocket;
    resetWebSocketClient();
  });

  // Helper to get the latest mock WebSocket instance.
  function getLatestWS(): MockWebSocket {
    return mockWebSocketInstances[mockWebSocketInstances.length - 1];
  }

  describe('constructor', () => {
    it('creates client with default config', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      expect(client.getState()).toBe('disconnected');
    });

    it('creates client with custom config', () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        agentId: 123,
        reconnect: false,
        reconnectInterval: 5000,
        maxReconnectAttempts: 5,
        pingInterval: 60000,
      });
      expect(client.getState()).toBe('disconnected');
    });
  });

  describe('connect', () => {
    it('establishes WebSocket connection', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();

      expect(client.getState()).toBe('connecting');
      expect(mockWebSocketInstances).toHaveLength(1);

      // Simulate connection open.
      getLatestWS().simulateOpen();

      expect(client.getState()).toBe('connected');
    });

    it('includes agent_id in URL when provided', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws', agentId: 42 });
      client.connect();

      expect(getLatestWS().url).toContain('agent_id=42');
    });

    it('does not reconnect when already connecting', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      client.connect(); // Second call should be ignored.

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('does not reconnect when already connected', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();
      client.connect(); // Second call should be ignored.

      expect(mockWebSocketInstances).toHaveLength(1);
    });
  });

  describe('disconnect', () => {
    it('closes WebSocket connection', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();

      client.disconnect();

      expect(getLatestWS().closeCalled).toBe(true);
      expect(client.getState()).toBe('disconnected');
    });

    it('prevents reconnection after disconnect', async () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws', reconnect: true });
      client.connect();
      getLatestWS().simulateOpen();

      client.disconnect();

      // Even with reconnect enabled, should not reconnect.
      await vi.advanceTimersByTimeAsync(10000);
      expect(mockWebSocketInstances).toHaveLength(1);
    });
  });

  describe('state changes', () => {
    it('notifies listeners on state change', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const listener = vi.fn();

      client.onStateChange(listener);
      client.connect();

      expect(listener).toHaveBeenCalledWith('connecting');

      getLatestWS().simulateOpen();

      expect(listener).toHaveBeenCalledWith('connected');
    });

    it('allows unsubscribing from state changes', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const listener = vi.fn();

      const unsubscribe = client.onStateChange(listener);
      unsubscribe();

      client.connect();

      expect(listener).not.toHaveBeenCalled();
    });
  });

  describe('message handling', () => {
    it('dispatches messages to listeners', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const listener = vi.fn();

      client.on('unread_count', listener);
      client.connect();
      getLatestWS().simulateOpen();

      const message: WSMessage = { type: 'unread_count', payload: { count: 5 } };
      getLatestWS().simulateMessage(message);

      expect(listener).toHaveBeenCalledWith({ count: 5 });
    });

    it('handles multiple listeners for same type', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const listener1 = vi.fn();
      const listener2 = vi.fn();

      client.on('agent_update', listener1);
      client.on('agent_update', listener2);
      client.connect();
      getLatestWS().simulateOpen();

      const message: WSMessage = { type: 'agent_update', payload: { agents: [] } };
      getLatestWS().simulateMessage(message);

      expect(listener1).toHaveBeenCalledWith({ agents: [] });
      expect(listener2).toHaveBeenCalledWith({ agents: [] });
    });

    it('allows unsubscribing from messages', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const listener = vi.fn();

      const unsubscribe = client.on('new_message', listener);
      unsubscribe();

      client.connect();
      getLatestWS().simulateOpen();

      const message: WSMessage = { type: 'new_message', payload: {} };
      getLatestWS().simulateMessage(message);

      expect(listener).not.toHaveBeenCalled();
    });

    it('handles invalid JSON gracefully', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      client.connect();
      getLatestWS().simulateOpen();

      // Directly call onmessage with invalid JSON.
      getLatestWS().onmessage?.(new MessageEvent('message', { data: 'invalid json' }));

      expect(consoleSpy).toHaveBeenCalled();
      consoleSpy.mockRestore();
    });
  });

  describe('send', () => {
    it('sends message when connected', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();

      const result = client.send('ping');

      expect(result).toBe(true);
      expect(getLatestWS().sentMessages).toHaveLength(1);
      expect(JSON.parse(getLatestWS().sentMessages[0])).toEqual({ type: 'ping' });
    });

    it('sends message with data', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();

      const result = client.send('subscribe', { agent_id: 42 });

      expect(result).toBe(true);
      expect(JSON.parse(getLatestWS().sentMessages[0])).toEqual({
        type: 'subscribe',
        data: { agent_id: 42 },
      });
    });

    it('returns false when not connected', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

      const result = client.send('ping');

      expect(result).toBe(false);
      consoleSpy.mockRestore();
    });
  });

  describe('ping', () => {
    it('sends ping message', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();

      client.ping();

      expect(JSON.parse(getLatestWS().sentMessages[0])).toEqual({ type: 'ping' });
    });

    it('automatically pings at interval', async () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        pingInterval: 1000,
      });
      client.connect();
      getLatestWS().simulateOpen();

      // Clear any initial messages.
      getLatestWS().sentMessages = [];

      await vi.advanceTimersByTimeAsync(1000);
      expect(getLatestWS().sentMessages).toHaveLength(1);

      await vi.advanceTimersByTimeAsync(1000);
      expect(getLatestWS().sentMessages).toHaveLength(2);
    });
  });

  describe('subscribeToAgent', () => {
    it('sends subscribe message with agent_id', () => {
      const client = new WebSocketClient({ url: 'ws://localhost/ws' });
      client.connect();
      getLatestWS().simulateOpen();

      client.subscribeToAgent(123);

      expect(JSON.parse(getLatestWS().sentMessages[0])).toEqual({
        type: 'subscribe',
        data: { agent_id: 123 },
      });
    });
  });

  describe('reconnection', () => {
    it('reconnects on disconnect when enabled', async () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        reconnect: true,
        reconnectInterval: 1000,
      });
      client.connect();
      getLatestWS().simulateOpen();

      // Simulate disconnection.
      getLatestWS().simulateClose();

      expect(client.getState()).toBe('reconnecting');

      // Advance time for reconnection.
      await vi.advanceTimersByTimeAsync(2000);

      expect(mockWebSocketInstances).toHaveLength(2);
    });

    it('does not reconnect when disabled', async () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        reconnect: false,
      });
      client.connect();
      getLatestWS().simulateOpen();

      // Simulate disconnection.
      getLatestWS().simulateClose();

      expect(client.getState()).toBe('disconnected');

      // Advance time.
      await vi.advanceTimersByTimeAsync(10000);

      expect(mockWebSocketInstances).toHaveLength(1);
    });

    it('stops reconnecting after max attempts', async () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        reconnect: true,
        reconnectInterval: 100,
        maxReconnectAttempts: 2,
      });
      client.connect();
      getLatestWS().simulateOpen();

      // Fail first connection.
      getLatestWS().simulateClose();
      await vi.advanceTimersByTimeAsync(200);

      // Fail second attempt.
      getLatestWS().simulateClose();
      await vi.advanceTimersByTimeAsync(400);

      // Third attempt.
      getLatestWS().simulateClose();
      await vi.advanceTimersByTimeAsync(800);

      // Should give up after max attempts.
      expect(client.getState()).toBe('disconnected');
    });

    it('uses exponential backoff', async () => {
      const client = new WebSocketClient({
        url: 'ws://localhost/ws',
        reconnect: true,
        reconnectInterval: 1000,
        maxReconnectAttempts: 5,
      });
      client.connect();
      getLatestWS().simulateOpen();

      // First disconnect.
      getLatestWS().simulateClose();
      expect(client.getState()).toBe('reconnecting');

      // First reconnect should be ~1000ms (base delay).
      await vi.advanceTimersByTimeAsync(900);
      expect(mockWebSocketInstances).toHaveLength(1);

      // After full delay it should reconnect.
      await vi.advanceTimersByTimeAsync(200);
      expect(mockWebSocketInstances).toHaveLength(2);
    });
  });
});

describe('createWebSocketClient', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    resetWebSocketClient();
  });

  it('creates client instance', () => {
    const client = createWebSocketClient();
    expect(client).toBeInstanceOf(WebSocketClient);
  });

  it('creates client with agent ID', () => {
    const client = createWebSocketClient(42);
    expect(client).toBeInstanceOf(WebSocketClient);
  });
});

describe('getWebSocketClient', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    resetWebSocketClient();
  });

  it('returns singleton instance', () => {
    const client1 = getWebSocketClient();
    const client2 = getWebSocketClient();

    expect(client1).toBe(client2);
  });
});

describe('resetWebSocketClient', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('creates new instance after reset', () => {
    const client1 = getWebSocketClient();
    resetWebSocketClient();
    const client2 = getWebSocketClient();

    expect(client1).not.toBe(client2);
  });
});
