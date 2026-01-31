// Input component with variants for text input, textarea, and validation states.

import {
  forwardRef,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
  type ReactNode,
} from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export type InputSize = 'sm' | 'md' | 'lg';

export interface BaseInputProps {
  size?: InputSize | undefined;
  label?: string | undefined;
  helperText?: string | undefined;
  error?: string | undefined;
  leftIcon?: ReactNode | undefined;
  rightIcon?: ReactNode | undefined;
}

export interface TextInputProps
  extends BaseInputProps,
    Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  multiline?: false | undefined;
}

export interface TextareaProps
  extends BaseInputProps,
    Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'> {
  multiline: true;
  rows?: number | undefined;
}

export type InputProps = TextInputProps | TextareaProps;

// Size styles mapping.
const sizeStyles: Record<InputSize, string> = {
  sm: 'px-2.5 py-1.5 text-xs',
  md: 'px-3 py-2 text-sm',
  lg: 'px-4 py-3 text-base',
};

// Icon size styles mapping.
const iconSizeStyles: Record<InputSize, string> = {
  sm: 'w-4 h-4',
  md: 'w-5 h-5',
  lg: 'w-6 h-6',
};

// Base input styles.
const baseInputStyles = cn(
  'block w-full rounded-md border shadow-sm',
  'transition-colors duration-150 ease-in-out',
  'focus:outline-none focus:ring-2 focus:ring-offset-0',
  'disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-500',
  'placeholder:text-gray-400',
);

// Get border and focus styles based on error state.
function getBorderStyles(hasError: boolean): string {
  if (hasError) {
    return 'border-red-300 focus:border-red-500 focus:ring-red-500';
  }
  return 'border-gray-300 focus:border-blue-500 focus:ring-blue-500';
}

// Input wrapper for icons.
function InputWrapper({
  children,
  leftIcon,
  rightIcon,
  size,
}: {
  children: ReactNode;
  leftIcon?: ReactNode | undefined;
  rightIcon?: ReactNode | undefined;
  size: InputSize;
}) {
  if (!leftIcon && !rightIcon) {
    return <>{children}</>;
  }

  return (
    <div className="relative">
      {leftIcon ? (
        <div
          className={cn(
            'pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400',
            iconSizeStyles[size],
          )}
        >
          {leftIcon}
        </div>
      ) : null}
      {children}
      {rightIcon ? (
        <div
          className={cn(
            'pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400',
            iconSizeStyles[size],
          )}
        >
          {rightIcon}
        </div>
      ) : null}
    </div>
  );
}

// Label component.
function Label({
  htmlFor,
  children,
  required,
}: {
  htmlFor: string;
  children: ReactNode;
  required?: boolean | undefined;
}) {
  return (
    <label htmlFor={htmlFor} className="mb-1 block text-sm font-medium text-gray-700">
      {children}
      {required ? <span className="ml-1 text-red-500">*</span> : null}
    </label>
  );
}

// Helper text component.
function HelperText({
  children,
  isError,
  id,
}: {
  children: ReactNode;
  isError?: boolean | undefined;
  id?: string | undefined;
}) {
  return (
    <p id={id} className={cn('mt-1 text-sm', isError ? 'text-red-600' : 'text-gray-500')}>
      {children}
    </p>
  );
}

// TextInput component.
export const TextInput = forwardRef<HTMLInputElement, TextInputProps>(
  function TextInput(
    {
      size = 'md',
      label,
      helperText,
      error,
      leftIcon,
      rightIcon,
      className,
      id,
      required,
      disabled,
      ...props
    },
    ref,
  ) {
    const inputId = id ?? `input-${Math.random().toString(36).slice(2, 9)}`;
    const hasError = Boolean(error);
    const describedBy = error ? `${inputId}-error` : helperText ? `${inputId}-helper` : undefined;

    return (
      <div className="w-full">
        {label ? (
          <Label htmlFor={inputId} required={required}>
            {label}
          </Label>
        ) : null}
        <InputWrapper leftIcon={leftIcon} rightIcon={rightIcon} size={size}>
          <input
            ref={ref}
            id={inputId}
            type="text"
            disabled={disabled}
            required={required}
            aria-invalid={hasError}
            aria-describedby={describedBy}
            className={cn(
              baseInputStyles,
              getBorderStyles(hasError),
              sizeStyles[size],
              leftIcon ? 'pl-10' : '',
              rightIcon ? 'pr-10' : '',
              className,
            )}
            {...props}
          />
        </InputWrapper>
        {error ? (
          <HelperText isError id={`${inputId}-error`}>
            {error}
          </HelperText>
        ) : helperText ? (
          <HelperText id={`${inputId}-helper`}>{helperText}</HelperText>
        ) : null}
      </div>
    );
  },
);

// Textarea component.
export const Textarea = forwardRef<HTMLTextAreaElement, Omit<TextareaProps, 'multiline'>>(
  function Textarea(
    {
      size = 'md',
      label,
      helperText,
      error,
      className,
      id,
      required,
      disabled,
      rows = 4,
      ...props
    },
    ref,
  ) {
    const inputId = id ?? `textarea-${Math.random().toString(36).slice(2, 9)}`;
    const hasError = Boolean(error);
    const describedBy = error ? `${inputId}-error` : helperText ? `${inputId}-helper` : undefined;

    return (
      <div className="w-full">
        {label ? (
          <Label htmlFor={inputId} required={required}>
            {label}
          </Label>
        ) : null}
        <textarea
          ref={ref}
          id={inputId}
          disabled={disabled}
          required={required}
          rows={rows}
          aria-invalid={hasError}
          aria-describedby={describedBy}
          className={cn(
            baseInputStyles,
            getBorderStyles(hasError),
            sizeStyles[size],
            'resize-y',
            className,
          )}
          {...props}
        />
        {error ? (
          <HelperText isError id={`${inputId}-error`}>
            {error}
          </HelperText>
        ) : helperText ? (
          <HelperText id={`${inputId}-helper`}>{helperText}</HelperText>
        ) : null}
      </div>
    );
  },
);

// Combined Input component that handles both text and textarea.
export const Input = forwardRef<HTMLInputElement | HTMLTextAreaElement, InputProps>(
  function Input(props, ref) {
    if (props.multiline) {
      const { multiline: _, ...textareaProps } = props;
      return (
        <Textarea
          ref={ref as React.Ref<HTMLTextAreaElement>}
          {...textareaProps}
        />
      );
    }

    const { multiline: _, ...inputProps } = props as TextInputProps;
    return (
      <TextInput
        ref={ref as React.Ref<HTMLInputElement>}
        {...inputProps}
      />
    );
  },
);

// Select component props.
export interface SelectProps
  extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'size'> {
  size?: InputSize | undefined;
  label?: string | undefined;
  helperText?: string | undefined;
  error?: string | undefined;
  children: ReactNode;
}

// Select component.
export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  function Select(
    {
      size = 'md',
      label,
      helperText,
      error,
      className,
      id,
      required,
      disabled,
      children,
      ...props
    },
    ref,
  ) {
    const selectId = id ?? `select-${Math.random().toString(36).slice(2, 9)}`;
    const hasError = Boolean(error);
    const describedBy = error ? `${selectId}-error` : helperText ? `${selectId}-helper` : undefined;

    return (
      <div className="w-full">
        {label ? (
          <Label htmlFor={selectId} required={required}>
            {label}
          </Label>
        ) : null}
        <select
          ref={ref}
          id={selectId}
          disabled={disabled}
          required={required}
          aria-invalid={hasError}
          aria-describedby={describedBy}
          className={cn(
            baseInputStyles,
            getBorderStyles(hasError),
            sizeStyles[size],
            'appearance-none bg-white pr-10',
            'bg-no-repeat bg-right-3',
            className,
          )}
          style={{
            backgroundImage: `url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e")`,
            backgroundPosition: 'right 0.5rem center',
            backgroundSize: '1.5em 1.5em',
          }}
          {...props}
        >
          {children}
        </select>
        {error ? (
          <HelperText isError id={`${selectId}-error`}>
            {error}
          </HelperText>
        ) : helperText ? (
          <HelperText id={`${selectId}-helper`}>{helperText}</HelperText>
        ) : null}
      </div>
    );
  },
);
