// React hook for topic-related queries using TanStack Query.

import { useQuery } from '@tanstack/react-query';
import { fetchTopics, fetchTopic } from '@/api/topics.js';

// Query keys for topics.
export const topicKeys = {
  all: ['topics'] as const,
  lists: () => [...topicKeys.all, 'list'] as const,
  list: () => [...topicKeys.lists()] as const,
  details: () => [...topicKeys.all, 'detail'] as const,
  detail: (id: number) => [...topicKeys.details(), id] as const,
};

// Hook for fetching all topics.
export function useTopics() {
  return useQuery({
    queryKey: topicKeys.list(),
    queryFn: async ({ signal }) => {
      const response = await fetchTopics(signal);
      return response.data;
    },
    // Topics don't change often, use longer stale time.
    staleTime: 5 * 60 * 1000,
  });
}

// Hook for fetching a single topic.
export function useTopic(id: number, enabled = true) {
  return useQuery({
    queryKey: topicKeys.detail(id),
    queryFn: async ({ signal }) => {
      return fetchTopic(id, signal);
    },
    enabled,
  });
}

// Hook for getting topic count.
export function useTopicCount() {
  const query = useTopics();
  return {
    ...query,
    data: query.data?.length ?? 0,
  };
}
