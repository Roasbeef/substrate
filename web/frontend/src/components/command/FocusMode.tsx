// FocusMode snaps one agent near-fullscreen: the conversation timeline
// and composer on the left, a right rail carrying supporting context —
// the live digest, agent facts, and the document viewer when a
// referenced file is opened. Esc or the close button drops back to the
// canvas.

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { clsx } from 'clsx';
import type { CommandEvent, CommandLane } from '@/api/command.js';
import { getAgentDoc } from '@/api/command.js';
import type { AgentSummary } from '@/types/api.js';
import { renderMarkdownToHtml } from '@/lib/markdown.js';
import { useCanvasStore } from '@/stores/canvas.js';
import { EventCard } from './EventCard.js';
import { SteerComposer } from './SteerComposer.js';
import type { ReplyTarget } from './SteerComposer.js';
import { useImageDrop } from './useImageDrop.js';
import { HeartbeatTrace } from './HeartbeatTrace.js';
import { agentTint, timeAgo } from './kinds.js';

export interface FocusModeProps {
  lane: CommandLane;
  summary?: AgentSummary | undefined;
}

export function FocusMode({ lane, summary }: FocusModeProps) {
  const { agent } = lane;
  const setFocusedCard = useCanvasStore((s) => s.setFocusedCard);
  const focusedMessage = useCanvasStore((s) => s.focusedMessage);
  const setFocusedMessage = useCanvasStore((s) => s.setFocusedMessage);
  const openDoc = useCanvasStore((s) => s.openDoc);
  const setOpenDoc = useCanvasStore((s) => s.setOpenDoc);
  const [replyTarget, setReplyTarget] = useState<ReplyTarget | null>(
    null,
  );

  // Focus mode accepts image drops on the whole timeline column, same
  // as the canvas cards.
  const {
    attachments,
    dropActive,
    dropHandlers,
    removeAttachment,
    clearAttachments,
  } = useImageDrop();

  // Esc leaves focus mode (or closes the doc first if one is open).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') {
        return;
      }
      const st = useCanvasStore.getState();
      if (st.openDoc) {
        setOpenDoc(null);
      } else if (st.focusedMessage !== null) {
        setFocusedMessage(null);
      } else {
        setFocusedCard(null);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [setFocusedCard, setFocusedMessage, setOpenDoc]);

  const doc = useQuery({
    queryKey: ['command', 'doc', openDoc?.agentId, openDoc?.path],
    queryFn: () => getAgentDoc(openDoc!.agentId, openDoc!.path),
    enabled: openDoc !== null,
    staleTime: 30_000,
  });

  const docHtml = useMemo(() => {
    if (!doc.data) {
      return '';
    }
    return doc.data.path.endsWith('.md')
      ? renderMarkdownToHtml(doc.data.content)
      : '';
  }, [doc.data]);

  const handleReply = (event: CommandEvent) =>
    setReplyTarget({
      threadId: event.thread_id,
      subject: event.subject,
    });

  const context =
    agent.project_key || agent.working_dir || agent.purpose;

  return (
    <div
      className="absolute inset-0 z-40 flex bg-[var(--c-paper)]/80 p-4 backdrop-blur-[2px]"
      // The overlay lives inside the canvas container; keep pointer
      // and wheel activity from reaching the pan/zoom handlers.
      onPointerDown={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      {/* Main column: identity header, timeline, composer. Accepts
          image drops anywhere on the column. */}
      <div
        {...dropHandlers}
        className={clsx(
          'flex min-w-0 flex-[3] flex-col overflow-hidden rounded-xl border bg-[var(--c-card)] shadow-[0_16px_48px_rgba(28,32,36,0.18)]',
          dropActive
            ? 'border-[var(--c-steel)] ring-2 ring-[var(--c-steel)]/30'
            : 'border-[var(--c-hair)]',
        )}
      >
        <header className="flex items-center gap-3 border-b border-[var(--c-hair2)] px-4 py-3">
          <span
            className="flex h-9 w-9 items-center justify-center rounded-lg font-mono text-[15px] font-bold text-white"
            style={{ backgroundColor: agentTint(agent.name) }}
          >
            {agent.name.slice(0, 1)}
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-2.5">
              <h2 className="text-[16px] font-semibold text-[var(--c-ink)]">
                {agent.name}
              </h2>
              <HeartbeatTrace status={agent.status} />
              <span className="font-mono text-[10.5px] text-[var(--c-faint)]">
                {timeAgo(agent.last_active_at)}
              </span>
            </div>
            <p className="truncate font-mono text-[11px] text-[var(--c-faint2)]">
              {context || 'no context recorded'}
              {agent.git_branch && ` · ${agent.git_branch}`}
            </p>
          </div>
          <button
            type="button"
            onClick={() => setFocusedCard(null)}
            title="Back to canvas (Esc)"
            className="ml-auto rounded-md p-1.5 text-[var(--c-faint)] hover:bg-[var(--c-hover)] hover:text-[var(--c-ink)]"
          >
            <svg className="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24"
              stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M9 9L4 4m0 0v4m0-4h4m7 5l5-5m0 0v4m0-4h-4M9 15l-5 5m0 0v-4m0 4h4m7-5l5 5m0 0v-4m0 4h-4" />
            </svg>
          </button>
        </header>

        {focusedMessage !== null && (
          <button
            type="button"
            onClick={() => setFocusedMessage(null)}
            className="border-b border-[var(--c-hair2)] bg-[var(--c-hover)] px-4 py-1.5 text-left font-mono text-[10.5px] uppercase tracking-[0.1em] text-[var(--c-mut)] hover:text-[var(--c-ink)]"
          >
            ← single message · show full timeline
          </button>
        )}
        <div className="scrollbar-thin min-h-0 flex-1 divide-y divide-[var(--c-fill)] overflow-y-auto">
          {lane.events.length === 0 && (
            <p className="px-4 py-6 text-center text-[12.5px] text-[var(--c-dim)]">
              No traffic yet.
            </p>
          )}
          {(focusedMessage === null
            ? lane.events
            : lane.events.filter(
                (ev) => ev.message_id === focusedMessage,
              )
          ).map((event) => (
            <EventCard
              key={`f-${event.direction}-${event.message_id}-${String(focusedMessage !== null)}`}
              event={event}
              onReply={handleReply}
              defaultExpanded={focusedMessage !== null}
              onOpenDoc={(path) =>
                setOpenDoc({ agentId: agent.id, path })
              }
            />
          ))}
        </div>

        <SteerComposer
          agentName={agent.name}
          replyTarget={replyTarget}
          onClearReply={() => setReplyTarget(null)}
          attachments={attachments}
          onRemoveAttachment={removeAttachment}
          onClearAttachments={clearAttachments}
        />
      </div>

      {/* Right rail: doc viewer or digest + facts. */}
      <div className="ml-4 flex min-w-0 flex-[2] flex-col overflow-hidden rounded-xl border border-[var(--c-hair)] bg-[var(--c-card)] shadow-[0_16px_48px_rgba(28,32,36,0.14)]">
        {openDoc ? (
          <>
            <header className="flex items-center gap-2 border-b border-[var(--c-hair2)] px-4 py-2.5">
              <span className="truncate font-mono text-[11.5px] text-[var(--c-ink2)]">
                {openDoc.path}
              </span>
              {doc.data?.truncated && (
                <span className="shrink-0 rounded bg-[#C98A1B]/12 px-1 font-mono text-[9px] uppercase text-[var(--c-amber)]">
                  truncated
                </span>
              )}
              <button
                type="button"
                onClick={() => setOpenDoc(null)}
                className="ml-auto rounded p-1 text-[var(--c-faint)] hover:bg-[var(--c-hover)] hover:text-[var(--c-ink)]"
                aria-label="Close document"
              >
                ✕
              </button>
            </header>
            <div className="scrollbar-thin min-h-0 flex-1 overflow-y-auto px-4 py-3">
              {doc.isLoading && (
                <p className="text-[12px] text-[var(--c-faint)]">
                  Loading…
                </p>
              )}
              {doc.isError && (
                <div className="space-y-2">
                  <p className="text-[12px] text-[var(--c-rust)]">
                    Could not read this file from the agent's working
                    directory.
                  </p>
                  <dl className="space-y-1 rounded-md border border-[var(--c-hair)] bg-[var(--c-hover)] p-2 font-mono text-[10.5px] text-[var(--c-mut)]">
                    <div>path: {openDoc.path}</div>
                    <div>
                      workdir:{' '}
                      {agent.working_dir || '(not recorded)'}
                    </div>
                    <div>
                      project: {agent.project_key || '(none)'}
                    </div>
                  </dl>
                  <p className="text-[11.5px] leading-snug text-[var(--c-dim)]">
                    Substrate resolves files against the agent's
                    recorded working directory (worktrees included).
                    If it shows "(not recorded)", the agent has never
                    reported one — it backfills from the agent's
                    session identity when available, and future CLI
                    heartbeats can report it directly.
                  </p>
                </div>
              )}
              {doc.data &&
                (docHtml ? (
                  <div
                    className="prose prose-command max-w-none text-[13px] leading-relaxed text-[var(--c-ink2)]"
                    dangerouslySetInnerHTML={{ __html: docHtml }}
                  />
                ) : (
                  <pre className="whitespace-pre-wrap font-mono text-[11.5px] leading-5 text-[var(--c-ink2)]">
                    {doc.data.content}
                  </pre>
                ))}
            </div>
          </>
        ) : (
          <div className="scrollbar-thin min-h-0 flex-1 overflow-y-auto px-4 py-3.5">
            <p className="font-mono text-[9.5px] font-semibold uppercase tracking-[0.12em] text-[var(--c-mut)]">
              Now
            </p>
            <p
              className={clsx(
                'mt-1 text-[13px] leading-snug',
                summary?.summary
                  ? 'text-[var(--c-ink2)]'
                  : 'italic text-[var(--c-faint)]',
              )}
            >
              {summary?.summary ??
                'No live summary — the agent has no project key or ' +
                  'transcript for the summarizer to read.'}
            </p>
            {summary?.delta && (
              <p className="mt-1 text-[12px] text-[var(--c-green)]">
                Δ {summary.delta}
              </p>
            )}

            <p className="mt-4 font-mono text-[9.5px] font-semibold uppercase tracking-[0.12em] text-[var(--c-mut)]">
              Facts
            </p>
            <dl className="mt-1 space-y-1 text-[12px]">
              {[
                ['project', agent.project_key],
                ['workdir', agent.working_dir],
                ['branch', agent.git_branch],
                ['purpose', agent.purpose],
                ['session', agent.session_id ?? ''],
              ]
                .filter(([, v]) => v)
                .map(([k, v]) => (
                  <div key={k} className="flex gap-2">
                    <dt className="w-16 shrink-0 font-mono text-[10.5px] uppercase text-[var(--c-faint)]">
                      {k}
                    </dt>
                    <dd className="min-w-0 break-all font-mono text-[11px] text-[var(--c-text2)]">
                      {v}
                    </dd>
                  </div>
                ))}
            </dl>

            <p className="mt-4 text-[11.5px] leading-snug text-[var(--c-dim)]">
              Click a file reference in any message to open it here.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
