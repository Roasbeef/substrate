// React hooks for agent summary queries using TanStack Query.

import { useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  fetchAllSummaries,
  fetchAgentSummary,
  fetchSummaryHistory,
} from '@/api/summaries.js';
import { useSummaryUpdates } from './useWebSocket.js';
import type { AgentSummary, AgentSummaryHistory } from '@/types/api.js';

// Query keys for summaries.
export const summaryKeys = {
  all: ['summaries'] as const,
  list: () => [...summaryKeys.all, 'list'] as const,
  detail: (agentId: number) => [...summaryKeys.all, 'detail', agentId] as const,
  history: (agentId: number) =>
    [...summaryKeys.all, 'history', agentId] as const,
};

// Shared stale time — summaries are refreshed server-side on a 3-minute
// cycle, so treat cached data as fresh for 2 minutes to avoid redundant
// refetches that trigger more summary generation.
const SUMMARY_STALE_TIME = 120_000;

// Hook for fetching all active agent summaries.
export function useAgentSummaries(activeOnly = true) {
  return useQuery<AgentSummary[]>({
    queryKey: [...summaryKeys.list(), activeOnly],
    queryFn: async ({ signal }) => {
      return fetchAllSummaries(activeOnly, signal);
    },
    staleTime: SUMMARY_STALE_TIME,
    refetchInterval: 120_000,
  });
}

// Hook for fetching a single agent's summary.
export function useAgentSummary(agentId: number, enabled = true) {
  return useQuery<AgentSummary | null>({
    queryKey: summaryKeys.detail(agentId),
    queryFn: async ({ signal }) => {
      return fetchAgentSummary(agentId, signal);
    },
    staleTime: SUMMARY_STALE_TIME,
    refetchInterval: 120_000,
    enabled,
  });
}

// Hook for fetching an agent's summary history.
export function useSummaryHistory(
  agentId: number,
  limit = 50,
  enabled = true,
) {
  return useQuery<AgentSummaryHistory[]>({
    queryKey: [...summaryKeys.history(agentId), limit],
    queryFn: async ({ signal }) => {
      return fetchSummaryHistory(agentId, limit, signal);
    },
    staleTime: SUMMARY_STALE_TIME,
    enabled,
  });
}

// Hook that listens for WebSocket summary_updated events and
// directly updates the query cache from the WS payload. This avoids
// triggering refetches (which would hit the API, trigger more summary
// generation, broadcast more WS events, and create an infinite loop).
export function useSummaryRealtime(): void {
  const queryClient = useQueryClient();

  const handleUpdate = useCallback(
    (payload: { agent_id: number; summary: string; delta: string }) => {
      // Update the list cache by merging the new summary data into
      // the matching entry. No refetch needed.
      queryClient.setQueriesData<AgentSummary[]>(
        { queryKey: summaryKeys.list() },
        (old) => {
          if (!old) return old;
          return old.map((s) =>
            s.agent_id === payload.agent_id
              ? {
                  ...s,
                  summary: payload.summary,
                  delta: payload.delta,
                  is_stale: false,
                  generated_at: new Date().toISOString(),
                }
              : s,
          );
        },
      );

      // Update the individual detail cache if it exists.
      queryClient.setQueryData<AgentSummary | null>(
        summaryKeys.detail(payload.agent_id),
        (old) => {
          if (!old) return old;
          return {
            ...old,
            summary: payload.summary,
            delta: payload.delta,
            is_stale: false,
            generated_at: new Date().toISOString(),
          };
        },
      );

      // Only invalidate the history query since it needs a full
      // refetch to pick up the new entry. Invalidation is safe here
      // because history queries don't trigger summary generation.
      void queryClient.invalidateQueries({
        queryKey: summaryKeys.history(payload.agent_id),
      });
    },
    [queryClient],
  );

  useSummaryUpdates(handleUpdate);
}
