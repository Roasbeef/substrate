// Unit tests for useActivities hooks.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../../mocks/server.js';
import {
  useActivities,
  useInfiniteActivities,
  useAgentActivities,
  useInfiniteAgentActivities,
  activityKeys,
  flattenActivities,
} from '@/hooks/useActivities.js';
import type { ActivitiesResponse } from '@/api/activities.js';

// Create a fresh QueryClient for each test.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

// Wrapper component for hooks.
function createWrapper() {
  const queryClient = createTestQueryClient();
  return {
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}

// Mock activity data generator for grpc-gateway format (used by MSW handlers).
function createGatewayActivities(count: number, startId = 1) {
  return Array.from({ length: count }, (_, i) => ({
    id: String(startId + i),
    type: 'ACTIVITY_TYPE_MESSAGE_SENT',
    agent_id: '1',
    agent_name: 'Agent1',
    description: `Activity ${startId + i}`,
    created_at: new Date(Date.now() - i * 60000).toISOString(),
  }));
}

// Format activities response in grpc-gateway format.
function formatActivitiesResponse(
  activities: ReturnType<typeof createGatewayActivities>,
  total: number,
  page: number,
  pageSize: number,
) {
  return {
    activities,
    total: String(total),
    page,
    page_size: pageSize,
  };
}

// Mock activity data generator for frontend format (used by unit tests).
function createMockActivities(count: number, startId = 1) {
  return Array.from({ length: count }, (_, i) => ({
    id: startId + i,
    type: 'message_sent' as const,
    agent_id: 1,
    agent_name: 'Agent1',
    description: `Activity ${startId + i}`,
    created_at: new Date(Date.now() - i * 60000).toISOString(),
  }));
}

describe('activityKeys', () => {
  it('creates correct key for all activities', () => {
    expect(activityKeys.all).toEqual(['activities']);
  });

  it('creates correct key for activities list', () => {
    expect(activityKeys.list()).toEqual(['activities', 'list', {}]);
  });

  it('creates correct key for activities list with options', () => {
    const options = { type: 'heartbeat', page: 2 };
    expect(activityKeys.list(options)).toEqual(['activities', 'list', options]);
  });

  it('creates correct key for agent activities', () => {
    expect(activityKeys.agent(5)).toEqual(['activities', 'agent', 5, {}]);
  });

  it('creates correct key for agent activities with options', () => {
    const options = { page_size: 10 };
    expect(activityKeys.agent(5, options)).toEqual(['activities', 'agent', 5, options]);
  });
});

describe('useActivities', () => {
  it('fetches activities successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toBeDefined();
    expect(Array.isArray(result.current.data?.data)).toBe(true);
    expect(result.current.data?.meta).toBeDefined();
  });

  it('fetches activities with options', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useActivities({ type: 'heartbeat', page: 1 }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/activities', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useInfiniteActivities', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        const url = new URL(request.url);
        const page = Number(url.searchParams.get('page')) || 1;
        const pageSize = Number(url.searchParams.get('page_size')) || 20;

        const totalItems = 45;
        const startId = (page - 1) * pageSize + 1;
        const itemsThisPage = Math.min(pageSize, totalItems - (page - 1) * pageSize);

        return HttpResponse.json(
          formatActivitiesResponse(
            createGatewayActivities(itemsThisPage, startId),
            totalItems,
            page,
            pageSize,
          ),
        );
      }),
    );
  });

  it('fetches initial page successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.pages).toHaveLength(1);
    expect(result.current.data?.pages[0].data).toBeDefined();
  });

  it('indicates when more pages are available', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.hasNextPage).toBe(true);
  });

  it('provides fetchNextPage function', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the hook provides pagination controls.
    expect(result.current.hasNextPage).toBe(true);
    expect(typeof result.current.fetchNextPage).toBe('function');
    expect(result.current.data?.pages).toHaveLength(1);
    expect(result.current.data?.pageParams).toBeDefined();
  });

  it('stops fetching when no more pages', async () => {
    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        const url = new URL(request.url);
        const pageSize = Number(url.searchParams.get('page_size')) || 20;

        return HttpResponse.json(
          formatActivitiesResponse(createGatewayActivities(5), 5, 1, pageSize),
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteActivities(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.hasNextPage).toBe(false);
  });

  it('respects page_size option', async () => {
    let receivedPageSize: number | null = null;

    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        const url = new URL(request.url);
        receivedPageSize = Number(url.searchParams.get('page_size'));

        return HttpResponse.json(
          formatActivitiesResponse(createGatewayActivities(10), 10, 1, receivedPageSize || 20),
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useInfiniteActivities({ page_size: 10 }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(receivedPageSize).toBe(10);
  });
});

describe('useAgentActivities', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        const url = new URL(request.url);
        const agentId = url.searchParams.get('agent_id');

        if (agentId) {
          const activities = createGatewayActivities(5).map((a) => ({
            ...a,
            agent_id: agentId,
            agent_name: `Agent${agentId}`,
          }));
          return HttpResponse.json(formatActivitiesResponse(activities, 5, 1, 20));
        }

        return HttpResponse.json(formatActivitiesResponse([], 0, 1, 20));
      }),
    );
  });

  it('fetches activities for specific agent', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentActivities(5), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toBeDefined();
    result.current.data?.data.forEach((activity) => {
      expect(activity.agent_id).toBe(5);
    });
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentActivities(5, {}, false), { wrapper });

    // Should not fetch when disabled.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('passes additional options', async () => {
    let receivedUrl = '';

    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        receivedUrl = request.url;
        return HttpResponse.json(formatActivitiesResponse([], 0, 2, 10));
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useAgentActivities(5, { page: 2, page_size: 10 }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(receivedUrl).toContain('agent_id=5');
    expect(receivedUrl).toContain('page=2');
    expect(receivedUrl).toContain('page_size=10');
  });
});

describe('useInfiniteAgentActivities', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/activities', ({ request }) => {
        const url = new URL(request.url);
        const agentId = url.searchParams.get('agent_id');
        const page = Number(url.searchParams.get('page')) || 1;
        const pageSize = Number(url.searchParams.get('page_size')) || 20;

        if (!agentId) {
          return HttpResponse.json(formatActivitiesResponse([], 0, 1, pageSize));
        }

        const totalItems = 25;
        const startId = (page - 1) * pageSize + 1;
        const itemsThisPage = Math.min(pageSize, totalItems - (page - 1) * pageSize);

        const activities = createGatewayActivities(Math.max(0, itemsThisPage), startId).map((a) => ({
          ...a,
          agent_id: agentId,
          agent_name: `Agent${agentId}`,
        }));

        return HttpResponse.json(
          formatActivitiesResponse(activities, totalItems, page, pageSize),
        );
      }),
    );
  });

  it('fetches initial page for agent', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteAgentActivities(5), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.pages).toHaveLength(1);
    result.current.data?.pages[0].data.forEach((activity) => {
      expect(activity.agent_id).toBe(5);
    });
  });

  it('fetches next page for agent', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useInfiniteAgentActivities(5), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.hasNextPage).toBe(true);

    await act(async () => {
      result.current.fetchNextPage();
    });

    await waitFor(() => {
      expect(result.current.isFetching).toBe(false);
    });

    await waitFor(() => {
      expect(result.current.data?.pages.length).toBeGreaterThanOrEqual(2);
    }, { timeout: 3000 });
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useInfiniteAgentActivities(5, {}, false),
      { wrapper },
    );

    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });
});

describe('flattenActivities', () => {
  it('flattens multiple pages into a single array', () => {
    const pages: ActivitiesResponse[] = [
      {
        data: createMockActivities(3, 1),
        meta: { total: 9, page: 1, page_size: 3 },
      },
      {
        data: createMockActivities(3, 4),
        meta: { total: 9, page: 2, page_size: 3 },
      },
      {
        data: createMockActivities(3, 7),
        meta: { total: 9, page: 3, page_size: 3 },
      },
    ];

    const result = flattenActivities({ pages });

    expect(result).toHaveLength(9);
    expect(result[0].id).toBe(1);
    expect(result[8].id).toBe(9);
  });

  it('returns empty array for undefined data', () => {
    const result = flattenActivities(undefined);

    expect(result).toEqual([]);
  });

  it('returns empty array for empty pages', () => {
    const result = flattenActivities({ pages: [] });

    expect(result).toEqual([]);
  });

  it('handles single page', () => {
    const pages: ActivitiesResponse[] = [
      {
        data: createMockActivities(5),
        meta: { total: 5, page: 1, page_size: 20 },
      },
    ];

    const result = flattenActivities({ pages });

    expect(result).toHaveLength(5);
  });
});
