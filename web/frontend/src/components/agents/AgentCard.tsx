// AgentCard component - displays a single agent's status, info, and activity
// summary.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { getAgentContext } from '@/lib/utils.js';
import type { AgentWithStatus, AgentStatusType, AgentSummary } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Status badge component.
export interface StatusBadgeProps {
  status: AgentStatusType;
  size?: 'sm' | 'md';
  className?: string;
}

const statusColors: Record<AgentStatusType, { bg: string; text: string; dot: string }> = {
  active: { bg: 'bg-green-50', text: 'text-green-700', dot: 'bg-green-400' },
  busy: { bg: 'bg-yellow-50', text: 'text-yellow-700', dot: 'bg-yellow-400' },
  idle: { bg: 'bg-gray-50', text: 'text-gray-600', dot: 'bg-gray-400' },
  offline: { bg: 'bg-gray-100', text: 'text-gray-500', dot: 'bg-gray-300' },
};

const statusLabels: Record<AgentStatusType, string> = {
  active: 'Active',
  busy: 'Busy',
  idle: 'Idle',
  offline: 'Offline',
};

export function StatusBadge({ status, size = 'md', className }: StatusBadgeProps) {
  const colors = statusColors[status];

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full font-medium',
        colors.bg,
        colors.text,
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm',
        className,
      )}
    >
      <span className={cn('rounded-full', colors.dot, size === 'sm' ? 'h-1.5 w-1.5' : 'h-2 w-2')} />
      {statusLabels[status]}
    </span>
  );
}

// Format time since last activity.
function formatTimeSince(seconds: number): string {
  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

// Format a relative time string from an ISO date.
function formatRelativeTime(isoDate: string): string {
  const date = new Date(isoDate);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  return formatTimeSince(seconds);
}

// Activity summary sub-component displayed inside the agent card.
function ActivitySummarySection({ summary }: { summary: AgentSummary }) {
  return (
    <div className="mt-3 rounded-md border border-blue-100 bg-blue-50/50 px-3 py-2.5">
      <div className="flex items-center justify-between mb-1">
        <p className="text-xs font-medium text-blue-700">Current Activity</p>
        <div className="flex items-center gap-1">
          {summary.is_stale ? (
            <svg className="h-3 w-3 text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
          ) : null}
          <span className="text-[10px] text-blue-400">
            {formatRelativeTime(summary.generated_at)}
          </span>
        </div>
      </div>
      <p className="text-sm text-gray-700 leading-snug line-clamp-2">{summary.summary}</p>
      {summary.delta && summary.delta !== 'Initial summary' ? (
        <p className="mt-1.5 text-xs text-gray-500">
          <span className="font-medium text-blue-600">&#916;</span>{' '}
          {summary.delta}
        </p>
      ) : null}
    </div>
  );
}

// Skeleton for summary loading state.
function SummarySkeleton() {
  return (
    <div className="mt-3 rounded-md border border-gray-100 bg-gray-50/50 px-3 py-2.5">
      <div className="h-3 w-24 animate-pulse rounded bg-gray-200 mb-2" />
      <div className="h-4 w-full animate-pulse rounded bg-gray-200 mb-1" />
      <div className="h-4 w-3/4 animate-pulse rounded bg-gray-200" />
    </div>
  );
}

// Props for AgentCard component.
export interface AgentCardProps {
  /** Agent data to display. */
  agent: AgentWithStatus;
  /** Optional summary data for the agent. */
  summary?: AgentSummary | null;
  /** Whether summary is loading. */
  summaryLoading?: boolean;
  /** Handler for clicking the card. */
  onClick?: () => void;
  /** Whether the card is selected. */
  isSelected?: boolean;
  /** Additional class name. */
  className?: string;
}

export function AgentCard({
  agent,
  summary,
  summaryLoading,
  onClick,
  isSelected = false,
  className,
}: AgentCardProps) {
  return (
    <div
      className={cn(
        'rounded-lg border bg-white p-4 transition-all',
        onClick ? 'cursor-pointer hover:shadow-md' : '',
        isSelected
          ? 'border-blue-300 ring-2 ring-blue-100'
          : 'border-gray-200 hover:border-gray-300',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <Avatar name={agent.name} size="md" />
          <div className="min-w-0 flex-1">
            <h3 className="font-medium text-gray-900">{agent.name}</h3>
            {getAgentContext(agent) ? (
              <p className="text-xs text-gray-500 truncate" title={getAgentContext(agent) ?? ''}>
                {getAgentContext(agent)}
              </p>
            ) : null}
            {agent.git_branch ? (
              <p className="flex items-center gap-1 text-xs text-gray-500 truncate" title={agent.git_branch}>
                <svg className="h-3 w-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 3v12M18 9a3 3 0 01-3 3H6m12-6a3 3 0 10-6 0 3 3 0 006 0zM6 21a3 3 0 100-6 3 3 0 000 6z" />
                </svg>
                <code className="truncate">{agent.git_branch}</code>
              </p>
            ) : null}
            <p className="text-sm text-gray-500">
              {formatTimeSince(agent.seconds_since_heartbeat)}
            </p>
          </div>
        </div>
        <StatusBadge status={agent.status} size="sm" />
      </div>

      {/* Activity summary section. */}
      {summaryLoading ? (
        <SummarySkeleton />
      ) : summary ? (
        <ActivitySummarySection summary={summary} />
      ) : null}

      {/* Session info if busy. */}
      {agent.session_id !== undefined ? (
        <div className="mt-3 rounded bg-gray-50 px-3 py-2">
          <p className="text-xs text-gray-500">Current session</p>
          <p className="text-sm font-medium text-gray-700">#{agent.session_id}</p>
        </div>
      ) : null}
    </div>
  );
}

// Compact variant for sidebars.
export interface CompactAgentCardProps {
  agent: AgentWithStatus;
  onClick?: () => void;
  className?: string;
}

export function CompactAgentCard({
  agent,
  onClick,
  className,
}: CompactAgentCardProps) {
  const colors = statusColors[agent.status];

  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-md px-3 py-2 transition-colors',
        onClick ? 'cursor-pointer hover:bg-gray-50' : '',
        className,
      )}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      <div className="relative">
        <Avatar name={agent.name} size="sm" />
        <span
          className={cn(
            'absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-white',
            colors.dot,
          )}
        />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-gray-900">{agent.name}</p>
        {getAgentContext(agent) ? (
          <p className="truncate text-xs text-gray-500">{getAgentContext(agent)}</p>
        ) : null}
      </div>
    </div>
  );
}

// Skeleton for loading state.
export function AgentCardSkeleton() {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 animate-pulse rounded-full bg-gray-200" />
          <div className="space-y-2">
            <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
            <div className="h-3 w-16 animate-pulse rounded bg-gray-200" />
          </div>
        </div>
        <div className="h-6 w-16 animate-pulse rounded-full bg-gray-200" />
      </div>
    </div>
  );
}
