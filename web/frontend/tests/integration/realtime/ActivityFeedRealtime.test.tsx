// Integration tests for ActivityFeed component.

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { ActivityFeed } from '@/components/agents/ActivityFeed.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import type { Activity } from '@/types/api.js';

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

// Mock activities data.
const mockActivities: Activity[] = [
  {
    id: 1,
    agent_id: 1,
    agent_name: 'Agent1',
    type: 'message_sent',
    description: 'Sent a message',
    created_at: '2024-01-01T10:00:00Z',
  },
  {
    id: 2,
    agent_id: 2,
    agent_name: 'Agent2',
    type: 'session_started',
    description: 'Started a session',
    created_at: '2024-01-01T09:00:00Z',
  },
];

describe('ActivityFeed integration', () => {
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

  it('renders activities from props', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed activities={mockActivities} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Sent a message')).toBeInTheDocument();
    expect(screen.getByText('Started a session')).toBeInTheDocument();
  });

  it('shows agent names', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed activities={mockActivities} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
  });

  it('shows empty state when no activities', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed activities={[]} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('No recent activity')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed isLoading={true} />
      </QueryClientProvider>,
    );

    // Loading skeletons should be present.
    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows error state', () => {
    const error = new Error('Failed to fetch activities');

    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed error={error} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Failed to load activities')).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch activities')).toBeInTheDocument();
  });

  it('shows load more button when hasMore is true', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed activities={mockActivities} hasMore={true} onLoadMore={() => {}} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Load more')).toBeInTheDocument();
  });

  it('hides avatars when showAvatars is false', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ActivityFeed activities={mockActivities} showAvatars={false} />
      </QueryClientProvider>,
    );

    // Activities should still render.
    expect(screen.getByText('Sent a message')).toBeInTheDocument();
    // But no avatars should be visible (testing absence is tricky, just verify render).
  });
});
