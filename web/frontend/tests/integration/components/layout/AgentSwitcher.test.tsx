// Integration tests for AgentSwitcher component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AgentSwitcher, ConnectedAgentSwitcher } from '@/components/layout/AgentSwitcher.js';
import { useAuthStore } from '@/stores/auth.js';
import type { AgentWithStatus } from '@/types/api.js';

// Mock agents data.
const mockAgents: AgentWithStatus[] = [
  {
    id: 1,
    name: 'Agent Alpha',
    status: 'active',
    last_active_at: new Date().toISOString(),
  },
  {
    id: 2,
    name: 'Agent Beta',
    status: 'busy',
    last_active_at: new Date().toISOString(),
    session_id: 1,
  },
  {
    id: 3,
    name: 'Agent Gamma',
    status: 'idle',
    last_active_at: new Date(Date.now() - 600000).toISOString(),
  },
  {
    id: 4,
    name: 'Agent Delta',
    status: 'offline',
    last_active_at: new Date(Date.now() - 3600000).toISOString(),
  },
];

describe('AgentSwitcher', () => {
  const defaultProps = {
    agents: mockAgents,
    onSelectAgent: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with Global when no agent selected', () => {
    render(<AgentSwitcher {...defaultProps} />);

    // When no agent is selected, the button shows "Global".
    expect(screen.getByText('Global')).toBeInTheDocument();
  });

  it('renders selected agent name', () => {
    render(<AgentSwitcher {...defaultProps} selectedAgentId={1} />);

    expect(screen.getByText('Agent Alpha')).toBeInTheDocument();
  });

  it('shows status badge for selected agent', () => {
    render(<AgentSwitcher {...defaultProps} selectedAgentId={2} />);

    // StatusBadge uses capitalized labels.
    expect(screen.getByText('Busy')).toBeInTheDocument();
  });

  it('opens dropdown when clicked', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher {...defaultProps} />);

    await user.click(screen.getByRole('button'));

    // All agents should be visible in dropdown.
    expect(screen.getByText('Agent Alpha')).toBeInTheDocument();
    expect(screen.getByText('Agent Beta')).toBeInTheDocument();
    expect(screen.getByText('Agent Gamma')).toBeInTheDocument();
    expect(screen.getByText('Agent Delta')).toBeInTheDocument();
  });

  it('calls onSelectAgent when agent is selected', async () => {
    const user = userEvent.setup();
    const onSelectAgent = vi.fn();
    render(<AgentSwitcher {...defaultProps} onSelectAgent={onSelectAgent} />);

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('Agent Beta'));

    expect(onSelectAgent).toHaveBeenCalledWith(2);
  });

  it('shows checkmark for currently selected agent', async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AgentSwitcher {...defaultProps} selectedAgentId={1} />,
    );

    await user.click(screen.getByRole('button'));

    // The checkmark should be next to Agent Alpha in the dropdown menu.
    // Use getAllByText since the name appears in both the button and dropdown.
    const alphaItems = screen.getAllByText('Agent Alpha');
    // The second match is in the dropdown menu item.
    const dropdownItem = alphaItems.find((el) => el.closest('[role="menu"]'));
    const alphaButton = dropdownItem?.closest('button');
    expect(alphaButton).not.toBeNull();
    // Look for the CheckIcon SVG (it has a specific path).
    const checkmark = alphaButton?.querySelector('svg path[d="M5 13l4 4L19 7"]');
    expect(checkmark).toBeInTheDocument();
  });

  it('shows loading state', () => {
    const { container } = render(<AgentSwitcher {...defaultProps} isLoading />);

    // Should show skeleton elements.
    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('is disabled when disabled prop is true', () => {
    render(<AgentSwitcher {...defaultProps} disabled />);

    expect(screen.getByRole('button')).toBeDisabled();
  });

  it('shows avatar for selected agent', () => {
    render(<AgentSwitcher {...defaultProps} selectedAgentId={1} />);

    // Avatar shows initials.
    expect(screen.getByText('AA')).toBeInTheDocument();
  });

  it('shows status badge for each agent in dropdown', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher {...defaultProps} />);

    await user.click(screen.getByRole('button'));

    // StatusBadge uses capitalized labels.
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Busy')).toBeInTheDocument();
    expect(screen.getByText('Idle')).toBeInTheDocument();
    expect(screen.getByText('Offline')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <AgentSwitcher {...defaultProps} className="custom-switcher" />,
    );

    expect(container.firstChild).toHaveClass('custom-switcher');
  });

  it('shows Global option even when no agents', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={[]} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));

    // Global option is always available even with empty agents list.
    // Use getAllByText since "Global" appears in both button and dropdown.
    const globalElements = screen.getAllByText('Global');
    expect(globalElements.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('All agents')).toBeInTheDocument();
  });
});

describe('AgentSwitcher search', () => {
  // Create more agents to trigger search.
  const manyAgents: AgentWithStatus[] = Array.from({ length: 10 }, (_, i) => ({
    id: i + 1,
    name: `Agent ${i + 1}`,
    status: 'active' as const,
    last_active_at: new Date().toISOString(),
  }));

  it('shows search input when more than 5 agents', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={manyAgents} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));

    expect(screen.getByPlaceholderText('Search agents...')).toBeInTheDocument();
  });

  it('hides search input when 5 or fewer agents', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={mockAgents} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));

    expect(screen.queryByPlaceholderText('Search agents...')).not.toBeInTheDocument();
  });

  it('filters agents by search query', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={manyAgents} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));
    await user.type(screen.getByPlaceholderText('Search agents...'), '1');

    // Should show Agent 1 and Agent 10.
    expect(screen.getByText('Agent 1')).toBeInTheDocument();
    expect(screen.getByText('Agent 10')).toBeInTheDocument();
    expect(screen.queryByText('Agent 2')).not.toBeInTheDocument();
  });

  it('shows no results message when search finds nothing', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={manyAgents} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));
    await user.type(screen.getByPlaceholderText('Search agents...'), 'xyz');

    expect(screen.getByText('No agents found')).toBeInTheDocument();
  });

  it('search is case insensitive', async () => {
    const user = userEvent.setup();
    render(<AgentSwitcher agents={manyAgents} onSelectAgent={vi.fn()} />);

    await user.click(screen.getByRole('button'));
    // Search with uppercase to test case insensitivity.
    await user.type(screen.getByPlaceholderText('Search agents...'), 'AGENT');

    // Should find all agents since they all contain "agent" (case insensitive).
    expect(screen.getByText('Agent 1')).toBeInTheDocument();
    expect(screen.getByText('Agent 5')).toBeInTheDocument();
  });

  it('can hide search with showSearch prop', async () => {
    const user = userEvent.setup();
    render(
      <AgentSwitcher
        agents={manyAgents}
        onSelectAgent={vi.fn()}
        showSearch={false}
      />,
    );

    await user.click(screen.getByRole('button'));

    expect(screen.queryByPlaceholderText('Search agents...')).not.toBeInTheDocument();
  });

  it('clears search when agent is selected', async () => {
    const user = userEvent.setup();
    const onSelectAgent = vi.fn();
    render(<AgentSwitcher agents={manyAgents} onSelectAgent={onSelectAgent} />);

    await user.click(screen.getByRole('button'));
    await user.type(screen.getByPlaceholderText('Search agents...'), '5');
    await user.click(screen.getByText('Agent 5'));

    // Reopen dropdown.
    await user.click(screen.getByRole('button'));

    // Search should be cleared, all agents visible.
    const searchInput = screen.getByPlaceholderText('Search agents...') as HTMLInputElement;
    expect(searchInput.value).toBe('');
  });
});

describe('ConnectedAgentSwitcher', () => {
  beforeEach(() => {
    // Reset auth store with all fields including new aggregate/global support.
    useAuthStore.setState({
      currentAgent: null,
      currentAgentStatus: null,
      selectedAgentIds: [],
      selectedAggregate: null,
      isGlobalExplicit: false,
      isAuthenticated: false,
      isLoading: false,
      availableAgents: [],
    });
  });

  it('shows selected agent from auth store', () => {
    useAuthStore.setState({
      currentAgent: {
        id: 1,
        name: 'Agent Alpha',
        createdAt: new Date().toISOString(),
        lastActiveAt: new Date().toISOString(),
      },
      isAuthenticated: true,
    });

    render(<ConnectedAgentSwitcher agents={mockAgents} />);

    expect(screen.getByText('Agent Alpha')).toBeInTheDocument();
  });

  it('updates auth store when agent is selected', async () => {
    const user = userEvent.setup();
    render(<ConnectedAgentSwitcher agents={mockAgents} />);

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('Agent Beta'));

    const state = useAuthStore.getState();
    expect(state.currentAgent?.id).toBe(2);
    expect(state.currentAgent?.name).toBe('Agent Beta');
  });

  it('updates available agents in store', async () => {
    const user = userEvent.setup();
    render(<ConnectedAgentSwitcher agents={mockAgents} />);

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByText('Agent Alpha'));

    const state = useAuthStore.getState();
    expect(state.availableAgents.length).toBe(4);
  });

  it('shows loading state', () => {
    const { container } = render(
      <ConnectedAgentSwitcher agents={mockAgents} isLoading />,
    );

    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('applies custom className', () => {
    const { container } = render(
      <ConnectedAgentSwitcher agents={mockAgents} className="custom-connected" />,
    );

    expect(container.firstChild).toHaveClass('custom-connected');
  });
});
