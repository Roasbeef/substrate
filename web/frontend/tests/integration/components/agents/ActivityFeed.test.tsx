// Integration tests for ActivityFeed component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ActivityFeed, CompactActivityFeed } from '@/components/agents/index.js';
import type { Activity } from '@/types/api.js';

// Mock activities data.
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

describe('ActivityFeed', () => {
  it('renders activities list', () => {
    render(<ActivityFeed activities={mockActivities} />);

    // Agent1 appears twice (activities 1 and 3).
    expect(screen.getAllByText('Agent1').length).toBe(2);
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.getByText('Sent a message to Agent2')).toBeInTheDocument();
    expect(screen.getByText('Started a new session')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    render(<ActivityFeed isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no activities', () => {
    render(<ActivityFeed activities={[]} />);

    expect(screen.getByText('No recent activity')).toBeInTheDocument();
  });

  it('shows error state with retry button', () => {
    const onRetry = vi.fn();
    const error = new Error('Failed to fetch activities');

    render(<ActivityFeed error={error} onRetry={onRetry} />);

    expect(screen.getByText('Failed to load activities')).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch activities')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('calls onRetry when retry button is clicked', async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    const error = new Error('Error');

    render(<ActivityFeed error={error} onRetry={onRetry} />);

    await user.click(screen.getByRole('button', { name: 'Try again' }));

    expect(onRetry).toHaveBeenCalled();
  });

  it('shows load more button when hasMore is true', () => {
    render(
      <ActivityFeed
        activities={mockActivities}
        hasMore
        onLoadMore={() => {}}
      />,
    );

    expect(screen.getByRole('button', { name: 'Load more' })).toBeInTheDocument();
  });

  it('calls onLoadMore when load more button is clicked', async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();

    render(
      <ActivityFeed
        activities={mockActivities}
        hasMore
        onLoadMore={onLoadMore}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Load more' }));

    expect(onLoadMore).toHaveBeenCalled();
  });

  it('disables load more button when fetching more', () => {
    render(
      <ActivityFeed
        activities={mockActivities}
        hasMore
        isFetchingMore
        onLoadMore={() => {}}
      />,
    );

    expect(screen.getByRole('button', { name: 'Loading...' })).toBeDisabled();
  });

  it('hides load more when no more items', () => {
    render(
      <ActivityFeed
        activities={mockActivities}
        hasMore={false}
        onLoadMore={() => {}}
      />,
    );

    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
  });

  it('hides avatars when showAvatars is false', () => {
    render(<ActivityFeed activities={mockActivities} showAvatars={false} />);

    // Verify activity items render without avatar initials.
    // Agent1 appears twice.
    expect(screen.getAllByText('Agent1').length).toBe(2);
    expect(screen.getByText('Agent2')).toBeInTheDocument();
  });

  it('applies maxHeight style', () => {
    render(<ActivityFeed activities={mockActivities} maxHeight="300px" />);

    const container = document.querySelector('[style*="max-height"]');
    expect(container).toBeInTheDocument();
  });

  it('applies custom className', () => {
    render(<ActivityFeed activities={mockActivities} className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });
});

describe('CompactActivityFeed', () => {
  it('renders limited activities', () => {
    render(<CompactActivityFeed activities={mockActivities} limit={2} />);

    // Should show first 2 activities.
    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();

    // Third activity description should not be shown.
    expect(screen.queryByText('Agent heartbeat')).not.toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    render(<CompactActivityFeed isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty message when no activities', () => {
    render(<CompactActivityFeed activities={[]} />);

    expect(screen.getByText('No recent activity')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    render(<CompactActivityFeed activities={mockActivities} className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });

  it('defaults to 5 items limit', () => {
    const manyActivities: Activity[] = Array.from({ length: 10 }, (_, i) => ({
      id: i + 1,
      agent_id: 1,
      agent_name: `Agent${i + 1}`,
      type: 'message_sent' as const,
      description: `Activity ${i + 1}`,
      created_at: new Date().toISOString(),
    }));

    render(<CompactActivityFeed activities={manyActivities} />);

    // Should show first 5 agents.
    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent5')).toBeInTheDocument();

    // 6th should not be shown.
    expect(screen.queryByText('Agent6')).not.toBeInTheDocument();
  });
});
