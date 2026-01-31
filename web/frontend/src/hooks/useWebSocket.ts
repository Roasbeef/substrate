// React hook for WebSocket integration with the server.

import { useEffect, useCallback, useRef, useSyncExternalStore } from 'react';
import {
  WebSocketClient,
  getWebSocketClient,
  type ConnectionState,
  type WSMessageType,
  type WSEventListener,
} from '@/api/websocket.js';
import { useAuthStore } from '@/stores/auth.js';

// Re-export types for convenience.
export type { ConnectionState, WSMessageType, WSEventListener };

// WebSocket connection state store for React.
let connectionState: ConnectionState = 'disconnected';
const stateListeners = new Set<() => void>();

// Subscribe to state changes.
function subscribeToState(callback: () => void) {
  stateListeners.add(callback);
  return () => stateListeners.delete(callback);
}

// Get current state.
function getState() {
  return connectionState;
}

// Update state and notify listeners.
function updateState(state: ConnectionState) {
  connectionState = state;
  stateListeners.forEach((listener) => listener());
}

// Initialize state listener on the WebSocket client.
let clientInitialized = false;
function ensureClientInitialized(agentId?: number): WebSocketClient {
  const client = getWebSocketClient(agentId);

  if (!clientInitialized) {
    client.onStateChange(updateState);
    clientInitialized = true;
  }

  return client;
}

// Hook to access WebSocket connection state.
export function useConnectionState(): ConnectionState {
  return useSyncExternalStore(subscribeToState, getState, getState);
}

// Hook to get the WebSocket client instance.
export function useWebSocketClient(): WebSocketClient {
  const currentAgent = useAuthStore((state) => state.currentAgent);
  const agentId = currentAgent?.id;

  return ensureClientInitialized(agentId);
}

// Hook to connect/disconnect WebSocket.
export function useWebSocketConnection(autoConnect = true): {
  state: ConnectionState;
  connect: () => void;
  disconnect: () => void;
} {
  const client = useWebSocketClient();
  const state = useConnectionState();
  const mounted = useRef(true);

  const connect = useCallback(() => {
    if (mounted.current) {
      client.connect();
    }
  }, [client]);

  const disconnect = useCallback(() => {
    client.disconnect();
  }, [client]);

  useEffect(() => {
    mounted.current = true;

    if (autoConnect && state === 'disconnected') {
      connect();
    }

    return () => {
      mounted.current = false;
    };
  }, [autoConnect, connect, state]);

  return { state, connect, disconnect };
}

// Hook to subscribe to WebSocket messages.
export function useWebSocketMessage<T = unknown>(
  type: WSMessageType,
  handler: WSEventListener<T>,
): void {
  const client = useWebSocketClient();

  useEffect(() => {
    const unsubscribe = client.on<T>(type, handler);
    return unsubscribe;
  }, [client, type, handler]);
}

// Reset hook state (useful for testing).
export function resetWebSocketHookState(): void {
  connectionState = 'disconnected';
  clientInitialized = false;
  unreadCount = 0;
  stateListeners.clear();
  unreadListeners.clear();
}

// Unread count state.
let unreadCount = 0;
const unreadListeners = new Set<() => void>();

function subscribeToUnread(callback: () => void) {
  unreadListeners.add(callback);
  return () => unreadListeners.delete(callback);
}

function getUnreadCount() {
  return unreadCount;
}

function updateUnreadCount(count: number) {
  unreadCount = count;
  unreadListeners.forEach((listener) => listener());
}

// Hook to get unread message count from WebSocket.
export function useUnreadCount(): number {
  const client = useWebSocketClient();
  const count = useSyncExternalStore(subscribeToUnread, getUnreadCount, getUnreadCount);

  useEffect(() => {
    const unsubscribe = client.on<{ count: number }>('unread_count', (payload) => {
      updateUnreadCount(payload.count);
    });
    return unsubscribe;
  }, [client]);

  return count;
}

// Agent update payload type.
export interface AgentUpdatePayload {
  agents: Array<{
    id: number;
    name: string;
    status: string;
    last_active_at: string;
    seconds_since_heartbeat: number;
  }>;
  counts: {
    active: number;
    busy: number;
    idle: number;
    offline: number;
  };
}

// Activity payload type.
export interface ActivityPayload {
  id: number;
  agent_id: number;
  agent_name: string;
  type: string;
  description: string;
  created_at: string;
}

// New message payload type.
export interface NewMessagePayload {
  id: number;
  sender_id: number;
  sender_name: string;
  subject: string;
  body: string;
  priority: string;
  created_at: string;
}

// Hook to subscribe to agent updates.
export function useAgentUpdates(handler: (payload: AgentUpdatePayload) => void): void {
  const client = useWebSocketClient();

  useEffect(() => {
    const unsubscribe = client.on<AgentUpdatePayload>('agent_update', handler);
    return unsubscribe;
  }, [client, handler]);
}

// Hook to subscribe to activity updates.
export function useActivityUpdates(handler: (payload: ActivityPayload[]) => void): void {
  const client = useWebSocketClient();

  useEffect(() => {
    const unsubscribe = client.on<ActivityPayload[]>('activity', handler);
    return unsubscribe;
  }, [client, handler]);
}

// Hook to subscribe to new messages.
export function useNewMessages(handler: (payload: NewMessagePayload) => void): void {
  const client = useWebSocketClient();

  useEffect(() => {
    const unsubscribe = client.on<NewMessagePayload>('new_message', handler);
    return unsubscribe;
  }, [client, handler]);
}

// Combined hook for common WebSocket operations.
export function useWebSocket(options: { autoConnect?: boolean } = {}): {
  state: ConnectionState;
  connect: () => void;
  disconnect: () => void;
  unreadCount: number;
  client: WebSocketClient;
} {
  const { autoConnect = true } = options;
  const { state, connect, disconnect } = useWebSocketConnection(autoConnect);
  const count = useUnreadCount();
  const client = useWebSocketClient();

  return {
    state,
    connect,
    disconnect,
    unreadCount: count,
    client,
  };
}
