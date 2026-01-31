// Integration tests for StartSessionModal component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StartSessionModal } from '@/components/sessions/index.js';

describe('StartSessionModal', () => {
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
    render(<StartSessionModal {...defaultProps} />);

    expect(screen.getByText('Start New Session')).toBeInTheDocument();
    expect(screen.getByLabelText(/project path/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/branch/i)).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    render(<StartSessionModal {...defaultProps} isOpen={false} />);

    expect(screen.queryByText('Start New Session')).not.toBeInTheDocument();
  });

  it('shows info about sessions', () => {
    render(<StartSessionModal {...defaultProps} />);

    expect(screen.getByText('About Sessions')).toBeInTheDocument();
  });

  it('submits form with project and branch', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<StartSessionModal {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText(/project path/i), '/path/to/project');
    await user.type(screen.getByLabelText(/branch/i), 'main');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        project: '/path/to/project',
        branch: 'main',
      });
    });
  });

  it('submits form with empty optional fields', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<StartSessionModal {...defaultProps} onSubmit={onSubmit} />);

    await user.click(screen.getByRole('button', { name: /start session/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        project: undefined,
        branch: undefined,
      });
    });
  });

  it('validates project path format', async () => {
    const user = userEvent.setup();
    render(<StartSessionModal {...defaultProps} />);

    await user.type(screen.getByLabelText(/project path/i), 'relative/path');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    expect(screen.getByText(/must be absolute/i)).toBeInTheDocument();
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('accepts absolute path starting with /', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<StartSessionModal {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText(/project path/i), '/Users/test/project');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });
  });

  it('accepts path starting with ~', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<StartSessionModal {...defaultProps} onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText(/project path/i), '~/projects/my-app');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });
  });

  it('shows loading state when submitting', () => {
    render(<StartSessionModal {...defaultProps} isSubmitting />);

    expect(screen.getByRole('button', { name: /start session/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
  });

  it('shows submit error message', () => {
    render(<StartSessionModal {...defaultProps} submitError="Session already exists" />);

    expect(screen.getByText('Session already exists')).toBeInTheDocument();
  });

  it('calls onClose when cancel is clicked', async () => {
    const user = userEvent.setup();
    render(<StartSessionModal {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('shows confirmation when closing with dirty form', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    window.confirm = vi.fn().mockReturnValue(false);

    render(<StartSessionModal {...defaultProps} onClose={onClose} />);

    // Make the form dirty.
    await user.type(screen.getByLabelText(/project path/i), 'test');

    // Try to cancel.
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(window.confirm).toHaveBeenCalledWith('Discard changes?');
    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes without confirmation when form is clean', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(<StartSessionModal {...defaultProps} onClose={onClose} />);

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(window.confirm).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('clears errors when user types', async () => {
    const user = userEvent.setup();
    render(<StartSessionModal {...defaultProps} />);

    // Trigger validation error.
    await user.type(screen.getByLabelText(/project path/i), 'relative');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    expect(screen.getByText(/must be absolute/i)).toBeInTheDocument();

    // Clear input and retype.
    await user.clear(screen.getByLabelText(/project path/i));
    await user.type(screen.getByLabelText(/project path/i), '/absolute');

    await waitFor(() => {
      expect(screen.queryByText(/must be absolute/i)).not.toBeInTheDocument();
    });
  });

  it('resets form after successful submission', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<StartSessionModal {...defaultProps} onSubmit={onSubmit} />);

    const projectInput = screen.getByLabelText(/project path/i);
    await user.type(projectInput, '/test/project');
    await user.click(screen.getByRole('button', { name: /start session/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });

    // Input should be cleared.
    expect(projectInput).toHaveValue('');
  });

  it('uses default values if provided', () => {
    render(
      <StartSessionModal
        {...defaultProps}
        defaultProject="/default/path"
        defaultBranch="develop"
      />,
    );

    expect(screen.getByLabelText(/project path/i)).toHaveValue('/default/path');
    expect(screen.getByLabelText(/branch/i)).toHaveValue('develop');
  });

  it('applies custom className', () => {
    render(<StartSessionModal {...defaultProps} className="custom-class" />);

    // The modal should have the custom class.
    const modal = document.querySelector('.custom-class');
    expect(modal).toBeInTheDocument();
  });
});
