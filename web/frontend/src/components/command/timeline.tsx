// Shared three-tier timeline machinery for the command center. The
// canvas cards and focus mode both merge mail (low), Haiku summary
// history (medium), and raw transcript flow (high) into one stream,
// dialed per agent via the persisted granularity setting.

import { useMemo, useState } from 'react';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane, FlowEvent } from '@/api/command.js';
import { useAgentFlow } from '@/hooks/useCommandFeed.js';
import { useSummaryHistory } from '@/hooks/useSummaries.js';
import { useCanvasStore } from '@/stores/canvas.js';
import type { Granularity } from '@/stores/canvas.js';
import { timeAgo } from './kinds.js';

// One entry in the merged three-tier timeline.
export type TimelineItem =
  | { tier: 'mail'; ts: string; event: CommandEvent }
  | { tier: 'summary'; ts: string; text: string; delta: string }
  | { tier: 'flow'; ts: string; flow: FlowEvent };

// Glyphs for flow event kinds, kept to quiet mono marks.
const flowGlyph: Record<FlowEvent['kind'], string> = {
  tool: '$',
  thinking: '~',
  text: '¶',
  prompt: '›',
};

// useAgentTimeline fetches the medium and high tiers as the dial
// demands and merges them with the lane's mail, newest first.
export function useAgentTimeline(
  lane: CommandLane,
  granularity: Granularity,
): TimelineItem[] {
  const { data: history } = useSummaryHistory(
    lane.agent.id, 20, granularity !== 'lo',
  );
  const { data: flowData } = useAgentFlow(
    lane.agent.id, granularity === 'hi',
  );

  return useMemo<TimelineItem[]>(() => {
    const items: TimelineItem[] = lane.events.map((event) => ({
      tier: 'mail', ts: event.created_at, event,
    }));

    if (granularity !== 'lo') {
      for (const h of history ?? []) {
        items.push({
          tier: 'summary', ts: h.created_at,
          text: h.summary, delta: h.delta,
        });
      }
    }

    if (granularity === 'hi') {
      for (const f of flowData?.events ?? []) {
        items.push({ tier: 'flow', ts: f.timestamp, flow: f });
      }
    }

    items.sort((a, b) => (a.ts < b.ts ? 1 : -1));
    return items;
  }, [lane.events, history, flowData, granularity]);
}

// GranularityDial is the LO / MED / HI selector, persisted per agent.
export function GranularityDial({ agentId }: { agentId: number }) {
  const granularity =
    useCanvasStore((s) => s.granularity[agentId]) ?? 'med';
  const setGranularity = useCanvasStore((s) => s.setGranularity);

  return (
    <span className="flex overflow-hidden rounded-md border border-[var(--c-hair)]">
      {(['lo', 'med', 'hi'] as Granularity[]).map((g) => (
        <button
          key={g}
          type="button"
          title={
            g === 'lo'
              ? 'Mail only'
              : g === 'med'
                ? 'Mail + activity summaries'
                : 'Everything incl. raw agent flow'
          }
          onClick={() => setGranularity(agentId, g)}
          className={clsx(
            'px-1.5 py-px font-mono text-[9px] font-semibold uppercase',
            granularity === g
              ? 'bg-[var(--c-ink)] text-white'
              : 'text-[var(--c-faint)] hover:bg-[var(--c-fill)]',
          )}
        >
          {g}
        </button>
      ))}
    </span>
  );
}

// FlowRow renders one high-granularity transcript event as a faint
// mono line, expandable when a detail is present.
export function FlowRow({ flow }: { flow: FlowEvent }) {
  const [open, setOpen] = useState(false);
  return (
    <button
      type="button"
      onClick={() => flow.detail && setOpen((v) => !v)}
      className="flex w-full items-baseline gap-2 px-3 py-[3px] text-left hover:bg-[var(--c-paper)]"
    >
      <span className="w-3 shrink-0 text-center font-mono text-[10px] text-[var(--c-ghost)]">
        {flowGlyph[flow.kind]}
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={clsx(
            'block truncate font-mono text-[10.5px] leading-4',
            flow.kind === 'tool' ? 'text-[var(--c-mut)]' : 'text-[var(--c-faint)]',
            flow.kind === 'thinking' && 'italic',
          )}
        >
          {flow.kind === 'tool' && flow.detail
            ? `${flow.label} ${flow.detail}`
            : flow.label}
        </span>
        {open && flow.detail && flow.kind !== 'tool' && (
          <span className="block whitespace-pre-wrap font-mono text-[10.5px] leading-4 text-[var(--c-mut)]">
            {flow.detail}
          </span>
        )}
      </span>
      <span className="shrink-0 font-mono text-[9.5px] text-[var(--c-ghost)]">
        {timeAgo(flow.timestamp)}
      </span>
    </button>
  );
}

// SummaryRow renders one Haiku digest history entry: the medium tier.
export function SummaryRow({ text, delta, ts }: {
  text: string; delta: string; ts: string;
}) {
  return (
    <div className="flex items-baseline gap-2 px-3 py-1.5">
      <span className="w-3 shrink-0 text-center font-mono text-[10px] font-bold text-[var(--c-green)]">
        Δ
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-[12px] leading-snug text-[var(--c-text2)]">
          {delta || text}
        </span>
      </span>
      <span className="shrink-0 font-mono text-[9.5px] text-[var(--c-dim)]">
        {timeAgo(ts)}
      </span>
    </div>
  );
}
