// AgentCanvasCard is one agent's dossier on the command canvas: a
// draggable card with a live heartbeat trace, digest, classified event
// feed, and steering composer. Dragging the header moves the card in
// canvas space; everything inside behaves like a normal document.

import { useMemo, useRef, useState } from 'react';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane, FlowEvent } from '@/api/command.js';
import type { AgentSummary } from '@/types/api.js';
import { useCanvasStore } from '@/stores/canvas.js';
import type { CardPosition, CardSize, Granularity } from '@/stores/canvas.js';
import { useAgentFlow } from '@/hooks/useCommandFeed.js';
import { useUIStore } from '@/stores/ui.js';
import { useSummaryHistory } from '@/hooks/useSummaries.js';
import { EventCard } from './EventCard.js';
import { SteerComposer } from './SteerComposer.js';
import type { PendingAttachment, ReplyTarget } from './SteerComposer.js';
import { HeartbeatTrace } from './HeartbeatTrace.js';
import { agentTint, timeAgo } from './kinds.js';

// Number of timeline items shown before the "older" fold.
const VISIBLE_EVENTS = 8;

// One entry in the merged three-tier timeline.
type TimelineItem =
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

// FlowRow renders one high-granularity transcript event as a faint
// mono line, expandable when a detail is present.
function FlowRow({ flow }: { flow: FlowEvent }) {
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
function SummaryRow({ text, delta, ts }: {
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

export interface AgentCanvasCardProps {
  lane: CommandLane;
  summary?: AgentSummary | undefined;
  position: CardPosition;
  // Explicit size from drag-resizing; undefined means default width
  // with automatic height.
  size?: CardSize | undefined;
  // Current canvas zoom, needed to convert pointer deltas into canvas
  // coordinates while dragging.
  scale: number;
  zIndex: number;
}

export function AgentCanvasCard({
  lane,
  summary,
  position,
  size,
  scale,
  zIndex,
}: AgentCanvasCardProps) {
  const { agent } = lane;
  const [replyTarget, setReplyTarget] = useState<ReplyTarget | null>(
    null,
  );
  const [showAll, setShowAll] = useState(false);
  const [digestOpen, setDigestOpen] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [resizing, setResizing] = useState(false);
  const [dropActive, setDropActive] = useState(false);
  const [pendingAttachments, setPendingAttachments] = useState<
    PendingAttachment[]
  >([]);

  const addToast = useUIStore((st) => st.addToast);

  const setPosition = useCanvasStore((s) => s.setPosition);
  const setSize = useCanvasStore((s) => s.setSize);
  const bringToFront = useCanvasStore((s) => s.bringToFront);
  const granularity =
    useCanvasStore((s) => s.granularity[agent.id]) ?? 'med';
  const setFocusedCard = useCanvasStore((s) => s.setFocusedCard);
  const setFocusedMessage = useCanvasStore((s) => s.setFocusedMessage);
  const setOpenDoc = useCanvasStore((s) => s.setOpenDoc);
  const setGranularity = useCanvasStore((s) => s.setGranularity);

  // Medium tier: Haiku summary history; high tier: transcript flow.
  const { data: history } = useSummaryHistory(
    agent.id, 20, granularity !== 'lo',
  );
  const { data: flowData } = useAgentFlow(
    agent.id, granularity === 'hi',
  );

  // Drag bookkeeping lives in a ref; only the store position renders.
  const dragRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    origX: number;
    origY: number;
    moved: boolean;
  } | null>(null);

  // Resize bookkeeping for the corner handle.
  const resizeRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    origW: number;
    origH: number;
  } | null>(null);

  // Merge the three tiers into one timeline, newest first. Mail is
  // always present; summary history joins at MED; raw flow at HI.
  const timeline = useMemo<TimelineItem[]>(() => {
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

  const visibleItems = useMemo(
    () => (showAll ? timeline : timeline.slice(0, VISIBLE_EVENTS)),
    [timeline, showAll],
  );
  const hiddenCount = timeline.length - visibleItems.length;

  const handleReply = (event: CommandEvent) => {
    setReplyTarget({
      threadId: event.thread_id,
      subject: event.subject,
    });
  };

  // Header drag: pointer capture keeps the gesture on the handle even
  // when the cursor outruns the card.
  const onHeaderPointerDown = (e: React.PointerEvent) => {
    // Only left-button / primary touch drags.
    if (e.button !== 0) {
      return;
    }
    (e.target as Element).setPointerCapture?.(e.pointerId);
    dragRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      origX: position.x,
      origY: position.y,
      moved: false,
    };
    setDragging(true);
    bringToFront(agent.id);
  };

  const onHeaderPointerMove = (e: React.PointerEvent) => {
    const d = dragRef.current;
    if (!d || e.pointerId !== d.pointerId) {
      return;
    }
    const dx = (e.clientX - d.startX) / scale;
    const dy = (e.clientY - d.startY) / scale;
    if (Math.abs(dx) + Math.abs(dy) > 2) {
      d.moved = true;
    }
    setPosition(agent.id, { x: d.origX + dx, y: d.origY + dy });
  };

  const onHeaderPointerUp = (e: React.PointerEvent) => {
    if (dragRef.current?.pointerId === e.pointerId) {
      dragRef.current = null;
      setDragging(false);
    }
  };

  // Corner handle: drag to resize the card in canvas coordinates.
  const onResizePointerDown = (e: React.PointerEvent) => {
    if (e.button !== 0) {
      return;
    }
    e.stopPropagation();
    (e.target as Element).setPointerCapture?.(e.pointerId);
    // Measure the rendered card so auto-height cards don't jump on
    // the first resize. offsetWidth/Height are pre-transform values,
    // i.e. already in canvas units.
    const card = e.currentTarget.parentElement as HTMLElement;
    resizeRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      origW: card?.offsetWidth ?? size?.w ?? 380,
      origH: card?.offsetHeight ?? size?.h ?? 480,
    };
    setResizing(true);
    bringToFront(agent.id);
  };

  const onResizePointerMove = (e: React.PointerEvent) => {
    const r = resizeRef.current;
    if (!r || e.pointerId !== r.pointerId) {
      return;
    }
    setSize(agent.id, {
      w: r.origW + (e.clientX - r.startX) / scale,
      h: r.origH + (e.clientY - r.startY) / scale,
    });
  };

  const onResizePointerUp = (e: React.PointerEvent) => {
    if (resizeRef.current?.pointerId === e.pointerId) {
      resizeRef.current = null;
      setResizing(false);
    }
  };

  // Dropped images upload immediately but stage in the composer so
  // the operator can add a message before anything is sent.
  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDropActive(false);

    const images = Array.from(e.dataTransfer.files).filter((f) =>
      f.type.startsWith('image/'),
    );
    if (images.length === 0) {
      return;
    }

    try {
      const staged: PendingAttachment[] = [];
      for (const img of images) {
        const form = new FormData();
        form.append('file', img);
        const res = await fetch('/api/v1/command/upload', {
          method: 'POST',
          body: form,
        });
        if (!res.ok) {
          throw new Error(`upload failed (${res.status})`);
        }
        const data = (await res.json()) as {
          url: string;
          markdown: string;
        };
        staged.push({
          name: img.name, url: data.url, markdown: data.markdown,
        });
      }
      setPendingAttachments((prev) => [...prev, ...staged]);
    } catch (err) {
      addToast({
        variant: 'error',
        title: 'Attachment failed',
        message:
          err instanceof Error ? err.message : 'Upload error',
      });
    }
  };

  return (
    <article
      id={`card-${agent.id}`}
      aria-label={`Agent ${agent.name}`}
      // Card-local pointer/wheel activity must not pan or zoom the
      // canvas underneath.
      onPointerDown={(e) => {
        e.stopPropagation();
        bringToFront(agent.id);
      }}
      onWheel={(e) => e.stopPropagation()}
      onDragOver={(e) => {
        e.preventDefault();
        e.stopPropagation();
        setDropActive(true);
      }}
      onDragLeave={() => setDropActive(false)}
      onDrop={(e) => void handleDrop(e)}
      className={clsx(
        'absolute flex flex-col overflow-hidden rounded-xl',
        'border bg-[var(--c-card)]',
        dropActive
          ? 'border-[var(--c-steel)] ring-2 ring-[var(--c-steel)]/30 shadow-[0_12px_32px_rgba(28,32,36,0.16)]'
          : dragging || resizing
            ? 'border-[var(--c-ghost)] shadow-[0_12px_32px_rgba(28,32,36,0.16)]'
            : 'border-[var(--c-hair)] shadow-[0_1px_2px_rgba(28,32,36,0.05),0_10px_28px_rgba(28,32,36,0.07)]',
        !resizing && 'transition-shadow duration-150',
      )}
      style={{
        left: 0,
        top: 0,
        transform: `translate(${position.x}px, ${position.y}px)`,
        width: size?.w ?? 380,
        ...(size
          ? { height: size.h }
          : { maxHeight: 640 }),
        zIndex,
      }}
    >
      {/* Drag handle header. */}
      <header
        onPointerDown={onHeaderPointerDown}
        onPointerMove={onHeaderPointerMove}
        onPointerUp={onHeaderPointerUp}
        className={clsx(
          'select-none border-b border-[var(--c-hair2)] px-3.5 pb-2 pt-2.5',
          dragging ? 'cursor-grabbing' : 'cursor-grab',
        )}
        style={{ touchAction: 'none' }}
      >
        <div className="flex items-center gap-2.5">
          <span
            className="flex h-5 w-5 shrink-0 items-center justify-center rounded-[5px] font-mono text-[11px] font-bold text-white"
            style={{ backgroundColor: agentTint(agent.name) }}
          >
            {agent.name.slice(0, 1)}
          </span>
          <h2 className="text-[14.5px] font-semibold tracking-[-0.01em] text-[var(--c-ink)]">
            {agent.name}
          </h2>
          <HeartbeatTrace status={agent.status} />
          <span className="ml-auto font-mono text-[10px] text-[var(--c-faint)]">
            {timeAgo(agent.last_active_at)}
          </span>
          <button
            type="button"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={() => setFocusedCard(agent.id)}
            title="Focus (near-fullscreen)"
            className="rounded p-0.5 text-[var(--c-dim)] hover:bg-[var(--c-hover)] hover:text-[var(--c-ink)]"
          >
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
              stroke="currentColor" strokeWidth={1.8}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          </button>
          {lane.unread_count > 0 && (
            <span className="rounded-full bg-[#33608D]/10 px-1.5 py-px font-mono text-[10px] font-semibold text-[var(--c-steel)]">
              {lane.unread_count}
            </span>
          )}
        </div>
        <p className="mt-0.5 truncate font-mono text-[10.5px] text-[var(--c-faint2)]">
          {agent.project_key || agent.working_dir || 'unassigned'}
          {agent.git_branch && (
            <span className="text-[var(--c-dim)]"> · {agent.git_branch}</span>
          )}
        </p>
        {agent.purpose && (
          <p className="truncate text-[11px] leading-4 text-[var(--c-mut)]">
            {agent.purpose}
          </p>
        )}
      </header>

      {/* Waiting-on flag: the one loud row, and only when a human is
          actually needed. */}
      {lane.waiting_for && (
        <div className="flex items-baseline gap-2 border-b border-[var(--c-hair2)] bg-[#C98A1B]/8 px-3.5 py-1.5">
          <span className="font-mono text-[9.5px] font-semibold uppercase tracking-[0.1em] text-[var(--c-amber)]">
            waiting on you
          </span>
          <span className="truncate text-[12.5px] text-[var(--c-amberink)]">
            {lane.waiting_for}
          </span>
        </div>
      )}

      {/* Digest: clamped by default, click to unfold the full text. */}
      <div className="border-b border-[var(--c-hair2)]">
        {(() => {
          const latest = lane.events.find(
            (ev) => ev.direction !== 'out' && ev.body,
          );
          const main = summary?.summary ?? latest?.body ?? '';

          if (!main) {
            return (
              <p className="px-3.5 py-2.5 text-[13px] italic leading-snug text-[var(--c-faint)]">
                {agent.purpose || 'No live summary yet.'}
              </p>
            );
          }

          return (
            <button
              type="button"
              onClick={() => setDigestOpen((v) => !v)}
              title={digestOpen ? 'Collapse' : 'Expand'}
              className="block w-full px-3.5 py-2.5 text-left hover:bg-[var(--c-hover)]"
            >
              <span
                className={clsx(
                  'block whitespace-pre-line text-[13px] leading-snug',
                  summary?.summary
                    ? 'text-[var(--c-ink2)]'
                    : 'text-[var(--c-text2)]',
                  !digestOpen && 'line-clamp-2',
                )}
              >
                {digestOpen ? main : main.slice(0, 400)}
              </span>
              {summary?.delta && (
                <span
                  className={clsx(
                    'mt-1 block text-[12px] leading-snug text-[var(--c-green)]',
                    !digestOpen && 'line-clamp-1',
                  )}
                >
                  Δ {summary.delta}
                </span>
              )}
            </button>
          );
        })()}
      </div>

      {/* Timeline: merged mail / summary / flow tiers, with the
          granularity dial on the divider. */}
      <div className="flex items-center gap-1 border-b border-[var(--c-hair2)] px-3.5 py-1">
        <span className="font-mono text-[9px] font-semibold uppercase tracking-[0.12em] text-[var(--c-dim)]">
          timeline
        </span>
        <span className="ml-auto flex overflow-hidden rounded-md border border-[var(--c-hair)]">
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
              onClick={() => setGranularity(agent.id, g)}
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
      </div>
      <div className="scrollbar-thin min-h-0 flex-1 divide-y divide-[var(--c-fill)] overflow-y-auto">
        {visibleItems.length === 0 && (
          <p className="px-3.5 py-4 text-center text-[12px] text-[var(--c-dim)]">
            No recent traffic.
          </p>
        )}

        {visibleItems.map((item, i) =>
          item.tier === 'mail' ? (
            <EventCard
              key={`m-${item.event.direction}-${item.event.message_id}`}
              event={item.event}
              onReply={handleReply}
              onOpenDoc={(path) => {
                setFocusedCard(agent.id);
                setOpenDoc({ agentId: agent.id, path });
              }}
              onFocusMessage={(ev) => {
                setFocusedCard(agent.id);
                setFocusedMessage(ev.message_id);
              }}
            />
          ) : item.tier === 'summary' ? (
            <SummaryRow
              key={`s-${item.ts}-${i}`}
              text={item.text}
              delta={item.delta}
              ts={item.ts}
            />
          ) : (
            <FlowRow key={`f-${item.ts}-${i}`} flow={item.flow} />
          ),
        )}

        {hiddenCount > 0 && (
          <button
            type="button"
            onClick={() => setShowAll(true)}
            className="w-full py-1.5 text-center font-mono text-[10.5px] uppercase tracking-[0.08em] text-[var(--c-faint)] hover:bg-[var(--c-hover)] hover:text-[var(--c-text2)]"
          >
            {hiddenCount} older
          </button>
        )}
      </div>

      <SteerComposer
        agentName={agent.name}
        replyTarget={replyTarget}
        onClearReply={() => setReplyTarget(null)}
        attachments={pendingAttachments}
        onRemoveAttachment={(i) =>
          setPendingAttachments((prev) =>
            prev.filter((_, idx) => idx !== i),
          )
        }
        onClearAttachments={() => setPendingAttachments([])}
      />

      {/* Corner resize handle: drag to grow or shrink the card. */}
      <div
        role="presentation"
        title="Drag to resize"
        onPointerDown={onResizePointerDown}
        onPointerMove={onResizePointerMove}
        onPointerUp={onResizePointerUp}
        className="absolute bottom-0 right-0 flex h-5 w-5 cursor-nwse-resize items-end justify-end p-[3px] text-[var(--c-ghost)] hover:text-[var(--c-faint2)]"
        style={{ touchAction: 'none' }}
      >
        <svg className="h-2.5 w-2.5" viewBox="0 0 10 10"
          fill="none" stroke="currentColor" strokeWidth={1.4}
          strokeLinecap="round">
          <path d="M9 1L1 9M9 5L5 9" />
        </svg>
      </div>
    </article>
  );
}
