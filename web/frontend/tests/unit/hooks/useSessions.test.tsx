// Unit tests for useSessions hooks.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../../mocks/server.js';
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
import type { Session } from '@/types/api.js';

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

// Mock session data.
const mockSessions: Session[] = [
  {
    id: 1,
    agent_id: 1,
    agent_name: 'Agent1',
    project: '/path/to/project-a',
    branch: 'main',
    started_at: new Date().toISOString(),
    status: 'active',
  },
  {
    id: 2,
    agent_id: 2,
    agent_name: 'Agent2',
    project: '/path/to/project-b',
    branch: 'feature',
    started_at: new Date(Date.now() - 3600000).toISOString(),
    ended_at: new Date().toISOString(),
    status: 'completed',
  },
  {
    id: 3,
    agent_id: 3,
    agent_name: 'Agent3',
    project: '/path/to/project-c',
    branch: 'dev',
    started_at: new Date(Date.now() - 7200000).toISOString(),
    ended_at: new Date(Date.now() - 3600000).toISOString(),
    status: 'abandoned',
  },
];

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

// Helper to format sessions in grpc-gateway format.
function formatSessionsResponse(sessions: Session[]) {
  return {
    sessions: sessions.map((s) => ({
      id: String(s.id),
      agent_id: String(s.agent_id),
      agent_name: s.agent_name,
      project: s.project,
      branch: s.branch,
      started_at: s.started_at,
      ended_at: s.ended_at,
      status: `SESSION_STATUS_${s.status?.toUpperCase() ?? 'ACTIVE'}`,
    })),
  };
}

describe('useActiveSessions', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(
          formatSessionsResponse(mockSessions.filter((s) => s.status === 'active'))
        );
      }),
    );
  });

  it('fetches active sessions successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActiveSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toBeDefined();
    expect(result.current.data?.data.every((s) => s.status === 'active')).toBe(true);
  });

  it('handles fetch error', async () => {
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
  beforeEach(() => {
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(formatSessionsResponse(mockSessions));
      }),
    );
  });

  it('fetches all sessions successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toHaveLength(3);
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useSession', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/sessions/:id', ({ params }) => {
        const id = Number(params.id);
        const session = mockSessions.find((s) => s.id === id);
        if (!session) {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Session not found' } },
            { status: 404 },
          );
        }
        // Return in grpc-gateway format.
        return HttpResponse.json({
          session: {
            id: String(session.id),
            agent_id: String(session.agent_id),
            agent_name: session.agent_name,
            project: session.project,
            branch: session.branch,
            started_at: session.started_at,
            ended_at: session.ended_at,
            status: `SESSION_STATUS_${session.status?.toUpperCase() ?? 'ACTIVE'}`,
          },
        });
      }),
    );
  });

  it('fetches a single session', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe(1);
    expect(result.current.data?.agent_name).toBe('Agent1');
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(1, false), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles not found error', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSession(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useStartSession', () => {
  beforeEach(() => {
    server.use(
      http.post('/api/v1/sessions', async ({ request }) => {
        const body = (await request.json()) as { project?: string; branch?: string };
        return HttpResponse.json({
          session: {
            id: '100',
            agent_id: '1',
            agent_name: 'TestAgent',
            project: body.project,
            branch: body.branch,
            started_at: new Date().toISOString(),
            status: 'SESSION_STATUS_ACTIVE',
          },
        });
      }),
    );
  });

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

    expect(result.current.data?.project).toBe('/test/project');
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('handles start error', async () => {
    server.use(
      http.post('/api/v1/sessions', () => {
        return HttpResponse.json(
          { error: { code: 'already_active', message: 'Session already active' } },
          { status: 400 },
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
  });
});

describe('useCompleteSession', () => {
  beforeEach(() => {
    server.use(
      http.post('/api/v1/sessions/:id/complete', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );
  });

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
  });
});

describe('useSessionsByStatus', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(formatSessionsResponse(mockSessions));
      }),
    );
  });

  it('returns all sessions when no status filter', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveLength(3);
  });

  it('filters sessions by active status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus('active'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.data?.forEach((session) => {
      expect(session.status).toBe('active');
    });
  });

  it('filters sessions by completed status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus('completed'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.data?.forEach((session) => {
      expect(session.status).toBe('completed');
    });
  });

  it('filters sessions by abandoned status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionsByStatus('abandoned'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.data?.forEach((session) => {
      expect(session.status).toBe('abandoned');
    });
  });
});

describe('useSessionCounts', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/sessions', () => {
        return HttpResponse.json(formatSessionsResponse(mockSessions));
      }),
    );
  });

  it('returns counts by status', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCounts(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.active).toBe(1);
    expect(result.current.data?.completed).toBe(1);
    expect(result.current.data?.abandoned).toBe(1);
  });

  it('returns zero counts when no data', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCounts(), { wrapper });

    // Before data loads.
    expect(result.current.data).toEqual({ active: 0, completed: 0, abandoned: 0 });
  });
});

describe('useSessionMutations', () => {
  it('returns all mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionMutations(), { wrapper });

    expect(result.current.startSession).toBeDefined();
    expect(result.current.completeSession).toBeDefined();
  });

  it('all mutations have mutate function', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionMutations(), { wrapper });

    expect(typeof result.current.startSession.mutate).toBe('function');
    expect(typeof result.current.completeSession.mutate).toBe('function');
  });
});
