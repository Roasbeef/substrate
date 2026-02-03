// Unit tests for sessions API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchActiveSessions,
  fetchSessions,
  fetchSession,
  startSession,
  completeSession,
} from '@/api/sessions.js';
import type { Session } from '@/types/api.js';

// Mock session data.
const mockSession: Session = {
  id: 1,
  agent_id: 1,
  agent_name: 'TestAgent',
  project: '/path/to/project',
  branch: 'main',
  started_at: new Date().toISOString(),
  status: 'active',
};

describe('sessions API', () => {
  describe('fetchActiveSessions', () => {
    it('should fetch active sessions', async () => {
      const response = await fetchActiveSessions();

      expect(response.data).toBeDefined();
      expect(Array.isArray(response.data)).toBe(true);
    });

    it('should only return active sessions', async () => {
      server.use(
        http.get('/api/v1/sessions/active', () => {
          return HttpResponse.json({
            data: [mockSession],
            meta: { total: 1, page: 1, page_size: 20 },
          });
        }),
      );

      const response = await fetchActiveSessions();

      response.data.forEach((session) => {
        expect(session.status).toBe('active');
      });
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchActiveSessions(controller.signal)).rejects.toThrow();
    });
  });

  describe('fetchSessions', () => {
    it('should fetch all sessions', async () => {
      const response = await fetchSessions();

      expect(response.data).toBeDefined();
      expect(Array.isArray(response.data)).toBe(true);
    });

    it('should include sessions of all statuses', async () => {
      server.use(
        http.get('/api/v1/sessions', () => {
          return HttpResponse.json({
            data: [
              { ...mockSession, id: 1, status: 'active' },
              { ...mockSession, id: 2, status: 'completed' },
              { ...mockSession, id: 3, status: 'abandoned' },
            ],
            meta: { total: 3, page: 1, page_size: 20 },
          });
        }),
      );

      const response = await fetchSessions();

      expect(response.data).toHaveLength(3);
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchSessions(controller.signal)).rejects.toThrow();
    });
  });

  describe('fetchSession', () => {
    it('should fetch a single session by ID', async () => {
      server.use(
        http.get('/api/v1/sessions/1', () => {
          return HttpResponse.json(mockSession);
        }),
      );

      const session = await fetchSession(1);

      expect(session.id).toBe(1);
      expect(session.agent_name).toBe('TestAgent');
    });

    it('should handle 404 for non-existent session', async () => {
      server.use(
        http.get('/api/v1/sessions/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Session not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(fetchSession(999)).rejects.toThrow();
    });

    it('should handle abort signal', async () => {
      server.use(
        http.get('/api/v1/sessions/1', () => {
          return HttpResponse.json(mockSession);
        }),
      );

      const controller = new AbortController();
      controller.abort();

      await expect(fetchSession(1, controller.signal)).rejects.toThrow();
    });
  });

  describe('startSession', () => {
    it('should start a new session', async () => {
      server.use(
        http.post('/api/v1/sessions', async ({ request }) => {
          const body = (await request.json()) as { project?: string; branch?: string };
          return HttpResponse.json({
            ...mockSession,
            id: 100,
            project: body.project,
            branch: body.branch,
          });
        }),
      );

      const session = await startSession({
        project: '/new/project',
        branch: 'feature',
      });

      expect(session.id).toBe(100);
      expect(session.project).toBe('/new/project');
      expect(session.branch).toBe('feature');
    });

    it('should start session with optional fields', async () => {
      server.use(
        http.post('/api/v1/sessions', () => {
          return HttpResponse.json({
            ...mockSession,
            id: 101,
            project: undefined,
            branch: undefined,
          });
        }),
      );

      const session = await startSession({});

      expect(session.id).toBe(101);
    });

    it('should handle errors', async () => {
      server.use(
        http.post('/api/v1/sessions', () => {
          return HttpResponse.json(
            { error: { code: 'already_active', message: 'Session already active' } },
            { status: 400 },
          );
        }),
      );

      await expect(startSession({})).rejects.toThrow();
    });
  });

  describe('completeSession', () => {
    it('should complete a session', async () => {
      server.use(
        http.post('/api/v1/sessions/1/complete', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(completeSession(1)).resolves.toBeUndefined();
    });

    it('should handle 404 for non-existent session', async () => {
      server.use(
        http.post('/api/v1/sessions/999/complete', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Session not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(completeSession(999)).rejects.toThrow();
    });

    it('should handle already completed session', async () => {
      server.use(
        http.post('/api/v1/sessions/1/complete', () => {
          return HttpResponse.json(
            { error: { code: 'already_completed', message: 'Session already completed' } },
            { status: 400 },
          );
        }),
      );

      await expect(completeSession(1)).rejects.toThrow();
    });
  });
});
