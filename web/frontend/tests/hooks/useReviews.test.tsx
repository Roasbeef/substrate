// Tests for useReviews hook and related hooks.

import { describe, it, expect } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '../mocks/server.js';
import {
  useReviews,
  useReview,
  useReviewIssues,
  useCreateReview,
  useCancelReview,
  useUpdateIssueStatus,
  reviewKeys,
} from '@/hooks/useReviews.js';

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

describe('reviewKeys', () => {
  it('creates correct key for all reviews', () => {
    expect(reviewKeys.all).toEqual(['reviews']);
  });

  it('creates correct key for review lists', () => {
    expect(reviewKeys.lists()).toEqual(['reviews', 'list']);
  });

  it('creates correct key for filtered list', () => {
    expect(reviewKeys.list({ state: 'approved' })).toEqual([
      'reviews',
      'list',
      { state: 'approved' },
    ]);
  });

  it('creates correct key for review details', () => {
    expect(reviewKeys.details()).toEqual(['reviews', 'detail']);
  });

  it('creates correct key for specific review', () => {
    expect(reviewKeys.detail('abc123')).toEqual([
      'reviews',
      'detail',
      'abc123',
    ]);
  });

  it('creates correct key for review issues', () => {
    expect(reviewKeys.issues('abc123')).toEqual([
      'reviews',
      'issues',
      'abc123',
    ]);
  });
});

describe('useReviews', () => {
  it('fetches reviews successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReviews(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data!.length).toBeGreaterThan(0);
  });

  it('fetches reviews with state filter', async () => {
    server.use(
      http.get('/api/v1/reviews', ({ request }) => {
        const url = new URL(request.url);
        const state = url.searchParams.get('state');
        return HttpResponse.json({
          reviews: state
            ? [{ review_id: 'r1', state, branch: 'test', review_type: 'full' }]
            : [],
        });
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useReviews({ state: 'approved' }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveLength(1);
    expect(result.current.data![0].state).toBe('approved');
  });

  it('handles error response', async () => {
    server.use(
      http.get('/api/v1/reviews', () => {
        return HttpResponse.json(
          { error: { code: 'internal', message: 'Server error' } },
          { status: 500 },
        );
      }),
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReviews(), { wrapper });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useReview', () => {
  it('fetches a single review', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReview('abc123'), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.review_id).toBe('abc123');
    expect(result.current.data?.state).toBeDefined();
  });

  it('does not fetch when disabled', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReview('abc123', false), {
      wrapper,
    });

    // Should not fetch when enabled is false.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });

  it('does not fetch with empty ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReview(''), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });
});

describe('useReviewIssues', () => {
  it('fetches issues for a review', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReviewIssues('abc123'), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data!.length).toBeGreaterThan(0);
    expect(result.current.data![0]).toHaveProperty('severity');
  });

  it('does not fetch with empty review ID', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useReviewIssues(''), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });
});

describe('useCreateReview', () => {
  it('creates a review successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateReview(), { wrapper });

    result.current.mutate({
      branch: 'feature/test',
      commit_sha: 'sha123',
      repo_path: '/repo',
      requester_id: 1,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.review_id).toBeDefined();
  });
});

describe('useCancelReview', () => {
  it('cancels a review successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCancelReview(), { wrapper });

    result.current.mutate({ reviewId: 'abc123' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });
});

describe('useUpdateIssueStatus', () => {
  it('updates issue status successfully', async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useUpdateIssueStatus(), { wrapper });

    result.current.mutate({
      reviewId: 'abc123',
      issueId: 1,
      status: 'fixed',
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });
});
