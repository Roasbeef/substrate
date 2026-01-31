// API functions for dashboard statistics.

import { get, unwrapResponse } from './client.js';
import type { APIResponse, DashboardStats, HealthResponse } from '@/types/api.js';

// Fetch dashboard statistics.
export async function fetchDashboardStats(
  signal?: AbortSignal,
): Promise<DashboardStats> {
  const response = await get<APIResponse<DashboardStats>>(
    '/stats/dashboard',
    signal,
  );
  return unwrapResponse(response);
}

// Health check.
export function healthCheck(signal?: AbortSignal): Promise<HealthResponse> {
  return get<HealthResponse>('/health', signal);
}
