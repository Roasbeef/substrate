// Modal component using Headless UI for accessibility.

import { Fragment, type ReactNode } from 'react';
import {
  Dialog,
  DialogPanel,
  DialogTitle,
  Transition,
  TransitionChild,
} from '@headlessui/react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl' | 'full';

export interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
  size?: ModalSize | undefined;
  title?: string | undefined;
  description?: string | undefined;
  showCloseButton?: boolean | undefined;
  closeOnOverlayClick?: boolean | undefined;
  className?: string | undefined;
  initialFocus?: React.RefObject<HTMLElement | null> | undefined;
}

// Size styles mapping.
const sizeStyles: Record<ModalSize, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  full: 'max-w-4xl',
};

// Close button icon.
function CloseIcon() {
  return (
    <svg
      className="h-5 w-5"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M6 18L18 6M6 6l12 12"
      />
    </svg>
  );
}

export function Modal({
  isOpen,
  onClose,
  children,
  size = 'md',
  title,
  description,
  showCloseButton = true,
  closeOnOverlayClick = true,
  className,
  initialFocus,
}: ModalProps) {
  const handleClose = () => {
    if (closeOnOverlayClick) {
      onClose();
    }
  };

  return (
    <Transition show={isOpen} as={Fragment}>
      <Dialog
        as="div"
        className="relative z-50"
        onClose={handleClose}
        {...(initialFocus ? { initialFocus } : {})}
      >
        {/* Backdrop. */}
        <TransitionChild
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-black/50" aria-hidden="true" />
        </TransitionChild>

        {/* Modal container. */}
        <div className="fixed inset-0 overflow-y-auto">
          <div className="flex min-h-full items-center justify-center p-4">
            <TransitionChild
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 scale-95"
              enterTo="opacity-100 scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 scale-100"
              leaveTo="opacity-0 scale-95"
            >
              <DialogPanel
                className={cn(
                  'w-full transform overflow-hidden rounded-lg bg-white shadow-xl transition-all',
                  sizeStyles[size],
                  className,
                )}
              >
                {/* Header. */}
                {(title || showCloseButton) ? (
                  <div className="flex items-start justify-between border-b border-gray-200 px-6 py-4">
                    <div>
                      {title ? (
                        <DialogTitle
                          as="h3"
                          className="text-lg font-semibold text-gray-900"
                        >
                          {title}
                        </DialogTitle>
                      ) : null}
                      {description ? (
                        <p className="mt-1 text-sm text-gray-500">{description}</p>
                      ) : null}
                    </div>
                    {showCloseButton ? (
                      <button
                        type="button"
                        className="ml-4 rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                        onClick={onClose}
                        aria-label="Close modal"
                      >
                        <CloseIcon />
                      </button>
                    ) : null}
                  </div>
                ) : null}

                {/* Content. */}
                <div className="px-6 py-4">{children}</div>
              </DialogPanel>
            </TransitionChild>
          </div>
        </div>
      </Dialog>
    </Transition>
  );
}

// Modal footer for actions.
export interface ModalFooterProps {
  children: ReactNode;
  className?: string | undefined;
}

export function ModalFooter({ children, className }: ModalFooterProps) {
  return (
    <div
      className={cn(
        'flex justify-end gap-3 border-t border-gray-200 px-6 py-4 -mx-6 -mb-4 mt-4',
        className,
      )}
    >
      {children}
    </div>
  );
}

// Confirmation modal preset.
export interface ConfirmModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  confirmText?: string | undefined;
  cancelText?: string | undefined;
  variant?: 'danger' | 'primary' | undefined;
  isLoading?: boolean | undefined;
}

export function ConfirmModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'primary',
  isLoading = false,
}: ConfirmModalProps) {
  const confirmButtonClass =
    variant === 'danger'
      ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
      : 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500';

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="sm" title={title}>
      <p className="text-gray-600">{message}</p>
      <ModalFooter>
        <button
          type="button"
          className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          onClick={onClose}
          disabled={isLoading}
        >
          {cancelText}
        </button>
        <button
          type="button"
          className={cn(
            'rounded-md px-4 py-2 text-sm font-medium text-white focus:outline-none focus:ring-2 focus:ring-offset-2',
            confirmButtonClass,
            isLoading ? 'opacity-50 cursor-not-allowed' : '',
          )}
          onClick={onConfirm}
          disabled={isLoading}
        >
          {isLoading ? 'Loading...' : confirmText}
        </button>
      </ModalFooter>
    </Modal>
  );
}
