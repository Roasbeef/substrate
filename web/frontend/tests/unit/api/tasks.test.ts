// Unit tests for tasks API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchTasks,
  fetchTask,
  fetchTaskLists,
  fetchTaskStats,
  fetchAgentTaskStats,
  updateTaskStatus,
  updateTaskOwner,
  syncTaskList,
  statusToDisplay,
  displayToStatus,
} from '@/api/tasks.js';

describe('tasks API', () => {
  describe('fetchTasks', () => {
    it('should fetch all tasks with default options', async () => {
      const tasks = await fetchTasks();

      expect(Array.isArray(tasks)).toBe(true);
      expect(tasks.length).toBeGreaterThan(0);
      expect(tasks[0]).toHaveProperty('id');
      expect(tasks[0]).toHaveProperty('list_id');
      expect(tasks[0]).toHaveProperty('claude_task_id');
      expect(tasks[0]).toHaveProperty('subject');
      expect(tasks[0]).toHaveProperty('status');
    });

    it('should filter tasks by agent ID', async () => {
      server.use(
        http.get('/api/v1/tasks', ({ request }) => {
          const url = new URL(request.url);
          const agentId = url.searchParams.get('agent_id');
          expect(agentId).toBe('5');
          return HttpResponse.json({
            tasks: [
              {
                id: '10',
                agent_id: '5',
                list_id: 'list-xyz',
                claude_task_id: 'task-filtered',
                subject: 'Filtered task',
                status: 'TASK_STATUS_PENDING',
                blocked_by: [],
                blocks: [],
                created_at: '2026-01-20T10:00:00Z',
                updated_at: '2026-01-20T10:00:00Z',
              },
            ],
          });
        }),
      );

      const tasks = await fetchTasks({ agentId: 5 });

      expect(tasks).toHaveLength(1);
      expect(tasks[0].agent_id).toBe(5);
      expect(tasks[0].subject).toBe('Filtered task');
    });

    it('should filter tasks by status', async () => {
      server.use(
        http.get('/api/v1/tasks', ({ request }) => {
          const url = new URL(request.url);
          const status = url.searchParams.get('status');
          expect(status).toBe('TASK_STATUS_IN_PROGRESS');
          return HttpResponse.json({
            tasks: [
              {
                id: '2',
                agent_id: '2',
                list_id: 'list-abc',
                claude_task_id: 'task-002',
                subject: 'In progress task',
                status: 'TASK_STATUS_IN_PROGRESS',
                owner: 'Agent1',
                blocked_by: [],
                blocks: [],
                created_at: '2026-01-15T10:00:00Z',
                updated_at: '2026-01-15T12:00:00Z',
              },
            ],
          });
        }),
      );

      const tasks = await fetchTasks({ status: 'in_progress' });

      expect(tasks).toHaveLength(1);
      expect(tasks[0].status).toBe('in_progress');
    });

    it('should build query string with all filter options', async () => {
      server.use(
        http.get('/api/v1/tasks', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('agent_id')).toBe('3');
          expect(url.searchParams.get('list_id')).toBe('list-def');
          expect(url.searchParams.get('active_only')).toBe('true');
          expect(url.searchParams.get('available_only')).toBe('true');
          expect(url.searchParams.get('limit')).toBe('10');
          expect(url.searchParams.get('offset')).toBe('20');
          return HttpResponse.json({ tasks: [] });
        }),
      );

      await fetchTasks({
        agentId: 3,
        listId: 'list-def',
        activeOnly: true,
        availableOnly: true,
        limit: 10,
        offset: 20,
      });
    });

    it('should handle empty response', async () => {
      server.use(
        http.get('/api/v1/tasks', () => {
          return HttpResponse.json({ tasks: [] });
        }),
      );

      const tasks = await fetchTasks();

      expect(tasks).toHaveLength(0);
    });

    it('should handle missing tasks field', async () => {
      server.use(
        http.get('/api/v1/tasks', () => {
          return HttpResponse.json({});
        }),
      );

      const tasks = await fetchTasks();

      expect(tasks).toHaveLength(0);
    });

    it('should parse proto int64 string fields to numbers', async () => {
      server.use(
        http.get('/api/v1/tasks', () => {
          return HttpResponse.json({
            tasks: [
              {
                id: '42',
                agent_id: '7',
                list_id: 'list-parse',
                claude_task_id: 'task-parse',
                subject: 'Parse test',
                status: 'TASK_STATUS_COMPLETED',
                blocked_by: ['task-a', 'task-b'],
                blocks: ['task-c'],
                created_at: '2026-02-01T08:00:00Z',
                updated_at: '2026-02-01T09:00:00Z',
                started_at: '2026-02-01T08:30:00Z',
                completed_at: '2026-02-01T09:00:00Z',
              },
            ],
          });
        }),
      );

      const tasks = await fetchTasks();

      expect(tasks).toHaveLength(1);
      const task = tasks[0];
      expect(task.id).toBe(42);
      expect(task.agent_id).toBe(7);
      expect(task.list_id).toBe('list-parse');
      expect(task.claude_task_id).toBe('task-parse');
      expect(task.status).toBe('completed');
      expect(task.blocked_by).toEqual(['task-a', 'task-b']);
      expect(task.blocks).toEqual(['task-c']);
      expect(task.created_at).toBeInstanceOf(Date);
      expect(task.updated_at).toBeInstanceOf(Date);
      expect(task.started_at).toBeInstanceOf(Date);
      expect(task.completed_at).toBeInstanceOf(Date);
    });

    it('should handle missing optional fields with defaults', async () => {
      server.use(
        http.get('/api/v1/tasks', () => {
          return HttpResponse.json({
            tasks: [
              {
                id: '1',
              },
            ],
          });
        }),
      );

      const tasks = await fetchTasks();

      expect(tasks).toHaveLength(1);
      const task = tasks[0];
      expect(task.id).toBe(1);
      expect(task.agent_id).toBe(0);
      expect(task.list_id).toBe('');
      expect(task.claude_task_id).toBe('');
      expect(task.subject).toBe('');
      expect(task.description).toBe('');
      expect(task.status).toBe('unknown');
      expect(task.owner).toBe('');
      expect(task.blocked_by).toEqual([]);
      expect(task.blocks).toEqual([]);
      expect(task.created_at).toBeNull();
      expect(task.updated_at).toBeNull();
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchTasks({}, controller.signal)).rejects.toThrow();
    });
  });

  describe('fetchTask', () => {
    it('should fetch a single task by list and claude task IDs', async () => {
      const task = await fetchTask('list-abc', 'task-001');

      expect(task.list_id).toBe('list-abc');
      expect(task.claude_task_id).toBe('task-001');
      expect(task.subject).toBe('Implement feature X');
      expect(task.status).toBe('pending');
    });

    it('should handle 404 for non-existent task', async () => {
      await expect(
        fetchTask('nonexistent', 'nonexistent'),
      ).rejects.toThrow();
    });

    it('should parse all task fields from gateway response', async () => {
      server.use(
        http.get('/api/v1/tasks/:listId/:claudeTaskId', () => {
          return HttpResponse.json({
            task: {
              id: '99',
              agent_id: '4',
              list_id: 'list-full',
              claude_task_id: 'task-full',
              subject: 'Full parse test',
              description: 'A complete task for testing',
              active_form: 'some-form',
              metadata_json: '{"key":"value"}',
              status: 'TASK_STATUS_IN_PROGRESS',
              owner: 'AgentX',
              blocked_by: ['dep-1'],
              blocks: ['dep-2', 'dep-3'],
              created_at: '2026-01-10T00:00:00Z',
              updated_at: '2026-01-11T00:00:00Z',
              started_at: '2026-01-10T01:00:00Z',
              completed_at: '',
            },
          });
        }),
      );

      const task = await fetchTask('list-full', 'task-full');

      expect(task.id).toBe(99);
      expect(task.agent_id).toBe(4);
      expect(task.list_id).toBe('list-full');
      expect(task.subject).toBe('Full parse test');
      expect(task.description).toBe('A complete task for testing');
      expect(task.active_form).toBe('some-form');
      expect(task.metadata_json).toBe('{"key":"value"}');
      expect(task.status).toBe('in_progress');
      expect(task.owner).toBe('AgentX');
      expect(task.blocked_by).toEqual(['dep-1']);
      expect(task.blocks).toEqual(['dep-2', 'dep-3']);
      expect(task.created_at).toBeInstanceOf(Date);
      expect(task.started_at).toBeInstanceOf(Date);
      expect(task.completed_at).toBeNull();
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(
        fetchTask('list-abc', 'task-001', controller.signal),
      ).rejects.toThrow();
    });
  });

  describe('fetchTaskLists', () => {
    it('should fetch all task lists without filters', async () => {
      const lists = await fetchTaskLists();

      expect(Array.isArray(lists)).toBe(true);
      expect(lists.length).toBeGreaterThan(0);
      expect(lists[0]).toHaveProperty('id');
      expect(lists[0]).toHaveProperty('list_id');
      expect(lists[0]).toHaveProperty('agent_id');
      expect(lists[0]).toHaveProperty('watch_path');
    });

    it('should filter task lists by agent ID', async () => {
      server.use(
        http.get('/api/v1/task-lists', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('agent_id')).toBe('3');
          return HttpResponse.json({
            task_lists: [
              {
                id: '2',
                list_id: 'list-def',
                agent_id: '3',
                watch_path: '/home/user/.tasks/list-def.md',
                created_at: '2026-01-15T09:00:00Z',
                last_synced_at: '2026-01-15T12:00:00Z',
              },
            ],
          });
        }),
      );

      const lists = await fetchTaskLists(3);

      expect(lists).toHaveLength(1);
      expect(lists[0].agent_id).toBe(3);
      expect(lists[0].list_id).toBe('list-def');
    });

    it('should parse task list fields correctly', async () => {
      const lists = await fetchTaskLists();

      expect(lists[0].id).toBeTypeOf('number');
      expect(lists[0].agent_id).toBeTypeOf('number');
      expect(lists[0].list_id).toBeTypeOf('string');
      expect(lists[0].watch_path).toBeTypeOf('string');
      expect(lists[0].created_at).toBeInstanceOf(Date);
      expect(lists[0].last_synced_at).toBeInstanceOf(Date);
    });

    it('should handle empty response', async () => {
      server.use(
        http.get('/api/v1/task-lists', () => {
          return HttpResponse.json({ task_lists: [] });
        }),
      );

      const lists = await fetchTaskLists();

      expect(lists).toHaveLength(0);
    });

    it('should handle missing task_lists field', async () => {
      server.use(
        http.get('/api/v1/task-lists', () => {
          return HttpResponse.json({});
        }),
      );

      const lists = await fetchTaskLists();

      expect(lists).toHaveLength(0);
    });
  });

  describe('fetchTaskStats', () => {
    it('should fetch task stats with defaults', async () => {
      const stats = await fetchTaskStats();

      expect(stats.pending_count).toBe(5);
      expect(stats.in_progress_count).toBe(3);
      expect(stats.completed_count).toBe(12);
      expect(stats.blocked_count).toBe(1);
      expect(stats.available_count).toBe(4);
      expect(stats.completed_today).toBe(2);
    });

    it('should pass agent_id filter', async () => {
      server.use(
        http.get('/api/v1/tasks/stats', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('agent_id')).toBe('5');
          return HttpResponse.json({
            stats: {
              pending_count: '2',
              in_progress_count: '1',
              completed_count: '8',
              blocked_count: '0',
              available_count: '2',
              completed_today: '1',
            },
          });
        }),
      );

      const stats = await fetchTaskStats(5);

      expect(stats.pending_count).toBe(2);
      expect(stats.in_progress_count).toBe(1);
    });

    it('should pass list_id filter', async () => {
      server.use(
        http.get('/api/v1/tasks/stats', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('list_id')).toBe('list-abc');
          return HttpResponse.json({
            stats: {
              pending_count: '1',
              in_progress_count: '1',
              completed_count: '3',
              blocked_count: '0',
              available_count: '1',
              completed_today: '0',
            },
          });
        }),
      );

      const stats = await fetchTaskStats(undefined, 'list-abc');

      expect(stats.pending_count).toBe(1);
      expect(stats.completed_count).toBe(3);
    });

    it('should handle missing stats field', async () => {
      server.use(
        http.get('/api/v1/tasks/stats', () => {
          return HttpResponse.json({});
        }),
      );

      const stats = await fetchTaskStats();

      expect(stats.pending_count).toBe(0);
      expect(stats.in_progress_count).toBe(0);
      expect(stats.completed_count).toBe(0);
      expect(stats.blocked_count).toBe(0);
      expect(stats.available_count).toBe(0);
      expect(stats.completed_today).toBe(0);
    });

    it('should include today_since parameter', async () => {
      server.use(
        http.get('/api/v1/tasks/stats', ({ request }) => {
          const url = new URL(request.url);
          const todaySince = url.searchParams.get('today_since');
          expect(todaySince).toBeTruthy();
          // Should be a valid ISO date string.
          expect(new Date(todaySince!).getTime()).not.toBeNaN();
          return HttpResponse.json({
            stats: {
              pending_count: '0',
              in_progress_count: '0',
              completed_count: '0',
              blocked_count: '0',
              available_count: '0',
              completed_today: '0',
            },
          });
        }),
      );

      await fetchTaskStats();
    });
  });

  describe('fetchAgentTaskStats', () => {
    it('should fetch task stats grouped by agent', async () => {
      const stats = await fetchAgentTaskStats();

      expect(Array.isArray(stats)).toBe(true);
      expect(stats.length).toBe(2);

      expect(stats[0].agent_id).toBe(2);
      expect(stats[0].agent_name).toBe('Agent1');
      expect(stats[0].pending_count).toBe(3);
      expect(stats[0].in_progress_count).toBe(2);
      expect(stats[0].blocked_count).toBe(0);
      expect(stats[0].completed_today).toBe(1);

      expect(stats[1].agent_id).toBe(3);
      expect(stats[1].agent_name).toBe('Agent2');
      expect(stats[1].pending_count).toBe(2);
      expect(stats[1].blocked_count).toBe(1);
    });

    it('should handle empty stats', async () => {
      server.use(
        http.get('/api/v1/tasks/stats/by-agent', () => {
          return HttpResponse.json({ stats: [] });
        }),
      );

      const stats = await fetchAgentTaskStats();

      expect(stats).toHaveLength(0);
    });

    it('should handle missing stats field', async () => {
      server.use(
        http.get('/api/v1/tasks/stats/by-agent', () => {
          return HttpResponse.json({});
        }),
      );

      const stats = await fetchAgentTaskStats();

      expect(stats).toHaveLength(0);
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(
        fetchAgentTaskStats(controller.signal),
      ).rejects.toThrow();
    });
  });

  describe('statusToDisplay', () => {
    it('should map TASK_STATUS_PENDING to pending', () => {
      expect(statusToDisplay('TASK_STATUS_PENDING')).toBe('pending');
    });

    it('should map TASK_STATUS_IN_PROGRESS to in_progress', () => {
      expect(statusToDisplay('TASK_STATUS_IN_PROGRESS')).toBe('in_progress');
    });

    it('should map TASK_STATUS_COMPLETED to completed', () => {
      expect(statusToDisplay('TASK_STATUS_COMPLETED')).toBe('completed');
    });

    it('should map TASK_STATUS_DELETED to deleted', () => {
      expect(statusToDisplay('TASK_STATUS_DELETED')).toBe('deleted');
    });

    it('should map TASK_STATUS_UNSPECIFIED to unknown', () => {
      expect(statusToDisplay('TASK_STATUS_UNSPECIFIED')).toBe('unknown');
    });
  });

  describe('displayToStatus', () => {
    it('should map pending to TASK_STATUS_PENDING', () => {
      expect(displayToStatus('pending')).toBe('TASK_STATUS_PENDING');
    });

    it('should map in_progress to TASK_STATUS_IN_PROGRESS', () => {
      expect(displayToStatus('in_progress')).toBe('TASK_STATUS_IN_PROGRESS');
    });

    it('should map completed to TASK_STATUS_COMPLETED', () => {
      expect(displayToStatus('completed')).toBe('TASK_STATUS_COMPLETED');
    });

    it('should map deleted to TASK_STATUS_DELETED', () => {
      expect(displayToStatus('deleted')).toBe('TASK_STATUS_DELETED');
    });

    it('should map unknown string to TASK_STATUS_UNSPECIFIED', () => {
      expect(displayToStatus('bogus')).toBe('TASK_STATUS_UNSPECIFIED');
    });
  });

  describe('statusToDisplay / displayToStatus roundtrip', () => {
    it('should roundtrip all known statuses', () => {
      const statuses = ['pending', 'in_progress', 'completed', 'deleted'] as const;

      for (const display of statuses) {
        const proto = displayToStatus(display);
        const back = statusToDisplay(proto);
        expect(back).toBe(display);
      }
    });
  });

  describe('updateTaskStatus', () => {
    it('should send PATCH request with status', async () => {
      server.use(
        http.patch(
          '/api/v1/tasks/:listId/:claudeTaskId/status',
          async ({ params, request }) => {
            expect(params.listId).toBe('list-abc');
            expect(params.claudeTaskId).toBe('task-001');
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.status).toBe('TASK_STATUS_COMPLETED');
            return HttpResponse.json({});
          },
        ),
      );

      await expect(
        updateTaskStatus('list-abc', 'task-001', 'completed'),
      ).resolves.not.toThrow();
    });

    it('should handle error response', async () => {
      server.use(
        http.patch('/api/v1/tasks/:listId/:claudeTaskId/status', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Task not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(
        updateTaskStatus('bad-list', 'bad-task', 'completed'),
      ).rejects.toThrow();
    });
  });

  describe('updateTaskOwner', () => {
    it('should send PATCH request with owner', async () => {
      server.use(
        http.patch(
          '/api/v1/tasks/:listId/:claudeTaskId/owner',
          async ({ params, request }) => {
            expect(params.listId).toBe('list-abc');
            expect(params.claudeTaskId).toBe('task-002');
            const body = (await request.json()) as Record<string, unknown>;
            expect(body.owner).toBe('NewAgent');
            return HttpResponse.json({});
          },
        ),
      );

      await expect(
        updateTaskOwner('list-abc', 'task-002', 'NewAgent'),
      ).resolves.not.toThrow();
    });

    it('should handle error response', async () => {
      server.use(
        http.patch('/api/v1/tasks/:listId/:claudeTaskId/owner', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Task not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(
        updateTaskOwner('bad-list', 'bad-task', 'Agent'),
      ).rejects.toThrow();
    });
  });

  describe('syncTaskList', () => {
    it('should send POST request and return sync result', async () => {
      const result = await syncTaskList('list-abc');

      expect(result.tasks_updated).toBe(3);
      expect(result.tasks_deleted).toBe(1);
    });

    it('should handle error response', async () => {
      server.use(
        http.post('/api/v1/task-lists/:listId/sync', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Task list not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(syncTaskList('nonexistent')).rejects.toThrow();
    });

    it('should use the correct list ID in the URL', async () => {
      server.use(
        http.post('/api/v1/task-lists/:listId/sync', ({ params }) => {
          expect(params.listId).toBe('my-special-list');
          return HttpResponse.json({
            tasks_updated: 0,
            tasks_deleted: 0,
          });
        }),
      );

      const result = await syncTaskList('my-special-list');

      expect(result.tasks_updated).toBe(0);
      expect(result.tasks_deleted).toBe(0);
    });
  });
});
