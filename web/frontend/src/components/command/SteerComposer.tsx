// SteerComposer is the one-line input at the foot of an agent card.
// It sends steering messages straight to the agent, optionally as a
// reply to a thread picked via an event's Reply button.

import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { clsx } from 'clsx';
import { useSendMessage } from '@/hooks/useMessages.js';
import { useReplyToThread } from '@/hooks/useThreads.js';
import { commandKeys } from '@/hooks/useCommandFeed.js';
import { useAuthStore } from '@/stores/auth.js';
import { useCanvasStore } from '@/stores/canvas.js';
import { useUIStore } from '@/stores/ui.js';

// Reply target context set by an event card.
export interface ReplyTarget {
  threadId: string;
  subject: string;
}

// A staged image attachment waiting to be sent with the next message.
export interface PendingAttachment {
  name: string;
  url: string;
  markdown: string;
}

export interface SteerComposerProps {
  agentName: string;
  // Active reply target; null means a fresh steering message.
  replyTarget: ReplyTarget | null;
  onClearReply: () => void;
  // Images staged by drag-drop, sent along with the next message.
  attachments?: PendingAttachment[] | undefined;
  onRemoveAttachment?: ((index: number) => void) | undefined;
  onClearAttachments?: (() => void) | undefined;
}

export function SteerComposer({
  agentName,
  replyTarget,
  onClearReply,
  attachments = [],
  onRemoveAttachment,
  onClearAttachments,
}: SteerComposerProps) {
  // The draft lives in the canvas store (persisted to localStorage),
  // so text survives card remounts (panning, focus mode, feed
  // refreshes) and full page reloads — and the canvas card and focus
  // mode composers stay in sync for the same agent.
  const text = useCanvasStore(
    (s) => s.drafts[agentName]?.text ?? '',
  );
  const urgent = useCanvasStore(
    (s) => s.drafts[agentName]?.urgent ?? false,
  );
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // updateDraft writes the draft through to the store, dropping the
  // entry entirely once it is back to the empty state.
  const updateDraft = (nextText: string, nextUrgent: boolean) => {
    const { setDraft, clearDraft } = useCanvasStore.getState();
    if (nextText === '' && !nextUrgent) {
      clearDraft(agentName);
    } else {
      setDraft(agentName, { text: nextText, urgent: nextUrgent });
    }
  };
  const setText = (v: string) => updateDraft(v, urgent);
  const setUrgent = (v: boolean) => updateDraft(text, v);

  const sendMessage = useSendMessage();
  const replyToThread = useReplyToThread();
  const addToast = useUIStore((s) => s.addToast);
  const queryClient = useQueryClient();

  // Focus the input when a reply target is picked.
  useEffect(() => {
    if (replyTarget) {
      inputRef.current?.focus();
    }
  }, [replyTarget]);

  const isSending = sendMessage.isPending || replyToThread.isPending;

  // Send the steering message, either as a thread reply or a new
  // direct message to the agent.
  const submit = async () => {
    let body = text.trim();
    if ((!body && attachments.length === 0) || isSending) {
      return;
    }

    // Staged images ride along as markdown references.
    if (attachments.length > 0) {
      const refs = attachments.map((a) => a.markdown).join('\n');
      body = body ? `${body}\n\n${refs}` : refs;
    }

    try {
      if (replyTarget) {
        await replyToThread.mutateAsync({
          id: replyTarget.threadId,
          body,
        });
      } else {
        const { currentAgent, availableAgents } =
          useAuthStore.getState();
        const userAgent = availableAgents.find(
          (a) => a.name === 'User',
        );
        const subject = text.trim()
          ? `Steer: ${text.trim().slice(0, 60)}`
          : `Image: ${attachments.map((a) => a.name).join(', ')}`;
        await sendMessage.mutateAsync({
          sender_id: currentAgent?.id ?? userAgent?.id ?? 0,
          recipient_names: [agentName],
          subject,
          body,
          priority: urgent ? 'PRIORITY_URGENT' : 'PRIORITY_NORMAL',
        });
      }

      updateDraft('', false);
      onClearReply();
      onClearAttachments?.();
      void queryClient.invalidateQueries({
        queryKey: commandKeys.feed(),
      });
    } catch (err) {
      addToast({
        variant: 'error',
        title: 'Send failed',
        message:
          err instanceof Error ? err.message : 'Unable to send',
      });
    }
  };

  return (
    <div className="border-t border-[var(--c-hair2)] px-3 py-2">
      {replyTarget && (
        <div className="mb-1 flex items-center gap-2 text-[11px] text-[var(--c-mut)]">
          <span className="truncate">
            Reply to: {replyTarget.subject}
          </span>
          <button
            type="button"
            onClick={onClearReply}
            className="ml-auto shrink-0 text-[var(--c-faint)] hover:text-[var(--c-text2)]"
            aria-label="Cancel reply"
          >
            ✕
          </button>
        </div>
      )}

      {attachments.length > 0 && (
        <div className="mb-1.5 flex flex-wrap gap-1.5">
          {attachments.map((a, i) => (
            <span
              key={`${a.url}-${i}`}
              className="flex items-center gap-1.5 rounded-md border border-[var(--c-hair)] bg-[var(--c-hover)] py-0.5 pl-0.5 pr-1.5"
            >
              <img
                src={a.url}
                alt={a.name}
                className="h-6 w-6 rounded object-cover"
              />
              <span className="max-w-[120px] truncate font-mono text-[10.5px] text-[var(--c-text2)]">
                {a.name}
              </span>
              <button
                type="button"
                onClick={() => onRemoveAttachment?.(i)}
                className="text-[var(--c-faint)] hover:text-[var(--c-rust)]"
                aria-label={`Remove ${a.name}`}
              >
                ✕
              </button>
            </span>
          ))}
        </div>
      )}

      <div className="flex items-end gap-1.5">
        <textarea
          ref={inputRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              void submit();
            }
          }}
          rows={1}
          placeholder={
            replyTarget
              ? 'Type your reply…'
              : `Steer ${agentName}…`
          }
          className="max-h-24 min-h-[30px] flex-1 resize-none border-b border-transparent bg-transparent py-1 text-[13px] text-[var(--c-ink)] placeholder:text-[var(--c-dim)] focus:border-[var(--c-ink)] focus:outline-none"
        />
        <button
          type="button"
          onClick={() => setUrgent(!urgent)}
          title="Mark urgent"
          className={clsx(
            'rounded-md px-1.5 py-1 font-mono text-[11px] font-bold',
            urgent
              ? 'bg-[#B3372B]/10 text-[var(--c-rust)]'
              : 'text-[var(--c-ghost)] hover:text-[var(--c-mut)]',
          )}
        >
          !
        </button>
        <button
          type="button"
          onClick={() => void submit()}
          disabled={
            (!text.trim() && attachments.length === 0) || isSending
          }
          className="rounded-md bg-[var(--c-ink)] px-2.5 py-1 text-[12px] font-medium text-white hover:bg-[var(--c-inkhover)] disabled:opacity-30"
        >
          {isSending ? '…' : 'Send'}
        </button>
      </div>
    </div>
  );
}
