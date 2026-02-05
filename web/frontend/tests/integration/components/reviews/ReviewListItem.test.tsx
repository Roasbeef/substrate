// Integration tests for ReviewListItem component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../../utils.js';
import { ReviewListItem } from '@/components/reviews/ReviewListItem.js';
import { BrowserRouter } from 'react-router-dom';
import type { ReviewSummary } from '@/types/api.js';

// Wrapper with router context.
function renderWithRouter(ui: React.ReactElement) {
  return render(<BrowserRouter>{ui}</BrowserRouter>);
}

const mockReview: ReviewSummary = {
  review_id: 'abc123',
  thread_id: 'thread-1',
  requester_id: 1,
  branch: 'feature/add-reviews',
  state: 'under_review',
  review_type: 'full',
  created_at: Math.floor(Date.now() / 1000) - 3600,
};

describe('ReviewListItem', () => {
  it('renders branch name', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    expect(screen.getByText('feature/add-reviews')).toBeInTheDocument();
  });

  it('renders state badge', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    expect(screen.getByText('In Review')).toBeInTheDocument();
  });

  it('renders review type', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    expect(screen.getByText('full')).toBeInTheDocument();
  });

  it('renders truncated review ID', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    // The component renders review_id.slice(0, 8) = "abc123" (since it's only 6 chars).
    expect(screen.getByText(/ID: abc123/)).toBeInTheDocument();
  });

  it('links to review detail page', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/reviews/abc123');
  });

  it('renders relative time for recent reviews', () => {
    renderWithRouter(<ReviewListItem review={mockReview} />);

    // Should show something like "1h ago" for a review created 1 hour ago.
    expect(screen.getByText('1h ago')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = renderWithRouter(
      <ReviewListItem review={mockReview} className="my-custom-class" />,
    );

    const link = container.querySelector('a');
    expect(link?.className).toContain('my-custom-class');
  });

  it('renders different review types with correct styling', () => {
    const securityReview = { ...mockReview, review_type: 'security' as const };
    renderWithRouter(<ReviewListItem review={securityReview} />);

    expect(screen.getByText('security')).toBeInTheDocument();
  });

  it('renders approved state badge', () => {
    const approvedReview = { ...mockReview, state: 'approved' as const };
    renderWithRouter(<ReviewListItem review={approvedReview} />);

    expect(screen.getByText('Approved')).toBeInTheDocument();
  });
});
