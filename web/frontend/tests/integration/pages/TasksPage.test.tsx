// Integration tests for TasksPage component.
// Tests board view, list view, detail panel, status filters, agent filter,
// dependency visualization, and stats cards using MSW mock data.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import TasksPage from '@/pages/TasksPage.js';

// Create a fresh QueryClient for each test.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
        staleTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

// Wrapper component for tests.
function TestWrapper({ children }: { children: React.ReactNode }) {
  const queryClient = createTestQueryClient();
  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
}

// Mock task data with dependency chain: task-001 → task-002 → task-003.
const mockTasksWithDeps = [
  {
    id: '1',
    agent_id: '2',
    list_id: 'list-abc',
    claude_task_id: 'task-001',
    subject: 'Implement feature X',
    description: 'Build the new feature with tests',
    active_form: '',
    metadata_json: '',
    status: 'TASK_STATUS_PENDING',
    owner: '',
    blocked_by: [],
    blocks: ['task-002'],
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T12:00:00Z',
    started_at: '',
    completed_at: '',
  },
  {
    id: '2',
    agent_id: '2',
    list_id: 'list-abc',
    claude_task_id: 'task-002',
    subject: 'Fix bug in parser',
    description: 'Parser fails on nested expressions',
    active_form: 'Fixing parser bug',
    metadata_json: '{"priority":"high"}',
    status: 'TASK_STATUS_IN_PROGRESS',
    owner: 'Agent1',
    blocked_by: ['task-001'],
    blocks: ['task-003'],
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T14:00:00Z',
    started_at: '2026-01-15T13:00:00Z',
    completed_at: '',
  },
  {
    id: '3',
    agent_id: '3',
    list_id: 'list-def',
    claude_task_id: 'task-003',
    subject: 'Write documentation',
    description: 'API reference docs for the new feature',
    active_form: '',
    metadata_json: '',
    status: 'TASK_STATUS_COMPLETED',
    owner: 'Agent2',
    blocked_by: ['task-002'],
    blocks: [],
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T16:00:00Z',
    started_at: '2026-01-15T15:00:00Z',
    completed_at: '2026-01-15T16:00:00Z',
  },
];

// Override the default MSW tasks handler to return our detailed mock data.
function useDetailedTaskMocks() {
  server.use(
    http.get('/api/v1/tasks', () => {
      return HttpResponse.json({ tasks: mockTasksWithDeps });
    }),
  );
}

describe('TasksPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page header and subtitle', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Tasks' })).toBeInTheDocument();
    });
    expect(screen.getByText('Track Claude Code agent tasks and progress.')).toBeInTheDocument();
  });

  it('renders view toggle with list and board buttons', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /list/i })).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /board/i })).toBeInTheDocument();
  });

  it('renders stats cards with correct values', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      // Stats from default mock handler: 5 pending, 3 in progress, 12 completed,
      // 1 blocked, 4 available, 2 today.
      expect(screen.getByText('5')).toBeInTheDocument();
    });
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
  });

  it('renders agent filter dropdown', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
    expect(screen.getByText('All Agents')).toBeInTheDocument();
  });
});

describe('TasksPage — Board View', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useDetailedTaskMocks();
  });

  it('renders three kanban columns by default', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      // Column headers for the three statuses.
      const pendingHeaders = screen.getAllByText('Pending');
      expect(pendingHeaders.length).toBeGreaterThanOrEqual(1);
    });
    expect(screen.getAllByText('In Progress').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Completed').length).toBeGreaterThanOrEqual(1);
  });

  it('renders task cards in correct columns', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      // Mock data has: Implement feature X (pending), Fix bug (in_progress),
      // Write documentation (completed).
      expect(screen.getByText('Implement feature X')).toBeInTheDocument();
    });
    expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    expect(screen.getByText('Write documentation')).toBeInTheDocument();
  });

  it('shows activeForm text for in-progress tasks', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fixing parser bug')).toBeInTheDocument();
    });
  });

  it('shows task IDs as monospace text', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('#task-001')).toBeInTheDocument();
    });
    expect(screen.getByText('#task-002')).toBeInTheDocument();
    expect(screen.getByText('#task-003')).toBeInTheDocument();
  });

  it('shows blocked indicator on tasks with blocked_by', async () => {
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      // Task task-002 is blocked by task-001, should show "Blocked" chip.
      const blockedChips = screen.getAllByText('Blocked');
      expect(blockedChips.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('opens detail panel when clicking a task card', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Implement feature X')).toBeInTheDocument();
    });

    // Click the first task card.
    await user.click(screen.getByText('Implement feature X'));

    // Detail panel should appear with section headers.
    await waitFor(() => {
      expect(screen.getByText('Timeline')).toBeInTheDocument();
    });
    expect(screen.getByText('Identifiers')).toBeInTheDocument();
  });

  it('closes detail panel on Escape key', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Implement feature X')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Implement feature X'));
    await waitFor(() => {
      expect(screen.getByText('Timeline')).toBeInTheDocument();
    });

    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByText('Timeline')).not.toBeInTheDocument();
    });
  });
});

describe('TasksPage — List View', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useDetailedTaskMocks();
  });

  it('switches to list view when list button is clicked', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /list/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /list/i }));

    // List view shows status filter tabs.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    });
  });

  it('shows status filter tabs in list view', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /list/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /list/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'In Progress' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Completed' })).toBeInTheDocument();
    });
  });

  it('filters tasks by status when clicking filter tabs', async () => {
    const user = userEvent.setup();

    // Override to return only in-progress tasks when status filter is applied.
    server.use(
      http.get('/api/v1/tasks', ({ request }) => {
        const url = new URL(request.url);
        const status = url.searchParams.get('status');
        if (status === 'TASK_STATUS_IN_PROGRESS') {
          return HttpResponse.json({
            tasks: [mockTasksWithDeps[1]],
          });
        }
        return HttpResponse.json({ tasks: mockTasksWithDeps });
      }),
    );

    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /list/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /list/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'In Progress' })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'In Progress' }));

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });
  });
});

describe('TasksPage — Detail Panel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useDetailedTaskMocks();
  });

  it('shows task subject as panel heading', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      // Panel shows the subject as h2.
      const heading = screen.getByRole('heading', { level: 2 });
      expect(heading).toHaveTextContent('Fix bug in parser');
    });
  });

  it('shows description section', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Description')).toBeInTheDocument();
      // Description text appears in both the card and the panel.
      const descElements = screen.getAllByText('Parser fails on nested expressions');
      expect(descElements.length).toBeGreaterThanOrEqual(2);
    });
  });

  it('shows owner section for tasks with an owner', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Owner')).toBeInTheDocument();
      // Owner chip shows "Agent1".
      const ownerElements = screen.getAllByText('Agent1');
      expect(ownerElements.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows activeForm with pulsing indicator for in-progress tasks', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      // Active form text appears in the panel.
      const activeFormElements = screen.getAllByText('Fixing parser bug');
      expect(activeFormElements.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows dependencies section for blocked tasks', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    // Click the blocked task (task-002 is blocked by task-001 and blocks task-003).
    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Dependencies')).toBeInTheDocument();
    });

    // "Blocked by" appears in both DependencyGraph label and DepCard label.
    const blockedByElements = screen.getAllByText('Blocked by');
    expect(blockedByElements.length).toBeGreaterThanOrEqual(1);

    // "Blocks" may also appear in multiple places.
    const blocksElements = screen.getAllByText('Blocks');
    expect(blocksElements.length).toBeGreaterThanOrEqual(1);

    // Should show the dep graph with the "Current" node label.
    expect(screen.getByText('Current')).toBeInTheDocument();
  });

  it('shows dependency graph with upstream and downstream nodes', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Dependencies')).toBeInTheDocument();
    });

    // The dep graph should show task-001 (upstream) and task-003 (downstream).
    // Both are referenced by claude_task_id in the graph nodes.
    const graphTaskIds = screen.getAllByText(/#task-00[13]/);
    expect(graphTaskIds.length).toBeGreaterThanOrEqual(2);
  });

  it('shows metadata section when metadata_json is present', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    // Task task-002 has metadata_json: {"priority":"high"}.
    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Metadata')).toBeInTheDocument();
      expect(screen.getByText('priority')).toBeInTheDocument();
      expect(screen.getByText('high')).toBeInTheDocument();
    });
  });

  it('shows timeline with Created timestamp', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Fix bug in parser')).toBeInTheDocument();
    });

    // Click task-002 which has created_at, updated_at, and started_at.
    await user.click(screen.getByText('Fix bug in parser'));

    await waitFor(() => {
      expect(screen.getByText('Timeline')).toBeInTheDocument();
      expect(screen.getByText('Created')).toBeInTheDocument();
      expect(screen.getByText('Updated')).toBeInTheDocument();
      // task-002 has started_at set.
      expect(screen.getByText('Started')).toBeInTheDocument();
    });
  });

  it('shows identifiers with List ID and Task ID', async () => {
    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      expect(screen.getByText('Implement feature X')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Implement feature X'));

    await waitFor(() => {
      expect(screen.getByText('Identifiers')).toBeInTheDocument();
      expect(screen.getByText('List ID')).toBeInTheDocument();
      expect(screen.getByText('Task ID')).toBeInTheDocument();
      expect(screen.getByText('list-abc')).toBeInTheDocument();
      expect(screen.getByText('task-001')).toBeInTheDocument();
    });
  });
});

describe('TasksPage — Empty State', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows empty state when no tasks exist', async () => {
    server.use(
      http.get('/api/v1/tasks', () => {
        return HttpResponse.json({ tasks: [] });
      }),
    );

    const user = userEvent.setup();
    render(<TasksPage />, { wrapper: TestWrapper });

    // Switch to list view to see the empty state text.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /list/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /list/i }));

    await waitFor(() => {
      expect(screen.getByText('No tasks')).toBeInTheDocument();
      expect(screen.getByText('No tasks have been tracked yet.')).toBeInTheDocument();
    });
  });

  it('shows empty board columns when no tasks exist', async () => {
    server.use(
      http.get('/api/v1/tasks', () => {
        return HttpResponse.json({ tasks: [] });
      }),
    );

    render(<TasksPage />, { wrapper: TestWrapper });

    await waitFor(() => {
      // Board view shows "No tasks" in each column.
      const noTasksElements = screen.getAllByText('No tasks');
      expect(noTasksElements.length).toBe(3);
    });
  });
});
