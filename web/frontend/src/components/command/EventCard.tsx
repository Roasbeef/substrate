// EventCard renders one classified feed entry inside an agent card.
// Non-actionable events collapse to a single quiet row; actionable
// events expand with inline markdown, diff viewing, and plan approval.

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
  // Called when the user hits "Reply" so the card composer can focus
  // with the thread context attached.
  onReply: (event: CommandEvent) => void;
}

export function EventCard({ event, onReply }: EventCardProps) {
  // Actionable events start expanded so the canvas shows work items
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
        'border-l-2 transition-colors',
        expanded ? kind.stripe : 'border-l-transparent',
      )}
    >
      {/* Collapsed row: kind icon, subject, time. */}
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-baseline gap-2 px-3 py-[7px] text-left hover:bg-[#F4F3EE]"
      >
        <span
          className={clsx(
            'relative top-[1.5px] shrink-0',
            kind.text,
          )}
        >
          {kind.icon}
        </span>
        <span
          className={clsx(
            'min-w-0 flex-1 truncate text-[13px] leading-5',
            event.state === 'unread' && event.needs_action
              ? 'font-semibold text-[#22262A]'
              : 'text-[#4A4F55]',
          )}
        >
          {event.subject}
        </span>
        {event.kind === 'plan' && event.plan_state && (
          <span
            className={clsx(
              'shrink-0 rounded-[3px] px-1 py-px font-mono text-[9.5px] uppercase tracking-[0.08em]',
              planPending
                ? 'bg-[#C98A1B]/12 text-[#92610E]'
                : 'bg-[#178A5B]/10 text-[#178A5B]',
            )}
          >
            {event.plan_state.replace('_', ' ')}
          </span>
        )}
        <span className="shrink-0 font-mono text-[10px] text-[#9BA0A6]">
          {timeAgo(event.created_at)}
        </span>
      </button>

      {expanded && (
        <div className="px-3 pb-2.5 pl-[34px]">
          {/* Plan decisions live above the plan body so approving
              never requires scrolling past the full plan text. */}
          {event.kind === 'plan' && planPending && (
            <div className="mb-2 space-y-1.5">
              <input
                type="text"
                value={planComment}
                onChange={(e) => setPlanComment(e.target.value)}
                placeholder="Note to agent (optional)"
                className="w-full border-b border-[#E6E4DD] bg-transparent pb-1 text-[13px] text-[#22262A] placeholder:text-[#B0ADA4] focus:border-[#22262A] focus:outline-none"
              />
              <div className="flex gap-1.5 pt-0.5">
                <button
                  type="button"
                  onClick={() => decidePlan('approved')}
                  disabled={updatePlan.isPending}
                  className="rounded-md bg-[#178A5B] px-2.5 py-1 text-[12px] font-medium text-white hover:bg-[#116D48] disabled:opacity-50"
                >
                  Approve
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('changes_requested')}
                  disabled={updatePlan.isPending}
                  className="rounded-md border border-[#D8D6CE] px-2.5 py-1 text-[12px] font-medium text-[#4A4F55] hover:bg-[#F4F3EE] disabled:opacity-50"
                >
                  Request changes
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('rejected')}
                  disabled={updatePlan.isPending}
                  className="rounded-md px-2 py-1 text-[12px] font-medium text-[#B3372B] hover:bg-[#B3372B]/8 disabled:opacity-50"
                >
                  Reject
                </button>
              </div>
            </div>
          )}

          {text && (
            <div
              className="scrollbar-thin prose prose-command max-h-80 max-w-none overflow-y-auto text-[13px] leading-relaxed text-[#33383D]"
              dangerouslySetInnerHTML={{ __html: renderedBody }}
            />
          )}

          {patch && (
            <div className="mt-2 overflow-hidden rounded-md border border-[#E6E4DD]">
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

          <div className="mt-1.5 flex items-center gap-3">
            <button
              type="button"
              onClick={() => onReply(event)}
              className="text-[12px] font-medium text-[#33608D] hover:underline"
            >
              Reply
            </button>
            {event.thread_id && (
              <a
                href={`/thread/${event.thread_id}`}
                className="text-[12px] text-[#9BA0A6] hover:text-[#4A4F55]"
              >
                Open thread
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
