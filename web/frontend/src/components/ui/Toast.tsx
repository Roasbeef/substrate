// Toast notification component using Zustand store for state management.

import React, { useEffect, useCallback } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore, type Toast as ToastData } from '@/stores/ui';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type ToastVariant = 'success' | 'error' | 'warning' | 'info';

// Icon components for each variant.
function SuccessIcon() {
  return (
    <svg
      className="h-5 w-5 text-green-400"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function ErrorIcon() {
  return (
    <svg
      className="h-5 w-5 text-red-400"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function WarningIcon() {
  return (
    <svg
      className="h-5 w-5 text-yellow-400"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function InfoIcon() {
  return (
    <svg
      className="h-5 w-5 text-blue-400"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg
      className="h-4 w-4"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
    </svg>
  );
}

// Variant icon mapping.
const variantIcons: Record<ToastVariant, () => React.JSX.Element> = {
  success: SuccessIcon,
  error: ErrorIcon,
  warning: WarningIcon,
  info: InfoIcon,
};

// Variant styles mapping.
const variantStyles: Record<ToastVariant, string> = {
  success: 'bg-white border-green-200',
  error: 'bg-white border-red-200',
  warning: 'bg-white border-yellow-200',
  info: 'bg-white border-blue-200',
};

// Single toast item component.
interface ToastItemProps {
  toast: ToastData;
  onClose: (id: string) => void;
}

function ToastItem({ toast, onClose }: ToastItemProps) {
  const Icon = variantIcons[toast.variant];

  // Auto-dismiss after duration.
  useEffect(() => {
    if (toast.duration && toast.duration > 0) {
      const timer = setTimeout(() => {
        onClose(toast.id);
      }, toast.duration);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [toast.id, toast.duration, onClose]);

  return (
    <div
      className={cn(
        'pointer-events-auto w-full max-w-sm overflow-hidden rounded-lg border shadow-lg',
        'animate-slide-in',
        variantStyles[toast.variant],
      )}
      role="alert"
      aria-live="assertive"
      aria-atomic="true"
    >
      <div className="p-4">
        <div className="flex items-start">
          <div className="flex-shrink-0">
            <Icon />
          </div>
          <div className="ml-3 w-0 flex-1 pt-0.5">
            {toast.title ? (
              <p className="text-sm font-medium text-gray-900">{toast.title}</p>
            ) : null}
            <p
              className={cn(
                'text-sm text-gray-500',
                toast.title ? 'mt-1' : '',
              )}
            >
              {toast.message}
            </p>
            {toast.action ? (
              <div className="mt-3 flex space-x-7">
                <button
                  type="button"
                  onClick={toast.action.onClick}
                  className="rounded-md bg-white text-sm font-medium text-blue-600 hover:text-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  {toast.action.label}
                </button>
              </div>
            ) : null}
          </div>
          <div className="ml-4 flex flex-shrink-0">
            <button
              type="button"
              className="inline-flex rounded-md bg-white text-gray-400 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              onClick={() => onClose(toast.id)}
              aria-label="Close notification"
            >
              <CloseIcon />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// Toast container component that renders all toasts.
export function ToastContainer() {
  const toasts = useUIStore((state) => state.toasts);
  const removeToast = useUIStore((state) => state.removeToast);

  const handleClose = useCallback(
    (id: string) => {
      removeToast(id);
    },
    [removeToast],
  );

  if (toasts.length === 0) {
    return null;
  }

  return (
    <div
      aria-live="assertive"
      className="pointer-events-none fixed inset-0 z-50 flex items-end px-4 py-6 sm:items-start sm:p-6"
    >
      <div className="flex w-full flex-col items-center space-y-4 sm:items-end">
        {toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} onClose={handleClose} />
        ))}
      </div>
    </div>
  );
}

// Hook for showing toasts.
export function useToast() {
  const addToast = useUIStore((state) => state.addToast);
  const removeToast = useUIStore((state) => state.removeToast);

  const toast = useCallback(
    (options: Omit<ToastData, 'id'>) => {
      addToast(options);
    },
    [addToast],
  );

  const success = useCallback(
    (message: string, options?: Partial<Omit<ToastData, 'id' | 'variant' | 'message'>>) => {
      addToast({ ...options, message, variant: 'success' });
    },
    [addToast],
  );

  const error = useCallback(
    (message: string, options?: Partial<Omit<ToastData, 'id' | 'variant' | 'message'>>) => {
      addToast({ ...options, message, variant: 'error' });
    },
    [addToast],
  );

  const warning = useCallback(
    (message: string, options?: Partial<Omit<ToastData, 'id' | 'variant' | 'message'>>) => {
      addToast({ ...options, message, variant: 'warning' });
    },
    [addToast],
  );

  const info = useCallback(
    (message: string, options?: Partial<Omit<ToastData, 'id' | 'variant' | 'message'>>) => {
      addToast({ ...options, message, variant: 'info' });
    },
    [addToast],
  );

  const dismiss = useCallback(
    (id: string) => {
      removeToast(id);
    },
    [removeToast],
  );

  return {
    toast,
    success,
    error,
    warning,
    info,
    dismiss,
  };
}
