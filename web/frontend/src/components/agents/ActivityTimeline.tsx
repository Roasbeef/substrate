// ActivityTimeline component - displays a chronological timeline of agent
// activities and summary history, inspired by the centered vertical timeline
// in the reference design (design_ref/refreshed_ui_for_inbox_and_mail/).
//
// Visual design: centered vertical line with open circles for summaries,
// filled circles for activities. Summary cards branch left from the line,
// activity entries sit inline next to filled dots.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { Activity, AgentSummaryHistory } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Unified timeline entry combining activities and summaries.
interface TimelineEntry {
  id: string;
  type: 'summary' | 'activity';
  timestamp: Date;
  title: string;
  description: string;
  delta?: string;
  icon: 'summary' | 'heartbeat' | 'session' | 'message' | 'agent';
}

// Merge and sort activities and summaries into a single timeline.
function buildTimeline(
  activities: Activity[],
  summaries: AgentSummaryHistory[],
): TimelineEntry[] {
  const entries: TimelineEntry[] = [];

  for (const s of summaries) {
    entries.push({
      id: `summary-${s.id}`,
      type: 'summary',
      timestamp: new Date(s.created_at),
      title: 'Summary updated',
      description: s.summary,
      delta: s.delta,
      icon: 'summary',
    });
  }

  for (const a of activities) {
    const icon = getActivityIcon(a.type);
    entries.push({
      id: `activity-${a.id}`,
      type: 'activity',
      timestamp: new Date(a.created_at),
      title: formatActivityTitle(a.type),
      description: a.description,
      icon,
    });
  }

  entries.sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());
  return entries;
}

// Map activity type to icon category.
function getActivityIcon(
  type: string,
): TimelineEntry['icon'] {
  switch (type) {
    case 'heartbeat':
      return 'heartbeat';
    case 'session_started':
    case 'session_completed':
      return 'session';
    case 'message_sent':
    case 'message_read':
      return 'message';
    case 'agent_registered':
      return 'agent';
    default:
      return 'agent';
  }
}

// Format activity type as a human-readable title.
function formatActivityTitle(type: string): string {
  switch (type) {
    case 'heartbeat':
      return 'Heartbeat received';
    case 'session_started':
      return 'Session started';
    case 'session_completed':
      return 'Session completed';
    case 'message_sent':
      return 'Message sent';
    case 'message_read':
      return 'Message read';
    case 'agent_registered':
      return 'Agent registered';
    default:
      return type.replace(/_/g, ' ');
  }
}

// Format timestamp for timeline display.
function formatTimelineTime(date: Date): string {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// Format date for timeline group headers.
function formatTimelineDate(date: Date): string {
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(yesterday.getDate() - 1);

  if (date.toDateString() === today.toDateString()) {
    return 'Today';
  }
  if (date.toDateString() === yesterday.toDateString()) {
    return 'Yesterday';
  }
  return date.toLocaleDateString([], {
    weekday: 'long',
    month: 'short',
    day: 'numeric',
  });
}

// Group entries by date.
function groupByDate(entries: TimelineEntry[]): Map<string, TimelineEntry[]> {
  const groups = new Map<string, TimelineEntry[]>();
  for (const entry of entries) {
    const key = entry.timestamp.toDateString();
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)!.push(entry);
  }
  return groups;
}

// Summary card component - message-preview style card matching reference.
function SummaryCard({ entry }: { entry: TimelineEntry }) {
  return (
    <div className="ml-1 max-w-md rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm">
      <p className="text-[13px] font-medium text-gray-900 mb-1">
        Activity Summary
      </p>
      <p className="text-[13px] text-gray-600 leading-relaxed line-clamp-3">
        {entry.description}
      </p>
      {entry.delta && entry.delta !== 'Initial summary' ? (
        <div className="mt-2.5 flex items-start gap-1.5 border-t border-gray-100 pt-2">
          <span className="text-xs font-bold text-blue-600">&#916;</span>
          <p className="text-xs text-gray-500 leading-relaxed line-clamp-2">
            {entry.delta}
          </p>
        </div>
      ) : null}
    </div>
  );
}

// Activity inline entry - compact text next to filled dot.
function ActivityEntry({ entry }: { entry: TimelineEntry }) {
  return (
    <span className="text-[13px] text-gray-600">
      <span className="font-medium text-gray-800">{entry.title}</span>
      {entry.description ? (
        <span className="text-gray-500"> &mdash; {entry.description}</span>
      ) : null}
    </span>
  );
}

// Props for ActivityTimeline.
export interface ActivityTimelineProps {
  activities: Activity[];
  summaries: AgentSummaryHistory[];
  className?: string;
  maxEntries?: number;
}

export function ActivityTimeline({
  activities,
  summaries,
  className,
  maxEntries = 50,
}: ActivityTimelineProps) {
  const timeline = buildTimeline(activities, summaries).slice(0, maxEntries);
  const groups = groupByDate(timeline);

  if (timeline.length === 0) {
    return (
      <div className={cn('py-12 text-center', className)}>
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-gray-100">
          <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <p className="text-sm font-medium text-gray-500">No activity recorded yet</p>
        <p className="mt-1 text-xs text-gray-400">Activity and summaries will appear here.</p>
      </div>
    );
  }

  return (
    <div className={cn('overflow-y-auto', className)}>
      {/* Section heading. */}
      <h3 className="mb-4 text-xs font-semibold uppercase tracking-wider text-gray-400">
        Activity History
      </h3>

      <div className="space-y-6">
        {Array.from(groups.entries()).map(([dateKey, entries]) => (
          <div key={dateKey}>
            {/* Date group header centered with horizontal rules. */}
            <div className="flex items-center gap-4 mb-5">
              <div className="h-px flex-1 bg-gray-200" />
              <span className="text-xs font-semibold text-gray-500 whitespace-nowrap">
                {entries[0] ? formatTimelineDate(entries[0].timestamp) : ''}
              </span>
              <div className="h-px flex-1 bg-gray-200" />
            </div>

            {/* Timeline entries with vertical line. */}
            <div className="relative pl-6">
              {/* Vertical timeline line. */}
              <div className="absolute left-[7px] top-1 bottom-1 w-[2px] bg-gray-200 rounded-full" />

              <div className="space-y-5">
                {entries.map((entry) => {
                  const isSummary = entry.type === 'summary';
                  return (
                    <div key={entry.id} className="relative flex items-start gap-3">
                      {/* Timeline dot on the vertical line. */}
                      <div className={cn(
                        'absolute left-[-24px] z-10 flex items-center justify-center',
                        isSummary ? 'top-0.5' : 'top-1',
                      )}>
                        {isSummary ? (
                          // Open circle for summaries - larger.
                          <div className="h-4 w-4 rounded-full border-[2.5px] border-gray-400 bg-white" />
                        ) : (
                          // Filled circle for activities - smaller.
                          <div className={cn(
                            'h-2.5 w-2.5 rounded-full',
                            entry.icon === 'heartbeat' ? 'bg-green-400' :
                            entry.icon === 'session' ? 'bg-purple-400' :
                            entry.icon === 'message' ? 'bg-gray-700' :
                            'bg-gray-400',
                          )} />
                        )}
                      </div>

                      {/* Timestamp column. */}
                      <div className="w-16 shrink-0 pt-0.5">
                        <span className="text-xs text-gray-400 tabular-nums">
                          {formatTimelineTime(entry.timestamp)}
                        </span>
                      </div>

                      {/* Content area. */}
                      <div className="min-w-0 flex-1">
                        {isSummary ? (
                          <SummaryCard entry={entry} />
                        ) : (
                          <ActivityEntry entry={entry} />
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
