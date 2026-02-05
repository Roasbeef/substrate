// MSW request handlers for API mocking.

import { http, HttpResponse } from 'msw';
import type {
  Activity,
  AgentsStatusResponse,
  DashboardStats,
  HealthResponse,
  MessageWithRecipients,
  ReviewIssue,
  ReviewSummary,
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

// Mock reviews.
const mockReviews: ReviewSummary[] = [
  {
    review_id: 'abc123',
    thread_id: 'thread-1',
    requester_id: 1,
    branch: 'feature/add-reviews',
    state: 'under_review',
    review_type: 'full',
    created_at: Math.floor(Date.now() / 1000) - 3600,
  },
  {
    review_id: 'def456',
    thread_id: 'thread-2',
    requester_id: 1,
    branch: 'fix/null-pointer',
    state: 'approved',
    review_type: 'incremental',
    created_at: Math.floor(Date.now() / 1000) - 7200,
  },
];

const mockReviewIssues: ReviewIssue[] = [
  {
    id: 1,
    review_id: 'abc123',
    iteration_num: 1,
    issue_type: 'bug',
    severity: 'major',
    file_path: 'internal/review/service.go',
    line_start: 42,
    line_end: 50,
    title: 'Missing nil check before dereference',
    description: 'The pointer could be nil when the review is not found.',
    code_snippet: 'review := s.reviews[id]\nreview.State = "approved"',
    suggestion: 'Add a nil check: if review == nil { return ErrNotFound }',
    claude_md_ref: '',
    status: 'open',
  },
  {
    id: 2,
    review_id: 'abc123',
    iteration_num: 1,
    issue_type: 'style',
    severity: 'suggestion',
    file_path: 'internal/review/fsm.go',
    line_start: 15,
    line_end: 15,
    title: 'Missing function comment',
    description: 'All exported functions should have comments.',
    code_snippet: 'func NewFSM() *FSM {',
    suggestion: '// NewFSM creates a new review state machine.',
    claude_md_ref: 'Code Style: Function and Method Comments',
    status: 'fixed',
  },
];

export function createMockReview(
  overrides: Partial<ReviewSummary> = {},
): ReviewSummary {
  return {
    review_id: overrides.review_id ?? 'test-review-1',
    thread_id: overrides.thread_id ?? 'thread-1',
    requester_id: overrides.requester_id ?? 1,
    branch: overrides.branch ?? 'test-branch',
    state: overrides.state ?? 'under_review',
    review_type: overrides.review_type ?? 'full',
    created_at: overrides.created_at ?? Math.floor(Date.now() / 1000),
  };
}

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

  // Agents status (grpc-gateway uses hyphen path).
  http.get(`${API_BASE}/agents-status`, () => {
    return HttpResponse.json(mockAgentsStatus);
  }),

  // Legacy agents status endpoint (with slash).
  http.get(`${API_BASE}/agents/status`, () => {
    return HttpResponse.json(mockAgentsStatus);
  }),

  // Dashboard stats.
  http.get(`${API_BASE}/stats/dashboard`, () => {
    return HttpResponse.json({ data: mockDashboardStats });
  }),

  // Messages (gateway format with flat state fields).
  http.get(`${API_BASE}/messages`, () => {
    return HttpResponse.json({
      messages: mockMessages.map((m) => ({
        id: String(m.id),
        sender_id: String(m.sender_id),
        sender_name: m.sender_name,
        sender_project_key: m.sender_project_key,
        sender_git_branch: m.sender_git_branch,
        subject: m.subject,
        body: m.body,
        priority: `PRIORITY_${m.priority.toUpperCase()}`,
        state: m.recipients[0]?.state === 'unread' ? 'STATE_UNREAD' : 'STATE_READ',
        created_at: m.created_at,
        thread_id: m.thread_id,
      })),
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

  // UpdateState via PATCH (star, archive, read, snooze, etc.).
  http.patch(`${API_BASE}/messages/:id`, () => {
    return HttpResponse.json({ success: true });
  }),

  // Ack still uses its own POST endpoint.
  http.post(`${API_BASE}/messages/:id/ack`, () => {
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
  // Note: API uses /sessions?active_only=true for active sessions.
  http.get(`${API_BASE}/sessions`, ({ request }) => {
    const url = new URL(request.url);
    const activeOnly = url.searchParams.get('active_only') === 'true';
    const sessions = activeOnly
      ? mockSessions.filter((s) => s.status === 'active')
      : mockSessions;
    // Return in grpc-gateway format.
    return HttpResponse.json({
      sessions: sessions.map((s) => ({
        id: String(s.id),
        agent_id: String(s.agent_id),
        agent_name: s.agent_name,
        project: s.project,
        branch: s.branch,
        started_at: s.started_at,
        status: s.status?.toUpperCase(),
      })),
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
    // Return in grpc-gateway format.
    return HttpResponse.json({
      session: {
        id: String(newSession.id),
        agent_id: String(newSession.agent_id),
        agent_name: newSession.agent_name,
        project: newSession.project,
        branch: newSession.branch,
        started_at: newSession.started_at,
        status: newSession.status?.toUpperCase(),
      },
    }, { status: 201 });
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
    // Return in grpc-gateway format.
    return HttpResponse.json({
      session: {
        id: String(session.id),
        agent_id: String(session.agent_id),
        agent_name: session.agent_name,
        project: session.project,
        branch: session.branch,
        started_at: session.started_at,
        status: session.status?.toUpperCase(),
      },
    });
  }),

  http.post(`${API_BASE}/sessions/:id/complete`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Threads - returns messages in grpc-gateway format.
  http.get(`${API_BASE}/threads/:id`, ({ params }) => {
    const id = String(params.id);
    // Return in grpc-gateway format - subject comes from first message.
    return HttpResponse.json({
      messages: [
        {
          id: '1',
          thread_id: id,
          sender_id: '1',
          sender_name: 'SenderAgent',
          subject: 'Test Thread',
          body: 'First message body',
          priority: 'PRIORITY_NORMAL',
          created_at: new Date().toISOString(),
        },
        {
          id: '2',
          thread_id: id,
          sender_id: '2',
          sender_name: 'RecipientAgent',
          subject: 'Re: Test Thread',
          body: 'Reply body',
          priority: 'PRIORITY_NORMAL',
          created_at: new Date().toISOString(),
        },
      ],
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

  // Search - returns InboxMessage objects in grpc-gateway format.
  http.get(`${API_BASE}/search`, ({ request }) => {
    const url = new URL(request.url);
    const query = url.searchParams.get('query') ?? '';
    const results = mockMessages
      .filter((m) =>
        m.subject.toLowerCase().includes(query.toLowerCase()),
      )
      .map((m) => ({
        id: String(m.id),
        thread_id: `thread-${m.id}`,
        subject: m.subject,
        body: m.body,
        priority: `PRIORITY_${m.priority.toUpperCase()}`,
        created_at: m.created_at,
        sender_name: m.sender_name,
      }));
    return HttpResponse.json({ results });
  }),

  // Autocomplete - returns recipients in grpc-gateway format.
  http.get(`${API_BASE}/autocomplete/recipients`, ({ request }) => {
    const url = new URL(request.url);
    const query = url.searchParams.get('query') ?? '';
    const recipients = mockAgentsStatus.agents
      .filter((a) =>
        a.name.toLowerCase().includes(query.toLowerCase()),
      )
      .map((a) => ({
        id: String(a.id),
        name: a.name,
        status: `AGENT_STATUS_${a.status.toUpperCase()}`,
      }));
    return HttpResponse.json({ recipients });
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

  // Reviews.
  http.get(`${API_BASE}/reviews`, ({ request }) => {
    const url = new URL(request.url);
    const state = url.searchParams.get('state');
    let filtered = [...mockReviews];
    if (state) {
      filtered = filtered.filter((r) => r.state === state);
    }
    return HttpResponse.json({ reviews: filtered });
  }),

  http.get(`${API_BASE}/reviews/:reviewId`, ({ params }) => {
    const id = params.reviewId as string;
    const review = mockReviews.find((r) => r.review_id === id);
    if (!review) {
      return HttpResponse.json(
        { error: { code: 'not_found', message: 'Review not found' } },
        { status: 404 },
      );
    }
    return HttpResponse.json({
      review_id: review.review_id,
      thread_id: review.thread_id,
      state: review.state,
      branch: review.branch,
      base_branch: 'main',
      review_type: review.review_type,
      iterations: 1,
      open_issues: '1',
    });
  }),

  http.post(`${API_BASE}/reviews`, async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    return HttpResponse.json({
      review_id: 'new-review-1',
      thread_id: 'new-thread-1',
      state: 'under_review',
    }, { status: 201 });
  }),

  http.post(`${API_BASE}/reviews/:reviewId/resubmit`, () => {
    return HttpResponse.json({
      review_id: 'abc123',
      state: 'under_review',
    });
  }),

  http.delete(`${API_BASE}/reviews/:reviewId`, () => {
    return new HttpResponse(null, { status: 204 });
  }),

  http.get(`${API_BASE}/reviews/:reviewId/issues`, ({ params }) => {
    const id = params.reviewId as string;
    const issues = mockReviewIssues.filter((i) => i.review_id === id);
    return HttpResponse.json({ issues });
  }),

  http.patch(`${API_BASE}/reviews/:reviewId/issues/:issueId`, () => {
    return HttpResponse.json({});
  }),
];
