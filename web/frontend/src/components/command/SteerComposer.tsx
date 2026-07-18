// SteerComposer is the always-visible one-line input pinned to the
// bottom of an agent lane. It sends steering messages straight to the
// agent, optionally as a reply to a thread the user picked via an
// event's Reply button.

import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { clsx } from 'clsx';
import { useSendMessage } from '@/hooks/useMessages.js';
import { useReplyToThread } from '@/hooks/useThreads.js';
import { commandKeys } from '@/hooks/useCommandFeed.js';
import { useAuthStore } from '@/stores/auth.js';
import { useUIStore } from '@/stores/ui.js';

// Reply target context set by an event card.
export interface ReplyTarget {
  threadId: string;
  subject: string;
}

export interface SteerComposerProps {
  agentName: string;
  // Active reply target; null means a fresh steering message.
  replyTarget: ReplyTarget | null;
  onClearReply: () => void;
}

export function SteerComposer({
  agentName,
  replyTarget,
  onClearReply,
}: SteerComposerProps) {
  const [text, setText] = useState('');
  const [urgent, setUrgent] = useState(false);
  const inputRef = useRef<HTMLTextAreaElement>(null);

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
    const body = text.trim();
    if (!body || isSending) {
      return;
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
        await sendMessage.mutateAsync({
          sender_id: currentAgent?.id ?? userAgent?.id ?? 0,
          recipient_names: [agentName],
          subject: `Steer: ${body.slice(0, 60)}`,
          body,
          priority: urgent ? 'PRIORITY_URGENT' : 'PRIORITY_NORMAL',
        });
      }

      setText('');
      setUrgent(false);
      onClearReply();
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
    <div className="border-t border-slate-800 bg-slate-900/90 p-2">
      {replyTarget && (
        <div className="mb-1.5 flex items-center gap-2 rounded bg-slate-800/80 px-2 py-1 text-[11px] text-slate-400">
          <span className="truncate">
            Replying to: {replyTarget.subject}
          </span>
          <button
            type="button"
            onClick={onClearReply}
            className="ml-auto shrink-0 text-slate-500 hover:text-slate-300"
            aria-label="Cancel reply"
          >
            ✕
          </button>
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
          className="max-h-28 min-h-[34px] flex-1 resize-none rounded-md border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-[13px] text-slate-200 placeholder:text-slate-600 focus:border-sky-600 focus:outline-none"
        />
        <button
          type="button"
          onClick={() => setUrgent((v) => !v)}
          title="Mark urgent"
          className={clsx(
            'rounded-md border px-2 py-1.5 text-[11px] font-semibold',
            urgent
              ? 'border-red-500 bg-red-500/20 text-red-300'
              : 'border-slate-700 text-slate-500 hover:text-slate-300',
          )}
        >
          !
        </button>
        <button
          type="button"
          onClick={() => void submit()}
          disabled={!text.trim() || isSending}
          className="rounded-md bg-sky-600 px-3 py-1.5 text-[13px] font-medium text-white hover:bg-sky-500 disabled:opacity-40"
        >
          {isSending ? '…' : 'Send'}
        </button>
      </div>
    </div>
  );
}
