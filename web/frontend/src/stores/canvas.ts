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

// A card's explicit size, set by drag-resizing. Cards without a stored
// size use the default width with automatic height.
export interface CardSize {
  w: number;
  h: number;
}

// Timeline granularity per card: mail only (lo), mail + Haiku summary
// history (med), or everything including raw transcript flow (hi).
export type Granularity = 'lo' | 'med' | 'hi';

// Resize clamps keep cards usable at both extremes.
export const MIN_CARD_W = 320;
export const MAX_CARD_W = 820;
export const MIN_CARD_H = 220;
export const MAX_CARD_H = 940;

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

// An unsent composer draft, keyed by agent name. Persisted so text
// typed into a card's composer survives card remounts (panning, focus
// mode, feed refreshes) and full page reloads.
export interface ComposerDraft {
  text: string;
  urgent: boolean;
}

// Default card footprint used for auto-placement collision checks.
export const CARD_WIDTH = 380;
export const CARD_GAP = 28;
const CARD_HEIGHT_ESTIMATE = 420;

interface CanvasState {
  positions: Record<number, CardPosition>;
  sizes: Record<number, CardSize>;
  viewport: Viewport;
  filters: StatusFilters;
  // Monotonic counter handing out z-order; the last touched card sits
  // on top.
  zTop: number;
  zOrder: Record<number, number>;

  granularity: Record<number, Granularity>;
  // Unsent composer drafts keyed by agent name.
  drafts: Record<string, ComposerDraft>;
  // Free-text card filter matched against name, project, branch, and
  // purpose. Session-only so a stale query never hides the fleet
  // after a reload.
  query: string;
  // When true, agents with no mail traffic at all are hidden. They
  // are usually hook-registration noise rather than working agents.
  showQuiet: boolean;
  // When true, cards are auto-arranged into labeled clusters by
  // project instead of using the manually dragged positions. Manual
  // positions are kept and restored when toggled back off.
  groupByProject: boolean;
  // Focus mode: agent snapped near-fullscreen, with an optional open
  // document shown in its right rail. Session-only state.
  focusedCard: number | null;
  focusedMessage: number | null;
  openDoc: { agentId: number; path: string } | null;

  setDraft: (agentName: string, draft: ComposerDraft) => void;
  clearDraft: (agentName: string) => void;
  setQuery: (query: string) => void;
  toggleQuiet: () => void;
  toggleGroupByProject: () => void;
  setPosition: (agentId: number, pos: CardPosition) => void;
  setSize: (agentId: number, size: CardSize) => void;
  setGranularity: (agentId: number, g: Granularity) => void;
  setFocusedCard: (agentId: number | null) => void;
  setFocusedMessage: (messageId: number | null) => void;
  setOpenDoc: (doc: { agentId: number; path: string } | null) => void;
  ensurePositions: (agentIds: number[]) => void;
  setViewport: (v: Viewport) => void;
  toggleFilter: (key: keyof StatusFilters) => void;
  bringToFront: (agentId: number) => void;
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
      sizes: {},
      granularity: {},
      drafts: {},
      query: '',
      showQuiet: true,
      groupByProject: false,
      focusedCard: null,
      focusedMessage: null,
      openDoc: null,
      viewport: { x: 0, y: 0, scale: 1 },
      filters: { active: true, idle: false, offline: false },
      zTop: 1,
      zOrder: {},

      setQuery: (query) => set({ query }),

      toggleQuiet: () =>
        set((s) => ({ showQuiet: !s.showQuiet })),

      toggleGroupByProject: () =>
        set((s) => ({ groupByProject: !s.groupByProject })),

      setDraft: (agentName, draft) =>
        set((s) => ({
          drafts: { ...s.drafts, [agentName]: draft },
        })),

      clearDraft: (agentName) =>
        set((s) => {
          if (!(agentName in s.drafts)) {
            return s;
          }
          const next = { ...s.drafts };
          delete next[agentName];
          return { drafts: next };
        }),

      setPosition: (agentId, pos) =>
        set((s) => ({
          positions: { ...s.positions, [agentId]: pos },
        })),

      setSize: (agentId, size) =>
        set((s) => ({
          sizes: {
            ...s.sizes,
            [agentId]: {
              w: Math.min(
                MAX_CARD_W, Math.max(MIN_CARD_W, size.w),
              ),
              h: Math.min(
                MAX_CARD_H, Math.max(MIN_CARD_H, size.h),
              ),
            },
          },
        })),

      ensurePositions: (agentIds) => {
        const { positions } = get();
        const missing = agentIds.filter((id) => !positions[id]);
        if (missing.length === 0) {
          return;
        }
        set({ positions: autoPlace(positions, agentIds) });
      },

      setFocusedCard: (focusedCard) =>
        set(focusedCard === null
          ? { focusedCard, openDoc: null, focusedMessage: null }
          : { focusedCard }),

      setFocusedMessage: (focusedMessage) =>
        set({ focusedMessage }),

      setOpenDoc: (openDoc) => set({ openDoc }),

      setGranularity: (agentId, g) =>
        set((s) => ({
          granularity: { ...s.granularity, [agentId]: g },
        })),

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
    }),
    {
      name: 'substrate-command-canvas',
      partialize: (s) => ({
        positions: s.positions,
        sizes: s.sizes,
        granularity: s.granularity,
        drafts: s.drafts,
        viewport: s.viewport,
        filters: s.filters,
        showQuiet: s.showQuiet,
        groupByProject: s.groupByProject,
      }),
    },
  ),
);
