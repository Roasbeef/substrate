// AgentsSidebar component - compact list of agents for sidebar display.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { CompactAgentCard } from './AgentCard.js';
import type { AgentWithStatus, AgentStatusType } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Props for AgentsSidebar.
export interface AgentsSidebarProps {
  /** List of agents to display. */
  agents?: AgentWithStatus[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Handler for clicking an agent. */
  onAgentClick?: (agentId: number) => void;
  /** Currently selected agent ID. */
  selectedAgentId?: number;
  /** Maximum number of agents to show. */
  maxVisible?: number;
  /** Title for the section. */
  title?: string;
  /** Filter to specific status. */
  filterStatus?: AgentStatusType;
  /** Handler for "View All" click. */
  onViewAllClick?: () => void;
  /** Additional class name. */
  className?: string;
}

// Status dot component.
function StatusDot({ status }: { status: AgentStatusType }) {
  const colors: Record<AgentStatusType, string> = {
    active: 'bg-[#178A5B]',
    busy: 'bg-[#178A5B]',
    idle: 'bg-[#C98A1B]',
    offline: 'bg-[#B0ADA4]',
  };

  return (
    <span
      className={cn('h-2 w-2 flex-shrink-0 rounded-full', colors[status])}
      title={status}
    />
  );
}

// Loading skeleton.
function AgentsSidebarSkeleton({ count = 5 }: { count?: number }) {
  return (
    <div className="space-y-1">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="flex items-center gap-3 rounded-md px-3 py-2">
          <div className="h-8 w-8 animate-pulse rounded-full bg-[#E6E4DD]" />
          <div className="h-4 flex-1 animate-pulse rounded bg-[#E6E4DD]" />
        </div>
      ))}
    </div>
  );
}

// Empty state.
function EmptyState({ filterStatus }: { filterStatus?: AgentStatusType }) {
  const message = filterStatus
    ? `No ${filterStatus} agents`
    : 'No agents registered';

  return (
    <div className="px-3 py-4 text-center">
      <p className="text-sm text-[#6B7280]">{message}</p>
    </div>
  );
}

export function AgentsSidebar({
  agents,
  isLoading = false,
  onAgentClick,
  selectedAgentId,
  maxVisible = 10,
  title = 'Agents',
  filterStatus,
  onViewAllClick,
  className,
}: AgentsSidebarProps) {
  // Filter agents by status if specified.
  const filteredAgents = filterStatus
    ? agents?.filter((a) => a.status === filterStatus)
    : agents;

  // Limit visible agents.
  const visibleAgents = filteredAgents?.slice(0, maxVisible);
  const hasMore = (filteredAgents?.length ?? 0) > maxVisible;
  const totalCount = filteredAgents?.length ?? 0;

  return (
    <div className={cn('', className)}>
      {/* Section header. */}
      <div className="mb-2 flex items-center justify-between px-3">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-[#6B7280]">
          {title}
          {!isLoading && totalCount > 0 ? (
            <span className="ml-1 text-[#9BA0A6]">({totalCount})</span>
          ) : null}
        </h3>
        {onViewAllClick ? (
          <button
            onClick={onViewAllClick}
            className="text-xs font-medium text-[#33608D] hover:text-[#22262A]"
          >
            View All
          </button>
        ) : null}
      </div>

      {/* Content. */}
      {isLoading ? (
        <AgentsSidebarSkeleton count={maxVisible} />
      ) : !visibleAgents || visibleAgents.length === 0 ? (
        <EmptyState {...(filterStatus !== undefined && { filterStatus })} />
      ) : (
        <div className="space-y-0.5">
          {visibleAgents.map((agent) => (
            <CompactAgentCard
              key={agent.id}
              agent={agent}
              {...(onAgentClick && { onClick: () => onAgentClick(agent.id) })}
              className={cn(
                selectedAgentId === agent.id
                  ? 'bg-[#F1EFE9] text-[#22262A]'
                  : '',
              )}
            />
          ))}

          {/* Show more indicator. */}
          {hasMore ? (
            <button
              onClick={onViewAllClick}
              className="w-full rounded-md px-3 py-2 text-left text-sm text-[#6B7280] hover:bg-[#F1EFE9]"
            >
              +{totalCount - maxVisible} more...
            </button>
          ) : null}
        </div>
      )}
    </div>
  );
}

// Compact version that just shows status dots.
export interface AgentStatusListProps {
  /** List of agents to display. */
  agents?: AgentWithStatus[];
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Handler for clicking an agent. */
  onAgentClick?: (agentId: number) => void;
  /** Maximum number to show. */
  maxVisible?: number;
  /** Additional class name. */
  className?: string;
}

export function AgentStatusList({
  agents,
  isLoading = false,
  onAgentClick,
  maxVisible = 8,
  className,
}: AgentStatusListProps) {
  if (isLoading) {
    return (
      <div className={cn('flex flex-wrap gap-1', className)}>
        {Array.from({ length: maxVisible }, (_, i) => (
          <div
            key={i}
            className="h-3 w-3 animate-pulse rounded-full bg-[#E6E4DD]"
          />
        ))}
      </div>
    );
  }

  if (!agents || agents.length === 0) {
    return null;
  }

  const visibleAgents = agents.slice(0, maxVisible);
  const hasMore = agents.length > maxVisible;

  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {visibleAgents.map((agent) => (
        <button
          key={agent.id}
          onClick={onAgentClick ? () => onAgentClick(agent.id) : undefined}
          className="group relative"
          title={`${agent.name} (${agent.status})`}
        >
          <StatusDot status={agent.status} />
        </button>
      ))}
      {hasMore ? (
        <span className="text-xs text-[#9BA0A6]">
          +{agents.length - maxVisible}
        </span>
      ) : null}
    </div>
  );
}
