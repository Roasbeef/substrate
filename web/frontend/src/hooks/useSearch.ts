// Custom hooks for search functionality with debouncing.

import { useQuery, queryOptions } from '@tanstack/react-query';
import { useState, useEffect, useMemo } from 'react';
import { search, autocompleteRecipients } from '@/api/search.js';
import type { SearchResult, AutocompleteRecipient, APIResponse } from '@/types/api.js';

// Search query keys.
export const searchKeys = {
  all: ['search'] as const,
  query: (q: string) => [...searchKeys.all, 'query', q] as const,
  autocomplete: (q: string) => [...searchKeys.all, 'autocomplete', q] as const,
};

// Search query options.
export function searchQueryOptions(query: string) {
  return queryOptions({
    queryKey: searchKeys.query(query),
    queryFn: ({ signal }) => search(query, signal),
    enabled: query.trim().length >= 2,
    staleTime: 30_000, // 30 seconds.
    gcTime: 60_000, // 1 minute.
  });
}

// Autocomplete query options.
export function autocompleteQueryOptions(query: string) {
  return queryOptions({
    queryKey: searchKeys.autocomplete(query),
    queryFn: ({ signal }) => autocompleteRecipients(query, signal),
    enabled: query.trim().length >= 1,
    staleTime: 30_000,
    gcTime: 60_000,
  });
}

// Hook for debouncing input value.
export function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(timer);
    };
  }, [value, delay]);

  return debouncedValue;
}

// Hook for search with debouncing.
export function useSearch(query: string, debounceMs = 300) {
  const debouncedQuery = useDebounce(query, debounceMs);

  const queryResult = useQuery(searchQueryOptions(debouncedQuery));

  return {
    ...queryResult,
    // Override data to provide a convenient accessor.
    results: queryResult.data?.data ?? [],
    // Show loading when typing or when query is pending.
    isSearching: queryResult.isFetching || (query !== debouncedQuery && query.trim().length >= 2),
    debouncedQuery,
  };
}

// Hook for recipient autocomplete with debouncing.
export function useAutocomplete(query: string, debounceMs = 200) {
  const debouncedQuery = useDebounce(query, debounceMs);

  const queryResult = useQuery(autocompleteQueryOptions(debouncedQuery));

  return {
    ...queryResult,
    suggestions: queryResult.data ?? [],
    isSearching: queryResult.isFetching || (query !== debouncedQuery && query.trim().length >= 1),
    debouncedQuery,
  };
}

// Type for search result item with routing information.
export interface SearchResultWithRoute extends SearchResult {
  route: string;
}

// Add route information to search results.
export function enrichSearchResult(result: SearchResult): SearchResultWithRoute {
  switch (result.type) {
    case 'message':
      return { ...result, route: `/inbox/thread/${result.id}` };
    case 'thread':
      return { ...result, route: `/inbox/thread/${result.id}` };
    case 'agent':
      return { ...result, route: `/agents/${result.id}` };
    case 'topic':
      return { ...result, route: `/topics/${result.id}` };
    default:
      return { ...result, route: '/' };
  }
}

// Hook that returns enriched search results with routes.
export function useEnrichedSearch(query: string, debounceMs = 300) {
  const searchResult = useSearch(query, debounceMs);

  const enrichedResults = useMemo(
    () => searchResult.results.map(enrichSearchResult),
    [searchResult.results],
  );

  return {
    ...searchResult,
    enrichedResults,
  };
}
