// API functions for agent-related operations.

import { get, post, patch } from './client.js';
import type {
  Agent,
  AgentsStatusResponse,
  CreateAgentRequest,
  HeartbeatRequest,
} from '@/types/api.js';

// Fetch all agents with their current status.
export function fetchAgentsStatus(
  signal?: AbortSignal,
): Promise<AgentsStatusResponse> {
  return get<AgentsStatusResponse>('/agents/status', signal);
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
