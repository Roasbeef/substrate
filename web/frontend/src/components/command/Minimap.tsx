// Minimap gives spatial orientation on the infinite canvas: card
// footprints plus the current viewport rectangle. Clicking jumps the
// viewport to that spot.

import { useMemo } from 'react';
import type { CardPosition } from '@/stores/canvas.js';
import { CARD_WIDTH } from '@/stores/canvas.js';

// Approximate card height for footprint rendering.
const CARD_H = 380;
const MAP_W = 168;
const MAP_H = 112;

export interface MinimapProps {
  cards: Array<{
    id: number;
    position: CardPosition;
    needsAction: boolean;
    live: boolean;
  }>;
  viewport: { x: number; y: number; scale: number };
  screen: { width: number; height: number };
  onJump: (canvasX: number, canvasY: number) => void;
}

export function Minimap({
  cards,
  viewport,
  screen,
  onJump,
}: MinimapProps) {
  // World bounds cover all cards plus the visible viewport rect.
  const world = useMemo(() => {
    const viewX = -viewport.x / viewport.scale;
    const viewY = -viewport.y / viewport.scale;
    const viewW = screen.width / viewport.scale;
    const viewH = screen.height / viewport.scale;

    let minX = viewX;
    let minY = viewY;
    let maxX = viewX + viewW;
    let maxY = viewY + viewH;
    for (const c of cards) {
      minX = Math.min(minX, c.position.x);
      minY = Math.min(minY, c.position.y);
      maxX = Math.max(maxX, c.position.x + CARD_WIDTH);
      maxY = Math.max(maxY, c.position.y + CARD_H);
    }
    // Padding so footprints never hug the edge.
    const pad = 120;
    minX -= pad; minY -= pad; maxX += pad; maxY += pad;

    const spanX = maxX - minX;
    const spanY = maxY - minY;
    const k = Math.min(MAP_W / spanX, MAP_H / spanY);
    return {
      minX, minY, k,
      view: { x: viewX, y: viewY, w: viewW, h: viewH },
    };
  }, [cards, viewport, screen]);

  const toMap = (x: number, y: number) => ({
    x: (x - world.minX) * world.k,
    y: (y - world.minY) * world.k,
  });

  const handleClick = (e: React.MouseEvent<SVGSVGElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    onJump(mx / world.k + world.minX, my / world.k + world.minY);
  };

  const viewRect = toMap(world.view.x, world.view.y);

  return (
    <div
      className="pointer-events-auto overflow-hidden rounded-lg border border-[#E6E4DD] bg-white/95 shadow-[0_1px_2px_rgba(28,32,36,0.05),0_6px_18px_rgba(28,32,36,0.07)] backdrop-blur-sm"
      onPointerDown={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      <svg
        width={MAP_W}
        height={MAP_H}
        onClick={handleClick}
        className="block cursor-pointer"
        role="img"
        aria-label="Canvas minimap"
      >
        {cards.map((c) => {
          const p = toMap(c.position.x, c.position.y);
          return (
            <rect
              key={c.id}
              x={p.x}
              y={p.y}
              width={Math.max(4, CARD_WIDTH * world.k)}
              height={Math.max(3, CARD_H * world.k)}
              rx={1.5}
              fill={
                c.needsAction
                  ? '#C98A1B'
                  : c.live
                    ? '#178A5B'
                    : '#C9C7BF'
              }
              opacity={c.needsAction ? 0.9 : 0.55}
            />
          );
        })}
        <rect
          x={viewRect.x}
          y={viewRect.y}
          width={world.view.w * world.k}
          height={world.view.h * world.k}
          fill="none"
          stroke="#22262A"
          strokeWidth={1.2}
          rx={2}
        />
      </svg>
    </div>
  );
}
