// Header component - top navigation bar with search, agent switcher, and settings.

import { type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore } from '@/stores/ui.js';
import { routes } from '@/router.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export interface HeaderProps {
  /** Additional class name for the header. */
  className?: string;
  /** Optional left-side content (e.g., toggle button). */
  leftContent?: ReactNode;
  /** Optional right-side content (e.g., user menu). */
  rightContent?: ReactNode;
}

// Menu icon for sidebar toggle.
function MenuIcon({ className }: { className?: string }) {
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
        d="M4 6h16M4 12h16M4 18h16"
      />
    </svg>
  );
}

// Search icon.
function SearchIcon({ className }: { className?: string }) {
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
        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
      />
    </svg>
  );
}

// Settings icon.
function SettingsIcon({ className }: { className?: string }) {
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
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
      />
    </svg>
  );
}

// Bell icon for notifications.
function BellIcon({ className }: { className?: string }) {
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
        d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
      />
    </svg>
  );
}

// Icon button component.
interface IconButtonProps {
  onClick?: () => void;
  ariaLabel: string;
  children: ReactNode;
  className?: string;
  showBadge?: boolean;
}

function IconButton({
  onClick,
  ariaLabel,
  children,
  className,
  showBadge = false,
}: IconButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'relative rounded-md p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-500',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        className,
      )}
      aria-label={ariaLabel}
    >
      {children}
      {showBadge ? (
        <span className="absolute right-1 top-1 block h-2 w-2 rounded-full bg-red-500 ring-2 ring-white" />
      ) : null}
    </button>
  );
}

// Search bar component for the header.
function HeaderSearchBar() {
  const toggleSearch = useUIStore((state) => state.toggleSearch);

  return (
    <button
      type="button"
      onClick={toggleSearch}
      className={cn(
        'flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2',
        'text-sm text-gray-500 hover:border-gray-300 hover:bg-gray-100',
        'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
        'min-w-[200px] md:min-w-[300px]',
      )}
    >
      <SearchIcon className="text-gray-400" />
      <span className="flex-1 text-left">Search messages...</span>
      <kbd className="hidden rounded bg-gray-200 px-1.5 py-0.5 text-xs font-medium text-gray-500 md:inline-block">
        ⌘K
      </kbd>
    </button>
  );
}

// Main Header component.
export function Header({ className, leftContent, rightContent }: HeaderProps) {
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);
  const toggleSearch = useUIStore((state) => state.toggleSearch);

  return (
    <header
      className={cn(
        'flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4',
        className,
      )}
    >
      {/* Left section - sidebar toggle and custom content. */}
      <div className="flex items-center gap-4">
        <IconButton onClick={toggleSidebar} ariaLabel="Toggle sidebar">
          <MenuIcon />
        </IconButton>

        {leftContent}

        {/* Search bar - visible on larger screens. */}
        <div className="hidden md:block">
          <HeaderSearchBar />
        </div>
      </div>

      {/* Right section - actions and custom content. */}
      <div className="flex items-center gap-2">
        {/* Mobile search button. */}
        <div className="md:hidden">
          <IconButton onClick={toggleSearch} ariaLabel="Search">
            <SearchIcon />
          </IconButton>
        </div>

        {/* Notifications button. */}
        <IconButton ariaLabel="View notifications" showBadge>
          <BellIcon />
        </IconButton>

        {/* Settings link. */}
        <Link
          to={routes.settings}
          className={cn(
            'rounded-md p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-500',
            'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
          )}
          aria-label="Settings"
        >
          <SettingsIcon />
        </Link>

        {rightContent}
      </div>
    </header>
  );
}

// Compact header variant for mobile or reduced layouts.
export interface CompactHeaderProps {
  title?: string;
  onBack?: () => void;
  rightContent?: ReactNode;
  className?: string;
}

// Back arrow icon.
function ArrowLeftIcon({ className }: { className?: string }) {
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
        d="M10 19l-7-7m0 0l7-7m-7 7h18"
      />
    </svg>
  );
}

export function CompactHeader({
  title,
  onBack,
  rightContent,
  className,
}: CompactHeaderProps) {
  return (
    <header
      className={cn(
        'flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4',
        className,
      )}
    >
      <div className="flex items-center gap-3">
        {onBack ? (
          <IconButton onClick={onBack} ariaLabel="Go back">
            <ArrowLeftIcon />
          </IconButton>
        ) : null}
        {title ? (
          <h1 className="text-lg font-semibold text-gray-900">{title}</h1>
        ) : null}
      </div>

      {rightContent ? (
        <div className="flex items-center gap-2">{rightContent}</div>
      ) : null}
    </header>
  );
}
