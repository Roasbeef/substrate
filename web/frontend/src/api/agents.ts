// API functions for agent-related operations.
// Uses grpc-gateway REST API directly.

import { get, post, patch } from './client.js';
import type {
  Agent,
  AgentWithStatus,
  AgentsStatusResponse,
  AgentStatusType,
  CreateAgentRequest,
  HeartbeatRequest,
} from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize agent status enum from proto format.
function normalizeStatus(status?: string): AgentStatusType {
  if (!status) return 'offline';
  const normalized = status.toLowerCase().replace('agent_status_', '');
  if (normalized === 'active' || normalized === 'busy' || normalized === 'idle' || normalized === 'offline') {
    return normalized;
  }
  return 'offline';
}

// Gateway response format.
interface GatewayAgentsStatusResponse {
  agents?: Array<{
    id: string;
    name: string;
    project_key?: string;
    git_branch?: string;
    status?: string;
    last_active_at?: string;
    session_id?: string;
    seconds_since_heartbeat?: string;
  }>;
  counts?: { active: number; busy: number; idle: number; offline: number };
}

// Parse gateway response to internal format.
function parseAgentsStatusResponse(response: GatewayAgentsStatusResponse): AgentsStatusResponse {
  const agents = (response.agents ?? []).map((agent): AgentWithStatus => {
    const result: AgentWithStatus = {
      id: toNumber(agent.id),
      name: agent.name,
      status: normalizeStatus(agent.status),
      last_active_at: agent.last_active_at ?? new Date().toISOString(),
      seconds_since_heartbeat: toNumber(agent.seconds_since_heartbeat),
    };
    if (agent.project_key !== undefined) {
      result.project_key = agent.project_key;
    }
    if (agent.git_branch !== undefined) {
      result.git_branch = agent.git_branch;
    }
    if (agent.session_id !== undefined) {
      result.session_id = toNumber(agent.session_id);
    }
    return result;
  });
  return {
    agents,
    counts: response.counts ?? { active: 0, busy: 0, idle: 0, offline: 0 },
  };
}

// Fetch all agents with their current status.
export async function fetchAgentsStatus(
  signal?: AbortSignal,
): Promise<AgentsStatusResponse> {
  const response = await get<GatewayAgentsStatusResponse>('/agents-status', signal);
  return parseAgentsStatusResponse(response);
}

// Fetch a single agent by ID.
export function fetchAgent(id: number, signal?: AbortSignal): Promise<Agent> {
  return get<Agent>(`/agents/${id}`, signal);
}

// Register a new agent.
export function createAgent(data: CreateAgentRequest): Promise<Agent> {
  return post<Agent>('/agents', data);
}

// Update an agent.
export function updateAgent(
  id: number,
  data: Partial<CreateAgentRequest>,
): Promise<Agent> {
  return patch<Agent>(`/agents/${id}`, data);
}

// Send a heartbeat for an agent.
export function sendHeartbeat(data: HeartbeatRequest): Promise<void> {
  return post<void>('/heartbeat', data);
}
