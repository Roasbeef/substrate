// Integration tests for MessageActionModals components.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  DeleteConfirmationModal,
  SnoozePickerModal,
  BulkActionsToolbar,
} from '@/components/inbox/MessageActionModals.js';

describe('DeleteConfirmationModal', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
  };

  it('renders when isOpen is true', () => {
    render(<DeleteConfirmationModal {...defaultProps} />);

    expect(screen.getByText('Delete message?')).toBeInTheDocument();
  });

  it('does not render when isOpen is false', () => {
    render(<DeleteConfirmationModal {...defaultProps} isOpen={false} />);

    expect(screen.queryByText('Delete message?')).not.toBeInTheDocument();
  });

  it('shows single delete message for count=1', () => {
    render(<DeleteConfirmationModal {...defaultProps} count={1} />);

    expect(screen.getByText('Delete message?')).toBeInTheDocument();
    expect(screen.getByText(/This action cannot be undone/)).toBeInTheDocument();
  });

  it('shows bulk delete message for count > 1', () => {
    render(<DeleteConfirmationModal {...defaultProps} count={5} />);

    expect(screen.getByText('Delete 5 messages?')).toBeInTheDocument();
    expect(screen.getByText(/delete 5 messages/)).toBeInTheDocument();
  });

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DeleteConfirmationModal {...defaultProps} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(onClose).toHaveBeenCalled();
  });

  it('calls onConfirm when Delete is clicked', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(<DeleteConfirmationModal {...defaultProps} onConfirm={onConfirm} />);

    await user.click(screen.getByRole('button', { name: /^delete$/i }));

    expect(onConfirm).toHaveBeenCalled();
  });

  it('shows loading state when isDeleting', () => {
    render(<DeleteConfirmationModal {...defaultProps} isDeleting />);

    // Delete button should show loading.
    const deleteButton = screen.getByRole('button', { name: /delete/i });
    expect(deleteButton).toBeInTheDocument();
  });

  it('disables buttons when isDeleting', () => {
    render(<DeleteConfirmationModal {...defaultProps} isDeleting />);

    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
  });

  it('has warning icon', () => {
    render(<DeleteConfirmationModal {...defaultProps} />);

    // Should have the warning/delete message content.
    expect(screen.getByText('Delete message?')).toBeInTheDocument();
    expect(screen.getByText(/cannot be undone/)).toBeInTheDocument();
  });
});

describe('SnoozePickerModal', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onSnooze: vi.fn(),
  };

  it('renders when isOpen is true', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    expect(screen.getByText('Snooze until')).toBeInTheDocument();
  });

  it('does not render when isOpen is false', () => {
    render(<SnoozePickerModal {...defaultProps} isOpen={false} />);

    expect(screen.queryByText('Snooze until')).not.toBeInTheDocument();
  });

  it('shows preset duration options', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    expect(screen.getByText('Later today')).toBeInTheDocument();
    expect(screen.getByText('Tomorrow morning')).toBeInTheDocument();
    expect(screen.getByText('Next week')).toBeInTheDocument();
    expect(screen.getByText('Next month')).toBeInTheDocument();
  });

  it('calls onSnooze when preset is clicked', async () => {
    const user = userEvent.setup();
    const onSnooze = vi.fn();
    render(<SnoozePickerModal {...defaultProps} onSnooze={onSnooze} />);

    await user.click(screen.getByText('Later today'));

    expect(onSnooze).toHaveBeenCalledWith(expect.any(String));
    // Verify it's a valid ISO date string.
    const calledWith = onSnooze.mock.calls[0]?.[0];
    expect(() => new Date(calledWith)).not.toThrow();
  });

  it('shows date and time inputs', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    // Has date input.
    expect(document.querySelector('input[type="date"]')).toBeInTheDocument();
    // Has time input.
    expect(document.querySelector('input[type="time"]')).toBeInTheDocument();
  });

  it('disables Snooze button when no custom date selected', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    // Find the custom Snooze button (not the preset buttons).
    const snoozeButtons = screen.getAllByRole('button', { name: /snooze/i });
    const customSnoozeButton = snoozeButtons[snoozeButtons.length - 1];
    expect(customSnoozeButton).toBeDisabled();
  });

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<SnoozePickerModal {...defaultProps} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(onClose).toHaveBeenCalled();
  });

  it('disables preset buttons when isSnoozing', () => {
    render(<SnoozePickerModal {...defaultProps} isSnoozing />);

    const laterToday = screen.getByText('Later today').closest('button');
    expect(laterToday).toBeDisabled();
  });

  it('has clock icon', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    // Should have the snooze title.
    expect(screen.getByText('Snooze until')).toBeInTheDocument();
  });

  it('shows custom date picker section', () => {
    render(<SnoozePickerModal {...defaultProps} />);

    expect(screen.getByText(/or pick a date/i)).toBeInTheDocument();
  });
});

describe('BulkActionsToolbar', () => {
  const defaultProps = {
    selectedCount: 3,
    onClearSelection: vi.fn(),
  };

  it('renders when selectedCount > 0', () => {
    render(<BulkActionsToolbar {...defaultProps} />);

    expect(screen.getByText('3 selected')).toBeInTheDocument();
  });

  it('does not render when selectedCount is 0', () => {
    render(<BulkActionsToolbar {...defaultProps} selectedCount={0} />);

    expect(screen.queryByText('selected')).not.toBeInTheDocument();
  });

  it('shows Star button when onStar provided', () => {
    render(<BulkActionsToolbar {...defaultProps} onStar={() => {}} />);

    expect(screen.getByRole('button', { name: /star/i })).toBeInTheDocument();
  });

  it('hides Star button when onStar not provided', () => {
    render(<BulkActionsToolbar {...defaultProps} />);

    expect(screen.queryByRole('button', { name: /star/i })).not.toBeInTheDocument();
  });

  it('shows Archive button when onArchive provided', () => {
    render(<BulkActionsToolbar {...defaultProps} onArchive={() => {}} />);

    expect(screen.getByRole('button', { name: /archive/i })).toBeInTheDocument();
  });

  it('shows Mark read button when onMarkRead provided', () => {
    render(<BulkActionsToolbar {...defaultProps} onMarkRead={() => {}} />);

    expect(screen.getByRole('button', { name: /mark read/i })).toBeInTheDocument();
  });

  it('shows Delete button when onDelete provided', () => {
    render(<BulkActionsToolbar {...defaultProps} onDelete={() => {}} />);

    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
  });

  it('shows Clear selection button', () => {
    render(<BulkActionsToolbar {...defaultProps} />);

    expect(screen.getByRole('button', { name: /clear selection/i })).toBeInTheDocument();
  });

  it('calls onStar when Star is clicked', async () => {
    const user = userEvent.setup();
    const onStar = vi.fn();
    render(<BulkActionsToolbar {...defaultProps} onStar={onStar} />);

    await user.click(screen.getByRole('button', { name: /star/i }));

    expect(onStar).toHaveBeenCalled();
  });

  it('calls onArchive when Archive is clicked', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(<BulkActionsToolbar {...defaultProps} onArchive={onArchive} />);

    await user.click(screen.getByRole('button', { name: /archive/i }));

    expect(onArchive).toHaveBeenCalled();
  });

  it('calls onMarkRead when Mark read is clicked', async () => {
    const user = userEvent.setup();
    const onMarkRead = vi.fn();
    render(<BulkActionsToolbar {...defaultProps} onMarkRead={onMarkRead} />);

    await user.click(screen.getByRole('button', { name: /mark read/i }));

    expect(onMarkRead).toHaveBeenCalled();
  });

  it('calls onDelete when Delete is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<BulkActionsToolbar {...defaultProps} onDelete={onDelete} />);

    await user.click(screen.getByRole('button', { name: /delete/i }));

    expect(onDelete).toHaveBeenCalled();
  });

  it('calls onClearSelection when Clear selection is clicked', async () => {
    const user = userEvent.setup();
    const onClearSelection = vi.fn();
    render(<BulkActionsToolbar {...defaultProps} onClearSelection={onClearSelection} />);

    await user.click(screen.getByRole('button', { name: /clear selection/i }));

    expect(onClearSelection).toHaveBeenCalled();
  });

  it('disables all buttons when isLoading', () => {
    render(
      <BulkActionsToolbar
        {...defaultProps}
        onStar={() => {}}
        onArchive={() => {}}
        onDelete={() => {}}
        isLoading
      />,
    );

    expect(screen.getByRole('button', { name: /star/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /archive/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /delete/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /clear selection/i })).toBeDisabled();
  });

  it('Delete button has red styling', () => {
    render(<BulkActionsToolbar {...defaultProps} onDelete={() => {}} />);

    const deleteButton = screen.getByRole('button', { name: /delete/i });
    expect(deleteButton).toHaveClass('text-red-600');
  });

  it('applies custom className', () => {
    const { container } = render(
      <BulkActionsToolbar {...defaultProps} className="custom-toolbar" />,
    );

    expect(container.firstChild).toHaveClass('custom-toolbar');
  });

  it('displays correct count for different selections', () => {
    const { rerender } = render(<BulkActionsToolbar {...defaultProps} selectedCount={1} />);
    expect(screen.getByText('1 selected')).toBeInTheDocument();

    rerender(<BulkActionsToolbar {...defaultProps} selectedCount={10} />);
    expect(screen.getByText('10 selected')).toBeInTheDocument();
  });
});
