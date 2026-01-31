// Integration tests for SearchResultsPage component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import SearchResultsPage from '@/pages/SearchResultsPage.js';
import type { SearchResult } from '@/types/api.js';

// Create a fresh QueryClient for each test.
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
}

// Mock search results without overlapping words to avoid highlighting issues.
const mockSearchResults: SearchResult[] = [
  {
    type: 'message',
    id: 1,
    title: 'Team Meeting Notes',
    snippet: 'Summary of the weekly team sync.',
    created_at: new Date().toISOString(),
  },
  {
    type: 'thread',
    id: 2,
    title: 'Budget Discussion',
    snippet: 'Quarterly budget review thread.',
    created_at: new Date().toISOString(),
  },
  {
    type: 'agent',
    id: 3,
    title: 'Agent Alpha',
    snippet: 'Handles automated tasks.',
    created_at: new Date().toISOString(),
  },
  {
    type: 'topic',
    id: 4,
    title: 'General Announcements',
    snippet: 'Company-wide updates and news.',
    created_at: new Date().toISOString(),
  },
];

// Test wrapper with routing.
function renderWithRouter(
  initialPath = '/search',
) {
  const queryClient = createTestQueryClient();

  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialPath]}>
          <Routes>
            <Route path="/search" element={<SearchResultsPage />} />
            <Route path="/inbox/thread/:id" element={<div>Thread View</div>} />
            <Route path="/agents/:id" element={<div>Agent View</div>} />
            <Route path="/topics/:id" element={<div>Topic View</div>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    ),
    queryClient,
  };
}

describe('SearchResultsPage', () => {
  beforeEach(() => {
    // Reset to default handlers.
    server.resetHandlers();
  });

  describe('rendering', () => {
    it('renders the page title', () => {
      renderWithRouter();

      expect(screen.getByRole('heading', { name: 'Search Results' })).toBeInTheDocument();
    });

    it('renders the search input', () => {
      renderWithRouter();

      expect(screen.getByPlaceholderText(/search messages/i)).toBeInTheDocument();
    });

    it('renders filter tabs', () => {
      renderWithRouter();

      expect(screen.getByRole('button', { name: /all/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /messages/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /threads/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /agents/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /topics/i })).toBeInTheDocument();
    });

    it('shows empty state when no query', () => {
      renderWithRouter();

      expect(screen.getByText(/enter a search term/i)).toBeInTheDocument();
    });
  });

  describe('search from URL params', () => {
    it('uses query from URL', async () => {
      server.use(
        http.get('/api/v1/search', ({ request }) => {
          const url = new URL(request.url);
          const q = url.searchParams.get('q');
          expect(q).toBe('meeting');
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=meeting');

      await waitFor(() => {
        // The result title should be in the document.
        expect(screen.getByText(/Team/)).toBeInTheDocument();
      });
    });

    it('shows result count for query', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=meeting');

      await waitFor(() => {
        expect(screen.getByText(/4 results/i)).toBeInTheDocument();
      });
    });

    it('uses type filter from URL', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test&type=message');

      await waitFor(() => {
        // Wait for results to load by checking for a link.
        expect(screen.getByRole('link')).toBeInTheDocument();
      });

      // Messages filter should be active.
      const messagesButton = screen.getByRole('button', { name: /messages/i });
      expect(messagesButton).toHaveClass('bg-blue-600');
    });
  });

  describe('search functionality', () => {
    it('submits search form', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/search', ({ request }) => {
          const url = new URL(request.url);
          const q = url.searchParams.get('q');
          if (q === 'test') {
            return HttpResponse.json({ data: mockSearchResults });
          }
          return HttpResponse.json({ data: [] });
        }),
      );

      renderWithRouter();

      const input = screen.getByPlaceholderText(/search messages/i);
      await user.type(input, 'test');
      await user.click(screen.getByRole('button', { name: 'Search' }));

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });
    });

    it('disables search button when input is empty', () => {
      renderWithRouter();

      expect(screen.getByRole('button', { name: 'Search' })).toBeDisabled();
    });

    it('enables search button when input has value', async () => {
      const user = userEvent.setup();
      renderWithRouter();

      const input = screen.getByPlaceholderText(/search messages/i);
      await user.type(input, 'test');

      expect(screen.getByRole('button', { name: 'Search' })).not.toBeDisabled();
    });

    it('clears input when clear button is clicked', async () => {
      const user = userEvent.setup();
      renderWithRouter();

      const input = screen.getByPlaceholderText(/search messages/i);
      await user.type(input, 'test');

      // Find and click the clear button (x icon).
      const clearButton = screen.getByRole('button', { name: '' });
      await user.click(clearButton);

      expect(input).toHaveValue('');
    });
  });

  describe('filter tabs', () => {
    it('filters results by type', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        // Wait for all 4 links to appear.
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      // Click Messages filter.
      await user.click(screen.getByRole('button', { name: /messages/i }));

      // Should only show 1 message (filter applied client-side).
      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(1);
      });
    });

    it('shows count on filter tabs', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      // All tab should show total count.
      const allButton = screen.getByRole('button', { name: /all/i });
      expect(allButton).toHaveTextContent('4');
    });

    it('highlights selected filter', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      // Click Agents filter.
      await user.click(screen.getByRole('button', { name: /agents/i }));

      const agentsButton = screen.getByRole('button', { name: /agents/i });
      expect(agentsButton).toHaveClass('bg-blue-600');
    });
  });

  describe('search results display', () => {
    it('renders result items with correct content', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      // Check result shows type badges.
      expect(screen.getByText('Message')).toBeInTheDocument();
      expect(screen.getByText('Thread')).toBeInTheDocument();
    });

    it('highlights search query in results', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=Team');

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      // Check for highlighted text (using mark element).
      const marks = document.querySelectorAll('mark');
      expect(marks.length).toBeGreaterThan(0);
    });

    it('shows type badges for each result type', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getAllByRole('link')).toHaveLength(4);
      });

      expect(screen.getByText('Message')).toBeInTheDocument();
      expect(screen.getByText('Thread')).toBeInTheDocument();
      expect(screen.getByText('Agent')).toBeInTheDocument();
      expect(screen.getByText('Topic')).toBeInTheDocument();
    });

    it('renders results as links', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        const links = screen.getAllByRole('link');
        expect(links).toHaveLength(4);
      });
    });
  });

  describe('loading state', () => {
    it('shows spinner while searching', async () => {
      let resolveSearch: (() => void) | undefined;
      const searchPromise = new Promise<void>((resolve) => {
        resolveSearch = resolve;
      });

      server.use(
        http.get('/api/v1/search', async () => {
          await searchPromise;
          return HttpResponse.json({ data: mockSearchResults });
        }),
      );

      renderWithRouter('/search?q=test');

      // Should show spinner.
      expect(screen.getByText('Searching...')).toBeInTheDocument();

      // Resolve the search.
      resolveSearch!();

      await waitFor(() => {
        expect(screen.queryByText('Searching...')).not.toBeInTheDocument();
      });
    });
  });

  describe('empty state', () => {
    it('shows empty state when no results', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: [] });
        }),
      );

      renderWithRouter('/search?q=nonexistent');

      await waitFor(() => {
        expect(screen.getByText('No results found')).toBeInTheDocument();
      });

      expect(screen.getByText(/no results found for "nonexistent"/i)).toBeInTheDocument();
    });

    it('shows filter hint in empty state when filtered', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: [mockSearchResults[0]] }); // Only message.
        }),
      );

      renderWithRouter('/search?q=test&type=agent');

      await waitFor(() => {
        expect(screen.getByText('No results found')).toBeInTheDocument();
      });

      expect(screen.getByText(/try removing the filter/i)).toBeInTheDocument();
    });
  });

  describe('error handling', () => {
    it('shows error message on failure', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Search failed' } },
            { status: 500 },
          );
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        // The API error shows in a red container.
        expect(screen.getByText('Search failed')).toBeInTheDocument();
      });
    });

    it('shows clear button on error', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json(
            { error: { code: 'server_error', message: 'Search failed' } },
            { status: 500 },
          );
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /clear search/i })).toBeInTheDocument();
      });
    });
  });

  describe('navigation', () => {
    it('navigates to thread view on message click', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: [mockSearchResults[0]] });
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getByRole('link')).toBeInTheDocument();
      });

      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/inbox/thread/1');
    });

    it('navigates to agent view on agent click', async () => {
      server.use(
        http.get('/api/v1/search', () => {
          return HttpResponse.json({ data: [mockSearchResults[2]] }); // Agent result.
        }),
      );

      renderWithRouter('/search?q=test');

      await waitFor(() => {
        expect(screen.getByRole('link')).toBeInTheDocument();
      });

      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/agents/3');
    });
  });
});
