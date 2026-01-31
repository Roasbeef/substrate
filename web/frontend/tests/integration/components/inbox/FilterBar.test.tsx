// Integration tests for FilterBar component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FilterBar, SimpleFilterBar } from '@/components/inbox/FilterBar.js';
import type { FilterType } from '@/components/inbox/CategoryTabs.js';

describe('FilterBar', () => {
  const defaultProps = {
    filter: 'all' as FilterType,
    onFilterChange: vi.fn(),
    selectedCount: 0,
    totalCount: 10,
    allSelected: false,
    isIndeterminate: false,
    onSelectAll: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders filter tabs', () => {
    render(<FilterBar {...defaultProps} />);

    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('Starred')).toBeInTheDocument();
  });

  it('renders select all checkbox', () => {
    render(<FilterBar {...defaultProps} />);

    expect(screen.getByRole('checkbox')).toBeInTheDocument();
  });

  it('calls onSelectAll when checkbox is clicked', async () => {
    const user = userEvent.setup();
    const onSelectAll = vi.fn();
    render(<FilterBar {...defaultProps} onSelectAll={onSelectAll} />);

    await user.click(screen.getByRole('checkbox'));

    expect(onSelectAll).toHaveBeenCalledWith(true);
  });

  it('displays total message count when no selection', () => {
    render(<FilterBar {...defaultProps} totalCount={25} />);

    expect(screen.getByText('25 messages')).toBeInTheDocument();
  });

  it('displays selected count when messages are selected', () => {
    render(<FilterBar {...defaultProps} selectedCount={3} />);

    expect(screen.getByText('3 selected')).toBeInTheDocument();
    expect(screen.queryByText('10 messages')).not.toBeInTheDocument();
  });

  it('shows bulk action buttons when messages are selected', () => {
    render(
      <FilterBar
        {...defaultProps}
        selectedCount={2}
        onArchive={() => {}}
        onStar={() => {}}
        onDelete={() => {}}
      />,
    );

    expect(screen.getByLabelText('Archive selected')).toBeInTheDocument();
    expect(screen.getByLabelText('Star selected')).toBeInTheDocument();
    expect(screen.getByLabelText('Delete selected')).toBeInTheDocument();
  });

  it('does not show bulk actions when no selection', () => {
    render(
      <FilterBar
        {...defaultProps}
        selectedCount={0}
        onArchive={() => {}}
        onDelete={() => {}}
      />,
    );

    expect(screen.queryByLabelText('Archive selected')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Delete selected')).not.toBeInTheDocument();
  });

  it('calls onArchive when archive button is clicked', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(
      <FilterBar {...defaultProps} selectedCount={1} onArchive={onArchive} />,
    );

    await user.click(screen.getByLabelText('Archive selected'));

    expect(onArchive).toHaveBeenCalled();
  });

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <FilterBar {...defaultProps} selectedCount={1} onDelete={onDelete} />,
    );

    await user.click(screen.getByLabelText('Delete selected'));

    expect(onDelete).toHaveBeenCalled();
  });

  it('calls onStar when star button is clicked', async () => {
    const user = userEvent.setup();
    const onStar = vi.fn();
    render(<FilterBar {...defaultProps} selectedCount={1} onStar={onStar} />);

    await user.click(screen.getByLabelText('Star selected'));

    expect(onStar).toHaveBeenCalled();
  });

  it('shows refresh button when onRefresh is provided', () => {
    render(<FilterBar {...defaultProps} onRefresh={() => {}} />);

    expect(screen.getByLabelText('Refresh')).toBeInTheDocument();
  });

  it('calls onRefresh when refresh button is clicked', async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    render(<FilterBar {...defaultProps} onRefresh={onRefresh} />);

    await user.click(screen.getByLabelText('Refresh'));

    expect(onRefresh).toHaveBeenCalled();
  });

  it('disables actions when isLoading', () => {
    render(
      <FilterBar
        {...defaultProps}
        selectedCount={1}
        onArchive={() => {}}
        onDelete={() => {}}
        isLoading
      />,
    );

    expect(screen.getByLabelText('Archive selected')).toBeDisabled();
    expect(screen.getByLabelText('Delete selected')).toBeDisabled();
  });

  it('calls onFilterChange when filter tab is clicked', async () => {
    const user = userEvent.setup();
    const onFilterChange = vi.fn();
    render(<FilterBar {...defaultProps} onFilterChange={onFilterChange} />);

    await user.click(screen.getByText('Unread'));

    expect(onFilterChange).toHaveBeenCalledWith('unread');
  });

  it('checks checkbox when allSelected is true', () => {
    render(<FilterBar {...defaultProps} allSelected />);

    expect(screen.getByRole('checkbox')).toBeChecked();
  });

  it('sets indeterminate state on checkbox', () => {
    render(<FilterBar {...defaultProps} isIndeterminate />);

    const checkbox = screen.getByRole('checkbox') as HTMLInputElement;
    expect(checkbox.indeterminate).toBe(true);
  });

  it('applies custom className', () => {
    const { container } = render(
      <FilterBar {...defaultProps} className="custom-filter-bar" />,
    );

    expect(container.firstChild).toHaveClass('custom-filter-bar');
  });

  it('hides refresh button when messages are selected', () => {
    render(
      <FilterBar {...defaultProps} selectedCount={1} onRefresh={() => {}} />,
    );

    expect(screen.queryByLabelText('Refresh')).not.toBeInTheDocument();
  });
});

describe('SimpleFilterBar', () => {
  const defaultProps = {
    filter: 'all' as FilterType,
    onFilterChange: vi.fn(),
  };

  it('renders filter tabs', () => {
    render(<SimpleFilterBar {...defaultProps} />);

    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('Unread')).toBeInTheDocument();
    expect(screen.getByText('Starred')).toBeInTheDocument();
  });

  it('does not render checkbox', () => {
    render(<SimpleFilterBar {...defaultProps} />);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('displays total count when provided', () => {
    render(<SimpleFilterBar {...defaultProps} totalCount={15} />);

    expect(screen.getByText('15 messages')).toBeInTheDocument();
  });

  it('does not display total count when not provided', () => {
    render(<SimpleFilterBar {...defaultProps} />);

    expect(screen.queryByText(/messages/)).not.toBeInTheDocument();
  });

  it('shows refresh button when onRefresh is provided', () => {
    render(<SimpleFilterBar {...defaultProps} onRefresh={() => {}} />);

    expect(screen.getByLabelText('Refresh')).toBeInTheDocument();
  });

  it('calls onRefresh when clicked', async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    render(<SimpleFilterBar {...defaultProps} onRefresh={onRefresh} />);

    await user.click(screen.getByLabelText('Refresh'));

    expect(onRefresh).toHaveBeenCalled();
  });

  it('disables refresh when isLoading', () => {
    render(
      <SimpleFilterBar {...defaultProps} onRefresh={() => {}} isLoading />,
    );

    expect(screen.getByLabelText('Refresh')).toBeDisabled();
  });

  it('animates refresh icon when loading', () => {
    render(
      <SimpleFilterBar {...defaultProps} onRefresh={() => {}} isLoading />,
    );

    const icon = screen.getByLabelText('Refresh').querySelector('svg');
    expect(icon).toHaveClass('animate-spin');
  });

  it('calls onFilterChange when filter is selected', async () => {
    const user = userEvent.setup();
    const onFilterChange = vi.fn();
    render(<SimpleFilterBar {...defaultProps} onFilterChange={onFilterChange} />);

    await user.click(screen.getByText('Starred'));

    expect(onFilterChange).toHaveBeenCalledWith('starred');
  });

  it('disables filter tabs when isLoading', () => {
    render(<SimpleFilterBar {...defaultProps} isLoading />);

    const buttons = screen.getAllByRole('button');
    // Exclude refresh button if present, check filter buttons are disabled.
    const filterButtons = buttons.filter((b) => !b.getAttribute('aria-label'));
    filterButtons.forEach((button) => {
      expect(button).toBeDisabled();
    });
  });

  it('applies custom className', () => {
    const { container } = render(
      <SimpleFilterBar {...defaultProps} className="custom-simple-bar" />,
    );

    expect(container.firstChild).toHaveClass('custom-simple-bar');
  });
});
