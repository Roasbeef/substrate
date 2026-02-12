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
  project_key?: string;
  git_branch?: string;
  created_at: string;
  last_active_at: string;
}

export type AgentStatusType = 'active' | 'busy' | 'idle' | 'offline';

export interface AgentWithStatus {
  id: number;
  name: string;
  project_key?: string;
  git_branch?: string;
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
  sender_project_key?: string;
  sender_git_branch?: string;
  subject: string;
  body: string;
  priority: MessagePriority;
  created_at: string;
  thread_id?: string;
  recipient_names?: string[];
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
  id: string;
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
  // Additional fields for message results to enable navigation.
  thread_id?: string;
  sender_name?: string;
}

// Autocomplete types.
export interface AutocompleteRecipient {
  id: number;
  name: string;
  project_key?: string;
  git_branch?: string;
  status?: AgentStatusType;
}

// Review types.
export type ReviewState =
  | 'pending_review'
  | 'under_review'
  | 'changes_requested'
  | 'approved'
  | 'rejected'
  | 'cancelled';

export type ReviewType = 'full' | 'incremental' | 'security' | 'performance';

export interface ReviewSummary {
  review_id: string;
  thread_id: string;
  requester_id: number;
  branch: string;
  state: ReviewState;
  review_type: ReviewType;
  created_at: number;
}

export interface ReviewIterationDetail {
  iteration_num: number;
  reviewer_id: string;
  decision: string;
  summary: string;
  files_reviewed: number;
  lines_analyzed: number;
  duration_ms: number;
  cost_usd: number;
  started_at: number;
  completed_at: number;
}

export interface ReviewDetail {
  review_id: string;
  thread_id: string;
  state: ReviewState;
  branch: string;
  base_branch: string;
  review_type: ReviewType;
  iterations: number;
  open_issues: number;
  error?: string;
  iteration_details?: ReviewIterationDetail[];
}

export type IssueSeverity = 'critical' | 'major' | 'minor' | 'suggestion';
export type IssueType = 'bug' | 'security' | 'performance' | 'style' | 'documentation' | 'other';
export type IssueStatus = 'open' | 'fixed' | 'wont_fix' | 'duplicate';

export interface ReviewIssue {
  id: number;
  review_id: string;
  iteration_num: number;
  issue_type: IssueType;
  severity: IssueSeverity;
  file_path: string;
  line_start: number;
  line_end: number;
  title: string;
  description: string;
  code_snippet: string;
  suggestion: string;
  claude_md_ref: string;
  status: IssueStatus;
}

export interface CreateReviewRequest {
  branch: string;
  base_branch?: string;
  commit_sha: string;
  repo_path: string;
  pr_number?: number;
  review_type?: ReviewType;
  priority?: string;
  reviewers?: string[];
  description?: string;
  requester_id: number;
  remote_url?: string;
}

export interface CreateReviewResponse {
  review_id: string;
  thread_id: string;
  state: string;
  error?: string;
}

// Plan review types.
export type PlanReviewState = 'pending' | 'approved' | 'rejected' | 'changes_requested';

export interface PlanReview {
  id: number;
  plan_review_id: string;
  thread_id: string;
  requester_id: number;
  reviewer_name: string;
  plan_path: string;
  plan_title: string;
  plan_summary: string;
  state: PlanReviewState;
  reviewer_comment: string;
  reviewed_by: number;
  session_id: string;
  message_id: number;
  created_at: number;
  updated_at: number;
  reviewed_at: number;
}

// Create/update request types.
// Proto priority enum values accepted by gRPC-gateway.
export type ProtoPriority =
  | 'PRIORITY_UNSPECIFIED'
  | 'PRIORITY_LOW'
  | 'PRIORITY_NORMAL'
  | 'PRIORITY_URGENT';

export interface SendMessageRequest {
  sender_id: number;
  recipient_names: string[];
  subject: string;
  body: string;
  priority?: ProtoPriority;
  topic_name?: string;
  thread_id?: string;
  deadline_at?: string;
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
