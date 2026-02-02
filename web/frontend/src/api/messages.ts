// API functions for message-related operations.

import { get, post } from './client.js';
import type {
  APIResponse,
  Message,
  MessageWithRecipients,
  SendMessageRequest,
} from '@/types/api.js';

// Gateway response types (proto-based format from grpc-gateway).
interface GatewayInboxMessage {
  id: string;
  thread_id?: string;
  topic_id?: string;
  sender_id: string;
  sender_name?: string;
  subject?: string;
  body?: string;
  priority?: string;
  state?: string;
  created_at?: string;
  deadline_at?: string;
  snoozed_until?: string;
  read_at?: string;
  acked_at?: string;
}

interface GatewayFetchInboxResponse {
  messages?: GatewayInboxMessage[];
}

// Check if response is in gateway format (has 'messages' field, not 'data').
function isGatewayResponse(
  response: unknown,
): response is GatewayFetchInboxResponse {
  return (
    typeof response === 'object' &&
    response !== null &&
    'messages' in response &&
    !('data' in response)
  );
}

// Convert gateway message to frontend format.
function convertGatewayMessage(msg: GatewayInboxMessage): MessageWithRecipients {
  const result: MessageWithRecipients = {
    id: Number(msg.id),
    sender_id: Number(msg.sender_id),
    sender_name: msg.sender_name ?? 'Unknown',
    subject: msg.subject ?? '',
    body: msg.body ?? '',
    priority: (msg.priority?.toLowerCase().replace('priority_', '') ?? 'normal') as MessageWithRecipients['priority'],
    created_at: msg.created_at ?? new Date().toISOString(),
    recipients: [],
  };
  // Only set thread_id if it's defined (exactOptionalPropertyTypes compliance).
  if (msg.thread_id !== undefined) {
    result.thread_id = msg.thread_id;
  }
  return result;
}

// Normalize response to APIResponse format.
function normalizeMessagesResponse(
  response: APIResponse<MessageWithRecipients[]> | GatewayFetchInboxResponse,
): APIResponse<MessageWithRecipients[]> {
  if (isGatewayResponse(response)) {
    const messages = (response.messages ?? []).map(convertGatewayMessage);
    return {
      data: messages,
      meta: {
        total: messages.length,
        page: 1,
        page_size: messages.length,
      },
    };
  }
  return response;
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
function buildQueryString(options: MessageListOptions): string {
  const params = new URLSearchParams();

  if (options.page !== undefined) {
    params.set('page', String(options.page));
  }
  if (options.pageSize !== undefined) {
    params.set('page_size', String(options.pageSize));
  }
  if (options.filter !== undefined) {
    params.set('filter', options.filter);
  }
  if (options.category !== undefined) {
    params.set('category', options.category);
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
  const response = await get<APIResponse<MessageWithRecipients[]> | GatewayFetchInboxResponse>(
    `/messages${query}`,
    signal,
  );
  return normalizeMessagesResponse(response);
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
