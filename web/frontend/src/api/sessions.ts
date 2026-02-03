// API functions for session-related operations.
// Uses grpc-gateway REST API directly.

import { get, post } from './client.js';
import type {
  APIResponse,
  Session,
  SessionStatus,
  StartSessionRequest,
} from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize session status enum from proto format.
function normalizeStatus(status?: string): SessionStatus {
  if (!status) return 'active';
  const normalized = status.toLowerCase().replace('session_status_', '');
  if (normalized === 'active' || normalized === 'completed' || normalized === 'abandoned') {
    return normalized;
  }
  return 'active';
}

// Gateway response formats.
interface GatewaySession {
  id: string;
  agent_id: string;
  agent_name: string;
  project?: string;
  branch?: string;
  started_at?: string;
  ended_at?: string;
  status?: string;
}

interface GatewaySessionsResponse {
  sessions?: GatewaySession[];
}

interface GatewaySessionResponse {
  session?: GatewaySession;
}

// Parse gateway session to internal format.
function parseSession(session: GatewaySession): Session {
  const result: Session = {
    id: toNumber(session.id),
    agent_id: toNumber(session.agent_id),
    agent_name: session.agent_name,
    started_at: session.started_at ?? new Date().toISOString(),
    status: normalizeStatus(session.status),
  };
  if (session.project !== undefined) {
    result.project = session.project;
  }
  if (session.branch !== undefined) {
    result.branch = session.branch;
  }
  if (session.ended_at !== undefined) {
    result.ended_at = session.ended_at;
  }
  return result;
}

// Fetch active sessions.
export async function fetchActiveSessions(
  signal?: AbortSignal,
): Promise<APIResponse<Session[]>> {
  const response = await get<GatewaySessionsResponse>(
    '/sessions?active_only=true',
    signal,
  );
  const sessions = (response.sessions ?? []).map(parseSession);
  return {
    data: sessions,
    meta: { total: sessions.length, page: 1, page_size: sessions.length },
  };
}

// Fetch all sessions (including completed).
export async function fetchSessions(
  signal?: AbortSignal,
): Promise<APIResponse<Session[]>> {
  const response = await get<GatewaySessionsResponse>('/sessions', signal);
  const sessions = (response.sessions ?? []).map(parseSession);
  return {
    data: sessions,
    meta: { total: sessions.length, page: 1, page_size: sessions.length },
  };
}

// Fetch a single session by ID.
export async function fetchSession(
  id: number,
  signal?: AbortSignal,
): Promise<Session> {
  const response = await get<GatewaySessionResponse>(`/sessions/${id}`, signal);
  if (!response.session) {
    throw new Error('Session not found');
  }
  return parseSession(response.session);
}

// Start a new session.
export async function startSession(data: StartSessionRequest): Promise<Session> {
  const response = await post<GatewaySessionResponse>('/sessions', data);
  if (!response.session) {
    throw new Error('Failed to start session');
  }
  return parseSession(response.session);
}

// Complete a session.
export function completeSession(id: number): Promise<void> {
  return post<void>(`/sessions/${id}/complete`, {});
}
