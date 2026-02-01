// React hook for activity-related queries using TanStack Query.

import { useQuery, useInfiniteQuery } from '@tanstack/react-query';
import {
  fetchActivities,
  fetchAgentActivities,
  type ActivitiesQueryOptions,
  type ActivitiesResponse,
} from '@/api/activities.js';

// Query keys for activities.
export const activityKeys = {
  all: ['activities'] as const,
  list: (options?: ActivitiesQueryOptions) =>
    [...activityKeys.all, 'list', options ?? {}] as const,
  agent: (agentId: number, options?: Omit<ActivitiesQueryOptions, 'agent_id'>) =>
    [...activityKeys.all, 'agent', agentId, options ?? {}] as const,
};

// Default page size for activities.
const DEFAULT_PAGE_SIZE = 20;

// Hook for fetching paginated activities.
export function useActivities(options: ActivitiesQueryOptions = {}) {
  return useQuery({
    queryKey: activityKeys.list(options),
    queryFn: async ({ signal }) => {
      return fetchActivities(options, signal);
    },
  });
}

// Hook for infinite scrolling activities.
export function useInfiniteActivities(
  options: Omit<ActivitiesQueryOptions, 'page'> = {},
) {
  const pageSize = options.page_size ?? DEFAULT_PAGE_SIZE;

  return useInfiniteQuery({
    queryKey: activityKeys.list({ ...options, infinite: true } as ActivitiesQueryOptions),
    queryFn: async ({ pageParam, signal }) => {
      return fetchActivities({ ...options, page: pageParam, page_size: pageSize }, signal);
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage, _allPages, lastPageParam) => {
      // Check if there are more pages.
      const totalPages = Math.ceil(lastPage.meta.total / pageSize);
      if (lastPageParam < totalPages) {
        return lastPageParam + 1;
      }
      return undefined;
    },
  });
}

// Hook for fetching activities for a specific agent.
export function useAgentActivities(
  agentId: number,
  options: Omit<ActivitiesQueryOptions, 'agent_id'> = {},
  enabled = true,
) {
  return useQuery({
    queryKey: activityKeys.agent(agentId, options),
    queryFn: async ({ signal }) => {
      return fetchAgentActivities(agentId, options, signal);
    },
    enabled,
  });
}

// Hook for infinite scrolling agent activities.
export function useInfiniteAgentActivities(
  agentId: number,
  options: Omit<ActivitiesQueryOptions, 'agent_id' | 'page'> = {},
  enabled = true,
) {
  const pageSize = options.page_size ?? DEFAULT_PAGE_SIZE;

  return useInfiniteQuery({
    queryKey: activityKeys.agent(agentId, { ...options, infinite: true } as Omit<ActivitiesQueryOptions, 'agent_id'>),
    queryFn: async ({ pageParam, signal }) => {
      return fetchAgentActivities(
        agentId,
        { ...options, page: pageParam, page_size: pageSize },
        signal,
      );
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage, _allPages, lastPageParam) => {
      const totalPages = Math.ceil(lastPage.meta.total / pageSize);
      if (lastPageParam < totalPages) {
        return lastPageParam + 1;
      }
      return undefined;
    },
    enabled,
  });
}

// Utility to flatten paginated activities.
export function flattenActivities(
  data: { pages: ActivitiesResponse[] } | undefined,
) {
  if (!data) return [];
  return data.pages.flatMap((page) => page.data);
}
