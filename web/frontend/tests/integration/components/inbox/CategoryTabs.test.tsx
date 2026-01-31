// Integration tests for CategoryTabs component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  CategoryTabs,
  FilterTabs,
  type InboxCategory,
  type FilterType,
} from '@/components/inbox/CategoryTabs.js';

describe('CategoryTabs', () => {
  const defaultProps = {
    selected: 'primary' as InboxCategory,
    onSelect: vi.fn(),
  };

  it('renders default tabs', () => {
    render(<CategoryTabs {...defaultProps} />);

    expect(screen.getByText('Primary')).toBeInTheDocument();
    expect(screen.getByText('Agents')).toBeInTheDocument();
    expect(screen.getByText('Topics')).toBeInTheDocument();
  });

  it('marks selected tab as current', () => {
    render(<CategoryTabs {...defaultProps} selected="primary" />);

    const primaryButton = screen.getByText('Primary').closest('button');
    expect(primaryButton).toHaveAttribute('aria-current', 'true');
  });

  it('calls onSelect when tab is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CategoryTabs {...defaultProps} onSelect={onSelect} />);

    await user.click(screen.getByText('Agents'));

    expect(onSelect).toHaveBeenCalledWith('agents');
  });

  it('calls onSelect with correct category for each tab', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CategoryTabs {...defaultProps} onSelect={onSelect} />);

    await user.click(screen.getByText('Primary'));
    expect(onSelect).toHaveBeenCalledWith('primary');

    await user.click(screen.getByText('Topics'));
    expect(onSelect).toHaveBeenCalledWith('topics');
  });

  it('disables tabs when disabled prop is true', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CategoryTabs {...defaultProps} onSelect={onSelect} disabled />);

    await user.click(screen.getByText('Agents'));

    expect(onSelect).not.toHaveBeenCalled();
  });

  it('renders custom tabs when provided', () => {
    const customTabs = [
      { id: 'primary' as InboxCategory, label: 'Custom Primary', count: 5 },
      { id: 'agents' as InboxCategory, label: 'Custom Agents', count: 3 },
    ];

    render(<CategoryTabs {...defaultProps} tabs={customTabs} />);

    expect(screen.getByText('Custom Primary')).toBeInTheDocument();
    expect(screen.getByText('Custom Agents')).toBeInTheDocument();
    expect(screen.queryByText('Topics')).not.toBeInTheDocument();
  });

  it('displays count badge when count is provided', () => {
    const tabsWithCount = [
      { id: 'primary' as InboxCategory, label: 'Primary', count: 10 },
    ];

    render(<CategoryTabs {...defaultProps} tabs={tabsWithCount} />);

    expect(screen.getByText('10')).toBeInTheDocument();
  });

  it('does not display count badge when count is zero', () => {
    const tabsWithZeroCount = [
      { id: 'primary' as InboxCategory, label: 'Primary', count: 0 },
    ];

    render(<CategoryTabs {...defaultProps} tabs={tabsWithZeroCount} />);

    expect(screen.queryByText('0')).not.toBeInTheDocument();
  });

  it('renders with underline variant by default', () => {
    const { container } = render(<CategoryTabs {...defaultProps} />);

    const nav = container.querySelector('nav');
    expect(nav).toHaveClass('border-b');
  });

  it('renders with pills variant when specified', () => {
    const { container } = render(
      <CategoryTabs {...defaultProps} variant="pills" />,
    );

    const nav = container.querySelector('nav');
    expect(nav).not.toHaveClass('border-b');
    expect(nav).toHaveClass('gap-2');
  });

  it('renders with custom icon in tab', () => {
    const tabsWithIcon = [
      {
        id: 'primary' as InboxCategory,
        label: 'Primary',
        icon: <span data-testid="tab-icon">Icon</span>,
      },
    ];

    render(<CategoryTabs {...defaultProps} tabs={tabsWithIcon} />);

    expect(screen.getByTestId('tab-icon')).toBeInTheDocument();
  });

  it('has correct navigation aria label', () => {
    render(<CategoryTabs {...defaultProps} />);

    expect(screen.getByRole('navigation')).toHaveAttribute(
      'aria-label',
      'Category tabs',
    );
  });

  it('applies custom className', () => {
    const { container } = render(
      <CategoryTabs {...defaultProps} className="custom-tabs" />,
    );

    expect(container.firstChild).toHaveClass('custom-tabs');
  });
});

describe('FilterTabs', () => {
  const defaultProps = {
    selected: 'all' as FilterType,
    onSelect: vi.fn(),
  };

  it('renders all filter options', () => {
    render(<FilterTabs {...defaultProps} />);

    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('Starred')).toBeInTheDocument();
  });

  it('marks selected filter as current', () => {
    render(<FilterTabs {...defaultProps} selected="unread" />);

    const unreadButton = screen.getByText('Unread').closest('button');
    expect(unreadButton).toHaveAttribute('aria-current', 'true');
  });

  it('calls onSelect when filter is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<FilterTabs {...defaultProps} onSelect={onSelect} />);

    await user.click(screen.getByText('Starred'));

    expect(onSelect).toHaveBeenCalledWith('starred');
  });

  it('calls onSelect with all when All is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<FilterTabs {...defaultProps} selected="unread" onSelect={onSelect} />);

    await user.click(screen.getByText('All'));

    expect(onSelect).toHaveBeenCalledWith('all');
  });

  it('disables filters when disabled prop is true', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<FilterTabs {...defaultProps} onSelect={onSelect} disabled />);

    await user.click(screen.getByText('Unread'));

    expect(onSelect).not.toHaveBeenCalled();
  });

  it('applies selected styles to active filter', () => {
    render(<FilterTabs {...defaultProps} selected="starred" />);

    const starredButton = screen.getByText('Starred').closest('button');
    expect(starredButton).toHaveClass('bg-white');
    expect(starredButton).toHaveClass('shadow-sm');
  });

  it('applies default styles to inactive filters', () => {
    render(<FilterTabs {...defaultProps} selected="all" />);

    const unreadButton = screen.getByText('Unread').closest('button');
    expect(unreadButton).not.toHaveClass('bg-white');
    expect(unreadButton).toHaveClass('text-gray-600');
  });

  it('applies disabled styles when disabled', () => {
    render(<FilterTabs {...defaultProps} disabled />);

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toBeDisabled();
    });
  });

  it('renders inside rounded container', () => {
    const { container } = render(<FilterTabs {...defaultProps} />);

    const wrapper = container.firstChild;
    expect(wrapper).toHaveClass('rounded-lg');
    expect(wrapper).toHaveClass('bg-gray-100');
  });

  it('applies custom className', () => {
    const { container } = render(
      <FilterTabs {...defaultProps} className="custom-filter" />,
    );

    expect(container.firstChild).toHaveClass('custom-filter');
  });
});
