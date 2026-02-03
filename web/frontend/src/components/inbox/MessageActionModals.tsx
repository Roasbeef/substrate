// Modal components for message actions (delete confirmation, snooze picker).

import { Fragment, useState } from 'react';
import {
  Dialog,
  DialogPanel,
  DialogTitle,
  Transition,
  TransitionChild,
} from '@headlessui/react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Button } from '@/components/ui/Button.js';
import { snoozeDurations, type SnoozeDuration } from '@/hooks/useMessageActions.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Warning icon.
function WarningIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-6 w-6', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}

// Clock icon.
function ClockIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-6 w-6', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

// Delete confirmation modal props.
export interface DeleteConfirmationModalProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** Handler for confirming deletion. */
  onConfirm: () => void;
  /** Number of messages to delete (for bulk delete). */
  count?: number;
  /** Whether the delete operation is in progress. */
  isDeleting?: boolean;
}

export function DeleteConfirmationModal({
  isOpen,
  onClose,
  onConfirm,
  count = 1,
  isDeleting = false,
}: DeleteConfirmationModalProps) {
  const isBulk = count > 1;
  const title = isBulk ? `Delete ${count} messages?` : 'Delete message?';
  const description = isBulk
    ? `Are you sure you want to delete ${count} messages? This action cannot be undone.`
    : 'Are you sure you want to delete this message? This action cannot be undone.';

  return (
    <Transition show={isOpen} as={Fragment}>
      <Dialog onClose={onClose} className="relative z-50">
        {/* Backdrop. */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-200"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-150"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div
            className="fixed inset-0 bg-gray-900/50 backdrop-blur-sm"
            aria-hidden="true"
          />
        </TransitionChild>

        {/* Dialog. */}
        <div className="fixed inset-0 flex items-center justify-center p-4">
          <TransitionChild
            as={Fragment}
            enter="ease-out duration-200"
            enterFrom="opacity-0 scale-95"
            enterTo="opacity-100 scale-100"
            leave="ease-in duration-150"
            leaveFrom="opacity-100 scale-100"
            leaveTo="opacity-0 scale-95"
          >
            <DialogPanel className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
              <div className="flex items-start gap-4">
                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-red-100">
                  <WarningIcon className="text-red-600" />
                </div>
                <div className="flex-1">
                  <DialogTitle className="text-lg font-semibold text-gray-900">
                    {title}
                  </DialogTitle>
                  <p className="mt-2 text-sm text-gray-500">{description}</p>
                </div>
              </div>

              <div className="mt-6 flex justify-end gap-3">
                <Button variant="outline" onClick={onClose} disabled={isDeleting}>
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  onClick={onConfirm}
                  isLoading={isDeleting}
                >
                  Delete
                </Button>
              </div>
            </DialogPanel>
          </TransitionChild>
        </div>
      </Dialog>
    </Transition>
  );
}

// Snooze picker modal props.
export interface SnoozePickerModalProps {
  /** Whether the modal is open. */
  isOpen: boolean;
  /** Handler for closing the modal. */
  onClose: () => void;
  /** Handler for selecting a snooze time. */
  onSnooze: (until: string) => void;
  /** Whether the snooze operation is in progress. */
  isSnoozing?: boolean;
}

export function SnoozePickerModal({
  isOpen,
  onClose,
  onSnooze,
  isSnoozing = false,
}: SnoozePickerModalProps) {
  const [customDate, setCustomDate] = useState('');
  const [customTime, setCustomTime] = useState('09:00');

  const handlePresetClick = (duration: SnoozeDuration) => {
    const date = duration.getDate();
    onSnooze(date.toISOString());
  };

  const handleCustomSnooze = () => {
    if (!customDate) return;
    const date = new Date(`${customDate}T${customTime}`);
    onSnooze(date.toISOString());
  };

  // Format date for display.
  const formatPresetDate = (duration: SnoozeDuration): string => {
    const date = duration.getDate();
    return date.toLocaleDateString(undefined, {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  };

  return (
    <Transition show={isOpen} as={Fragment}>
      <Dialog onClose={onClose} className="relative z-50">
        {/* Backdrop. */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-200"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-150"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div
            className="fixed inset-0 bg-gray-900/50 backdrop-blur-sm"
            aria-hidden="true"
          />
        </TransitionChild>

        {/* Dialog. */}
        <div className="fixed inset-0 flex items-center justify-center p-4">
          <TransitionChild
            as={Fragment}
            enter="ease-out duration-200"
            enterFrom="opacity-0 scale-95"
            enterTo="opacity-100 scale-100"
            leave="ease-in duration-150"
            leaveFrom="opacity-100 scale-100"
            leaveTo="opacity-0 scale-95"
          >
            <DialogPanel className="w-full max-w-sm rounded-xl bg-white shadow-xl">
              <div className="flex items-center gap-3 border-b border-gray-200 px-6 py-4">
                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-blue-100">
                  <ClockIcon className="text-blue-600" />
                </div>
                <DialogTitle className="text-lg font-semibold text-gray-900">
                  Snooze until
                </DialogTitle>
              </div>

              {/* Preset options. */}
              <div className="px-6 py-4">
                <div className="space-y-1">
                  {snoozeDurations.map((duration) => (
                    <button
                      key={duration.label}
                      type="button"
                      onClick={() => handlePresetClick(duration)}
                      disabled={isSnoozing}
                      className={cn(
                        'flex w-full items-center justify-between rounded-lg px-4 py-3',
                        'text-left hover:bg-gray-50 focus:bg-gray-50',
                        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-inset',
                        isSnoozing ? 'cursor-not-allowed opacity-50' : '',
                      )}
                    >
                      <span className="font-medium text-gray-900">
                        {duration.label}
                      </span>
                      <span className="text-sm text-gray-500">
                        {formatPresetDate(duration)}
                      </span>
                    </button>
                  ))}
                </div>

                {/* Divider. */}
                <div className="my-4 flex items-center gap-2">
                  <div className="h-px flex-1 bg-gray-200" />
                  <span className="text-xs text-gray-400">or pick a date</span>
                  <div className="h-px flex-1 bg-gray-200" />
                </div>

                {/* Custom date/time picker. */}
                <div className="flex gap-2">
                  <input
                    type="date"
                    value={customDate}
                    onChange={(e) => setCustomDate(e.target.value)}
                    min={new Date().toISOString().split('T')[0]}
                    className={cn(
                      'flex-1 rounded-lg border border-gray-200 px-3 py-2',
                      'text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
                    )}
                    disabled={isSnoozing}
                  />
                  <input
                    type="time"
                    value={customTime}
                    onChange={(e) => setCustomTime(e.target.value)}
                    className={cn(
                      'w-24 rounded-lg border border-gray-200 px-3 py-2',
                      'text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500',
                    )}
                    disabled={isSnoozing}
                  />
                </div>
              </div>

              {/* Actions. */}
              <div className="flex justify-end gap-3 border-t border-gray-200 px-6 py-4">
                <Button variant="outline" onClick={onClose} disabled={isSnoozing}>
                  Cancel
                </Button>
                <Button
                  onClick={handleCustomSnooze}
                  disabled={!customDate || isSnoozing}
                  isLoading={isSnoozing}
                >
                  Snooze
                </Button>
              </div>
            </DialogPanel>
          </TransitionChild>
        </div>
      </Dialog>
    </Transition>
  );
}

// Bulk actions toolbar props.
export interface BulkActionsToolbarProps {
  /** Number of selected messages. */
  selectedCount: number;
  /** Handler for deselecting all. */
  onClearSelection: () => void;
  /** Handler for bulk star. */
  onStar?: () => void;
  /** Handler for bulk archive. */
  onArchive?: () => void;
  /** Handler for bulk mark read. */
  onMarkRead?: () => void;
  /** Handler for bulk delete. */
  onDelete?: () => void;
  /** Whether any action is in progress. */
  isLoading?: boolean;
  /** Additional class name. */
  className?: string;
}

export function BulkActionsToolbar({
  selectedCount,
  onClearSelection,
  onStar,
  onArchive,
  onMarkRead,
  onDelete,
  isLoading = false,
  className,
}: BulkActionsToolbarProps) {
  if (selectedCount === 0) return null;

  return (
    <div
      className={cn(
        'flex items-center gap-4 rounded-lg bg-blue-50 px-4 py-3',
        className,
      )}
    >
      <span className="text-sm font-medium text-blue-900">
        {selectedCount} selected
      </span>

      <div className="flex items-center gap-2">
        {onStar ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={onStar}
            disabled={isLoading}
          >
            Star
          </Button>
        ) : null}

        {onMarkRead ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={onMarkRead}
            disabled={isLoading}
          >
            Mark read
          </Button>
        ) : null}

        {onArchive ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={onArchive}
            disabled={isLoading}
          >
            Archive
          </Button>
        ) : null}

        {onDelete ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={onDelete}
            disabled={isLoading}
            className="text-red-600 hover:bg-red-50 hover:text-red-700"
          >
            Delete
          </Button>
        ) : null}
      </div>

      <div className="flex-1" />

      <Button
        variant="ghost"
        size="sm"
        onClick={onClearSelection}
        disabled={isLoading}
      >
        Clear selection
      </Button>
    </div>
  );
}
