// API client for the command center feed.

import { get } from './client.js';

// Event kinds classified by the backend feed.
export type CommandEventKind =
  | 'plan'
  | 'diff'
  | 'question'
  | 'status'
  | 'review'
  | 'message'
  | 'steer';

// Attention queue item kinds.
export type AttentionKind = 'plan' | 'question' | 'urgent' | 'blocked';

// A single classified feed entry inside an agent lane.
export interface CommandEvent {
  message_id: number;
  thread_id: string;
  kind: CommandEventKind;
  subject: string;
  body: string;
  priority: string;
  state: string;
  created_at: string;
  needs_action: boolean;
  waiting_for?: string;
  plan_review_id?: string;
  plan_state?: string;
  has_diff: boolean;
  direction: 'in' | 'out';
}

// Agent header info for a lane.
export interface CommandLaneAgent {
  id: number;
  name: string;
  project_key: string;
  git_branch: string;
  purpose: string;
  working_dir: string;
  status: string;
  last_active_at: string;
  seconds_since_heartbeat: number;
  session_id?: string;
}

// One agent's live state plus recent classified events.
export interface CommandLane {
  agent: CommandLaneAgent;
  events: CommandEvent[];
  unread_count: number;
  needs_action_count: number;
  waiting_for?: string;
  last_event_at?: string;
}

// One actionable entry in the cross-agent attention queue.
export interface AttentionItem {
  kind: AttentionKind;
  agent_id: number;
  agent_name: string;
  title: string;
  detail?: string;
  thread_id?: string;
  message_id?: number;
  plan_review_id?: string;
  priority?: string;
  created_at: string;
}

// Full command feed response.
export interface CommandFeedResponse {
  generated_at: string;
  lanes: CommandLane[];
  attention: AttentionItem[];
}

// Fetch the aggregated command center feed.
export function getCommandFeed(): Promise<CommandFeedResponse> {
  return get<CommandFeedResponse>('/command/feed');
}

// One high-granularity event parsed from an agent's Claude Code
// session transcript.
export interface FlowEvent {
  timestamp: string;
  kind: 'prompt' | 'text' | 'tool' | 'thinking';
  label: string;
  detail?: string;
}

// Response for the per-agent flow endpoint.
export interface AgentFlowResponse {
  agent_id: number;
  session_id?: string;
  events: FlowEvent[];
}

// Fetch recent transcript flow events for one agent.
export function getAgentFlow(
  agentId: number,
  limit = 60,
): Promise<AgentFlowResponse> {
  return get<AgentFlowResponse>(
    `/command/flow/${agentId}?limit=${limit}`,
  );
}

// A document referenced by an agent, served from its working dir.
export interface AgentDocResponse {
  path: string;
  size: number;
  modified: string;
  content: string;
  truncated: boolean;
}

// Fetch a referenced document for the viewer panel.
export function getAgentDoc(
  agentId: number,
  path: string,
): Promise<AgentDocResponse> {
  return get<AgentDocResponse>(
    `/command/doc/${agentId}?path=${encodeURIComponent(path)}`,
  );
}
