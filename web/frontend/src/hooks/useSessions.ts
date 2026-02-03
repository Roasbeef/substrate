// React hook for session-related queries and mutations using TanStack Query.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchActiveSessions,
  fetchSessions,
  fetchSession,
  startSession,
  completeSession,
} from '@/api/sessions.js';
import type { Session, StartSessionRequest } from '@/types/api.js';

// Query keys for sessions.
export const sessionKeys = {
  all: ['sessions'] as const,
  lists: () => [...sessionKeys.all, 'list'] as const,
  listActive: () => [...sessionKeys.lists(), 'active'] as const,
  listAll: () => [...sessionKeys.lists(), 'all'] as const,
  details: () => [...sessionKeys.all, 'detail'] as const,
  detail: (id: number) => [...sessionKeys.details(), id] as const,
};

// Hook for fetching active sessions.
export function useActiveSessions() {
  return useQuery({
    queryKey: sessionKeys.listActive(),
    queryFn: async ({ signal }) => {
      const response = await fetchActiveSessions(signal);
      return response;
    },
    // Refetch every 30 seconds to keep sessions up to date.
    refetchInterval: 30000,
  });
}

// Hook for fetching all sessions.
export function useSessions() {
  return useQuery({
    queryKey: sessionKeys.listAll(),
    queryFn: async ({ signal }) => {
      const response = await fetchSessions(signal);
      return response;
    },
  });
}

// Hook for fetching a single session.
export function useSession(id: number, enabled = true) {
  return useQuery({
    queryKey: sessionKeys.detail(id),
    queryFn: async ({ signal }) => {
      return fetchSession(id, signal);
    },
    enabled,
  });
}

// Hook for starting a new session.
export function useStartSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: StartSessionRequest) => startSession(data),
    onSuccess: (newSession) => {
      // Add the new session to the cache.
      queryClient.setQueryData<{ data: Session[] }>(
        sessionKeys.listActive(),
        (old) => {
          if (!old) return { data: [newSession] };
          return { data: [...old.data, newSession] };
        },
      );

      // Invalidate lists to ensure fresh data.
      void queryClient.invalidateQueries({ queryKey: sessionKeys.lists() });
    },
  });
}

// Hook for completing a session.
export function useCompleteSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => completeSession(id),
    onMutate: async (id) => {
      // Cancel outgoing refetches.
      await queryClient.cancelQueries({ queryKey: sessionKeys.listActive() });

      // Snapshot previous value.
      const previousSessions = queryClient.getQueryData<{ data: Session[] }>(
        sessionKeys.listActive(),
      );

      // Optimistically remove the session from active list.
      queryClient.setQueryData<{ data: Session[] }>(
        sessionKeys.listActive(),
        (old) => {
          if (!old) return old;
          return {
            ...old,
            data: old.data.filter((s) => s.id !== id),
          };
        },
      );

      // Update the session detail if it exists.
      const previousSession = queryClient.getQueryData<Session>(
        sessionKeys.detail(id),
      );
      if (previousSession) {
        queryClient.setQueryData<Session>(sessionKeys.detail(id), {
          ...previousSession,
          status: 'completed',
          ended_at: new Date().toISOString(),
        });
      }

      return { previousSessions, previousSession };
    },
    onError: (_err, id, context) => {
      // Rollback on error.
      if (context?.previousSessions) {
        queryClient.setQueryData(
          sessionKeys.listActive(),
          context.previousSessions,
        );
      }
      if (context?.previousSession) {
        queryClient.setQueryData(
          sessionKeys.detail(id),
          context.previousSession,
        );
      }
    },
    onSettled: () => {
      // Invalidate lists to ensure fresh data.
      void queryClient.invalidateQueries({ queryKey: sessionKeys.lists() });
    },
  });
}

// Hook for filtering sessions by status.
export function useSessionsByStatus(status?: Session['status']) {
  const query = useSessions();

  const filteredSessions =
    status === undefined
      ? query.data?.data
      : query.data?.data.filter((s) => s.status === status);

  return {
    ...query,
    data: filteredSessions,
  };
}

// Hook for getting session count by status.
export function useSessionCounts() {
  const query = useSessions();

  const counts =
    query.data?.data.reduce(
      (acc, session) => {
        acc[session.status]++;
        return acc;
      },
      { active: 0, completed: 0, abandoned: 0 },
    ) ?? { active: 0, completed: 0, abandoned: 0 };

  return {
    ...query,
    data: counts,
  };
}

// Convenience hook that returns all session-related mutations.
export function useSessionMutations() {
  const startSessionMutation = useStartSession();
  const completeSessionMutation = useCompleteSession();

  return {
    startSession: startSessionMutation,
    completeSession: completeSessionMutation,
  };
}
