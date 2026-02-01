// StartSessionModal component - modal for starting a new session.

import { useState } from 'react';
// import { clsx } from 'clsx';
// import { twMerge } from 'tailwind-merge';
import { Modal } from '@/components/ui/Modal.js';
import { TextInput } from '@/components/ui/Input.js';
import { Button } from '@/components/ui/Button.js';
import type { StartSessionRequest } from '@/types/api.js';


// Form state for new session.
interface StartSessionFormState {
  project: string;
  branch: string;
}

// Form errors.
interface StartSessionFormErrors {
  project?: string;
  branch?: string;
}

// Props for StartSessionModal.
export interface StartSessionModalProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** Handler for submitting the form. */
  onSubmit: (data: StartSessionRequest) => Promise<void>;
  /** Whether the form is submitting. */
  isSubmitting?: boolean;
  /** Error message from submission. */
  submitError?: string | null;
  /** Default project path. */
  defaultProject?: string;
  /** Default branch. */
  defaultBranch?: string;
  /** Additional class name. */
  className?: string;
}

// Validate form state.
function validateForm(state: StartSessionFormState): StartSessionFormErrors {
  const errors: StartSessionFormErrors = {};

  // Project is optional, but if provided must be valid path.
  if (state.project.trim() && !state.project.startsWith('/') && !state.project.startsWith('~')) {
    errors.project = 'Project path must be absolute (start with / or ~)';
  }

  // Branch is optional.

  return errors;
}

export function StartSessionModal({
  isOpen,
  onClose,
  onSubmit,
  isSubmitting = false,
  submitError,
  defaultProject = '',
  defaultBranch = '',
  className,
}: StartSessionModalProps) {
  const [formState, setFormState] = useState<StartSessionFormState>({
    project: defaultProject,
    branch: defaultBranch,
  });
  const [errors, setErrors] = useState<StartSessionFormErrors>({});
  const [isDirty, setIsDirty] = useState(false);

  // Reset form when modal opens/closes.
  const handleClose = () => {
    if (isDirty && !window.confirm('Discard changes?')) {
      return;
    }
    setFormState({ project: defaultProject, branch: defaultBranch });
    setErrors({});
    setIsDirty(false);
    onClose();
  };

  // Handle input change.
  const handleChange = (field: keyof StartSessionFormState, value: string) => {
    setFormState((prev) => ({ ...prev, [field]: value }));
    setIsDirty(true);

    // Clear error when user starts typing.
    if (errors[field]) {
      setErrors((prev) => ({ ...prev, [field]: undefined }));
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
      await onSubmit({
        project: formState.project.trim() || undefined,
        branch: formState.branch.trim() || undefined,
      });
      // Reset form on success.
      setFormState({ project: defaultProject, branch: defaultBranch });
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
      title="Start New Session"
      className={className}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Submit error message. */}
        {submitError ? (
          <div className="rounded-md bg-red-50 p-3">
            <p className="text-sm text-red-700">{submitError}</p>
          </div>
        ) : null}

        {/* Session info. */}
        <div className="rounded-md bg-blue-50 p-3">
          <h4 className="text-sm font-medium text-blue-800">
            About Sessions
          </h4>
          <p className="mt-1 text-sm text-blue-700">
            A session tracks your agent's work context. It records the project
            being worked on and provides continuity across context compactions.
          </p>
        </div>

        {/* Project path input. */}
        <TextInput
          label="Project Path"
          placeholder="e.g., /Users/agent/projects/my-app"
          value={formState.project}
          onChange={(e) => handleChange('project', e.target.value)}
          error={errors.project}
          helperText="The absolute path to the project directory (optional)."
          disabled={isSubmitting}
        />

        {/* Branch input. */}
        <TextInput
          label="Branch"
          placeholder="e.g., main, feature/new-feature"
          value={formState.branch}
          onChange={(e) => handleChange('branch', e.target.value)}
          error={errors.branch}
          helperText="The git branch being worked on (optional)."
          disabled={isSubmitting}
        />

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
            Start Session
          </Button>
        </div>
      </form>
    </Modal>
  );
}
