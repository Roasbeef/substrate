// Error boundary component for catching and displaying React errors gracefully.

import { Component, type ReactNode } from 'react';
import { Button } from './Button';

interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: ReactNode;
  onReset?: () => void;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: React.ErrorInfo | null;
}

// ErrorBoundary catches JavaScript errors in child component tree and displays fallback UI.
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo): void {
    this.setState({ errorInfo });
    // Log error to console for development.
    console.error('ErrorBoundary caught an error:', error, errorInfo);
  }

  handleReset = (): void => {
    this.setState({ hasError: false, error: null, errorInfo: null });
    this.props.onReset?.();
  };

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <ErrorFallback
          error={this.state.error}
          errorInfo={this.state.errorInfo}
          onReset={this.handleReset}
        />
      );
    }

    return this.props.children;
  }
}

interface ErrorFallbackProps {
  error: Error | null;
  errorInfo: React.ErrorInfo | null;
  onReset: () => void;
}

// ErrorFallback displays a user-friendly error message with recovery options.
function ErrorFallback({ error, errorInfo, onReset }: ErrorFallbackProps): ReactNode {
  const isDev = import.meta.env.DEV;

  return (
    <div className="flex min-h-[400px] flex-col items-center justify-center p-8">
      <div className="max-w-md text-center">
        <div className="mb-4 text-6xl">⚠️</div>
        <h2 className="mb-2 text-xl font-semibold text-gray-900">
          Something went wrong
        </h2>
        <p className="mb-6 text-gray-600">
          An unexpected error occurred. You can try refreshing the page or going back.
        </p>

        <div className="flex justify-center gap-4">
          <Button variant="primary" onClick={onReset}>
            Try Again
          </Button>
          <Button variant="secondary" onClick={() => window.location.reload()}>
            Refresh Page
          </Button>
        </div>

        {isDev && error && (
          <details className="mt-8 text-left">
            <summary className="cursor-pointer text-sm font-medium text-gray-700">
              Error Details (Development Only)
            </summary>
            <div className="mt-2 overflow-auto rounded bg-gray-100 p-4">
              <p className="mb-2 font-mono text-sm text-red-600">
                {error.name}: {error.message}
              </p>
              {errorInfo?.componentStack && (
                <pre className="whitespace-pre-wrap font-mono text-xs text-gray-600">
                  {errorInfo.componentStack}
                </pre>
              )}
            </div>
          </details>
        )}
      </div>
    </div>
  );
}

interface PageErrorBoundaryProps {
  children: ReactNode;
}

// PageErrorBoundary wraps individual pages to isolate errors.
export function PageErrorBoundary({ children }: PageErrorBoundaryProps): ReactNode {
  return (
    <ErrorBoundary
      fallback={
        <div className="flex h-full items-center justify-center p-8">
          <div className="text-center">
            <div className="mb-4 text-4xl">📄</div>
            <h3 className="mb-2 text-lg font-medium text-gray-900">
              This page encountered an error
            </h3>
            <p className="mb-4 text-sm text-gray-600">
              Try navigating to a different page or refreshing.
            </p>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => window.location.reload()}
            >
              Refresh
            </Button>
          </div>
        </div>
      }
    >
      {children}
    </ErrorBoundary>
  );
}

export default ErrorBoundary;
