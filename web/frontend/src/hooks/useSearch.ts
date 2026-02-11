// Custom hooks for search functionality with debouncing.

import { useQuery, queryOptions } from '@tanstack/react-query';
import { useState, useEffect, useMemo } from 'react';
import { search, autocompleteRecipients } from '@/api/search.js';
import type { AutocompleteRecipient, SearchResult } from '@/types/api.js';
import { getAgentContext } from '@/lib/utils.js';

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
      // For messages, use thread_id if available, otherwise fall back to id.
      // The thread view route is /thread/:threadId (not /inbox/thread/).
      if (result.thread_id) {
        return { ...result, route: `/thread/${result.thread_id}` };
      }
      return { ...result, route: `/thread/${result.id}` };
    case 'thread':
      return { ...result, route: `/thread/${result.id}` };
    case 'agent':
      return { ...result, route: `/agents/${result.id}` };
    case 'topic':
      return { ...result, route: `/topics/${result.id}` };
    default:
      return { ...result, route: '/' };
  }
}

// Convert an autocomplete recipient to a search result with route.
function recipientToSearchResult(
  recipient: AutocompleteRecipient,
): SearchResultWithRoute {
  const context = getAgentContext({
    name: recipient.name,
    project_key: recipient.project_key,
    git_branch: recipient.git_branch,
  });

  return {
    type: 'agent',
    id: recipient.id,
    title: recipient.name,
    snippet: context ?? '',
    created_at: new Date().toISOString(),
    route: `/agents/${recipient.id}`,
  };
}

// Hook that returns enriched search results with routes.
// Merges message search results with agent autocomplete results.
export function useEnrichedSearch(query: string, debounceMs = 300) {
  const searchResult = useSearch(query, debounceMs);
  const autocompleteResult = useAutocomplete(query, debounceMs);

  const enrichedResults = useMemo(() => {
    const messageResults = searchResult.results.map(enrichSearchResult);
    const agentResults = (autocompleteResult.suggestions ?? []).map(
      recipientToSearchResult,
    );

    // Show agents first, then messages.
    return [...agentResults, ...messageResults];
  }, [searchResult.results, autocompleteResult.suggestions]);

  return {
    ...searchResult,
    enrichedResults,
    isSearching: searchResult.isSearching || autocompleteResult.isSearching,
  };
}
