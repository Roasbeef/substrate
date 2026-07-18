// Zustand store for the command canvas: card positions, viewport
// pan/zoom, and status filters. Everything persists to localStorage so
// the operator's spatial arrangement survives reloads.

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

// A card's position in canvas coordinates (unscaled).
export interface CardPosition {
  x: number;
  y: number;
}

// The viewport transform: canvas offset and zoom scale.
export interface Viewport {
  x: number;
  y: number;
  scale: number;
}

// Which liveness buckets are visible on the canvas.
export interface StatusFilters {
  active: boolean;
  idle: boolean;
  offline: boolean;
}

// Default card footprint used for auto-placement collision checks.
export const CARD_WIDTH = 380;
export const CARD_GAP = 28;
const CARD_HEIGHT_ESTIMATE = 420;

interface CanvasState {
  positions: Record<number, CardPosition>;
  viewport: Viewport;
  filters: StatusFilters;
  // Monotonic counter handing out z-order; the last touched card sits
  // on top.
  zTop: number;
  zOrder: Record<number, number>;
  openCard: number | null;

  setPosition: (agentId: number, pos: CardPosition) => void;
  ensurePositions: (agentIds: number[]) => void;
  setViewport: (v: Viewport) => void;
  toggleFilter: (key: keyof StatusFilters) => void;
  bringToFront: (agentId: number) => void;
  setOpenCard: (agentId: number | null) => void;
}

// autoPlace lays out cards without stored positions in rows of three,
// skipping spots already occupied by user-arranged cards.
function autoPlace(
  existing: Record<number, CardPosition>,
  agentIds: number[],
): Record<number, CardPosition> {
  const next = { ...existing };
  const taken = Object.values(next);

  const collides = (x: number, y: number) =>
    taken.some(
      (p) =>
        Math.abs(p.x - x) < CARD_WIDTH + CARD_GAP &&
        Math.abs(p.y - y) < CARD_HEIGHT_ESTIMATE,
    );

  let slot = 0;
  for (const id of agentIds) {
    if (next[id]) {
      continue;
    }
    // Walk grid slots until a free one turns up.
    for (;;) {
      // Start to the right of the attention tray overlay.
      const col = slot % 3;
      const row = Math.floor(slot / 3);
      const x = 360 + col * (CARD_WIDTH + CARD_GAP);
      const y = 60 + row * (CARD_HEIGHT_ESTIMATE + CARD_GAP);
      slot++;
      if (!collides(x, y)) {
        next[id] = { x, y };
        taken.push(next[id]);
        break;
      }
    }
  }

  return next;
}

export const useCanvasStore = create<CanvasState>()(
  persist(
    (set, get) => ({
      positions: {},
      viewport: { x: 0, y: 0, scale: 1 },
      filters: { active: true, idle: false, offline: false },
      zTop: 1,
      zOrder: {},
      openCard: null,

      setPosition: (agentId, pos) =>
        set((s) => ({
          positions: { ...s.positions, [agentId]: pos },
        })),

      ensurePositions: (agentIds) => {
        const { positions } = get();
        const missing = agentIds.filter((id) => !positions[id]);
        if (missing.length === 0) {
          return;
        }
        set({ positions: autoPlace(positions, agentIds) });
      },

      setViewport: (viewport) => set({ viewport }),

      toggleFilter: (key) =>
        set((s) => ({
          filters: { ...s.filters, [key]: !s.filters[key] },
        })),

      bringToFront: (agentId) =>
        set((s) => ({
          zTop: s.zTop + 1,
          zOrder: { ...s.zOrder, [agentId]: s.zTop + 1 },
        })),

      setOpenCard: (openCard) => set({ openCard }),
    }),
    {
      name: 'substrate-command-canvas',
      partialize: (s) => ({
        positions: s.positions,
        viewport: s.viewport,
        filters: s.filters,
      }),
    },
  ),
);
