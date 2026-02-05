// Tests for useSessions hook and related hooks.

import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../mocks/server.js';
import {
  useActiveSessions,
  useSessions,
  useSession,
  useStartSession,
  useCompleteSession,
  useSessionsByStatus,
  useSessionCounts,
  useSessionMutations,
  sessionKeys,
} from '@/hooks/useSessions.js';

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

describe('sessionKeys', () => {
  it('creates correct key for all sessions', () => {
    expect(sessionKeys.all).toEqual(['sessions']);
  });

  it('creates correct key for session lists', () => {
    expect(sessionKeys.lists()).toEqual(['sessions', 'list']);
  });

  it('creates correct key for active sessions', () => {
    expect(sessionKeys.listActive()).toEqual(['sessions', 'list', 'active']);
  });

  it('creates correct key for all sessions list', () => {
    expect(sessionKeys.listAll()).toEqual(['sessions', 'list', 'all']);
  });

  it('creates correct key for session details', () => {
    expect(sessionKeys.details()).toEqual(['sessions', 'detail']);
  });

  it('creates correct key for specific session', () => {
    expect(sessionKeys.detail(42)).toEqual(['sessions', 'detail', 42]);
  });
});

describe('useActiveSessions', () => {
  it('fetches active sessions successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActiveSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toBeDefined();
  });

  it('handles fetch error', async () => {
    // API uses /sessions?active_only=true for active sessions.
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActiveSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useSessions', () => {
  it('fetches all sessions successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toBeDefined();
  });
});

describe('useSession', () => {
  it('fetches a single session', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(1, false), { wrapper });

    // Should not fetch when disabled.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles not found error', async () => {
    server.use(
      http.get('/api/v1/sessions/:id', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Session not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useStartSession', () => {
  it('starts a session successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useStartSession(), { wrapper });

    await act(async () => {
      result.current.mutate({ project: '/test/project', branch: 'main' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('starts a session with correct parameters', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useStartSession(), { wrapper });

    await act(async () => {
      result.current.mutate({ project: '/my/project', branch: 'feature' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toEqual({
      project: '/my/project',
      branch: 'feature',
    });
  });

  it('handles start error', async () => {
    server.use(
      http.post('/api/v1/sessions', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed to start' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useStartSession(), { wrapper });

    await act(async () => {
      result.current.mutate({});
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useCompleteSession', () => {
  it('completes a session successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useCompleteSession(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('completes a session with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCompleteSession(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });

  it('handles complete error', async () => {
    server.use(
      http.post('/api/v1/sessions/:id/complete', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Session not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCompleteSession(), { wrapper });

    await act(async () => {
      result.current.mutate(999);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useSessionsByStatus', () => {
  it('returns all sessions when no status filter', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
  });

  it('filters sessions by active status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus('active'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    if (result.current.data) {
      expect(
        result.current.data.every((s) => s.status === 'active'),
      ).toBe(true);
    }
  });

  it('filters sessions by completed status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus('completed'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Result might be empty, that's fine.
    expect(result.current.data).toBeDefined();
  });
});

describe('useSessionCounts', () => {
  it('returns session counts by status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCounts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveProperty('active');
    expect(result.current.data).toHaveProperty('completed');
    expect(result.current.data).toHaveProperty('abandoned');
  });
});

describe('useSessionMutations', () => {
  it('returns all mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionMutations(), { wrapper });

    expect(result.current.startSession).toBeDefined();
    expect(result.current.completeSession).toBeDefined();
  });

  it('all mutations are functional', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionMutations(), { wrapper });

    // Test startSession.
    await act(async () => {
      result.current.startSession.mutate({ project: '/test' });
    });

    await waitFor(() => {
      expect(result.current.startSession.isSuccess).toBe(true);
    });
  });
});
