// Hook for real-time activity updates via WebSocket.

import { useCallback, useRef, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  useActivityUpdates,
  useWebSocketConnection,
  type ActivityPayload,
} from '@/hooks/useWebSocket.js';
import { activityKeys } from '@/hooks/useActivities.js';
import type { Activity, APIResponse } from '@/types/api.js';

// Type alias for activity list response.
type ActivitiesResponse = APIResponse<Activity[]>;

// Interface for real-time activity state.
export interface RealtimeActivityState {
  isConnected: boolean;
  connect: () => void;
  disconnect: () => void;
}

// Hook to enable real-time updates for activities.
export function useActivitiesRealtime(): RealtimeActivityState {
  const queryClient = useQueryClient();
  const { state, connect, disconnect } = useWebSocketConnection();
  const isConnected = state === 'connected';

  // Track processed activity IDs to prevent duplicates.
  const processedIds = useRef(new Set<number>());

  // Handle activity update received via WebSocket.
  const handleActivityUpdate = useCallback(
    (payload: ActivityPayload[]) => {
      // Filter out activities we've already processed.
      const newActivities = payload.filter((a) => !processedIds.current.has(a.id));

      if (newActivities.length === 0) {
        return;
      }

      // Mark as processed.
      newActivities.forEach((a) => processedIds.current.add(a.id));

      // Clean up old IDs periodically (keep last 200).
      if (processedIds.current.size > 200) {
        const ids = Array.from(processedIds.current);
        processedIds.current = new Set(ids.slice(-100));
      }

      // Convert WebSocket payload to Activity format.
      const activities: Activity[] = newActivities.map((a) => ({
        id: a.id,
        agent_id: a.agent_id,
        agent_name: a.agent_name,
        type: a.type as Activity['type'],
        description: a.description,
        created_at: a.created_at,
      }));

      // Update all activity list queries by prepending new activities.
      queryClient.setQueriesData<ActivitiesResponse>(
        { queryKey: activityKeys.all },
        (oldData) => {
          if (!oldData) return oldData;

          // Filter out any activities that already exist.
          const existingIds = new Set(oldData.data.map((a) => a.id));
          const trulyNew = activities.filter((a) => !existingIds.has(a.id));

          if (trulyNew.length === 0) return oldData;

          return {
            ...oldData,
            data: [...trulyNew, ...oldData.data],
            ...(oldData.meta && {
              meta: {
                ...oldData.meta,
                total: oldData.meta.total + trulyNew.length,
              },
            }),
          };
        },
      );

      // Also invalidate to ensure eventual consistency.
      void queryClient.invalidateQueries({
        queryKey: activityKeys.all,
        exact: false,
        refetchType: 'none',
      });
    },
    [queryClient],
  );

  // Subscribe to activity update events.
  useActivityUpdates(handleActivityUpdate);

  // Clean up processed IDs on unmount.
  useEffect(() => {
    return () => {
      processedIds.current.clear();
    };
  }, []);

  return {
    isConnected,
    connect,
    disconnect,
  };
}
