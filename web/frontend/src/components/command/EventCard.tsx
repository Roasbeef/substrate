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

// docPathRe finds repo-relative file references like docs/design.md
// so they can become implicit attachments.
const DOC_PATH_RE =
  /(?:^|[\s(`'"])((?:[\w.-]+\/)+[\w.-]+\.(?:md|go|ts|tsx|js|py|rs|txt|json|ya?ml|sql|sh|proto))/g;

// extractDocPaths pulls up to four unique file references from a body.
function extractDocPaths(body: string): string[] {
  const out: string[] = [];
  for (const m of body.matchAll(DOC_PATH_RE)) {
    const path = m[1] as string;
    if (!out.includes(path)) {
      out.push(path);
    }
    if (out.length >= 4) {
      break;
    }
  }
  return out;
}

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
  // Called when a referenced document chip is clicked.
  onOpenDoc?: ((path: string) => void) | undefined;
  // Called when the message-focus affordance is clicked.
  onFocusMessage?: ((event: CommandEvent) => void) | undefined;
  // Start expanded regardless of classification (message focus view).
  defaultExpanded?: boolean | undefined;
}

export function EventCard({
  event, onReply, onOpenDoc, onFocusMessage, defaultExpanded,
}: EventCardProps) {
  // Actionable events start expanded so the canvas shows work items
  // without a click; informational ones start collapsed.
  const [expanded, setExpanded] = useState(
    defaultExpanded ?? event.needs_action,
  );
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
  const docPaths = useMemo(
    () => (onOpenDoc ? extractDocPaths(event.body) : []),
    [event.body, onOpenDoc],
  );
  const outbound = event.direction === 'out';

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
        className="flex w-full items-baseline gap-2 px-3 py-[7px] text-left hover:bg-[var(--c-hover)]"
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
              ? 'font-semibold text-[var(--c-ink)]'
              : outbound
                ? 'text-[var(--c-faint2)] italic'
                : 'text-[var(--c-text2)]',
          )}
        >
          {event.subject}
        </span>
        {event.kind === 'plan' && event.plan_state && (
          <span
            className={clsx(
              'shrink-0 rounded-[3px] px-1 py-px font-mono text-[9.5px] uppercase tracking-[0.08em]',
              planPending
                ? 'bg-[#C98A1B]/12 text-[var(--c-amber)]'
                : 'bg-[#178A5B]/10 text-[var(--c-green)]',
            )}
          >
            {event.plan_state.replace('_', ' ')}
          </span>
        )}
        <span className="shrink-0 font-mono text-[10px] text-[var(--c-faint)]">
          {timeAgo(event.created_at)}
        </span>
        {onFocusMessage && (
          <span
            role="button"
            tabIndex={0}
            title="Focus this message"
            onClick={(e) => {
              e.stopPropagation();
              onFocusMessage(event);
            }}
            className="shrink-0 rounded p-0.5 text-[var(--c-ghost)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
          >
            <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24"
              stroke="currentColor" strokeWidth={1.8}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          </span>
        )}
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
                className="w-full border-b border-[var(--c-hair)] bg-transparent pb-1 text-[13px] text-[var(--c-ink)] placeholder:text-[var(--c-dim)] focus:border-[var(--c-ink)] focus:outline-none"
              />
              <div className="flex gap-1.5 pt-0.5">
                <button
                  type="button"
                  onClick={() => decidePlan('approved')}
                  disabled={updatePlan.isPending}
                  className="rounded-md bg-[var(--c-green)] px-2.5 py-1 text-[12px] font-medium text-white hover:bg-[var(--c-greenhover)] disabled:opacity-50"
                >
                  Approve
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('changes_requested')}
                  disabled={updatePlan.isPending}
                  className="rounded-md border border-[var(--c-ghost2)] px-2.5 py-1 text-[12px] font-medium text-[var(--c-text2)] hover:bg-[var(--c-hover)] disabled:opacity-50"
                >
                  Request changes
                </button>
                <button
                  type="button"
                  onClick={() => decidePlan('rejected')}
                  disabled={updatePlan.isPending}
                  className="rounded-md px-2 py-1 text-[12px] font-medium text-[var(--c-rust)] hover:bg-[#B3372B]/8 disabled:opacity-50"
                >
                  Reject
                </button>
              </div>
            </div>
          )}

          {text && (
            <div
              className="scrollbar-thin prose prose-command max-h-80 max-w-none overflow-y-auto text-[13px] leading-relaxed text-[var(--c-ink2)]"
              dangerouslySetInnerHTML={{ __html: renderedBody }}
            />
          )}

          {docPaths.length > 0 && (
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {docPaths.map((path) => (
                <button
                  key={path}
                  type="button"
                  onClick={() => onOpenDoc?.(path)}
                  className="flex items-center gap-1 rounded border border-[var(--c-hair)] bg-[var(--c-hover)] px-1.5 py-0.5 font-mono text-[10.5px] text-[var(--c-steel)] hover:border-[var(--c-steel)]"
                >
                  <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24"
                    stroke="currentColor" strokeWidth={1.8}>
                    <path strokeLinecap="round" strokeLinejoin="round"
                      d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  {path}
                </button>
              ))}
            </div>
          )}

          {patch && (
            <div className="mt-2 overflow-hidden rounded-md border border-[var(--c-hair)]">
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
              className="text-[12px] font-medium text-[var(--c-steel)] hover:underline"
            >
              Reply
            </button>
            {event.thread_id && (
              <a
                href={`/thread/${event.thread_id}`}
                className="text-[12px] text-[var(--c-faint)] hover:text-[var(--c-text2)]"
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
