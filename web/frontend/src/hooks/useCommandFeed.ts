// TanStack Query hooks for the command center feed, including
// WebSocket-driven cache updates so lanes stay live without polling
// aggressively.

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef } from 'react';
import { getAgentFlow, getCommandFeed } from '@/api/command.js';
import type { CommandFeedResponse } from '@/api/command.js';
import {
  useAgentUpdates,
  useNewMessages,
  useTaskUpdates,
} from './useWebSocket.js';
import type { AgentUpdatePayload, NewMessagePayload } from './useWebSocket.js';

// Query keys for command feed data.
export const commandKeys = {
  all: ['command'] as const,
  feed: () => [...commandKeys.all, 'feed'] as const,
  flow: (agentId: number) =>
    [...commandKeys.all, 'flow', agentId] as const,
};

// Fetch the aggregated command feed. A modest refetch interval acts as
// a fallback; WebSocket events trigger immediate refreshes.
export function useCommandFeed() {
  return useQuery({
    queryKey: commandKeys.feed(),
    queryFn: getCommandFeed,
    refetchInterval: 30_000,
    staleTime: 5_000,
  });
}

// Fetch the high-granularity transcript flow for one agent. Enabled
// only when a card's granularity dial is set to HI so idle cards cost
// nothing.
export function useAgentFlow(agentId: number, enabled: boolean) {
  return useQuery({
    queryKey: commandKeys.flow(agentId),
    queryFn: () => getAgentFlow(agentId),
    enabled,
    refetchInterval: 20_000,
    staleTime: 10_000,
  });
}

// Wire WebSocket events into the feed cache: new messages refetch the
// feed (debounced), agent status ticks patch lane headers in place.
export function useCommandFeedRealtime(
  onActionable?: (payload: NewMessagePayload) => void,
): void {
  const queryClient = useQueryClient();
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const invalidateFeed = useCallback(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }
    debounceRef.current = setTimeout(() => {
      void queryClient.invalidateQueries({
        queryKey: commandKeys.feed(),
      });
    }, 400);
  }, [queryClient]);

  useNewMessages(
    useCallback(
      (payload: NewMessagePayload) => {
        invalidateFeed();

        // Surface actionable arrivals (urgent or interrogative)
        // to the caller for a lightweight toast.
        const subject = payload.subject ?? '';
        const actionable =
          payload.priority === 'urgent' ||
          subject.trim().endsWith('?') ||
          subject.startsWith('[PLAN]');
        if (actionable && onActionable) {
          onActionable(payload);
        }
      },
      [invalidateFeed, onActionable],
    ),
  );

  useTaskUpdates(useCallback(() => invalidateFeed(), [invalidateFeed]));

  useAgentUpdates(
    useCallback(
      (payload: AgentUpdatePayload) => {
        // Patch lane agent statuses in place without a refetch.
        queryClient.setQueryData<CommandFeedResponse>(
          commandKeys.feed(),
          (prev) => {
            if (!prev) {
              return prev;
            }

            const byId = new Map(
              payload.agents.map((a) => [a.id, a]),
            );

            return {
              ...prev,
              lanes: prev.lanes.map((lane) => {
                const update = byId.get(lane.agent.id);
                if (!update) {
                  return lane;
                }
                return {
                  ...lane,
                  agent: {
                    ...lane.agent,
                    status: update.status,
                    last_active_at: update.last_active_at,
                    seconds_since_heartbeat:
                      update.seconds_since_heartbeat,
                  },
                };
              }),
            };
          },
        );
      },
      [queryClient],
    ),
  );
}
