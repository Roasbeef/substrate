// API functions for topic-related operations.

import { get } from './client.js';
import type { APIResponse, Topic } from '@/types/api.js';

// Fetch all topics.
export function fetchTopics(
  signal?: AbortSignal,
): Promise<APIResponse<Topic[]>> {
  return get<APIResponse<Topic[]>>('/topics', signal);
}

// Fetch a single topic by ID.
export function fetchTopic(id: number, signal?: AbortSignal): Promise<Topic> {
  return get<Topic>(`/topics/${id}`, signal);
}
