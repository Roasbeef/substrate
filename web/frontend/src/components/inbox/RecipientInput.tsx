// RecipientInput component - autocomplete input for selecting message recipients.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import type { AutocompleteRecipient, AgentStatusType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Status indicator dot.
function StatusDot({ status }: { status?: AgentStatusType }) {
  const colors: Record<AgentStatusType, string> = {
    active: 'bg-green-400',
    busy: 'bg-yellow-400',
    idle: 'bg-gray-400',
    offline: 'bg-gray-300',
  };

  return (
    <span
      className={cn(
        'inline-block h-2 w-2 rounded-full',
        status ? colors[status] : 'bg-gray-300',
      )}
    />
  );
}

// Selected recipient chip.
interface RecipientChipProps {
  recipient: AutocompleteRecipient;
  onRemove: () => void;
  disabled?: boolean;
}

function RecipientChip({ recipient, onRemove, disabled }: RecipientChipProps) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-blue-100 py-0.5 pl-2 pr-1 text-sm text-blue-700">
      {recipient.name}
      <button
        type="button"
        onClick={onRemove}
        disabled={disabled}
        className={cn(
          'rounded-full p-0.5 hover:bg-blue-200',
          'focus:outline-none focus:ring-2 focus:ring-blue-500',
          disabled ? 'cursor-not-allowed opacity-50' : '',
        )}
        aria-label={`Remove ${recipient.name}`}
      >
        <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </span>
  );
}

// Autocomplete suggestion item.
interface SuggestionItemProps {
  recipient: AutocompleteRecipient;
  isHighlighted: boolean;
  onSelect: () => void;
}

function SuggestionItem({
  recipient,
  isHighlighted,
  onSelect,
}: SuggestionItemProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'flex w-full items-center gap-3 px-3 py-2 text-left',
        isHighlighted ? 'bg-blue-50 text-blue-700' : 'hover:bg-gray-50',
      )}
      role="option"
      aria-selected={isHighlighted}
    >
      <Avatar name={recipient.name} size="sm" />
      <span className="flex-1 truncate font-medium">{recipient.name}</span>
      <StatusDot status={recipient.status} />
    </button>
  );
}

// Props for RecipientInput component.
export interface RecipientInputProps {
  /** Currently selected recipients. */
  value: AutocompleteRecipient[];
  /** Handler for selection changes. */
  onChange: (recipients: AutocompleteRecipient[]) => void;
  /** Function to fetch suggestions based on query. */
  onSearch: (query: string) => Promise<AutocompleteRecipient[]>;
  /** Placeholder text. */
  placeholder?: string;
  /** Whether the input is disabled. */
  disabled?: boolean;
  /** Error message. */
  error?: string;
  /** Label for the input. */
  label?: string;
  /** Additional class name. */
  className?: string;
}

export function RecipientInput({
  value,
  onChange,
  onSearch,
  placeholder = 'Add recipients...',
  disabled = false,
  error,
  label,
  className,
}: RecipientInputProps) {
  const [query, setQuery] = useState('');
  const [suggestions, setSuggestions] = useState<AutocompleteRecipient[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(false);

  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Debounce search.
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>();

  // Filter out already selected recipients from suggestions.
  const filteredSuggestions = useMemo(
    () => suggestions.filter((s) => !value.some((v) => v.id === s.id)),
    [suggestions, value],
  );

  // Handle search.
  useEffect(() => {
    if (!query.trim()) {
      setSuggestions([]);
      setIsOpen(false);
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      setIsLoading(true);
      void onSearch(query)
        .then((results) => {
          setSuggestions(results);
          setIsOpen(results.length > 0);
          setHighlightedIndex(0);
        })
        .finally(() => {
          setIsLoading(false);
        });
    }, 200);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [query, onSearch]);

  // Handle outside click.
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Handle selecting a suggestion.
  const handleSelect = useCallback(
    (recipient: AutocompleteRecipient) => {
      onChange([...value, recipient]);
      setQuery('');
      setIsOpen(false);
      inputRef.current?.focus();
    },
    [onChange, value],
  );

  // Handle removing a recipient.
  const handleRemove = useCallback(
    (id: number) => {
      onChange(value.filter((r) => r.id !== id));
    },
    [onChange, value],
  );

  // Handle keyboard navigation.
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!isOpen) {
        if (e.key === 'Backspace' && !query && value.length > 0) {
          // Remove last recipient.
          onChange(value.slice(0, -1));
        }
        return;
      }

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault();
          setHighlightedIndex((prev) =>
            prev < filteredSuggestions.length - 1 ? prev + 1 : prev,
          );
          break;
        case 'ArrowUp':
          e.preventDefault();
          setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : prev));
          break;
        case 'Enter':
          e.preventDefault();
          if (filteredSuggestions[highlightedIndex]) {
            handleSelect(filteredSuggestions[highlightedIndex]);
          }
          break;
        case 'Escape':
          setIsOpen(false);
          break;
      }
    },
    [isOpen, query, value, filteredSuggestions, highlightedIndex, onChange, handleSelect],
  );

  const inputId = useMemo(
    () => `recipient-input-${Math.random().toString(36).slice(2)}`,
    [],
  );

  return (
    <div className={cn('relative', className)} ref={containerRef}>
      {label ? (
        <label
          htmlFor={inputId}
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          {label}
        </label>
      ) : null}

      <div
        className={cn(
          'flex flex-wrap items-center gap-2 rounded-md border bg-white px-3 py-2',
          'focus-within:border-blue-500 focus-within:ring-2 focus-within:ring-blue-100',
          error
            ? 'border-red-300 focus-within:border-red-500 focus-within:ring-red-100'
            : 'border-gray-300',
          disabled ? 'cursor-not-allowed bg-gray-50' : '',
        )}
      >
        {/* Selected recipients. */}
        {value.map((recipient) => (
          <RecipientChip
            key={recipient.id}
            recipient={recipient}
            onRemove={() => handleRemove(recipient.id)}
            disabled={disabled}
          />
        ))}

        {/* Input. */}
        <input
          ref={inputRef}
          id={inputId}
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          onFocus={() => query.trim() && setIsOpen(true)}
          placeholder={value.length === 0 ? placeholder : ''}
          disabled={disabled}
          className={cn(
            'min-w-[120px] flex-1 border-none bg-transparent p-0 text-sm',
            'placeholder:text-gray-400 focus:outline-none focus:ring-0',
          )}
          role="combobox"
          aria-expanded={isOpen}
          aria-haspopup="listbox"
          aria-autocomplete="list"
        />

        {/* Loading indicator. */}
        {isLoading ? (
          <svg
            className="h-4 w-4 animate-spin text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
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
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        ) : null}
      </div>

      {/* Error message. */}
      {error ? (
        <p className="mt-1 text-sm text-red-600">{error}</p>
      ) : null}

      {/* Suggestions dropdown. */}
      {isOpen && filteredSuggestions.length > 0 ? (
        <div
          className="absolute left-0 right-0 z-10 mt-1 max-h-60 overflow-auto rounded-md border border-gray-200 bg-white py-1 shadow-lg"
          role="listbox"
        >
          {filteredSuggestions.map((suggestion, index) => (
            <SuggestionItem
              key={suggestion.id}
              recipient={suggestion}
              isHighlighted={index === highlightedIndex}
              onSelect={() => handleSelect(suggestion)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
