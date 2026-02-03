// React Router configuration for the application.

import { createBrowserRouter, Outlet, Navigate, useRouteError, isRouteErrorResponse } from 'react-router-dom';
import { Suspense, lazy } from 'react';
import { Spinner } from '@/components/ui/Spinner';
import { AppShell } from '@/components/layout/AppShell';
import { Button } from '@/components/ui/Button';

// Lazy load pages for code splitting.
const InboxPage = lazy(() => import('@/pages/InboxPage'));
const AgentsDashboard = lazy(() => import('@/pages/AgentsDashboard'));
const SessionsPage = lazy(() => import('@/pages/SessionsPage'));
const SettingsPage = lazy(() => import('@/pages/SettingsPage'));
const SearchResultsPage = lazy(() => import('@/pages/SearchResultsPage'));

// Page loading fallback.
function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <Spinner size="lg" variant="primary" label="Loading page..." />
    </div>
  );
}

// Route error boundary component.
function RouteErrorBoundary() {
  const error = useRouteError();

  // Handle 404 and other route errors.
  if (isRouteErrorResponse(error)) {
    if (error.status === 404) {
      return (
        <div className="flex h-full flex-col items-center justify-center p-8">
          <div className="text-center">
            <div className="mb-4 text-6xl">🔍</div>
            <h1 className="mb-2 text-2xl font-bold text-gray-900">Page Not Found</h1>
            <p className="mb-6 text-gray-600">
              The page you're looking for doesn't exist or has been moved.
            </p>
            <Button variant="primary" onClick={() => window.location.href = '/inbox'}>
              Go to Inbox
            </Button>
          </div>
        </div>
      );
    }

    return (
      <div className="flex h-full flex-col items-center justify-center p-8">
        <div className="text-center">
          <div className="mb-4 text-6xl">⚠️</div>
          <h1 className="mb-2 text-2xl font-bold text-gray-900">
            {error.status} {error.statusText}
          </h1>
          <p className="mb-6 text-gray-600">{error.data?.message || 'An error occurred.'}</p>
          <Button variant="primary" onClick={() => window.location.reload()}>
            Try Again
          </Button>
        </div>
      </div>
    );
  }

  // Handle other errors.
  return (
    <div className="flex h-full flex-col items-center justify-center p-8">
      <div className="max-w-md text-center">
        <div className="mb-4 text-6xl">💥</div>
        <h1 className="mb-2 text-2xl font-bold text-gray-900">Something went wrong</h1>
        <p className="mb-6 text-gray-600">
          An unexpected error occurred. Please try refreshing the page.
        </p>
        <div className="flex justify-center gap-4">
          <Button variant="primary" onClick={() => window.location.reload()}>
            Refresh Page
          </Button>
          <Button variant="secondary" onClick={() => window.location.href = '/inbox'}>
            Go to Inbox
          </Button>
        </div>
        {import.meta.env.DEV && error instanceof Error && (
          <details className="mt-8 text-left">
            <summary className="cursor-pointer text-sm font-medium text-gray-700">
              Error Details
            </summary>
            <pre className="mt-2 overflow-auto rounded bg-gray-100 p-4 text-xs text-red-600">
              {error.message}
              {error.stack && `\n\n${error.stack}`}
            </pre>
          </details>
        )}
      </div>
    </div>
  );
}

// Layout component that wraps all routes.
function RootLayout() {
  return (
    <AppShell>
      <Suspense fallback={<PageLoader />}>
        <Outlet />
      </Suspense>
    </AppShell>
  );
}

// Router configuration.
export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    errorElement: <RouteErrorBoundary />,
    children: [
      // Default redirect to inbox.
      {
        index: true,
        element: <Navigate to="/inbox" replace />,
      },
      // Inbox routes.
      {
        path: 'inbox',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'inbox/:category',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'starred',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'snoozed',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'sent',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'archive',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      // Thread view (direct and nested under inbox).
      {
        path: 'thread/:threadId',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'inbox/thread/:threadId',
        element: <InboxPage />,
        errorElement: <RouteErrorBoundary />,
      },
      // Agents routes.
      {
        path: 'agents',
        element: <AgentsDashboard />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'agents/:agentId',
        element: <AgentsDashboard />,
        errorElement: <RouteErrorBoundary />,
      },
      // Sessions routes.
      {
        path: 'sessions',
        element: <SessionsPage />,
        errorElement: <RouteErrorBoundary />,
      },
      {
        path: 'sessions/:sessionId',
        element: <SessionsPage />,
        errorElement: <RouteErrorBoundary />,
      },
      // Settings.
      {
        path: 'settings',
        element: <SettingsPage />,
        errorElement: <RouteErrorBoundary />,
      },
      // Search.
      {
        path: 'search',
        element: <SearchResultsPage />,
        errorElement: <RouteErrorBoundary />,
      },
      // Catch-all redirect.
      {
        path: '*',
        element: <Navigate to="/inbox" replace />,
      },
    ],
  },
]);

// Re-export routes for backward compatibility.
export { routes } from '@/lib/routes.js';
