// Thread grouping utility — collapses messages sharing a thread_id into
// a single representative row for the inbox list.

import type { MessageWithRecipients, MessagePriority } from '@/types/api.js';

// Represents a group of messages sharing a thread_id, collapsed into a
// single row in the inbox list.
export interface ThreadGroup {
  // The thread_id shared by all messages (or the single message's id
  // as string for standalone messages).
  threadId: string;
  // The most recent message in the thread (displayed in the row).
  latestMessage: MessageWithRecipients;
  // Total number of messages in this thread.
  messageCount: number;
  // Whether ANY message in the thread is unread.
  hasUnread: boolean;
  // Whether ANY message in the thread is starred.
  hasStarred: boolean;
  // All message IDs in this thread (for bulk operations).
  messageIds: number[];
  // Highest priority across all thread messages.
  highestPriority: MessagePriority;
  // Unique sender names across all thread messages.
  senderNames: string[];
}

// Priority ordering for comparison (higher index = higher priority).
const PRIORITY_ORDER: Record<MessagePriority, number> = {
  low: 0,
  normal: 1,
  high: 2,
  urgent: 3,
};

// groupMessagesByThread groups a flat list of messages by thread_id,
// returning one ThreadGroup per unique thread sorted by most recent
// activity. Messages without a thread_id are treated as standalone
// threads using their message ID as the thread identifier.
export function groupMessagesByThread(
  messages: MessageWithRecipients[],
): ThreadGroup[] {
  const threadMap = new Map<string, MessageWithRecipients[]>();

  // Bucket messages by thread_id.
  for (const message of messages) {
    const key = message.thread_id ?? String(message.id);
    const bucket = threadMap.get(key);
    if (bucket) {
      bucket.push(message);
    } else {
      threadMap.set(key, [message]);
    }
  }

  // Build a ThreadGroup for each bucket.
  const groups: ThreadGroup[] = [];
  for (const [threadId, msgs] of threadMap) {
    // Skip empty buckets defensively (should never happen since each
    // bucket starts with at least one message).
    if (msgs.length === 0) continue;

    // Sort descending by created_at to find the latest message.
    msgs.sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );

    const latestMessage = msgs[0]!;
    const senderSet = new Set<string>();
    let hasUnread = false;
    let hasStarred = false;
    let highestPriority: MessagePriority = 'low';

    const messageIds: number[] = [];
    for (const msg of msgs) {
      messageIds.push(msg.id);
      senderSet.add(msg.sender_name);

      // Check across all recipients, not just the first.
      if (msg.recipients.some((r) => r.state === 'unread')) {
        hasUnread = true;
      }
      if (msg.recipients.some((r) => r.is_starred)) {
        hasStarred = true;
      }
      if (PRIORITY_ORDER[msg.priority] > PRIORITY_ORDER[highestPriority]) {
        highestPriority = msg.priority;
      }
    }

    groups.push({
      threadId,
      latestMessage,
      messageCount: msgs.length,
      hasUnread,
      hasStarred,
      messageIds,
      highestPriority,
      senderNames: Array.from(senderSet),
    });
  }

  // Sort groups by latest message timestamp descending.
  groups.sort(
    (a, b) =>
      new Date(b.latestMessage.created_at).getTime() -
      new Date(a.latestMessage.created_at).getTime(),
  );

  return groups;
}
