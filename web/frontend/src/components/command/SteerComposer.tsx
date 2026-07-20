// SteerComposer is the one-line input at the foot of an agent card.
// It sends steering messages straight to the agent, optionally as a
// reply to a thread picked via an event's Reply button.

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
          onClick={() => setUrgent((v) => !v)}
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
          disabled={!text.trim() || isSending}
          className="rounded-md bg-[var(--c-ink)] px-2.5 py-1 text-[12px] font-medium text-white hover:bg-[var(--c-inkhover)] disabled:opacity-30"
        >
          {isSending ? '…' : 'Send'}
        </button>
      </div>
    </div>
  );
}
