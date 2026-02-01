// API functions for activities.

import { get } from './client.js';
import type { Activity, APIMeta } from '@/types/api.js';

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

// Fetch activities list.
export async function fetchActivities(
  options: ActivitiesQueryOptions = {},
  signal?: AbortSignal,
): Promise<ActivitiesResponse> {
  const queryString = buildQueryString(options);
  return get<ActivitiesResponse>(`/activities${queryString}`, signal);
}

// Fetch activities for a specific agent.
export async function fetchAgentActivities(
  agentId: number,
  options: Omit<ActivitiesQueryOptions, 'agent_id'> = {},
  signal?: AbortSignal,
): Promise<ActivitiesResponse> {
  return fetchActivities({ ...options, agent_id: agentId }, signal);
}
