// API functions for agent summary operations.

import { get } from './client.js';
import type { AgentSummary, AgentSummaryHistory } from '@/types/api.js';

// Response shape from the backend.
interface SummariesResponse {
  summaries: AgentSummary[];
}

interface SummaryResponse {
  summary: AgentSummary | null;
}

interface HistoryResponse {
  history: AgentSummaryHistory[];
}

// Fetch summaries for all active agents.
export async function fetchAllSummaries(
  activeOnly = true,
  signal?: AbortSignal,
): Promise<AgentSummary[]> {
  const params = activeOnly ? '?active_only=true' : '';
  const response = await get<SummariesResponse>(
    `/agents/summaries${params}`,
    signal,
  );
  return response.summaries ?? [];
}

// Fetch the current summary for a specific agent.
export async function fetchAgentSummary(
  agentId: number,
  signal?: AbortSignal,
): Promise<AgentSummary | null> {
  const response = await get<SummaryResponse>(
    `/agents/summary/${agentId}`,
    signal,
  );
  return response.summary ?? null;
}

// Fetch summary history for a specific agent.
export async function fetchSummaryHistory(
  agentId: number,
  limit = 20,
  signal?: AbortSignal,
): Promise<AgentSummaryHistory[]> {
  const response = await get<HistoryResponse>(
    `/agents/summary/${agentId}/history?limit=${limit}`,
    signal,
  );
  return response.history ?? [];
}
