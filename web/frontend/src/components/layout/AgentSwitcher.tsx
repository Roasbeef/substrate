// AgentSwitcher component - dropdown for selecting current agent context.

import { Fragment, useState, useMemo } from 'react';
import {
  Menu,
  MenuButton,
  MenuItem,
  MenuItems,
  Transition,
} from '@headlessui/react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { StatusBadge } from '@/components/ui/Badge.js';
import { useAuthStore } from '@/stores/auth.js';
import type { AgentWithStatus, AgentStatusType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Chevron icon.
function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
        clipRule="evenodd"
      />
    </svg>
  );
}

// Search icon.
function SearchIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
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

// Check icon for selected agent.
function CheckIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-4 w-4', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 13l4 4L19 7"
      />
    </svg>
  );
}

// Props for AgentSwitcher.
export interface AgentSwitcherProps {
  /** List of agents to display. */
  agents: AgentWithStatus[];
  /** Currently selected agent ID. */
  selectedAgentId?: number;
  /** Handler for agent selection. */
  onSelectAgent?: (agentId: number) => void;
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Whether the dropdown is disabled. */
  disabled?: boolean;
  /** Additional class name. */
  className?: string;
  /** Whether to show the search filter. */
  showSearch?: boolean;
}

// Agent list item component.
function AgentListItem({
  agent,
  isSelected,
  onClick,
}: {
  agent: AgentWithStatus;
  isSelected: boolean;
  onClick: () => void;
}) {
  return (
    <MenuItem>
      {({ focus }) => (
        <button
          type="button"
          onClick={onClick}
          className={cn(
            'flex w-full items-center gap-3 px-3 py-2 text-left text-sm',
            focus ? 'bg-gray-100' : '',
            isSelected ? 'bg-blue-50' : '',
          )}
        >
          <Avatar name={agent.name} size="sm" />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-gray-900 truncate">
                {agent.name}
              </span>
              {isSelected ? (
                <CheckIcon className="h-4 w-4 text-blue-600 flex-shrink-0" />
              ) : null}
            </div>
            <StatusBadge status={agent.status} size="sm" />
          </div>
        </button>
      )}
    </MenuItem>
  );
}

// Presentational AgentSwitcher component.
export function AgentSwitcher({
  agents,
  selectedAgentId,
  onSelectAgent,
  isLoading = false,
  disabled = false,
  className,
  showSearch = true,
}: AgentSwitcherProps) {
  const [searchQuery, setSearchQuery] = useState('');

  // Find the selected agent.
  const selectedAgent = useMemo(
    () => agents.find((a) => a.id === selectedAgentId),
    [agents, selectedAgentId],
  );

  // Filter agents by search query.
  const filteredAgents = useMemo(() => {
    if (!searchQuery.trim()) return agents;
    const query = searchQuery.toLowerCase();
    return agents.filter((a) => a.name.toLowerCase().includes(query));
  }, [agents, searchQuery]);

  // Group agents by status for better organization.
  const groupedAgents = useMemo(() => {
    const groups: Record<AgentStatusType, AgentWithStatus[]> = {
      active: [],
      busy: [],
      idle: [],
      offline: [],
    };

    filteredAgents.forEach((agent) => {
      groups[agent.status].push(agent);
    });

    return groups;
  }, [filteredAgents]);

  // Flattened list with status order preserved.
  const sortedAgents = useMemo(() => {
    return [
      ...groupedAgents.active,
      ...groupedAgents.busy,
      ...groupedAgents.idle,
      ...groupedAgents.offline,
    ];
  }, [groupedAgents]);

  const handleSelect = (agentId: number) => {
    onSelectAgent?.(agentId);
    setSearchQuery('');
  };

  return (
    <Menu as="div" className={cn('relative inline-block text-left', className)}>
      <MenuButton
        className={cn(
          'flex items-center gap-2 rounded-lg px-3 py-2 text-sm',
          'bg-white border border-gray-200 shadow-sm',
          'hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
          disabled ? 'cursor-not-allowed opacity-50' : '',
        )}
        disabled={disabled}
      >
        {isLoading ? (
          <div className="flex items-center gap-2">
            <div className="h-6 w-6 animate-pulse rounded-full bg-gray-200" />
            <div className="h-4 w-20 animate-pulse rounded bg-gray-200" />
          </div>
        ) : selectedAgent ? (
          <>
            <Avatar name={selectedAgent.name} size="xs" />
            <span className="font-medium text-gray-900">{selectedAgent.name}</span>
            <StatusBadge status={selectedAgent.status} size="sm" />
          </>
        ) : (
          <span className="text-gray-500">Select agent...</span>
        )}
        <ChevronDownIcon className="text-gray-400" />
      </MenuButton>

      <Transition
        as={Fragment}
        enter="transition ease-out duration-100"
        enterFrom="transform opacity-0 scale-95"
        enterTo="transform opacity-100 scale-100"
        leave="transition ease-in duration-75"
        leaveFrom="transform opacity-100 scale-100"
        leaveTo="transform opacity-0 scale-95"
      >
        <MenuItems
          className={cn(
            'absolute right-0 z-10 mt-2 w-64 origin-top-right rounded-lg',
            'bg-white shadow-lg ring-1 ring-black ring-opacity-5',
            'focus:outline-none',
          )}
        >
          {/* Search input. */}
          {showSearch && agents.length > 5 ? (
            <div className="p-2 border-b border-gray-100">
              <div className="relative">
                <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search agents..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onClick={(e) => e.stopPropagation()}
                  className={cn(
                    'w-full rounded-md border border-gray-200 py-1.5 pl-9 pr-3 text-sm',
                    'focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
                    'placeholder:text-gray-400',
                  )}
                />
              </div>
            </div>
          ) : null}

          {/* Agent list. */}
          <div className="max-h-64 overflow-auto py-1">
            {sortedAgents.length === 0 ? (
              <div className="px-3 py-4 text-center text-sm text-gray-500">
                {searchQuery ? 'No agents found' : 'No agents available'}
              </div>
            ) : (
              sortedAgents.map((agent) => (
                <AgentListItem
                  key={agent.id}
                  agent={agent}
                  isSelected={agent.id === selectedAgentId}
                  onClick={() => handleSelect(agent.id)}
                />
              ))
            )}
          </div>
        </MenuItems>
      </Transition>
    </Menu>
  );
}

// Connected component that uses auth store.
export function ConnectedAgentSwitcher({
  agents,
  isLoading,
  className,
}: {
  agents: AgentWithStatus[];
  isLoading?: boolean;
  className?: string;
}) {
  const { currentAgent, switchAgent, setAvailableAgents } = useAuthStore();

  // Update available agents in store when agents prop changes.
  // Convert AgentWithStatus to Agent format for store.
  const handleSelectAgent = (agentId: number) => {
    const agent = agents.find((a) => a.id === agentId);
    if (agent) {
      // Update available agents with converted format.
      setAvailableAgents(
        agents.map((a) => ({
          id: a.id,
          name: a.name,
          createdAt: a.last_active_at,
          lastActiveAt: a.last_active_at,
        })),
      );
      switchAgent(agentId);
    }
  };

  return (
    <AgentSwitcher
      agents={agents}
      {...(currentAgent?.id !== undefined && { selectedAgentId: currentAgent.id })}
      onSelectAgent={handleSelectAgent}
      {...(isLoading !== undefined && { isLoading })}
      {...(className !== undefined && { className })}
    />
  );
}
