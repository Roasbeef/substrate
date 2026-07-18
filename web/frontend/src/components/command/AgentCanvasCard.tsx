// AgentCanvasCard is one agent's dossier on the command canvas: a
// draggable card with a live heartbeat trace, digest, classified event
// feed, and steering composer. Dragging the header moves the card in
// canvas space; everything inside behaves like a normal document.

import { useMemo, useRef, useState } from 'react';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane } from '@/api/command.js';
import type { AgentSummary } from '@/types/api.js';
import { useCanvasStore } from '@/stores/canvas.js';
import type { CardPosition, CardSize } from '@/stores/canvas.js';
import { EventCard } from './EventCard.js';
import { SteerComposer } from './SteerComposer.js';
import type { ReplyTarget } from './SteerComposer.js';
import { HeartbeatTrace } from './HeartbeatTrace.js';
import { timeAgo } from './kinds.js';

// Number of events shown before the "show older" fold.
const VISIBLE_EVENTS = 6;

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
  const [dragging, setDragging] = useState(false);
  const [resizing, setResizing] = useState(false);

  const setPosition = useCanvasStore((s) => s.setPosition);
  const setSize = useCanvasStore((s) => s.setSize);
  const bringToFront = useCanvasStore((s) => s.bringToFront);

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

  const visibleEvents = useMemo(
    () =>
      showAll ? lane.events : lane.events.slice(0, VISIBLE_EVENTS),
    [lane.events, showAll],
  );
  const hiddenCount = lane.events.length - visibleEvents.length;

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
      className={clsx(
        'absolute flex flex-col overflow-hidden rounded-xl',
        'border bg-white',
        dragging || resizing
          ? 'border-[#C9C7BF] shadow-[0_12px_32px_rgba(28,32,36,0.16)]'
          : 'border-[#E6E4DD] shadow-[0_1px_2px_rgba(28,32,36,0.05),0_10px_28px_rgba(28,32,36,0.07)]',
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
          'select-none border-b border-[#EDEBE4] px-3.5 pb-2 pt-2.5',
          dragging ? 'cursor-grabbing' : 'cursor-grab',
        )}
        style={{ touchAction: 'none' }}
      >
        <div className="flex items-center gap-2.5">
          <h2 className="text-[14.5px] font-semibold tracking-[-0.01em] text-[#22262A]">
            {agent.name}
          </h2>
          <HeartbeatTrace status={agent.status} />
          <span className="ml-auto font-mono text-[10px] text-[#9BA0A6]">
            {timeAgo(agent.last_active_at)}
          </span>
          {lane.unread_count > 0 && (
            <span className="rounded-full bg-[#33608D]/10 px-1.5 py-px font-mono text-[10px] font-semibold text-[#33608D]">
              {lane.unread_count}
            </span>
          )}
        </div>
        <p className="mt-0.5 truncate font-mono text-[10.5px] text-[#8A8F96]">
          {agent.project_key || 'unassigned'}
          {agent.git_branch && (
            <span className="text-[#B0ADA4]"> · {agent.git_branch}</span>
          )}
        </p>
      </header>

      {/* Waiting-on flag: the one loud row, and only when a human is
          actually needed. */}
      {lane.waiting_for && (
        <div className="flex items-baseline gap-2 border-b border-[#EDEBE4] bg-[#C98A1B]/8 px-3.5 py-1.5">
          <span className="font-mono text-[9.5px] font-semibold uppercase tracking-[0.1em] text-[#92610E]">
            waiting on you
          </span>
          <span className="truncate text-[12.5px] text-[#6d5613]">
            {lane.waiting_for}
          </span>
        </div>
      )}

      {/* Digest. */}
      <div className="border-b border-[#EDEBE4] px-3.5 py-2.5">
        {summary?.summary ? (
          <>
            <p className="text-[13px] leading-snug text-[#33383D]">
              {summary.summary}
            </p>
            {summary.delta && (
              <p className="mt-1 text-[12px] leading-snug text-[#178A5B]">
                Δ {summary.delta}
              </p>
            )}
          </>
        ) : (
          <p className="text-[13px] italic leading-snug text-[#9BA0A6]">
            {agent.purpose || 'No live summary yet.'}
          </p>
        )}
      </div>

      {/* Event feed. */}
      <div className="scrollbar-thin min-h-0 flex-1 divide-y divide-[#F1EFE9] overflow-y-auto">
        {visibleEvents.length === 0 && (
          <p className="px-3.5 py-4 text-center text-[12px] text-[#B0ADA4]">
            No recent traffic.
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
            className="w-full py-1.5 text-center font-mono text-[10.5px] uppercase tracking-[0.08em] text-[#9BA0A6] hover:bg-[#F4F3EE] hover:text-[#4A4F55]"
          >
            {hiddenCount} older
          </button>
        )}
      </div>

      <SteerComposer
        agentName={agent.name}
        replyTarget={replyTarget}
        onClearReply={() => setReplyTarget(null)}
      />

      {/* Corner resize handle: drag to grow or shrink the card. */}
      <div
        role="presentation"
        title="Drag to resize"
        onPointerDown={onResizePointerDown}
        onPointerMove={onResizePointerMove}
        onPointerUp={onResizePointerUp}
        className="absolute bottom-0 right-0 flex h-5 w-5 cursor-nwse-resize items-end justify-end p-[3px] text-[#C9C7BF] hover:text-[#8A8F96]"
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
