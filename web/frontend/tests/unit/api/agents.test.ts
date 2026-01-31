// Unit tests for agents API functions.

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchAgentsStatus,
  fetchAgent,
  createAgent,
  updateAgent,
  sendHeartbeat,
} from '@/api/agents.js';
import type { Agent, AgentsStatusResponse } from '@/types/api.js';

describe('agents API', () => {
  describe('fetchAgentsStatus', () => {
    it('should fetch agents with their status', async () => {
      const response = await fetchAgentsStatus();

      expect(response.agents).toBeDefined();
      expect(Array.isArray(response.agents)).toBe(true);
      expect(response.counts).toBeDefined();
    });

    it('should return counts by status', async () => {
      const response = await fetchAgentsStatus();

      expect(response.counts).toHaveProperty('active');
      expect(response.counts).toHaveProperty('busy');
      expect(response.counts).toHaveProperty('idle');
      expect(response.counts).toHaveProperty('offline');
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchAgentsStatus(controller.signal)).rejects.toThrow();
    });
  });

  describe('fetchAgent', () => {
    it('should fetch a single agent by ID', async () => {
      server.use(
        http.get('/api/v1/agents/1', () => {
          return HttpResponse.json({
            id: 1,
            name: 'TestAgent',
            created_at: '2024-01-01T00:00:00Z',
          });
        }),
      );

      const agent = await fetchAgent(1);

      expect(agent.id).toBe(1);
      expect(agent.name).toBe('TestAgent');
    });

    it('should handle 404 for non-existent agent', async () => {
      server.use(
        http.get('/api/v1/agents/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Agent not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(fetchAgent(999)).rejects.toThrow();
    });

    it('should handle abort signal', async () => {
      server.use(
        http.get('/api/v1/agents/1', () => {
          return HttpResponse.json({
            id: 1,
            name: 'TestAgent',
            created_at: '2024-01-01T00:00:00Z',
          });
        }),
      );

      const controller = new AbortController();
      controller.abort();

      await expect(fetchAgent(1, controller.signal)).rejects.toThrow();
    });
  });

  describe('createAgent', () => {
    it('should create a new agent', async () => {
      server.use(
        http.post('/api/v1/agents', async ({ request }) => {
          const body = (await request.json()) as { name: string };
          return HttpResponse.json({
            id: 100,
            name: body.name,
            created_at: new Date().toISOString(),
          });
        }),
      );

      const agent = await createAgent({ name: 'NewAgent' });

      expect(agent.id).toBe(100);
      expect(agent.name).toBe('NewAgent');
    });

    it('should handle validation errors', async () => {
      server.use(
        http.post('/api/v1/agents', () => {
          return HttpResponse.json(
            { error: { code: 'validation_error', message: 'Name is required' } },
            { status: 400 },
          );
        }),
      );

      await expect(createAgent({ name: '' })).rejects.toThrow();
    });

    it('should handle duplicate name errors', async () => {
      server.use(
        http.post('/api/v1/agents', () => {
          return HttpResponse.json(
            { error: { code: 'conflict', message: 'Agent already exists' } },
            { status: 409 },
          );
        }),
      );

      await expect(createAgent({ name: 'ExistingAgent' })).rejects.toThrow();
    });
  });

  describe('updateAgent', () => {
    it('should update an existing agent', async () => {
      server.use(
        http.patch('/api/v1/agents/1', async ({ request }) => {
          const body = (await request.json()) as { name: string };
          return HttpResponse.json({
            id: 1,
            name: body.name,
            created_at: '2024-01-01T00:00:00Z',
          });
        }),
      );

      const agent = await updateAgent(1, { name: 'UpdatedName' });

      expect(agent.id).toBe(1);
      expect(agent.name).toBe('UpdatedName');
    });

    it('should handle 404 for non-existent agent', async () => {
      server.use(
        http.patch('/api/v1/agents/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Agent not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(updateAgent(999, { name: 'Test' })).rejects.toThrow();
    });
  });

  describe('sendHeartbeat', () => {
    it('should send a heartbeat', async () => {
      server.use(
        http.post('/api/v1/heartbeat', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      await expect(sendHeartbeat({ agent_name: 'TestAgent' })).resolves.toBeUndefined();
    });

    it('should handle errors', async () => {
      server.use(
        http.post('/api/v1/heartbeat', () => {
          return HttpResponse.json(
            { error: { code: 'invalid_agent', message: 'Agent not found' } },
            { status: 400 },
          );
        }),
      );

      await expect(sendHeartbeat({ agent_name: 'InvalidAgent' })).rejects.toThrow();
    });
  });
});
