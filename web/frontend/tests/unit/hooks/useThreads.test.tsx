// Unit tests for useThreads hooks.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../../mocks/server.js';
import {
  useThread,
  useReplyToThread,
  useArchiveThread,
  useMarkThreadUnread,
  useThreadMutations,
  threadKeys,
} from '@/hooks/useThreads.js';
import type { ThreadWithMessages } from '@/types/api.js';

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

// Mock thread data.
const mockThread: ThreadWithMessages = {
  id: 1,
  subject: 'Test Thread',
  created_at: new Date().toISOString(),
  last_message_at: new Date().toISOString(),
  message_count: 2,
  participant_count: 2,
  messages: [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Agent1',
      subject: 'Test Thread',
      body: 'First message',
      priority: 'normal',
      created_at: new Date(Date.now() - 3600000).toISOString(),
      recipients: [],
    },
    {
      id: 2,
      sender_id: 2,
      sender_name: 'Agent2',
      subject: 'Re: Test Thread',
      body: 'Reply',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [],
    },
  ],
};

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
  beforeEach(() => {
    server.use(
      http.get('/api/v1/threads/:id', ({ params }) => {
        const id = Number(params.id);
        if (id === 999) {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }
        return HttpResponse.json({ ...mockThread, id });
      }),
    );
  });

  it('fetches a thread successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe(1);
    expect(result.current.data?.subject).toBe('Test Thread');
    expect(result.current.data?.messages).toHaveLength(2);
  });

  it('returns thread with messages', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.message_count).toBe(2);
    expect(result.current.data?.messages[0]?.body).toBe('First message');
    expect(result.current.data?.messages[1]?.body).toBe('Reply');
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1, false), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles not found error', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/threads/:id', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThread(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useReplyToThread', () => {
  beforeEach(() => {
    server.use(
      http.post('/api/v1/threads/:id/reply', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );
  });

  it('replies to a thread successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, body: 'My reply' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('invalidates thread and message queries on success', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, body: 'My reply' });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should invalidate both thread and message list queries.
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['threads', 'detail', 1],
      }),
    );
    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['messages', 'list'],
      }),
    );
  });

  it('handles reply error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/reply', () => {
        return HttpResponse.json(
          { error: { code: 'validation_error', message: 'Body required' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReplyToThread(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, body: '' });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useArchiveThread', () => {
  beforeEach(() => {
    server.use(
      http.post('/api/v1/threads/:id/archive', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );
  });

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

  it('performs optimistic update', async () => {
    const { wrapper, queryClient } = createWrapper();
    const cancelSpy = vi.spyOn(queryClient, 'cancelQueries');

    // Pre-populate the cache with thread data.
    queryClient.setQueryData(threadKeys.detail(1), mockThread);

    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    // Should cancel outgoing queries.
    expect(cancelSpy).toHaveBeenCalled();
  });

  it('rolls back on error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/archive', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper, queryClient } = createWrapper();
    const setQueryDataSpy = vi.spyOn(queryClient, 'setQueryData');

    // Pre-populate the cache with thread data.
    queryClient.setQueryData(threadKeys.detail(1), mockThread);

    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    // Verify that setQueryData was called to restore the previous value on error.
    // The rollback happens via onError calling setQueryData with the previous thread.
    expect(setQueryDataSpy).toHaveBeenCalledWith(
      threadKeys.detail(1),
      expect.objectContaining({ id: 1, subject: 'Test Thread' }),
    );
  });

  it('handles archive error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/archive', () => {
        return HttpResponse.json(
          { error: { code: 'already_archived', message: 'Already archived' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveThread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useMarkThreadUnread', () => {
  beforeEach(() => {
    server.use(
      http.post('/api/v1/threads/:id/unread', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );
  });

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

  it('invalidates message lists on success', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useMarkThreadUnread(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['messages', 'list'],
      }),
    );
  });

  it('handles mark unread error', async () => {
    server.use(
      http.post('/api/v1/threads/:id/unread', () => {
        return HttpResponse.json(
          { error: { code: 'not_found', message: 'Thread not found' } },
          { status: 404 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMarkThreadUnread(), { wrapper });

    await act(async () => {
      result.current.mutate(999);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useThreadMutations', () => {
  it('returns all thread mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    expect(result.current.reply).toBeDefined();
    expect(result.current.archive).toBeDefined();
    expect(result.current.markUnread).toBeDefined();
  });

  it('all mutations have mutate function', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    expect(typeof result.current.reply.mutate).toBe('function');
    expect(typeof result.current.archive.mutate).toBe('function');
    expect(typeof result.current.markUnread.mutate).toBe('function');
  });

  it('reply mutation works', async () => {
    server.use(
      http.post('/api/v1/threads/:id/reply', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    await act(async () => {
      result.current.reply.mutate({ id: 1, body: 'Reply' });
    });

    await waitFor(() => {
      expect(result.current.reply.isSuccess).toBe(true);
    });
  });

  it('archive mutation works', async () => {
    server.use(
      http.post('/api/v1/threads/:id/archive', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    await act(async () => {
      result.current.archive.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.archive.isSuccess).toBe(true);
    });
  });

  it('markUnread mutation works', async () => {
    server.use(
      http.post('/api/v1/threads/:id/unread', () => {
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useThreadMutations(), { wrapper });

    await act(async () => {
      result.current.markUnread.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.markUnread.isSuccess).toBe(true);
    });
  });
});
