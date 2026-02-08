// TasksPage — mission control for Claude Code agent tasks.
// List view, kanban board, and inspector-style detail panel.

import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useTasks, useTaskStats, useAgentTaskStats } from '@/hooks/useTasks.js';
import { useAgentsStatus } from '@/hooks/useAgents.js';
import { getAgentContext } from '@/lib/utils.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { Task, TaskListOptions } from '@/api/tasks.js';
import type { AgentWithStatus } from '@/types/api.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// ---------------------------------------------------------------------------
// Types and constants.
// ---------------------------------------------------------------------------

type ViewMode = 'list' | 'board';

const statusFilters = [
  { label: 'All', value: '' },
  { label: 'In Progress', value: 'in_progress' },
  { label: 'Pending', value: 'pending' },
  { label: 'Completed', value: 'completed' },
] as const;

// Semantic status configuration — colors, labels, and tailwind classes.
const STATUS: Record<
  string,
  {
    label: string;
    bg: string;
    bgSubtle: string;
    text: string;
    border: string;
    dot: string;
    accent: string;
    columnBg: string;
  }
> = {
  pending: {
    label: 'Pending',
    bg: 'bg-amber-100',
    bgSubtle: 'bg-amber-50',
    text: 'text-amber-700',
    border: 'border-amber-200',
    dot: 'bg-amber-400',
    accent: 'text-amber-600',
    columnBg: 'bg-amber-50/60',
  },
  in_progress: {
    label: 'In Progress',
    bg: 'bg-blue-100',
    bgSubtle: 'bg-blue-50',
    text: 'text-blue-700',
    border: 'border-blue-200',
    dot: 'bg-blue-500',
    accent: 'text-blue-600',
    columnBg: 'bg-blue-50/60',
  },
  completed: {
    label: 'Completed',
    bg: 'bg-emerald-100',
    bgSubtle: 'bg-emerald-50',
    text: 'text-emerald-700',
    border: 'border-emerald-200',
    dot: 'bg-emerald-500',
    accent: 'text-emerald-600',
    columnBg: 'bg-emerald-50/60',
  },
  deleted: {
    label: 'Deleted',
    bg: 'bg-gray-100',
    bgSubtle: 'bg-gray-50',
    text: 'text-gray-500',
    border: 'border-gray-200',
    dot: 'bg-gray-400',
    accent: 'text-gray-500',
    columnBg: 'bg-gray-50/60',
  },
};

// Fallback status config for unknown statuses.
const FALLBACK_STATUS = STATUS['deleted']!;

function getStatus(status: string): typeof FALLBACK_STATUS {
  return STATUS[status] ?? FALLBACK_STATUS;
}

// Build a unique key for a task.
function taskKey(t: Task) {
  return `${t.list_id}::${t.claude_task_id}`;
}

// ---------------------------------------------------------------------------
// Micro-components.
// ---------------------------------------------------------------------------

function Badge({ status }: { status: string }) {
  const s = getStatus(status);
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5',
        'text-[11px] font-semibold tracking-wide uppercase',
        s.bg, s.text,
      )}
    >
      <span className={cn('h-1.5 w-1.5 rounded-full', s.dot)} />
      {s.label}
    </span>
  );
}

function PulsingDot({ status, blocked }: { status: string; blocked?: boolean }) {
  if (status === 'in_progress') {
    return (
      <span className="relative flex h-2.5 w-2.5">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-60" />
        <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-blue-500" />
      </span>
    );
  }
  if (status === 'completed') {
    return (
      <svg className="h-3.5 w-3.5 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
          clipRule="evenodd"
        />
      </svg>
    );
  }
  return (
    <span
      className={cn(
        'h-2.5 w-2.5 rounded-full border-2',
        blocked ? 'border-orange-400 bg-orange-100' : 'border-gray-300 bg-white',
      )}
    />
  );
}

// ---------------------------------------------------------------------------
// Time formatting.
// ---------------------------------------------------------------------------

function relativeTime(date: Date): string {
  const ms = Date.now() - date.getTime();
  const mins = Math.floor(ms / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 7) return `${days}d ago`;
  return date.toLocaleDateString();
}

function fullTimestamp(date: Date | null): string {
  if (!date) return '—';
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// ---------------------------------------------------------------------------
// Stats card — compact, punchy number display.
// ---------------------------------------------------------------------------

function Stat({
  label,
  value,
  color,
}: {
  label: string;
  value: number;
  color: 'blue' | 'yellow' | 'green' | 'orange' | 'gray';
}) {
  const palette: Record<string, string> = {
    blue: 'bg-blue-50 text-blue-700 border-blue-200',
    yellow: 'bg-yellow-50 text-yellow-700 border-yellow-200',
    green: 'bg-green-50 text-green-700 border-green-200',
    orange: 'bg-orange-50 text-orange-700 border-orange-200',
    gray: 'bg-gray-50 text-gray-600 border-gray-200',
  };
  return (
    <div className={cn('rounded-lg border px-3 py-2.5', palette[color])}>
      <div className="text-2xl font-bold tabular-nums leading-none">{value}</div>
      <div className="mt-1 text-[11px] font-medium uppercase tracking-wider opacity-70">
        {label}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// View toggle — segmented control.
// ---------------------------------------------------------------------------

function ViewToggle({ mode, onChange }: { mode: ViewMode; onChange: (m: ViewMode) => void }) {
  return (
    <div className="inline-flex rounded-lg bg-gray-100 p-0.5">
      {(['list', 'board'] as const).map((m) => (
        <button
          key={m}
          type="button"
          onClick={() => onChange(m)}
          className={cn(
            'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium',
            'transition-all duration-150',
            mode === m
              ? 'bg-white text-gray-900 shadow-sm'
              : 'text-gray-500 hover:text-gray-700',
          )}
        >
          {m === 'list' ? (
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          ) : (
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"
              />
            </svg>
          )}
          <span className="capitalize">{m}</span>
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent filter dropdown — only active agents, with project/branch context.
// ---------------------------------------------------------------------------

function AgentFilterSelect({
  agents,
  value,
  onChange,
}: {
  agents: AgentWithStatus[];
  value: number | undefined;
  onChange: (v: number | undefined) => void;
}) {
  // Only include agents that are not offline.
  const activeAgents = useMemo(
    () => agents.filter((a) => a.status !== 'offline'),
    [agents],
  );

  return (
    <select
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value ? Number(e.target.value) : undefined)}
      className={cn(
        'rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm',
        'text-gray-700 focus:border-blue-400 focus:outline-none focus:ring-1 focus:ring-blue-400',
        'max-w-xs truncate',
      )}
    >
      <option value="">All Agents</option>
      {activeAgents.map((agent) => {
        const context = getAgentContext(agent);
        const display = context
          ? `${agent.name}  ·  ${context}`
          : agent.name;
        return (
          <option key={agent.id} value={agent.id}>
            {display}
          </option>
        );
      })}
    </select>
  );
}

// ---------------------------------------------------------------------------
// Task row — list view. Clickable, with selection highlight.
// ---------------------------------------------------------------------------

function TaskRow({
  task,
  selected,
  onClick,
  style,
}: {
  task: Task;
  selected: boolean;
  onClick: () => void;
  style?: React.CSSProperties;
}) {
  const blocked = task.blocked_by?.length > 0;
  const blocking = task.blocks?.length > 0;

  return (
    <button
      type="button"
      onClick={onClick}
      style={style}
      className={cn(
        'w-full text-left flex items-start gap-3.5 rounded-lg border p-4',
        'transition-all duration-150 cursor-pointer group animate-fade-up',
        selected
          ? 'border-blue-400 bg-blue-50/50 ring-1 ring-blue-200 shadow-sm'
          : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-sm',
      )}
    >
      <div className="flex-shrink-0 pt-1">
        <PulsingDot status={task.status} blocked={blocked} />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-gray-900 leading-snug truncate">
              {task.subject}
            </h3>
            {task.active_form && task.status === 'in_progress' && (
              <p className="mt-0.5 text-xs text-blue-600 italic truncate">
                {task.active_form}
              </p>
            )}
            {task.description && (
              <p className="mt-1 text-[13px] text-gray-500 line-clamp-2 leading-relaxed">
                {task.description}
              </p>
            )}
          </div>
          <Badge status={task.status} />
        </div>

        {/* Metadata chips. */}
        <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-400">
          {task.owner && (
            <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-gray-600 font-medium">
              <svg className="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
              {task.owner}
            </span>
          )}
          {blocked && (
            <span className="inline-flex items-center gap-1 text-orange-600 font-medium">
              <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              Blocked by {task.blocked_by.length}
            </span>
          )}
          {blocking && (
            <span className="inline-flex items-center gap-1 text-purple-600 font-medium">
              <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
              Blocks {task.blocks.length}
            </span>
          )}
          {task.claude_task_id && (
            <span className="font-mono text-gray-400">
              #{task.claude_task_id}
            </span>
          )}
          {task.updated_at && (
            <span title={task.updated_at.toLocaleString()}>
              {relativeTime(task.updated_at)}
            </span>
          )}
        </div>
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Kanban card — compact card for board columns.
// ---------------------------------------------------------------------------

function KanbanCard({
  task,
  selected,
  onClick,
  style,
  depHighlight,
}: {
  task: Task;
  selected: boolean;
  onClick: () => void;
  style?: React.CSSProperties;
  depHighlight?: 'upstream' | 'downstream' | null | undefined;
}) {
  const blocked = task.blocked_by?.length > 0;

  return (
    <button
      type="button"
      onClick={onClick}
      style={style}
      className={cn(
        'w-full text-left rounded-lg border p-3 animate-fade-up',
        'transition-all duration-150 cursor-pointer',
        selected
          ? 'border-blue-400 bg-blue-50/60 ring-1 ring-blue-200 shadow-sm'
          : depHighlight === 'upstream'
            ? 'border-orange-300 bg-orange-50/40 ring-2 ring-orange-200 shadow-sm'
            : depHighlight === 'downstream'
              ? 'border-purple-300 bg-purple-50/40 ring-2 ring-purple-200 shadow-sm'
              : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-md',
      )}
    >
      <h4 className="text-[13px] font-semibold text-gray-900 leading-snug">
        {task.subject}
      </h4>

      {task.active_form && task.status === 'in_progress' && (
        <p className="mt-1 text-xs text-blue-600 italic truncate">{task.active_form}</p>
      )}

      {task.description && (
        <p className="mt-1 text-xs text-gray-500 line-clamp-2 leading-relaxed">
          {task.description}
        </p>
      )}

      <div className="mt-2.5 flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 min-w-0">
          {task.owner && (
            <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 truncate max-w-[120px]">
              <svg className="h-2.5 w-2.5 flex-shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
              <span className="truncate">{task.owner}</span>
            </span>
          )}
          {blocked && (
            <span className="inline-flex items-center gap-1 rounded-full bg-orange-50 px-1.5 py-0.5 text-[10px] font-semibold text-orange-600">
              <svg className="h-2.5 w-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01" />
              </svg>
              Blocked
            </span>
          )}
        </div>
        <div className="flex items-center gap-1.5 text-[10px] text-gray-400 flex-shrink-0">
          {task.claude_task_id && (
            <span className="font-mono">#{task.claude_task_id}</span>
          )}
          {task.updated_at && (
            <span>{relativeTime(task.updated_at)}</span>
          )}
        </div>
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Kanban column.
// ---------------------------------------------------------------------------

function KanbanColumn({
  status,
  tasks,
  selectedKey,
  onSelect,
  getDepHighlight,
}: {
  status: string;
  tasks: Task[];
  selectedKey: string | null;
  onSelect: (t: Task) => void;
  getDepHighlight?: (task: Task) => 'upstream' | 'downstream' | null;
}) {
  const s = getStatus(status);

  return (
    <div className="flex flex-1 flex-col min-w-[280px] max-w-[400px]">
      {/* Column header. */}
      <div className={cn('flex items-center gap-2 rounded-t-lg border px-3 py-2', s.bgSubtle, s.border)}>
        <span className={cn('h-2 w-2 rounded-full', s.dot)} />
        <span className={cn('text-sm font-semibold', s.text)}>{s.label}</span>
        <span
          className={cn(
            'ml-auto rounded-full px-1.5 py-0.5 text-[10px] font-bold tabular-nums',
            s.bg, s.text,
          )}
        >
          {tasks.length}
        </span>
      </div>

      {/* Card list. */}
      <div
        className={cn(
          'flex-1 space-y-2 rounded-b-lg border border-t-0 p-2',
          'border-gray-200 overflow-y-auto scrollbar-thin',
          'min-h-[180px] max-h-[calc(100vh-340px)]',
          s.columnBg,
        )}
      >
        {tasks.length === 0 ? (
          <div className="flex h-20 items-center justify-center">
            <p className="text-xs text-gray-400 italic">No tasks</p>
          </div>
        ) : (
          tasks.map((task, i) => (
            <KanbanCard
              key={taskKey(task)}
              task={task}
              selected={selectedKey === taskKey(task)}
              onClick={() => onSelect(task)}
              style={{ animationDelay: `${i * 40}ms` }}
              depHighlight={getDepHighlight?.(task)}
            />
          ))
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Inspector-style detail panel (slide-over).
// ---------------------------------------------------------------------------

function DetailPanel({
  task,
  allTasks,
  onClose,
  onNavigateToDep,
}: {
  task: Task;
  allTasks: Task[];
  onClose: () => void;
  onNavigateToDep: (id: string) => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const s = getStatus(task.status);
  const blocked = task.blocked_by?.length > 0;
  const blocking = task.blocks?.length > 0;

  // Parse metadata JSON if present.
  let metadata: Record<string, unknown> | null = null;
  if (task.metadata_json) {
    try {
      metadata = JSON.parse(task.metadata_json);
    } catch {
      // Ignore parse errors.
    }
  }

  // Close on Escape.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <>
      {/* Scrim. */}
      <div
        className="fixed inset-0 z-40 bg-black/15 backdrop-blur-[2px] animate-fade-in"
        onClick={onClose}
      />

      {/* Panel. */}
      <div
        ref={panelRef}
        className={cn(
          'fixed inset-y-0 right-0 z-50 w-full sm:w-[460px]',
          'flex flex-col bg-white shadow-2xl animate-slide-in',
          'border-l border-gray-200',
        )}
      >
        {/* Header. */}
        <div className={cn('flex items-center justify-between border-b px-5 py-3', s.bgSubtle, s.border)}>
          <div className="flex items-center gap-3 min-w-0">
            <Badge status={task.status} />
            {task.claude_task_id && (
              <span className="text-xs text-gray-400 font-mono">#{task.claude_task_id}</span>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100/80 transition-colors"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Body. */}
        <div className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="px-5 py-5 space-y-5">
            {/* Title. */}
            <div>
              <h2 className="text-lg font-bold text-gray-900 leading-snug tracking-tight">
                {task.subject}
              </h2>
              {task.active_form && task.status === 'in_progress' && (
                <div className="mt-2 flex items-center gap-2 text-sm text-blue-600">
                  <span className="relative flex h-2 w-2">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-60" />
                    <span className="relative inline-flex h-2 w-2 rounded-full bg-blue-500" />
                  </span>
                  <span className="italic">{task.active_form}</span>
                </div>
              )}
            </div>

            {/* Description. */}
            {task.description && (
              <Section title="Description">
                <p className="text-sm text-gray-700 leading-relaxed whitespace-pre-wrap">
                  {task.description}
                </p>
              </Section>
            )}

            {/* Owner. */}
            {task.owner && (
              <Section title="Owner">
                <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-700">
                  <svg className="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                  </svg>
                  {task.owner}
                </span>
              </Section>
            )}

            {/* Dependencies — visual graph and detail cards. */}
            {(blocked || blocking) && (
              <Section title="Dependencies">
                <DependencyGraph
                  task={task}
                  allTasks={allTasks}
                  onNavigate={(t) => onNavigateToDep(t.claude_task_id)}
                />
                <div className="mt-3 space-y-2">
                  {blocked && (
                    <DepCard
                      variant="blocked"
                      label="Blocked by"
                      ids={task.blocked_by}
                      onNavigate={onNavigateToDep}
                    />
                  )}
                  {blocking && (
                    <DepCard
                      variant="blocks"
                      label="Blocks"
                      ids={task.blocks}
                      onNavigate={onNavigateToDep}
                    />
                  )}
                </div>
              </Section>
            )}

            {/* Metadata. */}
            {metadata && Object.keys(metadata).length > 0 && (
              <Section title="Metadata">
                <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                  <dl className="space-y-1.5">
                    {Object.entries(metadata).map(([key, value]) => (
                      <div key={key} className="flex gap-2 text-xs">
                        <dt className="font-medium text-gray-500 min-w-[80px]">{key}</dt>
                        <dd className="text-gray-700 font-mono break-all">{String(value)}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              </Section>
            )}

            {/* Timeline. */}
            <Section title="Timeline">
              <div className="rounded-lg border border-gray-200 overflow-hidden">
                <table className="w-full text-xs">
                  <tbody className="divide-y divide-gray-100">
                    {[
                      ['Created', task.created_at],
                      ['Updated', task.updated_at],
                      ['Started', task.started_at],
                      ['Completed', task.completed_at],
                    ]
                      .filter(([, v]) => v != null)
                      .map(([label, date]) => (
                        <tr key={label as string}>
                          <td className="px-3 py-2 font-medium text-gray-500 bg-gray-50/80 w-24">
                            {label as string}
                          </td>
                          <td className="px-3 py-2 text-gray-700 font-mono">
                            {fullTimestamp(date as Date | null)}
                          </td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            </Section>

            {/* Identifiers. */}
            <Section title="Identifiers">
              <div className="space-y-1 text-xs">
                <IdRow label="List ID" value={task.list_id} mono />
                <IdRow label="Task ID" value={task.claude_task_id} mono />
                {task.agent_name && <IdRow label="Agent" value={task.agent_name} />}
              </div>
            </Section>
          </div>
        </div>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Detail panel sub-components.
// ---------------------------------------------------------------------------

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-[11px] font-semibold uppercase tracking-widest text-gray-400 mb-2">
        {title}
      </h3>
      {children}
    </div>
  );
}

function DepCard({
  variant,
  label,
  ids,
  onNavigate,
}: {
  variant: 'blocked' | 'blocks';
  label: string;
  ids: string[];
  onNavigate?: (id: string) => void;
}) {
  const isBlocked = variant === 'blocked';
  return (
    <div
      className={cn(
        'flex items-start gap-2 rounded-lg border p-2.5',
        isBlocked
          ? 'bg-orange-50 border-orange-200'
          : 'bg-purple-50 border-purple-200',
      )}
    >
      <svg
        className={cn(
          'h-4 w-4 flex-shrink-0 mt-0.5',
          isBlocked ? 'text-orange-500' : 'text-purple-500',
        )}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        {isBlocked ? (
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        ) : (
          <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
        )}
      </svg>
      <div>
        <p className={cn('text-xs font-medium', isBlocked ? 'text-orange-700' : 'text-purple-700')}>
          {label}
        </p>
        <div className="mt-1 flex flex-wrap gap-1">
          {ids.map((id) => (
            <button
              key={id}
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onNavigate?.(id);
              }}
              className={cn(
                'inline-flex rounded px-1.5 py-0.5 text-[11px] font-mono',
                'transition-colors',
                isBlocked
                  ? 'bg-orange-100 text-orange-700 hover:bg-orange-200'
                  : 'bg-purple-100 text-purple-700 hover:bg-purple-200',
                onNavigate && 'cursor-pointer',
              )}
            >
              #{id}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Dependency mini-graph — upstream (blocked_by) → current → downstream (blocks).
// ---------------------------------------------------------------------------

function DepArrow() {
  return (
    <div className="flex-shrink-0 flex items-center text-gray-300 self-center">
      <svg className="h-4 w-8" viewBox="0 0 32 16" fill="none">
        <path d="M0 8h24" stroke="currentColor" strokeWidth="1.5" />
        <path
          d="M22 4l6 4-6 4"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}

function DepGraphNode({
  task,
  variant,
  onClick,
}: {
  task: Task;
  variant: 'upstream' | 'current' | 'downstream';
  onClick?: () => void;
}) {
  const s = getStatus(task.status);
  const colors = {
    upstream: 'border-orange-200 bg-orange-50 hover:bg-orange-100',
    current: 'border-blue-400 bg-blue-50 ring-2 ring-blue-200',
    downstream: 'border-purple-200 bg-purple-50 hover:bg-purple-100',
  };

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!onClick}
      className={cn(
        'rounded-lg border px-3 py-2 text-left min-w-[140px] max-w-[180px]',
        'transition-colors',
        colors[variant],
        onClick ? 'cursor-pointer' : 'cursor-default',
      )}
    >
      <div className="flex items-center gap-1.5">
        <span className={cn('h-1.5 w-1.5 rounded-full flex-shrink-0', s.dot)} />
        <span className="text-[10px] font-mono text-gray-400">#{task.claude_task_id}</span>
        <span className={cn('text-[9px] font-semibold uppercase', s.text)}>{s.label}</span>
      </div>
      <p className="mt-1 text-xs font-medium text-gray-800 truncate">
        {task.subject || '(untitled)'}
      </p>
    </button>
  );
}

function DependencyGraph({
  task,
  allTasks,
  onNavigate,
}: {
  task: Task;
  allTasks: Task[];
  onNavigate: (t: Task) => void;
}) {
  const taskMap = useMemo(() => {
    const map = new Map<string, Task>();
    for (const t of allTasks) {
      map.set(t.claude_task_id, t);
    }
    return map;
  }, [allTasks]);

  const upstream = (task.blocked_by ?? [])
    .map((id) => taskMap.get(id))
    .filter((t): t is Task => t != null);
  const downstream = (task.blocks ?? [])
    .map((id) => taskMap.get(id))
    .filter((t): t is Task => t != null);

  // Collect unresolved IDs (referenced but not in current task list).
  const unresolvedUpstream = (task.blocked_by ?? []).filter((id) => !taskMap.has(id));
  const unresolvedDownstream = (task.blocks ?? []).filter((id) => !taskMap.has(id));

  if (upstream.length === 0 && downstream.length === 0 &&
      unresolvedUpstream.length === 0 && unresolvedDownstream.length === 0) {
    return null;
  }

  return (
    <div className="flex items-stretch gap-3 overflow-x-auto py-2 px-1">
      {/* Upstream nodes (blocked by). */}
      {(upstream.length > 0 || unresolvedUpstream.length > 0) && (
        <>
          <div className="flex flex-col gap-1.5 flex-shrink-0">
            <span className="text-[9px] font-semibold uppercase tracking-wider text-orange-400 text-center mb-0.5">
              Blocked by
            </span>
            {upstream.map((t) => (
              <DepGraphNode
                key={t.claude_task_id}
                task={t}
                variant="upstream"
                onClick={() => onNavigate(t)}
              />
            ))}
            {unresolvedUpstream.map((id) => (
              <div
                key={id}
                className="rounded-lg border border-dashed border-orange-200 bg-orange-50/50 px-3 py-2 min-w-[140px]"
              >
                <span className="text-[10px] font-mono text-orange-400">#{id}</span>
                <p className="mt-0.5 text-[10px] text-orange-300 italic">not in scope</p>
              </div>
            ))}
          </div>
          <DepArrow />
        </>
      )}

      {/* Current node. */}
      <div className="flex flex-col gap-1.5 flex-shrink-0">
        <span className="text-[9px] font-semibold uppercase tracking-wider text-blue-400 text-center mb-0.5">
          Current
        </span>
        <DepGraphNode task={task} variant="current" />
      </div>

      {/* Downstream nodes (blocks). */}
      {(downstream.length > 0 || unresolvedDownstream.length > 0) && (
        <>
          <DepArrow />
          <div className="flex flex-col gap-1.5 flex-shrink-0">
            <span className="text-[9px] font-semibold uppercase tracking-wider text-purple-400 text-center mb-0.5">
              Blocks
            </span>
            {downstream.map((t) => (
              <DepGraphNode
                key={t.claude_task_id}
                task={t}
                variant="downstream"
                onClick={() => onNavigate(t)}
              />
            ))}
            {unresolvedDownstream.map((id) => (
              <div
                key={id}
                className="rounded-lg border border-dashed border-purple-200 bg-purple-50/50 px-3 py-2 min-w-[140px]"
              >
                <span className="text-[10px] font-mono text-purple-400">#{id}</span>
                <p className="mt-0.5 text-[10px] text-purple-300 italic">not in scope</p>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function IdRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex gap-2">
      <span className="font-medium text-gray-500 w-16">{label}</span>
      <span className={cn('text-gray-700', mono && 'font-mono')}>{value}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Per-agent summary table.
// ---------------------------------------------------------------------------

function AgentSummaryTable({
  agentStats,
  onSelectAgent,
}: {
  agentStats: Array<{
    agent_id: number;
    agent_name: string;
    pending_count: number;
    in_progress_count: number;
    blocked_count: number;
    completed_today: number;
  }>;
  onSelectAgent: (id: number) => void;
}) {
  if (agentStats.length === 0) return null;

  return (
    <div className="mb-6 animate-fade-up">
      <h2 className="mb-2 text-xs font-semibold uppercase tracking-widest text-gray-400">
        Per-Agent Summary
      </h2>
      <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {['Agent', 'In Progress', 'Pending', 'Blocked', 'Done Today'].map((h) => (
                <th
                  key={h}
                  className={cn(
                    'px-4 py-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500',
                    h === 'Agent' ? 'text-left' : 'text-center',
                  )}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {agentStats.map((stat) => (
              <tr
                key={stat.agent_id}
                className="cursor-pointer hover:bg-gray-50 transition-colors"
                onClick={() => onSelectAgent(stat.agent_id)}
              >
                <td className="whitespace-nowrap px-4 py-2 text-sm font-medium text-gray-900">
                  {stat.agent_name || `Agent ${stat.agent_id}`}
                </td>
                <td className="whitespace-nowrap px-4 py-2 text-center text-sm tabular-nums text-blue-600 font-medium">
                  {stat.in_progress_count || '—'}
                </td>
                <td className="whitespace-nowrap px-4 py-2 text-center text-sm tabular-nums text-amber-600 font-medium">
                  {stat.pending_count || '—'}
                </td>
                <td className="whitespace-nowrap px-4 py-2 text-center text-sm tabular-nums text-orange-600 font-medium">
                  {stat.blocked_count || '—'}
                </td>
                <td className="whitespace-nowrap px-4 py-2 text-center text-sm tabular-nums text-emerald-600 font-medium">
                  {stat.completed_today || '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main TasksPage.
// ---------------------------------------------------------------------------

export default function TasksPage() {
  const [statusFilter, setStatusFilter] = useState('');
  const [agentFilter, setAgentFilter] = useState<number | undefined>(undefined);
  const [viewMode, setViewMode] = useState<ViewMode>('board');
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);

  // List query options.
  const taskOptions: TaskListOptions = {
    ...(agentFilter !== undefined ? { agentId: agentFilter } : {}),
    ...(statusFilter ? { status: statusFilter } : {}),
    limit: 100,
  };

  const { data: tasks, isLoading, error } = useTasks(taskOptions);

  // Board view needs all tasks unfiltered by status.
  const boardOptions: TaskListOptions = {
    ...(agentFilter !== undefined ? { agentId: agentFilter } : {}),
    limit: 200,
  };
  const { data: allTasks } = useTasks(boardOptions);

  const { data: stats } = useTaskStats(agentFilter);
  const { data: agentsData } = useAgentsStatus();
  const { data: agentStats } = useAgentTaskStats();

  const selectedKey = selectedTask ? taskKey(selectedTask) : null;

  const handleSelect = useCallback((task: Task) => {
    setSelectedTask((prev) => {
      if (prev && taskKey(prev) === taskKey(task)) return null;
      return task;
    });
  }, []);

  const handleClose = useCallback(() => setSelectedTask(null), []);

  // Group tasks for board.
  const boardTasks = allTasks ?? [];
  const pendingTasks = boardTasks.filter((t) => t.status === 'pending');
  const inProgressTasks = boardTasks.filter((t) => t.status === 'in_progress');
  const completedTasks = boardTasks.filter((t) => t.status === 'completed');

  // Build task map for dependency navigation.
  const taskMap = useMemo(() => {
    const map = new Map<string, Task>();
    for (const t of boardTasks) {
      map.set(t.claude_task_id, t);
    }
    return map;
  }, [boardTasks]);

  // Dependency highlight — when a task is selected, highlight its deps.
  const getDepHighlight = useCallback(
    (task: Task): 'upstream' | 'downstream' | null => {
      if (!selectedTask) return null;
      if (selectedTask.blocked_by?.includes(task.claude_task_id)) return 'upstream';
      if (selectedTask.blocks?.includes(task.claude_task_id)) return 'downstream';
      return null;
    },
    [selectedTask],
  );

  // Navigate to a dependency by its claude_task_id.
  const handleNavigateToDep = useCallback(
    (id: string) => {
      const target = taskMap.get(id);
      if (target) setSelectedTask(target);
    },
    [taskMap],
  );

  return (
    <div className="p-6">
      {/* Header. */}
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 tracking-tight">Tasks</h1>
          <p className="mt-1 text-sm text-gray-500">
            Track Claude Code agent tasks and progress.
          </p>
        </div>
        <ViewToggle mode={viewMode} onChange={setViewMode} />
      </div>

      {/* Stats row. */}
      {stats && (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <Stat label="In Progress" value={stats.in_progress_count} color="blue" />
          <Stat label="Pending" value={stats.pending_count} color="yellow" />
          <Stat label="Available" value={stats.available_count} color="green" />
          <Stat label="Blocked" value={stats.blocked_count} color="orange" />
          <Stat label="Completed" value={stats.completed_count} color="gray" />
          <Stat label="Today" value={stats.completed_today} color="green" />
        </div>
      )}

      {/* Filters. */}
      <div className="mb-4 flex flex-wrap items-center gap-3">
        {viewMode === 'list' && (
          <div className="flex gap-0.5 rounded-lg bg-gray-100 p-0.5">
            {statusFilters.map((f) => (
              <button
                key={f.value}
                type="button"
                onClick={() => setStatusFilter(f.value)}
                className={cn(
                  'rounded-md px-3 py-1.5 text-sm font-medium transition-all duration-150',
                  statusFilter === f.value
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-500 hover:text-gray-700',
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
        )}

        <AgentFilterSelect
          agents={agentsData?.agents ?? []}
          value={agentFilter}
          onChange={setAgentFilter}
        />
      </div>

      {/* Per-agent summary table (list mode, no agent filter). */}
      {viewMode === 'list' && agentStats && agentStats.length > 0 && !agentFilter && (
        <AgentSummaryTable agentStats={agentStats} onSelectAgent={setAgentFilter} />
      )}

      {/* Content. */}
      {isLoading ? (
        <div className="flex justify-center py-16">
          <Spinner size="lg" variant="primary" label="Loading tasks..." />
        </div>
      ) : error ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-8 text-center">
          <p className="text-sm text-red-700">Failed to load tasks: {error.message}</p>
        </div>
      ) : viewMode === 'board' ? (
        <div className="flex gap-4 overflow-x-auto pb-4">
          <KanbanColumn
            status="pending"
            tasks={pendingTasks}
            selectedKey={selectedKey}
            onSelect={handleSelect}
            getDepHighlight={getDepHighlight}
          />
          <KanbanColumn
            status="in_progress"
            tasks={inProgressTasks}
            selectedKey={selectedKey}
            onSelect={handleSelect}
            getDepHighlight={getDepHighlight}
          />
          <KanbanColumn
            status="completed"
            tasks={completedTasks}
            selectedKey={selectedKey}
            onSelect={handleSelect}
            getDepHighlight={getDepHighlight}
          />
        </div>
      ) : tasks && tasks.length > 0 ? (
        <div className="space-y-2">
          {tasks.map((task, i) => (
            <TaskRow
              key={taskKey(task)}
              task={task}
              selected={selectedKey === taskKey(task)}
              onClick={() => handleSelect(task)}
              style={{ animationDelay: `${i * 30}ms` }}
            />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-gray-200 bg-white p-12 text-center animate-fade-up">
          <svg
            className="mx-auto h-12 w-12 text-gray-300"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            />
          </svg>
          <h3 className="mt-4 text-sm font-semibold text-gray-900">No tasks</h3>
          <p className="mt-1 text-sm text-gray-500">
            {statusFilter
              ? `No tasks with status "${statusFilter.replace('_', ' ')}".`
              : agentFilter
                ? 'No tasks for this agent.'
                : 'No tasks have been tracked yet.'}
          </p>
          <p className="mt-3 text-xs text-gray-400">
            Tasks are automatically tracked when agents use TodoWrite.
          </p>
        </div>
      )}

      {/* Detail panel. */}
      {selectedTask && (
        <DetailPanel
          task={selectedTask}
          allTasks={boardTasks}
          onClose={handleClose}
          onNavigateToDep={handleNavigateToDep}
        />
      )}
    </div>
  );
}
