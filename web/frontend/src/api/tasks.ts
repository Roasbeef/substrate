// API functions for task-related operations.
// Uses grpc-gateway REST API directly.

import { get, post, patch } from './client.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Task status enum values.
export type TaskStatus =
  | 'TASK_STATUS_UNSPECIFIED'
  | 'TASK_STATUS_PENDING'
  | 'TASK_STATUS_IN_PROGRESS'
  | 'TASK_STATUS_COMPLETED'
  | 'TASK_STATUS_DELETED';

// Map status enum to display string.
export function statusToDisplay(status: TaskStatus): string {
  switch (status) {
    case 'TASK_STATUS_PENDING':
      return 'pending';
    case 'TASK_STATUS_IN_PROGRESS':
      return 'in_progress';
    case 'TASK_STATUS_COMPLETED':
      return 'completed';
    case 'TASK_STATUS_DELETED':
      return 'deleted';
    default:
      return 'unknown';
  }
}

// Map display string to status enum.
export function displayToStatus(display: string): TaskStatus {
  switch (display) {
    case 'pending':
      return 'TASK_STATUS_PENDING';
    case 'in_progress':
      return 'TASK_STATUS_IN_PROGRESS';
    case 'completed':
      return 'TASK_STATUS_COMPLETED';
    case 'deleted':
      return 'TASK_STATUS_DELETED';
    default:
      return 'TASK_STATUS_UNSPECIFIED';
  }
}

// Task interface for frontend.
export interface Task {
  id: number;
  agent_id: number;
  agent_name?: string;
  list_id: string;
  claude_task_id: string;
  subject: string;
  description: string;
  active_form: string;
  metadata_json: string;
  status: string;
  owner: string;
  blocked_by: string[];
  blocks: string[];
  created_at: Date | null;
  updated_at: Date | null;
  started_at: Date | null;
  completed_at: Date | null;
}

// TaskList interface for frontend.
export interface TaskList {
  id: number;
  list_id: string;
  agent_id: number;
  agent_name?: string;
  watch_path: string;
  created_at: Date | null;
  last_synced_at: Date | null;
}

// TaskStats interface for frontend.
export interface TaskStats {
  pending_count: number;
  in_progress_count: number;
  completed_count: number;
  blocked_count: number;
  available_count: number;
  completed_today: number;
}

// AgentTaskStats interface for frontend.
export interface AgentTaskStats {
  agent_id: number;
  agent_name: string;
  pending_count: number;
  in_progress_count: number;
  blocked_count: number;
  completed_today: number;
}

// Gateway response format for ListTasks.
interface GatewayListTasksResponse {
  tasks?: Array<{
    id?: string;
    agent_id?: string;
    list_id?: string;
    claude_task_id?: string;
    subject?: string;
    description?: string;
    active_form?: string;
    metadata_json?: string;
    status?: TaskStatus;
    owner?: string;
    blocked_by?: string[];
    blocks?: string[];
    created_at?: string;
    updated_at?: string;
    started_at?: string;
    completed_at?: string;
  }>;
  error?: string;
}

// Gateway response format for GetTask.
interface GatewayTaskResponse {
  task?: {
    id?: string;
    agent_id?: string;
    list_id?: string;
    claude_task_id?: string;
    subject?: string;
    description?: string;
    active_form?: string;
    metadata_json?: string;
    status?: TaskStatus;
    owner?: string;
    blocked_by?: string[];
    blocks?: string[];
    created_at?: string;
    updated_at?: string;
    started_at?: string;
    completed_at?: string;
  };
  error?: string;
}

// Gateway response format for ListTaskLists.
interface GatewayListTaskListsResponse {
  task_lists?: Array<{
    id?: string;
    list_id?: string;
    agent_id?: string;
    watch_path?: string;
    created_at?: string;
    last_synced_at?: string;
  }>;
  error?: string;
}

// Gateway response format for GetTaskStats.
interface GatewayTaskStatsResponse {
  stats?: {
    pending_count?: string;
    in_progress_count?: string;
    completed_count?: string;
    blocked_count?: string;
    available_count?: string;
    completed_today?: string;
  };
  error?: string;
}

// Gateway response format for GetAllAgentTaskStats.
interface GatewayAgentTaskStatsResponse {
  stats?: Array<{
    agent_id?: string;
    agent_name?: string;
    pending_count?: string;
    in_progress_count?: string;
    blocked_count?: string;
    completed_today?: string;
  }>;
  error?: string;
}

// Task list filter options.
export interface TaskListOptions {
  agentId?: number;
  listId?: string;
  status?: string;
  activeOnly?: boolean;
  availableOnly?: boolean;
  limit?: number;
  offset?: number;
}

// Build query string from filter options.
function buildQueryString(options: TaskListOptions): string {
  const params = new URLSearchParams();

  if (options.agentId !== undefined && options.agentId > 0) {
    params.set('agent_id', String(options.agentId));
  }
  if (options.listId !== undefined && options.listId !== '') {
    params.set('list_id', options.listId);
  }
  if (options.status !== undefined && options.status !== '') {
    params.set('status', displayToStatus(options.status));
  }
  if (options.activeOnly) {
    params.set('active_only', 'true');
  }
  if (options.availableOnly) {
    params.set('available_only', 'true');
  }
  if (options.limit !== undefined) {
    params.set('limit', String(options.limit));
  }
  if (options.offset !== undefined) {
    params.set('offset', String(options.offset));
  }

  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

// Parse timestamp to Date, returning null for empty or invalid strings.
function parseTimestamp(ts?: string): Date | null {
  if (!ts) return null;
  const date = new Date(ts);
  if (isNaN(date.getTime())) return null;
  return date;
}

// Parse gateway task response to Task.
function parseTask(t: GatewayTaskResponse['task']): Task {
  return {
    id: toNumber(t?.id),
    agent_id: toNumber(t?.agent_id),
    list_id: t?.list_id ?? '',
    claude_task_id: t?.claude_task_id ?? '',
    subject: t?.subject ?? '',
    description: t?.description ?? '',
    active_form: t?.active_form ?? '',
    metadata_json: t?.metadata_json ?? '',
    status: statusToDisplay(t?.status ?? 'TASK_STATUS_UNSPECIFIED'),
    owner: t?.owner ?? '',
    blocked_by: t?.blocked_by ?? [],
    blocks: t?.blocks ?? [],
    created_at: parseTimestamp(t?.created_at),
    updated_at: parseTimestamp(t?.updated_at),
    started_at: parseTimestamp(t?.started_at),
    completed_at: parseTimestamp(t?.completed_at),
  };
}

// Parse gateway task list response to TaskList.
function parseTaskList(tl: {
  id?: string;
  list_id?: string;
  agent_id?: string;
  watch_path?: string;
  created_at?: string;
  last_synced_at?: string;
}): TaskList {
  return {
    id: toNumber(tl.id),
    list_id: tl.list_id ?? '',
    agent_id: toNumber(tl.agent_id),
    watch_path: tl.watch_path ?? '',
    created_at: parseTimestamp(tl.created_at),
    last_synced_at: parseTimestamp(tl.last_synced_at),
  };
}

// Fetch tasks with optional filters.
export async function fetchTasks(
  options: TaskListOptions = {},
  signal?: AbortSignal,
): Promise<Task[]> {
  const query = buildQueryString(options);
  const response = await get<GatewayListTasksResponse>(`/tasks${query}`, signal);
  return (response.tasks ?? []).map((t) => parseTask(t));
}

// Fetch a single task by list ID and Claude task ID.
export async function fetchTask(
  listId: string,
  claudeTaskId: string,
  signal?: AbortSignal,
): Promise<Task> {
  const response = await get<GatewayTaskResponse>(
    `/tasks/${listId}/${claudeTaskId}`,
    signal,
  );
  return parseTask(response.task);
}

// Fetch task lists.
export async function fetchTaskLists(
  agentId?: number,
  signal?: AbortSignal,
): Promise<TaskList[]> {
  const query = agentId ? `?agent_id=${agentId}` : '';
  const response = await get<GatewayListTaskListsResponse>(
    `/task-lists${query}`,
    signal,
  );
  return (response.task_lists ?? []).map(parseTaskList);
}

// Fetch task statistics.
export async function fetchTaskStats(
  agentId?: number,
  listId?: string,
  signal?: AbortSignal,
): Promise<TaskStats> {
  const params = new URLSearchParams();
  if (agentId) params.set('agent_id', String(agentId));
  if (listId) params.set('list_id', listId);
  // Set today_since to start of today.
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  params.set('today_since', today.toISOString());

  const query = params.toString() ? `?${params.toString()}` : '';
  const response = await get<GatewayTaskStatsResponse>(
    `/tasks/stats${query}`,
    signal,
  );

  return {
    pending_count: toNumber(response.stats?.pending_count),
    in_progress_count: toNumber(response.stats?.in_progress_count),
    completed_count: toNumber(response.stats?.completed_count),
    blocked_count: toNumber(response.stats?.blocked_count),
    available_count: toNumber(response.stats?.available_count),
    completed_today: toNumber(response.stats?.completed_today),
  };
}

// Fetch task statistics grouped by agent.
export async function fetchAgentTaskStats(
  signal?: AbortSignal,
): Promise<AgentTaskStats[]> {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const response = await get<GatewayAgentTaskStatsResponse>(
    `/tasks/stats/by-agent?today_since=${today.toISOString()}`,
    signal,
  );

  return (response.stats ?? []).map((s) => ({
    agent_id: toNumber(s.agent_id),
    agent_name: s.agent_name ?? '',
    pending_count: toNumber(s.pending_count),
    in_progress_count: toNumber(s.in_progress_count),
    blocked_count: toNumber(s.blocked_count),
    completed_today: toNumber(s.completed_today),
  }));
}

// Update task status.
export async function updateTaskStatus(
  listId: string,
  claudeTaskId: string,
  status: string,
): Promise<void> {
  await patch<void>(`/tasks/${listId}/${claudeTaskId}/status`, {
    status: displayToStatus(status),
  });
}

// Update task owner.
export async function updateTaskOwner(
  listId: string,
  claudeTaskId: string,
  owner: string,
): Promise<void> {
  await patch<void>(`/tasks/${listId}/${claudeTaskId}/owner`, { owner });
}

// Sync a task list from files.
export async function syncTaskList(
  listId: string,
): Promise<{ tasks_updated: number; tasks_deleted: number }> {
  return post<{ tasks_updated: number; tasks_deleted: number }>(
    `/task-lists/${listId}/sync`,
    {},
  );
}
