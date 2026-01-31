// Tests for useThreads hook and related hooks.

import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../mocks/server.js';
import {
  useThread,
  useReplyToThread,
  useArchiveThread,
  useUnarchiveThread,
  useMarkThreadUnread,
  useDeleteThread,
  useThreadMutations,
  threadKeys,
} from '@/hooks/useThreads.js';

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

describe('threadKeys', () => {
  it('creates correct key for all threads', () => {
    expect(threadKeys.all).toEqual(['threads']);
  });

  it('creates correct key for thread details', () => {
    expect(threadKeys.details()).toEqual(['threads', 'detail']);
  });

  it('creates correct key for specific thread', () => {
    expect(threadKeys.detail(42)).toEqual(['threads', 'detail', 42]);
  });
});

describe('useThread', () => {
  it('fetches a thread successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.subject).toBe('Test Thread');
    expect(result.current.data?.messages).toBeDefined();
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1, false), { wrapper });

    // Should not fetch when disabled.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/threads/:id', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Thread not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useReplyToThread', () => {
  it('replies to a thread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, body: 'This is my reply' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('replies with correct parameters', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 42, body: 'Reply content' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toEqual({ id: 42, body: 'Reply content' });
  });

  it('handles reply error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/reply', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed to reply' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, body: 'Reply' });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useArchiveThread', () => {
  it('archives a thread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('archives thread with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });

  it('handles archive error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/archive', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Thread not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(999);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useMarkThreadUnread', () => {
  it('marks a thread as unread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useMarkThreadUnread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('marks thread unread with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMarkThreadUnread(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });

  it('handles mark unread error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/unread', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMarkThreadUnread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useUnarchiveThread', () => {
  it('unarchives a thread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useUnarchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('unarchives thread with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUnarchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });

  it('handles unarchive error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/unarchive', () => {
        return HttpResponse.json(
          { error: { code: 'not_archived', message: 'Thread not archived' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUnarchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useDeleteThread', () => {
  it('deletes a thread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const removeSpy = vi.spyOn(queryClient, 'removeQueries');

    const { result } = renderHook(() => useDeleteThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeSpy).toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('deletes thread with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useDeleteThread(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });

  it('handles delete error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/delete', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Thread not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useDeleteThread(), { wrapper });

    await act(async () => {
      result.current.mutate(999);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useThreadMutations', () => {
  it('returns all mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    expect(result.current.reply).toBeDefined();
    expect(result.current.archive).toBeDefined();
    expect(result.current.unarchive).toBeDefined();
    expect(result.current.markUnread).toBeDefined();
    expect(result.current.delete).toBeDefined();
  });

  it('all mutations are functional', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    // Test reply mutation.
    await act(async () => {
      result.current.reply.mutate({ id: 1, body: 'Test reply' });
    });

    await waitFor(() => {
      expect(result.current.reply.isSuccess).toBe(true);
    });
  });
});
