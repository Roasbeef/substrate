// Integration tests for SessionList component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  SessionList,
  CompactSessionList,
  SessionRow,
  CompactSessionRow,
} from '@/components/sessions/index.js';
import type { Session } from '@/types/api.js';

// Mock sessions data.
const mockSessions: Session[] = [
  {
    id: 1,
    agent_id: 1,
    agent_name: 'Agent1',
    project: '/path/to/project-a',
    branch: 'main',
    started_at: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
    status: 'active',
  },
  {
    id: 2,
    agent_id: 2,
    agent_name: 'Agent2',
    project: '/path/to/project-b',
    branch: 'feature-branch',
    started_at: new Date(Date.now() - 7200000).toISOString(), // 2 hours ago
    ended_at: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
    status: 'completed',
  },
  {
    id: 3,
    agent_id: 3,
    agent_name: 'Agent3',
    project: '/path/to/project-c',
    branch: 'dev',
    started_at: new Date(Date.now() - 86400000).toISOString(), // 1 day ago
    ended_at: new Date(Date.now() - 82800000).toISOString(),
    status: 'abandoned',
  },
];

describe('SessionRow', () => {
  it('renders session information', () => {
    render(
      <table>
        <tbody>
          <SessionRow session={mockSessions[0]} />
        </tbody>
      </table>,
    );

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('/path/to/project-a')).toBeInTheDocument();
    expect(screen.getByText('main')).toBeInTheDocument();
    expect(screen.getByText('active')).toBeInTheDocument();
  });

  it('displays session duration', () => {
    render(
      <table>
        <tbody>
          <SessionRow session={mockSessions[0]} />
        </tbody>
      </table>,
    );

    // Session started 1 hour ago and is still active.
    expect(screen.getByText('1h 0m')).toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(
      <table>
        <tbody>
          <SessionRow session={mockSessions[0]} onClick={onClick} />
        </tbody>
      </table>,
    );

    await user.click(screen.getByText('Agent1'));

    expect(onClick).toHaveBeenCalled();
  });

  it('shows View button on hover', () => {
    render(
      <table>
        <tbody>
          <SessionRow session={mockSessions[0]} onClick={() => {}} />
        </tbody>
      </table>,
    );

    expect(screen.getByText('View')).toBeInTheDocument();
  });

  it('highlights when selected', () => {
    const { container } = render(
      <table>
        <tbody>
          <SessionRow session={mockSessions[0]} isSelected />
        </tbody>
      </table>,
    );

    const row = container.querySelector('tr');
    expect(row).toHaveClass('bg-blue-50');
  });

  it('handles session without project', () => {
    const sessionWithoutProject: Session = {
      ...mockSessions[0],
      project: undefined,
      branch: undefined,
    };

    render(
      <table>
        <tbody>
          <SessionRow session={sessionWithoutProject} />
        </tbody>
      </table>,
    );

    expect(screen.getByText('—')).toBeInTheDocument();
  });
});

describe('CompactSessionRow', () => {
  it('renders session information', () => {
    render(<CompactSessionRow session={mockSessions[0]} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('/path/to/project-a')).toBeInTheDocument();
    expect(screen.getByText('active')).toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(<CompactSessionRow session={mockSessions[0]} onClick={onClick} />);

    await user.click(screen.getByText('Agent1'));

    expect(onClick).toHaveBeenCalled();
  });
});

describe('SessionList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders session list', () => {
    render(<SessionList sessions={mockSessions} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.getByText('Agent3')).toBeInTheDocument();
  });

  it('shows filter tabs', () => {
    render(<SessionList sessions={mockSessions} showFilters />);

    expect(screen.getByRole('button', { name: /all/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /active/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /completed/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /abandoned/i })).toBeInTheDocument();
  });

  it('shows counts in filter tabs', () => {
    render(<SessionList sessions={mockSessions} showFilters />);

    // Find buttons containing the count text.
    expect(screen.getByRole('button', { name: /all.*\(3\)/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /active.*\(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /completed.*\(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /abandoned.*\(1\)/i })).toBeInTheDocument();
  });

  it('filters sessions by status', async () => {
    const user = userEvent.setup();

    render(<SessionList sessions={mockSessions} showFilters />);

    // Click Active filter.
    await user.click(screen.getByRole('button', { name: /active/i }));

    // Only active session should be visible.
    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.queryByText('Agent2')).not.toBeInTheDocument();
    expect(screen.queryByText('Agent3')).not.toBeInTheDocument();
  });

  it('supports controlled filter mode', async () => {
    const user = userEvent.setup();
    const onFilterChange = vi.fn();

    render(
      <SessionList
        sessions={mockSessions}
        showFilters
        filter="all"
        onFilterChange={onFilterChange}
      />,
    );

    await user.click(screen.getByRole('button', { name: /completed/i }));

    expect(onFilterChange).toHaveBeenCalledWith('completed');
  });

  it('shows loading skeleton when loading', () => {
    render(<SessionList isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no sessions', () => {
    render(<SessionList sessions={[]} />);

    expect(screen.getByText('No sessions found')).toBeInTheDocument();
  });

  it('shows filtered empty state', async () => {
    const user = userEvent.setup();

    // Only active sessions.
    render(
      <SessionList
        sessions={[mockSessions[1]]} // Only completed.
        showFilters
      />,
    );

    // Filter to active.
    await user.click(screen.getByRole('button', { name: /active/i }));

    expect(screen.getByText('No active sessions')).toBeInTheDocument();
  });

  it('shows error state', () => {
    const error = new Error('Failed to load sessions');

    render(<SessionList error={error} />);

    expect(screen.getByText('Failed to load sessions')).toBeInTheDocument();
  });

  it('shows retry button on error', async () => {
    const user = userEvent.setup();
    const error = new Error('Failed to load sessions');
    const onRetry = vi.fn();

    render(<SessionList error={error} onRetry={onRetry} />);

    await user.click(screen.getByText('Try again'));

    expect(onRetry).toHaveBeenCalled();
  });

  it('calls onSessionClick when session is clicked', async () => {
    const user = userEvent.setup();
    const onSessionClick = vi.fn();

    render(<SessionList sessions={mockSessions} onSessionClick={onSessionClick} />);

    await user.click(screen.getByText('Agent1'));

    expect(onSessionClick).toHaveBeenCalledWith(1);
  });

  it('highlights selected session', () => {
    render(<SessionList sessions={mockSessions} selectedSessionId={1} />);

    // Find the row containing Agent1.
    const row = screen.getByText('Agent1').closest('tr');
    expect(row).toHaveClass('bg-blue-50');
  });

  it('hides filters when showFilters is false', () => {
    render(<SessionList sessions={mockSessions} showFilters={false} />);

    expect(screen.queryByRole('button', { name: /all/i })).not.toBeInTheDocument();
  });

  it('renders table headers', () => {
    render(<SessionList sessions={mockSessions} />);

    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('Project')).toBeInTheDocument();
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Duration')).toBeInTheDocument();
    expect(screen.getByText('Started')).toBeInTheDocument();
  });
});

describe('CompactSessionList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the title and session count', () => {
    render(<CompactSessionList sessions={mockSessions} />);

    expect(screen.getByText('Active Sessions')).toBeInTheDocument();
    expect(screen.getByText('(3)')).toBeInTheDocument();
  });

  it('renders custom title', () => {
    render(<CompactSessionList sessions={mockSessions} title="Running Sessions" />);

    expect(screen.getByText('Running Sessions')).toBeInTheDocument();
  });

  it('renders sessions', () => {
    render(<CompactSessionList sessions={mockSessions} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.getByText('Agent3')).toBeInTheDocument();
  });

  it('limits visible sessions to maxVisible', () => {
    render(<CompactSessionList sessions={mockSessions} maxVisible={2} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.queryByText('Agent3')).not.toBeInTheDocument();
    expect(screen.getByText('+1 more...')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    render(<CompactSessionList isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no sessions', () => {
    render(<CompactSessionList sessions={[]} />);

    expect(screen.getByText('No active sessions')).toBeInTheDocument();
  });

  it('shows View All button when handler provided', () => {
    render(
      <CompactSessionList sessions={mockSessions} onViewAllClick={() => {}} />,
    );

    expect(screen.getByText('View All')).toBeInTheDocument();
  });

  it('calls onViewAllClick when View All is clicked', async () => {
    const user = userEvent.setup();
    const onViewAllClick = vi.fn();

    render(
      <CompactSessionList sessions={mockSessions} onViewAllClick={onViewAllClick} />,
    );

    await user.click(screen.getByText('View All'));

    expect(onViewAllClick).toHaveBeenCalled();
  });

  it('calls onSessionClick when session is clicked', async () => {
    const user = userEvent.setup();
    const onSessionClick = vi.fn();

    render(
      <CompactSessionList sessions={mockSessions} onSessionClick={onSessionClick} />,
    );

    await user.click(screen.getByText('Agent1'));

    expect(onSessionClick).toHaveBeenCalledWith(1);
  });
});
