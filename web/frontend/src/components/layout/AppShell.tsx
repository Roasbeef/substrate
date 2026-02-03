// AppShell component - the main layout wrapper for the application.

import { type ReactNode, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Header } from './Header.js';
import { Sidebar } from './Sidebar.js';
import { NotificationPrompt } from './NotificationPrompt.js';
import { ModalContainer } from './ModalContainer.js';
import { useNewMessageNotifications } from '@/hooks/useNotifications.js';
import { useMessageToasts } from '@/hooks/useMessageToasts.js';
import { useUIStore } from '@/stores/ui.js';

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
  const navigate = useNavigate();
  const setPendingThread = useUIStore((state) => state.setPendingThread);

  // Handle notification thread click - navigate to inbox and open thread.
  const handleThreadClick = useCallback(
    (threadId: string) => {
      setPendingThread(threadId);
      navigate('/');
    },
    [setPendingThread, navigate],
  );

  // Enable automatic notifications for new messages.
  // Browser notifications (desktop).
  useNewMessageNotifications({ onThreadClick: handleThreadClick });
  // In-app toast notifications.
  useMessageToasts();

  return (
    <div className="flex h-screen w-full flex-col overflow-hidden">
      {/* Header spans full width at top. */}
      {!hideHeader ? <Header rightContent={headerContent} /> : null}

      {/* Content area with sidebar and main content. */}
      <div className="flex flex-1 overflow-hidden">
        {!hideSidebar ? <Sidebar footer={sidebarFooter} /> : null}
        <MainContent {...(mainClassName && { className: mainClassName })}>{children}</MainContent>
      </div>

      {/* Notification permission prompt. */}
      <NotificationPrompt />
      {/* Global modal container. */}
      <ModalContainer />
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
