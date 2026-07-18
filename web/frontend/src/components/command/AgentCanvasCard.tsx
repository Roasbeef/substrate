// AgentCanvasCard is one agent's dossier on the command canvas: a
// draggable card with a live heartbeat trace, digest, classified event
// feed, and steering composer. Dragging the header moves the card in
// canvas space; everything inside behaves like a normal document.

import { useMemo, useRef, useState } from 'react';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane } from '@/api/command.js';
import type { AgentSummary } from '@/types/api.js';
import { useCanvasStore } from '@/stores/canvas.js';
import type { CardPosition } from '@/stores/canvas.js';
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
  // Current canvas zoom, needed to convert pointer deltas into canvas
  // coordinates while dragging.
  scale: number;
  zIndex: number;
  open: boolean;
}

export function AgentCanvasCard({
  lane,
  summary,
  position,
  scale,
  zIndex,
  open,
}: AgentCanvasCardProps) {
  const { agent } = lane;
  const [replyTarget, setReplyTarget] = useState<ReplyTarget | null>(
    null,
  );
  const [showAll, setShowAll] = useState(false);
  const [dragging, setDragging] = useState(false);

  const setPosition = useCanvasStore((s) => s.setPosition);
  const bringToFront = useCanvasStore((s) => s.bringToFront);
  const setOpenCard = useCanvasStore((s) => s.setOpenCard);

  // Drag bookkeeping lives in a ref; only the store position renders.
  const dragRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    origX: number;
    origY: number;
    moved: boolean;
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
        'absolute flex max-h-[640px] flex-col overflow-hidden rounded-xl',
        'border bg-white',
        dragging
          ? 'border-[#C9C7BF] shadow-[0_12px_32px_rgba(28,32,36,0.16)]'
          : 'border-[#E6E4DD] shadow-[0_1px_2px_rgba(28,32,36,0.05),0_10px_28px_rgba(28,32,36,0.07)]',
        'transition-[width,box-shadow] duration-150',
      )}
      style={{
        left: 0,
        top: 0,
        transform: `translate(${position.x}px, ${position.y}px)`,
        width: open ? 600 : 380,
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
          <button
            type="button"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={() => setOpenCard(open ? null : agent.id)}
            title={open ? 'Shrink card' : 'Widen card'}
            className="rounded p-0.5 text-[#B0ADA4] hover:bg-[#F4F3EE] hover:text-[#4A4F55]"
          >
            {open ? (
              <svg className="h-3.5 w-3.5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor"
                strokeWidth={1.8}>
                <path strokeLinecap="round" strokeLinejoin="round"
                  d="M9 9L4 4m0 0v4m0-4h4m7 5l5-5m0 0v4m0-4h-4M9 15l-5 5m0 0v-4m0 4h4m7-5l5 5m0 0v-4m0 4h-4" />
              </svg>
            ) : (
              <svg className="h-3.5 w-3.5" fill="none"
                viewBox="0 0 24 24" stroke="currentColor"
                strokeWidth={1.8}>
                <path strokeLinecap="round" strokeLinejoin="round"
                  d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
            )}
          </button>
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
    </article>
  );
}
