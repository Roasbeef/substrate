// SessionDetail component - modal showing session details with tabs.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Modal } from '@/components/ui/Modal.js';
import { Button } from '@/components/ui/Button.js';
import { Badge } from '@/components/ui/Badge.js';
import { Avatar } from '@/components/ui/Avatar.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { Session, SessionStatus } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Tab types.
type TabId = 'overview' | 'log' | 'tasks';

// Session log entry.
export interface SessionLogEntry {
  id: number;
  timestamp: string;
  type: 'progress' | 'discovery' | 'decision' | 'blocker' | 'checkpoint';
  message: string;
  metadata?: Record<string, unknown>;
}

// Session task.
export interface SessionTask {
  id: string;
  subject: string;
  status: 'pending' | 'in_progress' | 'completed';
  description?: string;
}

// Props for SessionDetail.
export interface SessionDetailProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** The session to display. */
  session?: Session;
  /** Whether session data is loading. */
  isLoading?: boolean;
  /** Session log entries. */
  logEntries?: SessionLogEntry[];
  /** Session tasks. */
  tasks?: SessionTask[];
  /** Handler for completing the session. */
  onComplete?: (sessionId: number) => void;
  /** Whether complete action is in progress. */
  isCompleting?: boolean;
  /** Additional class name. */
  className?: string;
}

// Map session status to badge variant.
function getStatusVariant(status: SessionStatus): 'success' | 'warning' | 'default' {
  switch (status) {
    case 'active':
      return 'success';
    case 'completed':
      return 'default';
    case 'abandoned':
      return 'warning';
    default:
      return 'default';
  }
}

// Format duration from start time.
function formatDuration(startedAt: string, endedAt?: string): string {
  const start = new Date(startedAt);
  const end = endedAt ? new Date(endedAt) : new Date();
  const diffMs = end.getTime() - start.getTime();

  const minutes = Math.floor(diffMs / 60000);
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;

  if (hours > 0) {
    return `${hours}h ${remainingMinutes}m`;
  }
  return `${minutes}m`;
}

// Format timestamp.
function formatTimestamp(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Tab component.
function Tab({
  label,
  isActive,
  onClick,
}: {
  label: string;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'border-b-2 px-4 py-2 text-sm font-medium transition-colors',
        isActive
          ? 'border-blue-500 text-blue-600'
          : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700',
      )}
    >
      {label}
    </button>
  );
}

// Overview tab content.
function OverviewTab({ session }: { session: Session }) {
  const duration = formatDuration(session.started_at, session.ended_at);

  return (
    <div className="space-y-6">
      {/* Agent info. */}
      <div className="flex items-center gap-4">
        <Avatar name={session.agent_name} size="lg" />
        <div>
          <h3 className="text-lg font-medium text-gray-900">{session.agent_name}</h3>
          <p className="text-sm text-gray-500">Session #{session.id}</p>
        </div>
      </div>

      {/* Session details. */}
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-gray-200 p-4">
          <dt className="text-xs font-medium uppercase tracking-wider text-gray-500">
            Project
          </dt>
          <dd className="mt-1 text-sm text-gray-900">
            {session.project ?? '—'}
          </dd>
        </div>

        <div className="rounded-lg border border-gray-200 p-4">
          <dt className="text-xs font-medium uppercase tracking-wider text-gray-500">
            Branch
          </dt>
          <dd className="mt-1 text-sm font-mono text-gray-900">
            {session.branch ?? '—'}
          </dd>
        </div>

        <div className="rounded-lg border border-gray-200 p-4">
          <dt className="text-xs font-medium uppercase tracking-wider text-gray-500">
            Duration
          </dt>
          <dd className="mt-1 text-sm text-gray-900">{duration}</dd>
        </div>

        <div className="rounded-lg border border-gray-200 p-4">
          <dt className="text-xs font-medium uppercase tracking-wider text-gray-500">
            Status
          </dt>
          <dd className="mt-1">
            <Badge variant={getStatusVariant(session.status)}>
              {session.status}
            </Badge>
          </dd>
        </div>
      </div>

      {/* Timestamps. */}
      <div className="rounded-lg border border-gray-200 p-4">
        <h4 className="text-xs font-medium uppercase tracking-wider text-gray-500">
          Timeline
        </h4>
        <dl className="mt-2 space-y-2">
          <div className="flex justify-between">
            <dt className="text-sm text-gray-500">Started</dt>
            <dd className="text-sm text-gray-900">
              {formatTimestamp(session.started_at)}
            </dd>
          </div>
          {session.ended_at ? (
            <div className="flex justify-between">
              <dt className="text-sm text-gray-500">Ended</dt>
              <dd className="text-sm text-gray-900">
                {formatTimestamp(session.ended_at)}
              </dd>
            </div>
          ) : null}
        </dl>
      </div>
    </div>
  );
}

// Log entry icon component.
function LogEntryIcon({ type }: { type: SessionLogEntry['type'] }) {
  const iconClass = 'h-4 w-4';

  switch (type) {
    case 'progress':
      return (
        <svg className={cn(iconClass, 'text-green-500')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      );
    case 'discovery':
      return (
        <svg className={cn(iconClass, 'text-blue-500')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
      );
    case 'decision':
      return (
        <svg className={cn(iconClass, 'text-purple-500')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
        </svg>
      );
    case 'blocker':
      return (
        <svg className={cn(iconClass, 'text-red-500')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      );
    case 'checkpoint':
      return (
        <svg className={cn(iconClass, 'text-gray-500')} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" />
        </svg>
      );
    default:
      return null;
  }
}

// Log tab content.
function LogTab({ entries }: { entries: SessionLogEntry[] }) {
  if (entries.length === 0) {
    return (
      <div className="py-8 text-center">
        <p className="text-sm text-gray-500">No log entries yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {entries.map((entry) => (
        <div key={entry.id} className="flex gap-3">
          <div className="flex-shrink-0 pt-0.5">
            <LogEntryIcon type={entry.type} />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm text-gray-900">{entry.message}</p>
            <p className="mt-0.5 text-xs text-gray-500">
              {formatTimestamp(entry.timestamp)}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

// Task status icon.
function TaskStatusIcon({ status }: { status: SessionTask['status'] }) {
  switch (status) {
    case 'completed':
      return (
        <svg className="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
          <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
        </svg>
      );
    case 'in_progress':
      return <Spinner size="sm" />;
    case 'pending':
    default:
      return (
        <div className="h-5 w-5 rounded-full border-2 border-gray-300" />
      );
  }
}

// Tasks tab content.
function TasksTab({ tasks }: { tasks: SessionTask[] }) {
  if (tasks.length === 0) {
    return (
      <div className="py-8 text-center">
        <p className="text-sm text-gray-500">No tasks in this session</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {tasks.map((task) => (
        <div
          key={task.id}
          className="flex items-start gap-3 rounded-lg border border-gray-200 p-3"
        >
          <TaskStatusIcon status={task.status} />
          <div className="min-w-0 flex-1">
            <p className={cn(
              'text-sm',
              task.status === 'completed' ? 'text-gray-500 line-through' : 'text-gray-900',
            )}>
              {task.subject}
            </p>
            {task.description ? (
              <p className="mt-0.5 text-xs text-gray-500">{task.description}</p>
            ) : null}
          </div>
          <Badge
            size="sm"
            variant={
              task.status === 'completed'
                ? 'default'
                : task.status === 'in_progress'
                  ? 'success'
                  : 'warning'
            }
          >
            {task.status.replace('_', ' ')}
          </Badge>
        </div>
      ))}
    </div>
  );
}

// Loading state.
function LoadingState() {
  return (
    <div className="flex items-center justify-center py-12">
      <Spinner size="lg" />
    </div>
  );
}

export function SessionDetail({
  isOpen,
  onClose,
  session,
  isLoading = false,
  logEntries = [],
  tasks = [],
  onComplete,
  isCompleting = false,
  className,
}: SessionDetailProps) {
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  // Handle complete button click.
  const handleComplete = () => {
    if (session && onComplete) {
      onComplete(session.id);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={session ? `Session #${session.id}` : 'Session Details'}
      className={cn('max-w-2xl', className)}
    >
      {isLoading ? (
        <LoadingState />
      ) : !session ? (
        <div className="py-8 text-center">
          <p className="text-sm text-gray-500">Session not found</p>
        </div>
      ) : (
        <div className="space-y-4">
          {/* Tabs. */}
          <div className="border-b border-gray-200">
            <nav className="-mb-px flex gap-4">
              <Tab
                label="Overview"
                isActive={activeTab === 'overview'}
                onClick={() => setActiveTab('overview')}
              />
              <Tab
                label="Log"
                isActive={activeTab === 'log'}
                onClick={() => setActiveTab('log')}
              />
              <Tab
                label="Tasks"
                isActive={activeTab === 'tasks'}
                onClick={() => setActiveTab('tasks')}
              />
            </nav>
          </div>

          {/* Tab content. */}
          <div className="min-h-[300px]">
            {activeTab === 'overview' ? (
              <OverviewTab session={session} />
            ) : activeTab === 'log' ? (
              <LogTab entries={logEntries} />
            ) : activeTab === 'tasks' ? (
              <TasksTab tasks={tasks} />
            ) : null}
          </div>

          {/* Actions. */}
          {session.status === 'active' && onComplete ? (
            <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
              <Button variant="secondary" onClick={onClose}>
                Close
              </Button>
              <Button
                variant="primary"
                onClick={handleComplete}
                isLoading={isCompleting}
                disabled={isCompleting}
              >
                Complete Session
              </Button>
            </div>
          ) : (
            <div className="flex justify-end border-t border-gray-200 pt-4">
              <Button variant="secondary" onClick={onClose}>
                Close
              </Button>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
