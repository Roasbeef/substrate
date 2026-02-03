// API functions for dashboard statistics.
// Uses grpc-gateway REST API directly.

import { get } from './client.js';
import type { DashboardStats, HealthResponse } from '@/types/api.js';

// Gateway response formats.
interface GatewayDashboardStatsResponse {
  stats?: {
    active_agents?: number;
    running_sessions?: number;
    pending_messages?: number;
    completed_today?: number;
  };
}

interface GatewayHealthResponse {
  status: string;
  time?: string;
}

// Fetch dashboard statistics.
export async function fetchDashboardStats(
  signal?: AbortSignal,
): Promise<DashboardStats> {
  const response = await get<GatewayDashboardStatsResponse>('/stats/dashboard', signal);
  return {
    active_agents: response.stats?.active_agents ?? 0,
    running_sessions: response.stats?.running_sessions ?? 0,
    pending_messages: response.stats?.pending_messages ?? 0,
    completed_today: response.stats?.completed_today ?? 0,
  };
}

// Health check.
export async function healthCheck(signal?: AbortSignal): Promise<HealthResponse> {
  const response = await get<GatewayHealthResponse>('/health', signal);
  return {
    status: response.status === 'ok' ? 'ok' : 'error',
    time: response.time ?? new Date().toISOString(),
  };
}
