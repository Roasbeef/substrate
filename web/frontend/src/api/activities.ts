// API functions for activities.
// Uses grpc-gateway REST API directly.

import { get } from './client.js';
import type { Activity, ActivityType, APIMeta } from '@/types/api.js';

// Response type for activities list.
export interface ActivitiesResponse {
  data: Activity[];
  meta: APIMeta;
}

// Query options for activities.
export interface ActivitiesQueryOptions {
  agent_id?: number;
  type?: string;
  page?: number;
  page_size?: number;
}

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize activity type enum from proto format.
function normalizeActivityType(type?: string): ActivityType {
  if (!type) return 'heartbeat';
  const normalized = type.toLowerCase().replace('activity_type_', '');
  const validTypes: ActivityType[] = [
    'message_sent', 'message_read', 'session_started',
    'session_completed', 'agent_registered', 'heartbeat'
  ];
  return validTypes.includes(normalized as ActivityType) ? normalized as ActivityType : 'heartbeat';
}

// Gateway response format.
interface GatewayActivitiesResponse {
  activities?: Array<{
    id: string;
    agent_id: string;
    agent_name: string;
    type?: string;
    description: string;
    created_at?: string;
    metadata_json?: string;
  }>;
  total?: string;
  page?: number;
  page_size?: number;
}

// Build query string from options.
function buildQueryString(options: ActivitiesQueryOptions): string {
  const params = new URLSearchParams();

  if (options.agent_id !== undefined) {
    params.set('agent_id', String(options.agent_id));
  }
  if (options.type !== undefined) {
    params.set('type', options.type);
  }
  if (options.page !== undefined) {
    params.set('page', String(options.page));
  }
  if (options.page_size !== undefined) {
    params.set('page_size', String(options.page_size));
  }

  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

// Parse gateway response to internal format.
function parseActivitiesResponse(response: GatewayActivitiesResponse): ActivitiesResponse {
  const activities = (response.activities ?? []).map((activity): Activity => ({
    id: toNumber(activity.id),
    agent_id: toNumber(activity.agent_id),
    agent_name: activity.agent_name,
    type: normalizeActivityType(activity.type),
    description: activity.description,
    created_at: activity.created_at ?? new Date().toISOString(),
    metadata: activity.metadata_json ? JSON.parse(activity.metadata_json) : undefined,
  }));
  return {
    data: activities,
    meta: {
      total: toNumber(response.total) || activities.length,
      page: response.page ?? 1,
      page_size: response.page_size ?? activities.length,
    },
  };
}

// Fetch activities list.
export async function fetchActivities(
  options: ActivitiesQueryOptions = {},
  signal?: AbortSignal,
): Promise<ActivitiesResponse> {
  const queryString = buildQueryString(options);
  const response = await get<GatewayActivitiesResponse>(`/activities${queryString}`, signal);
  return parseActivitiesResponse(response);
}

// Fetch activities for a specific agent.
export async function fetchAgentActivities(
  agentId: number,
  options: Omit<ActivitiesQueryOptions, 'agent_id'> = {},
  signal?: AbortSignal,
): Promise<ActivitiesResponse> {
  return fetchActivities({ ...options, agent_id: agentId }, signal);
}
