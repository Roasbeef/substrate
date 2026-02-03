// Unit tests for threads API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchThread,
  replyToThread,
  archiveThread,
  unarchiveThread,
  markThreadUnread,
  deleteThread,
} from '@/api/threads.js';
import type { ThreadWithMessages } from '@/types/api.js';

// Mock thread data.
const mockThread: ThreadWithMessages = {
  id: 1,
  subject: 'Test Thread',
  created_at: new Date().toISOString(),
  last_message_at: new Date().toISOString(),
  message_count: 3,
  participant_count: 2,
  messages: [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Agent1',
      subject: 'Test Thread',
      body: 'First message',
      priority: 'normal',
      created_at: new Date(Date.now() - 7200000).toISOString(),
      recipients: [],
    },
    {
      id: 2,
      sender_id: 2,
      sender_name: 'Agent2',
      subject: 'Re: Test Thread',
      body: 'Reply message',
      priority: 'normal',
      created_at: new Date(Date.now() - 3600000).toISOString(),
      recipients: [],
    },
    {
      id: 3,
      sender_id: 1,
      sender_name: 'Agent1',
      subject: 'Re: Test Thread',
      body: 'Another reply',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [],
    },
  ],
};

describe('threads API', () => {
  describe('fetchThread', () => {
    it('should fetch a thread by ID', async () => {
      server.use(
        http.get('/api/v1/threads/1', () => {
          return HttpResponse.json(mockThread);
        }),
      );

      const thread = await fetchThread(1);

      expect(thread.id).toBe(1);
      expect(thread.subject).toBe('Test Thread');
      expect(thread.messages).toHaveLength(3);
    });

    it('should return thread with all messages', async () => {
      server.use(
        http.get('/api/v1/threads/1', () => {
          return HttpResponse.json(mockThread);
        }),
      );

      const thread = await fetchThread(1);

      expect(thread.message_count).toBe(3);
      expect(thread.messages[0]?.body).toBe('First message');
      expect(thread.messages[1]?.body).toBe('Reply message');
      expect(thread.messages[2]?.body).toBe('Another reply');
    });

    it('should return thread with participant count', async () => {
      server.use(
        http.get('/api/v1/threads/1', () => {
          return HttpResponse.json(mockThread);
        }),
      );

      const thread = await fetchThread(1);

      expect(thread.participant_count).toBe(2);
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.get('/api/v1/threads/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(fetchThread(999)).rejects.toThrow();
    });

    it('should handle abort signal', async () => {
      server.use(
        http.get('/api/v1/threads/1', () => {
          return HttpResponse.json(mockThread);
        }),
      );

      const controller = new AbortController();
      controller.abort();

      await expect(fetchThread(1, controller.signal)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.get('/api/v1/threads/1', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(fetchThread(1)).rejects.toThrow();
    });
  });

  describe('replyToThread', () => {
    it('should reply to a thread', async () => {
      server.use(
        http.post('/api/v1/threads/1/reply', async ({ request }) => {
          const body = (await request.json()) as { body?: string };
          expect(body.body).toBe('My reply message');
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(replyToThread(1, 'My reply message')).resolves.toBeUndefined();
    });

    it('should handle validation error for empty body', async () => {
      server.use(
        http.post('/api/v1/threads/1/reply', () => {
          return HttpResponse.json(
            { error: { code: 'validation_error', message: 'Body required' } },
            { status: 400 },
          );
        }),
      );

      await expect(replyToThread(1, '')).rejects.toThrow();
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.post('/api/v1/threads/999/reply', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(replyToThread(999, 'Reply')).rejects.toThrow();
    });

    it('should handle archived thread error', async () => {
      server.use(
        http.post('/api/v1/threads/1/reply', () => {
          return HttpResponse.json(
            { error: { code: 'thread_archived', message: 'Cannot reply to archived thread' } },
            { status: 400 },
          );
        }),
      );

      await expect(replyToThread(1, 'Reply')).rejects.toThrow();
    });
  });

  describe('archiveThread', () => {
    it('should archive a thread', async () => {
      server.use(
        http.post('/api/v1/threads/1/archive', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(archiveThread(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.post('/api/v1/threads/999/archive', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(archiveThread(999)).rejects.toThrow();
    });

    it('should handle already archived error', async () => {
      server.use(
        http.post('/api/v1/threads/1/archive', () => {
          return HttpResponse.json(
            { error: { code: 'already_archived', message: 'Thread already archived' } },
            { status: 400 },
          );
        }),
      );

      await expect(archiveThread(1)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.post('/api/v1/threads/1/archive', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(archiveThread(1)).rejects.toThrow();
    });
  });

  describe('markThreadUnread', () => {
    it('should mark a thread as unread', async () => {
      server.use(
        http.post('/api/v1/threads/1/unread', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(markThreadUnread(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.post('/api/v1/threads/999/unread', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(markThreadUnread(999)).rejects.toThrow();
    });

    it('should handle already unread error', async () => {
      server.use(
        http.post('/api/v1/threads/1/unread', () => {
          return HttpResponse.json(
            { error: { code: 'already_unread', message: 'Thread already unread' } },
            { status: 400 },
          );
        }),
      );

      await expect(markThreadUnread(1)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.post('/api/v1/threads/1/unread', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(markThreadUnread(1)).rejects.toThrow();
    });
  });

  describe('deleteThread', () => {
    it('should delete a thread', async () => {
      server.use(
        http.post('/api/v1/threads/1/delete', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(deleteThread(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.post('/api/v1/threads/999/delete', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(deleteThread(999)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.post('/api/v1/threads/1/delete', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(deleteThread(1)).rejects.toThrow();
    });
  });

  describe('unarchiveThread', () => {
    it('should unarchive a thread', async () => {
      server.use(
        http.post('/api/v1/threads/1/unarchive', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(unarchiveThread(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent thread', async () => {
      server.use(
        http.post('/api/v1/threads/999/unarchive', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Thread not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(unarchiveThread(999)).rejects.toThrow();
    });

    it('should handle not archived error', async () => {
      server.use(
        http.post('/api/v1/threads/1/unarchive', () => {
          return HttpResponse.json(
            { error: { code: 'not_archived', message: 'Thread not archived' } },
            { status: 400 },
          );
        }),
      );

      await expect(unarchiveThread(1)).rejects.toThrow();
    });

    it('should handle server error', async () => {
      server.use(
        http.post('/api/v1/threads/1/unarchive', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Internal error' } },
            { status: 500 },
          );
        }),
      );

      await expect(unarchiveThread(1)).rejects.toThrow();
    });
  });
});
