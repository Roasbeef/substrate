// Header component - top navigation bar with search, agent switcher, and settings.

import { type ReactNode, useState } from 'react';
import { Link } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore } from '@/stores/ui.js';
import { useAuthStore } from '@/stores/auth.js';
import { useAgentsStatus } from '@/hooks/useAgents.js';
import { useMessages } from '@/hooks/useMessages.js';
import { ConnectedAgentSwitcher } from './AgentSwitcher.js';
import { routes } from '@/lib/routes.js';

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

// Globe icon for global/all agents view.
function GlobeIcon({ className }: { className?: string }) {
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
        d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
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


// ThemeToggle flips between light and dark, persisting the choice.
function ThemeToggle() {
  const [dark, setDark] = useState(
    () => document.documentElement.classList.contains('dark'),
  );

  const toggle = () => {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle('dark', next);
    localStorage.setItem('substrate-theme', next ? 'dark' : 'light');
  };

  return (
    <button
      type="button"
      onClick={toggle}
      title={dark ? 'Switch to light mode' : 'Switch to dark mode'}
      aria-label="Toggle dark mode"
      className="rounded-md p-2 hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)] focus:outline-none"
    >
      {dark ? (
        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24"
          stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round"
            d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
      ) : (
        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24"
          stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round"
            d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      )}
    </button>
  );
}

// Quiet paper search bar - centered.
function HeaderSearchBar() {
  const toggleSearch = useUIStore((state) => state.toggleSearch);

  return (
    <button
      type="button"
      onClick={toggleSearch}
      className={cn(
        'flex items-center gap-2 rounded-lg border border-[var(--c-hair)] bg-[var(--c-card)] px-4 py-1.5',
        'text-sm text-[var(--c-faint)] transition-colors hover:border-[var(--c-ghost)]',
        'focus:outline-none focus:ring-2 focus:ring-[#22262A]/20',
        'w-full max-w-xl',
      )}
    >
      <SearchIcon className="h-4 w-4 text-[var(--c-dim)]" />
      <span className="flex-1 text-left">
        Search agents, threads, plans…
      </span>
      <kbd className="hidden rounded border border-[var(--c-hair)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--c-faint)] md:inline-block">
        ⌘K
      </kbd>
    </button>
  );
}

// The wordmark: tracked mono caps with the heartbeat trace, the
// product's signature element carried into the chrome.
function Wordmark() {
  return (
    <span className="flex items-center gap-2.5">
      <span className="font-mono text-[13px] font-bold tracking-[0.18em] text-[var(--c-ink)]">
        SUBSTRATE
      </span>
      <svg className="h-3 w-11" viewBox="0 0 64 16" aria-hidden="true">
        <path
          d="M0 8 H14 L18 8 L21 3 L25 13 L28 8 H40 L44 8 L47 5 L50 11 L52 8 H64"
          fill="none" stroke="#178A5B" strokeWidth="1.5"
          strokeLinecap="round" strokeLinejoin="round"
        />
      </svg>
    </span>
  );
}

// Main Header component.
export function Header({ className, leftContent, rightContent }: HeaderProps) {
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);
  const toggleSearch = useUIStore((state) => state.toggleSearch);
  const { currentAgent, setCurrentAgent } = useAuthStore();

  // Fetch agents and messages for agent switcher.
  const { data: agentsData, isLoading: agentsLoading } = useAgentsStatus();
  const { data: messagesData } = useMessages({ filter: 'unread' });

  // Calculate total unread count.
  const totalUnreadCount = messagesData?.data?.length ?? 0;

  // Check if Global (all agents) is currently selected.
  const isGlobalSelected = currentAgent === null;

  return (
    <header
      className={cn(
        'flex h-12 items-center border-b border-[var(--c-hair)] bg-[var(--c-paper)] px-4',
        className,
      )}
    >
      {/* Left section - branding. */}
      <div className="flex items-center gap-4 flex-shrink-0">
        <button
          type="button"
          onClick={toggleSidebar}
          className="rounded-md p-2 text-[var(--c-mut)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)] focus:outline-none"
          aria-label="Toggle sidebar"
        >
          <MenuIcon />
        </button>

        {/* Wordmark. */}
        <Link to={routes.command} className="flex items-center">
          <Wordmark />
        </Link>

        {leftContent}
      </div>

      {/* Center section - search bar (takes remaining space). */}
      <div className="flex-1 flex justify-center px-4 hidden md:flex">
        <HeaderSearchBar />
      </div>

      {/* Right section - actions and custom content. */}
      <div className="flex items-center gap-1 flex-shrink-0 text-[var(--c-mut)]">
        {/* Mobile search button. */}
        <div className="md:hidden">
          <button
            type="button"
            onClick={toggleSearch}
            className="rounded-md p-2 hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)] focus:outline-none"
            aria-label="Search"
          >
            <SearchIcon />
          </button>
        </div>

        {/* Global button - shows all messages from all agents. */}
        <button
          type="button"
          onClick={() => setCurrentAgent(null)}
          className={cn(
            'rounded-md p-2 transition-colors focus:outline-none',
            isGlobalSelected
              ? 'bg-[var(--c-ink)] text-white'
              : 'hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]',
          )}
          aria-label="View all agents"
          title="Global - View all agents"
        >
          <GlobeIcon />
        </button>

        {/* Agent switcher with unread count. */}
        {agentsData?.agents ? (
          <ConnectedAgentSwitcher
            agents={agentsData.agents}
            isLoading={agentsLoading}
            totalUnreadCount={totalUnreadCount}
          />
        ) : null}

        {/* Theme toggle. */}
        <ThemeToggle />

        {/* Notifications button. */}
        <button
          type="button"
          className="relative rounded-md p-2 hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)] focus:outline-none"
          aria-label="View notifications"
        >
          <BellIcon />
          {totalUnreadCount > 0 ? (
            <span className="absolute right-1.5 top-1.5 block h-2 w-2 rounded-full bg-[var(--c-rust)] ring-2 ring-[var(--c-paper)]" />
          ) : null}
        </button>

        {/* Settings link. */}
        <Link
          to={routes.settings}
          className="rounded-md p-2 hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)] focus:outline-none"
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
        'flex h-14 items-center justify-between border-b border-gray-200 bg-[var(--c-card)] px-4',
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
