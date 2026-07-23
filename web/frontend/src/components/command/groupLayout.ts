// groupLayout computes the auto-arranged "group by project" canvas
// layout: lanes are clustered by the repo they belong to, each
// cluster gets a labeled block, and blocks shelf-pack left to right
// so big fleets stay browsable without manual arrangement.

import type { CommandLane } from '@/api/command.js';
import type { CardPosition, CardSize } from '@/stores/canvas.js';
import { CARD_WIDTH, CARD_GAP } from '@/stores/canvas.js';

// Estimated card height for layout when the operator has not resized
// a card; matches the auto-height cap in AgentCanvasCard.
const CARD_HEIGHT_EST = 460;

// Block chrome: label strip height and inner padding around cards.
const LABEL_H = 40;
const BLOCK_PAD = 24;
const BLOCK_GAP = 56;

// Shelf-packing wraps to a new row of blocks past this world width.
const SHELF_MAX_W = 3800;

// A labeled cluster region rendered behind its cards.
export interface GroupBlock {
  label: string;
  x: number;
  y: number;
  w: number;
  h: number;
  count: number;
  liveCount: number;
}

export interface GroupLayout {
  positions: Record<number, CardPosition>;
  blocks: GroupBlock[];
}

// projectLabel derives a short repo-ish grouping key for a lane. The
// project key is preferred ("substrate.git/branch" → "substrate");
// otherwise the working directory's last path segment is used, with
// GitHub-style paths shortened to owner/repo.
export function projectLabel(lane: CommandLane): string {
  const key = lane.agent.project_key;
  if (key) {
    const repo = key.split('/')[0] ?? key;
    return repo.replace(/\.git$/, '') || 'unassigned';
  }

  const dir = lane.agent.working_dir;
  if (dir) {
    const gh = dir.match(/github\.com\/[^/]+\/([^/]+)/);
    if (gh?.[1]) {
      return gh[1];
    }
    const parts = dir.replace(/\/+$/, '').split('/').filter(Boolean);
    return parts[parts.length - 1] ?? 'unassigned';
  }

  return 'unassigned';
}

// isLiveStatus mirrors the canvas page's liveness bucketing.
function isLiveStatus(status: string): boolean {
  return status === 'active' || status === 'busy';
}

// layoutGroups arranges lanes into per-project blocks. Cards keep any
// operator-set size; groups with live agents sort first, then by
// size, then name, so the working clusters land top-left.
export function layoutGroups(
  lanes: CommandLane[],
  sizes: Record<number, CardSize>,
): GroupLayout {
  const groups = new Map<string, CommandLane[]>();
  for (const lane of lanes) {
    const label = projectLabel(lane);
    const list = groups.get(label);
    if (list) {
      list.push(lane);
    } else {
      groups.set(label, [lane]);
    }
  }

  const ordered = [...groups.entries()].sort(([la, a], [lb, b]) => {
    const liveA = a.filter((l) => isLiveStatus(l.agent.status)).length;
    const liveB = b.filter((l) => isLiveStatus(l.agent.status)).length;
    if (liveA !== liveB) {
      return liveB - liveA;
    }
    if (a.length !== b.length) {
      return b.length - a.length;
    }
    return la.localeCompare(lb);
  });

  const positions: Record<number, CardPosition> = {};
  const blocks: GroupBlock[] = [];

  // Shelf packing state: blocks flow left to right, wrapping to a new
  // shelf whose top sits under the tallest block of the previous one.
  let shelfX = 0;
  let shelfY = 0;
  let shelfH = 0;

  for (const [label, members] of ordered) {
    // Live members first inside a block, so a cluster's working
    // agents sit in its top row.
    const sorted = [...members].sort((a, b) => {
      const liveA = isLiveStatus(a.agent.status) ? 0 : 1;
      const liveB = isLiveStatus(b.agent.status) ? 0 : 1;
      if (liveA !== liveB) {
        return liveA - liveB;
      }
      return a.agent.name.localeCompare(b.agent.name);
    });

    const cols = Math.min(
      3, Math.max(1, Math.ceil(Math.sqrt(sorted.length))),
    );
    const rows = Math.ceil(sorted.length / cols);

    // Per-column widths and per-row heights honor resized cards.
    const colW = new Array<number>(cols).fill(CARD_WIDTH);
    const rowH = new Array<number>(rows).fill(CARD_HEIGHT_EST);
    sorted.forEach((lane, i) => {
      const s = sizes[lane.agent.id];
      const col = i % cols;
      const row = Math.floor(i / cols);
      colW[col] = Math.max(colW[col] ?? CARD_WIDTH, s?.w ?? CARD_WIDTH);
      rowH[row] = Math.max(
        rowH[row] ?? CARD_HEIGHT_EST, s?.h ?? CARD_HEIGHT_EST,
      );
    });

    const innerW =
      colW.reduce((a, b) => a + b, 0) + (cols - 1) * CARD_GAP;
    const innerH =
      rowH.reduce((a, b) => a + b, 0) + (rows - 1) * CARD_GAP;
    const blockW = innerW + BLOCK_PAD * 2;
    const blockH = innerH + BLOCK_PAD * 2 + LABEL_H;

    if (shelfX > 0 && shelfX + blockW > SHELF_MAX_W) {
      shelfX = 0;
      shelfY += shelfH + BLOCK_GAP;
      shelfH = 0;
    }

    const liveCount = members.filter((l) =>
      isLiveStatus(l.agent.status),
    ).length;
    blocks.push({
      label, x: shelfX, y: shelfY, w: blockW, h: blockH,
      count: members.length, liveCount,
    });

    sorted.forEach((lane, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      const x =
        shelfX + BLOCK_PAD +
        colW.slice(0, col).reduce((a, b) => a + b, 0) +
        col * CARD_GAP;
      const y =
        shelfY + LABEL_H + BLOCK_PAD +
        rowH.slice(0, row).reduce((a, b) => a + b, 0) +
        row * CARD_GAP;
      positions[lane.agent.id] = { x, y };
    });

    shelfX += blockW + BLOCK_GAP;
    shelfH = Math.max(shelfH, blockH);
  }

  return { positions, blocks };
}
