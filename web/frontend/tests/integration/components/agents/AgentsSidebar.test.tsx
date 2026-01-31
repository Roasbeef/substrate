// Integration tests for AgentsSidebar component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AgentsSidebar, AgentStatusList } from '@/components/agents/index.js';
import type { AgentWithStatus } from '@/types/api.js';

// Mock agents data.
const mockAgents: AgentWithStatus[] = [
  {
    id: 1,
    name: 'Agent1',
    status: 'active',
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 30,
  },
  {
    id: 2,
    name: 'Agent2',
    status: 'busy',
    last_active_at: new Date().toISOString(),
    session_id: 42,
    seconds_since_heartbeat: 60,
  },
  {
    id: 3,
    name: 'Agent3',
    status: 'idle',
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 600,
  },
];

describe('AgentsSidebar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the title and agent count', () => {
    render(<AgentsSidebar agents={mockAgents} />);

    expect(screen.getByText('Agents')).toBeInTheDocument();
    expect(screen.getByText('(3)')).toBeInTheDocument();
  });

  it('renders custom title', () => {
    render(<AgentsSidebar agents={mockAgents} title="Online Agents" />);

    expect(screen.getByText('Online Agents')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    render(<AgentsSidebar isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no agents', () => {
    render(<AgentsSidebar agents={[]} />);

    expect(screen.getByText('No agents registered')).toBeInTheDocument();
  });

  it('renders agent list', () => {
    render(<AgentsSidebar agents={mockAgents} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent2')).toBeInTheDocument();
    expect(screen.getByText('Agent3')).toBeInTheDocument();
  });

  it('calls onAgentClick when clicking an agent', async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();

    render(<AgentsSidebar agents={mockAgents} onAgentClick={onAgentClick} />);

    await user.click(screen.getByText('Agent1'));

    expect(onAgentClick).toHaveBeenCalledWith(1);
  });

  it('highlights selected agent', () => {
    render(<AgentsSidebar agents={mockAgents} selectedAgentId={2} />);

    // Agent2 should be highlighted.
    const agent2 = screen.getByText('Agent2').closest('.flex');
    expect(agent2).toHaveClass('bg-blue-50');
  });

  it('limits visible agents to maxVisible', () => {
    const manyAgents = Array.from({ length: 15 }, (_, i) => ({
      id: i + 1,
      name: `Agent${i + 1}`,
      status: 'active' as const,
      last_active_at: new Date().toISOString(),
      seconds_since_heartbeat: 0,
    }));

    render(<AgentsSidebar agents={manyAgents} maxVisible={5} />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.getByText('Agent5')).toBeInTheDocument();
    expect(screen.queryByText('Agent6')).not.toBeInTheDocument();
    expect(screen.getByText('+10 more...')).toBeInTheDocument();
  });

  it('shows View All button when handler provided', () => {
    render(
      <AgentsSidebar
        agents={mockAgents}
        onViewAllClick={() => {}}
      />,
    );

    expect(screen.getByRole('button', { name: 'View All' })).toBeInTheDocument();
  });

  it('calls onViewAllClick when View All is clicked', async () => {
    const user = userEvent.setup();
    const onViewAllClick = vi.fn();

    render(
      <AgentsSidebar agents={mockAgents} onViewAllClick={onViewAllClick} />,
    );

    await user.click(screen.getByRole('button', { name: 'View All' }));

    expect(onViewAllClick).toHaveBeenCalled();
  });

  it('filters by status when filterStatus is provided', () => {
    render(<AgentsSidebar agents={mockAgents} filterStatus="active" />);

    expect(screen.getByText('Agent1')).toBeInTheDocument();
    expect(screen.queryByText('Agent2')).not.toBeInTheDocument();
    expect(screen.queryByText('Agent3')).not.toBeInTheDocument();
    expect(screen.getByText('(1)')).toBeInTheDocument();
  });

  it('shows filtered empty state', () => {
    render(<AgentsSidebar agents={mockAgents} filterStatus="offline" />);

    expect(screen.getByText('No offline agents')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    render(<AgentsSidebar agents={mockAgents} className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });
});

describe('AgentStatusList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders status dots for agents', () => {
    render(<AgentStatusList agents={mockAgents} />);

    // Should have 3 status dots.
    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(3);
  });

  it('shows loading skeleton when loading', () => {
    render(<AgentStatusList isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('renders nothing when no agents', () => {
    const { container } = render(<AgentStatusList agents={[]} />);

    expect(container.firstChild).toBeNull();
  });

  it('calls onAgentClick when clicking a dot', async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();

    render(<AgentStatusList agents={mockAgents} onAgentClick={onAgentClick} />);

    const buttons = screen.getAllByRole('button');
    await user.click(buttons[0]);

    expect(onAgentClick).toHaveBeenCalledWith(1);
  });

  it('limits visible dots to maxVisible', () => {
    const manyAgents = Array.from({ length: 15 }, (_, i) => ({
      id: i + 1,
      name: `Agent${i + 1}`,
      status: 'active' as const,
      last_active_at: new Date().toISOString(),
      seconds_since_heartbeat: 0,
    }));

    render(<AgentStatusList agents={manyAgents} maxVisible={8} />);

    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(8);
    expect(screen.getByText('+7')).toBeInTheDocument();
  });

  it('shows agent name in title attribute', () => {
    render(<AgentStatusList agents={mockAgents} />);

    const buttons = screen.getAllByRole('button');
    expect(buttons[0]).toHaveAttribute('title', 'Agent1 (active)');
    expect(buttons[1]).toHaveAttribute('title', 'Agent2 (busy)');
  });

  it('applies custom className', () => {
    render(<AgentStatusList agents={mockAgents} className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });
});
