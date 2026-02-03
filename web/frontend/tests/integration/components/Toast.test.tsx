// Integration tests for the Toast component.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '../../utils.js';
import userEvent from '@testing-library/user-event';
import { ToastContainer, useToast } from '@/components/ui/Toast';
import { useUIStore } from '@/stores/ui';

// Reset store before each test.
beforeEach(() => {
  useUIStore.setState({ toasts: [] });
});

// Test component that uses the useToast hook.
function ToastTester() {
  const { success, error, warning, info } = useToast();

  return (
    <div>
      <button onClick={() => success('Success message')}>Show Success</button>
      <button onClick={() => error('Error message')}>Show Error</button>
      <button onClick={() => warning('Warning message')}>Show Warning</button>
      <button onClick={() => info('Info message')}>Show Info</button>
      <button onClick={() => success('With title', { title: 'Title' })}>
        With Title
      </button>
      <button
        onClick={() =>
          success('With action', {
            action: { label: 'Undo', onClick: () => {} },
          })
        }
      >
        With Action
      </button>
      <ToastContainer />
    </div>
  );
}

describe('ToastContainer', () => {
  describe('rendering', () => {
    it('renders nothing when no toasts', () => {
      const { container } = render(<ToastContainer />);
      expect(container.firstChild).toBeNull();
    });

    it('renders success toast', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        expect(screen.getByText('Success message')).toBeInTheDocument();
      });
    });

    it('renders error toast', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Error'));

      await waitFor(() => {
        expect(screen.getByText('Error message')).toBeInTheDocument();
      });
    });

    it('renders warning toast', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Warning'));

      await waitFor(() => {
        expect(screen.getByText('Warning message')).toBeInTheDocument();
      });
    });

    it('renders info toast', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Info'));

      await waitFor(() => {
        expect(screen.getByText('Info message')).toBeInTheDocument();
      });
    });

    it('renders toast with title', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('With Title'));

      await waitFor(() => {
        expect(screen.getByText('Title')).toBeInTheDocument();
        expect(screen.getByText('With title')).toBeInTheDocument();
      });
    });

    it('renders toast with action button', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('With Action'));

      await waitFor(() => {
        expect(screen.getByText('Undo')).toBeInTheDocument();
      });
    });

    it('renders multiple toasts', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));
      await user.click(screen.getByText('Show Error'));

      await waitFor(() => {
        expect(screen.getByText('Success message')).toBeInTheDocument();
        expect(screen.getByText('Error message')).toBeInTheDocument();
      });
    });
  });

  describe('interactions', () => {
    it('closes toast when close button is clicked', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        expect(screen.getByText('Success message')).toBeInTheDocument();
      });

      await user.click(screen.getByLabelText('Close notification'));

      await waitFor(() => {
        expect(screen.queryByText('Success message')).not.toBeInTheDocument();
      });
    });
  });

  describe('accessibility', () => {
    it('has role="alert"', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
      });
    });

    it('has aria-live="assertive"', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        const alert = screen.getByRole('alert');
        expect(alert).toHaveAttribute('aria-live', 'assertive');
      });
    });

    it('close button has accessible label', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        expect(screen.getByLabelText('Close notification')).toBeInTheDocument();
      });
    });
  });

  describe('variant styling', () => {
    it('success toast has green border', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Success'));

      await waitFor(() => {
        const alert = screen.getByRole('alert');
        expect(alert.className).toContain('border-green-200');
      });
    });

    it('error toast has red border', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Error'));

      await waitFor(() => {
        const alert = screen.getByRole('alert');
        expect(alert.className).toContain('border-red-200');
      });
    });

    it('warning toast has yellow border', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Warning'));

      await waitFor(() => {
        const alert = screen.getByRole('alert');
        expect(alert.className).toContain('border-yellow-200');
      });
    });

    it('info toast has blue border', async () => {
      const user = userEvent.setup();
      render(<ToastTester />);

      await user.click(screen.getByText('Show Info'));

      await waitFor(() => {
        const alert = screen.getByRole('alert');
        expect(alert.className).toContain('border-blue-200');
      });
    });
  });
});

describe('useToast', () => {
  it('adds toast to store', async () => {
    const user = userEvent.setup();
    render(<ToastTester />);

    await user.click(screen.getByText('Show Success'));

    const toasts = useUIStore.getState().toasts;
    expect(toasts).toHaveLength(1);
    expect(toasts[0]?.message).toBe('Success message');
    expect(toasts[0]?.variant).toBe('success');
  });

  it('removes toast from store when dismissed', () => {
    // Add a toast directly to the store.
    act(() => {
      useUIStore.getState().addToast({
        message: 'Test toast',
        variant: 'info',
      });
    });

    expect(useUIStore.getState().toasts).toHaveLength(1);
    const toastId = useUIStore.getState().toasts[0]?.id;

    // Remove the toast.
    act(() => {
      if (toastId) {
        useUIStore.getState().removeToast(toastId);
      }
    });

    expect(useUIStore.getState().toasts).toHaveLength(0);
  });
});
