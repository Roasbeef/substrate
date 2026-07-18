// CommandCenterPage is the mission-control "infinite pane": one lane
// per agent with live digests, classified event timelines, inline plan
// approval and diffs, plus a cross-agent attention queue. Real-time
// updates arrive over the shared WebSocket connection.

import { useCallback, useMemo, useState } from 'react';
import { clsx } from 'clsx';
import { useCommandFeed, useCommandFeedRealtime } from '@/hooks/useCommandFeed.js';
import { useAgentSummaries } from '@/hooks/useSummaries.js';
import { useSummaryRealtime } from '@/hooks/useSummaries.js';
import { useConnectionState, useWebSocketConnection } from '@/hooks/useWebSocket.js';
import { AttentionRail } from '@/components/command/AttentionRail.js';
import { AgentLane } from '@/components/command/AgentLane.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { AttentionItem } from '@/api/command.js';

// Fleet status counts derived from lanes.
function useFleetCounts(
  statuses: string[],
): { active: number; idle: number; offline: number } {
  return useMemo(() => {
    const counts = { active: 0, idle: 0, offline: 0 };
    for (const s of statuses) {
      if (s === 'active' || s === 'busy') {
        counts.active++;
      } else if (s === 'idle') {
        counts.idle++;
      } else {
        counts.offline++;
      }
    }
    return counts;
  }, [statuses]);
}

export default function CommandCenterPage() {
  useWebSocketConnection();
  useSummaryRealtime();

  const connectionState = useConnectionState();

  const { data, isLoading, isError, refetch } = useCommandFeed();
  const { data: summaries } = useAgentSummaries(false);

  const [focusedAgentId, setFocusedAgentId] = useState<number | null>(
    null,
  );

  // Keep lanes live over WebSocket; the app's global message toasts
  // already announce actionable arrivals, so no extra toast here.
  useCommandFeedRealtime();

  const lanes = useMemo(() => data?.lanes ?? [], [data?.lanes]);
  const attention = data?.attention ?? [];

  const summariesByAgent = useMemo(() => {
    const map = new Map<number, NonNullable<typeof summaries>[number]>();
    for (const s of summaries ?? []) {
      map.set(s.agent_id, s);
    }
    return map;
  }, [summaries]);

  const fleet = useFleetCounts(
    useMemo(
      () => lanes.map((l) => l.agent.status),
      [lanes],
    ),
  );

  // Focus a lane and scroll it into view.
  const focusLane = useCallback((agentId: number) => {
    setFocusedAgentId((prev) =>
      prev === agentId ? null : agentId,
    );
    // Scroll after the width transition kicks in.
    requestAnimationFrame(() => {
      document
        .getElementById(`lane-${agentId}`)
        ?.scrollIntoView({
          behavior: 'smooth',
          inline: 'start',
          block: 'nearest',
        });
    });
  }, []);

  const handleAttentionSelect = useCallback(
    (item: AttentionItem) => {
      focusLane(item.agent_id);
    },
    [focusLane],
  );

  return (
    <div className="flex h-full flex-col bg-[#0B1120] text-slate-200">
      {/* Page header. */}
      <div className="flex items-center gap-4 border-b border-slate-800/80 px-4 py-2.5">
        <h1 className="text-base font-semibold tracking-tight text-slate-100">
          Command Center
        </h1>

        <div className="flex items-center gap-1.5 text-[12px] text-slate-500">
          <span
            className={clsx(
              'h-2 w-2 rounded-full',
              connectionState === 'connected'
                ? 'animate-pulse bg-emerald-400'
                : 'bg-slate-600',
            )}
          />
          {connectionState === 'connected' ? 'Live' : 'Reconnecting…'}
        </div>

        <div className="ml-auto flex items-center gap-3 text-[12px] text-slate-400">
          <span>
            <span className="font-semibold text-emerald-400">
              {fleet.active}
            </span>{' '}
            working
          </span>
          <span>
            <span className="font-semibold text-amber-400">
              {fleet.idle}
            </span>{' '}
            idle
          </span>
          <span>
            <span className="font-semibold text-slate-500">
              {fleet.offline}
            </span>{' '}
            offline
          </span>
        </div>
      </div>

      {/* Body: attention rail + infinite lane pane. */}
      <div className="flex min-h-0 flex-1 gap-3 p-3">
        {isLoading && (
          <div className="flex flex-1 items-center justify-center">
            <Spinner size="lg" label="Loading command center…" />
          </div>
        )}

        {isError && (
          <div className="flex flex-1 flex-col items-center justify-center gap-3">
            <p className="text-sm text-slate-400">
              Failed to load the command feed.
            </p>
            <button
              type="button"
              onClick={() => void refetch()}
              className="rounded-md bg-sky-600 px-3 py-1.5 text-sm text-white hover:bg-sky-500"
            >
              Retry
            </button>
          </div>
        )}

        {!isLoading && !isError && (
          <>
            <AttentionRail
              items={attention}
              onSelect={handleAttentionSelect}
            />

            <div className="scrollbar-thin flex min-w-0 flex-1 snap-x gap-3 overflow-x-auto pb-1 [overflow-anchor:none]">
              {lanes.length === 0 && (
                <div className="flex flex-1 items-center justify-center">
                  <p className="text-sm text-slate-500">
                    No agents yet — start a Claude Code session with
                    substrate hooks installed and it will appear here.
                  </p>
                </div>
              )}

              {lanes.map((lane) => (
                <AgentLane
                  key={lane.agent.id}
                  lane={lane}
                  summary={summariesByAgent.get(lane.agent.id)}
                  focused={focusedAgentId === lane.agent.id}
                  onToggleFocus={focusLane}
                />
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
