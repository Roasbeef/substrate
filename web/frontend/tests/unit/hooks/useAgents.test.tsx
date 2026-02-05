// Unit tests for useAgents hooks.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../../mocks/server.js';
import {
  useAgentsStatus,
  useAgent,
  useAgentsByStatus,
  useCreateAgent,
  useUpdateAgent,
  useHeartbeat,
  useAgentMutations,
  useAgentCounts,
  useAgentCount,
  agentKeys,
} from '@/hooks/useAgents.js';
import type { AgentsStatusResponse, AgentWithStatus } from '@/types/api.js';

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

describe('agentKeys', () => {
  it('creates correct key for all agents', () => {
    expect(agentKeys.all).toEqual(['agents']);
  });

  it('creates correct key for agents status', () => {
    expect(agentKeys.status()).toEqual(['agents', 'status']);
  });

  it('creates correct key for agent details', () => {
    expect(agentKeys.details()).toEqual(['agents', 'detail']);
  });

  it('creates correct key for specific agent detail', () => {
    expect(agentKeys.detail(42)).toEqual(['agents', 'detail', 42]);
  });
});

describe('useAgentsStatus', () => {
  it('fetches agents status successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.agents).toBeDefined();
    expect(Array.isArray(result.current.data?.agents)).toBe(true);
    expect(result.current.data?.counts).toBeDefined();
  });

  it('returns counts for each status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const counts = result.current.data?.counts;
    expect(counts).toHaveProperty('active');
    expect(counts).toHaveProperty('busy');
    expect(counts).toHaveProperty('idle');
    expect(counts).toHaveProperty('offline');
  });

  it('handles fetch error', async () => {
    // Mock the grpc-gateway endpoint (uses hyphen, not slash).
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useAgent', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/agents/:id', ({ params }) => {
        const id = Number(params.id);
        if (id === 999) {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Agent not found' } },
            { status: 404 },
          );
        }
        return HttpResponse.json({
          id,
          name: `Agent${id}`,
          created_at: '2024-01-01T00:00:00Z',
        });
      }),
    );
  });

  it('fetches a single agent', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgent(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe(1);
    expect(result.current.data?.name).toBe('Agent1');
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgent(1, false), { wrapper });

    // Should not fetch when disabled.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles not found error', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgent(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useAgentsByStatus', () => {
  it('returns all agents when no status filter', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
    expect(Array.isArray(result.current.data)).toBe(true);
  });

  it('filters agents by active status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('active'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // All returned agents should be active.
    result.current.data?.forEach((agent: AgentWithStatus) => {
      expect(agent.status).toBe('active');
    });
  });

  it('filters agents by busy status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('busy'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.data?.forEach((agent: AgentWithStatus) => {
      expect(agent.status).toBe('busy');
    });
  });

  it('filters agents by idle status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('idle'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.data?.forEach((agent: AgentWithStatus) => {
      expect(agent.status).toBe('idle');
    });
  });

  it('returns counts alongside filtered data', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('active'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.counts).toBeDefined();
  });
});

describe('useCreateAgent', () => {
  it('creates an agent successfully', async () => {
    server.use(
      http.post('/api/v1/agents', async ({ request }) => {
        const body = (await request.json()) as { name: string };
        return HttpResponse.json({
          id: 100,
          name: body.name,
          created_at: new Date().toISOString(),
        });
      }),
    );

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useCreateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ name: 'NewAgent' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('NewAgent');
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('handles creation error', async () => {
    server.use(
      http.post('/api/v1/agents', () => {
        return HttpResponse.json(
          { error: { code: 'conflict', message: 'Agent already exists' } },
          { status: 409 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ name: 'ExistingAgent' });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });

  it('invalidates queries on success', async () => {
    server.use(
      http.post('/api/v1/agents', async ({ request }) => {
        const body = (await request.json()) as { name: string };
        return HttpResponse.json({
          id: 100,
          name: body.name,
          created_at: new Date().toISOString(),
        });
      }),
    );

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useCreateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ name: 'NewAgent' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify invalidateQueries was called to refresh data.
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: agentKeys.status() }),
    );
  });
});

describe('useUpdateAgent', () => {
  beforeEach(() => {
    server.use(
      http.patch('/api/v1/agents/:id', async ({ params, request }) => {
        const id = Number(params.id);
        const body = (await request.json()) as { name: string };
        return HttpResponse.json({
          id,
          name: body.name,
          created_at: '2024-01-01T00:00:00Z',
        });
      }),
    );
  });

  it('updates an agent successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUpdateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, data: { name: 'UpdatedName' } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('UpdatedName');
  });

  it('updates agent detail cache on success', async () => {
    const { wrapper, queryClient } = createWrapper();
    const setQueryDataSpy = vi.spyOn(queryClient, 'setQueryData');

    const { result } = renderHook(() => useUpdateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, data: { name: 'NewName' } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify setQueryData was called to update agent detail cache.
    expect(setQueryDataSpy).toHaveBeenCalledWith(
      agentKeys.detail(1),
      expect.objectContaining({ id: 1, name: 'NewName' }),
    );
  });

  it('handles update error', async () => {
    server.use(
      http.patch('/api/v1/agents/:id', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Agent not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUpdateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 999, data: { name: 'Test' } });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useHeartbeat', () => {
  it('sends heartbeat successfully', async () => {
    server.use(
      http.post('/api/v1/heartbeat', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useHeartbeat(), { wrapper });

    await act(async () => {
      result.current.mutate({ agent_name: 'TestAgent' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should invalidate status to refresh.
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('handles heartbeat error', async () => {
    server.use(
      http.post('/api/v1/heartbeat', () => {
        return HttpResponse.json(
          { error: { code: 'invalid_agent', message: 'Agent not found' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useHeartbeat(), { wrapper });

    await act(async () => {
      result.current.mutate({ agent_name: 'InvalidAgent' });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useAgentMutations', () => {
  it('returns all mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentMutations(), { wrapper });

    expect(result.current.createAgent).toBeDefined();
    expect(result.current.updateAgent).toBeDefined();
    expect(result.current.heartbeat).toBeDefined();
  });

  it('all mutations have mutate function', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentMutations(), { wrapper });

    expect(typeof result.current.createAgent.mutate).toBe('function');
    expect(typeof result.current.updateAgent.mutate).toBe('function');
    expect(typeof result.current.heartbeat.mutate).toBe('function');
  });
});

describe('useAgentCounts', () => {
  it('returns agent counts', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCounts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
    expect(result.current.data).toHaveProperty('active');
    expect(result.current.data).toHaveProperty('busy');
    expect(result.current.data).toHaveProperty('idle');
    expect(result.current.data).toHaveProperty('offline');
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCounts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useAgentCount', () => {
  it('returns total agent count', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCount(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(typeof result.current.data).toBe('number');
    expect(result.current.data).toBeGreaterThanOrEqual(0);
  });

  it('calculates total from all statuses', async () => {
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json({
          agents: [],
          counts: { active: 2, busy: 1, idle: 3, offline: 4 },
        });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCount(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBe(10);
  });

  it('returns 0 when no data', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCount(), { wrapper });

    // Before data loads.
    expect(result.current.data).toBe(0);
  });
});
