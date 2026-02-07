// TasksPage component - list and detail view for Claude Code tasks.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useTasks, useTaskStats, useAgentTaskStats } from '@/hooks/useTasks.js';
import { useAgentsStatus } from '@/hooks/useAgents.js';
import { Spinner } from '@/components/ui/Spinner.js';
import type { Task } from '@/api/tasks.js';

function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Status filter options.
const statusFilters: Array<{ label: string; value: string }> = [
  { label: 'All', value: '' },
  { label: 'In Progress', value: 'in_progress' },
  { label: 'Pending', value: 'pending' },
  { label: 'Completed', value: 'completed' },
];

// Status badge component.
function TaskStatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800',
    in_progress: 'bg-blue-100 text-blue-800',
    completed: 'bg-green-100 text-green-800',
    deleted: 'bg-gray-100 text-gray-800',
  };

  const labels: Record<string, string> = {
    pending: 'Pending',
    in_progress: 'In Progress',
    completed: 'Completed',
    deleted: 'Deleted',
  };

  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        colors[status] ?? 'bg-gray-100 text-gray-800',
      )}
    >
      {labels[status] ?? status}
    </span>
  );
}

// Task row component.
function TaskRow({ task }: { task: Task }) {
  const hasBlockers = task.blocked_by && task.blocked_by.length > 0;

  return (
    <div
      className={cn(
        'flex items-start gap-4 rounded-lg border border-gray-200 bg-white p-4',
        'hover:border-gray-300 hover:shadow-sm transition-all',
      )}
    >
      {/* Status indicator */}
      <div className="flex-shrink-0 pt-0.5">
        {task.status === 'in_progress' ? (
          <div className="h-3 w-3 animate-pulse rounded-full bg-blue-500" />
        ) : task.status === 'completed' ? (
          <svg
            className="h-5 w-5 text-green-500"
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fillRule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clipRule="evenodd"
            />
          </svg>
        ) : (
          <div
            className={cn(
              'h-3 w-3 rounded-full border-2',
              hasBlockers ? 'border-orange-400' : 'border-gray-300',
            )}
          />
        )}
      </div>

      {/* Task content */}
      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h3 className="text-sm font-medium text-gray-900">{task.subject}</h3>
            {task.active_form && task.status === 'in_progress' && (
              <p className="mt-0.5 text-xs text-blue-600 italic">
                {task.active_form}
              </p>
            )}
            {task.description && (
              <p className="mt-1 text-sm text-gray-500 line-clamp-2">
                {task.description}
              </p>
            )}
          </div>
          <TaskStatusBadge status={task.status} />
        </div>

        {/* Metadata row */}
        <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-gray-500">
          {task.owner && (
            <span className="flex items-center gap-1">
              <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
              {task.owner}
            </span>
          )}
          {hasBlockers && (
            <span className="flex items-center gap-1 text-orange-600">
              <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              Blocked by {task.blocked_by.length} task{task.blocked_by.length !== 1 ? 's' : ''}
            </span>
          )}
          {task.updated_at && (
            <span title={task.updated_at.toLocaleString()}>
              Updated {formatRelativeTime(task.updated_at)}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

// Format relative time.
function formatRelativeTime(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

// Stats card component.
function StatsCard({
  label,
  value,
  color,
}: {
  label: string;
  value: number;
  color: 'blue' | 'yellow' | 'green' | 'orange' | 'gray';
}) {
  const colors = {
    blue: 'bg-blue-50 text-blue-700 border-blue-200',
    yellow: 'bg-yellow-50 text-yellow-700 border-yellow-200',
    green: 'bg-green-50 text-green-700 border-green-200',
    orange: 'bg-orange-50 text-orange-700 border-orange-200',
    gray: 'bg-gray-50 text-gray-700 border-gray-200',
  };

  return (
    <div className={cn('rounded-lg border p-3', colors[color])}>
      <div className="text-2xl font-bold">{value}</div>
      <div className="text-xs font-medium opacity-80">{label}</div>
    </div>
  );
}

export default function TasksPage() {
  const [statusFilter, setStatusFilter] = useState('');
  const [agentFilter, setAgentFilter] = useState<number | undefined>(undefined);

  // Build task list options.
  const taskOptions = {
    agentId: agentFilter,
    status: statusFilter || undefined,
    activeOnly: statusFilter === '' ? undefined : false,
    limit: 100,
  };

  // Fetch tasks.
  const {
    data: tasks,
    isLoading: tasksLoading,
    error: tasksError,
  } = useTasks(taskOptions);

  // Fetch stats for selected agent or global.
  const { data: stats } = useTaskStats(agentFilter);

  // Fetch agents for filter dropdown.
  const { data: agentsData } = useAgentsStatus();

  // Fetch per-agent stats.
  const { data: agentStats } = useAgentTaskStats();

  return (
    <div className="p-6">
      {/* Page header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Tasks</h1>
        <p className="mt-1 text-sm text-gray-500">
          Track Claude Code agent tasks and progress.
        </p>
      </div>

      {/* Stats cards */}
      {stats && (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <StatsCard
            label="In Progress"
            value={stats.in_progress_count}
            color="blue"
          />
          <StatsCard
            label="Pending"
            value={stats.pending_count}
            color="yellow"
          />
          <StatsCard
            label="Available"
            value={stats.available_count}
            color="green"
          />
          <StatsCard
            label="Blocked"
            value={stats.blocked_count}
            color="orange"
          />
          <StatsCard
            label="Completed"
            value={stats.completed_count}
            color="gray"
          />
          <StatsCard
            label="Today"
            value={stats.completed_today}
            color="green"
          />
        </div>
      )}

      {/* Filters */}
      <div className="mb-4 flex flex-wrap items-center gap-4">
        {/* Status filter tabs */}
        <div className="flex gap-1 rounded-lg bg-gray-100 p-1">
          {statusFilters.map((filter) => (
            <button
              key={filter.value}
              type="button"
              onClick={() => setStatusFilter(filter.value)}
              className={cn(
                'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                statusFilter === filter.value
                  ? 'bg-white text-gray-900 shadow-sm'
                  : 'text-gray-600 hover:text-gray-900',
              )}
            >
              {filter.label}
            </button>
          ))}
        </div>

        {/* Agent filter */}
        <select
          value={agentFilter ?? ''}
          onChange={(e) =>
            setAgentFilter(e.target.value ? Number(e.target.value) : undefined)
          }
          className={cn(
            'rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm',
            'focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
          )}
        >
          <option value="">All Agents</option>
          {agentsData?.agents.map((agent) => (
            <option key={agent.id} value={agent.id}>
              {agent.name}
            </option>
          ))}
        </select>
      </div>

      {/* Agent stats summary */}
      {agentStats && agentStats.length > 0 && !agentFilter && (
        <div className="mb-6">
          <h2 className="mb-2 text-sm font-medium text-gray-700">
            Per-Agent Summary
          </h2>
          <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">
                    Agent
                  </th>
                  <th className="px-4 py-2 text-center text-xs font-medium uppercase text-gray-500">
                    In Progress
                  </th>
                  <th className="px-4 py-2 text-center text-xs font-medium uppercase text-gray-500">
                    Pending
                  </th>
                  <th className="px-4 py-2 text-center text-xs font-medium uppercase text-gray-500">
                    Blocked
                  </th>
                  <th className="px-4 py-2 text-center text-xs font-medium uppercase text-gray-500">
                    Done Today
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {agentStats.map((stat) => (
                  <tr
                    key={stat.agent_id}
                    className="cursor-pointer hover:bg-gray-50"
                    onClick={() => setAgentFilter(stat.agent_id)}
                  >
                    <td className="whitespace-nowrap px-4 py-2 text-sm font-medium text-gray-900">
                      {stat.agent_name || `Agent ${stat.agent_id}`}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-center text-sm text-blue-600">
                      {stat.in_progress_count}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-center text-sm text-yellow-600">
                      {stat.pending_count}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-center text-sm text-orange-600">
                      {stat.blocked_count}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2 text-center text-sm text-green-600">
                      {stat.completed_today}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tasks list */}
      {tasksLoading ? (
        <div className="flex justify-center py-12">
          <Spinner size="lg" variant="primary" label="Loading tasks..." />
        </div>
      ) : tasksError ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-center">
          <p className="text-sm text-red-700">
            Failed to load tasks: {tasksError.message}
          </p>
        </div>
      ) : tasks && tasks.length > 0 ? (
        <div className="space-y-2">
          {tasks.map((task) => (
            <TaskRow key={`${task.list_id}-${task.claude_task_id}`} task={task} />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-gray-200 bg-white p-12 text-center">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            />
          </svg>
          <h3 className="mt-4 text-sm font-medium text-gray-900">No tasks</h3>
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
    </div>
  );
}
