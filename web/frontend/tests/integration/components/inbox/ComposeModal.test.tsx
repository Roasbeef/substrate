// Integration tests for ComposeModal component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ComposeModal, RecipientInput } from '@/components/inbox/index.js';
import type { AutocompleteRecipient } from '@/types/api.js';

// Mock recipients for testing.
const mockRecipients: AutocompleteRecipient[] = [
  { id: 1, name: 'Alice', status: 'active' },
  { id: 2, name: 'Bob', status: 'busy' },
  { id: 3, name: 'Charlie', status: 'idle' },
  { id: 4, name: 'Diana', status: 'offline' },
];

// Mock search function.
const mockSearch = vi.fn().mockImplementation(async (query: string) => {
  await new Promise((resolve) => setTimeout(resolve, 50));
  return mockRecipients.filter((r) =>
    r.name.toLowerCase().includes(query.toLowerCase()),
  );
});

describe('RecipientInput', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with placeholder when empty', () => {
    render(
      <RecipientInput
        value={[]}
        onChange={() => {}}
        onSearch={mockSearch}
        placeholder="Add recipients..."
      />,
    );

    expect(screen.getByPlaceholderText('Add recipients...')).toBeInTheDocument();
  });

  it('renders selected recipients as chips', () => {
    render(
      <RecipientInput
        value={[mockRecipients[0], mockRecipients[1]]}
        onChange={() => {}}
        onSearch={mockSearch}
      />,
    );

    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('shows suggestions when typing', async () => {
    const user = userEvent.setup();

    render(
      <RecipientInput
        value={[]}
        onChange={() => {}}
        onSearch={mockSearch}
      />,
    );

    const input = screen.getByRole('combobox');
    await user.type(input, 'ali');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('filters out already selected recipients from suggestions', async () => {
    const user = userEvent.setup();

    render(
      <RecipientInput
        value={[mockRecipients[0]]} // Alice already selected.
        onChange={() => {}}
        onSearch={mockSearch}
      />,
    );

    const input = screen.getByRole('combobox');
    await user.type(input, 'a');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    // Should not show Alice in suggestions.
    const suggestions = screen.getAllByRole('option');
    expect(suggestions.some((s) => s.textContent?.includes('Alice'))).toBe(false);
  });

  it('calls onChange when selecting a recipient', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RecipientInput
        value={[]}
        onChange={onChange}
        onSearch={mockSearch}
      />,
    );

    const input = screen.getByRole('combobox');
    await user.type(input, 'bob');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Bob'));

    expect(onChange).toHaveBeenCalledWith([mockRecipients[1]]);
  });

  it('calls onChange when removing a recipient', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RecipientInput
        value={[mockRecipients[0], mockRecipients[1]]}
        onChange={onChange}
        onSearch={mockSearch}
      />,
    );

    await user.click(screen.getByLabelText('Remove Alice'));

    expect(onChange).toHaveBeenCalledWith([mockRecipients[1]]);
  });

  it('supports keyboard navigation', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RecipientInput
        value={[]}
        onChange={onChange}
        onSearch={mockSearch}
      />,
    );

    const input = screen.getByRole('combobox');
    await user.type(input, 'a');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    // Navigate with arrow keys.
    await user.keyboard('{ArrowDown}');
    await user.keyboard('{Enter}');

    expect(onChange).toHaveBeenCalled();
  });

  it('removes last recipient on backspace when input is empty', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <RecipientInput
        value={[mockRecipients[0], mockRecipients[1]]}
        onChange={onChange}
        onSearch={mockSearch}
      />,
    );

    const input = screen.getByRole('combobox');
    await user.click(input);
    await user.keyboard('{Backspace}');

    expect(onChange).toHaveBeenCalledWith([mockRecipients[0]]);
  });

  it('shows error message when error prop is provided', () => {
    render(
      <RecipientInput
        value={[]}
        onChange={() => {}}
        onSearch={mockSearch}
        error="At least one recipient is required"
      />,
    );

    expect(
      screen.getByText('At least one recipient is required'),
    ).toBeInTheDocument();
  });

  it('disables input when disabled prop is true', () => {
    render(
      <RecipientInput
        value={[]}
        onChange={() => {}}
        onSearch={mockSearch}
        disabled
      />,
    );

    expect(screen.getByRole('combobox')).toBeDisabled();
  });
});

describe('ComposeModal', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onSend: vi.fn().mockResolvedValue(undefined),
    onSearchRecipients: mockSearch,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the form fields', () => {
    render(<ComposeModal {...defaultProps} />);

    expect(screen.getByText('Compose Message')).toBeInTheDocument();
    expect(screen.getByLabelText(/^to$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^subject$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^message$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^priority$/i)).toBeInTheDocument();
  });

  it('validates required fields on submit', async () => {
    const user = userEvent.setup();

    render(<ComposeModal {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(
      screen.getByText('At least one recipient is required'),
    ).toBeInTheDocument();
    expect(screen.getByText('Subject is required')).toBeInTheDocument();
    expect(screen.getByText('Message body is required')).toBeInTheDocument();

    expect(defaultProps.onSend).not.toHaveBeenCalled();
  });

  it('calls onSend with correct data when form is valid', async () => {
    const user = userEvent.setup();

    render(<ComposeModal {...defaultProps} />);

    // Add recipient - find the recipient input (first combobox).
    const recipientInputs = screen.getAllByRole('combobox');
    const recipientInput = recipientInputs[0];
    await user.type(recipientInput, 'alice');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Alice'));

    // Fill subject.
    await user.type(screen.getByLabelText(/^subject$/i), 'Test Subject');

    // Fill body.
    await user.type(screen.getByLabelText(/^message$/i), 'Test message body');

    // Submit.
    await user.click(screen.getByRole('button', { name: /send/i }));

    await waitFor(() => {
      expect(defaultProps.onSend).toHaveBeenCalledWith({
        to: [1],
        subject: 'Test Subject',
        body: 'Test message body',
        priority: 'normal',
        deadline: undefined,
      });
    });
  });

  it('allows changing priority', async () => {
    const user = userEvent.setup();

    render(<ComposeModal {...defaultProps} />);

    const prioritySelect = screen.getByLabelText(/^priority$/i);
    await user.selectOptions(prioritySelect, 'urgent');

    expect(prioritySelect).toHaveValue('urgent');
  });

  it('shows loading state when sending', () => {
    render(<ComposeModal {...defaultProps} isSending />);

    expect(screen.getByRole('button', { name: /send/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
  });

  it('closes modal when cancel is clicked (no dirty form)', async () => {
    const user = userEvent.setup();

    render(<ComposeModal {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('shows confirmation when closing with dirty form', async () => {
    const user = userEvent.setup();
    const originalConfirm = window.confirm;
    window.confirm = vi.fn().mockReturnValue(false);

    render(<ComposeModal {...defaultProps} />);

    // Make the form dirty.
    await user.type(screen.getByLabelText(/^subject$/i), 'Something');

    // Try to cancel.
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(window.confirm).toHaveBeenCalled();
    expect(defaultProps.onClose).not.toHaveBeenCalled();

    window.confirm = originalConfirm;
  });

  it('accepts custom title', () => {
    render(<ComposeModal {...defaultProps} title="Reply to Thread" />);

    expect(screen.getByText('Reply to Thread')).toBeInTheDocument();
  });

  it('accepts initial values', () => {
    render(
      <ComposeModal
        {...defaultProps}
        initialValues={{
          subject: 'Re: Original Subject',
          recipients: [mockRecipients[0]],
        }}
      />,
    );

    expect(screen.getByLabelText(/^subject$/i)).toHaveValue('Re: Original Subject');
    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('resets form after successful send', async () => {
    const user = userEvent.setup();
    const onSend = vi.fn().mockResolvedValue(undefined);

    render(<ComposeModal {...defaultProps} onSend={onSend} />);

    // Fill form - find the recipient input by its label context.
    const recipientInputs = screen.getAllByRole('combobox');
    const recipientInput = recipientInputs[0];
    await user.type(recipientInput, 'bob');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Bob'));
    await user.type(screen.getByLabelText(/^subject$/i), 'Test');
    await user.type(screen.getByLabelText(/^message$/i), 'Body');

    // Submit.
    await user.click(screen.getByRole('button', { name: /send/i }));

    await waitFor(() => {
      expect(onSend).toHaveBeenCalled();
    });

    // Modal should close.
    expect(defaultProps.onClose).toHaveBeenCalled();
  });
});
