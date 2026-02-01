// Search results page component with query from URL params.

import { useState, useMemo, useCallback } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useEnrichedSearch, type SearchResultWithRoute } from '@/hooks/useSearch.js';
import { Input } from '@/components/ui/Input.js';
import { Button } from '@/components/ui/Button.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { SearchResult } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Type filter options.
type SearchFilter = 'all' | 'message' | 'thread' | 'agent' | 'topic';

const filterOptions: { value: SearchFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'message', label: 'Messages' },
  { value: 'thread', label: 'Threads' },
  { value: 'agent', label: 'Agents' },
  { value: 'topic', label: 'Topics' },
];

// Type icons for results.
function TypeIcon({ type, className }: { type: SearchResult['type']; className?: string }) {
  switch (type) {
    case 'message':
      return (
        <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      );
    case 'thread':
      return (
        <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8h2a2 2 0 012 2v6a2 2 0 01-2 2h-2v4l-4-4H9a1.994 1.994 0 01-1.414-.586m0 0L11 14h4a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2v4l.586-.586z" />
        </svg>
      );
    case 'agent':
      return (
        <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      );
    case 'topic':
      return (
        <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
        </svg>
      );
    default:
      return null;
  }
}

// Highlight matching text in search results.
function HighlightedText({ text, query }: { text: string; query: string }) {
  if (!query.trim()) {
    return <span>{text}</span>;
  }

  // Split text into parts that match or don't match the query.
  const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  const parts = text.split(regex);

  return (
    <span>
      {parts.map((part, i) => {
        const isMatch = part.toLowerCase() === query.toLowerCase();
        return isMatch ? (
          <mark key={i} className="bg-yellow-200 text-gray-900">
            {part}
          </mark>
        ) : (
          <span key={i}>{part}</span>
        );
      })}
    </span>
  );
}

// Search result item component.
function SearchResultItem({
  result,
  query,
}: {
  result: SearchResultWithRoute;
  query: string;
}) {
  const typeColors = {
    message: 'text-blue-600 bg-blue-100',
    thread: 'text-purple-600 bg-purple-100',
    agent: 'text-green-600 bg-green-100',
    topic: 'text-orange-600 bg-orange-100',
  };

  const typeLabels = {
    message: 'Message',
    thread: 'Thread',
    agent: 'Agent',
    topic: 'Topic',
  };

  return (
    <Link
      to={result.route}
      className={cn(
        'block rounded-lg border border-gray-200 bg-white p-4',
        'hover:border-blue-300 hover:bg-blue-50/50',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        'transition-colors',
      )}
    >
      <div className="flex items-start gap-4">
        <div
          className={cn(
            'flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg',
            typeColors[result.type],
          )}
        >
          <TypeIcon type={result.type} />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-sm font-medium text-gray-900">
              <HighlightedText text={result.title} query={query} />
            </h3>
            <span
              className={cn(
                'flex-shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
                typeColors[result.type],
              )}
            >
              {typeLabels[result.type]}
            </span>
          </div>

          <p className="mt-1 line-clamp-2 text-sm text-gray-500">
            <HighlightedText text={result.snippet} query={query} />
          </p>

          <p className="mt-2 text-xs text-gray-400">
            {new Date(result.created_at).toLocaleDateString(undefined, {
              year: 'numeric',
              month: 'short',
              day: 'numeric',
            })}
          </p>
        </div>
      </div>
    </Link>
  );
}

// Empty state component.
function EmptyState({ query, hasFilter }: { query: string; hasFilter: boolean }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <svg
        className="h-16 w-16 text-gray-300"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={1.5}
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>
      <h3 className="mt-4 text-lg font-medium text-gray-900">No results found</h3>
      <p className="mt-2 max-w-sm text-sm text-gray-500">
        {query ? (
          <>
            No results found for "{query}".
            {hasFilter && ' Try removing the filter or searching for something else.'}
            {!hasFilter && ' Try searching for something else.'}
          </>
        ) : (
          'Enter a search term to find messages, threads, agents, and topics.'
        )}
      </p>
    </div>
  );
}

// Main SearchResultsPage component.
export default function SearchResultsPage() {
  const [searchParams, setSearchParams] = useSearchParams();

  // Get query and filter from URL.
  const initialQuery = searchParams.get('q') ?? '';
  const initialFilter = (searchParams.get('type') as SearchFilter) ?? 'all';

  // Local state for the search input.
  const [inputValue, setInputValue] = useState(initialQuery);
  const [filter, setFilter] = useState<SearchFilter>(initialFilter);

  // Use the search hook with the URL query.
  const { enrichedResults, isSearching, isError, error } = useEnrichedSearch(initialQuery);

  // Filter results based on selected type.
  const filteredResults = useMemo(() => {
    if (filter === 'all') {
      return enrichedResults;
    }
    return enrichedResults.filter((result) => result.type === filter);
  }, [enrichedResults, filter]);

  // Handle search form submission.
  const handleSearch = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      const trimmedQuery = inputValue.trim();
      if (trimmedQuery) {
        setSearchParams({ q: trimmedQuery, ...(filter !== 'all' ? { type: filter } : {}) });
      }
    },
    [inputValue, filter, setSearchParams],
  );

  // Handle filter change.
  const handleFilterChange = useCallback(
    (newFilter: SearchFilter) => {
      setFilter(newFilter);
      if (initialQuery) {
        setSearchParams({ q: initialQuery, ...(newFilter !== 'all' ? { type: newFilter } : {}) });
      }
    },
    [initialQuery, setSearchParams],
  );

  // Handle clear search.
  const handleClear = useCallback(() => {
    setInputValue('');
    setFilter('all');
    setSearchParams({});
  }, [setSearchParams]);

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      {/* Header. */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Search Results</h1>
        {initialQuery && (
          <p className="mt-1 text-sm text-gray-500">
            {filteredResults.length} result{filteredResults.length !== 1 ? 's' : ''} for "{initialQuery}"
            {filter !== 'all' && ` in ${filter}s`}
          </p>
        )}
      </div>

      {/* Search form. */}
      <form onSubmit={handleSearch} className="flex gap-3">
        <div className="relative flex-1">
          <Input
            type="text"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder="Search messages, threads, agents..."
            className="w-full pr-10"
          />
          {inputValue && (
            <button
              type="button"
              onClick={() => setInputValue('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
            >
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
        <Button type="submit" disabled={!inputValue.trim()}>
          Search
        </Button>
      </form>

      {/* Filter tabs. */}
      <div className="flex gap-2 border-b border-gray-200 pb-3">
        {filterOptions.map((option) => {
          const count =
            option.value === 'all'
              ? enrichedResults.length
              : enrichedResults.filter((r) => r.type === option.value).length;

          return (
            <button
              key={option.value}
              onClick={() => handleFilterChange(option.value)}
              className={cn(
                'rounded-full px-4 py-1.5 text-sm font-medium transition-colors',
                filter === option.value
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200',
              )}
            >
              {option.label}
              {count > 0 && (
                <span
                  className={cn(
                    'ml-1.5 inline-flex h-5 min-w-[20px] items-center justify-center rounded-full text-xs',
                    filter === option.value ? 'bg-blue-500 text-white' : 'bg-gray-200 text-gray-600',
                  )}
                >
                  {count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Results. */}
      <div className="space-y-4">
        {isSearching && (
          <div className="flex items-center justify-center py-12">
            <Spinner size="lg" variant="primary" label="Searching..." />
          </div>
        )}

        {isError && (
          <div className="rounded-lg bg-red-50 p-4 text-center">
            <p className="text-sm text-red-600">
              {error instanceof Error ? error.message : 'Failed to search. Please try again.'}
            </p>
            <Button variant="outline" size="sm" className="mt-2" onClick={handleClear}>
              Clear search
            </Button>
          </div>
        )}

        {!isSearching && !isError && filteredResults.length === 0 && (
          <EmptyState query={initialQuery} hasFilter={filter !== 'all'} />
        )}

        {!isSearching && !isError && filteredResults.length > 0 && (
          <div className="space-y-3">
            {filteredResults.map((result) => (
              <SearchResultItem
                key={`${result.type}-${result.id}`}
                result={result}
                query={initialQuery}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
