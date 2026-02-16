// WebSocket client for real-time communication with the server.

// WebSocket message types.
export type WSMessageType =
  | 'connected'
  | 'unread_count'
  | 'new_message'
  | 'agent_update'
  | 'activity'
  | 'task_update'
  | 'summary_updated'
  | 'pong'
  | 'subscribed'
  | 'error';

// WebSocket message structure.
export interface WSMessage<T = unknown> {
  type: WSMessageType;
  payload?: T;
}

// Connection state.
export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

// Event listener type.
export type WSEventListener<T = unknown> = (payload: T) => void;

// Configuration options.
export interface WebSocketClientConfig {
  url: string;
  agentId?: number;
  reconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  pingInterval?: number;
}

// Default configuration.
const DEFAULT_CONFIG: Required<Omit<WebSocketClientConfig, 'url' | 'agentId'>> = {
  reconnect: true,
  reconnectInterval: 1000,
  maxReconnectAttempts: 10,
  pingInterval: 30000,
};

// WebSocket client class for managing real-time connections.
export class WebSocketClient {
  private ws: WebSocket | null = null;
  private config: Required<Omit<WebSocketClientConfig, 'agentId'>> & { agentId?: number };
  private state: ConnectionState = 'disconnected';
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private listeners = new Map<WSMessageType, Set<WSEventListener>>();
  private stateListeners = new Set<(state: ConnectionState) => void>();

  constructor(config: WebSocketClientConfig) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  // Get the current connection state.
  getState(): ConnectionState {
    return this.state;
  }

  // Update and notify state listeners.
  private setState(state: ConnectionState): void {
    this.state = state;
    this.stateListeners.forEach((listener) => listener(state));
  }

  // Connect to the WebSocket server.
  connect(): void {
    if (this.ws && (this.ws.readyState === WebSocket.CONNECTING || this.ws.readyState === WebSocket.OPEN)) {
      return;
    }

    this.setState('connecting');

    const url = new URL(this.config.url);
    if (this.config.agentId) {
      url.searchParams.set('agent_id', String(this.config.agentId));
    }

    try {
      this.ws = new WebSocket(url.toString());
      this.setupEventHandlers();
    } catch (error) {
      console.error('WebSocket connection error:', error);
      this.handleDisconnect();
    }
  }

  // Disconnect from the WebSocket server.
  disconnect(): void {
    this.clearTimers();
    this.reconnectAttempts = this.config.maxReconnectAttempts; // Prevent reconnection.

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }

    this.setState('disconnected');
  }

  // Subscribe to connection state changes.
  onStateChange(listener: (state: ConnectionState) => void): () => void {
    this.stateListeners.add(listener);
    return () => this.stateListeners.delete(listener);
  }

  // Subscribe to a specific message type.
  on<T = unknown>(type: WSMessageType, listener: WSEventListener<T>): () => void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type)!.add(listener as WSEventListener);
    return () => this.off(type, listener);
  }

  // Unsubscribe from a specific message type.
  off<T = unknown>(type: WSMessageType, listener: WSEventListener<T>): void {
    const typeListeners = this.listeners.get(type);
    if (typeListeners) {
      typeListeners.delete(listener as WSEventListener);
    }
  }

  // Send a message to the server.
  send(type: string, data?: unknown): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected');
      return false;
    }

    try {
      this.ws.send(JSON.stringify({ type, data }));
      return true;
    } catch (error) {
      console.error('WebSocket send error:', error);
      return false;
    }
  }

  // Send a ping message to keep the connection alive.
  ping(): boolean {
    return this.send('ping');
  }

  // Subscribe to a specific agent's messages.
  subscribeToAgent(agentId: number): boolean {
    return this.send('subscribe', { agent_id: agentId });
  }

  // Setup WebSocket event handlers.
  private setupEventHandlers(): void {
    if (!this.ws) return;

    this.ws.onopen = () => {
      this.setState('connected');
      this.reconnectAttempts = 0;
      this.startPingTimer();
    };

    this.ws.onclose = (event) => {
      console.log('WebSocket closed:', event.code, event.reason);
      this.handleDisconnect();
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onmessage = (event) => {
      this.handleMessage(event);
    };
  }

  // Handle incoming messages.
  private handleMessage(event: MessageEvent): void {
    try {
      const message: WSMessage = JSON.parse(event.data);
      const typeListeners = this.listeners.get(message.type);

      if (typeListeners) {
        typeListeners.forEach((listener) => listener(message.payload));
      }
    } catch (error) {
      console.error('WebSocket message parse error:', error);
    }
  }

  // Handle disconnection and potential reconnection.
  private handleDisconnect(): void {
    this.clearTimers();
    this.ws = null;

    if (this.config.reconnect && this.reconnectAttempts < this.config.maxReconnectAttempts) {
      this.setState('reconnecting');
      this.scheduleReconnect();
    } else {
      this.setState('disconnected');
    }
  }

  // Schedule a reconnection attempt.
  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;

    // Exponential backoff with jitter.
    const baseDelay = this.config.reconnectInterval;
    const delay = Math.min(baseDelay * Math.pow(2, this.reconnectAttempts), 30000);
    const jitter = delay * 0.1 * Math.random();

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.reconnectAttempts++;
      this.connect();
    }, delay + jitter);
  }

  // Start the ping timer for keep-alive.
  private startPingTimer(): void {
    this.clearPingTimer();

    if (this.config.pingInterval > 0) {
      this.pingTimer = setInterval(() => {
        this.ping();
      }, this.config.pingInterval);
    }
  }

  // Clear the ping timer.
  private clearPingTimer(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  // Clear all timers.
  private clearTimers(): void {
    this.clearPingTimer();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

// Create a WebSocket client for the default endpoint.
export function createWebSocketClient(agentId?: number): WebSocketClient {
  // Determine WebSocket URL based on current location.
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const url = `${protocol}//${host}/ws`;

  return new WebSocketClient({
    url,
    ...(agentId !== undefined && { agentId }),
    reconnect: true,
    reconnectInterval: 1000,
    maxReconnectAttempts: 10,
    pingInterval: 30000,
  });
}

// Singleton instance for global use.
let globalClient: WebSocketClient | null = null;

// Get or create the global WebSocket client.
export function getWebSocketClient(agentId?: number): WebSocketClient {
  if (!globalClient) {
    globalClient = createWebSocketClient(agentId);
  }
  return globalClient;
}

// Reset the global client (useful for testing).
export function resetWebSocketClient(): void {
  if (globalClient) {
    globalClient.disconnect();
    globalClient = null;
  }
}
