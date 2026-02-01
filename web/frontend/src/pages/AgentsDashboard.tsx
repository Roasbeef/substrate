// Agents dashboard page - displays agent status overview and management.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useAgentsStatus } from '@/hooks/useAgents.js';
import { useAgentsRealtime } from '@/hooks/useAgentsRealtime.js';
import {
  AgentCard,
  AgentCardSkeleton,
  DashboardStats,
} from '@/components/agents/index.js';
import type { AgentStatusType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Filter tab configuration.
type FilterTab = 'all' | AgentStatusType;

interface FilterTabConfig {
  id: FilterTab;
  label: string;
}

const filterTabs: FilterTabConfig[] = [
  { id: 'all', label: 'All' },
  { id: 'active', label: 'Active' },
  { id: 'busy', label: 'Busy' },
  { id: 'idle', label: 'Idle' },
  { id: 'offline', label: 'Offline' },
];

// Error display component.
function ErrorDisplay({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-4 rounded-full bg-red-100 p-3">
        <svg
          className="h-6 w-6 text-red-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
      </div>
      <h3 className="mb-1 text-lg font-medium text-gray-900">
        Failed to load agents
      </h3>
      <p className="mb-4 text-sm text-gray-500">{message}</p>
      {onRetry ? (
        <button
          onClick={onRetry}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          Try Again
        </button>
      ) : null}
    </div>
  );
}

// Empty state component.
function EmptyState({ filter }: { filter: FilterTab }) {
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

// Props for AgentsDashboard.
export interface AgentsDashboardProps {
  /** Handler for clicking an agent card. */
  onAgentClick?: (agentId: number) => void;
  /** Handler for clicking register button. */
  onRegisterClick?: () => void;
  /** Additional class name. */
  className?: string;
}

export default function AgentsDashboard({
  onAgentClick,
  onRegisterClick,
  className,
}: AgentsDashboardProps) {
  const [filter, setFilter] = useState<FilterTab>('all');
  const { data, isLoading, error, refetch } = useAgentsStatus();

  // Enable real-time updates via WebSocket.
  const { isConnected: wsConnected } = useAgentsRealtime();

  // Filter agents based on selected tab.
  const filteredAgents =
    filter === 'all'
      ? data?.agents
      : data?.agents.filter((agent) => agent.status === filter);

  // Handle stat card click to filter.
  const handleStatClick = (status: 'active' | 'busy' | 'idle' | 'offline') => {
    setFilter(status);
  };

  return (
    <div className={cn('space-y-6 p-6', className)}>
      {/* Page header. */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-gray-900">Agents</h1>
            <div
              className="flex items-center gap-1.5 text-xs"
              title={wsConnected ? 'Real-time updates active' : 'Connecting...'}
            >
              <span
                className={`inline-block h-2 w-2 rounded-full ${
                  wsConnected ? 'bg-green-400' : 'bg-yellow-400 animate-pulse'
                }`}
              />
              <span className="text-gray-500">
                {wsConnected ? 'Live' : 'Connecting'}
              </span>
            </div>
          </div>
          <p className="mt-1 text-sm text-gray-500">
            Manage and monitor your registered agents.
          </p>
        </div>
        {onRegisterClick ? (
          <button
            onClick={onRegisterClick}
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 4v16m8-8H4"
              />
            </svg>
            Register Agent
          </button>
        ) : null}
      </div>

      {/* Dashboard stats. */}
      <DashboardStats
        {...(data?.counts && { counts: data.counts })}
        isLoading={isLoading}
        onStatClick={handleStatClick}
      />

      {/* Filter tabs. */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8" aria-label="Filter agents">
          {filterTabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id)}
              className={cn(
                'whitespace-nowrap border-b-2 px-1 py-4 text-sm font-medium transition-colors',
                filter === tab.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700',
              )}
              aria-current={filter === tab.id ? 'page' : undefined}
            >
              {tab.label}
              {/* Show count badge for non-all tabs. */}
              {tab.id !== 'all' && data?.counts ? (
                <span
                  className={cn(
                    'ml-2 rounded-full px-2 py-0.5 text-xs',
                    filter === tab.id
                      ? 'bg-blue-100 text-blue-600'
                      : 'bg-gray-100 text-gray-600',
                  )}
                >
                  {data.counts[tab.id]}
                </span>
              ) : null}
            </button>
          ))}
        </nav>
      </div>

      {/* Content area. */}
      {error ? (
        <ErrorDisplay
          message={error instanceof Error ? error.message : 'Unknown error'}
          onRetry={() => void refetch()}
        />
      ) : isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <AgentCardSkeleton key={i} />
          ))}
        </div>
      ) : filteredAgents && filteredAgents.length > 0 ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filteredAgents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              {...(onAgentClick && { onClick: () => onAgentClick(agent.id) })}
            />
          ))}
        </div>
      ) : (
        <EmptyState filter={filter} />
      )}
    </div>
  );
}
