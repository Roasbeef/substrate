// Integration tests for AgentCardGrid component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AgentCardGrid } from '@/components/agents/index.js';
import type { AgentWithStatus } from '@/types/api.js';

// Mock agents data with varied statuses.
const mockAgents: AgentWithStatus[] = [
  {
    id: 1,
    name: 'AlphaAgent',
    status: 'active',
    last_active_at: new Date('2024-01-15T10:00:00Z').toISOString(),
    seconds_since_heartbeat: 30,
  },
  {
    id: 2,
    name: 'BetaAgent',
    status: 'busy',
    last_active_at: new Date('2024-01-15T09:00:00Z').toISOString(),
    session_id: 42,
    seconds_since_heartbeat: 60,
  },
  {
    id: 3,
    name: 'GammaAgent',
    status: 'idle',
    last_active_at: new Date('2024-01-15T08:00:00Z').toISOString(),
    seconds_since_heartbeat: 600,
  },
  {
    id: 4,
    name: 'DeltaAgent',
    status: 'offline',
    last_active_at: new Date('2024-01-14T10:00:00Z').toISOString(),
    seconds_since_heartbeat: 3600,
  },
];

describe('AgentCardGrid', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders all agents', () => {
    render(<AgentCardGrid agents={mockAgents} />);

    expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    render(<AgentCardGrid isLoading />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no agents', () => {
    render(<AgentCardGrid agents={[]} />);

    expect(screen.getByText('No agents')).toBeInTheDocument();
    expect(
      screen.getByText('No agents have been registered yet.'),
    ).toBeInTheDocument();
  });

  it('shows filter tabs by default', () => {
    render(<AgentCardGrid agents={mockAgents} />);

    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Active' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Busy' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Idle' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Offline' })).toBeInTheDocument();
  });

  it('hides filter tabs when showFilters is false', () => {
    render(<AgentCardGrid agents={mockAgents} showFilters={false} />);

    expect(screen.queryByRole('button', { name: 'All' })).not.toBeInTheDocument();
  });

  it('shows sort dropdown by default', () => {
    render(<AgentCardGrid agents={mockAgents} />);

    expect(screen.getByLabelText('Sort by:')).toBeInTheDocument();
  });

  it('hides sort dropdown when showSort is false', () => {
    render(<AgentCardGrid agents={mockAgents} showSort={false} />);

    expect(screen.queryByLabelText('Sort by:')).not.toBeInTheDocument();
  });

  it('filters agents by status', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    // Click Active filter.
    await user.click(screen.getByRole('button', { name: 'Active' }));

    // Only active agent should be visible.
    expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    expect(screen.queryByText('BetaAgent')).not.toBeInTheDocument();
    expect(screen.queryByText('GammaAgent')).not.toBeInTheDocument();
    expect(screen.queryByText('DeltaAgent')).not.toBeInTheDocument();
  });

  it('filters to busy agents', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    await user.click(screen.getByRole('button', { name: 'Busy' }));

    expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('filters to idle agents', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    await user.click(screen.getByRole('button', { name: 'Idle' }));

    expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('filters to offline agents', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    await user.click(screen.getByRole('button', { name: 'Offline' }));

    expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('shows empty state when filter has no matches', async () => {
    const user = userEvent.setup();
    const agentsWithoutBusy = mockAgents.filter((a) => a.status !== 'busy');
    render(<AgentCardGrid agents={agentsWithoutBusy} />);

    await user.click(screen.getByRole('button', { name: 'Busy' }));

    expect(screen.getByText('No busy agents')).toBeInTheDocument();
    expect(
      screen.getByText('No agents are currently busy.'),
    ).toBeInTheDocument();
  });

  it('sorts agents by name', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    // Default sort is by name.
    const cards = document.querySelectorAll('[role="button"]');
    // Filter cards should be buttons too, so check the card content order.
    const cardTexts = Array.from(
      document.querySelectorAll('.font-medium.text-gray-900'),
    ).map((el) => el.textContent);

    expect(cardTexts).toContain('AlphaAgent');
    expect(cardTexts).toContain('BetaAgent');
  });

  it('sorts agents by status', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    const sortSelect = screen.getByLabelText('Sort by:');
    await user.selectOptions(sortSelect, 'status');

    // Status order: active, busy, idle, offline.
    const cardNames = Array.from(
      document.querySelectorAll('.rounded-lg.border.bg-white.p-4'),
    ).map((card) => card.querySelector('.font-medium.text-gray-900')?.textContent);

    expect(cardNames[0]).toBe('AlphaAgent'); // active.
    expect(cardNames[1]).toBe('BetaAgent'); // busy.
    expect(cardNames[2]).toBe('GammaAgent'); // idle.
    expect(cardNames[3]).toBe('DeltaAgent'); // offline.
  });

  it('sorts agents by last active', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    const sortSelect = screen.getByLabelText('Sort by:');
    await user.selectOptions(sortSelect, 'last_active');

    // Most recent first.
    const cardNames = Array.from(
      document.querySelectorAll('.rounded-lg.border.bg-white.p-4'),
    ).map((card) => card.querySelector('.font-medium.text-gray-900')?.textContent);

    expect(cardNames[0]).toBe('AlphaAgent'); // 2024-01-15T10:00.
    expect(cardNames[1]).toBe('BetaAgent'); // 2024-01-15T09:00.
    expect(cardNames[2]).toBe('GammaAgent'); // 2024-01-15T08:00.
    expect(cardNames[3]).toBe('DeltaAgent'); // 2024-01-14T10:00.
  });

  it('calls onAgentClick when clicking a card', async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();

    render(<AgentCardGrid agents={mockAgents} onAgentClick={onAgentClick} />);

    await user.click(screen.getByText('AlphaAgent'));

    expect(onAgentClick).toHaveBeenCalledWith(1);
  });

  it('highlights selected agent', () => {
    render(
      <AgentCardGrid
        agents={mockAgents}
        selectedAgentId={2}
        onAgentClick={() => {}}
      />,
    );

    // The selected card should have a ring.
    const betaCard = screen.getByText('BetaAgent').closest('.rounded-lg');
    expect(betaCard).toHaveClass('ring-2');
  });

  it('supports controlled filter state', async () => {
    const user = userEvent.setup();
    const onFilterChange = vi.fn();

    render(
      <AgentCardGrid
        agents={mockAgents}
        filter="active"
        onFilterChange={onFilterChange}
      />,
    );

    // Only active agent should be visible.
    expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    expect(screen.queryByText('BetaAgent')).not.toBeInTheDocument();

    // Click Busy filter.
    await user.click(screen.getByRole('button', { name: 'Busy' }));

    expect(onFilterChange).toHaveBeenCalledWith('busy');
  });

  it('supports controlled sort state', async () => {
    const user = userEvent.setup();
    const onSortChange = vi.fn();

    render(
      <AgentCardGrid
        agents={mockAgents}
        sort="name"
        onSortChange={onSortChange}
      />,
    );

    const sortSelect = screen.getByLabelText('Sort by:');
    await user.selectOptions(sortSelect, 'status');

    expect(onSortChange).toHaveBeenCalledWith('status');
  });

  it('sets aria-pressed on filter buttons', async () => {
    const user = userEvent.setup();
    render(<AgentCardGrid agents={mockAgents} />);

    const allButton = screen.getByRole('button', { name: 'All' });
    const activeButton = screen.getByRole('button', { name: 'Active' });

    // Initially All is pressed.
    expect(allButton).toHaveAttribute('aria-pressed', 'true');
    expect(activeButton).toHaveAttribute('aria-pressed', 'false');

    // Click Active.
    await user.click(activeButton);

    await waitFor(() => {
      expect(activeButton).toHaveAttribute('aria-pressed', 'true');
    });
    expect(allButton).toHaveAttribute('aria-pressed', 'false');
  });

  it('applies custom className', () => {
    render(<AgentCardGrid agents={mockAgents} className="custom-class" />);

    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });

  it('shows responsive grid', () => {
    render(<AgentCardGrid agents={mockAgents} />);

    const grid = document.querySelector('.grid');
    expect(grid).toHaveClass('grid-cols-1');
    expect(grid).toHaveClass('sm:grid-cols-2');
    expect(grid).toHaveClass('lg:grid-cols-3');
  });
});
