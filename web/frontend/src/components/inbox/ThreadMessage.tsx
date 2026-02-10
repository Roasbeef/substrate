// ThreadMessage component - a single message within a thread view.

import { useMemo, useState, lazy, Suspense } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { Avatar } from '@/components/ui/Avatar.js';
import { PriorityBadge } from '@/components/ui/Badge.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { Message, MessageWithRecipients } from '@/types/api.js';
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

// Configure marked options for safe rendering.
marked.setOptions({
  gfm: true,
  breaks: true,
});

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

// Render markdown text safely using marked and DOMPurify.
function renderMarkdownToHtml(text: string): string {
  // Parse markdown to HTML.
  const rawHtml = marked.parse(text, { async: false }) as string;
  // Sanitize HTML to prevent XSS.
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_TAGS: [
      'p', 'br', 'strong', 'em', 'code', 'pre', 'ul', 'ol', 'li',
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'a', 'blockquote', 'hr',
    ],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
  });
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

  return (
    <div
      className={cn(
        'rounded-lg border bg-white p-4 transition-colors',
        isFocused ? 'border-blue-300 ring-2 ring-blue-100' : 'border-gray-200',
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
              <span className="font-medium text-gray-900">
                {message.sender_name}
              </span>
              {getAgentContext(getSenderAsAgent(message)) ? (
                <span className="text-xs text-gray-400">
                  @{getAgentContext(getSenderAsAgent(message))}
                </span>
              ) : null}
              {message.priority !== 'normal' ? (
                <PriorityBadge priority={message.priority} size="sm" />
              ) : null}
            </div>
            <span className="text-sm text-gray-500">
              {formatMessageDate(message.created_at)}
            </span>
            {/* Recipient (To) field. */}
            {message.recipient_names && message.recipient_names.length > 0 ? (
              <div className="text-sm text-gray-500">
                <span className="text-gray-400">To: </span>
                {message.recipient_names.join(', ')}
              </div>
            ) : message.recipients && message.recipients.length > 0 ? (
              <div className="text-sm text-gray-500">
                <span className="text-gray-400">To: </span>
                {message.recipients.map((r) => r.agent_name).join(', ')}
              </div>
            ) : null}
          </div>
        </div>

        {/* Show subject only for first message. */}
        {isFirst ? null : (
          <span className="text-xs text-gray-400">Reply</span>
        )}
      </div>

      {/* Subject (only for first message). */}
      {isFirst ? (
        <h2 className="mt-3 text-lg font-semibold text-gray-900">
          {message.subject}
        </h2>
      ) : null}

      {/* Message body with rendered markdown. */}
      <div
        className="prose prose-sm mt-3 max-w-none text-gray-700"
        dangerouslySetInnerHTML={{ __html: renderedBody }}
      />

      {/* Embedded diff section (from substrate send-diff). */}
      {patch ? (
        <div className="mt-4 border-t border-gray-100 pt-3">
          <button
            type="button"
            onClick={() => setDiffExpanded(!diffExpanded)}
            className={cn(
              'rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
              diffExpanded
                ? 'border-blue-200 bg-blue-50 text-blue-700'
                : 'border-gray-200 text-gray-600 hover:bg-gray-50',
            )}
          >
            {diffExpanded ? 'Hide diff' : 'Show diff'}
          </button>

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
                <DiffViewer patch={patch} />
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
        'flex w-full items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 p-3 text-left',
        'hover:bg-gray-100 transition-colors',
        'focus:outline-none focus:ring-2 focus:ring-blue-500',
        className,
      )}
    >
      <Avatar name={message.sender_name} size="sm" />
      <div className="min-w-0 flex-1" title={formatAgentDisplayName(getSenderAsAgent(message))}>
        <span className="truncate text-sm font-medium text-gray-900">
          {message.sender_name}
        </span>
        {getAgentContext(getSenderAsAgent(message)) ? (
          <span className="text-xs text-gray-400">
            @{getAgentContext(getSenderAsAgent(message))}
          </span>
        ) : null}
        <span className="ml-2 truncate text-sm text-gray-500">
          {message.body.slice(0, 80)}
          {message.body.length > 80 ? '...' : ''}
        </span>
      </div>
      <span className="flex-shrink-0 text-xs text-gray-400">
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
        isPast ? 'bg-red-50' : 'bg-yellow-50',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <svg
          className={cn('h-5 w-5', isPast ? 'text-red-500' : 'text-yellow-500')}
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
            isPast ? 'text-red-700' : 'text-yellow-700',
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
              ? 'bg-red-100 text-red-700 hover:bg-red-200 focus:ring-red-500'
              : 'bg-yellow-100 text-yellow-700 hover:bg-yellow-200 focus:ring-yellow-500',
            isLoading ? 'cursor-not-allowed opacity-50' : '',
          )}
        >
          {isLoading ? 'Acknowledging...' : 'Acknowledge'}
        </button>
      ) : null}
    </div>
  );
}
