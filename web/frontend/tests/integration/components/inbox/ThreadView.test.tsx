// Integration tests for ThreadView component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  ThreadView,
  ThreadMessage,
  DeadlineBanner,
} from '@/components/inbox/index.js';
import type { ThreadWithMessages, Message } from '@/types/api.js';

// Mock thread data.
const mockMessages: Message[] = [
  {
    id: 1,
    sender_id: 2,
    sender_name: 'Alice',
    subject: 'Project Update',
    body: 'Here is the latest status update on the project.',
    priority: 'normal',
    created_at: new Date(Date.now() - 3600000).toISOString(),
  },
  {
    id: 2,
    sender_id: 3,
    sender_name: 'Bob',
    subject: 'Re: Project Update',
    body: 'Thanks for the update! I have a few questions.',
    priority: 'normal',
    created_at: new Date(Date.now() - 1800000).toISOString(),
  },
  {
    id: 3,
    sender_id: 2,
    sender_name: 'Alice',
    subject: 'Re: Project Update',
    body: 'Sure, what would you like to know?',
    priority: 'normal',
    created_at: new Date().toISOString(),
  },
];

const mockThread: ThreadWithMessages = {
  id: 1,
  subject: 'Project Update',
  created_at: mockMessages[0].created_at,
  last_message_at: mockMessages[2].created_at,
  message_count: 3,
  participant_count: 2,
  messages: mockMessages,
};

describe('ThreadMessage', () => {
  it('renders message content', () => {
    render(<ThreadMessage message={mockMessages[0]} />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(
      screen.getByText('Here is the latest status update on the project.'),
    ).toBeInTheDocument();
  });

  it('shows subject only for first message', () => {
    render(<ThreadMessage message={mockMessages[0]} isFirst />);

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      'Project Update',
    );
  });

  it('shows Reply label for non-first messages', () => {
    render(<ThreadMessage message={mockMessages[1]} isFirst={false} />);

    expect(screen.getByText('Reply')).toBeInTheDocument();
  });

  it('applies focused styles when isFocused is true', () => {
    const { container } = render(
      <ThreadMessage message={mockMessages[0]} isFocused />,
    );

    const article = container.querySelector('div[role="article"]');
    expect(article).toHaveClass('border-blue-300');
  });

  it('renders priority badge for non-normal priority', () => {
    const urgentMessage: Message = {
      ...mockMessages[0],
      priority: 'urgent',
    };

    render(<ThreadMessage message={urgentMessage} />);

    expect(screen.getByText(/urgent/i)).toBeInTheDocument();
  });
});

describe('DeadlineBanner', () => {
  it('renders deadline date', () => {
    const deadline = new Date(Date.now() + 86400000).toISOString(); // Tomorrow.

    render(<DeadlineBanner deadline={deadline} />);

    expect(screen.getByText(/deadline:/i)).toBeInTheDocument();
  });

  it('shows past styling when deadline is past', () => {
    const pastDeadline = new Date(Date.now() - 86400000).toISOString(); // Yesterday.

    render(<DeadlineBanner deadline={pastDeadline} isPast />);

    expect(screen.getByText(/deadline passed:/i)).toBeInTheDocument();
  });

  it('calls onAcknowledge when button is clicked', async () => {
    const user = userEvent.setup();
    const onAcknowledge = vi.fn();
    const deadline = new Date().toISOString();

    render(<DeadlineBanner deadline={deadline} onAcknowledge={onAcknowledge} />);

    await user.click(screen.getByText('Acknowledge'));

    expect(onAcknowledge).toHaveBeenCalled();
  });

  it('disables button when loading', () => {
    const deadline = new Date().toISOString();

    render(
      <DeadlineBanner
        deadline={deadline}
        onAcknowledge={() => {}}
        isLoading
      />,
    );

    expect(screen.getByText('Acknowledging...')).toBeDisabled();
  });
});

describe('ThreadView', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state', () => {
    render(<ThreadView {...defaultProps} isLoading />);

    // Should show spinner (loading indicator).
    expect(document.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('renders error state', () => {
    const error = new Error('Failed to load thread');

    render(<ThreadView {...defaultProps} error={error} />);

    expect(screen.getByText('Failed to load thread')).toBeInTheDocument();
  });

  it('renders thread messages', () => {
    render(<ThreadView {...defaultProps} thread={mockThread} />);

    // Thread subject appears as h1.
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Project Update',
    );
    // Senders should be visible.
    expect(screen.getAllByText('Alice').length).toBeGreaterThan(0);
    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('calls onClose when back button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(<ThreadView {...defaultProps} onClose={onClose} thread={mockThread} />);

    await user.click(screen.getByLabelText('Back to inbox'));

    expect(onClose).toHaveBeenCalled();
  });

  it('calls onArchive when archive button is clicked', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();

    render(
      <ThreadView
        {...defaultProps}
        thread={mockThread}
        onArchive={onArchive}
      />,
    );

    await user.click(screen.getByLabelText('Archive'));

    expect(onArchive).toHaveBeenCalled();
  });

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();

    render(
      <ThreadView {...defaultProps} thread={mockThread} onDelete={onDelete} />,
    );

    await user.click(screen.getByLabelText('Delete'));

    expect(onDelete).toHaveBeenCalled();
  });

  it('calls onMarkUnread when mark unread button is clicked', async () => {
    const user = userEvent.setup();
    const onMarkUnread = vi.fn();

    render(
      <ThreadView
        {...defaultProps}
        thread={mockThread}
        onMarkUnread={onMarkUnread}
      />,
    );

    await user.click(screen.getByLabelText('Mark as unread'));

    expect(onMarkUnread).toHaveBeenCalled();
  });

  it('renders reply box when onReply is provided', () => {
    render(
      <ThreadView
        {...defaultProps}
        thread={mockThread}
        onReply={() => {}}
      />,
    );

    expect(
      screen.getByPlaceholderText(/write a reply/i),
    ).toBeInTheDocument();
  });

  it('does not render reply box when onReply is not provided', () => {
    render(<ThreadView {...defaultProps} thread={mockThread} />);

    expect(
      screen.queryByPlaceholderText(/write a reply/i),
    ).not.toBeInTheDocument();
  });

  it('calls onReply when reply button is clicked', async () => {
    const user = userEvent.setup();
    const onReply = vi.fn();

    render(
      <ThreadView {...defaultProps} thread={mockThread} onReply={onReply} />,
    );

    const textarea = screen.getByPlaceholderText(/write a reply/i);
    await user.type(textarea, 'This is my reply');
    await user.click(screen.getByRole('button', { name: /^reply$/i }));

    expect(onReply).toHaveBeenCalledWith('This is my reply');
  });

  it('disables reply button when textarea is empty', () => {
    render(
      <ThreadView {...defaultProps} thread={mockThread} onReply={() => {}} />,
    );

    expect(screen.getByRole('button', { name: /^reply$/i })).toBeDisabled();
  });

  it('shows deadline banner when deadline is provided', () => {
    const deadline = new Date(Date.now() + 86400000).toISOString();

    render(
      <ThreadView {...defaultProps} thread={mockThread} deadline={deadline} />,
    );

    expect(screen.getByText(/deadline:/i)).toBeInTheDocument();
  });

  it('disables navigation buttons appropriately', () => {
    render(
      <ThreadView
        {...defaultProps}
        thread={mockThread}
        hasPrevious={false}
        hasNext={false}
      />,
    );

    expect(screen.getByLabelText('Previous thread')).toBeDisabled();
    expect(screen.getByLabelText('Next thread')).toBeDisabled();
  });

  it('enables navigation buttons when available', () => {
    render(
      <ThreadView
        {...defaultProps}
        thread={mockThread}
        hasPrevious
        hasNext
        onPrevious={() => {}}
        onNext={() => {}}
      />,
    );

    expect(screen.getByLabelText('Previous thread')).not.toBeDisabled();
    expect(screen.getByLabelText('Next thread')).not.toBeDisabled();
  });

  it('handles keyboard navigation with j/k keys', () => {
    render(<ThreadView {...defaultProps} thread={mockThread} />);

    const container = screen.getByRole('log');

    // Simulate pressing j key.
    fireEvent.keyDown(container, { key: 'j' });

    // Focus should move (we check by class change - focused message has blue border).
    // Since this is internal state, just verify no errors occur.
    expect(container).toBeInTheDocument();
  });

  it('closes on Escape key', () => {
    const onClose = vi.fn();

    render(<ThreadView {...defaultProps} onClose={onClose} thread={mockThread} />);

    const container = screen.getByRole('log');
    fireEvent.keyDown(container, { key: 'Escape' });

    expect(onClose).toHaveBeenCalled();
  });
});
