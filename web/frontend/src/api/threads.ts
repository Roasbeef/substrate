// API functions for thread-related operations.

import { get, post } from './client.js';
import type { ThreadWithMessages } from '@/types/api.js';

// Fetch a thread by ID with all its messages.
export function fetchThread(
  id: number,
  signal?: AbortSignal,
): Promise<ThreadWithMessages> {
  return get<ThreadWithMessages>(`/threads/${id}`, signal);
}

// Reply to a thread.
export function replyToThread(
  id: number,
  body: string,
): Promise<void> {
  return post<void>(`/threads/${id}/reply`, { body });
}

// Archive a thread (marks all messages as archived).
export function archiveThread(id: number): Promise<void> {
  return post<void>(`/threads/${id}/archive`, {});
}

// Mark thread as unread.
export function markThreadUnread(id: number): Promise<void> {
  return post<void>(`/threads/${id}/unread`, {});
}

// Delete a thread (deletes all messages in the thread).
export function deleteThread(id: number): Promise<void> {
  return post<void>(`/threads/${id}/delete`, {});
}

// Unarchive a thread (restores all messages from archive).
export function unarchiveThread(id: number): Promise<void> {
  return post<void>(`/threads/${id}/unarchive`, {});
}
