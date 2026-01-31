// AppShell component - the main layout wrapper for the application.

import { type ReactNode } from 'react';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Header } from './Header.js';
import { Sidebar } from './Sidebar.js';
import { NotificationPrompt } from './NotificationPrompt.js';
import { useNewMessageNotifications } from '@/hooks/useNotifications.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

export interface AppShellProps {
  /** Main content of the application. */
  children: ReactNode;
  /** Custom header content (renders in addition to default header). */
  headerContent?: ReactNode;
  /** Custom sidebar footer content. */
  sidebarFooter?: ReactNode;
  /** Hide the sidebar. */
  hideSidebar?: boolean;
  /** Hide the header. */
  hideHeader?: boolean;
  /** Additional class name for the main content area. */
  mainClassName?: string;
}

// Main content area wrapper.
interface MainContentProps {
  children: ReactNode;
  className?: string;
}

function MainContent({ children, className }: MainContentProps) {
  return (
    <main className={cn('flex-1 overflow-auto bg-gray-50', className)}>
      {children}
    </main>
  );
}

// AppShell combines all layout components.
export function AppShell({
  children,
  headerContent,
  sidebarFooter,
  hideSidebar = false,
  hideHeader = false,
  mainClassName,
}: AppShellProps) {
  // Enable automatic notifications for new messages.
  useNewMessageNotifications();

  return (
    <div className="flex h-screen w-full overflow-hidden">
      {!hideSidebar ? <Sidebar footer={sidebarFooter} /> : null}
      <div className="flex flex-1 flex-col overflow-hidden">
        {!hideHeader ? <Header rightContent={headerContent} /> : null}
        <MainContent className={mainClassName}>{children}</MainContent>
      </div>
      {/* Notification permission prompt. */}
      <NotificationPrompt />
    </div>
  );
}

// Minimal layout for auth pages or other fullscreen content.
export interface MinimalLayoutProps {
  children: ReactNode;
  className?: string;
}

export function MinimalLayout({ children, className }: MinimalLayoutProps) {
  return (
    <div className={cn('min-h-screen bg-gray-50', className)}>
      {children}
    </div>
  );
}
