// API type definitions for the Subtrate backend.

// Generic API response wrapper.
export interface APIResponse<T> {
  data: T;
  meta?: APIMeta;
}

// Pagination metadata.
export interface APIMeta {
  total: number;
  page: number;
  page_size: number;
}

// API error response.
export interface APIError {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}

// Agent types.
export interface Agent {
  id: number;
  name: string;
  created_at: string;
  last_active_at: string;
}

export type AgentStatusType = 'active' | 'busy' | 'idle' | 'offline';

export interface AgentWithStatus {
  id: number;
  name: string;
  status: AgentStatusType;
  last_active_at: string;
  session_id?: number;
  seconds_since_heartbeat: number;
}

export interface AgentStatusCounts {
  active: number;
  busy: number;
  idle: number;
  offline: number;
}

export interface AgentsStatusResponse {
  agents: AgentWithStatus[];
  counts: AgentStatusCounts;
}

// Message types.
export interface Message {
  id: number;
  sender_id: number;
  sender_name: string;
  subject: string;
  body: string;
  priority: MessagePriority;
  created_at: string;
  thread_id?: number;
}

export type MessagePriority = 'low' | 'normal' | 'high' | 'urgent';

export interface MessageRecipient {
  message_id: number;
  agent_id: number;
  agent_name: string;
  state: MessageState;
  is_starred: boolean;
  is_archived: boolean;
  snoozed_until?: string;
  read_at?: string;
  acknowledged_at?: string;
}

export type MessageState = 'unread' | 'read' | 'acknowledged';

export interface MessageWithRecipients extends Message {
  recipients: MessageRecipient[];
}

// Thread types.
export interface Thread {
  id: number;
  subject: string;
  created_at: string;
  last_message_at: string;
  message_count: number;
  participant_count: number;
}

export interface ThreadWithMessages extends Thread {
  messages: Message[];
}

// Topic types.
export interface Topic {
  id: number;
  name: string;
  description?: string;
  created_at: string;
  message_count: number;
}

// Session types.
export interface Session {
  id: number;
  agent_id: number;
  agent_name: string;
  project?: string;
  branch?: string;
  started_at: string;
  ended_at?: string;
  status: SessionStatus;
}

export type SessionStatus = 'active' | 'completed' | 'abandoned';

// Activity types.
export interface Activity {
  id: number;
  agent_id: number;
  agent_name: string;
  type: ActivityType;
  description: string;
  created_at: string;
  metadata?: Record<string, unknown>;
}

export type ActivityType =
  | 'message_sent'
  | 'message_read'
  | 'session_started'
  | 'session_completed'
  | 'agent_registered'
  | 'heartbeat';

// Dashboard stats.
export interface DashboardStats {
  active_agents: number;
  running_sessions: number;
  pending_messages: number;
  completed_today: number;
}

// Health check response.
export interface HealthResponse {
  status: 'ok' | 'error';
  time: string;
}

// Search types.
export interface SearchResult {
  type: 'message' | 'thread' | 'agent' | 'topic';
  id: number;
  title: string;
  snippet: string;
  created_at: string;
}

// Autocomplete types.
export interface AutocompleteRecipient {
  id: number;
  name: string;
  status?: AgentStatusType;
}

// Create/update request types.
export interface SendMessageRequest {
  to: number[];
  subject: string;
  body: string;
  priority?: MessagePriority;
  topic_id?: number;
  deadline?: string;
}

export interface CreateAgentRequest {
  name: string;
}

export interface StartSessionRequest {
  project?: string;
  branch?: string;
}

export interface HeartbeatRequest {
  agent_id?: number;
  session_id?: string;
}
