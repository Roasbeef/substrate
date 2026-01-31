// FilterBar component - toolbar with filters, search, and bulk actions.

import { type ReactNode, useState } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { FilterTabs, type FilterType } from './CategoryTabs.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Icon components.
function CheckIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 13l4 4L19 7"
      />
    </svg>
  );
}

function RefreshIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
      />
    </svg>
  );
}

function ArchiveIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"
      />
    </svg>
  );
}

function TrashIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
      />
    </svg>
  );
}

function StarIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('h-5 w-5', className)}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"
      />
    </svg>
  );
}

// Checkbox component.
interface CheckboxProps {
  checked: boolean;
  indeterminate?: boolean;
  onChange: (checked: boolean) => void;
  ariaLabel?: string;
  className?: string;
}

function Checkbox({
  checked,
  indeterminate = false,
  onChange,
  ariaLabel,
  className,
}: CheckboxProps) {
  return (
    <div className={cn('relative flex items-center', className)}>
      <input
        type="checkbox"
        checked={checked}
        ref={(el) => {
          if (el) {
            el.indeterminate = indeterminate;
          }
        }}
        onChange={(e) => onChange(e.target.checked)}
        className={cn(
          'h-4 w-4 rounded border-gray-300 text-blue-600',
          'focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        )}
        aria-label={ariaLabel}
      />
    </div>
  );
}

// Icon button for toolbar actions.
interface ToolbarButtonProps {
  onClick: () => void;
  icon: ReactNode;
  label: string;
  disabled?: boolean;
  className?: string;
}

function ToolbarButton({
  onClick,
  icon,
  label,
  disabled = false,
  className,
}: ToolbarButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'flex items-center justify-center rounded-md p-2',
        'text-gray-500 hover:bg-gray-100 hover:text-gray-700',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        disabled ? 'cursor-not-allowed opacity-50' : '',
        className,
      )}
      title={label}
      aria-label={label}
    >
      {icon}
    </button>
  );
}

// FilterBar props.
export interface FilterBarProps {
  /** Current filter type. */
  filter: FilterType;
  /** Handler for filter change. */
  onFilterChange: (filter: FilterType) => void;
  /** Number of selected messages. */
  selectedCount: number;
  /** Total number of messages. */
  totalCount: number;
  /** Whether all messages are selected. */
  allSelected: boolean;
  /** Whether selection is indeterminate. */
  isIndeterminate: boolean;
  /** Handler for select all toggle. */
  onSelectAll: (selected: boolean) => void;
  /** Handler for refresh. */
  onRefresh?: () => void;
  /** Handler for archive action. */
  onArchive?: () => void;
  /** Handler for delete action. */
  onDelete?: () => void;
  /** Handler for star action. */
  onStar?: () => void;
  /** Whether actions are loading. */
  isLoading?: boolean;
  /** Additional class name. */
  className?: string;
}

export function FilterBar({
  filter,
  onFilterChange,
  selectedCount,
  totalCount,
  allSelected,
  isIndeterminate,
  onSelectAll,
  onRefresh,
  onArchive,
  onDelete,
  onStar,
  isLoading = false,
  className,
}: FilterBarProps) {
  const hasSelection = selectedCount > 0;

  return (
    <div
      className={cn(
        'flex items-center justify-between gap-4 border-b border-gray-200 bg-white px-4 py-2',
        className,
      )}
    >
      {/* Left section - checkbox and bulk actions. */}
      <div className="flex items-center gap-2">
        <Checkbox
          checked={allSelected}
          indeterminate={isIndeterminate}
          onChange={onSelectAll}
          ariaLabel={allSelected ? 'Deselect all' : 'Select all'}
        />

        {hasSelection ? (
          <>
            <span className="ml-2 text-sm text-gray-600">
              {selectedCount} selected
            </span>
            <div className="ml-2 flex items-center border-l border-gray-200 pl-2">
              {onArchive ? (
                <ToolbarButton
                  onClick={onArchive}
                  icon={<ArchiveIcon />}
                  label="Archive selected"
                  disabled={isLoading}
                />
              ) : null}
              {onStar ? (
                <ToolbarButton
                  onClick={onStar}
                  icon={<StarIcon />}
                  label="Star selected"
                  disabled={isLoading}
                />
              ) : null}
              {onDelete ? (
                <ToolbarButton
                  onClick={onDelete}
                  icon={<TrashIcon />}
                  label="Delete selected"
                  disabled={isLoading}
                />
              ) : null}
            </div>
          </>
        ) : (
          <div className="ml-2 flex items-center gap-2">
            {onRefresh ? (
              <ToolbarButton
                onClick={onRefresh}
                icon={<RefreshIcon className={isLoading ? 'animate-spin' : ''} />}
                label="Refresh"
                disabled={isLoading}
              />
            ) : null}
            <span className="text-sm text-gray-500">{totalCount} messages</span>
          </div>
        )}
      </div>

      {/* Right section - filter tabs. */}
      <FilterTabs
        selected={filter}
        onSelect={onFilterChange}
        disabled={isLoading}
      />
    </div>
  );
}

// Simple filter bar without bulk actions.
export interface SimpleFilterBarProps {
  /** Current filter type. */
  filter: FilterType;
  /** Handler for filter change. */
  onFilterChange: (filter: FilterType) => void;
  /** Total number of messages. */
  totalCount?: number;
  /** Handler for refresh. */
  onRefresh?: () => void;
  /** Whether loading. */
  isLoading?: boolean;
  /** Additional class name. */
  className?: string;
}

export function SimpleFilterBar({
  filter,
  onFilterChange,
  totalCount,
  onRefresh,
  isLoading = false,
  className,
}: SimpleFilterBarProps) {
  return (
    <div
      className={cn(
        'flex items-center justify-between gap-4 bg-white px-4 py-3',
        className,
      )}
    >
      <div className="flex items-center gap-3">
        {onRefresh ? (
          <ToolbarButton
            onClick={onRefresh}
            icon={<RefreshIcon className={isLoading ? 'animate-spin' : ''} />}
            label="Refresh"
            disabled={isLoading}
          />
        ) : null}
        {totalCount !== undefined ? (
          <span className="text-sm text-gray-500">{totalCount} messages</span>
        ) : null}
      </div>

      <FilterTabs
        selected={filter}
        onSelect={onFilterChange}
        disabled={isLoading}
      />
    </div>
  );
}
