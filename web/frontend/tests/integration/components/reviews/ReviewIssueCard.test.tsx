// Integration tests for ReviewIssueCard component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '../../../utils.js';
import { ReviewIssueCard } from '@/components/reviews/ReviewIssueCard.js';
import type { ReviewIssue } from '@/types/api.js';

// Mock issue data.
const mockIssue: ReviewIssue = {
  id: 1,
  review_id: 'abc123',
  iteration_num: 1,
  issue_type: 'bug',
  severity: 'major',
  file_path: 'internal/review/service.go',
  line_start: 42,
  line_end: 50,
  title: 'Missing nil check before dereference',
  description: 'The pointer could be nil when the review is not found.',
  code_snippet: 'review := s.reviews[id]\nreview.State = "approved"',
  suggestion: 'Add a nil check: if review == nil { return ErrNotFound }',
  claude_md_ref: 'Code Style: Error Handling',
  status: 'open',
};

describe('ReviewIssueCard', () => {
  it('renders issue title', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    expect(
      screen.getByText('Missing nil check before dereference'),
    ).toBeInTheDocument();
  });

  it('renders severity badge', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    expect(screen.getByText('major')).toBeInTheDocument();
  });

  it('renders issue type badge', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    expect(screen.getByText('bug')).toBeInTheDocument();
  });

  it('renders file location with line numbers', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    expect(
      screen.getByText('internal/review/service.go:42-50'),
    ).toBeInTheDocument();
  });

  it('renders file path without line range for single line', () => {
    const singleLineIssue = { ...mockIssue, line_end: 42 };
    render(<ReviewIssueCard issue={singleLineIssue} />);

    expect(
      screen.getByText('internal/review/service.go:42'),
    ).toBeInTheDocument();
  });

  it('shows status indicator', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    expect(screen.getByText('[O] Open')).toBeInTheDocument();
  });

  it('shows fixed status for fixed issues', () => {
    const fixedIssue = { ...mockIssue, status: 'fixed' as const };
    render(<ReviewIssueCard issue={fixedIssue} />);

    expect(screen.getByText('[F] Fixed')).toBeInTheDocument();
  });

  it('toggles details on click', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    // Details should be hidden initially.
    expect(screen.queryByText('Description')).not.toBeInTheDocument();

    // Click to expand.
    fireEvent.click(screen.getByText('Show details'));

    // Details should now be visible.
    expect(screen.getByText('Description')).toBeInTheDocument();
    expect(
      screen.getByText('The pointer could be nil when the review is not found.'),
    ).toBeInTheDocument();

    // Click to collapse.
    fireEvent.click(screen.getByText('Hide details'));

    // Details should be hidden again.
    expect(screen.queryByText('Description')).not.toBeInTheDocument();
  });

  it('shows code snippet in expanded view', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.getByText('Code')).toBeInTheDocument();
    // The code snippet contains a newline, so use a function matcher.
    expect(
      screen.getByText((_content, element) => {
        return element?.tagName === 'PRE'
          && element.textContent?.includes('review := s.reviews[id]') === true;
      }),
    ).toBeInTheDocument();
  });

  it('shows suggestion in expanded view', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.getByText('Suggestion')).toBeInTheDocument();
    expect(
      screen.getByText('Add a nil check: if review == nil { return ErrNotFound }'),
    ).toBeInTheDocument();
  });

  it('shows CLAUDE.md reference in expanded view', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.getByText('CLAUDE.md Reference')).toBeInTheDocument();
    expect(
      screen.getByText('Code Style: Error Handling'),
    ).toBeInTheDocument();
  });

  it('shows status change buttons for open issues', () => {
    const onStatusChange = vi.fn();
    render(
      <ReviewIssueCard issue={mockIssue} onStatusChange={onStatusChange} />,
    );

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.getByText('Mark Fixed')).toBeInTheDocument();
    expect(screen.getByText("Won't Fix")).toBeInTheDocument();
  });

  it('calls onStatusChange when Mark Fixed is clicked', () => {
    const onStatusChange = vi.fn();
    render(
      <ReviewIssueCard issue={mockIssue} onStatusChange={onStatusChange} />,
    );

    fireEvent.click(screen.getByText('Show details'));
    fireEvent.click(screen.getByText('Mark Fixed'));

    expect(onStatusChange).toHaveBeenCalledWith(1, 'fixed');
  });

  it('calls onStatusChange when Won\'t Fix is clicked', () => {
    const onStatusChange = vi.fn();
    render(
      <ReviewIssueCard issue={mockIssue} onStatusChange={onStatusChange} />,
    );

    fireEvent.click(screen.getByText('Show details'));
    fireEvent.click(screen.getByText("Won't Fix"));

    expect(onStatusChange).toHaveBeenCalledWith(1, 'wont_fix');
  });

  it('does not show status buttons for fixed issues', () => {
    const onStatusChange = vi.fn();
    const fixedIssue = { ...mockIssue, status: 'fixed' as const };
    render(
      <ReviewIssueCard issue={fixedIssue} onStatusChange={onStatusChange} />,
    );

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.queryByText('Mark Fixed')).not.toBeInTheDocument();
  });

  it('does not show status buttons without onStatusChange', () => {
    render(<ReviewIssueCard issue={mockIssue} />);

    fireEvent.click(screen.getByText('Show details'));

    expect(screen.queryByText('Mark Fixed')).not.toBeInTheDocument();
  });

  it('applies opacity to fixed issues', () => {
    const fixedIssue = { ...mockIssue, status: 'fixed' as const };
    const { container } = render(<ReviewIssueCard issue={fixedIssue} />);

    const card = container.firstElementChild;
    expect(card?.className).toContain('opacity-60');
  });

  it('does not apply opacity to open issues', () => {
    const { container } = render(<ReviewIssueCard issue={mockIssue} />);

    const card = container.firstElementChild;
    expect(card?.className).not.toContain('opacity-60');
  });

  it('disables buttons when isUpdating is true', () => {
    const onStatusChange = vi.fn();
    render(
      <ReviewIssueCard
        issue={mockIssue}
        onStatusChange={onStatusChange}
        isUpdating
      />,
    );

    fireEvent.click(screen.getByText('Show details'));

    const fixedBtn = screen.getByText('Mark Fixed');
    expect(fixedBtn).toBeDisabled();
  });
});
