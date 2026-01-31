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
  const handleAgentUpdate = useCallback(
    (payload: AgentUpdatePayload) => {
      // Update the agents status cache directly.
      queryClient.setQueryData<AgentsStatusResponse>(
        agentKeys.status(),
        () => {
          // Map the payload to the expected format.
          return {
            agents: payload.agents.map((a) => ({
              id: a.id,
              name: a.name,
              status: a.status as AgentsStatusResponse['agents'][0]['status'],
              last_active_at: a.last_active_at,
              seconds_since_heartbeat: a.seconds_since_heartbeat,
            })),
            counts: payload.counts,
          };
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
