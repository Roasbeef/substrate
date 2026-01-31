// Unit tests for useSearch hook.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  useSearch,
  useAutocomplete,
  useDebounce,
  useEnrichedSearch,
  enrichSearchResult,
  searchKeys,
  searchQueryOptions,
  autocompleteQueryOptions,
} from '@/hooks/useSearch.js';
import * as searchApi from '@/api/search.js';
import type { ReactNode } from 'react';
import type { SearchResult, AutocompleteRecipient, APIResponse } from '@/types/api.js';

// Mock the search API module.
vi.mock('@/api/search.js', () => ({
  search: vi.fn(),
  autocompleteRecipients: vi.fn(),
}));

// Test wrapper with QueryClient.
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe('searchKeys', () => {
  it('generates correct all key', () => {
    expect(searchKeys.all).toEqual(['search']);
  });

  it('generates correct query key', () => {
    expect(searchKeys.query('test')).toEqual(['search', 'query', 'test']);
  });

  it('generates correct autocomplete key', () => {
    expect(searchKeys.autocomplete('test')).toEqual(['search', 'autocomplete', 'test']);
  });
});

describe('searchQueryOptions', () => {
  it('returns options with correct query key', () => {
    const options = searchQueryOptions('test query');
    expect(options.queryKey).toEqual(['search', 'query', 'test query']);
  });

  it('is enabled for queries with 2+ characters', () => {
    const options = searchQueryOptions('ab');
    expect(options.enabled).toBe(true);
  });

  it('is disabled for queries with less than 2 characters', () => {
    const options = searchQueryOptions('a');
    expect(options.enabled).toBe(false);
  });

  it('is disabled for empty queries', () => {
    const options = searchQueryOptions('');
    expect(options.enabled).toBe(false);
  });

  it('is disabled for whitespace-only queries', () => {
    const options = searchQueryOptions('   ');
    expect(options.enabled).toBe(false);
  });
});

describe('autocompleteQueryOptions', () => {
  it('returns options with correct query key', () => {
    const options = autocompleteQueryOptions('test');
    expect(options.queryKey).toEqual(['search', 'autocomplete', 'test']);
  });

  it('is enabled for queries with 1+ characters', () => {
    const options = autocompleteQueryOptions('a');
    expect(options.enabled).toBe(true);
  });

  it('is disabled for empty queries', () => {
    const options = autocompleteQueryOptions('');
    expect(options.enabled).toBe(false);
  });
});

describe('useDebounce', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns initial value immediately', () => {
    const { result } = renderHook(() => useDebounce('initial', 300));
    expect(result.current).toBe('initial');
  });

  it('debounces value changes', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: 'first' } },
    );

    expect(result.current).toBe('first');

    rerender({ value: 'second' });
    expect(result.current).toBe('first');

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe('second');
  });

  it('resets timer on subsequent changes', () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebounce(value, 300),
      { initialProps: { value: 'first' } },
    );

    rerender({ value: 'second' });
    act(() => {
      vi.advanceTimersByTime(200);
    });

    rerender({ value: 'third' });
    act(() => {
      vi.advanceTimersByTime(200);
    });

    // Still showing first value because timer keeps resetting.
    expect(result.current).toBe('first');

    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Now shows third because 300ms passed since last change.
    expect(result.current).toBe('third');
  });
});

describe('useSearch', () => {
  const mockResults: SearchResult[] = [
    { type: 'message', id: 1, title: 'Test Message', snippet: 'Test content', created_at: '2024-01-01' },
    { type: 'thread', id: 2, title: 'Test Thread', snippet: 'Thread content', created_at: '2024-01-01' },
  ];

  const mockResponse: APIResponse<SearchResult[]> = {
    data: mockResults,
  };

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    vi.mocked(searchApi.search).mockResolvedValue(mockResponse);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not search for empty query', async () => {
    const { result } = renderHook(() => useSearch(''), { wrapper: createWrapper() });

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(result.current.results).toEqual([]);
    expect(searchApi.search).not.toHaveBeenCalled();
  });

  it('does not search for query shorter than 2 characters', async () => {
    const { result } = renderHook(() => useSearch('a'), { wrapper: createWrapper() });

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(result.current.results).toEqual([]);
    expect(searchApi.search).not.toHaveBeenCalled();
  });

  it('searches after debounce delay', async () => {
    vi.useRealTimers();
    const { result } = renderHook(() => useSearch('test', 50), { wrapper: createWrapper() });

    // Wait for debounce and query.
    await waitFor(() => {
      expect(searchApi.search).toHaveBeenCalledWith('test', expect.any(AbortSignal));
    }, { timeout: 500 });

    await waitFor(() => {
      expect(result.current.results).toEqual(mockResults);
    });
  });

  it('returns isSearching true while typing', () => {
    const { result, rerender } = renderHook(
      ({ query }) => useSearch(query),
      { initialProps: { query: '' }, wrapper: createWrapper() },
    );

    rerender({ query: 'te' });

    // Query differs from debounced query, so isSearching is true.
    expect(result.current.isSearching).toBe(true);
  });

  it('returns debouncedQuery', () => {
    const { result, rerender } = renderHook(
      ({ query }) => useSearch(query),
      { initialProps: { query: 'initial' }, wrapper: createWrapper() },
    );

    expect(result.current.debouncedQuery).toBe('initial');

    rerender({ query: 'updated' });

    // Debounced query is still the old value.
    expect(result.current.debouncedQuery).toBe('initial');

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current.debouncedQuery).toBe('updated');
  });
});

describe('useAutocomplete', () => {
  const mockSuggestions: AutocompleteRecipient[] = [
    { id: 1, name: 'Agent One', status: 'active' },
    { id: 2, name: 'Agent Two', status: 'idle' },
  ];

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    vi.mocked(searchApi.autocompleteRecipients).mockResolvedValue(mockSuggestions);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not search for empty query', async () => {
    const { result } = renderHook(() => useAutocomplete(''), { wrapper: createWrapper() });

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current.suggestions).toEqual([]);
    expect(searchApi.autocompleteRecipients).not.toHaveBeenCalled();
  });

  it('searches after debounce delay', async () => {
    vi.useRealTimers();
    const { result } = renderHook(() => useAutocomplete('ag', 50), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(searchApi.autocompleteRecipients).toHaveBeenCalledWith('ag', expect.any(AbortSignal));
    }, { timeout: 500 });

    await waitFor(() => {
      expect(result.current.suggestions).toEqual(mockSuggestions);
    });
  });

  it('has shorter default debounce than search', () => {
    // Autocomplete uses 200ms by default.
    const { result } = renderHook(() => useAutocomplete('a'), { wrapper: createWrapper() });

    act(() => {
      vi.advanceTimersByTime(200);
    });

    expect(searchApi.autocompleteRecipients).toHaveBeenCalled();
  });
});

describe('enrichSearchResult', () => {
  it('adds route for message type', () => {
    const result: SearchResult = {
      type: 'message',
      id: 123,
      title: 'Test',
      snippet: 'Test',
      created_at: '2024-01-01',
    };

    const enriched = enrichSearchResult(result);
    expect(enriched.route).toBe('/inbox/thread/123');
  });

  it('adds route for thread type', () => {
    const result: SearchResult = {
      type: 'thread',
      id: 456,
      title: 'Test',
      snippet: 'Test',
      created_at: '2024-01-01',
    };

    const enriched = enrichSearchResult(result);
    expect(enriched.route).toBe('/inbox/thread/456');
  });

  it('adds route for agent type', () => {
    const result: SearchResult = {
      type: 'agent',
      id: 789,
      title: 'Test',
      snippet: 'Test',
      created_at: '2024-01-01',
    };

    const enriched = enrichSearchResult(result);
    expect(enriched.route).toBe('/agents/789');
  });

  it('adds route for topic type', () => {
    const result: SearchResult = {
      type: 'topic',
      id: 101,
      title: 'Test',
      snippet: 'Test',
      created_at: '2024-01-01',
    };

    const enriched = enrichSearchResult(result);
    expect(enriched.route).toBe('/topics/101');
  });

  it('preserves original properties', () => {
    const result: SearchResult = {
      type: 'message',
      id: 123,
      title: 'Original Title',
      snippet: 'Original Snippet',
      created_at: '2024-01-01',
    };

    const enriched = enrichSearchResult(result);
    expect(enriched.type).toBe('message');
    expect(enriched.id).toBe(123);
    expect(enriched.title).toBe('Original Title');
    expect(enriched.snippet).toBe('Original Snippet');
  });
});

describe('useEnrichedSearch', () => {
  const mockResults: SearchResult[] = [
    { type: 'message', id: 1, title: 'Message', snippet: 'Content', created_at: '2024-01-01' },
    { type: 'agent', id: 2, title: 'Agent', snippet: 'Content', created_at: '2024-01-01' },
  ];

  const mockResponse: APIResponse<SearchResult[]> = {
    data: mockResults,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(searchApi.search).mockResolvedValue(mockResponse);
  });

  it('returns enriched results with routes', async () => {
    const { result } = renderHook(() => useEnrichedSearch('test', 50), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.enrichedResults.length).toBe(2);
    }, { timeout: 500 });

    expect(result.current.enrichedResults[0]?.route).toBe('/inbox/thread/1');
    expect(result.current.enrichedResults[1]?.route).toBe('/agents/2');
  });

  it('memoizes enriched results', async () => {
    const { result, rerender } = renderHook(
      ({ query }) => useEnrichedSearch(query, 50),
      { initialProps: { query: 'test' }, wrapper: createWrapper() },
    );

    await waitFor(() => {
      expect(result.current.enrichedResults.length).toBe(2);
    }, { timeout: 500 });

    const firstResults = result.current.enrichedResults;

    // Rerender with same query should return same array reference.
    rerender({ query: 'test' });

    expect(result.current.enrichedResults).toBe(firstResults);
  });
});
