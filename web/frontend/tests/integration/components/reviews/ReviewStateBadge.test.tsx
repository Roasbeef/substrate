// Integration tests for ReviewStateBadge component.

import { describe, it, expect } from 'vitest';
import { render, screen } from '../../../utils.js';
import { ReviewStateBadge } from '@/components/reviews/ReviewStateBadge.js';
import type { ReviewState } from '@/types/api.js';

describe('ReviewStateBadge', () => {
  const states: Array<{ state: ReviewState; label: string }> = [
    { state: 'pending_review', label: 'Pending' },
    { state: 'under_review', label: 'In Review' },
    { state: 'changes_requested', label: 'Changes Requested' },
    { state: 'approved', label: 'Approved' },
    { state: 'rejected', label: 'Rejected' },
    { state: 'cancelled', label: 'Cancelled' },
  ];

  states.forEach(({ state, label }) => {
    it(`renders "${label}" for state "${state}"`, () => {
      render(<ReviewStateBadge state={state} />);

      expect(screen.getByText(label)).toBeInTheDocument();
    });
  });

  it('applies custom className', () => {
    const { container } = render(
      <ReviewStateBadge state="approved" className="extra-class" />,
    );

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('extra-class');
  });

  it('renders with correct base styling', () => {
    const { container } = render(<ReviewStateBadge state="approved" />);

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('rounded-full');
    expect(badge?.className).toContain('text-xs');
    expect(badge?.className).toContain('font-medium');
  });

  it('uses green styling for approved state', () => {
    const { container } = render(<ReviewStateBadge state="approved" />);

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('bg-green-100');
    expect(badge?.className).toContain('text-green-800');
  });

  it('uses red styling for rejected state', () => {
    const { container } = render(<ReviewStateBadge state="rejected" />);

    const badge = container.firstElementChild;
    expect(badge?.className).toContain('bg-red-100');
    expect(badge?.className).toContain('text-red-800');
  });
});
