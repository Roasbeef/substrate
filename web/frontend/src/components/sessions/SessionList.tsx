// SessionList component - table displaying sessions with filters.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { SessionRow, CompactSessionRow } from './SessionRow.js';
// import { Spinner } from '@/components/ui/Spinner.js';
import type { Session, SessionStatus } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Filter tabs.
type FilterTab = 'all' | SessionStatus;

// Props for SessionList.
export interface SessionListProps {
  /** List of sessions to display. */
  sessions?: Session[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Error state. */
  error?: Error | null;
  /** Handler for clicking a session. */
  onSessionClick?: (sessionId: number) => void;
  /** Handler for retry when error occurs. */
  onRetry?: () => void;
  /** Selected session ID. */
  selectedSessionId?: number;
  /** Show filter tabs. */
  showFilters?: boolean;
  /** External filter value (controlled mode). */
  filter?: FilterTab;
  /** Handler for filter change (controlled mode). */
  onFilterChange?: (filter: FilterTab) => void;
  /** Additional class name. */
  className?: string;
}

// Filter tab button.
function FilterTabButton({
  label,
  count,
  isActive,
  onClick,
}: {
  label: string;
  count?: number;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
        isActive
          ? 'bg-gray-900 text-white'
          : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
      )}
    >
      {label}
      {count !== undefined ? (
        <span
          className={cn(
            'ml-1.5',
            isActive ? 'text-gray-300' : 'text-gray-400',
          )}
        >
          ({count})
        </span>
      ) : null}
    </button>
  );
}

// Loading skeleton.
function SessionListSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-4 px-4 py-3">
          <div className="h-8 w-8 animate-pulse rounded-full bg-gray-200" />
          <div className="flex-1 space-y-2">
            <div className="h-4 w-32 animate-pulse rounded bg-gray-200" />
            <div className="h-3 w-48 animate-pulse rounded bg-gray-200" />
          </div>
          <div className="h-5 w-16 animate-pulse rounded bg-gray-200" />
          <div className="h-4 w-12 animate-pulse rounded bg-gray-200" />
        </div>
      ))}
    </div>
  );
}

// Empty state.
function EmptyState({ filter }: { filter: FilterTab }) {
  const messages: Record<FilterTab, string> = {
    all: 'No sessions found',
    active: 'No active sessions',
    completed: 'No completed sessions',
    abandoned: 'No abandoned sessions',
  };

  return (
    <div className="py-12 text-center">
      <svg
        className="mx-auto h-12 w-12 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={1.5}
          d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
        />
      </svg>
      <p className="mt-2 text-sm text-gray-500">{messages[filter]}</p>
    </div>
  );
}

// Error state.
function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="py-12 text-center">
      <svg
        className="mx-auto h-12 w-12 text-red-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={1.5}
          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
        />
      </svg>
      <p className="mt-2 text-sm text-red-600">{message}</p>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 text-sm font-medium text-blue-600 hover:text-blue-700"
        >
          Try again
        </button>
      ) : null}
    </div>
  );
}

export function SessionList({
  sessions,
  isLoading = false,
  error,
  onSessionClick,
  onRetry,
  selectedSessionId,
  showFilters = true,
  filter: controlledFilter,
  onFilterChange,
  className,
}: SessionListProps) {
  // Internal filter state for uncontrolled mode.
  const [internalFilter, setInternalFilter] = useState<FilterTab>('all');
  const filter = controlledFilter ?? internalFilter;

  // Handle filter change.
  const handleFilterChange = (newFilter: FilterTab) => {
    if (onFilterChange) {
      onFilterChange(newFilter);
    } else {
      setInternalFilter(newFilter);
    }
  };

  // Filter sessions.
  const filteredSessions =
    filter === 'all'
      ? sessions
      : sessions?.filter((s) => s.status === filter);

  // Count sessions by status.
  const counts = sessions?.reduce(
    (acc, s) => {
      acc[s.status]++;
      acc.all++;
      return acc;
    },
    { all: 0, active: 0, completed: 0, abandoned: 0 },
  ) ?? { all: 0, active: 0, completed: 0, abandoned: 0 };

  return (
    <div className={cn('', className)}>
      {/* Filter tabs. */}
      {showFilters ? (
        <nav className="mb-4 flex gap-2">
          <FilterTabButton
            label="All"
            count={counts.all}
            isActive={filter === 'all'}
            onClick={() => handleFilterChange('all')}
          />
          <FilterTabButton
            label="Active"
            count={counts.active}
            isActive={filter === 'active'}
            onClick={() => handleFilterChange('active')}
          />
          <FilterTabButton
            label="Completed"
            count={counts.completed}
            isActive={filter === 'completed'}
            onClick={() => handleFilterChange('completed')}
          />
          <FilterTabButton
            label="Abandoned"
            count={counts.abandoned}
            isActive={filter === 'abandoned'}
            onClick={() => handleFilterChange('abandoned')}
          />
        </nav>
      ) : null}

      {/* Table. */}
      <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
        {isLoading ? (
          <SessionListSkeleton />
        ) : error ? (
          <ErrorState message={error.message} onRetry={onRetry} />
        ) : !filteredSessions || filteredSessions.length === 0 ? (
          <EmptyState filter={filter} />
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th
                  scope="col"
                  className="py-3 pl-4 pr-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
                >
                  Agent
                </th>
                <th
                  scope="col"
                  className="px-3 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
                >
                  Project
                </th>
                <th
                  scope="col"
                  className="px-3 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
                >
                  Status
                </th>
                <th
                  scope="col"
                  className="px-3 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
                >
                  Duration
                </th>
                <th
                  scope="col"
                  className="px-3 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500"
                >
                  Started
                </th>
                <th scope="col" className="relative py-3 pl-3 pr-4">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 bg-white">
              {filteredSessions.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  onClick={onSessionClick ? () => onSessionClick(session.id) : undefined}
                  isSelected={selectedSessionId === session.id}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

// Compact version for sidebars.
export interface CompactSessionListProps {
  /** List of sessions to display. */
  sessions?: Session[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Handler for clicking a session. */
  onSessionClick?: (sessionId: number) => void;
  /** Maximum number of sessions to show. */
  maxVisible?: number;
  /** Title for the section. */
  title?: string;
  /** Handler for "View All" click. */
  onViewAllClick?: () => void;
  /** Additional class name. */
  className?: string;
}

export function CompactSessionList({
  sessions,
  isLoading = false,
  onSessionClick,
  maxVisible = 5,
  title = 'Active Sessions',
  onViewAllClick,
  className,
}: CompactSessionListProps) {
  const visibleSessions = sessions?.slice(0, maxVisible);
  const hasMore = (sessions?.length ?? 0) > maxVisible;

  return (
    <div className={cn('', className)}>
      {/* Section header. */}
      <div className="mb-2 flex items-center justify-between px-3">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500">
          {title}
          {!isLoading && sessions ? (
            <span className="ml-1 text-gray-400">({sessions.length})</span>
          ) : null}
        </h3>
        {onViewAllClick ? (
          <button
            onClick={onViewAllClick}
            className="text-xs font-medium text-blue-600 hover:text-blue-700"
          >
            View All
          </button>
        ) : null}
      </div>

      {/* Content. */}
      {isLoading ? (
        <div className="space-y-1 px-3">
          {Array.from({ length: maxVisible }, (_, i) => (
            <div key={i} className="flex items-center gap-3 py-2">
              <div className="h-6 w-6 animate-pulse rounded-full bg-gray-200" />
              <div className="h-4 flex-1 animate-pulse rounded bg-gray-200" />
            </div>
          ))}
        </div>
      ) : !visibleSessions || visibleSessions.length === 0 ? (
        <div className="px-3 py-4 text-center">
          <p className="text-sm text-gray-500">No active sessions</p>
        </div>
      ) : (
        <div className="space-y-0.5">
          {visibleSessions.map((session) => (
            <CompactSessionRow
              key={session.id}
              session={session}
              onClick={onSessionClick ? () => onSessionClick(session.id) : undefined}
            />
          ))}
          {hasMore ? (
            <button
              onClick={onViewAllClick}
              className="w-full rounded-md px-3 py-2 text-left text-sm text-gray-500 hover:bg-gray-50"
            >
              +{(sessions?.length ?? 0) - maxVisible} more...
            </button>
          ) : null}
        </div>
      )}
    </div>
  );
}
