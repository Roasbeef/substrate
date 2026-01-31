// AgentCardGrid component - displays a filterable grid of agent cards.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { AgentCard, AgentCardSkeleton } from './AgentCard.js';
import type { AgentWithStatus, AgentStatusType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Sort options for agents.
export type AgentSortOption = 'name' | 'status' | 'last_active';

// Filter options for agents.
export type AgentFilterOption = 'all' | AgentStatusType;

// Sort comparators.
const sortComparators: Record<
  AgentSortOption,
  (a: AgentWithStatus, b: AgentWithStatus) => number
> = {
  name: (a, b) => a.name.localeCompare(b.name),
  status: (a, b) => {
    const statusOrder: Record<AgentStatusType, number> = {
      active: 0,
      busy: 1,
      idle: 2,
      offline: 3,
    };
    return statusOrder[a.status] - statusOrder[b.status];
  },
  last_active: (a, b) =>
    new Date(b.last_active_at).getTime() - new Date(a.last_active_at).getTime(),
};

// Props for AgentCardGrid.
export interface AgentCardGridProps {
  /** List of agents to display. */
  agents?: AgentWithStatus[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Current filter selection. */
  filter?: AgentFilterOption;
  /** Handler for filter change. */
  onFilterChange?: (filter: AgentFilterOption) => void;
  /** Current sort selection. */
  sort?: AgentSortOption;
  /** Handler for sort change. */
  onSortChange?: (sort: AgentSortOption) => void;
  /** Handler for clicking an agent card. */
  onAgentClick?: (agentId: number) => void;
  /** Selected agent ID for highlighting. */
  selectedAgentId?: number;
  /** Show filter tabs. */
  showFilters?: boolean;
  /** Show sort dropdown. */
  showSort?: boolean;
  /** Additional class name. */
  className?: string;
}

// Filter tab configuration.
interface FilterTabConfig {
  id: AgentFilterOption;
  label: string;
}

const filterTabs: FilterTabConfig[] = [
  { id: 'all', label: 'All' },
  { id: 'active', label: 'Active' },
  { id: 'busy', label: 'Busy' },
  { id: 'idle', label: 'Idle' },
  { id: 'offline', label: 'Offline' },
];

// Sort option configuration.
interface SortOptionConfig {
  id: AgentSortOption;
  label: string;
}

const sortOptions: SortOptionConfig[] = [
  { id: 'name', label: 'Name' },
  { id: 'status', label: 'Status' },
  { id: 'last_active', label: 'Last Active' },
];

// Empty state component.
function EmptyState({ filter }: { filter: AgentFilterOption }) {
  const filterLabel = filter === 'all' ? '' : ` ${filter}`;

  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-4 rounded-full bg-gray-100 p-3">
        <svg
          className="h-6 w-6 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z"
          />
        </svg>
      </div>
      <h3 className="mb-1 text-lg font-medium text-gray-900">
        No{filterLabel} agents
      </h3>
      <p className="text-sm text-gray-500">
        {filter === 'all'
          ? 'No agents have been registered yet.'
          : `No agents are currently ${filter}.`}
      </p>
    </div>
  );
}

// Loading skeleton grid.
function SkeletonGrid({ count = 6 }: { count?: number }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }, (_, i) => (
        <AgentCardSkeleton key={i} />
      ))}
    </div>
  );
}

export function AgentCardGrid({
  agents,
  isLoading = false,
  filter: controlledFilter,
  onFilterChange,
  sort: controlledSort,
  onSortChange,
  onAgentClick,
  selectedAgentId,
  showFilters = true,
  showSort = true,
  className,
}: AgentCardGridProps) {
  // Use controlled or internal state for filter.
  const [internalFilter, setInternalFilter] =
    useState<AgentFilterOption>('all');
  const filter = controlledFilter ?? internalFilter;
  const setFilter = onFilterChange ?? setInternalFilter;

  // Use controlled or internal state for sort.
  const [internalSort, setInternalSort] = useState<AgentSortOption>('name');
  const sort = controlledSort ?? internalSort;
  const setSort = onSortChange ?? setInternalSort;

  // Filter and sort agents.
  const processedAgents = agents
    ? agents
        .filter((agent) => filter === 'all' || agent.status === filter)
        .sort(sortComparators[sort])
    : [];

  return (
    <div className={cn('space-y-4', className)}>
      {/* Controls bar. */}
      {(showFilters || showSort) ? (
        <div className="flex flex-wrap items-center justify-between gap-4">
          {/* Filter tabs. */}
          {showFilters ? (
            <div className="flex gap-1 rounded-lg bg-gray-100 p-1">
              {filterTabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setFilter(tab.id)}
                  className={cn(
                    'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                    filter === tab.id
                      ? 'bg-white text-gray-900 shadow-sm'
                      : 'text-gray-600 hover:text-gray-900',
                  )}
                  aria-pressed={filter === tab.id}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          ) : null}

          {/* Sort dropdown. */}
          {showSort ? (
            <div className="flex items-center gap-2">
              <label htmlFor="agent-sort" className="text-sm text-gray-500">
                Sort by:
              </label>
              <select
                id="agent-sort"
                value={sort}
                onChange={(e) => setSort(e.target.value as AgentSortOption)}
                className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              >
                {sortOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>
          ) : null}
        </div>
      ) : null}

      {/* Grid content. */}
      {isLoading ? (
        <SkeletonGrid />
      ) : processedAgents.length > 0 ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {processedAgents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              onClick={onAgentClick ? () => onAgentClick(agent.id) : undefined}
              isSelected={selectedAgentId === agent.id}
            />
          ))}
        </div>
      ) : (
        <EmptyState filter={filter} />
      )}
    </div>
  );
}
