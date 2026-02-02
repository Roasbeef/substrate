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
import { getAgentContext } from '@/lib/utils.js';
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

// Globe icon for global/all view.
function GlobeIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-6 w-6', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
      />
    </svg>
  );
}

// Extended agent data with unread count.
export interface AgentWithUnread extends AgentWithStatus {
  unreadCount?: number;
}

// Props for AgentSwitcher.
export interface AgentSwitcherProps {
  /** List of agents to display. */
  agents: AgentWithUnread[];
  /** Currently selected agent ID (null/undefined = Global/All). */
  selectedAgentId?: number | null;
  /** Handler for agent selection (null = Global/All). */
  onSelectAgent?: (agentId: number | null) => void;
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Whether the dropdown is disabled. */
  disabled?: boolean;
  /** Additional class name. */
  className?: string;
  /** Whether to show the search filter. */
  showSearch?: boolean;
  /** Total unread count across all agents (for display in button). */
  totalUnreadCount?: number;
}

// Agent list item component.
function AgentListItem({
  agent,
  isSelected,
  onClick,
}: {
  agent: AgentWithUnread;
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
          <div className="relative">
            <Avatar name={agent.name} size="sm" />
            {agent.unreadCount !== undefined && agent.unreadCount > 0 ? (
              <span className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white">
                {agent.unreadCount > 9 ? '9+' : agent.unreadCount}
              </span>
            ) : null}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-medium text-gray-900 truncate">
                {agent.name}
              </span>
              {isSelected ? (
                <CheckIcon className="h-4 w-4 text-blue-600 flex-shrink-0" />
              ) : null}
            </div>
            {getAgentContext(agent) ? (
              <div className="text-xs text-gray-500 truncate">
                {getAgentContext(agent)}
              </div>
            ) : null}
            <div className="flex items-center gap-2">
              <StatusBadge status={agent.status} size="sm" />
              {agent.unreadCount !== undefined && agent.unreadCount > 0 ? (
                <span className="text-xs text-gray-500">
                  {agent.unreadCount} unread
                </span>
              ) : null}
            </div>
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
  totalUnreadCount,
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
    const groups: Record<AgentStatusType, AgentWithUnread[]> = {
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

  const handleSelect = (agentId: number | null) => {
    onSelectAgent?.(agentId);
    setSearchQuery('');
  };

  // Check if Global (all agents) is selected.
  const isGlobalSelected = selectedAgentId === null || selectedAgentId === undefined;

  // Calculate total unread if not provided.
  const displayUnreadCount = totalUnreadCount ?? agents.reduce(
    (sum, a) => sum + (a.unreadCount ?? 0),
    0,
  );

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
            <div className="relative">
              <Avatar name={selectedAgent.name} size="xs" />
              {displayUnreadCount > 0 ? (
                <span className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white">
                  {displayUnreadCount > 9 ? '9+' : displayUnreadCount}
                </span>
              ) : null}
            </div>
            <span className="font-medium text-gray-900">{selectedAgent.name}</span>
            <StatusBadge status={selectedAgent.status} size="sm" />
          </>
        ) : (
          <>
            <div className="relative flex h-6 w-6 items-center justify-center rounded-full bg-blue-100 text-blue-600">
              <GlobeIcon className="h-4 w-4" />
              {displayUnreadCount > 0 ? (
                <span className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold text-white">
                  {displayUnreadCount > 9 ? '9+' : displayUnreadCount}
                </span>
              ) : null}
            </div>
            <span className="font-medium text-gray-900">Global</span>
          </>
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
            'absolute right-0 z-50 mt-2 w-64 origin-top-right rounded-lg',
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
            {/* Global option - always shown first. */}
            <MenuItem>
              {({ focus }) => (
                <button
                  type="button"
                  onClick={() => handleSelect(null)}
                  className={cn(
                    'flex w-full items-center gap-3 px-3 py-2 text-left text-sm',
                    focus ? 'bg-gray-100' : '',
                    isGlobalSelected ? 'bg-blue-50' : '',
                  )}
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-blue-100 text-blue-600">
                    <GlobeIcon className="h-5 w-5" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-900">Global</span>
                      {isGlobalSelected ? (
                        <CheckIcon className="h-4 w-4 text-blue-600 flex-shrink-0" />
                      ) : null}
                    </div>
                    <span className="text-xs text-gray-500">All agents</span>
                  </div>
                </button>
              )}
            </MenuItem>

            {/* Divider. */}
            {sortedAgents.length > 0 ? (
              <div className="my-1 border-t border-gray-100" />
            ) : null}

            {sortedAgents.length === 0 && searchQuery ? (
              <div className="px-3 py-4 text-center text-sm text-gray-500">
                No agents found
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
  totalUnreadCount,
}: {
  agents: AgentWithUnread[];
  isLoading?: boolean;
  className?: string;
  totalUnreadCount?: number;
}) {
  const { currentAgent, switchAgent, setAvailableAgents, setCurrentAgent } = useAuthStore();

  // Update available agents in store when agents prop changes.
  // Convert AgentWithUnread to Agent format for store.
  const handleSelectAgent = (agentId: number | null) => {
    // Handle Global selection (null = all agents).
    if (agentId === null) {
      setCurrentAgent(null);
      return;
    }

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
      selectedAgentId={currentAgent?.id ?? null}
      onSelectAgent={handleSelectAgent}
      {...(isLoading !== undefined && { isLoading })}
      {...(className !== undefined && { className })}
      {...(totalUnreadCount !== undefined && { totalUnreadCount })}
    />
  );
}
