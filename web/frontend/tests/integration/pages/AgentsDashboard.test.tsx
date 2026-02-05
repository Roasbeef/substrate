// Integration tests for AgentsDashboard page component.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import AgentsDashboard from '@/pages/AgentsDashboard.js';
import type { AgentsStatusResponse, AgentWithStatus } from '@/types/api.js';
import { resetWebSocketClient } from '@/api/websocket.js';
import { resetWebSocketHookState } from '@/hooks/useWebSocket.js';

// Mock WebSocket to prevent real connections during tests.
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.CONNECTING;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(_url: string) {
    // Auto-open after a tick.
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.(new Event('open'));
    }, 0);
  }

  send(): void {}
  close(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new CloseEvent('close', { code: 1000 }));
  }
}

// Create a test query client.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
        staleTime: 0,
      },
    },
  });
}

// Render with providers.
function renderWithProviders(ui: React.ReactElement) {
  const queryClient = createTestQueryClient();

  return {
    ...render(
      <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
    ),
    queryClient,
  };
}

// Mock agents data with varied statuses.
const mockAgents: AgentWithStatus[] = [
  {
    id: 1,
    name: 'AlphaAgent',
    status: 'active',
    last_active_at: new Date().toISOString(),
    seconds_since_heartbeat: 30,
  },
  {
    id: 2,
    name: 'BetaAgent',
    status: 'busy',
    last_active_at: new Date().toISOString(),
    session_id: 42,
    seconds_since_heartbeat: 60,
  },
  {
    id: 3,
    name: 'GammaAgent',
    status: 'idle',
    last_active_at: new Date(Date.now() - 600000).toISOString(),
    seconds_since_heartbeat: 600,
  },
  {
    id: 4,
    name: 'DeltaAgent',
    status: 'offline',
    last_active_at: new Date(Date.now() - 3600000).toISOString(),
    seconds_since_heartbeat: 3600,
  },
];

const mockAgentsResponse: AgentsStatusResponse = {
  agents: mockAgents,
  counts: {
    active: 1,
    busy: 1,
    idle: 1,
    offline: 1,
  },
};

describe('AgentsDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mock WebSocket to prevent real connections.
    vi.stubGlobal('WebSocket', MockWebSocket);
    resetWebSocketClient();
    resetWebSocketHookState();
    // Set up the default handler for agents status.
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json(mockAgentsResponse);
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetWebSocketClient();
    resetWebSocketHookState();
  });

  it('renders the page title and description', async () => {
    renderWithProviders(<AgentsDashboard />);

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Agents',
    );
    expect(
      screen.getByText('Manage and monitor your registered agents.'),
    ).toBeInTheDocument();
  });

  it('shows loading skeletons while fetching', () => {
    renderWithProviders(<AgentsDashboard />);

    // Should show skeleton cards while loading.
    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('displays all agents after loading', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
  });

  it('displays dashboard stats', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('Total Agents')).toBeInTheDocument();
    });

    // Find stat values by their parent structure.
    const statsGrid = document.querySelector('.grid.grid-cols-2');
    expect(statsGrid).toBeInTheDocument();

    // Total should be sum of all agents.
    expect(screen.getByText('4')).toBeInTheDocument();
  });

  it('displays filter tabs with counts', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Check filter tabs exist in the nav element.
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    expect(nav).toBeInTheDocument();

    // Find filter tab buttons within the nav.
    const filterButtons = nav.querySelectorAll('button');
    expect(filterButtons.length).toBe(5); // All, Active, Busy, Idle, Offline
  });

  it('filters agents by status when clicking tabs', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Click on Active filter tab (within the nav).
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const activeTab = nav.querySelector('button:nth-child(2)') as HTMLElement;
    await user.click(activeTab);

    await waitFor(() => {
      // Only active agent should be visible.
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Others should not be visible.
    expect(screen.queryByText('BetaAgent')).not.toBeInTheDocument();
    expect(screen.queryByText('GammaAgent')).not.toBeInTheDocument();
    expect(screen.queryByText('DeltaAgent')).not.toBeInTheDocument();
  });

  it('filters to busy agents', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    });

    // Click on Busy filter tab (within the nav).
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const busyTab = nav.querySelector('button:nth-child(3)') as HTMLElement;
    await user.click(busyTab);

    await waitFor(() => {
      expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    });

    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('filters to idle agents', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    });

    // Click on Idle filter tab (within the nav).
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const idleTab = nav.querySelector('button:nth-child(4)') as HTMLElement;
    await user.click(idleTab);

    await waitFor(() => {
      expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    });

    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('filters to offline agents', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
    });

    // Click on Offline filter tab (within the nav).
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const offlineTab = nav.querySelector('button:nth-child(5)') as HTMLElement;
    await user.click(offlineTab);

    await waitFor(() => {
      expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
    });

    expect(screen.queryByText('AlphaAgent')).not.toBeInTheDocument();
  });

  it('shows empty state when no agents match filter', async () => {
    const user = userEvent.setup();

    // Override with response that has no busy agents.
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json({
          agents: [mockAgents[0]], // Only active agent.
          counts: { active: 1, busy: 0, idle: 0, offline: 0 },
        });
      }),
    );

    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Click on Busy filter tab (which has 0 agents).
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const busyTab = nav.querySelector('button:nth-child(3)') as HTMLElement;
    await user.click(busyTab);

    await waitFor(() => {
      expect(screen.getByText('No busy agents')).toBeInTheDocument();
    });
  });

  it('shows empty state when no agents exist', async () => {
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json({
          agents: [],
          counts: { active: 0, busy: 0, idle: 0, offline: 0 },
        });
      }),
    );

    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('No agents')).toBeInTheDocument();
    });

    expect(
      screen.getByText('No agents have been registered yet.'),
    ).toBeInTheDocument();
  });

  it('shows error state when fetch fails', async () => {
    server.use(
      http.get('/api/v1/agents-status', () => {
        return HttpResponse.json(
          { error: { code: 'server_error', message: 'Internal server error' } },
          { status: 500 },
        );
      }),
    );

    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
    });
  });

  it('allows retry after error', async () => {
    const user = userEvent.setup();
    let callCount = 0;

    server.use(
      http.get('/api/v1/agents-status', () => {
        callCount++;
        if (callCount === 1) {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Error' } },
            { status: 500 },
          );
        }
        return HttpResponse.json(mockAgentsResponse);
      }),
    );

    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load agents')).toBeInTheDocument();
    });

    // Click retry button.
    await user.click(screen.getByRole('button', { name: /try again/i }));

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });
  });

  it('calls onAgentClick when clicking an agent card', async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();

    renderWithProviders(<AgentsDashboard onAgentClick={onAgentClick} />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Click on the first agent card.
    await user.click(screen.getByText('AlphaAgent'));

    expect(onAgentClick).toHaveBeenCalledWith(1);
  });

  it('calls onRegisterClick when clicking register button', async () => {
    const user = userEvent.setup();
    const onRegisterClick = vi.fn();

    renderWithProviders(<AgentsDashboard onRegisterClick={onRegisterClick} />);

    await waitFor(() => {
      expect(screen.getByText('Register Agent')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /register agent/i }));

    expect(onRegisterClick).toHaveBeenCalled();
  });

  it('does not show register button when no handler provided', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    expect(
      screen.queryByRole('button', { name: /register agent/i }),
    ).not.toBeInTheDocument();
  });

  it('shows status badges on agent cards', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Check for status badges - they appear multiple times (cards, stats, and tabs).
    // Verify status badge text exists (at least once for each status).
    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Busy').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Idle').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Offline').length).toBeGreaterThan(0);

    // Verify all 4 agents are displayed.
    expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    expect(screen.getByText('GammaAgent')).toBeInTheDocument();
    expect(screen.getByText('DeltaAgent')).toBeInTheDocument();
  });

  it('shows session info for busy agents', async () => {
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('BetaAgent')).toBeInTheDocument();
    });

    // Check for session info in the busy agent card.
    expect(screen.getByText('Current session')).toBeInTheDocument();
    expect(screen.getByText('#42')).toBeInTheDocument();
  });

  it('sets aria-current on active filter tab', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Get filter tabs from nav.
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const allTab = nav.querySelector('button:nth-child(1)') as HTMLElement;
    const activeTab = nav.querySelector('button:nth-child(2)') as HTMLElement;

    // Initially All tab should be current.
    expect(allTab).toHaveAttribute('aria-current', 'page');

    // Click Active filter tab.
    await user.click(activeTab);

    await waitFor(() => {
      expect(activeTab).toHaveAttribute('aria-current', 'page');
    });
    expect(allTab).not.toHaveAttribute('aria-current');
  });

  it('filters via stat card click', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AgentsDashboard />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // Find the Active stat card and click it.
    // The stat cards have onClick handlers for active, busy, idle.
    const statsGrid = document.querySelector('.grid.grid-cols-2');
    const activeStatButton = statsGrid?.querySelector('button:nth-child(2)');
    expect(activeStatButton).toBeInTheDocument();

    if (activeStatButton) {
      await user.click(activeStatButton);
    }

    await waitFor(() => {
      // Should filter to show only active agents.
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // The Active filter tab should now be selected.
    const nav = screen.getByRole('navigation', { name: 'Filter agents' });
    const activeTab = nav.querySelector('button:nth-child(2)') as HTMLElement;

    await waitFor(() => {
      expect(activeTab).toHaveAttribute('aria-current', 'page');
    });
  });

  it('applies custom className', async () => {
    renderWithProviders(<AgentsDashboard className="custom-class" />);

    await waitFor(() => {
      expect(screen.getByText('AlphaAgent')).toBeInTheDocument();
    });

    // The root div should have the custom class.
    const container = document.querySelector('.custom-class');
    expect(container).toBeInTheDocument();
  });
});
