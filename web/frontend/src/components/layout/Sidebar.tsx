// Sidebar component - main navigation sidebar with nav links and actions.

import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore, type SidebarSection } from '@/stores/ui.js';
import { Button } from '@/components/ui/Button.js';
import { routes } from '@/router.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Navigation items for sidebar.
interface NavItem {
  id: SidebarSection;
  label: string;
  path: string;
  icon: ReactNode;
  badge?: number;
}

// Icon components for navigation.
function InboxIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
      />
    </svg>
  );
}

function StarIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"
      />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
      />
    </svg>
  );
}

function ArchiveIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"
      />
    </svg>
  );
}

function UsersIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
      />
    </svg>
  );
}

function TerminalIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
      />
    </svg>
  );
}

function SettingsIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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

function PlusIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
    </svg>
  );
}

// Default navigation items.
const navItems: NavItem[] = [
  { id: 'inbox', label: 'Inbox', path: routes.inbox, icon: <InboxIcon /> },
  { id: 'starred', label: 'Starred', path: routes.starred, icon: <StarIcon /> },
  { id: 'snoozed', label: 'Snoozed', path: routes.snoozed, icon: <ClockIcon /> },
  { id: 'sent', label: 'Sent', path: routes.sent, icon: <SendIcon /> },
  { id: 'archive', label: 'Archive', path: routes.archive, icon: <ArchiveIcon /> },
  { id: 'agents', label: 'Agents', path: routes.agents, icon: <UsersIcon /> },
  { id: 'sessions', label: 'Sessions', path: routes.sessions, icon: <TerminalIcon /> },
];

// Logo component.
export interface LogoProps {
  collapsed?: boolean;
}

export function Logo({ collapsed = false }: LogoProps) {
  return (
    <Link to={routes.inbox} className="flex items-center gap-2 px-4 py-3">
      <div className="h-8 w-8 flex-shrink-0 rounded-lg bg-blue-600 flex items-center justify-center">
        <span className="text-white font-bold text-lg">S</span>
      </div>
      {!collapsed ? (
        <span className="text-xl font-semibold text-gray-900">Subtrate</span>
      ) : null}
    </Link>
  );
}

// Sidebar navigation link.
interface NavLinkProps {
  item: NavItem;
  isActive: boolean;
  collapsed?: boolean;
}

function NavLink({ item, isActive, collapsed = false }: NavLinkProps) {
  return (
    <Link
      to={item.path}
      className={cn(
        'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
        isActive
          ? 'bg-blue-50 text-blue-700'
          : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900',
        collapsed ? 'justify-center' : '',
      )}
      title={collapsed ? item.label : undefined}
    >
      <span className={cn(isActive ? 'text-blue-600' : 'text-gray-400')}>
        {item.icon}
      </span>
      {!collapsed ? (
        <>
          <span className="flex-1">{item.label}</span>
          {item.badge && item.badge > 0 ? (
            <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">
              {item.badge}
            </span>
          ) : null}
        </>
      ) : null}
    </Link>
  );
}

// Sidebar props.
export interface SidebarProps {
  /** Custom navigation items (overrides defaults). */
  navItems?: NavItem[];
  /** Whether to show the compose button. */
  showComposeButton?: boolean;
  /** Whether to show the settings link. */
  showSettings?: boolean;
  /** Additional class name. */
  className?: string;
  /** Custom footer content. */
  footer?: ReactNode;
}

// Determine active section from current path.
function useActiveSection(): SidebarSection {
  const location = useLocation();
  const path = location.pathname;

  if (path.startsWith('/agents')) return 'agents';
  if (path.startsWith('/sessions')) return 'sessions';
  if (path.startsWith('/starred')) return 'starred';
  if (path.startsWith('/snoozed')) return 'snoozed';
  if (path.startsWith('/sent')) return 'sent';
  if (path.startsWith('/archive')) return 'archive';
  return 'inbox';
}

// Main Sidebar component.
export function Sidebar({
  navItems: customNavItems,
  showComposeButton = true,
  showSettings = true,
  className,
  footer,
}: SidebarProps) {
  const openModal = useUIStore((state) => state.openModal);
  const sidebarCollapsed = useUIStore((state) => state.sidebarCollapsed);
  const activeSection = useActiveSection();

  const items = customNavItems ?? navItems;

  if (sidebarCollapsed) {
    return null;
  }

  return (
    <aside
      className={cn(
        'flex h-full w-64 flex-col border-r border-gray-200 bg-white',
        className,
      )}
    >
      <Logo />

      {showComposeButton ? (
        <div className="px-3 py-2">
          <Button
            className="w-full"
            onClick={() => openModal('compose')}
            leftIcon={<PlusIcon />}
          >
            Compose
          </Button>
        </div>
      ) : null}

      <nav className="flex-1 space-y-1 px-3 py-2">
        {items.map((item) => (
          <NavLink
            key={item.id}
            item={item}
            isActive={activeSection === item.id}
          />
        ))}
      </nav>

      {footer}

      {showSettings ? (
        <div className="border-t border-gray-200 p-3">
          <Link
            to={routes.settings}
            className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
          >
            <span className="text-gray-400">
              <SettingsIcon />
            </span>
            <span>Settings</span>
          </Link>
        </div>
      ) : null}
    </aside>
  );
}

// Collapsed sidebar variant.
export function CollapsedSidebar({ className }: { className?: string }) {
  const activeSection = useActiveSection();
  const openModal = useUIStore((state) => state.openModal);

  return (
    <aside
      className={cn(
        'flex h-full w-16 flex-col border-r border-gray-200 bg-white',
        className,
      )}
    >
      <Logo collapsed />

      <div className="px-2 py-2">
        <button
          type="button"
          onClick={() => openModal('compose')}
          className={cn(
            'flex h-10 w-10 items-center justify-center rounded-lg',
            'bg-blue-600 text-white hover:bg-blue-700',
            'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
          )}
          aria-label="Compose"
        >
          <PlusIcon />
        </button>
      </div>

      <nav className="flex-1 space-y-1 px-2 py-2">
        {navItems.map((item) => (
          <NavLink
            key={item.id}
            item={item}
            isActive={activeSection === item.id}
            collapsed
          />
        ))}
      </nav>

      <div className="border-t border-gray-200 p-2">
        <Link
          to={routes.settings}
          className={cn(
            'flex h-10 w-10 items-center justify-center rounded-lg',
            'text-gray-400 hover:bg-gray-100 hover:text-gray-500',
          )}
          aria-label="Settings"
        >
          <SettingsIcon />
        </Link>
      </div>
    </aside>
  );
}
