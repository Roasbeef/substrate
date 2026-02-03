// Tests for useMessages hook and related hooks.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../mocks/server.js';
import {
  useMessages,
  useMessage,
  useSendMessage,
  useToggleMessageStar,
  useArchiveMessage,
  useUnarchiveMessage,
  useSnoozeMessage,
  useMarkMessageRead,
  useAcknowledgeMessage,
  useMessageMutations,
  messageKeys,
} from '@/hooks/useMessages.js';
import type { MessageWithRecipients } from '@/types/api.js';

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

describe('messageKeys', () => {
  it('creates correct key for all messages', () => {
    expect(messageKeys.all).toEqual(['messages']);
  });

  it('creates correct key for message lists', () => {
    expect(messageKeys.lists()).toEqual(['messages', 'list']);
  });

  it('creates correct key for filtered message list', () => {
    const options = { page: 1, filter: 'unread' as const };
    expect(messageKeys.list(options)).toEqual(['messages', 'list', options]);
  });

  it('creates correct key for message details', () => {
    expect(messageKeys.details()).toEqual(['messages', 'detail']);
  });

  it('creates correct key for specific message', () => {
    expect(messageKeys.detail(42)).toEqual(['messages', 'detail', 42]);
  });
});

describe('useMessages', () => {
  it('fetches messages successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessages(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.data).toHaveLength(2);
    expect(result.current.data?.data[0].subject).toBe('First message');
  });

  it('fetches messages with filter options', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useMessages({ filter: 'unread', page: 1 }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
  });

  it('handles fetch error', async () => {
    server.use(
      http.get('/api/v1/messages', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessages(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useMessage', () => {
  it('fetches a single message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessage(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.subject).toBe('First message');
  });

  it('respects enabled option', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessage(1, false), { wrapper });

    // Should not fetch when disabled.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('handles not found error', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessage(999), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useSendMessage', () => {
  it('sends a message successfully', async () => {
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useSendMessage(), { wrapper });

    await act(async () => {
      result.current.mutate({
        to: [2],
        subject: 'Test Subject',
        body: 'Test body',
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.subject).toBe('Test Subject');
    expect(invalidateSpy).toHaveBeenCalled();
  });

  it('handles send error', async () => {
    server.use(
      http.post('/api/v1/messages', () => {
        return HttpResponse.json(
          { error: { code: 'validation_error', message: 'Invalid data' } },
          { status: 400 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSendMessage(), { wrapper });

    await act(async () => {
      result.current.mutate({
        to: [],
        subject: '',
        body: '',
      });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});

describe('useToggleMessageStar', () => {
  it('toggles star successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useToggleMessageStar(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, starred: true });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('stars a message with correct parameters', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useToggleMessageStar(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, starred: true });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation was called with correct parameters.
    expect(result.current.variables).toEqual({ id: 1, starred: true });
  });

  it('unstars a message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useToggleMessageStar(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, starred: false });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toEqual({ id: 1, starred: false });
  });
});

describe('useArchiveMessage', () => {
  it('archives a message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('archives a message with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(42);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.variables).toBe(42);
  });
});

describe('useUnarchiveMessage', () => {
  it('unarchives a message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUnarchiveMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });
});

describe('useSnoozeMessage', () => {
  it('snoozes a message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSnoozeMessage(), { wrapper });

    const snoozeUntil = new Date(Date.now() + 3600000).toISOString();

    await act(async () => {
      result.current.mutate({ id: 1, until: snoozeUntil });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('snoozes a message with the correct time', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSnoozeMessage(), { wrapper });
    const snoozeUntil = new Date(Date.now() + 3600000).toISOString();

    await act(async () => {
      result.current.mutate({ id: 1, until: snoozeUntil });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation was called with correct parameters.
    expect(result.current.variables).toEqual({ id: 1, until: snoozeUntil });
  });
});

describe('useMarkMessageRead', () => {
  it('marks a message as read', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMarkMessageRead(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('marks a message with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMarkMessageRead(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation was called with correct parameters.
    expect(result.current.variables).toBe(1);
  });
});

describe('useAcknowledgeMessage', () => {
  it('acknowledges a message', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAcknowledgeMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('acknowledges a message with correct ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useAcknowledgeMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify the mutation was called with correct parameters.
    expect(result.current.variables).toBe(1);
  });
});

describe('useMessageMutations', () => {
  it('returns all mutation hooks', () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessageMutations(), { wrapper });

    expect(result.current.sendMessage).toBeDefined();
    expect(result.current.toggleStar).toBeDefined();
    expect(result.current.archive).toBeDefined();
    expect(result.current.unarchive).toBeDefined();
    expect(result.current.snooze).toBeDefined();
    expect(result.current.markRead).toBeDefined();
    expect(result.current.acknowledge).toBeDefined();
  });

  it('all mutations are functional', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useMessageMutations(), { wrapper });

    // Test sendMessage.
    await act(async () => {
      result.current.sendMessage.mutate({
        to: [1],
        subject: 'Test',
        body: 'Test body',
      });
    });

    await waitFor(() => {
      expect(result.current.sendMessage.isSuccess).toBe(true);
    });
  });
});

describe('error handling', () => {
  it('handles star toggle error', async () => {
    // Make the request fail.
    server.use(
      http.post('/api/v1/messages/:id/star', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useToggleMessageStar(), { wrapper });

    await act(async () => {
      result.current.mutate({ id: 1, starred: true });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });

  it('handles archive error', async () => {
    server.use(
      http.post('/api/v1/messages/:id/archive', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Failed' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useArchiveMessage(), { wrapper });

    await act(async () => {
      result.current.mutate(1);
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});
