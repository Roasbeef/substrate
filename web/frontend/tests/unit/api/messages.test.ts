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
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          const url = new URL(request.url);
          const page = url.searchParams.get('page');
          return HttpResponse.json({
            data: [mockMessage],
            meta: { total: 1, page: page ? Number(page) : 1, page_size: 20 },
          });
        }),
      );

      const response = await fetchMessages({ page: 2 });

      expect(response.meta.page).toBe(2);
    });

    it('should include filter parameter in query string', async () => {
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          const url = new URL(request.url);
          const filter = url.searchParams.get('filter');
          expect(filter).toBe('unread');
          return HttpResponse.json({
            data: [mockMessage],
            meta: { total: 1, page: 1, page_size: 20 },
          });
        }),
      );

      await fetchMessages({ filter: 'unread' });
    });

    it('should include category parameter in query string', async () => {
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          const url = new URL(request.url);
          const category = url.searchParams.get('category');
          expect(category).toBe('starred');
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          });
        }),
      );

      await fetchMessages({ category: 'starred' });
    });

    it('should include pageSize parameter in query string', async () => {
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          const url = new URL(request.url);
          const pageSize = url.searchParams.get('page_size');
          expect(pageSize).toBe('50');
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 50 },
          });
        }),
      );

      await fetchMessages({ pageSize: 50 });
    });

    it('should handle multiple query parameters', async () => {
      server.use(
        http.get('/api/v1/messages', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('page')).toBe('3');
          expect(url.searchParams.get('filter')).toBe('starred');
          expect(url.searchParams.get('category')).toBe('inbox');
          return HttpResponse.json({
            data: [mockMessage],
            meta: { total: 1, page: 3, page_size: 20 },
          });
        }),
      );

      await fetchMessages({ page: 3, filter: 'starred', category: 'inbox' });
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
            to?: number[];
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
        to: [2],
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
          expect(body.priority).toBe('urgent');
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
        to: [2],
        subject: 'Urgent',
        body: 'Important!',
        priority: 'urgent',
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
        sendMessage({ to: [2], subject: '', body: 'test' }),
      ).rejects.toThrow();
    });
  });

  describe('toggleMessageStar', () => {
    it('should star a message', async () => {
      server.use(
        http.post('/api/v1/messages/1/star', async ({ request }) => {
          const body = (await request.json()) as { starred?: boolean };
          expect(body.starred).toBe(true);
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(toggleMessageStar(1, true)).resolves.toBeUndefined();
    });

    it('should unstar a message', async () => {
      server.use(
        http.post('/api/v1/messages/1/star', async ({ request }) => {
          const body = (await request.json()) as { starred?: boolean };
          expect(body.starred).toBe(false);
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(toggleMessageStar(1, false)).resolves.toBeUndefined();
    });

    it('should handle error', async () => {
      server.use(
        http.post('/api/v1/messages/1/star', () => {
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
        http.post('/api/v1/messages/1/archive', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(archiveMessage(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent message', async () => {
      server.use(
        http.post('/api/v1/messages/999/archive', () => {
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
        http.post('/api/v1/messages/1/unarchive', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(unarchiveMessage(1)).resolves.toBeUndefined();
    });

    it('should handle error for non-archived message', async () => {
      server.use(
        http.post('/api/v1/messages/1/unarchive', () => {
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
        http.post('/api/v1/messages/1/snooze', async ({ request }) => {
          const body = (await request.json()) as { until?: string };
          expect(body.until).toBe(snoozeUntil);
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(snoozeMessage(1, snoozeUntil)).resolves.toBeUndefined();
    });

    it('should handle invalid date error', async () => {
      server.use(
        http.post('/api/v1/messages/1/snooze', () => {
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
          const body = (await request.json()) as { state?: string };
          expect(body.state).toBe('read');
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(markMessageRead(1)).resolves.toBeUndefined();
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
