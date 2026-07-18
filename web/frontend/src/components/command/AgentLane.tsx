// AgentLane is one column of the infinite pane: a sticky agent header,
// a live digest of what the agent is doing, a classified event
// timeline, and a steering composer pinned at the bottom.

import { useMemo, useState } from 'react';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane } from '@/api/command.js';
import type { AgentSummary } from '@/types/api.js';
import { EventCard } from './EventCard.js';
import { SteerComposer } from './SteerComposer.js';
import type { ReplyTarget } from './SteerComposer.js';
import { statusDotClass, timeAgo } from './kinds.js';

// Number of events shown before the "show older" fold.
const VISIBLE_EVENTS = 8;

export interface AgentLaneProps {
  lane: CommandLane;
  // Live activity summary for the digest card, when available.
  summary?: AgentSummary | undefined;
  focused: boolean;
  onToggleFocus: (agentId: number) => void;
}

export function AgentLane({
  lane,
  summary,
  focused,
  onToggleFocus,
}: AgentLaneProps) {
  const { agent } = lane;
  const [replyTarget, setReplyTarget] = useState<ReplyTarget | null>(
    null,
  );
  const [showAll, setShowAll] = useState(false);

  const visibleEvents = useMemo(
    () =>
      showAll ? lane.events : lane.events.slice(0, VISIBLE_EVENTS),
    [lane.events, showAll],
  );

  const hiddenCount = lane.events.length - visibleEvents.length;

  // Reply targets flow from event cards into the composer.
  const handleReply = (event: CommandEvent) => {
    setReplyTarget({
      threadId: event.thread_id,
      subject: event.subject,
    });
  };

  const offline = agent.status === 'offline';

  return (
    <section
      id={`lane-${agent.id}`}
      aria-label={`Agent ${agent.name}`}
      className={clsx(
        'flex h-full shrink-0 snap-start flex-col overflow-hidden',
        'rounded-xl border bg-slate-900/60 backdrop-blur',
        'transition-[width,border-color] duration-200',
        focused
          ? 'w-[560px] border-sky-700/60 shadow-[0_0_24px_rgba(14,116,144,0.15)]'
          : 'w-[380px] border-slate-800',
        offline && !focused && 'opacity-70',
      )}
    >
      {/* Sticky lane header. */}
      <header className="flex items-center gap-2.5 border-b border-slate-800 bg-slate-900/90 px-3 py-2.5">
        <div className="relative">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-slate-600 to-slate-800 text-sm font-semibold text-slate-200">
            {agent.name.slice(0, 1)}
          </div>
          <span
            className={clsx(
              'absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-slate-900',
              statusDotClass(agent.status),
              (agent.status === 'active' || agent.status === 'busy') &&
                'animate-pulse',
            )}
          />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <h2 className="truncate text-sm font-semibold text-slate-100">
              {agent.name}
            </h2>
            <span className="shrink-0 text-[11px] text-slate-500">
              {timeAgo(agent.last_active_at)}
            </span>
          </div>
          <div className="truncate font-mono text-[11px] text-slate-500">
            {agent.project_key || 'no project'}
            {agent.git_branch && (
              <span className="text-slate-600">
                {' '}⎇ {agent.git_branch}
              </span>
            )}
          </div>
        </div>

        {lane.unread_count > 0 && (
          <span className="rounded-full bg-sky-500/20 px-1.5 py-0.5 text-[11px] font-semibold text-sky-300">
            {lane.unread_count}
          </span>
        )}

        <button
          type="button"
          onClick={() => onToggleFocus(agent.id)}
          title={focused ? 'Collapse lane' : 'Focus lane'}
          className="rounded p-1 text-slate-500 hover:bg-slate-800 hover:text-slate-200"
        >
          {focused ? (
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24"
              stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12" />
            </svg>
          ) : (
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24"
              stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          )}
        </button>
      </header>

      {/* Digest: what the agent is doing right now. */}
      <div className="border-b border-slate-800 px-3 py-2.5">
        {summary?.summary ? (
          <div>
            <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
              Now
              {summary.is_stale && (
                <span className="font-normal normal-case text-slate-600">
                  (refreshing…)
                </span>
              )}
            </div>
            <p className="mt-0.5 text-[13px] leading-snug text-slate-300">
              {summary.summary}
            </p>
            {summary.delta && (
              <p className="mt-1 text-[12px] leading-snug text-sky-400/80">
                Δ {summary.delta}
              </p>
            )}
          </div>
        ) : (
          <p className="text-[13px] italic leading-snug text-slate-500">
            {agent.purpose || 'No live summary yet.'}
          </p>
        )}

        {lane.waiting_for && (
          <div className="mt-2 flex items-start gap-1.5 rounded-md border border-amber-800/50 bg-amber-500/10 px-2 py-1.5">
            <span className="text-[11px] font-semibold uppercase tracking-wide text-amber-400">
              Waiting on
            </span>
            <span className="text-[12px] leading-snug text-amber-200/90">
              {lane.waiting_for}
            </span>
          </div>
        )}
      </div>

      {/* Event timeline. */}
      <div className="scrollbar-thin flex-1 space-y-2 overflow-y-auto px-3 py-2.5">
        {visibleEvents.length === 0 && (
          <p className="pt-4 text-center text-[12px] text-slate-600">
            No recent traffic from this agent.
          </p>
        )}

        {visibleEvents.map((event) => (
          <EventCard
            key={event.message_id}
            event={event}
            onReply={handleReply}
          />
        ))}

        {hiddenCount > 0 && (
          <button
            type="button"
            onClick={() => setShowAll(true)}
            className="w-full rounded-md border border-dashed border-slate-800 py-1.5 text-[12px] text-slate-500 hover:border-slate-700 hover:text-slate-300"
          >
            Show {hiddenCount} older
          </button>
        )}
      </div>

      <SteerComposer
        agentName={agent.name}
        replyTarget={replyTarget}
        onClearReply={() => setReplyTarget(null)}
      />
    </section>
  );
}
