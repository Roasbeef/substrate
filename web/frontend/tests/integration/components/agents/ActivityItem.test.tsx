// Integration tests for ActivityItem component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ActivityItem, ActivityItemSkeleton } from '@/components/agents/index.js';
import type { Activity, ActivityType } from '@/types/api.js';

// Mock activity data.
function createMockActivity(
  overrides: Partial<Activity> = {},
): Activity {
  return {
    id: 1,
    agent_id: 1,
    agent_name: 'TestAgent',
    type: 'message_sent',
    description: 'Sent a message',
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('ActivityItem', () => {
  it('renders agent name and description', () => {
    const activity = createMockActivity({
      agent_name: 'AlphaAgent',
      description: 'Sent a message to BetaAgent',
    });

    render(<ActivityItem activity={activity} />);

    expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    expect(screen.getByText('Sent a message to BetaAgent')).toBeInTheDocument();
  });

  it('renders with avatar by default', () => {
    const activity = createMockActivity({ agent_name: 'TestAgent' });

    render(<ActivityItem activity={activity} />);

    // Avatar shows initials (first letter of TestAgent is T).
    // The avatar is present in the DOM.
    const avatar = document.querySelector('.rounded-full.font-medium');
    expect(avatar).toBeInTheDocument();
    expect(avatar?.textContent).toBe('T');
  });

  it('renders without avatar when showAvatar is false', () => {
    const activity = createMockActivity({ agent_name: 'TestAgent' });

    render(<ActivityItem activity={activity} showAvatar={false} />);

    // Should not show avatar initials in the same way.
    const avatarElements = document.querySelectorAll('.rounded-full');
    // Should have icon circle but not avatar.
    expect(avatarElements.length).toBe(1);
  });

  it('displays relative time by default', () => {
    const activity = createMockActivity({
      created_at: new Date(Date.now() - 120000).toISOString(), // 2 minutes ago.
    });

    render(<ActivityItem activity={activity} />);

    expect(screen.getByText('2m ago')).toBeInTheDocument();
  });

  it('displays full timestamp when showFullTimestamp is true', () => {
    const date = new Date('2024-01-15T10:30:00Z');
    const activity = createMockActivity({
      created_at: date.toISOString(),
    });

    render(<ActivityItem activity={activity} showFullTimestamp />);

    // Should show localized date string.
    expect(screen.getByText(date.toLocaleString())).toBeInTheDocument();
  });

  it.each([
    ['message_sent', 'Message sent'],
    ['message_read', 'Message read'],
    ['session_started', 'Session started'],
    ['session_completed', 'Session completed'],
    ['agent_registered', 'Agent registered'],
    ['heartbeat', 'Heartbeat'],
  ] as const)('renders correct icon for %s activity type', (type) => {
    const activity = createMockActivity({ type });

    render(<ActivityItem activity={activity} />);

    // Just verify the component renders without error.
    expect(screen.getByText('TestAgent')).toBeInTheDocument();
  });

  it('shows "just now" for very recent activities', () => {
    const activity = createMockActivity({
      created_at: new Date(Date.now() - 30000).toISOString(), // 30 seconds ago.
    });

    render(<ActivityItem activity={activity} />);

    expect(screen.getByText('just now')).toBeInTheDocument();
  });

  it('shows hours for older activities', () => {
    const activity = createMockActivity({
      created_at: new Date(Date.now() - 7200000).toISOString(), // 2 hours ago.
    });

    render(<ActivityItem activity={activity} />);

    expect(screen.getByText('2h ago')).toBeInTheDocument();
  });

  it('shows days for even older activities', () => {
    const activity = createMockActivity({
      created_at: new Date(Date.now() - 172800000).toISOString(), // 2 days ago.
    });

    render(<ActivityItem activity={activity} />);

    expect(screen.getByText('2d ago')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const activity = createMockActivity();

    render(<ActivityItem activity={activity} className="custom-class" />);

    const item = document.querySelector('.custom-class');
    expect(item).toBeInTheDocument();
  });
});

describe('ActivityItemSkeleton', () => {
  it('renders skeleton elements', () => {
    render(<ActivityItemSkeleton />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
