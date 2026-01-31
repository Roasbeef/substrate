// API functions for message-related operations.

import { get, post, patch } from './client.js';
import type {
  APIResponse,
  Message,
  MessageWithRecipients,
  SendMessageRequest,
} from '@/types/api.js';

// Message list filter options.
export interface MessageListOptions {
  page?: number;
  pageSize?: number;
  filter?: 'all' | 'unread' | 'starred';
  category?: 'inbox' | 'starred' | 'snoozed' | 'sent' | 'archive';
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

  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

// Fetch messages with optional filters.
export function fetchMessages(
  options: MessageListOptions = {},
  signal?: AbortSignal,
): Promise<APIResponse<MessageWithRecipients[]>> {
  const query = buildQueryString(options);
  return get<APIResponse<MessageWithRecipients[]>>(`/messages${query}`, signal);
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
  return patch<void>(`/messages/${id}`, { state: 'read' });
}

// Mark a message as acknowledged.
export function acknowledgeMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/ack`, {});
}

// Delete a message.
export function deleteMessage(id: number): Promise<void> {
  return post<void>(`/messages/${id}/delete`, {});
}
