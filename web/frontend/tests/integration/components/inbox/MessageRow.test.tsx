// Integration tests for MessageRow component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MessageRow, CompactMessageRow } from '@/components/inbox/MessageRow.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Mock message data.
const createMockMessage = (
  overrides: Partial<MessageWithRecipients> = {},
): MessageWithRecipients => ({
  id: 1,
  sender_id: 1,
  sender_name: 'Test Sender',
  subject: 'Test Subject',
  body: 'This is the message body content that may be truncated.',
  priority: 'normal',
  created_at: new Date().toISOString(),
  recipients: [
    {
      message_id: 1,
      agent_id: 2,
      agent_name: 'RecipientAgent',
      state: 'unread',
      is_starred: false,
      is_archived: false,
    },
  ],
  ...overrides,
});

describe('MessageRow', () => {
  const mockMessage = createMockMessage();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders sender name', () => {
    render(<MessageRow message={mockMessage} />);

    expect(screen.getByText('Test Sender')).toBeInTheDocument();
  });

  it('renders subject', () => {
    render(<MessageRow message={mockMessage} />);

    expect(screen.getByText('Test Subject')).toBeInTheDocument();
  });

  it('renders truncated body preview', () => {
    render(<MessageRow message={mockMessage} />);

    expect(
      screen.getByText(/This is the message body content/),
    ).toBeInTheDocument();
  });

  it('renders timestamp', () => {
    render(<MessageRow message={mockMessage} />);

    // Just now or similar time format.
    expect(
      screen.getByText(/Just now|ago|Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/),
    ).toBeInTheDocument();
  });

  it('renders sender display', () => {
    render(<MessageRow message={mockMessage} />);

    // MessageRow shows sender name directly (no Avatar initials).
    expect(screen.getByText('Test Sender')).toBeInTheDocument();
  });

  it('shows checkbox when showCheckbox is true', () => {
    render(<MessageRow message={mockMessage} showCheckbox />);

    expect(
      screen.getByRole('checkbox', { name: /select message/i }),
    ).toBeInTheDocument();
  });

  it('hides checkbox when showCheckbox is false', () => {
    render(<MessageRow message={mockMessage} showCheckbox={false} />);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('renders star button', () => {
    render(<MessageRow message={mockMessage} />);

    expect(
      screen.getByRole('button', { name: /star message/i }),
    ).toBeInTheDocument();
  });

  it('calls onStar when star button is clicked', async () => {
    const user = userEvent.setup();
    const onStar = vi.fn();
    render(<MessageRow message={mockMessage} onStar={onStar} />);

    await user.click(screen.getByRole('button', { name: /star message/i }));

    expect(onStar).toHaveBeenCalledWith(true);
  });

  it('calls onStar with false when message is already starred', async () => {
    const user = userEvent.setup();
    const onStar = vi.fn();
    const starredMessage = createMockMessage({
      recipients: [
        {
          message_id: 1,
          agent_id: 2,
          agent_name: 'RecipientAgent',
          state: 'read',
          is_starred: true,
          is_archived: false,
        },
      ],
    });
    render(<MessageRow message={starredMessage} onStar={onStar} />);

    await user.click(screen.getByRole('button', { name: /unstar message/i }));

    expect(onStar).toHaveBeenCalledWith(false);
  });

  it('calls onClick when row is clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<MessageRow message={mockMessage} onClick={onClick} />);

    await user.click(screen.getByText('Test Subject'));

    expect(onClick).toHaveBeenCalled();
  });

  it('makes row focusable when onClick is provided', () => {
    const { container } = render(
      <MessageRow message={mockMessage} onClick={() => {}} />,
    );

    // The row div has role="button" and tabindex.
    const row = container.querySelector('[role="button"]');
    expect(row).toHaveAttribute('tabindex', '0');
  });

  it('handles keyboard activation with Enter', () => {
    const onClick = vi.fn();
    const { container } = render(
      <MessageRow message={mockMessage} onClick={onClick} />,
    );

    const row = container.querySelector('[role="button"]');
    expect(row).not.toBeNull();
    fireEvent.keyDown(row!, { key: 'Enter' });

    expect(onClick).toHaveBeenCalled();
  });

  it('handles keyboard activation with Space', () => {
    const onClick = vi.fn();
    const { container } = render(
      <MessageRow message={mockMessage} onClick={onClick} />,
    );

    const row = container.querySelector('[role="button"]');
    expect(row).not.toBeNull();
    fireEvent.keyDown(row!, { key: ' ' });

    expect(onClick).toHaveBeenCalled();
  });

  it('calls onSelect when checkbox is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<MessageRow message={mockMessage} onSelect={onSelect} />);

    await user.click(screen.getByRole('checkbox'));

    expect(onSelect).toHaveBeenCalledWith(true);
  });

  it('checkbox click does not trigger row click', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const onSelect = vi.fn();
    render(
      <MessageRow
        message={mockMessage}
        onClick={onClick}
        onSelect={onSelect}
      />,
    );

    await user.click(screen.getByRole('checkbox'));

    expect(onSelect).toHaveBeenCalled();
    expect(onClick).not.toHaveBeenCalled();
  });

  it('renders selected state', () => {
    const { container } = render(
      <MessageRow message={mockMessage} isSelected />,
    );

    // Selected state uses bg-blue-50.
    expect(container.firstChild).toHaveClass('bg-blue-50');
  });

  it('renders unread state', () => {
    const unreadMessage = createMockMessage({
      recipients: [
        {
          message_id: 1,
          agent_id: 2,
          agent_name: 'RecipientAgent',
          state: 'unread',
          is_starred: false,
          is_archived: false,
        },
      ],
    });
    const { container } = render(<MessageRow message={unreadMessage} />);

    // Unread messages have font-medium styling and show blue dot indicator.
    expect(container.firstChild).toHaveClass('font-medium');
    expect(screen.getByLabelText('Unread message')).toBeInTheDocument();
  });

  it('renders priority badge for urgent messages', () => {
    const urgentMessage = createMockMessage({ priority: 'urgent' });
    render(<MessageRow message={urgentMessage} />);

    expect(screen.getByText(/urgent/i)).toBeInTheDocument();
  });

  it('does not render priority badge for normal messages', () => {
    render(<MessageRow message={mockMessage} />);

    expect(screen.queryByText(/normal/i)).not.toBeInTheDocument();
  });

  it('shows action buttons on hover', () => {
    render(
      <MessageRow
        message={mockMessage}
        onArchive={() => {}}
        onSnooze={() => {}}
        onDelete={() => {}}
      />,
    );

    // Action buttons should exist but be hidden by default.
    expect(screen.getByLabelText('Archive')).toBeInTheDocument();
    expect(screen.getByLabelText('Snooze')).toBeInTheDocument();
    expect(screen.getByLabelText('Delete')).toBeInTheDocument();
  });

  it('calls onArchive when archive button is clicked', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(<MessageRow message={mockMessage} onArchive={onArchive} />);

    await user.click(screen.getByLabelText('Archive'));

    expect(onArchive).toHaveBeenCalled();
  });

  it('calls onSnooze when snooze button is clicked', async () => {
    const user = userEvent.setup();
    const onSnooze = vi.fn();
    render(<MessageRow message={mockMessage} onSnooze={onSnooze} />);

    await user.click(screen.getByLabelText('Snooze'));

    expect(onSnooze).toHaveBeenCalled();
  });

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<MessageRow message={mockMessage} onDelete={onDelete} />);

    await user.click(screen.getByLabelText('Delete'));

    expect(onDelete).toHaveBeenCalled();
  });

  it('action button clicks do not trigger row click', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const onArchive = vi.fn();
    render(
      <MessageRow
        message={mockMessage}
        onClick={onClick}
        onArchive={onArchive}
      />,
    );

    await user.click(screen.getByLabelText('Archive'));

    expect(onArchive).toHaveBeenCalled();
    expect(onClick).not.toHaveBeenCalled();
  });

  it('applies custom className', () => {
    const { container } = render(
      <MessageRow message={mockMessage} className="custom-row" />,
    );

    expect(container.firstChild).toHaveClass('custom-row');
  });
});

describe('CompactMessageRow', () => {
  const mockMessage = createMockMessage();

  it('renders subject', () => {
    render(<CompactMessageRow message={mockMessage} />);

    expect(screen.getByText('Test Subject')).toBeInTheDocument();
  });

  it('renders avatar', () => {
    render(<CompactMessageRow message={mockMessage} />);

    expect(screen.getByText('TS')).toBeInTheDocument();
  });

  it('renders timestamp', () => {
    render(<CompactMessageRow message={mockMessage} />);

    expect(
      screen.getByText(/Just now|ago|Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/),
    ).toBeInTheDocument();
  });

  it('shows star icon when message is starred', () => {
    const starredMessage = createMockMessage({
      recipients: [
        {
          message_id: 1,
          agent_id: 2,
          agent_name: 'RecipientAgent',
          state: 'read',
          is_starred: true,
          is_archived: false,
        },
      ],
    });
    const { container } = render(<CompactMessageRow message={starredMessage} />);

    const starIcon = container.querySelector('svg');
    expect(starIcon).toHaveClass('text-yellow-500');
  });

  it('does not show star icon when message is not starred', () => {
    const { container } = render(<CompactMessageRow message={mockMessage} />);

    const starIcon = container.querySelector('svg.text-yellow-500');
    expect(starIcon).not.toBeInTheDocument();
  });

  it('calls onClick when row is clicked', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<CompactMessageRow message={mockMessage} onClick={onClick} />);

    await user.click(screen.getByText('Test Subject'));

    expect(onClick).toHaveBeenCalled();
  });

  it('applies unread styling', () => {
    const unreadMessage = createMockMessage({
      recipients: [
        {
          message_id: 1,
          agent_id: 2,
          agent_name: 'RecipientAgent',
          state: 'unread',
          is_starred: false,
          is_archived: false,
        },
      ],
    });
    const { container } = render(<CompactMessageRow message={unreadMessage} />);

    expect(container.firstChild).toHaveClass('bg-blue-50/50');
  });

  it('does not have checkbox', () => {
    render(<CompactMessageRow message={mockMessage} />);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('does not have action buttons', () => {
    render(<CompactMessageRow message={mockMessage} />);

    expect(screen.queryByLabelText('Archive')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Delete')).not.toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <CompactMessageRow message={mockMessage} className="custom-compact" />,
    );

    expect(container.firstChild).toHaveClass('custom-compact');
  });
});
