// ComposeModal component - modal for composing and sending new messages.

import { useCallback, useMemo, useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Modal } from '@/components/ui/Modal.js';
import { Button } from '@/components/ui/Button.js';
import { Input, Textarea, Select } from '@/components/ui/Input.js';
import { RecipientInput } from './RecipientInput.js';
import type {
  AutocompleteRecipient,
  MessagePriority,
  SendMessageRequest,
} from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Form state interface.
interface ComposeFormState {
  recipients: AutocompleteRecipient[];
  subject: string;
  body: string;
  priority: MessagePriority;
  deadline: string;
}

// Initial form state.
const initialFormState: ComposeFormState = {
  recipients: [],
  subject: '',
  body: '',
  priority: 'normal',
  deadline: '',
};

// Priority options.
const priorityOptions: Array<{ value: MessagePriority; label: string }> = [
  { value: 'low', label: 'Low' },
  { value: 'normal', label: 'Normal' },
  { value: 'high', label: 'High' },
  { value: 'urgent', label: 'Urgent' },
];

// Props for ComposeModal component.
export interface ComposeModalProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** Handler for sending the message. */
  onSend: (data: SendMessageRequest) => Promise<void>;
  /** Function to search for recipients. */
  onSearchRecipients: (query: string) => Promise<AutocompleteRecipient[]>;
  /** Whether the send is in progress. */
  isSending?: boolean;
  /** Initial values for the form (for reply drafts). */
  initialValues?: Partial<ComposeFormState>;
  /** Title for the modal. */
  title?: string;
}

export function ComposeModal({
  isOpen,
  onClose,
  onSend,
  onSearchRecipients,
  isSending = false,
  initialValues,
  title = 'Compose Message',
}: ComposeModalProps) {
  // Form state.
  const [form, setForm] = useState<ComposeFormState>(() => ({
    ...initialFormState,
    ...initialValues,
  }));

  // Validation errors.
  const [errors, setErrors] = useState<Partial<Record<keyof ComposeFormState, string>>>({});

  // Track if form has been modified.
  const isDirty = useMemo(() => {
    const initial = { ...initialFormState, ...initialValues };
    return (
      form.recipients.length !== initial.recipients.length ||
      form.subject !== initial.subject ||
      form.body !== initial.body ||
      form.priority !== initial.priority ||
      form.deadline !== initial.deadline
    );
  }, [form, initialValues]);

  // Reset form when modal opens.
  const handleReset = useCallback(() => {
    setForm({ ...initialFormState, ...initialValues });
    setErrors({});
  }, [initialValues]);

  // Handle field change.
  const handleChange = useCallback(
    <K extends keyof ComposeFormState>(field: K, value: ComposeFormState[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }));
      // Clear error when field is modified.
      if (errors[field]) {
        setErrors((prev) => ({ ...prev, [field]: undefined }));
      }
    },
    [errors],
  );

  // Validate form.
  const validate = useCallback((): boolean => {
    const newErrors: Partial<Record<keyof ComposeFormState, string>> = {};

    if (form.recipients.length === 0) {
      newErrors.recipients = 'At least one recipient is required';
    }

    if (!form.subject.trim()) {
      newErrors.subject = 'Subject is required';
    }

    if (!form.body.trim()) {
      newErrors.body = 'Message body is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  }, [form]);

  // Handle submit.
  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      if (!validate()) {
        return;
      }

      const data: SendMessageRequest = {
        to: form.recipients.map((r) => r.id),
        subject: form.subject.trim(),
        body: form.body.trim(),
        priority: form.priority,
        deadline: form.deadline || undefined,
      };

      try {
        await onSend(data);
        handleReset();
        onClose();
      } catch {
        // Error handling is done in parent component.
      }
    },
    [form, validate, onSend, handleReset, onClose],
  );

  // Handle close with confirmation.
  const handleClose = useCallback(() => {
    if (isDirty) {
      const confirmed = window.confirm(
        'You have unsaved changes. Are you sure you want to discard them?',
      );
      if (!confirmed) {
        return;
      }
    }
    handleReset();
    onClose();
  }, [isDirty, handleReset, onClose]);

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="lg" title={title}>
      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
        {/* Recipients. */}
        <RecipientInput
          label="To"
          value={form.recipients}
          onChange={(recipients) => handleChange('recipients', recipients)}
          onSearch={onSearchRecipients}
          placeholder="Search for recipients..."
          disabled={isSending}
          error={errors.recipients}
        />

        {/* Subject. */}
        <Input
          label="Subject"
          value={form.subject}
          onChange={(e) => handleChange('subject', e.target.value)}
          placeholder="Enter subject..."
          disabled={isSending}
          error={errors.subject}
        />

        {/* Body. */}
        <Textarea
          label="Message"
          value={form.body}
          onChange={(e) => handleChange('body', e.target.value)}
          placeholder="Write your message... (Markdown supported)"
          rows={8}
          disabled={isSending}
          error={errors.body}
        />

        {/* Priority and Deadline row. */}
        <div className="grid grid-cols-2 gap-4">
          <Select
            label="Priority"
            value={form.priority}
            onChange={(e) =>
              handleChange('priority', e.target.value as MessagePriority)
            }
            disabled={isSending}
          >
            {priorityOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Select>

          <Input
            type="datetime-local"
            label="Deadline (optional)"
            value={form.deadline}
            onChange={(e) => handleChange('deadline', e.target.value)}
            disabled={isSending}
          />
        </div>

        {/* Actions. */}
        <div className="flex justify-end gap-3 border-t border-gray-200 pt-4">
          <Button
            type="button"
            variant="secondary"
            onClick={handleClose}
            disabled={isSending}
          >
            Cancel
          </Button>
          <Button type="submit" isLoading={isSending} disabled={isSending}>
            Send
          </Button>
        </div>
      </form>
    </Modal>
  );
}

// Hook for managing compose modal state.
export interface UseComposeModalOptions {
  /** Callback when message is sent. */
  onSent?: () => void;
}

export function useComposeModal(options: UseComposeModalOptions = {}) {
  const { onSent } = options;

  const [isOpen, setIsOpen] = useState(false);
  const [initialValues, setInitialValues] = useState<
    Partial<ComposeFormState> | undefined
  >();

  const open = useCallback(
    (values?: Partial<ComposeFormState>) => {
      setInitialValues(values);
      setIsOpen(true);
    },
    [],
  );

  const close = useCallback(() => {
    setIsOpen(false);
    setInitialValues(undefined);
  }, []);

  const handleSent = useCallback(() => {
    close();
    onSent?.();
  }, [close, onSent]);

  return {
    isOpen,
    initialValues,
    open,
    close,
    onSent: handleSent,
  };
}
