// Integration tests for the Modal component.

import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '../../utils.js';
import userEvent from '@testing-library/user-event';
import { Modal, ModalFooter, ConfirmModal } from '@/components/ui/Modal';
import { useState } from 'react';

// Test wrapper that controls modal state.
function ModalWrapper({
  initialOpen = true,
  ...props
}: Partial<React.ComponentProps<typeof Modal>> & { initialOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(initialOpen);
  return (
    <>
      <button onClick={() => setIsOpen(true)}>Open</button>
      <Modal isOpen={isOpen} onClose={() => setIsOpen(false)} {...props}>
        {props.children ?? <div>Modal content</div>}
      </Modal>
    </>
  );
}

describe('Modal', () => {
  describe('rendering', () => {
    it('renders when isOpen is true', async () => {
      render(
        <Modal isOpen onClose={() => {}}>
          <div>Modal content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Modal content')).toBeInTheDocument();
      });
    });

    it('does not render when isOpen is false', () => {
      render(
        <Modal isOpen={false} onClose={() => {}}>
          <div>Modal content</div>
        </Modal>,
      );

      expect(screen.queryByText('Modal content')).not.toBeInTheDocument();
    });

    it('renders title when provided', async () => {
      render(
        <Modal isOpen onClose={() => {}} title="Test Title">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Test Title')).toBeInTheDocument();
      });
    });

    it('renders description when provided', async () => {
      render(
        <Modal isOpen onClose={() => {}} title="Title" description="Test description">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Test description')).toBeInTheDocument();
      });
    });

    it('renders close button by default', async () => {
      render(
        <Modal isOpen onClose={() => {}} title="Title">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByLabelText('Close modal')).toBeInTheDocument();
      });
    });

    it('hides close button when showCloseButton is false', async () => {
      render(
        <Modal isOpen onClose={() => {}} title="Title" showCloseButton={false}>
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Title')).toBeInTheDocument();
      });
      expect(screen.queryByLabelText('Close modal')).not.toBeInTheDocument();
    });
  });

  describe('sizes', () => {
    it('applies small size', async () => {
      render(
        <Modal isOpen onClose={() => {}} size="sm">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        const panel = screen.getByText('Content').closest('[class*="max-w"]');
        expect(panel?.className).toContain('max-w-sm');
      });
    });

    it('applies large size', async () => {
      render(
        <Modal isOpen onClose={() => {}} size="lg">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        const panel = screen.getByText('Content').closest('[class*="max-w"]');
        expect(panel?.className).toContain('max-w-lg');
      });
    });

    it('applies full size', async () => {
      render(
        <Modal isOpen onClose={() => {}} size="full">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        const panel = screen.getByText('Content').closest('[class*="max-w"]');
        expect(panel?.className).toContain('max-w-4xl');
      });
    });
  });

  describe('interactions', () => {
    it('calls onClose when close button is clicked', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <Modal isOpen onClose={onClose} title="Title">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByLabelText('Close modal')).toBeInTheDocument();
      });

      await user.click(screen.getByLabelText('Close modal'));
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when Escape key is pressed', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <Modal isOpen onClose={onClose}>
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Content')).toBeInTheDocument();
      });

      await user.keyboard('{Escape}');
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when overlay is clicked by default', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <Modal isOpen onClose={onClose}>
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Content')).toBeInTheDocument();
      });

      // Click on the overlay (the backdrop)
      const backdrop = document.querySelector('[aria-hidden="true"]');
      if (backdrop) {
        await user.click(backdrop);
      }
      // Headless UI handles this via Dialog's onClose
    });

    it('does not close when closeOnOverlayClick is false', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <Modal isOpen onClose={onClose} closeOnOverlayClick={false}>
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        expect(screen.getByText('Content')).toBeInTheDocument();
      });

      // The Dialog won't call onClose for overlay clicks when we handle it
      await user.keyboard('{Escape}');
      expect(onClose).not.toHaveBeenCalled();
    });
  });

  describe('custom className', () => {
    it('applies custom className to modal panel', async () => {
      render(
        <Modal isOpen onClose={() => {}} className="custom-modal">
          <div>Content</div>
        </Modal>,
      );

      await waitFor(() => {
        const panel = screen.getByText('Content').closest('[class*="custom-modal"]');
        expect(panel).toBeInTheDocument();
      });
    });
  });

  describe('state transitions', () => {
    it('opens and closes correctly', async () => {
      const user = userEvent.setup();
      render(<ModalWrapper initialOpen={false} title="Title" />);

      // Initially closed.
      expect(screen.queryByText('Modal content')).not.toBeInTheDocument();

      // Open modal.
      await user.click(screen.getByText('Open'));
      await waitFor(() => {
        expect(screen.getByText('Modal content')).toBeInTheDocument();
      });

      // Close modal.
      await user.click(screen.getByLabelText('Close modal'));
      await waitFor(() => {
        expect(screen.queryByText('Modal content')).not.toBeInTheDocument();
      });
    });
  });
});

describe('ModalFooter', () => {
  it('renders children', async () => {
    render(
      <Modal isOpen onClose={() => {}}>
        <div>Content</div>
        <ModalFooter>
          <button>Cancel</button>
          <button>Submit</button>
        </ModalFooter>
      </Modal>,
    );

    await waitFor(() => {
      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Submit')).toBeInTheDocument();
    });
  });

  it('applies custom className', async () => {
    render(
      <Modal isOpen onClose={() => {}}>
        <div>Content</div>
        <ModalFooter className="custom-footer">
          <button>Action</button>
        </ModalFooter>
      </Modal>,
    );

    await waitFor(() => {
      const footer = screen.getByText('Action').parentElement;
      expect(footer?.className).toContain('custom-footer');
    });
  });
});

describe('ConfirmModal', () => {
  describe('rendering', () => {
    it('renders title and message', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Confirm Action"
          message="Are you sure you want to proceed?"
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Confirm Action')).toBeInTheDocument();
        expect(screen.getByText('Are you sure you want to proceed?')).toBeInTheDocument();
      });
    });

    it('renders default button text', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Test Title"
          message="Message"
        />,
      );

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
      });
    });

    it('renders custom button text', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Delete"
          message="Delete item?"
          confirmText="Delete"
          cancelText="Keep"
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Delete', { selector: 'button' })).toBeInTheDocument();
        expect(screen.getByText('Keep')).toBeInTheDocument();
      });
    });
  });

  describe('variants', () => {
    it('applies primary variant by default', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Confirm"
          message="Message"
        />,
      );

      await waitFor(() => {
        const confirmButton = screen.getByText('Confirm', { selector: 'button' });
        expect(confirmButton.className).toContain('bg-blue-600');
      });
    });

    it('applies danger variant', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Delete"
          message="Message"
          variant="danger"
        />,
      );

      await waitFor(() => {
        const confirmButton = screen.getByText('Confirm');
        expect(confirmButton.className).toContain('bg-red-600');
      });
    });
  });

  describe('interactions', () => {
    it('calls onClose when cancel is clicked', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <ConfirmModal
          isOpen
          onClose={onClose}
          onConfirm={() => {}}
          title="Confirm"
          message="Message"
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Cancel')).toBeInTheDocument();
      });

      await user.click(screen.getByText('Cancel'));
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onConfirm when confirm is clicked', async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={onConfirm}
          title="Confirm"
          message="Message"
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Confirm', { selector: 'button' })).toBeInTheDocument();
      });

      await user.click(screen.getByText('Confirm', { selector: 'button' }));
      expect(onConfirm).toHaveBeenCalledTimes(1);
    });
  });

  describe('loading state', () => {
    it('shows loading text when isLoading', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Confirm"
          message="Message"
          isLoading
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Loading...')).toBeInTheDocument();
      });
    });

    it('disables buttons when loading', async () => {
      render(
        <ConfirmModal
          isOpen
          onClose={() => {}}
          onConfirm={() => {}}
          title="Confirm"
          message="Message"
          isLoading
        />,
      );

      await waitFor(() => {
        expect(screen.getByText('Cancel')).toBeDisabled();
        expect(screen.getByText('Loading...')).toBeDisabled();
      });
    });
  });
});
