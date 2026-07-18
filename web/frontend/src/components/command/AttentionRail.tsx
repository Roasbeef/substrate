// AttentionRail is the "needs you" queue on the left of the command
// center: every plan awaiting approval, open question, urgent message,
// and blocked agent across the fleet, most pressing first. Clicking an
// item focuses the owning agent's lane.

import { clsx } from 'clsx';
import type { AttentionItem } from '@/api/command.js';
import { attentionKinds, timeAgo } from './kinds.js';

export interface AttentionRailProps {
  items: AttentionItem[];
  onSelect: (item: AttentionItem) => void;
  className?: string;
}

export function AttentionRail({
  items,
  onSelect,
  className,
}: AttentionRailProps) {
  return (
    <aside
      aria-label="Needs your attention"
      className={clsx(
        'flex h-full w-72 shrink-0 flex-col overflow-hidden',
        'rounded-xl border border-slate-800 bg-slate-900/60',
        className,
      )}
    >
      <header className="flex items-center gap-2 border-b border-slate-800 px-3 py-2.5">
        <span
          className={clsx(
            'flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-bold',
            items.length > 0
              ? 'bg-amber-500/20 text-amber-300'
              : 'bg-slate-800 text-slate-500',
          )}
        >
          {items.length}
        </span>
        <h2 className="text-sm font-semibold text-slate-200">
          Needs you
        </h2>
      </header>

      <div className="scrollbar-thin flex-1 space-y-1.5 overflow-y-auto p-2">
        {items.length === 0 && (
          <div className="px-2 pt-6 text-center">
            <p className="text-2xl">✓</p>
            <p className="mt-1 text-[13px] text-slate-500">
              All clear — every agent is unblocked.
            </p>
          </div>
        )}

        {items.map((item) => {
          const kind = attentionKinds[item.kind];
          return (
            <button
              key={`${item.kind}-${item.agent_id}-${item.message_id ?? item.plan_review_id ?? ''}`}
              type="button"
              onClick={() => onSelect(item)}
              className={clsx(
                'w-full rounded-lg border border-slate-800 border-l-2 bg-slate-900/80 px-2.5 py-2 text-left',
                'transition-colors hover:border-slate-700 hover:bg-slate-800/80',
                kind.stripe,
              )}
            >
              <div className="flex items-center gap-1.5">
                <span className={kind.text}>{kind.icon}</span>
                <span
                  className={clsx(
                    'rounded px-1 py-px text-[10px] font-semibold uppercase tracking-wide',
                    kind.chip,
                  )}
                >
                  {kind.label}
                </span>
                <span className="ml-auto text-[11px] text-slate-500">
                  {timeAgo(item.created_at)}
                </span>
              </div>
              <p className="mt-1 line-clamp-2 text-[13px] font-medium leading-snug text-slate-200">
                {item.title}
              </p>
              {item.detail && (
                <p className="mt-0.5 line-clamp-1 text-[12px] text-slate-400">
                  {item.detail}
                </p>
              )}
              <p className="mt-0.5 font-mono text-[11px] text-slate-500">
                {item.agent_name}
              </p>
            </button>
          );
        })}
      </div>
    </aside>
  );
}
