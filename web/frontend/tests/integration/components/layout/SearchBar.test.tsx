// Integration tests for SearchBar component.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import {
  SearchBar,
  SearchTrigger,
  InlineSearchInput,
  SearchResultItem,
  getResultTypeLabel,
  getResultTypeIcon,
} from '@/components/layout/SearchBar.js';
import { useUIStore } from '@/stores/ui.js';
import * as searchApi from '@/api/search.js';
import type { SearchResult, APIResponse } from '@/types/api.js';
import type { SearchResultWithRoute } from '@/hooks/useSearch.js';

// Mock the search API.
vi.mock('@/api/search.js', () => ({
  search: vi.fn(),
  autocompleteRecipients: vi.fn(),
}));

// Mock data.
const mockSearchResults: SearchResult[] = [
  { type: 'message', id: 1, title: 'Test Message', snippet: 'Message content', created_at: '2024-01-01' },
  { type: 'thread', id: 2, title: 'Test Thread', snippet: 'Thread content', created_at: '2024-01-02' },
  { type: 'agent', id: 3, title: 'Test Agent', snippet: 'Agent description', created_at: '2024-01-03' },
  { type: 'topic', id: 4, title: 'Test Topic', snippet: 'Topic description', created_at: '2024-01-04' },
];

const mockResponse: APIResponse<SearchResult[]> = {
  data: mockSearchResults,
};

// Test wrapper with providers.
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });

  return function TestWrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe('SearchBar', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    vi.mocked(searchApi.search).mockResolvedValue(mockResponse);
    // Mock autocomplete to return empty so only search results appear.
    vi.mocked(searchApi.autocompleteRecipients).mockResolvedValue([]);
    // Reset UI store.
    useUIStore.setState({
      searchOpen: true,
      searchQuery: '',
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders when searchOpen is true', () => {
    render(<SearchBar />, { wrapper: createWrapper() });

    expect(screen.getByPlaceholderText(/Search messages, threads, agents/i)).toBeInTheDocument();
  });

  it('does not render when searchOpen is false', () => {
    useUIStore.setState({ searchOpen: false });
    render(<SearchBar />, { wrapper: createWrapper() });

    expect(screen.queryByPlaceholderText(/Search messages, threads, agents/i)).not.toBeInTheDocument();
  });

  it('shows empty state when no query', () => {
    render(<SearchBar />, { wrapper: createWrapper() });

    expect(screen.getByText(/Type to search messages, threads, agents, and topics/i)).toBeInTheDocument();
  });

  it('shows minimum characters message for short queries', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'a');

    expect(screen.getByText(/Type at least 2 characters to search/i)).toBeInTheDocument();
  });

  it('searches after typing and debounce', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(searchApi.search).toHaveBeenCalledWith('test', expect.any(AbortSignal));
    });
  });

  it('displays search results', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText('Test Message')).toBeInTheDocument();
    });

    expect(screen.getByText('Test Thread')).toBeInTheDocument();
    expect(screen.getByText('Test Agent')).toBeInTheDocument();
    expect(screen.getByText('Test Topic')).toBeInTheDocument();
  });

  it('shows result type labels', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText('Message')).toBeInTheDocument();
    });

    expect(screen.getByText('Thread')).toBeInTheDocument();
    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('Topic')).toBeInTheDocument();
  });

  it('shows results count', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText('4 results')).toBeInTheDocument();
    });
  });

  it('shows no results message', async () => {
    vi.mocked(searchApi.search).mockResolvedValue({ data: [] });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'xyz');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText(/No results found for/i)).toBeInTheDocument();
    });
  });

  it('clears search when clear button is clicked', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    // Wait for debounce and search to complete.
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByLabelText('Clear search')).toBeInTheDocument();
    });

    const clearButton = screen.getByLabelText('Clear search');
    await user.click(clearButton);

    expect(useUIStore.getState().searchQuery).toBe('');
  });

  it('closes on Escape key', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.keyboard('{Escape}');

    expect(useUIStore.getState().searchOpen).toBe(false);
  });

  it('navigates results with arrow keys', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText('Test Message')).toBeInTheDocument();
    });

    // First result should be selected by default.
    const firstResult = screen.getByText('Test Message').closest('button');
    expect(firstResult).toHaveClass('bg-blue-50');

    // Press down arrow.
    await user.keyboard('{ArrowDown}');

    // Second result should be selected.
    const secondResult = screen.getByText('Test Thread').closest('button');
    expect(secondResult).toHaveClass('bg-blue-50');
    expect(firstResult).not.toHaveClass('bg-blue-50');

    // Press up arrow.
    await user.keyboard('{ArrowUp}');

    // First result should be selected again.
    expect(firstResult).toHaveClass('bg-blue-50');
  });

  it('has correct ARIA attributes', () => {
    render(<SearchBar />, { wrapper: createWrapper() });

    const input = screen.getByPlaceholderText(/Search messages/i);
    expect(input).toHaveAttribute('role', 'combobox');
    expect(input).toHaveAttribute('aria-autocomplete', 'list');
    expect(input).toHaveAttribute('aria-controls', 'search-results');
  });

  it('shows keyboard hints in footer', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<SearchBar />, { wrapper: createWrapper() });

    await user.type(screen.getByPlaceholderText(/Search messages/i), 'test');

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(screen.getByText(/Navigate/i)).toBeInTheDocument();
    });

    expect(screen.getByText(/Select/i)).toBeInTheDocument();
    expect(screen.getByText(/Close/i)).toBeInTheDocument();
  });

  it('applies custom className', () => {
    render(<SearchBar className="custom-search" />, { wrapper: createWrapper() });

    // The dialog panel should have the custom class.
    const panel = document.querySelector('.custom-search');
    expect(panel).toBeInTheDocument();
  });

  it('uses custom placeholder', () => {
    render(<SearchBar placeholder="Custom placeholder" />, { wrapper: createWrapper() });

    expect(screen.getByPlaceholderText('Custom placeholder')).toBeInTheDocument();
  });
});

describe('SearchTrigger', () => {
  beforeEach(() => {
    useUIStore.setState({ searchOpen: false });
  });

  it('renders trigger button', () => {
    render(<SearchTrigger />, { wrapper: createWrapper() });

    expect(screen.getByText('Search...')).toBeInTheDocument();
  });

  it('shows keyboard shortcut', () => {
    render(<SearchTrigger />, { wrapper: createWrapper() });

    expect(screen.getByText('⌘K')).toBeInTheDocument();
  });

  it('opens search on click', async () => {
    const user = userEvent.setup();
    render(<SearchTrigger />, { wrapper: createWrapper() });

    await user.click(screen.getByRole('button'));

    expect(useUIStore.getState().searchOpen).toBe(true);
  });

  it('applies custom className', () => {
    const { container } = render(<SearchTrigger className="custom-trigger" />, {
      wrapper: createWrapper(),
    });

    expect(container.firstChild).toHaveClass('custom-trigger');
  });
});

describe('InlineSearchInput', () => {
  it('renders with value', () => {
    render(
      <InlineSearchInput value="test" onChange={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByDisplayValue('test')).toBeInTheDocument();
  });

  it('calls onChange on input', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <InlineSearchInput value="" onChange={onChange} />,
      { wrapper: createWrapper() },
    );

    await user.type(screen.getByRole('textbox'), 'new');

    expect(onChange).toHaveBeenCalledWith('n');
  });

  it('shows clear button when has value', () => {
    render(
      <InlineSearchInput value="test" onChange={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByLabelText('Clear')).toBeInTheDocument();
  });

  it('hides clear button when empty', () => {
    render(
      <InlineSearchInput value="" onChange={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.queryByLabelText('Clear')).not.toBeInTheDocument();
  });

  it('clears on clear button click', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <InlineSearchInput value="test" onChange={onChange} />,
      { wrapper: createWrapper() },
    );

    await user.click(screen.getByLabelText('Clear'));

    expect(onChange).toHaveBeenCalledWith('');
  });

  it('shows loading spinner', () => {
    const { container } = render(
      <InlineSearchInput value="test" onChange={() => {}} isLoading />,
      { wrapper: createWrapper() },
    );

    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('is disabled when disabled prop is true', () => {
    render(
      <InlineSearchInput value="" onChange={() => {}} disabled />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByRole('textbox')).toBeDisabled();
  });

  it('uses custom placeholder', () => {
    render(
      <InlineSearchInput value="" onChange={() => {}} placeholder="Custom" />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByPlaceholderText('Custom')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <InlineSearchInput value="" onChange={() => {}} className="custom-input" />,
      { wrapper: createWrapper() },
    );

    expect(container.firstChild).toHaveClass('custom-input');
  });
});

describe('SearchResultItem', () => {
  const mockResult: SearchResultWithRoute = {
    type: 'message',
    id: 1,
    title: 'Test Result',
    snippet: 'Test snippet text',
    created_at: '2024-01-01',
    route: '/inbox/thread/1',
  };

  it('renders result title', () => {
    render(
      <SearchResultItem result={mockResult} isSelected={false} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByText('Test Result')).toBeInTheDocument();
  });

  it('renders result snippet', () => {
    render(
      <SearchResultItem result={mockResult} isSelected={false} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByText('Test snippet text')).toBeInTheDocument();
  });

  it('renders type label', () => {
    render(
      <SearchResultItem result={mockResult} isSelected={false} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(screen.getByText('Message')).toBeInTheDocument();
  });

  it('shows selected state', () => {
    const { container } = render(
      <SearchResultItem result={mockResult} isSelected={true} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    expect(container.firstChild).toHaveClass('bg-blue-50');
  });

  it('calls onSelect when clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <SearchResultItem result={mockResult} isSelected={false} onSelect={onSelect} />,
      { wrapper: createWrapper() },
    );

    await user.click(screen.getByRole('option'));

    expect(onSelect).toHaveBeenCalled();
  });

  it('has correct ARIA attributes', () => {
    render(
      <SearchResultItem result={mockResult} isSelected={true} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    const option = screen.getByRole('option');
    expect(option).toHaveAttribute('aria-selected', 'true');
  });

  it('renders different icon for different types', () => {
    const agentResult: SearchResultWithRoute = {
      ...mockResult,
      type: 'agent',
      route: '/agents/1',
    };

    const { container } = render(
      <SearchResultItem result={agentResult} isSelected={false} onSelect={() => {}} />,
      { wrapper: createWrapper() },
    );

    // Agent icon has green background.
    const iconContainer = container.querySelector('.bg-green-100');
    expect(iconContainer).toBeInTheDocument();
  });
});

describe('getResultTypeLabel', () => {
  it('returns correct label for message', () => {
    expect(getResultTypeLabel('message')).toBe('Message');
  });

  it('returns correct label for thread', () => {
    expect(getResultTypeLabel('thread')).toBe('Thread');
  });

  it('returns correct label for agent', () => {
    expect(getResultTypeLabel('agent')).toBe('Agent');
  });

  it('returns correct label for topic', () => {
    expect(getResultTypeLabel('topic')).toBe('Topic');
  });
});

describe('getResultTypeIcon', () => {
  it('returns an icon component for message', () => {
    const Icon = getResultTypeIcon('message');
    expect(Icon).toBeDefined();
  });

  it('returns an icon component for thread', () => {
    const Icon = getResultTypeIcon('thread');
    expect(Icon).toBeDefined();
  });

  it('returns an icon component for agent', () => {
    const Icon = getResultTypeIcon('agent');
    expect(Icon).toBeDefined();
  });

  it('returns an icon component for topic', () => {
    const Icon = getResultTypeIcon('topic');
    expect(Icon).toBeDefined();
  });
});
