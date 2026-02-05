// Unit tests for activities API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchActivities,
  fetchAgentActivities,
  type ActivitiesResponse,
} from '@/api/activities.js';

describe('activities API', () => {
  describe('fetchActivities', () => {
    it('should fetch activities list', async () => {
      const response = await fetchActivities();

      expect(response.data).toBeDefined();
      expect(Array.isArray(response.data)).toBe(true);
      expect(response.meta).toBeDefined();
    });

    it('should include pagination metadata', async () => {
      const response = await fetchActivities();

      expect(response.meta).toHaveProperty('total');
      expect(response.meta).toHaveProperty('page');
      expect(response.meta).toHaveProperty('page_size');
    });

    it('should handle empty results', async () => {
      server.use(
        http.get('/api/v1/activities', () => {
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          });
        }),
      );

      const response = await fetchActivities();

      expect(response.data).toHaveLength(0);
      expect(response.meta.total).toBe(0);
    });

    it('should pass page option in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 2, page_size: 20 },
          });
        }),
      );

      await fetchActivities({ page: 2 });

      expect(receivedUrl).toContain('page=2');
    });

    it('should pass page_size option in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 10 },
          });
        }),
      );

      await fetchActivities({ page_size: 10 });

      expect(receivedUrl).toContain('page_size=10');
    });

    it('should pass type option in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          });
        }),
      );

      await fetchActivities({ type: 'message_sent' });

      expect(receivedUrl).toContain('type=message_sent');
    });

    it('should pass agent_id option in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          });
        }),
      );

      await fetchActivities({ agent_id: 5 });

      expect(receivedUrl).toContain('agent_id=5');
    });

    it('should combine multiple options in query string', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 2, page_size: 10 },
          });
        }),
      );

      await fetchActivities({ page: 2, page_size: 10, type: 'heartbeat' });

      expect(receivedUrl).toContain('page=2');
      expect(receivedUrl).toContain('page_size=10');
      expect(receivedUrl).toContain('type=heartbeat');
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchActivities({}, controller.signal)).rejects.toThrow();
    });
  });

  describe('fetchAgentActivities', () => {
    it('should fetch activities for a specific agent', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            activities: [
              {
                id: '1',
                type: 'ACTIVITY_TYPE_MESSAGE_SENT',
                agent_id: '5',
                agent_name: 'Agent5',
                description: 'Sent a message',
                created_at: new Date().toISOString(),
              },
            ],
            total: '1',
            page: 1,
            page_size: 20,
          });
        }),
      );

      const response = await fetchAgentActivities(5);

      expect(receivedUrl).toContain('agent_id=5');
      expect(response.data).toHaveLength(1);
    });

    it('should pass additional options with agent_id', async () => {
      let receivedUrl = '';
      server.use(
        http.get('/api/v1/activities', ({ request }) => {
          receivedUrl = request.url;
          return HttpResponse.json({
            data: [],
            meta: { total: 0, page: 2, page_size: 10 },
          });
        }),
      );

      await fetchAgentActivities(5, { page: 2, page_size: 10 });

      expect(receivedUrl).toContain('agent_id=5');
      expect(receivedUrl).toContain('page=2');
      expect(receivedUrl).toContain('page_size=10');
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchAgentActivities(5, {}, controller.signal)).rejects.toThrow();
    });
  });
});
