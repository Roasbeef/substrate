// AttentionTray floats over the canvas listing everything actionable
// across the fleet. Selecting an item flies the viewport to the owning
// agent's card.

import { useState } from 'react';
import { clsx } from 'clsx';
import type { AttentionItem } from '@/api/command.js';
import { attentionKinds, timeAgo } from './kinds.js';

export interface AttentionTrayProps {
  items: AttentionItem[];
  onSelect: (item: AttentionItem) => void;
}

export function AttentionTray({ items, onSelect }: AttentionTrayProps) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div
      className="pointer-events-auto w-[300px] overflow-hidden rounded-xl border border-[#E6E4DD] bg-white/95 shadow-[0_1px_2px_rgba(28,32,36,0.05),0_10px_28px_rgba(28,32,36,0.08)] backdrop-blur-sm"
      onPointerDown={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        onClick={() => setCollapsed((v) => !v)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
      >
        <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#6B7280]">
          Needs you
        </span>
        <span
          className={clsx(
            'flex h-[18px] min-w-[18px] items-center justify-center rounded-full px-1 font-mono text-[10.5px] font-bold',
            items.length > 0
              ? 'bg-[#C98A1B]/15 text-[#92610E]'
              : 'bg-[#F1EFE9] text-[#9BA0A6]',
          )}
        >
          {items.length}
        </span>
        <svg
          className={clsx(
            'ml-auto h-3 w-3 text-[#B0ADA4] transition-transform',
            collapsed && '-rotate-90',
          )}
          fill="none" viewBox="0 0 24 24" stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round"
            d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {!collapsed && (
        <div className="scrollbar-thin max-h-[50vh] overflow-y-auto border-t border-[#EDEBE4]">
          {items.length === 0 && (
            <p className="px-3 py-3 text-[12px] text-[#9BA0A6]">
              All clear — nobody is waiting on you.
            </p>
          )}

          {items.map((item) => {
            const kind = attentionKinds[item.kind];
            return (
              <button
                key={`${item.kind}-${item.agent_id}-${item.message_id ?? item.plan_review_id ?? ''}`}
                type="button"
                onClick={() => onSelect(item)}
                className={clsx(
                  'block w-full border-l-2 px-3 py-2 text-left hover:bg-[#F4F3EE]',
                  kind.stripe,
                )}
              >
                <span className="flex items-baseline gap-1.5">
                  <span
                    className={clsx(
                      'rounded-[3px] px-1 py-px font-mono text-[9px] font-semibold uppercase tracking-[0.08em]',
                      kind.chip,
                    )}
                  >
                    {kind.label}
                  </span>
                  <span className="font-mono text-[10.5px] text-[#8A8F96]">
                    {item.agent_name}
                  </span>
                  <span className="ml-auto font-mono text-[10px] text-[#B0ADA4]">
                    {timeAgo(item.created_at)}
                  </span>
                </span>
                <span className="mt-0.5 line-clamp-2 block text-[12.5px] font-medium leading-snug text-[#33383D]">
                  {item.title}
                </span>
                {item.detail && (
                  <span className="line-clamp-1 block text-[11.5px] text-[#8A8F96]">
                    {item.detail}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
