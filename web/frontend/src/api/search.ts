// API functions for search and autocomplete.
// Uses grpc-gateway REST API directly.

import { get } from './client.js';
import type {
  APIResponse,
  SearchResult,
  AutocompleteRecipient,
  AgentStatusType,
} from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize agent status from proto format.
function normalizeAgentStatus(status?: string): AgentStatusType {
  if (!status) return 'offline';
  const normalized = status.toLowerCase().replace('agent_status_', '');
  if (normalized === 'active' || normalized === 'busy' || normalized === 'idle' || normalized === 'offline') {
    return normalized;
  }
  return 'offline';
}

// Gateway response format for search - returns InboxMessage objects.
interface GatewayInboxMessage {
  id: string;
  thread_id: string;
  topic_id?: string;
  sender_id?: string;
  sender_name?: string;
  sender_project_key?: string;
  sender_git_branch?: string;
  subject: string;
  body: string;
  priority?: string;
  state?: string;
  created_at?: string;
  deadline_at?: string | null;
  snoozed_until?: string | null;
  read_at?: string | null;
  acknowledged_at?: string | null;
}

interface GatewaySearchResponse {
  results?: GatewayInboxMessage[];
}

interface GatewayAutocompleteRecipient {
  id: string;
  name: string;
  project_key?: string;
  git_branch?: string;
  status?: string;
}

interface GatewayAutocompleteResponse {
  recipients?: GatewayAutocompleteRecipient[];
}

// Parse gateway search result (InboxMessage) to SearchResult format.
function parseSearchResult(msg: GatewayInboxMessage): SearchResult {
  // Extract a snippet from the body (first 150 chars).
  const snippet = msg.body.length > 150
    ? msg.body.substring(0, 150) + '...'
    : msg.body;

  const result: SearchResult = {
    type: 'message',
    id: toNumber(msg.id),
    title: msg.subject,
    snippet,
    created_at: msg.created_at ?? new Date().toISOString(),
    thread_id: msg.thread_id,
  };

  // Include sender name if available.
  if (msg.sender_name !== undefined) {
    result.sender_name = msg.sender_name;
  }

  return result;
}

// Parse gateway autocomplete recipient to internal format.
function parseAutocompleteRecipient(recipient: GatewayAutocompleteRecipient): AutocompleteRecipient {
  const result: AutocompleteRecipient = {
    id: toNumber(recipient.id),
    name: recipient.name,
  };
  if (recipient.project_key !== undefined) {
    result.project_key = recipient.project_key;
  }
  if (recipient.git_branch !== undefined) {
    result.git_branch = recipient.git_branch;
  }
  if (recipient.status !== undefined) {
    result.status = normalizeAgentStatus(recipient.status);
  }
  return result;
}

// Search across messages, threads, agents, and topics.
export async function search(
  query: string,
  signal?: AbortSignal,
): Promise<APIResponse<SearchResult[]>> {
  const encoded = encodeURIComponent(query);
  const response = await get<GatewaySearchResponse>(`/search?query=${encoded}`, signal);
  const results = (response.results ?? []).map(parseSearchResult);
  return {
    data: results,
    meta: {
      total: results.length,
      page: 1,
      page_size: results.length,
    },
  };
}

// Autocomplete recipients (agents).
export async function autocompleteRecipients(
  query: string,
  signal?: AbortSignal,
): Promise<AutocompleteRecipient[]> {
  const encoded = encodeURIComponent(query);
  const response = await get<GatewayAutocompleteResponse>(
    `/autocomplete/recipients?query=${encoded}`,
    signal,
  );
  return (response.recipients ?? []).map(parseAutocompleteRecipient);
}
