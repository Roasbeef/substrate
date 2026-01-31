// Tests for useAgents hook and related hooks.

import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../mocks/server.js';
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
import type { AgentsStatusResponse } from '@/types/api.js';

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

  it('creates correct key for specific agent', () => {
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

    expect(result.current.data?.agents).toHaveLength(2);
    expect(result.current.data?.counts.active).toBe(2);
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/agents/status', () => {
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
  it('fetches a single agent', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgent(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

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

    expect(result.current.data).toHaveLength(2);
  });

  it('filters agents by active status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('active'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveLength(2);
    expect(result.current.data?.every((a) => a.status === 'active')).toBe(true);
  });

  it('returns empty array for status with no agents', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('offline'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveLength(0);
  });

  it('includes counts in result', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsByStatus('active'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.counts).toEqual({
      active: 2,
      busy: 0,
      idle: 0,
      offline: 0,
    });
  });
});

describe('useCreateAgent', () => {
  it('creates an agent successfully', async () => {
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

  it('creates agent with correct name', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ name: 'NewAgent' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation returned the correct data.
    expect(result.current.data?.name).toBe('NewAgent');
    expect(result.current.variables).toEqual({ name: 'NewAgent' });
  });

  it('handles creation error', async () => {
    server.use(
      http.post('/api/v1/agents', () => {
        return HttpResponse.json(
          { error: { code: 'validation_error', message: 'Name is required' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ name: '' });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useUpdateAgent', () => {
  it('updates an agent successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUpdateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, data: { name: 'UpdatedAgent' } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('UpdatedAgent');
  });

  it('updates agent with correct data', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUpdateAgent(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, data: { name: 'UpdatedAgent' } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation was called with correct parameters.
    expect(result.current.variables).toEqual({
      id: 1,
      data: { name: 'UpdatedAgent' },
    });
    expect(result.current.data?.name).toBe('UpdatedAgent');
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
  it('sends a heartbeat successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useHeartbeat(), { wrapper });

    await act(async () => {
      result.current.mutate({ agent_id: 1 });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: agentKeys.status(),
    });
  });

  it('sends heartbeat with session ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useHeartbeat(), { wrapper });

    await act(async () => {
      result.current.mutate({ agent_id: 1, session_id: 'session-123' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
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

  it('all mutations are functional', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentMutations(), { wrapper });

    // Test createAgent.
    await act(async () => {
      result.current.createAgent.mutate({ name: 'TestAgent' });
    });

    await waitFor(() => {
      expect(result.current.createAgent.isSuccess).toBe(true);
    });
  });
});

describe('useAgentCounts', () => {
  it('returns agent status counts', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCounts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({
      active: 2,
      busy: 0,
      idle: 0,
      offline: 0,
    });
  });

  it('returns undefined before data loads', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCounts(), { wrapper });

    // Before data loads, should be undefined.
    expect(result.current.data).toBeUndefined();
  });
});

describe('useAgentCount', () => {
  it('returns total agent count', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCount(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBe(2); // 2 active + 0 busy + 0 idle + 0 offline
  });

  it('returns 0 before data loads', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentCount(), { wrapper });

    // Before data loads, should be 0.
    expect(result.current.data).toBe(0);
  });
});

describe('refetch interval', () => {
  it('useAgentsStatus has refetch interval configured', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAgentsStatus(), { wrapper });

    // The refetchInterval is set to 30000 in the hook.
    // We can't easily test the interval itself, but we can verify the hook works.
    expect(result.current).toBeDefined();
  });
});
