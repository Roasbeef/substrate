// MSW request handlers for API mocking.

import { http, HttpResponse } from 'msw';
import type {
  Activity,
  AgentsStatusResponse,
  DashboardStats,
  HealthResponse,
  MessageWithRecipients,
  Session,
  Thread,
  Topic,
} from '@/types/api.js';

const API_BASE = '/api/v1';

// Mock data factories.
export function createMockAgent(overrides: Partial<{ id: number; name: string }> = {}) {
  return {
    id: overrides.id ?? 1,
    name: overrides.name ?? 'TestAgent',
    status: 'active' as const,
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 0,
  };
}

export function createMockMessage(
  overrides: Partial<{
    id: number;
    subject: string;
    body: string;
    sender_name: string;
  }> = {},
): MessageWithRecipients {
  return {
    id: overrides.id ?? 1,
    sender_id: 1,
    sender_name: overrides.sender_name ?? 'SenderAgent',
    subject: overrides.subject ?? 'Test Subject',
    body: overrides.body ?? 'Test body content',
    priority: 'normal',
    created_at: new Date().toISOString(),
    recipients: [
      {
        message_id: overrides.id ?? 1,
        agent_id: 2,
        agent_name: 'RecipientAgent',
        state: 'unread',
        is_starred: false,
        is_archived: false,
      },
    ],
  };
}

export function createMockSession(
  overrides: Partial<{ id: number; agent_name: string; status: string }> = {},
): Session {
  return {
    id: overrides.id ?? 1,
    agent_id: 1,
    agent_name: overrides.agent_name ?? 'TestAgent',
    project: '/path/to/project',
    branch: 'main',
    started_at: new Date().toISOString(),
    status: (overrides.status as 'active') ?? 'active',
  };
}

// Default mock data.
const mockAgentsStatus: AgentsStatusResponse = {
  agents: [
    createMockAgent({ id: 1, name: 'Agent1' }),
    createMockAgent({ id: 2, name: 'Agent2' }),
  ],
  counts: {
    active: 2,
    busy: 0,
    idle: 0,
    offline: 0,
  },
};

const mockDashboardStats: DashboardStats = {
  active_agents: 2,
  running_sessions: 1,
  pending_messages: 5,
  completed_today: 10,
};

const mockMessages: MessageWithRecipients[] = [
  createMockMessage({ id: 1, subject: 'First message' }),
  createMockMessage({ id: 2, subject: 'Second message' }),
];

const mockTopics: Topic[] = [
  {
    id: 1,
    name: 'general',
    description: 'General discussion',
    created_at: new Date().toISOString(),
    message_count: 10,
  },
];

const mockSessions: Session[] = [
  createMockSession({ id: 1, agent_name: 'Agent1' }),
];

// Mock activities.
const mockActivities: Activity[] = [
  {
    id: 1,
    agent_id: 1,
    agent_name: 'Agent1',
    type: 'message_sent',
    description: 'Sent a message to Agent2',
    created_at: new Date(Date.now() - 60000).toISOString(),
  },
  {
    id: 2,
    agent_id: 2,
    agent_name: 'Agent2',
    type: 'session_started',
    description: 'Started a new session',
    created_at: new Date(Date.now() - 120000).toISOString(),
    metadata: { project: '/path/to/project' },
  },
  {
    id: 3,
    agent_id: 1,
    agent_name: 'Agent1',
    type: 'heartbeat',
    description: 'Agent heartbeat',
    created_at: new Date(Date.now() - 180000).toISOString(),
  },
];

// Request handlers.
export const handlers = [
  // Health check.
  http.get(`${API_BASE}/health`, () => {
    const response: HealthResponse = {
      status: 'ok',
      time: new Date().toISOString(),
    };
    return HttpResponse.json(response);
  }),

  // Agents status.
  http.get(`${API_BASE}/agents/status`, () => {
    return HttpResponse.json(mockAgentsStatus);
  }),

  // Dashboard stats.
  http.get(`${API_BASE}/stats/dashboard`, () => {
    return HttpResponse.json({ data: mockDashboardStats });
  }),

  // Messages.
  http.get(`${API_BASE}/messages`, () => {
    return HttpResponse.json({
      data: mockMessages,
      meta: { total: mockMessages.length, page: 1, page_size: 20 },
    });
  }),

  http.get(`${API_BASE}/messages/:id`, ({ params }) => {
    const id = Number(params.id);
    const message = mockMessages.find((m) => m.id === id);
    if (!message) {
      return HttpResponse.json(
        { error: { code: 'not_found', message: 'Message not found' } },
        { status: 404 },
      );
    }
    return HttpResponse.json(message);
  }),

  http.post(`${API_BASE}/messages`, async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const newMessage = createMockMessage({
      id: mockMessages.length + 1,
      subject: String(body.subject ?? ''),
      body: String(body.body ?? ''),
    });
    return HttpResponse.json(newMessage, { status: 201 });
  }),

  http.post(`${API_BASE}/messages/:id/star`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/messages/:id/archive`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/messages/:id/unarchive`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/messages/:id/snooze`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/messages/:id/ack`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.patch(`${API_BASE}/messages/:id`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Agents CRUD.
  http.get(`${API_BASE}/agents/:id`, ({ params }) => {
    const id = Number(params.id);
    const agent = mockAgentsStatus.agents.find((a) => a.id === id);
    if (!agent) {
      return HttpResponse.json(
        { error: { code: 'not_found', message: 'Agent not found' } },
        { status: 404 },
      );
    }
    return HttpResponse.json({
      id: agent.id,
      name: agent.name,
      created_at: agent.last_active_at,
      last_active_at: agent.last_active_at,
    });
  }),

  http.post(`${API_BASE}/agents`, async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const newAgent = {
      id: mockAgentsStatus.agents.length + 1,
      name: String(body.name ?? 'NewAgent'),
      created_at: new Date().toISOString(),
      last_active_at: new Date().toISOString(),
    };
    return HttpResponse.json(newAgent, { status: 201 });
  }),

  http.patch(`${API_BASE}/agents/:id`, async ({ params, request }) => {
    const id = Number(params.id);
    const body = (await request.json()) as Record<string, unknown>;
    const agent = mockAgentsStatus.agents.find((a) => a.id === id);
    if (!agent) {
      return HttpResponse.json(
        { error: { code: 'not_found', message: 'Agent not found' } },
        { status: 404 },
      );
    }
    return HttpResponse.json({
      id: agent.id,
      name: String(body.name ?? agent.name),
      created_at: agent.last_active_at,
      last_active_at: new Date().toISOString(),
    });
  }),

  // Topics.
  http.get(`${API_BASE}/topics`, () => {
    return HttpResponse.json({
      data: mockTopics,
      meta: { total: mockTopics.length, page: 1, page_size: 20 },
    });
  }),

  // Sessions.
  http.get(`${API_BASE}/sessions/active`, () => {
    return HttpResponse.json({
      data: mockSessions.filter((s) => s.status === 'active'),
      meta: { total: 1, page: 1, page_size: 20 },
    });
  }),

  http.get(`${API_BASE}/sessions`, () => {
    return HttpResponse.json({
      data: mockSessions,
      meta: { total: mockSessions.length, page: 1, page_size: 20 },
    });
  }),

  http.post(`${API_BASE}/sessions`, async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const newSession = createMockSession({
      id: mockSessions.length + 1,
      agent_name: 'TestAgent',
    });
    newSession.project = String(body.project ?? '');
    newSession.branch = String(body.branch ?? '');
    return HttpResponse.json(newSession, { status: 201 });
  }),

  http.get(`${API_BASE}/sessions/:id`, ({ params }) => {
    const id = Number(params.id);
    const session = mockSessions.find((s) => s.id === id);
    if (!session) {
      return HttpResponse.json(
        { error: { code: 'not_found', message: 'Session not found' } },
        { status: 404 },
      );
    }
    return HttpResponse.json(session);
  }),

  http.post(`${API_BASE}/sessions/:id/complete`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Threads.
  http.get(`${API_BASE}/threads/:id`, ({ params }) => {
    const id = Number(params.id);
    return HttpResponse.json({
      id,
      subject: 'Test Thread',
      created_at: new Date().toISOString(),
      last_message_at: new Date().toISOString(),
      message_count: 2,
      participant_count: 2,
      messages: mockMessages,
    });
  }),

  http.post(`${API_BASE}/threads/:id/reply`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/threads/:id/archive`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/threads/:id/unread`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/threads/:id/delete`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.post(`${API_BASE}/threads/:id/unarchive`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Messages delete.
  http.post(`${API_BASE}/messages/:id/delete`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Heartbeat.
  http.post(`${API_BASE}/heartbeat`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Search.
  http.get(`${API_BASE}/search`, ({ request }) => {
    const url = new URL(request.url);
    const query = url.searchParams.get('q') ?? '';
    const results = mockMessages
      .filter((m) =>
        m.subject.toLowerCase().includes(query.toLowerCase()),
      )
      .map((m) => ({
        type: 'message' as const,
        id: m.id,
        title: m.subject,
        snippet: m.body.substring(0, 100),
        created_at: m.created_at,
      }));
    return HttpResponse.json({ data: results });
  }),

  // Autocomplete.
  http.get(`${API_BASE}/autocomplete/recipients`, ({ request }) => {
    const url = new URL(request.url);
    const query = url.searchParams.get('q') ?? '';
    const results = mockAgentsStatus.agents
      .filter((a) =>
        a.name.toLowerCase().includes(query.toLowerCase()),
      )
      .map((a) => ({
        id: a.id,
        name: a.name,
        status: a.status,
      }));
    return HttpResponse.json(results);
  }),

  // Activities.
  http.get(`${API_BASE}/activities`, ({ request }) => {
    const url = new URL(request.url);
    const agentId = url.searchParams.get('agent_id');
    const type = url.searchParams.get('type');
    const page = Number(url.searchParams.get('page') ?? '1');
    const pageSize = Number(url.searchParams.get('page_size') ?? '20');

    let filtered = [...mockActivities];

    if (agentId) {
      filtered = filtered.filter((a) => a.agent_id === Number(agentId));
    }
    if (type) {
      filtered = filtered.filter((a) => a.type === type);
    }

    const start = (page - 1) * pageSize;
    const paginated = filtered.slice(start, start + pageSize);

    return HttpResponse.json({
      data: paginated,
      meta: { total: filtered.length, page, page_size: pageSize },
    });
  }),
];
