// Integration tests for StatsCards component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StatCard, InboxStats } from '@/components/inbox/StatsCards.js';

describe('StatCard', () => {
  it('renders label and value', () => {
    render(<StatCard label="Unread" value={42} />);

    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
  });

  it('renders with icon', () => {
    const icon = <span data-testid="custom-icon">Icon</span>;
    render(<StatCard label="Test" value={5} icon={icon} />);

    expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
  });

  it('applies default variant styles', () => {
    const { container } = render(<StatCard label="Test" value={0} />);

    const card = container.firstChild;
    expect(card).toHaveClass('bg-white');
  });

  it('applies blue variant styles', () => {
    const { container } = render(<StatCard label="Test" value={0} variant="blue" />);

    const card = container.firstChild;
    expect(card).toHaveClass('bg-blue-50');
  });

  it('applies yellow variant styles', () => {
    const { container } = render(<StatCard label="Test" value={0} variant="yellow" />);

    const card = container.firstChild;
    expect(card).toHaveClass('bg-yellow-50');
  });

  it('applies red variant styles', () => {
    const { container } = render(<StatCard label="Test" value={0} variant="red" />);

    const card = container.firstChild;
    expect(card).toHaveClass('bg-red-50');
  });

  it('applies green variant styles', () => {
    const { container } = render(<StatCard label="Test" value={0} variant="green" />);

    const card = container.firstChild;
    expect(card).toHaveClass('bg-green-50');
  });

  it('renders as button when clickable', () => {
    render(<StatCard label="Test" value={0} clickable />);

    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('renders as button when onClick is provided', () => {
    const onClick = vi.fn();
    render(<StatCard label="Test" value={0} onClick={onClick} />);

    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<StatCard label="Test" value={0} onClick={onClick} />);

    await user.click(screen.getByRole('button'));

    expect(onClick).toHaveBeenCalled();
  });

  it('renders as div when not clickable', () => {
    render(<StatCard label="Test" value={0} />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <StatCard label="Test" value={0} className="custom-class" />,
    );

    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('displays large numbers correctly', () => {
    render(<StatCard label="Count" value={999999} />);

    expect(screen.getByText('999999')).toBeInTheDocument();
  });

  it('displays zero correctly', () => {
    render(<StatCard label="Empty" value={0} />);

    expect(screen.getByText('0')).toBeInTheDocument();
  });
});

describe('InboxStats', () => {
  const defaultProps = {
    unread: 10,
    starred: 5,
    urgent: 2,
    acknowledged: 15,
  };

  it('renders all four stat cards', () => {
    render(<InboxStats {...defaultProps} />);

    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('Starred')).toBeInTheDocument();
    expect(screen.getByText('Urgent')).toBeInTheDocument();
    expect(screen.getByText('Completed')).toBeInTheDocument();
  });

  it('displays correct values', () => {
    render(<InboxStats {...defaultProps} />);

    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('15')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    const { container } = render(<InboxStats {...defaultProps} isLoading />);

    // Should show 4 skeleton cards.
    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBe(4);
  });

  it('does not show actual values when loading', () => {
    render(<InboxStats {...defaultProps} isLoading />);

    expect(screen.queryByText('Unread')).not.toBeInTheDocument();
    expect(screen.queryByText('10')).not.toBeInTheDocument();
  });

  it('makes cards clickable when onStatClick is provided', () => {
    const onStatClick = vi.fn();
    render(<InboxStats {...defaultProps} onStatClick={onStatClick} />);

    // All stat cards should be buttons.
    const buttons = screen.getAllByRole('button');
    expect(buttons.length).toBe(4);
  });

  it('calls onStatClick with correct stat type', async () => {
    const user = userEvent.setup();
    const onStatClick = vi.fn();
    render(<InboxStats {...defaultProps} onStatClick={onStatClick} />);

    const buttons = screen.getAllByRole('button');

    // Click unread card (first button).
    await user.click(buttons[0]);
    expect(onStatClick).toHaveBeenCalledWith('unread');

    // Click starred card (second button).
    await user.click(buttons[1]);
    expect(onStatClick).toHaveBeenCalledWith('starred');

    // Click urgent card (third button).
    await user.click(buttons[2]);
    expect(onStatClick).toHaveBeenCalledWith('urgent');

    // Click completed card (fourth button).
    await user.click(buttons[3]);
    expect(onStatClick).toHaveBeenCalledWith('acknowledged');
  });

  it('still renders cards as buttons when onStatClick is not provided', () => {
    // Note: The component always passes onClick functions, so cards are always buttons.
    // This is by design - the onClick just does nothing when onStatClick is not provided.
    render(<InboxStats {...defaultProps} />);

    // Cards are still buttons (onClick is always passed even if it does nothing).
    const buttons = screen.queryAllByRole('button');
    expect(buttons.length).toBe(4);
  });

  it('uses responsive grid layout', () => {
    const { container } = render(<InboxStats {...defaultProps} />);

    const grid = container.firstChild;
    expect(grid).toHaveClass('grid-cols-2');
    expect(grid).toHaveClass('md:grid-cols-4');
  });

  it('applies custom className', () => {
    const { container } = render(
      <InboxStats {...defaultProps} className="custom-stats" />,
    );

    expect(container.firstChild).toHaveClass('custom-stats');
  });

  it('renders with all zero values', () => {
    render(
      <InboxStats unread={0} starred={0} urgent={0} acknowledged={0} />,
    );

    // Should show 4 zeros.
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBe(4);
  });
});
