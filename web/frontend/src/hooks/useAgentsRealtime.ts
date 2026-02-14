// Hook for real-time agent updates via WebSocket.

import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useAgentUpdates,
  useWebSocketConnection,
  type AgentUpdatePayload,
} from '@/hooks/useWebSocket.js';
import { agentKeys } from '@/hooks/useAgents.js';
import type { AgentsStatusResponse } from '@/types/api.js';

// Interface for real-time agent state.
export interface RealtimeAgentState {
  isConnected: boolean;
  connect: () => void;
  disconnect: () => void;
}

// Hook to enable real-time updates for agents.
export function useAgentsRealtime(): RealtimeAgentState {
  const queryClient = useQueryClient();
  const { state, connect, disconnect } = useWebSocketConnection();
  const isConnected = state === 'connected';

  // Handle agent update received via WebSocket.
  // Merges WS payload into existing cache to preserve fields not
  // included in the lightweight WS broadcast (project_key, git_branch,
  // session_id). This prevents flicker from incomplete data replacement.
  const handleAgentUpdate = useCallback(
    (payload: AgentUpdatePayload) => {
      queryClient.setQueryData<AgentsStatusResponse>(
        agentKeys.status(),
        (prev) => {
          const existingMap = new Map(
            (prev?.agents ?? []).map((a) => [a.id, a]),
          );

          const merged = payload.agents.map((a) => {
            const existing = existingMap.get(a.id);
            return {
              ...existing,
              id: a.id,
              name: a.name,
              status: a.status as AgentsStatusResponse['agents'][0]['status'],
              last_active_at: a.last_active_at,
              seconds_since_heartbeat: a.seconds_since_heartbeat,
            };
          });

          return { agents: merged, counts: payload.counts };
        },
      );
    },
    [queryClient],
  );

  // Subscribe to agent update events.
  useAgentUpdates(handleAgentUpdate);

  return {
    isConnected,
    connect,
    disconnect,
  };
}
