// ThreadMessage component - a single message within a thread view.

import { useMemo, useState, useCallback, useEffect, lazy, Suspense } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { renderMarkdownToHtml } from '@/lib/markdown.js';
import { Avatar } from '@/components/ui/Avatar.js';
import { PriorityBadge } from '@/components/ui/Badge.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { Message, MessageWithRecipients } from '@/types/api.js';
import type { DiffAnnotation } from '@/types/annotations.js';
import { useAnnotationStore } from '@/stores/annotations.js';
import { useAuthStore } from '@/stores/auth.js';
import { sendMessage } from '@/api/messages.js';
import { exportDiffAnnotations } from '@/lib/feedback-export.js';
import { formatAgentDisplayName, getAgentContext } from '@/lib/utils.js';

// Lazy-load DiffViewer to avoid bundling Shiki grammars in inbox chunk.
const DiffViewer = lazy(
  () => import('@/components/reviews/DiffViewer.js').then(
    (m) => ({ default: m.DiffViewer }),
  ),
);

// Convert message sender to AgentLike format for display formatting.
function getSenderAsAgent(message: Message) {
  return {
    name: message.sender_name,
    project_key: message.sender_project_key,
    git_branch: message.sender_git_branch,
  };
}

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format date for message display.
function formatMessageDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  const isThisYear = date.getFullYear() === now.getFullYear();

  if (isToday) {
    return date.toLocaleTimeString(undefined, {
      hour: 'numeric',
      minute: '2-digit',
    });
  }

  if (isThisYear) {
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  }

  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

// Diff marker used by `substrate send-diff` to embed patches in messages.
const DIFF_MARKER = '<!-- substrate:diff -->';

// Split message body into markdown text and optional diff patch.
function splitBodyAndDiff(body: string): { text: string; patch: string | null } {
  const idx = body.indexOf(DIFF_MARKER);
  if (idx === -1) {
    return { text: body, patch: null };
  }

  return {
    text: body.slice(0, idx).trimEnd(),
    patch: body.slice(idx + DIFF_MARKER.length).trim(),
  };
}

// Message with optional recipients for flexible usage.
type MessageMaybeWithRecipients = Message & {
  recipients?: MessageWithRecipients['recipients'];
};

// Props for ThreadMessage component.
export interface ThreadMessageProps {
  /** The message to display (recipients optional for "To" field). */
  message: MessageMaybeWithRecipients;
  /** Whether this is the first (original) message in the thread. */
  isFirst?: boolean;
  /** Whether this message is currently focused. */
  isFocused?: boolean;
  /** Additional class name. */
  className?: string;
}

export function ThreadMessage({
  message,
  isFirst = false,
  isFocused = false,
  className,
}: ThreadMessageProps) {
  // Split body into markdown text and optional diff patch.
  const { text: bodyText, patch } = useMemo(
    () => splitBodyAndDiff(message.body),
    [message.body],
  );

  // Memoize the rendered markdown to avoid re-parsing on every render.
  const renderedBody = useMemo(
    () => renderMarkdownToHtml(bodyText),
    [bodyText],
  );

  // Track whether the diff section is expanded.
  const [diffExpanded, setDiffExpanded] = useState(false);
  const [reviewMode, setReviewMode] = useState(false);

  // Diff annotation support, scoped to this message's ID.
  const {
    diffAnnotations: allDiffAnnotations,
    addDiffAnnotation,
    updateDiffAnnotation: storeUpdateDiffAnnotation,
    deleteDiffAnnotation,
    setDiffMessageId,
  } = useAnnotationStore();

  // Filter annotations to only this message.
  const messageId = message.id;
  const diffAnnotations = useMemo(
    () => allDiffAnnotations,
    [allDiffAnnotations],
  );

  // Set the diff message ID when entering review mode.
  useEffect(() => {
    if (reviewMode && messageId) {
      setDiffMessageId(messageId);
    }
  }, [reviewMode, messageId, setDiffMessageId]);

  const handleAddDiffAnnotation = useCallback(
    (params: {
      filePath: string;
      type: DiffAnnotation['type'];
      scope: DiffAnnotation['scope'];
      lineStart: number;
      lineEnd: number;
      side: 'old' | 'new';
      text: string;
      suggestedCode?: string | undefined;
    }) => {
      addDiffAnnotation(params);
    },
    [addDiffAnnotation],
  );

  const handleUpdateDiffAnnotation = useCallback(
    (id: string, text: string, suggestedCode?: string | undefined) => {
      storeUpdateDiffAnnotation(id, {
        ...(text !== undefined ? { text } : {}),
        ...(suggestedCode !== undefined ? { suggestedCode } : {}),
      });
    },
    [storeUpdateDiffAnnotation],
  );

  const handleDeleteDiffAnnotation = useCallback(
    (id: string) => {
      deleteDiffAnnotation(id);
    },
    [deleteDiffAnnotation],
  );

  const handleSubmitReview = useCallback(() => {
    if (diffAnnotations.length === 0) return;

    const feedback = exportDiffAnnotations(diffAnnotations);
    const { currentAgent, availableAgents } = useAuthStore.getState();
    const userAgent = availableAgents.find((a) => a.name === 'User');
    const senderId = currentAgent?.id ?? userAgent?.id ?? 0;
    const senderName = message.sender_name || 'User';

    console.log(
      '[Review] Submitting review with',
      diffAnnotations.length, 'annotations to', senderName,
      'from agent', currentAgent?.name, '(id:', currentAgent?.id, ')',
    );

    sendMessage({
      sender_id: senderId,
      recipient_names: [senderName],
      ...(message.thread_id ? { thread_id: message.thread_id } : {}),
      subject: `Re: ${message.subject} [Code Review]`,
      body: feedback,
      priority: 'PRIORITY_NORMAL',
    })
      .then(() => {
        console.log('[Review] Review submitted successfully');
      })
      .catch((err) => {
        console.error('[Review] Submit failed:', err);
      });
  }, [diffAnnotations, message]);

  return (
    <div
      className={cn(
        'rounded-lg border bg-white p-4 transition-colors',
        isFocused ? 'border-[#22262A]/30 ring-2 ring-[#22262A]/10' : 'border-[#E6E4DD]',
        className,
      )}
      role="article"
      aria-label={`Message from ${formatAgentDisplayName(getSenderAsAgent(message))}`}
    >
      {/* Message header. */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <Avatar name={message.sender_name} size="md" />
          <div>
            <div
              className="flex items-center gap-2"
              title={formatAgentDisplayName(getSenderAsAgent(message))}
            >
              <span className="font-medium text-[#22262A]">
                {message.sender_name}
              </span>
              {getAgentContext(getSenderAsAgent(message)) ? (
                <span className="text-xs text-[#9BA0A6]">
                  @{getAgentContext(getSenderAsAgent(message))}
                </span>
              ) : null}
              {message.priority !== 'normal' ? (
                <PriorityBadge priority={message.priority} size="sm" />
              ) : null}
            </div>
            <span className="text-sm text-[#6B7280]">
              {formatMessageDate(message.created_at)}
            </span>
            {/* Recipient (To) field. */}
            {message.recipient_names && message.recipient_names.length > 0 ? (
              <div className="text-sm text-[#6B7280]">
                <span className="text-[#9BA0A6]">To: </span>
                {message.recipient_names.join(', ')}
              </div>
            ) : message.recipients && message.recipients.length > 0 ? (
              <div className="text-sm text-[#6B7280]">
                <span className="text-[#9BA0A6]">To: </span>
                {message.recipients.map((r) => r.agent_name).join(', ')}
              </div>
            ) : null}
          </div>
        </div>

        {/* Show subject only for first message. */}
        {isFirst ? null : (
          <span className="text-xs text-[#9BA0A6]">Reply</span>
        )}
      </div>

      {/* Subject (only for first message). */}
      {isFirst ? (
        <h2 className="mt-3 text-lg font-semibold text-[#22262A]">
          {message.subject}
        </h2>
      ) : null}

      {/* Message body with rendered markdown. */}
      <div
        className="prose prose-sm mt-3 max-w-none text-[#22262A]"
        dangerouslySetInnerHTML={{ __html: renderedBody }}
      />

      {/* Embedded diff section (from substrate send-diff). */}
      {patch ? (
        <div className="mt-4 border-t border-[#F1EFE9] pt-3">
          <button
            type="button"
            onClick={() => setDiffExpanded(!diffExpanded)}
            className={cn(
              'rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
              diffExpanded
                ? 'border-[#0E7490]/30 bg-[#0E7490]/10 text-[#0E7490]'
                : 'border-[#E6E4DD] text-[#6B7280] hover:bg-[#F4F3EE]',
            )}
          >
            {diffExpanded ? 'Hide diff' : 'Show diff'}
          </button>
          {diffExpanded && (
            <button
              type="button"
              onClick={() => setReviewMode(!reviewMode)}
              className={cn(
                'ml-2 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
                reviewMode
                  ? 'border-[#178A5B]/30 bg-[#178A5B]/10 text-[#178A5B]'
                  : 'border-[#E6E4DD] text-[#6B7280] hover:bg-[#F4F3EE]',
              )}
            >
              {reviewMode ? 'Exit Review' : 'Review'}
            </button>
          )}

          {diffExpanded ? (
            <div className="mt-3">
              <Suspense
                fallback={
                  <div className="flex justify-center py-8">
                    <Spinner
                      size="md"
                      variant="primary"
                      label="Loading diff viewer..."
                    />
                  </div>
                }
              >
                <DiffViewer
                  patch={patch}
                  reviewMode={reviewMode}
                  annotations={diffAnnotations}
                  onAddAnnotation={handleAddDiffAnnotation}
                  onUpdateAnnotation={handleUpdateDiffAnnotation}
                  onDeleteAnnotation={handleDeleteDiffAnnotation}
                  onSubmitReview={handleSubmitReview}
                />
              </Suspense>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

// Compact variant for collapsed messages.
export interface CompactThreadMessageProps {
  /** The message to display. */
  message: Message;
  /** Handler for clicking to expand. */
  onClick?: () => void;
  /** Additional class name. */
  className?: string;
}

export function CompactThreadMessage({
  message,
  onClick,
  className,
}: CompactThreadMessageProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-3 rounded-lg border border-[#E6E4DD] bg-[#F7F6F3] p-3 text-left',
        'hover:bg-[#F1EFE9] transition-colors',
        'focus:outline-none focus:ring-2 focus:ring-[#22262A]/30',
        className,
      )}
    >
      <Avatar name={message.sender_name} size="sm" />
      <div className="min-w-0 flex-1" title={formatAgentDisplayName(getSenderAsAgent(message))}>
        <span className="truncate text-sm font-medium text-[#22262A]">
          {message.sender_name}
        </span>
        {getAgentContext(getSenderAsAgent(message)) ? (
          <span className="text-xs text-[#9BA0A6]">
            @{getAgentContext(getSenderAsAgent(message))}
          </span>
        ) : null}
        <span className="ml-2 truncate text-sm text-[#6B7280]">
          {message.body.slice(0, 80)}
          {message.body.length > 80 ? '...' : ''}
        </span>
      </div>
      <span className="flex-shrink-0 text-xs text-[#9BA0A6]">
        {formatMessageDate(message.created_at)}
      </span>
    </button>
  );
}

// Deadline banner for messages with deadlines.
export interface DeadlineBannerProps {
  /** The deadline date string. */
  deadline: string;
  /** Whether the deadline has passed. */
  isPast?: boolean;
  /** Handler for acknowledging. */
  onAcknowledge?: () => void;
  /** Whether the ack action is loading. */
  isLoading?: boolean;
  /** Additional class name. */
  className?: string;
}

export function DeadlineBanner({
  deadline,
  isPast = false,
  onAcknowledge,
  isLoading = false,
  className,
}: DeadlineBannerProps) {
  const date = new Date(deadline);
  const formattedDate = date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });

  return (
    <div
      className={cn(
        'flex items-center justify-between rounded-lg px-4 py-3',
        isPast ? 'bg-[#B3372B]/10' : 'bg-[#C98A1B]/12',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <svg
          className={cn('h-5 w-5', isPast ? 'text-[#B3372B]' : 'text-[#92610E]')}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span
          className={cn(
            'text-sm font-medium',
            isPast ? 'text-[#B3372B]' : 'text-[#92610E]',
          )}
        >
          {isPast ? 'Deadline passed: ' : 'Deadline: '}
          {formattedDate}
        </span>
      </div>

      {onAcknowledge ? (
        <button
          type="button"
          onClick={onAcknowledge}
          disabled={isLoading}
          className={cn(
            'rounded px-3 py-1 text-sm font-medium transition-colors',
            'focus:outline-none focus:ring-2 focus:ring-offset-2',
            isPast
              ? 'bg-[#B3372B]/10 text-[#B3372B] hover:bg-[#B3372B]/20 focus:ring-[#B3372B]/30'
              : 'bg-[#C98A1B]/12 text-[#92610E] hover:bg-[#C98A1B]/20 focus:ring-[#92610E]/30',
            isLoading ? 'cursor-not-allowed opacity-50' : '',
          )}
        >
          {isLoading ? 'Acknowledging...' : 'Acknowledge'}
        </button>
      ) : null}
    </div>
  );
}
