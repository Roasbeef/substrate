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
    <div className="ml-1 max-w-md rounded-lg border border-[#E6E4DD] bg-white px-4 py-3 shadow-sm">
      <p className="text-[13px] font-medium text-[#22262A] mb-1">
        Activity Summary
      </p>
      <p className="text-[13px] text-[#6B7280] leading-relaxed line-clamp-3">
        {entry.description}
      </p>
      {entry.delta && entry.delta !== 'Initial summary' ? (
        <div className="mt-2.5 flex items-start gap-1.5 border-t border-[#F1EFE9] pt-2">
          <span className="text-xs font-bold text-[#33608D]">&#916;</span>
          <p className="text-xs text-[#6B7280] leading-relaxed line-clamp-2">
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
    <span className="text-[13px] text-[#6B7280]">
      <span className="font-medium text-[#22262A]">{entry.title}</span>
      {entry.description ? (
        <span className="text-[#6B7280]"> &mdash; {entry.description}</span>
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
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-[#F1EFE9]">
          <svg className="h-5 w-5 text-[#9BA0A6]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <p className="text-sm font-medium text-[#6B7280]">No activity recorded yet</p>
        <p className="mt-1 text-xs text-[#9BA0A6]">Activity and summaries will appear here.</p>
      </div>
    );
  }

  return (
    <div className={cn('overflow-y-auto', className)}>
      {/* Section heading. */}
      <h3 className="mb-4 text-xs font-semibold uppercase tracking-wider text-[#6B7280]">
        Activity History
      </h3>

      <div className="space-y-6">
        {Array.from(groups.entries()).map(([dateKey, entries]) => (
          <div key={dateKey}>
            {/* Date group header centered with horizontal rules. */}
            <div className="flex items-center gap-4 mb-5">
              <div className="h-px flex-1 bg-[#E6E4DD]" />
              <span className="text-xs font-semibold text-[#6B7280] whitespace-nowrap">
                {entries[0] ? formatTimelineDate(entries[0].timestamp) : ''}
              </span>
              <div className="h-px flex-1 bg-[#E6E4DD]" />
            </div>

            {/* Timeline entries with vertical line. */}
            <div className="relative pl-6">
              {/* Vertical timeline line. */}
              <div className="absolute left-[7px] top-1 bottom-1 w-[2px] bg-[#E6E4DD] rounded-full" />

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
                          <div className="h-4 w-4 rounded-full border-[2.5px] border-[#9BA0A6] bg-white" />
                        ) : (
                          // Filled circle for activities - smaller.
                          <div className={cn(
                            'h-2.5 w-2.5 rounded-full',
                            entry.icon === 'heartbeat' ? 'bg-[#178A5B]' :
                            entry.icon === 'session' ? 'bg-[#5B5BD6]' :
                            entry.icon === 'message' ? 'bg-[#22262A]' :
                            'bg-[#9BA0A6]',
                          )} />
                        )}
                      </div>

                      {/* Timestamp column. */}
                      <div className="w-16 shrink-0 pt-0.5">
                        <span className="text-xs text-[#9BA0A6] tabular-nums">
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
