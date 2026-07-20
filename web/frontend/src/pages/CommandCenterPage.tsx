// CommandCenterPage is an infinite canvas: every agent is a draggable
// dossier card on a pannable, zoomable surface. Active agents are
// front and center; idle and offline agents sit behind filter toggles.
// A floating attention tray lists everything that needs the operator,
// and a minimap keeps orientation.

import {
  useCallback, useEffect, useMemo, useRef, useState,
} from 'react';
import { clsx } from 'clsx';
import {
  useCommandFeed, useCommandFeedRealtime,
} from '@/hooks/useCommandFeed.js';
import { useAgentSummaries, useSummaryRealtime } from '@/hooks/useSummaries.js';
import {
  useConnectionState, useWebSocketConnection,
} from '@/hooks/useWebSocket.js';
import { useCanvasStore, CARD_WIDTH } from '@/stores/canvas.js';
import { AgentCanvasCard } from '@/components/command/AgentCanvasCard.js';
import { FocusMode } from '@/components/command/FocusMode.js';
import { AttentionTray } from '@/components/command/AttentionTray.js';
import { Minimap } from '@/components/command/Minimap.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { AttentionItem, CommandLane } from '@/api/command.js';

const MIN_SCALE = 0.35;
const MAX_SCALE = 1.6;

// isLive groups busy+active for filtering and display.
function isLive(status: string): boolean {
  return status === 'active' || status === 'busy';
}

// laneNeedsOperator marks lanes that stay visible regardless of
// filters: something on them is waiting for the human.
function laneNeedsOperator(lane: CommandLane): boolean {
  return lane.needs_action_count > 0 || Boolean(lane.waiting_for);
}

export default function CommandCenterPage() {
  useWebSocketConnection();
  useSummaryRealtime();
  useCommandFeedRealtime();

  const connectionState = useConnectionState();
  const { data, isLoading, isError, refetch } = useCommandFeed();
  const { data: summaries } = useAgentSummaries(false);

  const viewport = useCanvasStore((s) => s.viewport);
  const setViewport = useCanvasStore((s) => s.setViewport);
  const positions = useCanvasStore((s) => s.positions);
  const ensurePositions = useCanvasStore((s) => s.ensurePositions);
  const filters = useCanvasStore((s) => s.filters);
  const toggleFilter = useCanvasStore((s) => s.toggleFilter);
  const zOrder = useCanvasStore((s) => s.zOrder);
  const sizes = useCanvasStore((s) => s.sizes);
  const focusedCard = useCanvasStore((s) => s.focusedCard);
  const bringToFront = useCanvasStore((s) => s.bringToFront);

  const containerRef = useRef<HTMLDivElement>(null);
  const [screen, setScreen] = useState({ width: 1280, height: 800 });

  const panRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    origX: number;
    origY: number;
  } | null>(null);
  const [panning, setPanning] = useState(false);

  // Track the visible area for the minimap and fit-to-view.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) {
      return;
    }
    const update = () =>
      setScreen({ width: el.clientWidth, height: el.clientHeight });
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const lanes = useMemo(() => data?.lanes ?? [], [data?.lanes]);
  const attention = data?.attention ?? [];

  // Give every known agent a canvas position.
  useEffect(() => {
    if (lanes.length > 0) {
      ensurePositions(lanes.map((l) => l.agent.id));
    }
  }, [lanes, ensurePositions]);

  const summariesByAgent = useMemo(() => {
    const map = new Map<number, NonNullable<typeof summaries>[number]>();
    for (const s of summaries ?? []) {
      map.set(s.agent_id, s);
    }
    return map;
  }, [summaries]);

  // Filter lanes by liveness, always keeping anyone who needs the
  // operator.
  const visibleLanes = useMemo(
    () =>
      lanes.filter((lane) => {
        if (laneNeedsOperator(lane)) {
          return true;
        }
        if (isLive(lane.agent.status)) {
          return filters.active;
        }
        if (lane.agent.status === 'idle') {
          return filters.idle;
        }
        return filters.offline;
      }),
    [lanes, filters],
  );

  const counts = useMemo(() => {
    const c = { live: 0, idle: 0, offline: 0 };
    for (const lane of lanes) {
      if (isLive(lane.agent.status)) {
        c.live++;
      } else if (lane.agent.status === 'idle') {
        c.idle++;
      } else {
        c.offline++;
      }
    }
    return c;
  }, [lanes]);

  // ------ Canvas gestures ------

  const onBackgroundPointerDown = (e: React.PointerEvent) => {
    if (e.button !== 0 && e.button !== 1) {
      return;
    }
    (e.currentTarget as Element).setPointerCapture?.(e.pointerId);
    panRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      origX: viewport.x,
      origY: viewport.y,
    };
    setPanning(true);
  };

  const onBackgroundPointerMove = (e: React.PointerEvent) => {
    const p = panRef.current;
    if (!p || e.pointerId !== p.pointerId) {
      return;
    }
    setViewport({
      x: p.origX + (e.clientX - p.startX),
      y: p.origY + (e.clientY - p.startY),
      scale: viewport.scale,
    });
  };

  const onBackgroundPointerUp = (e: React.PointerEvent) => {
    if (panRef.current?.pointerId === e.pointerId) {
      panRef.current = null;
      setPanning(false);
    }
  };

  // Wheel pans; ctrl/cmd+wheel (and trackpad pinch) zooms about the
  // cursor.
  const onWheel = (e: React.WheelEvent) => {
    if (e.ctrlKey || e.metaKey) {
      const rect = containerRef.current?.getBoundingClientRect();
      const cx = e.clientX - (rect?.left ?? 0);
      const cy = e.clientY - (rect?.top ?? 0);
      const factor = Math.exp(-e.deltaY * 0.0022);
      const next = Math.min(
        MAX_SCALE, Math.max(MIN_SCALE, viewport.scale * factor),
      );
      const k = next / viewport.scale;
      setViewport({
        x: cx - (cx - viewport.x) * k,
        y: cy - (cy - viewport.y) * k,
        scale: next,
      });
    } else {
      setViewport({
        x: viewport.x - e.deltaX,
        y: viewport.y - e.deltaY,
        scale: viewport.scale,
      });
    }
  };

  const zoomBy = useCallback(
    (factor: number) => {
      const cx = screen.width / 2;
      const cy = screen.height / 2;
      const next = Math.min(
        MAX_SCALE, Math.max(MIN_SCALE, viewport.scale * factor),
      );
      const k = next / viewport.scale;
      setViewport({
        x: cx - (cx - viewport.x) * k,
        y: cy - (cy - viewport.y) * k,
        scale: next,
      });
    },
    [screen, viewport, setViewport],
  );

  // fitAll frames every visible card in the viewport.
  const fitAll = useCallback(() => {
    if (visibleLanes.length === 0) {
      setViewport({ x: 0, y: 0, scale: 1 });
      return;
    }
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const lane of visibleLanes) {
      const p = positions[lane.agent.id];
      if (!p) {
        continue;
      }
      const s = sizes[lane.agent.id];
      minX = Math.min(minX, p.x);
      minY = Math.min(minY, p.y);
      maxX = Math.max(maxX, p.x + (s?.w ?? CARD_WIDTH));
      maxY = Math.max(maxY, p.y + (s?.h ?? 460));
    }
    const pad = 48;
    const spanX = maxX - minX + pad * 2;
    const spanY = maxY - minY + pad * 2;
    const scale = Math.min(
      1,
      Math.max(
        MIN_SCALE,
        Math.min(screen.width / spanX, screen.height / spanY),
      ),
    );
    setViewport({
      x: -(minX - pad) * scale +
        (screen.width - spanX * scale) / 2,
      y: -(minY - pad) * scale +
        (screen.height - spanY * scale) / 2,
      scale,
    });
  }, [visibleLanes, positions, sizes, screen, setViewport]);

  // flyTo centers the viewport on an agent's card.
  const flyTo = useCallback(
    (agentId: number) => {
      const p = positions[agentId];
      if (!p) {
        return;
      }
      const scale = Math.max(viewport.scale, 0.8);
      setViewport({
        x: screen.width / 2 - (p.x + CARD_WIDTH / 2) * scale,
        y: screen.height / 2 - (p.y + 200) * scale,
        scale,
      });
      bringToFront(agentId);
    },
    [positions, screen, viewport.scale, setViewport, bringToFront],
  );

  const handleAttentionSelect = useCallback(
    (item: AttentionItem) => flyTo(item.agent_id),
    [flyTo],
  );

  // ------ Render ------

  if (isLoading) {
    return (
      <div className="canvas-dots flex h-full items-center justify-center">
        <Spinner size="lg" label="Loading command canvas…" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="canvas-dots flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-[var(--c-mut)]">
          Failed to load the command feed.
        </p>
        <button
          type="button"
          onClick={() => void refetch()}
          className="rounded-md bg-[var(--c-ink)] px-3 py-1.5 text-sm text-white hover:bg-[var(--c-inkhover)]"
        >
          Retry
        </button>
      </div>
    );
  }

  const dotSize = 24 * viewport.scale;

  return (
    <div
      ref={containerRef}
      className={clsx(
        'canvas-dots relative h-full touch-none overflow-hidden',
        panning ? 'cursor-grabbing' : 'cursor-default',
      )}
      style={{
        backgroundSize: `${dotSize}px ${dotSize}px`,
        backgroundPosition: `${viewport.x}px ${viewport.y}px`,
      }}
      onPointerDown={onBackgroundPointerDown}
      onPointerMove={onBackgroundPointerMove}
      onPointerUp={onBackgroundPointerUp}
      onWheel={onWheel}
    >
      {/* World layer: cards live in canvas coordinates. */}
      <div
        className="absolute left-0 top-0"
        style={{
          transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.scale})`,
          transformOrigin: '0 0',
        }}
      >
        {visibleLanes.map((lane) => {
          const p = positions[lane.agent.id];
          if (!p) {
            return null;
          }
          return (
            <AgentCanvasCard
              key={lane.agent.id}
              lane={lane}
              summary={summariesByAgent.get(lane.agent.id)}
              position={p}
              size={sizes[lane.agent.id]}
              scale={viewport.scale}
              zIndex={zOrder[lane.agent.id] ?? 1}
            />
          );
        })}
      </div>

      {/* Empty state. */}
      {visibleLanes.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <p className="max-w-sm text-center text-sm text-[var(--c-faint)]">
            {lanes.length === 0
              ? 'No agents yet. Start a Claude Code session with substrate hooks installed and it will appear here.'
              : 'Nothing matches the current filters.'}
          </p>
        </div>
      )}

      {/* Focus mode overlay. */}
      {focusedCard !== null &&
        (() => {
          const lane = lanes.find(
            (l) => l.agent.id === focusedCard,
          );
          return lane ? (
            <FocusMode
              lane={lane}
              summary={summariesByAgent.get(lane.agent.id)}
            />
          ) : null;
        })()}

      {/* Overlays. */}
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute left-4 top-4">
          <AttentionTray
            items={attention}
            onSelect={handleAttentionSelect}
          />
        </div>

        {/* Toolbar: liveness filters + zoom. */}
        <div
          className="pointer-events-auto absolute right-4 top-4 flex items-center gap-2"
          onPointerDown={(e) => e.stopPropagation()}
          onWheel={(e) => e.stopPropagation()}
        >
          <div className="flex items-center gap-1.5 rounded-xl border border-[var(--c-hair)] bg-[var(--c-card95)] px-2 py-1.5 shadow-[0_1px_2px_rgba(28,32,36,0.05),0_6px_18px_rgba(28,32,36,0.07)] backdrop-blur-sm">
            <span
              className={clsx(
                'mr-0.5 h-1.5 w-1.5 rounded-full',
                connectionState === 'connected'
                  ? 'animate-pulse-dot bg-[var(--c-green)]'
                  : 'bg-[var(--c-ghost)]',
              )}
              title={
                connectionState === 'connected'
                  ? 'Live'
                  : 'Reconnecting…'
              }
            />
            <FilterChip
              label="live"
              count={counts.live}
              on={filters.active}
              onClick={() => toggleFilter('active')}
            />
            <FilterChip
              label="idle"
              count={counts.idle}
              on={filters.idle}
              onClick={() => toggleFilter('idle')}
            />
            <FilterChip
              label="off"
              count={counts.offline}
              on={filters.offline}
              onClick={() => toggleFilter('offline')}
            />
          </div>

          <div className="flex items-center overflow-hidden rounded-xl border border-[var(--c-hair)] bg-[var(--c-card95)] shadow-[0_1px_2px_rgba(28,32,36,0.05),0_6px_18px_rgba(28,32,36,0.07)] backdrop-blur-sm">
            <ToolButton label="−" title="Zoom out"
              onClick={() => zoomBy(1 / 1.25)} />
            <span className="px-1 font-mono text-[10.5px] text-[var(--c-faint2)]">
              {Math.round(viewport.scale * 100)}%
            </span>
            <ToolButton label="+" title="Zoom in"
              onClick={() => zoomBy(1.25)} />
            <div className="h-4 w-px bg-[var(--c-hair2)]" />
            <ToolButton label="Fit" title="Fit all cards"
              onClick={fitAll} />
          </div>
        </div>

        {/* Minimap. */}
        <div className="absolute bottom-4 right-4">
          <Minimap
            cards={visibleLanes.map((lane) => ({
              id: lane.agent.id,
              position: positions[lane.agent.id] ?? { x: 0, y: 0 },
              needsAction: laneNeedsOperator(lane),
              live: isLive(lane.agent.status),
            }))}
            viewport={viewport}
            screen={screen}
            onJump={(cx, cy) =>
              setViewport({
                x: screen.width / 2 - cx * viewport.scale,
                y: screen.height / 2 - cy * viewport.scale,
                scale: viewport.scale,
              })
            }
          />
        </div>

        {/* Hint line, bottom-left. */}
        <p className="absolute bottom-4 left-4 select-none font-mono text-[10px] text-[var(--c-dim)]">
          drag cards · corner resizes · drag canvas to pan · ⌘+scroll to zoom
        </p>
      </div>
    </div>
  );
}

// FilterChip is a toggle for one liveness bucket, showing its count.
function FilterChip({
  label,
  count,
  on,
  onClick,
}: {
  label: string;
  count: number;
  on: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={on}
      className={clsx(
        'rounded-lg px-2 py-0.5 font-mono text-[10.5px] uppercase tracking-[0.08em]',
        on
          ? 'bg-[var(--c-ink)] text-white'
          : 'text-[var(--c-faint2)] hover:bg-[var(--c-hover)] hover:text-[var(--c-text2)]',
      )}
    >
      {label} {count}
    </button>
  );
}

// ToolButton is a quiet toolbar button.
function ToolButton({
  label,
  title,
  onClick,
}: {
  label: string;
  title: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className="px-2 py-1 font-mono text-[11px] text-[var(--c-text2)] hover:bg-[var(--c-hover)]"
    >
      {label}
    </button>
  );
}
