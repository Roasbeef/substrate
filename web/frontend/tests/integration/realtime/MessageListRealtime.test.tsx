// Integration tests for real-time MessageList updates.

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../../mocks/server.js';
import { MessageList } from '@/components/inbox/MessageList.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';
import type { MessageWithRecipients } from '@/types/api.js';

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

// Mock messages with proper MessageWithRecipients structure.
const mockMessages: MessageWithRecipients[] = [
  {
    id: 1,
    sender_id: 1,
    sender_name: 'Agent1',
    subject: 'Test Message 1',
    body: 'Body 1',
    priority: 'normal',
    created_at: '2024-01-01T10:00:00Z',
    recipients: [
      {
        message_id: 1,
        agent_id: 100,
        agent_name: 'Recipient1',
        state: 'unread',
        is_starred: false,
        is_archived: false,
      },
    ],
  },
  {
    id: 2,
    sender_id: 2,
    sender_name: 'Agent2',
    subject: 'Test Message 2',
    body: 'Body 2',
    priority: 'urgent',
    created_at: '2024-01-01T09:00:00Z',
    recipients: [
      {
        message_id: 2,
        agent_id: 100,
        agent_name: 'Recipient1',
        state: 'read',
        is_starred: true,
        is_archived: false,
      },
    ],
  },
];

describe('MessageList integration', () => {
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

  it('renders messages from props', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MessageList messages={mockMessages} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Test Message 1')).toBeInTheDocument();
    expect(screen.getByText('Test Message 2')).toBeInTheDocument();
  });

  it('shows empty state when no messages', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MessageList messages={[]} isEmpty={true} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('No messages')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MessageList messages={[]} isLoading={true} loadingRows={3} />
      </QueryClientProvider>,
    );

    // Loading skeletons should be present.
    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('renders message with priority badge', () => {
    const urgentMessages: MessageWithRecipients[] = [
      {
        id: 1,
        sender_id: 1,
        sender_name: 'Agent1',
        subject: 'Urgent Message',
        body: 'Body',
        priority: 'urgent',
        created_at: '2024-01-01T10:00:00Z',
        recipients: [
          {
            message_id: 1,
            agent_id: 100,
            agent_name: 'Recipient1',
            state: 'unread',
            is_starred: false,
            is_archived: false,
          },
        ],
      },
    ];

    render(
      <QueryClientProvider client={queryClient}>
        <MessageList messages={urgentMessages} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Urgent Message')).toBeInTheDocument();
    // PriorityBadge displays 'Urgent' (capitalized).
    expect(screen.getByText('Urgent')).toBeInTheDocument();
  });

  it('displays sender names correctly', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MessageList messages={mockMessages} />
      </QueryClientProvider>,
    );

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
  });
});
