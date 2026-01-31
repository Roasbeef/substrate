// NewAgentModal component - modal for registering a new agent.

import { useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Modal } from '@/components/ui/Modal.js';
import { TextInput } from '@/components/ui/Input.js';
import { Button } from '@/components/ui/Button.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Form state for new agent.
interface NewAgentFormState {
  name: string;
}

// Form errors.
interface NewAgentFormErrors {
  name?: string;
}

// Props for NewAgentModal.
export interface NewAgentModalProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** Handler for submitting the form. */
  onSubmit: (data: NewAgentFormState) => Promise<void>;
  /** Whether the form is submitting. */
  isSubmitting?: boolean;
  /** Error message from submission. */
  submitError?: string | null;
  /** Additional class name. */
  className?: string;
}

// Validate form state.
function validateForm(state: NewAgentFormState): NewAgentFormErrors {
  const errors: NewAgentFormErrors = {};

  if (!state.name.trim()) {
    errors.name = 'Agent name is required';
  } else if (state.name.length < 2) {
    errors.name = 'Name must be at least 2 characters';
  } else if (state.name.length > 50) {
    errors.name = 'Name must be less than 50 characters';
  } else if (!/^[a-zA-Z][a-zA-Z0-9_-]*$/.test(state.name)) {
    errors.name = 'Name must start with a letter and contain only letters, numbers, underscores, and hyphens';
  }

  return errors;
}

export function NewAgentModal({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting = false,
  submitError,
  className,
}: NewAgentModalProps) {
  const [formState, setFormState] = useState<NewAgentFormState>({ name: '' });
  const [errors, setErrors] = useState<NewAgentFormErrors>({});
  const [isDirty, setIsDirty] = useState(false);

  // Reset form when modal opens/closes.
  const handleClose = () => {
    if (isDirty && !window.confirm('Discard changes?')) {
      return;
    }
    setFormState({ name: '' });
    setErrors({});
    setIsDirty(false);
    onClose();
  };

  // Handle input change.
  const handleChange = (value: string) => {
    setFormState({ name: value });
    setIsDirty(true);

    // Clear error when user starts typing.
    if (errors.name) {
      setErrors({});
    }
  };

  // Handle form submission.
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const validationErrors = validateForm(formState);
    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }

    try {
      await onSubmit(formState);
      // Reset form on success.
      setFormState({ name: '' });
      setErrors({});
      setIsDirty(false);
    } catch {
      // Error is handled by parent via submitError prop.
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Register New Agent"
      className={className}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Submit error message. */}
        {submitError ? (
          <div className="rounded-md bg-red-50 p-3">
            <p className="text-sm text-red-700">{submitError}</p>
          </div>
        ) : null}

        {/* Agent name input. */}
        <TextInput
          label="Agent Name"
          placeholder="e.g., claude-agent-1"
          value={formState.name}
          onChange={(e) => handleChange(e.target.value)}
          error={errors.name}
          helperText="A unique identifier for your agent. Use letters, numbers, underscores, or hyphens."
          disabled={isSubmitting}
          autoFocus
        />

        {/* Agent info. */}
        <div className="rounded-md bg-blue-50 p-3">
          <h4 className="text-sm font-medium text-blue-800">
            What happens when you register?
          </h4>
          <ul className="mt-1 list-inside list-disc text-sm text-blue-700">
            <li>A new agent identity is created</li>
            <li>The agent can send and receive messages</li>
            <li>Activity will be tracked in the dashboard</li>
          </ul>
        </div>

        {/* Form actions. */}
        <div className="flex justify-end gap-3 pt-4">
          <Button
            type="button"
            variant="secondary"
            onClick={handleClose}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            isLoading={isSubmitting}
            disabled={isSubmitting}
          >
            Register Agent
          </Button>
        </div>
      </form>
    </Modal>
  );
}
