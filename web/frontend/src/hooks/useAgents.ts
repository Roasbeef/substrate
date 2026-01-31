// React hook for agent-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchAgentsStatus,
  fetchAgent,
  createAgent,
  updateAgent,
  sendHeartbeat,
} from '@/api/agents.js';
import type {
  Agent,
  AgentWithStatus,
  AgentsStatusResponse,
  CreateAgentRequest,
  HeartbeatRequest,
} from '@/types/api.js';

// Query keys for agents.
export const agentKeys = {
  all: ['agents'] as const,
  status: () => [...agentKeys.all, 'status'] as const,
  details: () => [...agentKeys.all, 'detail'] as const,
  detail: (id: number) => [...agentKeys.details(), id] as const,
};

// Hook for fetching all agents with their status.
export function useAgentsStatus() {
  return useQuery({
    queryKey: agentKeys.status(),
    queryFn: async ({ signal }) => {
      return fetchAgentsStatus(signal);
    },
    // Refetch every 30 seconds to keep status up to date.
    refetchInterval: 30000,
  });
}

// Hook for fetching a single agent.
export function useAgent(id: number, enabled = true) {
  return useQuery({
    queryKey: agentKeys.detail(id),
    queryFn: async ({ signal }) => {
      return fetchAgent(id, signal);
    },
    enabled,
  });
}

// Hook for getting filtered agents by status.
export function useAgentsByStatus(status?: AgentWithStatus['status']) {
  const query = useAgentsStatus();

  // Filter agents by status if provided.
  const filteredAgents =
    status === undefined
      ? query.data?.agents
      : query.data?.agents.filter((a) => a.status === status);

  return {
    ...query,
    data: filteredAgents,
    counts: query.data?.counts,
  };
}

// Hook for creating a new agent.
export function useCreateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAgentRequest) => createAgent(data),
    onSuccess: (newAgent) => {
      // Add the new agent to the status query data.
      queryClient.setQueryData<AgentsStatusResponse>(
        agentKeys.status(),
        (old) => {
          if (!old) return old;

          const agentWithStatus: AgentWithStatus = {
            id: newAgent.id,
            name: newAgent.name,
            status: 'active',
            last_active_at: newAgent.created_at,
            seconds_since_heartbeat: 0,
          };

          return {
            ...old,
            agents: [...old.agents, agentWithStatus],
            counts: {
              ...old.counts,
              active: old.counts.active + 1,
            },
          };
        },
      );

      // Invalidate to ensure fresh data.
      void queryClient.invalidateQueries({ queryKey: agentKeys.status() });
    },
  });
}

// Hook for updating an agent.
export function useUpdateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<CreateAgentRequest> }) =>
      updateAgent(id, data),
    onSuccess: (updatedAgent, { id }) => {
      // Update the agent in the cache.
      queryClient.setQueryData<Agent>(agentKeys.detail(id), updatedAgent);

      // Update the agent in the status list.
      queryClient.setQueryData<AgentsStatusResponse>(
        agentKeys.status(),
        (old) => {
          if (!old) return old;

          return {
            ...old,
            agents: old.agents.map((a) =>
              a.id === id ? { ...a, name: updatedAgent.name } : a,
            ),
          };
        },
      );
    },
  });
}

// Hook for sending a heartbeat.
export function useHeartbeat() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: HeartbeatRequest) => sendHeartbeat(data),
    onSuccess: () => {
      // Invalidate status to refresh agent statuses.
      void queryClient.invalidateQueries({ queryKey: agentKeys.status() });
    },
  });
}

// Convenience hook that returns all agent-related mutations.
export function useAgentMutations() {
  const createAgentMutation = useCreateAgent();
  const updateAgentMutation = useUpdateAgent();
  const heartbeatMutation = useHeartbeat();

  return {
    createAgent: createAgentMutation,
    updateAgent: updateAgentMutation,
    heartbeat: heartbeatMutation,
  };
}

// Hook to get agent status counts.
export function useAgentCounts() {
  const query = useAgentsStatus();
  return {
    ...query,
    data: query.data?.counts,
  };
}

// Hook to get the total number of agents.
export function useAgentCount() {
  const query = useAgentsStatus();
  const total = query.data
    ? query.data.counts.active +
      query.data.counts.busy +
      query.data.counts.idle +
      query.data.counts.offline
    : 0;

  return {
    ...query,
    data: total,
  };
}
