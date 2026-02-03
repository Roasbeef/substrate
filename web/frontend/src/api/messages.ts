// API functions for message-related operations.
// Uses grpc-gateway REST API directly.

import { get, post } from './client.js';
import type {
  APIResponse,
  Message,
  MessageWithRecipients,
  SendMessageRequest,
} from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize priority enum from proto format.
function normalizePriority(priority: string | undefined): MessageWithRecipients['priority'] {
  if (!priority) return 'normal';
  const normalized = priority.toLowerCase().replace('priority_', '');
  if (normalized === 'low' || normalized === 'normal' || normalized === 'urgent') {
    return normalized;
  }
  return 'normal';
}

// Gateway response format.
interface GatewayMessagesResponse {
  messages?: Array<{
    id: string;
    thread_id?: string;
    topic_id?: string;
    sender_id: string;
    sender_name?: string;
    sender_project_key?: string;
    sender_git_branch?: string;
    subject?: string;
    body?: string;
    priority?: string;
    state?: string;
    created_at?: string;
    deadline_at?: string;
    snoozed_until?: string;
    read_at?: string;
    acknowledged_at?: string;
  }>;
}

// Convert gateway response to internal format.
function parseMessagesResponse(response: GatewayMessagesResponse): APIResponse<MessageWithRecipients[]> {
  const messages = (response.messages ?? []).map((msg): MessageWithRecipients => {
    const result: MessageWithRecipients = {
      id: toNumber(msg.id),
      sender_id: toNumber(msg.sender_id),
      sender_name: msg.sender_name ?? 'Unknown',
      subject: msg.subject ?? '',
      body: msg.body ?? '',
      priority: normalizePriority(msg.priority),
      created_at: msg.created_at ?? new Date().toISOString(),
      recipients: [],
    };
    if (msg.thread_id !== undefined) {
      result.thread_id = msg.thread_id;
    }
    if (msg.sender_project_key) {
      result.sender_project_key = msg.sender_project_key;
    }
    if (msg.sender_git_branch) {
      result.sender_git_branch = msg.sender_git_branch;
    }
    return result;
  });
  return {
    data: messages,
    meta: { total: messages.length, page: 1, page_size: messages.length },
  };
}

// Message list filter options.
export interface MessageListOptions {
  page?: number;
  pageSize?: number;
  filter?: 'all' | 'unread' | 'starred';
  category?: 'inbox' | 'starred' | 'snoozed' | 'sent' | 'archive';
  agentId?: number;
}

// Build query string from options.
// Maps frontend filter options to proto field names.
function buildQueryString(options: MessageListOptions): string {
  const params = new URLSearchParams();

  if (options.page !== undefined) {
    params.set('page', String(options.page));
  }
  if (options.pageSize !== undefined) {
    params.set('limit', String(options.pageSize));
  }
  // Map filter to proto fields: unread_only or state_filter.
  if (options.filter === 'unread') {
    params.set('unread_only', 'true');
  } else if (options.filter === 'starred') {
    params.set('state_filter', 'STATE_STARRED');
  }
  // Handle sent category.
  if (options.category === 'sent') {
    params.set('sent_only', 'true');
  }
  if (options.agentId !== undefined) {
    params.set('agent_id', String(options.agentId));
  }

  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

// Fetch messages with optional filters.
export async function fetchMessages(
  options: MessageListOptions = {},
  signal?: AbortSignal,
): Promise<APIResponse<MessageWithRecipients[]>> {
  const query = buildQueryString(options);
  const response = await get<GatewayMessagesResponse>(`/messages${query}`, signal);
  return parseMessagesResponse(response);
}

// Fetch a single message by ID.
export function fetchMessage(
  id: number,
  signal?: AbortSignal,
): Promise<MessageWithRecipients> {
  return get<MessageWithRecipients>(`/messages/${id}`, signal);
}

// Send a new message.
export function sendMessage(data: SendMessageRequest): Promise<Message> {
  return post<Message>('/messages', data);
}

// Star/unstar a message.
export function toggleMessageStar(
  id: number,
  starred: boolean,
): Promise<void> {
  return post<void>(`/messages/${id}/star`, { starred });
}

// Archive a message.
export function archiveMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/archive`, {});
}

// Unarchive a message.
export function unarchiveMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/unarchive`, {});
}

// Snooze a message.
export function snoozeMessage(id: number, until: string): Promise<void> {
  return post<void>(`/messages/${id}/snooze`, { until });
}

// Mark a message as read.
export function markMessageRead(id: number): Promise<void> {
  return post<void>(`/messages/${id}/read`, {});
}

// Mark a message as acknowledged.
export function acknowledgeMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/ack`, {});
}

// Delete a message.
export function deleteMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/delete`, {});
}
