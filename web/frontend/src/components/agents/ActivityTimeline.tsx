// ActivityTimeline component - displays a chronological timeline of agent
// activities and summary history, inspired by the reference design.

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

// Icon colors and SVG paths for each timeline entry type.
const iconStyles: Record<TimelineEntry['icon'], { bg: string; color: string }> = {
  summary: { bg: 'bg-blue-100', color: 'text-blue-600' },
  heartbeat: { bg: 'bg-green-100', color: 'text-green-600' },
  session: { bg: 'bg-purple-100', color: 'text-purple-600' },
  message: { bg: 'bg-yellow-100', color: 'text-yellow-600' },
  agent: { bg: 'bg-gray-100', color: 'text-gray-600' },
};

function TimelineIcon({ icon }: { icon: TimelineEntry['icon'] }) {
  const styles = iconStyles[icon];

  return (
    <div className={cn('flex h-7 w-7 items-center justify-center rounded-full', styles.bg)}>
      {icon === 'summary' ? (
        <svg className={cn('h-3.5 w-3.5', styles.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
      ) : icon === 'heartbeat' ? (
        <svg className={cn('h-3.5 w-3.5', styles.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
        </svg>
      ) : icon === 'session' ? (
        <svg className={cn('h-3.5 w-3.5', styles.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      ) : icon === 'message' ? (
        <svg className={cn('h-3.5 w-3.5', styles.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      ) : (
        <svg className={cn('h-3.5 w-3.5', styles.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      )}
    </div>
  );
}

// Format timestamp for timeline display.
function formatTimelineTime(date: Date): string {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

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
    weekday: 'short',
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
      <div className={cn('py-8 text-center text-sm text-gray-500', className)}>
        No activity recorded yet.
      </div>
    );
  }

  return (
    <div className={cn('space-y-6', className)}>
      {Array.from(groups.entries()).map(([dateKey, entries]) => (
        <div key={dateKey}>
          <h4 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400">
            {formatTimelineDate(entries[0].timestamp)}
          </h4>
          <div className="relative">
            {/* Vertical timeline line. */}
            <div className="absolute left-3.5 top-0 bottom-0 w-px bg-gray-200" />

            <div className="space-y-4">
              {entries.map((entry) => (
                <div key={entry.id} className="relative flex gap-3">
                  {/* Timeline dot/icon. */}
                  <div className="relative z-10">
                    <TimelineIcon icon={entry.icon} />
                  </div>

                  {/* Content. */}
                  <div className="min-w-0 flex-1 pb-1">
                    <div className="flex items-baseline justify-between gap-2">
                      <p className="text-sm font-medium text-gray-900">
                        {entry.title}
                      </p>
                      <span className="shrink-0 text-xs text-gray-400">
                        {formatTimelineTime(entry.timestamp)}
                      </span>
                    </div>
                    {entry.description ? (
                      <p className={cn(
                        'mt-0.5 text-sm leading-snug',
                        entry.type === 'summary'
                          ? 'text-gray-700 italic'
                          : 'text-gray-500',
                      )}>
                        {entry.type === 'summary' ? `"${entry.description}"` : entry.description}
                      </p>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
