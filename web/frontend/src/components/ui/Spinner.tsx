// Spinner component for loading states.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type SpinnerSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';
export type SpinnerVariant = 'default' | 'primary' | 'white';

export interface SpinnerProps {
  /** Size of the spinner. */
  size?: SpinnerSize | undefined;
  /** Color variant. */
  variant?: SpinnerVariant | undefined;
  /** Additional class names. */
  className?: string | undefined;
  /** Accessible label for screen readers. */
  label?: string | undefined;
}

// Size styles mapping.
const sizeStyles: Record<SpinnerSize, string> = {
  xs: 'h-3 w-3',
  sm: 'h-4 w-4',
  md: 'h-6 w-6',
  lg: 'h-8 w-8',
  xl: 'h-12 w-12',
};

// Variant styles mapping.
const variantStyles: Record<SpinnerVariant, string> = {
  default: 'text-gray-400',
  primary: 'text-blue-600',
  white: 'text-white',
};

export function Spinner({
  size = 'md',
  variant = 'default',
  className,
  label = 'Loading',
}: SpinnerProps) {
  return (
    <div
      role="status"
      aria-label={label}
      className={cn('inline-flex items-center justify-center', className)}
    >
      <svg
        className={cn(
          'animate-spin',
          sizeStyles[size],
          variantStyles[variant],
        )}
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
        aria-hidden="true"
      >
        <circle
          className="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          strokeWidth="4"
        />
        <path
          className="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        />
      </svg>
      <span className="sr-only">{label}</span>
    </div>
  );
}

// Full page loading overlay.
export interface LoadingOverlayProps {
  /** Whether to show the overlay. */
  isLoading: boolean;
  /** Loading message to display. */
  message?: string | undefined;
  /** Whether to cover the entire viewport. */
  fullScreen?: boolean | undefined;
}

export function LoadingOverlay({
  isLoading,
  message = 'Loading...',
  fullScreen = false,
}: LoadingOverlayProps) {
  if (!isLoading) {
    return null;
  }

  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center bg-white/80 backdrop-blur-sm',
        fullScreen
          ? 'fixed inset-0 z-50'
          : 'absolute inset-0 z-10',
      )}
      role="alert"
      aria-busy="true"
      aria-live="polite"
    >
      <Spinner size="lg" variant="primary" />
      <p className="mt-3 text-sm text-gray-600">{message}</p>
    </div>
  );
}

// Inline loading indicator for buttons or inline content.
export interface InlineLoadingProps {
  /** Size of the spinner. */
  size?: SpinnerSize | undefined;
  /** Loading text to display. */
  text?: string | undefined;
  /** Color variant. */
  variant?: SpinnerVariant | undefined;
  /** Additional class names. */
  className?: string | undefined;
}

export function InlineLoading({
  size = 'sm',
  text,
  variant = 'default',
  className,
}: InlineLoadingProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <Spinner size={size} variant={variant} label={text ?? 'Loading'} />
      {text ? (
        <span className="text-sm text-gray-600">{text}</span>
      ) : null}
    </span>
  );
}

// Skeleton loading placeholder.
export interface SkeletonProps {
  /** Width class or style. */
  width?: string | undefined;
  /** Height class or style. */
  height?: string | undefined;
  /** Whether to show as a circle. */
  circle?: boolean | undefined;
  /** Additional class names. */
  className?: string | undefined;
}

export function Skeleton({
  width = 'w-full',
  height = 'h-4',
  circle = false,
  className,
}: SkeletonProps) {
  return (
    <div
      className={cn(
        'animate-pulse bg-gray-200',
        circle ? 'rounded-full' : 'rounded',
        width,
        height,
        className,
      )}
      aria-hidden="true"
    />
  );
}

// Skeleton text block.
export interface SkeletonTextProps {
  lines?: number | undefined;
  className?: string | undefined;
}

export function SkeletonText({ lines = 3, className }: SkeletonTextProps) {
  return (
    <div className={cn('space-y-2', className)}>
      {Array.from({ length: lines }).map((_, index) => (
        <Skeleton
          key={index}
          width={index === lines - 1 ? 'w-3/4' : 'w-full'}
        />
      ))}
    </div>
  );
}
