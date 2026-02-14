// AgentDetailPanel - modal overlay detail view for a single agent.
// Renders as a full-screen overlay with centered timeline inspired by the
// reference design in design_ref/refreshed_ui_for_inbox_and_mail/.

import { useState, useCallback, useEffect } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Avatar } from '@/components/ui/Avatar.js';
import { getAgentContext } from '@/lib/utils.js';
import { StatusBadge } from './AgentCard.js';
import { ActivityTimeline } from './ActivityTimeline.js';
import { ComposeModal } from '@/components/inbox/ComposeModal.js';
import { useAgentSummary, useSummaryHistory } from '@/hooks/useSummaries.js';
import { useInfiniteAgentActivities } from '@/hooks/useActivities.js';
import { useSendMessage } from '@/hooks/useMessages.js';
import { autocompleteRecipients } from '@/api/search.js';
import { useUIStore } from '@/stores/ui.js';
import type { AgentWithStatus, AutocompleteRecipient } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Format time since last activity.
function formatTimeSince(seconds: number): string {
  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

// Format relative time from ISO date string.
function formatRelativeTime(isoDate: string): string {
  const date = new Date(isoDate);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  return formatTimeSince(seconds);
}

export interface AgentDetailPanelProps {
  agent: AgentWithStatus;
  onBack: () => void;
  isOpen?: boolean;
  className?: string;
}

export function AgentDetailPanel({
  agent,
  onBack,
  isOpen = true,
  className,
}: AgentDetailPanelProps) {
  const { data: summary, isLoading: summaryLoading } = useAgentSummary(agent.id);
  const { data: summaryHistory } = useSummaryHistory(agent.id, 50);

  // Fetch activities for the agent.
  const activitiesQuery = useInfiniteAgentActivities(agent.id);
  const activities = activitiesQuery.data?.pages.flatMap((p) => p.data) ?? [];

  // Compose modal state for sending messages to this agent.
  const [composeOpen, setComposeOpen] = useState(false);
  const sendMessage = useSendMessage();
  const addToast = useUIStore((state) => state.addToast);

  // Pre-fill the agent as the sole recipient.
  const agentRecipient: AutocompleteRecipient = {
    id: agent.id,
    name: agent.name,
    ...(agent.project_key && { project_key: agent.project_key }),
    ...(agent.git_branch && { git_branch: agent.git_branch }),
    status: agent.status,
  };

  const handleSendMessage = useCallback(
    async (data: Parameters<typeof sendMessage.mutateAsync>[0]) => {
      try {
        await sendMessage.mutateAsync(data);
        addToast({ variant: 'success', message: `Message sent to ${agent.name}` });
        setComposeOpen(false);
      } catch {
        addToast({ variant: 'error', message: 'Failed to send message' });
        throw new Error('Send failed');
      }
    },
    [sendMessage, addToast, agent.name],
  );

  // Close on Escape key.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !composeOpen) {
        onBack();
      }
    };
    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown);
      // Prevent body scroll while modal is open.
      document.body.style.overflow = 'hidden';
    }
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = '';
    };
  }, [isOpen, onBack, composeOpen]);

  if (!isOpen) return null;

  return (
    <>
      {/* Backdrop. */}
      <div
        className="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm"
        onClick={onBack}
        aria-hidden="true"
      />

      {/* Centered modal panel. */}
      <div
        className={cn(
          'fixed inset-4 z-50 mx-auto flex max-w-5xl flex-col',
          'rounded-xl bg-white shadow-2xl',
          className,
        )}
      >
        {/* Sticky header with back button and agent info. */}
        <div className="shrink-0 border-b border-gray-200 bg-white px-8 py-5">
          <div className="flex items-center justify-between">
            <button
              onClick={onBack}
              className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 transition-colors"
            >
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
              Close
            </button>
            <div className="flex items-center gap-3">
              <button
                onClick={() => setComposeOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                Send Message
              </button>
              <StatusBadge status={agent.status} size="md" />
            </div>
          </div>
        </div>

        {/* Scrollable content. */}
        <div className="flex-1 overflow-y-auto">
          {/* Centered agent header — inspired by reference "Conversation with..." */}
          <div className="px-8 pt-8 pb-6 text-center">
            <div className="flex justify-center mb-3">
              <Avatar name={agent.name} size="lg" />
            </div>
            <h2 className="text-2xl font-bold text-gray-900">{agent.name}</h2>
            {getAgentContext(agent) ? (
              <p className="mt-1 text-sm text-gray-500">{getAgentContext(agent)}</p>
            ) : null}
            {agent.git_branch ? (
              <p className="mt-1 flex items-center justify-center gap-1 text-xs text-gray-400">
                <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 3v12M18 9a3 3 0 01-3 3H6m12-6a3 3 0 10-6 0 3 3 0 006 0zM6 21a3 3 0 100-6 3 3 0 000 6z" />
                </svg>
                <code>{agent.git_branch}</code>
              </p>
            ) : null}
            <div className="mt-2 flex items-center justify-center gap-3 text-sm text-gray-500">
              {agent.session_id !== undefined && agent.session_id !== 0 ? (
                <span>Session #{agent.session_id}</span>
              ) : null}
              <span>{formatTimeSince(agent.seconds_since_heartbeat)}</span>
            </div>
          </div>

          {/* Current activity summary. */}
          {summaryLoading ? (
            <div className="mx-8 mb-6 rounded-lg border border-gray-100 bg-gray-50 p-5">
              <div className="space-y-2">
                <div className="h-4 w-full animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-3/4 animate-pulse rounded bg-gray-200" />
              </div>
            </div>
          ) : summary ? (
            <div className="mx-8 mb-6 rounded-lg border border-blue-100 bg-blue-50/50 p-5">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 shrink-0 rounded-full bg-blue-100 p-1.5">
                  <svg className="h-4 w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-gray-900">Current Activity</p>
                  <p className="mt-1 text-sm text-gray-700 leading-relaxed">
                    {summary.summary}
                  </p>
                  {summary.delta && summary.delta !== 'Initial summary' ? (
                    <div className="mt-2 flex items-start gap-1.5">
                      <span className="text-xs font-bold text-blue-600">&#916;</span>
                      <p className="text-xs text-gray-500">{summary.delta}</p>
                    </div>
                  ) : null}
                  <div className="mt-2 flex items-center justify-between">
                    <span className="text-xs text-gray-400">
                      Updated {formatRelativeTime(summary.generated_at)}
                    </span>
                    {summary.is_stale ? (
                      <span className="inline-flex items-center gap-1 text-xs text-blue-500">
                        <svg className="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                        </svg>
                        Refreshing
                      </span>
                    ) : null}
                  </div>
                </div>
              </div>
            </div>
          ) : null}

          {/* Activity timeline section. */}
          <div className="px-8 pb-8">
            <ActivityTimeline
              activities={activities}
              summaries={summaryHistory ?? []}
            />
          </div>
        </div>
      </div>

      {/* Compose message modal (renders above the overlay). */}
      <ComposeModal
        isOpen={composeOpen}
        onClose={() => setComposeOpen(false)}
        onSend={handleSendMessage}
        onSearchRecipients={autocompleteRecipients}
        isSending={sendMessage.isPending}
        initialValues={{ recipients: [agentRecipient] }}
        title={`Message ${agent.name}`}
      />
    </>
  );
}
