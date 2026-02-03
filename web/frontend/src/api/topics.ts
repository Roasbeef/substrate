// API functions for topic-related operations.
// Uses grpc-gateway REST API directly.

import { get } from './client.js';
import type { APIResponse, Topic } from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Gateway response formats.
interface GatewayTopic {
  id: string;
  name: string;
  topic_type?: string;
  created_at?: string;
  message_count?: string;
}

interface GatewayTopicsResponse {
  topics?: GatewayTopic[];
}

interface GatewayTopicResponse {
  topic?: GatewayTopic;
}

// Parse gateway topic to internal format.
function parseTopic(topic: GatewayTopic): Topic {
  const result: Topic = {
    id: toNumber(topic.id),
    name: topic.name,
    created_at: topic.created_at ?? new Date().toISOString(),
    message_count: toNumber(topic.message_count),
  };
  if (topic.topic_type !== undefined) {
    result.description = topic.topic_type;
  }
  return result;
}

// Fetch all topics.
export async function fetchTopics(
  signal?: AbortSignal,
): Promise<APIResponse<Topic[]>> {
  const response = await get<GatewayTopicsResponse>('/topics', signal);
  const topics = (response.topics ?? []).map(parseTopic);
  return {
    data: topics,
    meta: { total: topics.length, page: 1, page_size: topics.length },
  };
}

// Fetch a single topic by ID.
export async function fetchTopic(
  id: number,
  signal?: AbortSignal,
): Promise<Topic> {
  const response = await get<GatewayTopicResponse>(`/topics/${id}`, signal);
  if (!response.topic) {
    throw new Error('Topic not found');
  }
  return parseTopic(response.topic);
}
