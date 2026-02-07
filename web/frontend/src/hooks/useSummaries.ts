// React hooks for agent summary queries using TanStack Query.

import { useQuery } from '@tanstack/react-query';
import {
  fetchAllSummaries,
  fetchAgentSummary,
  fetchSummaryHistory,
} from '@/api/summaries.js';
import type { AgentSummary, AgentSummaryHistory } from '@/types/api.js';

// Query keys for summaries.
export const summaryKeys = {
  all: ['summaries'] as const,
  list: () => [...summaryKeys.all, 'list'] as const,
  detail: (agentId: number) => [...summaryKeys.all, 'detail', agentId] as const,
  history: (agentId: number) =>
    [...summaryKeys.all, 'history', agentId] as const,
};

// Hook for fetching all active agent summaries.
export function useAgentSummaries(activeOnly = true) {
  return useQuery<AgentSummary[]>({
    queryKey: [...summaryKeys.list(), activeOnly],
    queryFn: async ({ signal }) => {
      return fetchAllSummaries(activeOnly, signal);
    },
    refetchInterval: 45_000,
  });
}

// Hook for fetching a single agent's summary.
export function useAgentSummary(agentId: number, enabled = true) {
  return useQuery<AgentSummary | null>({
    queryKey: summaryKeys.detail(agentId),
    queryFn: async ({ signal }) => {
      return fetchAgentSummary(agentId, signal);
    },
    refetchInterval: 30_000,
    enabled,
  });
}

// Hook for fetching an agent's summary history.
export function useSummaryHistory(
  agentId: number,
  limit = 20,
  enabled = true,
) {
  return useQuery<AgentSummaryHistory[]>({
    queryKey: [...summaryKeys.history(agentId), limit],
    queryFn: async ({ signal }) => {
      return fetchSummaryHistory(agentId, limit, signal);
    },
    enabled,
  });
}
