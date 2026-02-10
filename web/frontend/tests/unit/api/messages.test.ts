// Unit tests for messages API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchMessages,
  fetchMessage,
  sendMessage,
  toggleMessageStar,
  archiveMessage,
  unarchiveMessage,
  snoozeMessage,
  markMessageRead,
  acknowledgeMessage,
  deleteMessage,
} from '@/api/messages.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Mock message data.
const mockMessage: MessageWithRecipients = {
  id: 1,
  sender_id: 1,
  sender_name: 'TestAgent',
  subject: 'Test Subject',
  body: 'Test body content',
  priority: 'normal',
  created_at: new Date().toISOString(),
  recipients: [
    {
      message_id: 1,
      agent_id: 2,
      agent_name: 'RecipientAgent',
      state: 'unread',
      is_starred: false,
      is_archived: false,
    },
  ],
};

describe('messages API', () => {
  describe('fetchMessages', () => {
    it('should fetch messages with default options', async () => {
      const response = await fetchMessages();

      expect(response.data).toBeDefined();
      expect(Array.isArray(response.data)).toBe(true);
    });

    it('should include page parameter in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            messages: [],
          });
        }),
      );

      await fetchMessages({ page: 2 });

      expect(receivedUrl).toContain('page=2');
    });

    it('should include filter parameter in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            messages: [],
          });
        }),
      );

      await fetchMessages({ filter: 'unread' });

      // Filter 'unread' maps to unread_only=true.
      expect(receivedUrl).toContain('unread_only=true');
    });

    it('should include category parameter in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            messages: [],
          });
        }),
      );

      // Note: 'starred' category actually uses filter='starred' which maps to
      // state_filter, and 'sent' maps to sent_only=true. The category param
      // is only used for 'sent'.
      await fetchMessages({ category: 'sent' });

      expect(receivedUrl).toContain('sent_only=true');
    });

    it('should include pageSize parameter in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            messages: [],
          });
        }),
      );

      await fetchMessages({ pageSize: 50 });

      // pageSize maps to 'limit' in the query string.
      expect(receivedUrl).toContain('limit=50');
    });

    it('should handle multiple query parameters', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            messages: [],
          });
        }),
      );

      await fetchMessages({ page: 3, filter: 'starred' });

      // filter 'starred' maps to state_filter=STATE_STARRED.
      expect(receivedUrl).toContain('page=3');
      expect(receivedUrl).toContain('state_filter=STATE_STARRED');
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchMessages({}, controller.signal)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.get('/api/v1/messages', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(fetchMessages()).rejects.toThrow();
    });
  });

  describe('fetchMessage', () => {
    it('should fetch a single message by ID', async () => {
      server.use(
        http.get('/api/v1/messages/1', () => {
          return HttpResponse.json(mockMessage);
        }),
      );

      const message = await fetchMessage(1);

      expect(message.id).toBe(1);
      expect(message.subject).toBe('Test Subject');
    });

    it('should handle 404 for non-existent message', async () => {
      server.use(
        http.get('/api/v1/messages/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Message not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(fetchMessage(999)).rejects.toThrow();
    });

    it('should handle abort signal', async () => {
      server.use(
        http.get('/api/v1/messages/1', () => {
          return HttpResponse.json(mockMessage);
        }),
      );

      const controller = new AbortController();
      controller.abort();

      await expect(fetchMessage(1, controller.signal)).rejects.toThrow();
    });
  });

  describe('sendMessage', () => {
    it('should send a new message', async () => {
      server.use(
        http.post('/api/v1/messages', async ({ request }) => {
          const body = (await request.json()) as {
            sender_id?: number;
            recipient_names?: string[];
            subject?: string;
            body?: string;
          };
          return HttpResponse.json({
            id: 100,
            sender_id: 1,
            sender_name: 'TestAgent',
            subject: body.subject,
            body: body.body,
            priority: 'normal',
            created_at: new Date().toISOString(),
          });
        }),
      );

      const result = await sendMessage({
        sender_id: 1,
        recipient_names: ['Bob'],
        subject: 'New Message',
        body: 'Hello, World!',
      });

      expect(result.id).toBe(100);
      expect(result.subject).toBe('New Message');
    });

    it('should send message with priority', async () => {
      server.use(
        http.post('/api/v1/messages', async ({ request }) => {
          const body = (await request.json()) as { priority?: string };
          expect(body.priority).toBe('PRIORITY_URGENT');
          return HttpResponse.json({
            id: 101,
            sender_id: 1,
            sender_name: 'TestAgent',
            subject: 'Urgent',
            body: 'Important!',
            priority: body.priority,
            created_at: new Date().toISOString(),
          });
        }),
      );

      await sendMessage({
        sender_id: 1,
        recipient_names: ['Bob'],
        subject: 'Urgent',
        body: 'Important!',
        priority: 'PRIORITY_URGENT',
      });
    });

    it('should handle validation error', async () => {
      server.use(
        http.post('/api/v1/messages', () => {
          return HttpResponse.json(
            { error: { code: 'validation_error', message: 'Subject required' } },
            { status: 400 },
          );
        }),
      );

      await expect(
        sendMessage({ sender_id: 1, recipient_names: ['Bob'], subject: '', body: 'test' }),
      ).rejects.toThrow();
    });
  });

  describe('toggleMessageStar', () => {
    it('should star a message', async () => {
      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string };
          expect(body.new_state).toBe('STATE_STARRED');
          return HttpResponse.json({ success: true });
        }),
      );

      await toggleMessageStar(1, true);
    });

    it('should unstar a message', async () => {
      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string };
          expect(body.new_state).toBe('STATE_READ');
          return HttpResponse.json({ success: true });
        }),
      );

      await toggleMessageStar(1, false);
    });

    it('should handle error', async () => {
      server.use(
        http.patch('/api/v1/messages/1', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Message not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(toggleMessageStar(1, true)).rejects.toThrow();
    });
  });

  describe('archiveMessage', () => {
    it('should archive a message', async () => {
      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string };
          expect(body.new_state).toBe('STATE_ARCHIVED');
          return HttpResponse.json({ success: true });
        }),
      );

      await archiveMessage(1);
    });

    it('should handle 404 for non-existent message', async () => {
      server.use(
        http.patch('/api/v1/messages/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Message not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(archiveMessage(999)).rejects.toThrow();
    });
  });

  describe('unarchiveMessage', () => {
    it('should unarchive a message', async () => {
      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string };
          expect(body.new_state).toBe('STATE_READ');
          return HttpResponse.json({ success: true });
        }),
      );

      await unarchiveMessage(1);
    });

    it('should handle error for non-archived message', async () => {
      server.use(
        http.patch('/api/v1/messages/1', () => {
          return HttpResponse.json(
            { error: { code: 'invalid_state', message: 'Message not archived' } },
            { status: 400 },
          );
        }),
      );

      await expect(unarchiveMessage(1)).rejects.toThrow();
    });
  });

  describe('snoozeMessage', () => {
    it('should snooze a message', async () => {
      const snoozeUntil = new Date(Date.now() + 3600000).toISOString();

      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string; snoozed_until?: string };
          expect(body.new_state).toBe('STATE_SNOOZED');
          expect(body.snoozed_until).toBe(snoozeUntil);
          return HttpResponse.json({ success: true });
        }),
      );

      await snoozeMessage(1, snoozeUntil);
    });

    it('should handle invalid date error', async () => {
      server.use(
        http.patch('/api/v1/messages/1', () => {
          return HttpResponse.json(
            { error: { code: 'validation_error', message: 'Invalid date' } },
            { status: 400 },
          );
        }),
      );

      await expect(snoozeMessage(1, 'invalid-date')).rejects.toThrow();
    });
  });

  describe('markMessageRead', () => {
    it('should mark a message as read', async () => {
      server.use(
        http.patch('/api/v1/messages/1', async ({ request }) => {
          const body = (await request.json()) as { new_state?: string };
          expect(body.new_state).toBe('STATE_READ');
          return HttpResponse.json({ success: true });
        }),
      );

      await markMessageRead(1);
    });

    it('should handle error', async () => {
      server.use(
        http.patch('/api/v1/messages/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Message not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(markMessageRead(999)).rejects.toThrow();
    });
  });

  describe('acknowledgeMessage', () => {
    it('should acknowledge a message', async () => {
      server.use(
        http.post('/api/v1/messages/1/ack', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(acknowledgeMessage(1)).resolves.toBeUndefined();
    });

    it('should handle already acknowledged error', async () => {
      server.use(
        http.post('/api/v1/messages/1/ack', () => {
          return HttpResponse.json(
            { error: { code: 'already_acknowledged', message: 'Already acked' } },
            { status: 400 },
          );
        }),
      );

      await expect(acknowledgeMessage(1)).rejects.toThrow();
    });
  });

  describe('deleteMessage', () => {
    it('should delete a message', async () => {
      server.use(
        http.post('/api/v1/messages/1/delete', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(deleteMessage(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent message', async () => {
      server.use(
        http.post('/api/v1/messages/999/delete', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Message not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(deleteMessage(999)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.post('/api/v1/messages/1/delete', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(deleteMessage(1)).rejects.toThrow();
    });
  });
});
