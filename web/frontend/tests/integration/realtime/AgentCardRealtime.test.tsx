// Integration tests for AgentCardGrid component.

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { AgentCardGrid } from '@/components/agents/AgentCardGrid.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import type { AgentWithStatus } from '@/types/api.js';

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
  }

  send(): void {}

  close(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close', { code: 1000 }));
  }
}

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

// Mock agents data.
const mockAgents: AgentWithStatus[] = [
  {
    id: 1,
    name: 'Agent1',
    status: 'active',
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 30,
  },
  {
    id: 2,
    name: 'Agent2',
    status: 'idle',
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 600,
  },
  {
    id: 3,
    name: 'Agent3',
    status: 'busy',
    last_active_at: new Date().toISOString(),
    session_id: 42,
    seconds_since_heartbeat: 10,
  },
  {
    id: 4,
    name: 'Agent4',
    status: 'offline',
    last_active_at: new Date(Date.now() - 3600000).toISOString(),
    seconds_since_heartbeat: 3600,
  },
];

describe('AgentCardGrid integration', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('WebSocket', MockWebSocket);
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient = createTestQueryClient();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetWebSocketClient();
    resetWebSocketHookState();
    queryClient.clear();
  });

  it('renders agents from props', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <AgentCardGrid agents={mockAgents} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.getByText('Agent3')).toBeInTheDocument();
    expect(screen.getByText('Agent4')).toBeInTheDocument();
  });

  it('shows status badges', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <AgentCardGrid agents={mockAgents} />
      </QueryClientProvider>,
    );

    // Status labels appear both as filter buttons and status badges.
    // Use getAllByText since statuses appear in both places.
    const activeElements = screen.getAllByText('Active');
    expect(activeElements.length).toBeGreaterThanOrEqual(1);

    const idleElements = screen.getAllByText('Idle');
    expect(idleElements.length).toBeGreaterThanOrEqual(1);

    const busyElements = screen.getAllByText('Busy');
    expect(busyElements.length).toBeGreaterThanOrEqual(1);

    const offlineElements = screen.getAllByText('Offline');
    expect(offlineElements.length).toBeGreaterThanOrEqual(1);
  });

  it('shows session info for busy agents', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <AgentCardGrid agents={mockAgents} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('#42')).toBeInTheDocument();
  });

  it('shows empty state when no agents', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <AgentCardGrid agents={[]} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('No agents')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <AgentCardGrid agents={[]} isLoading={true} />
      </QueryClientProvider>,
    );

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
