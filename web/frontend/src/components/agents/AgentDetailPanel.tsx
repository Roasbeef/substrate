// AgentDetailPanel - click-to-focus detail view for a single agent.
// Inspired by the reference design's "click on each to make front and center,
// then click back" pattern.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { getAgentContext } from '@/lib/utils.js';
import { StatusBadge } from './AgentCard.js';
import { ActivityTimeline } from './ActivityTimeline.js';
import { useAgentSummary, useSummaryHistory } from '@/hooks/useSummaries.js';
import { useInfiniteAgentActivities } from '@/hooks/useActivities.js';
import type { AgentWithStatus } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format time since last activity.
function formatTimeSince(seconds: number): string {
  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

// Format relative time from ISO date string.
function formatRelativeTime(isoDate: string): string {
  const date = new Date(isoDate);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  return formatTimeSince(seconds);
}

export interface AgentDetailPanelProps {
  agent: AgentWithStatus;
  onBack: () => void;
  className?: string;
}

export function AgentDetailPanel({
  agent,
  onBack,
  className,
}: AgentDetailPanelProps) {
  const { data: summary, isLoading: summaryLoading } = useAgentSummary(agent.id);
  const { data: summaryHistory } = useSummaryHistory(agent.id, 30);

  // Fetch activities for the agent.
  const activitiesQuery = useInfiniteAgentActivities(agent.id);
  const activities = activitiesQuery.data?.pages.flatMap((p) => p.data) ?? [];

  return (
    <div className={cn('space-y-6', className)}>
      {/* Back navigation. */}
      <button
        onClick={onBack}
        className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 transition-colors"
      >
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
        </svg>
        Back to all agents
      </button>

      {/* Agent header. */}
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <Avatar name={agent.name} size="lg" />
            <div>
              <h2 className="text-xl font-semibold text-gray-900">{agent.name}</h2>
              {getAgentContext(agent) ? (
                <p className="text-sm text-gray-500">{getAgentContext(agent)}</p>
              ) : null}
              <div className="mt-1 flex items-center gap-3 text-sm text-gray-500">
                {agent.session_id !== undefined ? (
                  <span>Session #{agent.session_id}</span>
                ) : null}
                <span>{formatTimeSince(agent.seconds_since_heartbeat)}</span>
              </div>
            </div>
          </div>
          <StatusBadge status={agent.status} size="md" />
        </div>
      </div>

      {/* Current activity summary card. */}
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <h3 className="text-sm font-semibold text-gray-900 mb-3">
          Current Activity
        </h3>
        {summaryLoading ? (
          <div className="space-y-2">
            <div className="h-5 w-full animate-pulse rounded bg-gray-200" />
            <div className="h-5 w-3/4 animate-pulse rounded bg-gray-200" />
          </div>
        ) : summary ? (
          <div>
            <p className="text-sm text-gray-700 leading-relaxed">
              {summary.summary}
            </p>
            {summary.delta && summary.delta !== 'Initial summary' ? (
              <div className="mt-3 flex items-start gap-1.5">
                <span className="font-semibold text-blue-600 text-sm">&#916;</span>
                <p className="text-sm text-gray-500">{summary.delta}</p>
              </div>
            ) : null}
            <div className="mt-3 flex items-center justify-between">
              <span className="text-xs text-gray-400">
                Updated {formatRelativeTime(summary.generated_at)}
              </span>
              {summary.is_stale ? (
                <span className="inline-flex items-center gap-1 text-xs text-blue-500">
                  <svg className="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  Refreshing
                </span>
              ) : null}
            </div>
          </div>
        ) : (
          <p className="text-sm text-gray-400 italic">
            No activity summary available. Summary will be generated when the agent has an active session.
          </p>
        )}
      </div>

      {/* Activity timeline. */}
      <div className="rounded-lg border border-gray-200 bg-white p-6">
        <h3 className="text-sm font-semibold text-gray-900 mb-4">
          Activity Timeline
        </h3>
        <ActivityTimeline
          activities={activities}
          summaries={summaryHistory ?? []}
        />
      </div>
    </div>
  );
}
