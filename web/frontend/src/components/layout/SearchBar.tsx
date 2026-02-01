// SearchBar component with debounced search and results dropdown.

import {
  Fragment,
  useState,
  useRef,
  useEffect,
  useCallback,
  type KeyboardEvent,
} from 'react';
import { useNavigate } from 'react-router-dom';
import { Dialog, DialogPanel, Transition, TransitionChild } from '@headlessui/react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore } from '@/stores/ui.js';
import { useEnrichedSearch, type SearchResultWithRoute } from '@/hooks/useSearch.js';
import type { SearchResult } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Search icon.
function SearchIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
      />
    </svg>
  );
}

// Close icon.
function CloseIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M6 18L18 6M6 6l12 12"
      />
    </svg>
  );
}

// Spinner icon.
function SpinnerIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5 animate-spin', className)}
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
  );
}

// Type icons for search results.
function MessageIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

function ThreadIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
      />
    </svg>
  );
}

function AgentIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  );
}

function TopicIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z"
      />
    </svg>
  );
}

// Get icon for result type.
function getResultTypeIcon(type: SearchResult['type']) {
  switch (type) {
    case 'message':
      return MessageIcon;
    case 'thread':
      return ThreadIcon;
    case 'agent':
      return AgentIcon;
    case 'topic':
      return TopicIcon;
    default:
      return MessageIcon;
  }
}

// Get label for result type.
function getResultTypeLabel(type: SearchResult['type']): string {
  switch (type) {
    case 'message':
      return 'Message';
    case 'thread':
      return 'Thread';
    case 'agent':
      return 'Agent';
    case 'topic':
      return 'Topic';
    default:
      return 'Result';
  }
}

// Search result item component.
interface SearchResultItemProps {
  result: SearchResultWithRoute;
  isSelected: boolean;
  onSelect: () => void;
}

function SearchResultItem({ result, isSelected, onSelect }: SearchResultItemProps) {
  const Icon = getResultTypeIcon(result.type);

  return (
    <button
      type="button"
      className={cn(
        'flex w-full items-start gap-3 px-4 py-3 text-left',
        isSelected ? 'bg-blue-50' : 'hover:bg-gray-50',
      )}
      onClick={onSelect}
      role="option"
      aria-selected={isSelected}
    >
      <div
        className={cn(
          'mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg',
          result.type === 'message' ? 'bg-blue-100 text-blue-600' : '',
          result.type === 'thread' ? 'bg-purple-100 text-purple-600' : '',
          result.type === 'agent' ? 'bg-green-100 text-green-600' : '',
          result.type === 'topic' ? 'bg-yellow-100 text-yellow-600' : '',
        )}
      >
        <Icon />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium text-gray-900">{result.title}</span>
          <span className="flex-shrink-0 text-xs text-gray-400">
            {getResultTypeLabel(result.type)}
          </span>
        </div>
        <p className="mt-0.5 truncate text-sm text-gray-500">{result.snippet}</p>
      </div>
    </button>
  );
}

// Props for SearchBar component.
export interface SearchBarProps {
  /** Additional class name. */
  className?: string;
  /** Placeholder text. */
  placeholder?: string;
  /** Auto-focus input on open. */
  autoFocus?: boolean;
}

// Global search bar modal component.
export function SearchBar({
  className,
  placeholder = 'Search messages, threads, agents...',
  autoFocus = true,
}: SearchBarProps) {
  const navigate = useNavigate();
  const { searchOpen, searchQuery, setSearchQuery, closeSearch } = useUIStore();
  const inputRef = useRef<HTMLInputElement>(null);
  const [selectedIndex, setSelectedIndex] = useState(0);

  // Use enriched search for results with routes.
  const { enrichedResults, isSearching, debouncedQuery } = useEnrichedSearch(searchQuery);

  // Reset selection when results change.
  useEffect(() => {
    setSelectedIndex(0);
  }, [enrichedResults]);

  // Focus input when dialog opens.
  useEffect(() => {
    if (searchOpen && autoFocus) {
      // Small delay to ensure dialog is rendered.
      const timer = setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    }
    return;
  }, [searchOpen, autoFocus]);

  // Handle result selection.
  const handleSelectResult = useCallback(
    (result: SearchResultWithRoute) => {
      closeSearch();
      navigate(result.route);
    },
    [closeSearch, navigate],
  );

  // Handle keyboard navigation.
  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLInputElement>) => {
      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault();
          setSelectedIndex((prev) =>
            Math.min(prev + 1, enrichedResults.length - 1),
          );
          break;
        case 'ArrowUp':
          e.preventDefault();
          setSelectedIndex((prev) => Math.max(prev - 1, 0));
          break;
        case 'Enter': {
          e.preventDefault();
          const selectedResult = enrichedResults[selectedIndex];
          if (selectedResult) {
            handleSelectResult(selectedResult);
          }
          break;
        }
        case 'Escape':
          e.preventDefault();
          closeSearch();
          break;
      }
    },
    [enrichedResults, selectedIndex, handleSelectResult, closeSearch],
  );

  // Clear search input.
  const handleClear = useCallback(() => {
    setSearchQuery('');
    inputRef.current?.focus();
  }, [setSearchQuery]);

  return (
    <Transition show={searchOpen} as={Fragment}>
      <Dialog onClose={closeSearch} className="relative z-50">
        {/* Backdrop. */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-200"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-150"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div
            className="fixed inset-0 bg-gray-900/50 backdrop-blur-sm"
            aria-hidden="true"
          />
        </TransitionChild>

        {/* Dialog panel. */}
        <div className="fixed inset-0 overflow-y-auto p-4 pt-[15vh]">
          <TransitionChild
            as={Fragment}
            enter="ease-out duration-200"
            enterFrom="opacity-0 scale-95"
            enterTo="opacity-100 scale-100"
            leave="ease-in duration-150"
            leaveFrom="opacity-100 scale-100"
            leaveTo="opacity-0 scale-95"
          >
            <DialogPanel
              className={cn(
                'mx-auto max-w-2xl overflow-hidden rounded-xl bg-white shadow-2xl ring-1 ring-black/5',
                className,
              )}
            >
              {/* Search input. */}
              <div className="relative flex items-center border-b border-gray-200">
                <SearchIcon className="pointer-events-none absolute left-4 text-gray-400" />
                <input
                  ref={inputRef}
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder={placeholder}
                  className={cn(
                    'w-full border-0 bg-transparent py-4 pl-12 pr-12',
                    'text-gray-900 placeholder:text-gray-400',
                    'focus:outline-none focus:ring-0',
                  )}
                  aria-label="Search"
                  aria-autocomplete="list"
                  aria-controls="search-results"
                  role="combobox"
                  aria-expanded={enrichedResults.length > 0}
                />
                {isSearching ? (
                  <SpinnerIcon className="absolute right-4 text-gray-400" />
                ) : searchQuery ? (
                  <button
                    type="button"
                    onClick={handleClear}
                    className="absolute right-4 rounded p-1 text-gray-400 hover:text-gray-600"
                    aria-label="Clear search"
                  >
                    <CloseIcon className="h-4 w-4" />
                  </button>
                ) : null}
              </div>

              {/* Results section. */}
              <div
                id="search-results"
                role="listbox"
                className="max-h-[60vh] overflow-y-auto"
              >
                {/* Empty state - no query. */}
                {!searchQuery.trim() ? (
                  <div className="px-4 py-8 text-center">
                    <SearchIcon className="mx-auto h-12 w-12 text-gray-300" />
                    <p className="mt-2 text-sm text-gray-500">
                      Type to search messages, threads, agents, and topics
                    </p>
                    <p className="mt-1 text-xs text-gray-400">
                      Press <kbd className="rounded bg-gray-100 px-1">↑</kbd>{' '}
                      <kbd className="rounded bg-gray-100 px-1">↓</kbd> to
                      navigate, <kbd className="rounded bg-gray-100 px-1">Enter</kbd> to
                      select, <kbd className="rounded bg-gray-100 px-1">Esc</kbd> to
                      close
                    </p>
                  </div>
                ) : searchQuery.trim().length < 2 ? (
                  // Query too short.
                  <div className="px-4 py-8 text-center text-sm text-gray-500">
                    Type at least 2 characters to search
                  </div>
                ) : isSearching && enrichedResults.length === 0 ? (
                  // Loading state.
                  <div className="px-4 py-8 text-center text-sm text-gray-500">
                    Searching...
                  </div>
                ) : enrichedResults.length === 0 && debouncedQuery.trim().length >= 2 ? (
                  // No results.
                  <div className="px-4 py-8 text-center">
                    <p className="text-sm text-gray-500">
                      No results found for &ldquo;{debouncedQuery}&rdquo;
                    </p>
                    <p className="mt-1 text-xs text-gray-400">
                      Try adjusting your search terms
                    </p>
                  </div>
                ) : (
                  // Results list.
                  <div className="py-2">
                    {enrichedResults.map((result, index) => (
                      <SearchResultItem
                        key={`${result.type}-${result.id}`}
                        result={result}
                        isSelected={index === selectedIndex}
                        onSelect={() => handleSelectResult(result)}
                      />
                    ))}
                  </div>
                )}
              </div>

              {/* Footer with keyboard hints. */}
              {enrichedResults.length > 0 ? (
                <div className="flex items-center justify-between border-t border-gray-200 bg-gray-50 px-4 py-2 text-xs text-gray-500">
                  <span>{enrichedResults.length} results</span>
                  <span>
                    <kbd className="rounded bg-gray-200 px-1">↑↓</kbd> Navigate{' '}
                    <kbd className="ml-2 rounded bg-gray-200 px-1">Enter</kbd> Select{' '}
                    <kbd className="ml-2 rounded bg-gray-200 px-1">Esc</kbd> Close
                  </span>
                </div>
              ) : null}
            </DialogPanel>
          </TransitionChild>
        </div>
      </Dialog>
    </Transition>
  );
}

// Inline search input for embedding in layouts.
export interface InlineSearchInputProps {
  /** Value of the input. */
  value: string;
  /** Change handler. */
  onChange: (value: string) => void;
  /** Placeholder text. */
  placeholder?: string;
  /** Additional class name. */
  className?: string;
  /** Whether input is disabled. */
  disabled?: boolean;
  /** Loading state. */
  isLoading?: boolean;
}

export function InlineSearchInput({
  value,
  onChange,
  placeholder = 'Search...',
  className,
  disabled = false,
  isLoading = false,
}: InlineSearchInputProps) {
  return (
    <div className={cn('relative', className)}>
      <SearchIcon className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        className={cn(
          'w-full rounded-lg border border-gray-200 bg-white py-2 pl-10 pr-10',
          'text-sm text-gray-900 placeholder:text-gray-400',
          'focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
          disabled ? 'cursor-not-allowed opacity-50' : '',
        )}
      />
      {isLoading ? (
        <SpinnerIcon className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400" />
      ) : value ? (
        <button
          type="button"
          onClick={() => onChange('')}
          className="absolute right-3 top-1/2 -translate-y-1/2 rounded text-gray-400 hover:text-gray-600"
          aria-label="Clear"
        >
          <CloseIcon className="h-4 w-4" />
        </button>
      ) : null}
    </div>
  );
}

// Compact search trigger button (for header).
export interface SearchTriggerProps {
  /** Additional class name. */
  className?: string;
}

export function SearchTrigger({ className }: SearchTriggerProps) {
  const toggleSearch = useUIStore((state) => state.toggleSearch);

  return (
    <button
      type="button"
      onClick={toggleSearch}
      className={cn(
        'flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2',
        'text-sm text-gray-500 hover:border-gray-300 hover:bg-gray-100',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        'min-w-[200px] md:min-w-[300px]',
        className,
      )}
    >
      <SearchIcon className="text-gray-400" />
      <span className="flex-1 text-left">Search...</span>
      <kbd className="hidden rounded bg-gray-200 px-1.5 py-0.5 text-xs font-medium text-gray-500 md:inline-block">
        ⌘K
      </kbd>
    </button>
  );
}

// Export the SearchResultItem for use in other components.
export { SearchResultItem, getResultTypeIcon, getResultTypeLabel };
