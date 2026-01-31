// API functions for session-related operations.

import { get, post } from './client.js';
import type {
  APIResponse,
  Session,
  StartSessionRequest,
} from '@/types/api.js';

// Fetch active sessions.
export function fetchActiveSessions(
  signal?: AbortSignal,
): Promise<APIResponse<Session[]>> {
  return get<APIResponse<Session[]>>('/sessions/active', signal);
}

// Fetch all sessions (including completed).
export function fetchSessions(
  signal?: AbortSignal,
): Promise<APIResponse<Session[]>> {
  return get<APIResponse<Session[]>>('/sessions', signal);
}

// Fetch a single session by ID.
export function fetchSession(
  id: number,
  signal?: AbortSignal,
): Promise<Session> {
  return get<Session>(`/sessions/${id}`, signal);
}

// Start a new session.
export function startSession(data: StartSessionRequest): Promise<Session> {
  return post<Session>('/sessions', data);
}

// Complete a session.
export function completeSession(id: number): Promise<void> {
  return post<void>(`/sessions/${id}/complete`, {});
}
