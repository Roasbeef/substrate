// API functions for thread-related operations.
// Uses grpc-gateway REST API directly.

import { get, post } from './client.js';
import type { Message, ThreadWithMessages, MessagePriority } from '@/types/api.js';

// Helper to convert proto int64 (string) to number.
function toNumber(value: string | number | undefined): number {
  if (value === undefined) return 0;
  return typeof value === 'string' ? Number(value) : value;
}

// Helper to normalize priority enum from proto format.
function normalizePriority(priority: string | undefined): MessagePriority {
  if (!priority) return 'normal';
  const normalized = priority.toLowerCase().replace('priority_', '');
  if (normalized === 'low' || normalized === 'normal' || normalized === 'urgent') {
    return normalized;
  }
  return 'normal';
}

// Gateway response format for thread.
interface GatewayReadThreadResponse {
  messages?: Array<{
    id: string;
    thread_id?: string;
    sender_id: string;
    sender_name?: string;
    sender_project_key?: string;
    sender_git_branch?: string;
    subject?: string;
    body?: string;
    priority?: string;
    created_at?: string;
    recipient_names?: string[];
  }>;
}

// Parse gateway messages to internal format.
function parseMessages(messages: GatewayReadThreadResponse['messages']): Message[] {
  return (messages ?? []).map((msg): Message => {
    const result: Message = {
      id: toNumber(msg.id),
      sender_id: toNumber(msg.sender_id),
      sender_name: msg.sender_name ?? 'Unknown',
      subject: msg.subject ?? '',
      body: msg.body ?? '',
      priority: normalizePriority(msg.priority),
      created_at: msg.created_at ?? new Date().toISOString(),
    };
    if (msg.thread_id !== undefined) {
      result.thread_id = msg.thread_id;
    }
    if (msg.sender_project_key !== undefined) {
      result.sender_project_key = msg.sender_project_key;
    }
    if (msg.sender_git_branch !== undefined) {
      result.sender_git_branch = msg.sender_git_branch;
    }
    if (msg.recipient_names && msg.recipient_names.length > 0) {
      result.recipient_names = msg.recipient_names;
    }
    return result;
  });
}

// Fetch a thread by ID with all its messages.
export async function fetchThread(
  id: string,
  signal?: AbortSignal,
): Promise<ThreadWithMessages> {
  const response = await get<GatewayReadThreadResponse>(`/threads/${id}`, signal);
  const messages = parseMessages(response.messages);

  // Build thread info from messages
  const firstMsg = messages[0];
  const lastMsg = messages[messages.length - 1];

  return {
    id,
    subject: firstMsg?.subject ?? '',
    created_at: firstMsg?.created_at ?? new Date().toISOString(),
    last_message_at: lastMsg?.created_at ?? new Date().toISOString(),
    message_count: messages.length,
    participant_count: new Set(messages.map(m => m.sender_id)).size,
    messages,
  };
}

// Reply to a thread.
export function replyToThread(
  id: string,
  body: string,
): Promise<void> {
  return post<void>(`/threads/${id}/reply`, { body });
}

// Archive a thread (marks all messages as archived).
export function archiveThread(id: string): Promise<void> {
  return post<void>(`/threads/${id}/archive`, {});
}

// Mark thread as unread.
export function markThreadUnread(id: string): Promise<void> {
  return post<void>(`/threads/${id}/unread`, {});
}

// Delete a thread (deletes all messages in the thread).
export function deleteThread(id: string): Promise<void> {
  return post<void>(`/threads/${id}/delete`, {});
}

// Unarchive a thread (restores all messages from archive).
export function unarchiveThread(id: string): Promise<void> {
  return post<void>(`/threads/${id}/unarchive`, {});
}
