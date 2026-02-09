// React hooks for task-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchTasks,
  fetchTask,
  fetchTaskLists,
  fetchTaskStats,
  fetchAgentTaskStats,
  updateTaskStatus,
  updateTaskOwner,
  syncTaskList,
  type TaskListOptions,
} from '@/api/tasks.js';

// Query keys for tasks.
export const taskKeys = {
  all: ['tasks'] as const,
  lists: () => [...taskKeys.all, 'list'] as const,
  list: (options: TaskListOptions) => [...taskKeys.lists(), options] as const,
  details: () => [...taskKeys.all, 'detail'] as const,
  detail: (listId: string, claudeTaskId: string) =>
    [...taskKeys.details(), listId, claudeTaskId] as const,
  taskLists: () => [...taskKeys.all, 'taskLists'] as const,
  taskListsByAgent: (agentId?: number) =>
    [...taskKeys.taskLists(), agentId] as const,
  stats: () => [...taskKeys.all, 'stats'] as const,
  statsByAgent: (agentId?: number) => [...taskKeys.stats(), 'agent', agentId] as const,
  statsByList: (listId?: string) => [...taskKeys.stats(), 'list', listId] as const,
  agentStats: () => [...taskKeys.stats(), 'byAgent'] as const,
};

// Hook for fetching a list of tasks.
export function useTasks(options: TaskListOptions = {}) {
  return useQuery({
    queryKey: taskKeys.list(options),
    queryFn: async ({ signal }) => {
      return fetchTasks(options, signal);
    },
    // Refetch every 30 seconds to keep task list fresh.
    refetchInterval: 30000,
  });
}

// Hook for fetching a single task.
export function useTask(listId: string, claudeTaskId: string, enabled = true) {
  return useQuery({
    queryKey: taskKeys.detail(listId, claudeTaskId),
    queryFn: async ({ signal }) => {
      return fetchTask(listId, claudeTaskId, signal);
    },
    enabled: enabled && listId !== '' && claudeTaskId !== '',
  });
}

// Hook for fetching task lists.
export function useTaskLists(agentId?: number) {
  return useQuery({
    queryKey: taskKeys.taskListsByAgent(agentId),
    queryFn: async ({ signal }) => {
      return fetchTaskLists(agentId, signal);
    },
  });
}

// Hook for fetching task statistics.
export function useTaskStats(agentId?: number, listId?: string) {
  return useQuery({
    queryKey: agentId
      ? taskKeys.statsByAgent(agentId)
      : listId
        ? taskKeys.statsByList(listId)
        : taskKeys.stats(),
    queryFn: async ({ signal }) => {
      return fetchTaskStats(agentId, listId, signal);
    },
    // Refetch every 30 seconds.
    refetchInterval: 30000,
  });
}

// Hook for fetching task statistics grouped by agent.
export function useAgentTaskStats() {
  return useQuery({
    queryKey: taskKeys.agentStats(),
    queryFn: async ({ signal }) => {
      return fetchAgentTaskStats(signal);
    },
    refetchInterval: 30000,
  });
}

// Hook for updating task status.
export function useUpdateTaskStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      listId,
      claudeTaskId,
      status,
    }: {
      listId: string;
      claudeTaskId: string;
      status: string;
    }) => updateTaskStatus(listId, claudeTaskId, status),
    onSuccess: (_data, { listId, claudeTaskId }) => {
      void queryClient.invalidateQueries({
        queryKey: taskKeys.detail(listId, claudeTaskId),
      });
      void queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: taskKeys.stats() });
    },
  });
}

// Hook for updating task owner.
export function useUpdateTaskOwner() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      listId,
      claudeTaskId,
      owner,
    }: {
      listId: string;
      claudeTaskId: string;
      owner: string;
    }) => updateTaskOwner(listId, claudeTaskId, owner),
    onSuccess: (_data, { listId, claudeTaskId }) => {
      void queryClient.invalidateQueries({
        queryKey: taskKeys.detail(listId, claudeTaskId),
      });
      void queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
    },
  });
}

// Hook for syncing a task list.
export function useSyncTaskList() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (listId: string) => syncTaskList(listId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: taskKeys.taskLists() });
      void queryClient.invalidateQueries({ queryKey: taskKeys.stats() });
    },
  });
}
