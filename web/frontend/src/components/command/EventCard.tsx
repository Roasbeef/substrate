// EventCard renders one classified feed entry inside an agent lane.
// Non-actionable events collapse to a single line; actionable events
// expand with inline markdown, diff viewing, and plan approval.

import { lazy, Suspense, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { clsx } from 'clsx';
import type { CommandEvent } from '@/api/command.js';
import { renderMarkdownToHtml } from '@/lib/markdown.js';
import { useUpdatePlanReviewStatus } from '@/hooks/usePlanReviews.js';
import { commandKeys } from '@/hooks/useCommandFeed.js';
import { Spinner } from '@/components/ui/Spinner.js';
import { eventKinds, timeAgo } from './kinds.js';

// DiffViewer is heavy (syntax highlighting); load it on demand.
const DiffViewer = lazy(() =>
  import('@/components/reviews/DiffViewer.js').then((m) => ({
    default: m.DiffViewer,
  })),
);

// Marker used by `substrate send-diff` to embed patches in bodies.
const DIFF_MARKER = '<!-- substrate:diff -->';

// Split a message body into markdown text and an optional diff patch.
function splitBodyAndDiff(body: string): {
  text: string;
  patch: string | null;
} {
  const idx = body.indexOf(DIFF_MARKER);
  if (idx === -1) {
    return { text: body, patch: null };
  }
  return {
    text: body.slice(0, idx).trimEnd(),
    patch: body.slice(idx + DIFF_MARKER.length).trim(),
  };
}

export interface EventCardProps {
  event: CommandEvent;
  // Called when the user hits "Reply" so the lane composer can focus
  // with the thread context attached.
  onReply: (event: CommandEvent) => void;
}

export function EventCard({ event, onReply }: EventCardProps) {
  // Actionable events start expanded so the pane shows work items
  // without a click; informational ones start collapsed.
  const [expanded, setExpanded] = useState(event.needs_action);
  const [planComment, setPlanComment] = useState('');

  const kind = eventKinds[event.kind] ?? eventKinds.message;
  const updatePlan = useUpdatePlanReviewStatus();
  const queryClient = useQueryClient();

  const { text, patch } = useMemo(
    () => splitBodyAndDiff(event.body),
    [event.body],
  );

  const renderedBody = useMemo(
    () => (expanded ? renderMarkdownToHtml(text) : ''),
    [expanded, text],
  );

  const planPending = event.plan_state === 'pending';

  // Submit a plan decision with the optional inline comment.
  const decidePlan = (
    state: 'approved' | 'rejected' | 'changes_requested',
  ) => {
    if (!event.plan_review_id) {
      return;
    }
    updatePlan.mutate(
      {
        planReviewId: event.plan_review_id,
        state,
        ...(planComment.trim() && { comment: planComment.trim() }),
      },
      {
        onSuccess: () => {
          // Refresh the feed so the plan card reflects the decision.
          void queryClient.invalidateQueries({
            queryKey: commandKeys.feed(),
          });
        },
      },
    );
  };

  return (
    <div
      className={clsx(
        'rounded-lg border border-slate-800 bg-slate-900/70',
        'border-l-2 transition-colors',
        kind.stripe,
        event.needs_action && 'ring-1 ring-inset ring-slate-700/60',
      )}
    >
      {/* Collapsed row: kind icon, subject, time. */}
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-start gap-2 px-3 py-2 text-left"
      >
        <span className={clsx('mt-0.5 shrink-0', kind.text)}>
          {kind.icon}
        </span>
        <span className="min-w-0 flex-1">
          <span
            className={clsx(
              'block truncate text-[13px] leading-5',
              event.state === 'unread'
                ? 'font-semibold text-slate-100'
                : 'text-slate-300',
            )}
          >
            {event.subject}
          </span>
          <span className="flex items-center gap-2 text-[11px] text-slate-500">
            <span className={clsx('rounded px-1 py-px', kind.chip)}>
              {kind.label}
            </span>
            {event.plan_state && event.kind === 'plan' && (
              <span className="uppercase tracking-wide">
                {event.plan_state.replace('_', ' ')}
              </span>
            )}
            <span>{timeAgo(event.created_at)}</span>
          </span>
        </span>
        <svg
          className={clsx(
            'mt-1 h-3 w-3 shrink-0 text-slate-600 transition-transform',
            expanded && 'rotate-180',
          )}
          fill="none" viewBox="0 0 24 24" stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round"
            d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {expanded && (
        <div className="border-t border-slate-800 px-3 py-2">
          {/* Plan decisions live above the plan body so approving
              never requires scrolling past the full plan text. */}
          {event.kind === 'plan' && planPending && (
            <div className="mb-2 space-y-2 rounded-md border border-violet-800/40 bg-violet-500/5 p-2">
              <input
                type="text"
                value={planComment}
                onChange={(e) => setPlanComment(e.target.value)}
                placeholder="Optional comment for the agent…"
                className="w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5 text-[13px] text-slate-200 placeholder:text-slate-600 focus:border-slate-500 focus:outline-none"
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => decidePlan('approved')}
                  disabled={updatePlan.isPending}
                  className="rounded bg-emerald-600 px-2.5 py-1.5 text-xs font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
                >
                  Approve
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('changes_requested')}
                  disabled={updatePlan.isPending}
                  className="rounded border border-slate-600 px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:bg-slate-800 disabled:opacity-50"
                >
                  Request changes
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('rejected')}
                  disabled={updatePlan.isPending}
                  className="rounded border border-rose-700/60 px-2.5 py-1.5 text-xs font-medium text-rose-300 hover:bg-rose-900/30 disabled:opacity-50"
                >
                  Reject
                </button>
              </div>
            </div>
          )}

          {text && (
            <div
              className="scrollbar-thin prose prose-command max-h-96 max-w-none overflow-y-auto text-[13px] leading-relaxed text-slate-300"
              dangerouslySetInnerHTML={{ __html: renderedBody }}
            />
          )}

          {patch && (
            <div className="mt-2 overflow-hidden rounded border border-slate-800">
              <Suspense
                fallback={
                  <div className="flex justify-center p-4">
                    <Spinner size="sm" />
                  </div>
                }
              >
                <DiffViewer patch={patch} initialStyle="unified" />
              </Suspense>
            </div>
          )}

          <div className="mt-2 flex items-center gap-3">
            <button
              type="button"
              onClick={() => onReply(event)}
              className="text-[12px] font-medium text-sky-400 hover:text-sky-300"
            >
              Reply
            </button>
            {event.thread_id && (
              <a
                href={`/thread/${event.thread_id}`}
                className="text-[12px] text-slate-500 hover:text-slate-300"
              >
                Open thread →
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
