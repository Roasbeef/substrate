// Integration tests for SessionDetail component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  SessionDetail,
  type SessionLogEntry,
  type SessionTask,
} from '@/components/sessions/index.js';
import type { Session } from '@/types/api.js';

// Mock session data.
const mockSession: Session = {
  id: 1,
  agent_id: 1,
  agent_name: 'TestAgent',
  project: '/path/to/project',
  branch: 'feature-branch',
  started_at: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
  status: 'active',
};

const mockCompletedSession: Session = {
  ...mockSession,
  id: 2,
  status: 'completed',
  ended_at: new Date().toISOString(),
};

const mockLogEntries: SessionLogEntry[] = [
  {
    id: 1,
    timestamp: new Date(Date.now() - 3000000).toISOString(),
    type: 'progress',
    message: 'Started implementing feature X',
  },
  {
    id: 2,
    timestamp: new Date(Date.now() - 2400000).toISOString(),
    type: 'discovery',
    message: 'Found existing utility that can be reused',
  },
  {
    id: 3,
    timestamp: new Date(Date.now() - 1800000).toISOString(),
    type: 'blocker',
    message: 'Need clarification on API response format',
  },
];

const mockTasks: SessionTask[] = [
  {
    id: 'task-1',
    subject: 'Implement user authentication',
    status: 'completed',
    description: 'Add JWT-based auth',
  },
  {
    id: 'task-2',
    subject: 'Create API endpoints',
    status: 'in_progress',
  },
  {
    id: 'task-3',
    subject: 'Write tests',
    status: 'pending',
  },
];

describe('SessionDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders session information', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
      />,
    );

    // Modal title shows "Session #ID".
    expect(screen.getByRole('heading', { name: 'Session #1' })).toBeInTheDocument();
    expect(screen.getByText('TestAgent')).toBeInTheDocument();
  });

  it('shows tabs', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
      />,
    );

    expect(screen.getByRole('button', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Log' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tasks' })).toBeInTheDocument();
  });

  it('shows overview tab by default', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
      />,
    );

    expect(screen.getByText('/path/to/project')).toBeInTheDocument();
    expect(screen.getByText('feature-branch')).toBeInTheDocument();
    expect(screen.getByText('active')).toBeInTheDocument();
  });

  it('shows session duration', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
      />,
    );

    expect(screen.getByText('1h 0m')).toBeInTheDocument();
  });

  it('switches to log tab', async () => {
    const user = userEvent.setup();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        logEntries={mockLogEntries}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Log' }));

    expect(screen.getByText('Started implementing feature X')).toBeInTheDocument();
    expect(screen.getByText('Found existing utility that can be reused')).toBeInTheDocument();
    expect(screen.getByText('Need clarification on API response format')).toBeInTheDocument();
  });

  it('switches to tasks tab', async () => {
    const user = userEvent.setup();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        tasks={mockTasks}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Tasks' }));

    expect(screen.getByText('Implement user authentication')).toBeInTheDocument();
    expect(screen.getByText('Create API endpoints')).toBeInTheDocument();
    expect(screen.getByText('Write tests')).toBeInTheDocument();
  });

  it('shows empty state for log tab', async () => {
    const user = userEvent.setup();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        logEntries={[]}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Log' }));

    expect(screen.getByText('No log entries yet')).toBeInTheDocument();
  });

  it('shows empty state for tasks tab', async () => {
    const user = userEvent.setup();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        tasks={[]}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Tasks' }));

    expect(screen.getByText('No tasks in this session')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        isLoading
      />,
    );

    // Should show spinner.
    expect(document.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('shows not found state when no session', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={undefined}
      />,
    );

    expect(screen.getByText('Session not found')).toBeInTheDocument();
  });

  it('shows complete button for active session', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        onComplete={() => {}}
      />,
    );

    expect(screen.getByRole('button', { name: 'Complete Session' })).toBeInTheDocument();
  });

  it('hides complete button for completed session', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockCompletedSession}
        onComplete={() => {}}
      />,
    );

    expect(screen.queryByRole('button', { name: 'Complete Session' })).not.toBeInTheDocument();
  });

  it('calls onComplete when complete button is clicked', async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        onComplete={onComplete}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Complete Session' }));

    expect(onComplete).toHaveBeenCalledWith(1);
  });

  it('shows loading state on complete button when completing', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        onComplete={() => {}}
        isCompleting
      />,
    );

    const completeButton = screen.getByRole('button', { name: 'Complete Session' });
    expect(completeButton).toBeDisabled();
  });

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(
      <SessionDetail
        isOpen
        onClose={onClose}
        session={mockSession}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Close' }));

    expect(onClose).toHaveBeenCalled();
  });

  it('does not render when closed', () => {
    render(
      <SessionDetail
        isOpen={false}
        onClose={() => {}}
        session={mockSession}
      />,
    );

    expect(screen.queryByText('Session #1')).not.toBeInTheDocument();
  });

  it('shows task statuses correctly', async () => {
    const user = userEvent.setup();

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
        tasks={mockTasks}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Tasks' }));

    // Check status badges.
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('in progress')).toBeInTheDocument();
    expect(screen.getByText('pending')).toBeInTheDocument();
  });

  it('handles session without project/branch', () => {
    const sessionWithoutDetails: Session = {
      ...mockSession,
      project: undefined,
      branch: undefined,
    };

    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={sessionWithoutDetails}
      />,
    );

    // Should show dashes for missing values.
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it('shows ended timestamp for completed session', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockCompletedSession}
      />,
    );

    expect(screen.getByText('Ended')).toBeInTheDocument();
  });

  it('does not show ended timestamp for active session', () => {
    render(
      <SessionDetail
        isOpen
        onClose={() => {}}
        session={mockSession}
      />,
    );

    expect(screen.queryByText('Ended')).not.toBeInTheDocument();
  });
});
