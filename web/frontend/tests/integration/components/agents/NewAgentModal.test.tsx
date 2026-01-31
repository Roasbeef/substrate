// Integration tests for NewAgentModal component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NewAgentModal } from '@/components/agents/index.js';

describe('NewAgentModal', () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onSubmit: vi.fn().mockResolvedValue(undefined),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    window.confirm = vi.fn().mockReturnValue(true);
  });

  it('renders the modal when open', () => {
    render(<NewAgentModal {...defaultProps} />);

    expect(screen.getByText('Register New Agent')).toBeInTheDocument();
    expect(screen.getByLabelText(/agent name/i)).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    render(<NewAgentModal {...defaultProps} isOpen={false} />);

    expect(screen.queryByText('Register New Agent')).not.toBeInTheDocument();
  });

  it('validates required name field', async () => {
    const user = userEvent.setup();
    render(<NewAgentModal {...defaultProps} />);

    // Try to submit with empty name.
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    await waitFor(() => {
      expect(screen.getByText('Agent name is required')).toBeInTheDocument();
    });
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('validates minimum name length', async () => {
    const user = userEvent.setup();
    render(<NewAgentModal {...defaultProps} />);

    await user.type(screen.getByLabelText(/agent name/i), 'a');
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    expect(screen.getByText('Name must be at least 2 characters')).toBeInTheDocument();
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('validates name format', async () => {
    const user = userEvent.setup();
    render(<NewAgentModal {...defaultProps} />);

    // Name starting with number is invalid.
    await user.type(screen.getByLabelText(/agent name/i), '123agent');
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    expect(
      screen.getByText(/name must start with a letter/i),
    ).toBeInTheDocument();
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('submits valid form data', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<NewAgentModal {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText(/agent name/i), 'my-new-agent');
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({ name: 'my-new-agent' });
    });
  });

  it('shows loading state when submitting', () => {
    render(<NewAgentModal {...defaultProps} isSubmitting />);

    expect(screen.getByRole('button', { name: /register agent/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
  });

  it('shows submit error message', () => {
    render(<NewAgentModal {...defaultProps} submitError="Agent already exists" />);

    expect(screen.getByText('Agent already exists')).toBeInTheDocument();
  });

  it('calls onClose when cancel is clicked', async () => {
    const user = userEvent.setup();
    render(<NewAgentModal {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('shows confirmation when closing with dirty form', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    window.confirm = vi.fn().mockReturnValue(false);

    render(<NewAgentModal {...defaultProps} onClose={onClose} />);

    // Make the form dirty.
    await user.type(screen.getByLabelText(/agent name/i), 'test');

    // Try to cancel.
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(window.confirm).toHaveBeenCalledWith('Discard changes?');
    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes without confirmation when form is clean', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(<NewAgentModal {...defaultProps} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(window.confirm).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('clears errors when user types', async () => {
    const user = userEvent.setup();
    render(<NewAgentModal {...defaultProps} />);

    // Trigger validation error.
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    await waitFor(() => {
      expect(screen.getByText('Agent name is required')).toBeInTheDocument();
    });

    // Type something to clear error.
    await user.type(screen.getByLabelText(/agent name/i), 't');

    await waitFor(() => {
      expect(screen.queryByText('Agent name is required')).not.toBeInTheDocument();
    });
  });

  it('resets form after successful submission', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<NewAgentModal {...defaultProps} onSubmit={onSubmit} />);

    const input = screen.getByLabelText(/agent name/i);
    await user.type(input, 'test-agent');
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });

    // Input should be cleared.
    expect(input).toHaveValue('');
  });

  it('shows info about registration', () => {
    render(<NewAgentModal {...defaultProps} />);

    expect(screen.getByText('What happens when you register?')).toBeInTheDocument();
    expect(screen.getByText('A new agent identity is created')).toBeInTheDocument();
  });

  it('accepts valid names with underscores and hyphens', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<NewAgentModal {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText(/agent name/i), 'my_agent-01');
    await user.click(screen.getByRole('button', { name: /register agent/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({ name: 'my_agent-01' });
    });
  });
});
