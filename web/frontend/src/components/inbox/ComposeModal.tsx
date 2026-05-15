// ComposeModal component - modal for composing and sending new messages.

import { useCallback, useMemo, useState } from 'react';
import { Modal } from '@/components/ui/Modal.js';
import { Button } from '@/components/ui/Button.js';
import { Input, Textarea, Select } from '@/components/ui/Input.js';
import { RecipientInput } from './RecipientInput.js';
import { useAuthStore } from '@/stores/auth.js';
import type {
  AutocompleteRecipient,
  MessagePriority,
  ProtoPriority,
  SendMessageRequest,
} from '@/types/api.js';

// Map human-friendly priority values to proto enum names.
const priorityToProto: Record<MessagePriority, ProtoPriority> = {
  low: 'PRIORITY_LOW',
  normal: 'PRIORITY_NORMAL',
  high: 'PRIORITY_URGENT',
  urgent: 'PRIORITY_URGENT',
};

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

  // Whether the modal is expanded to fullscreen.
  const [isExpanded, setIsExpanded] = useState(false);

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

  // Reset form and UI state when modal opens or closes.
  const handleReset = useCallback(() => {
    setForm({ ...initialFormState, ...initialValues });
    setErrors({});
    setIsExpanded(false);
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

      const { currentAgent, availableAgents } = useAuthStore.getState();
      const userAgent = availableAgents.find((a) => a.name === 'User');
      const senderId = currentAgent?.id ?? userAgent?.id ?? 0;
      const data: SendMessageRequest = {
        sender_id: senderId,
        recipient_names: form.recipients.map((r) => r.name),
        subject: form.subject.trim(),
        body: form.body.trim(),
        priority: priorityToProto[form.priority],
        ...(form.deadline && { deadline_at: form.deadline }),
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

  // Expand/collapse toggle button rendered in the modal header.
  const expandButton = (
    <button
      type="button"
      className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      onClick={() => setIsExpanded((prev) => !prev)}
      aria-label={isExpanded ? 'Exit fullscreen' : 'Fullscreen'}
      title={isExpanded ? 'Exit fullscreen' : 'Fullscreen'}
    >
      {isExpanded ? (
        // Collapse icon (arrows pointing inward).
        <svg className="h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 9L4 4m0 0v4m0-4h4m6 6l5 5m0 0v-4m0 4h-4M9 15l-5 5m0 0v-4m0 4h4m6-6l5-5m0 0v4m0-4h-4" />
        </svg>
      ) : (
        // Expand icon (arrows pointing outward).
        <svg className="h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-5h-4m4 0v4m0-4l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5h-4m4 0v-4m0 4l-5-5" />
        </svg>
      )}
    </button>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      size={isExpanded ? 'full' : '3xl'}
      title={title}
      headerActions={expandButton}
      resizable={!isExpanded}
      className={isExpanded
        ? 'max-w-[calc(100vw-2rem)] !w-[calc(100vw-2rem)] h-[calc(100vh-2rem)]'
        : undefined
      }
    >
      <form
        onSubmit={(e) => void handleSubmit(e)}
        className={
          isExpanded
            // 10rem accounts for: modal padding (2rem) + header (4rem) + footer (4rem).
            ? 'flex flex-col gap-4 h-[calc(100vh-10rem)]'
            : 'space-y-4'
        }
      >
        {/* Recipients. */}
        <RecipientInput
          label="To"
          value={form.recipients}
          onChange={(recipients) => handleChange('recipients', recipients)}
          onSearch={onSearchRecipients}
          placeholder="Search for recipients..."
          disabled={isSending}
          {...(errors.recipients && { error: errors.recipients })}
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

        {/* Body - grows to fill available space when expanded. */}
        <div
          className={
            isExpanded
              ? 'flex-1 flex flex-col min-h-0 [&_div.w-full]:flex-1 [&_div.w-full]:flex [&_div.w-full]:flex-col [&_textarea]:flex-1'
              : ''
          }
        >
          <Textarea
            label="Message"
            value={form.body}
            onChange={(e) => handleChange('body', e.target.value)}
            placeholder="Write your message... (Markdown supported)"
            rows={isExpanded ? undefined : 12}
            disabled={isSending}
            error={errors.body}
            className={isExpanded ? 'resize-none' : ''}
          />
        </div>

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
