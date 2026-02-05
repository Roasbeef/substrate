// Integration tests for MessageList component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderHook, act } from '@testing-library/react';
import {
  MessageList,
  ConnectedMessageList,
  useMessageSelection,
} from '@/components/inbox/MessageList.js';
import type { MessageWithRecipients } from '@/types/api.js';

// Create mock messages.
const createMockMessage = (
  id: number,
  overrides: Partial<MessageWithRecipients> = {},
): MessageWithRecipients => ({
  id,
  sender_id: 1,
  sender_name: `Sender ${id}`,
  subject: `Subject ${id}`,
  body: `Body content for message ${id}`,
  priority: 'normal',
  created_at: new Date(Date.now() - id * 3600000).toISOString(),
  recipients: [
    {
      message_id: id,
      agent_id: 2,
      agent_name: 'RecipientAgent',
      state: 'unread',
      is_starred: false,
      is_archived: false,
    },
  ],
  ...overrides,
});

const mockMessages: MessageWithRecipients[] = [
  createMockMessage(1),
  createMockMessage(2),
  createMockMessage(3),
];

describe('MessageList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders list of messages', () => {
    render(<MessageList messages={mockMessages} />);

    expect(screen.getByText('Sender 1')).toBeInTheDocument();
    expect(screen.getByText('Sender 2')).toBeInTheDocument();
    expect(screen.getByText('Sender 3')).toBeInTheDocument();
  });

  it('renders message subjects', () => {
    render(<MessageList messages={mockMessages} />);

    expect(screen.getByText('Subject 1')).toBeInTheDocument();
    expect(screen.getByText('Subject 2')).toBeInTheDocument();
    expect(screen.getByText('Subject 3')).toBeInTheDocument();
  });

  it('shows loading skeleton when isLoading', () => {
    const { container } = render(
      <MessageList messages={[]} isLoading loadingRows={3} />,
    );

    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows correct number of skeleton rows', () => {
    const { container } = render(
      <MessageList messages={[]} isLoading loadingRows={5} />,
    );

    // Each skeleton row has multiple animated elements.
    const skeletonRows = container.querySelectorAll(
      '.border-b.border-gray-100',
    );
    expect(skeletonRows.length).toBe(5);
  });

  it('shows empty state when no messages', () => {
    render(<MessageList messages={[]} isEmpty />);

    expect(screen.getByText('No messages')).toBeInTheDocument();
    expect(screen.getByText('Your inbox is empty.')).toBeInTheDocument();
  });

  it('shows custom empty state text', () => {
    render(
      <MessageList
        messages={[]}
        isEmpty
        emptyTitle="Nothing here"
        emptyDescription="Try again later."
      />,
    );

    expect(screen.getByText('Nothing here')).toBeInTheDocument();
    expect(screen.getByText('Try again later.')).toBeInTheDocument();
  });

  it('calls onMessageClick when message is clicked', async () => {
    const user = userEvent.setup();
    const onMessageClick = vi.fn();
    render(
      <MessageList messages={mockMessages} onMessageClick={onMessageClick} />,
    );

    await user.click(screen.getByText('Subject 1'));

    expect(onMessageClick).toHaveBeenCalledWith(mockMessages[0]);
  });

  it('calls onStar when star button is clicked', async () => {
    const user = userEvent.setup();
    const onStar = vi.fn();
    render(<MessageList messages={mockMessages} onStar={onStar} />);

    const starButtons = screen.getAllByRole('button', { name: /star message/i });
    await user.click(starButtons[0]);

    expect(onStar).toHaveBeenCalledWith(1, true);
  });

  it('calls onArchive when archive button is clicked', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(<MessageList messages={mockMessages} onArchive={onArchive} />);

    const archiveButtons = screen.getAllByLabelText('Archive');
    await user.click(archiveButtons[0]);

    expect(onArchive).toHaveBeenCalledWith(1);
  });

  it('calls onSnooze when snooze button is clicked', async () => {
    const user = userEvent.setup();
    const onSnooze = vi.fn();
    render(<MessageList messages={mockMessages} onSnooze={onSnooze} />);

    const snoozeButtons = screen.getAllByLabelText('Snooze');
    await user.click(snoozeButtons[0]);

    expect(onSnooze).toHaveBeenCalledWith(1);
  });

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<MessageList messages={mockMessages} onDelete={onDelete} />);

    const deleteButtons = screen.getAllByLabelText('Delete');
    await user.click(deleteButtons[0]);

    expect(onDelete).toHaveBeenCalledWith(1);
  });

  it('shows checkboxes when showCheckboxes is true', () => {
    render(<MessageList messages={mockMessages} showCheckboxes />);

    const checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes.length).toBe(3);
  });

  it('hides checkboxes when showCheckboxes is false', () => {
    render(<MessageList messages={mockMessages} showCheckboxes={false} />);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('calls onSelectionChange when checkbox is clicked', async () => {
    const user = userEvent.setup();
    const onSelectionChange = vi.fn();
    render(
      <MessageList
        messages={mockMessages}
        onSelectionChange={onSelectionChange}
      />,
    );

    const checkboxes = screen.getAllByRole('checkbox');
    await user.click(checkboxes[0]);

    expect(onSelectionChange).toHaveBeenCalled();
    const newSelection = onSelectionChange.mock.calls[0][0];
    expect(newSelection.has(1)).toBe(true);
  });

  it('shows selected state for selected messages', () => {
    const selectedIds = new Set([1, 2]);
    const { container } = render(
      <MessageList messages={mockMessages} selectedIds={selectedIds} />,
    );

    // Selected messages have bg-blue-50 style.
    const rows = container.querySelectorAll('.bg-blue-50');
    expect(rows.length).toBe(2);
  });

  it('renders compact variant', () => {
    render(<MessageList messages={mockMessages} compact />);

    // Compact rows don't have checkboxes.
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();

    // But should still show subjects.
    expect(screen.getByText('Subject 1')).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(
      <MessageList messages={mockMessages} className="custom-list" />,
    );

    expect(container.firstChild).toHaveClass('custom-list');
  });
});

describe('useMessageSelection', () => {
  it('initializes with empty selection', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    expect(result.current.selectedIds.size).toBe(0);
    expect(result.current.selectedCount).toBe(0);
    expect(result.current.hasSelection).toBe(false);
  });

  it('returns correct total count', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3, 4, 5] }),
    );

    expect(result.current.totalCount).toBe(5);
  });

  it('selectAll selects all messages', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.selectAll(true);
    });

    expect(result.current.selectedCount).toBe(3);
    expect(result.current.allSelected).toBe(true);
    expect(result.current.selectedIds.has(1)).toBe(true);
    expect(result.current.selectedIds.has(2)).toBe(true);
    expect(result.current.selectedIds.has(3)).toBe(true);
  });

  it('selectAll(false) deselects all messages', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.selectAll(true);
    });

    act(() => {
      result.current.selectAll(false);
    });

    expect(result.current.selectedCount).toBe(0);
    expect(result.current.allSelected).toBe(false);
  });

  it('clearSelection clears all selected', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.selectAll(true);
    });

    act(() => {
      result.current.clearSelection();
    });

    expect(result.current.selectedCount).toBe(0);
  });

  it('toggleSelection adds unselected message', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.toggleSelection(2);
    });

    expect(result.current.selectedIds.has(2)).toBe(true);
    expect(result.current.selectedCount).toBe(1);
  });

  it('toggleSelection removes selected message', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.toggleSelection(2);
    });

    act(() => {
      result.current.toggleSelection(2);
    });

    expect(result.current.selectedIds.has(2)).toBe(false);
    expect(result.current.selectedCount).toBe(0);
  });

  it('setSelection replaces current selection', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.selectAll(true);
    });

    act(() => {
      result.current.setSelection(new Set([1]));
    });

    expect(result.current.selectedCount).toBe(1);
    expect(result.current.selectedIds.has(1)).toBe(true);
    expect(result.current.selectedIds.has(2)).toBe(false);
  });

  it('isIndeterminate is true when some but not all selected', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.toggleSelection(1);
    });

    expect(result.current.isIndeterminate).toBe(true);
    expect(result.current.allSelected).toBe(false);
  });

  it('isIndeterminate is false when all selected', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    act(() => {
      result.current.selectAll(true);
    });

    expect(result.current.isIndeterminate).toBe(false);
    expect(result.current.allSelected).toBe(true);
  });

  it('isIndeterminate is false when none selected', () => {
    const { result } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    expect(result.current.isIndeterminate).toBe(false);
    expect(result.current.allSelected).toBe(false);
  });
});

describe('ConnectedMessageList', () => {
  it('renders messages from data prop', () => {
    render(<ConnectedMessageList data={mockMessages} />);

    expect(screen.getByText('Subject 1')).toBeInTheDocument();
    expect(screen.getByText('Subject 2')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    const { container } = render(<ConnectedMessageList isLoading />);

    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows error state', () => {
    const error = new Error('Failed to fetch messages');
    render(<ConnectedMessageList error={error} />);

    expect(screen.getByText('Failed to load messages')).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch messages')).toBeInTheDocument();
  });

  it('shows empty state when data is empty', () => {
    render(<ConnectedMessageList data={[]} />);

    expect(screen.getByText('No messages')).toBeInTheDocument();
  });

  it('shows empty state when data is undefined and not loading', () => {
    render(<ConnectedMessageList />);

    expect(screen.getByText('No messages')).toBeInTheDocument();
  });

  it('integrates with selection state', async () => {
    const user = userEvent.setup();
    const { result: selectionResult } = renderHook(() =>
      useMessageSelection({ messageIds: [1, 2, 3] }),
    );

    const { rerender } = render(
      <ConnectedMessageList
        data={mockMessages}
        selection={selectionResult.current}
      />,
    );

    const checkboxes = screen.getAllByRole('checkbox');
    await user.click(checkboxes[0]);

    // Re-render with updated selection.
    rerender(
      <ConnectedMessageList
        data={mockMessages}
        selection={selectionResult.current}
      />,
    );

    expect(checkboxes[0]).toBeChecked();
  });

  it('passes through onMessageClick', async () => {
    const user = userEvent.setup();
    const onMessageClick = vi.fn();
    render(
      <ConnectedMessageList
        data={mockMessages}
        onMessageClick={onMessageClick}
      />,
    );

    await user.click(screen.getByText('Subject 1'));

    expect(onMessageClick).toHaveBeenCalledWith(mockMessages[0]);
  });

  it('passes through action handlers', async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(
      <ConnectedMessageList data={mockMessages} onArchive={onArchive} />,
    );

    const archiveButtons = screen.getAllByLabelText('Archive');
    await user.click(archiveButtons[0]);

    expect(onArchive).toHaveBeenCalledWith(1);
  });
});
