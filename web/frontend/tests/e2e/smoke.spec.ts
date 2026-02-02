// E2E smoke tests to catch runtime JavaScript errors.
// These tests verify the production bundle loads without errors.

import { test, expect } from '@playwright/test';

// Collect console errors during tests.
test.beforeEach(async ({ page }) => {
  // Listen for console errors.
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      // Store error for later assertion.
      const errors = (page as unknown as { _consoleErrors?: string[] })._consoleErrors ?? [];
      errors.push(msg.text());
      (page as unknown as { _consoleErrors: string[] })._consoleErrors = errors;
    }
  });

  // Listen for page errors (uncaught exceptions).
  page.on('pageerror', (error) => {
    const errors = (page as unknown as { _pageErrors?: Error[] })._pageErrors ?? [];
    errors.push(error);
    (page as unknown as { _pageErrors: Error[] })._pageErrors = errors;
  });
});

// Helper to check for console errors.
function getConsoleErrors(page: unknown): string[] {
  return (page as { _consoleErrors?: string[] })._consoleErrors ?? [];
}

// Helper to check for page errors.
function getPageErrors(page: unknown): Error[] {
  return (page as { _pageErrors?: Error[] })._pageErrors ?? [];
}

test.describe('Application Smoke Tests', () => {
  test('app loads without JavaScript errors', async ({ page }) => {
    // Navigate to the app.
    await page.goto('/');

    // Wait for the app to be fully loaded.
    await page.waitForLoadState('networkidle');

    // Check for page errors (uncaught exceptions like "Cannot access before initialization").
    const pageErrors = getPageErrors(page);
    expect(pageErrors, 'Should not have any uncaught JavaScript errors').toHaveLength(0);

    // Check for critical console errors (exclude expected warnings).
    const consoleErrors = getConsoleErrors(page).filter((error) => {
      // Filter out known non-critical errors.
      return !error.includes('favicon.ico') && !error.includes('sourcemap');
    });

    if (consoleErrors.length > 0) {
      console.log('Console errors found:', consoleErrors);
    }

    // App should not have critical console errors.
    expect(consoleErrors, 'Should not have critical console errors').toHaveLength(0);
  });

  test('app renders main layout', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Verify the app renders the main layout.
    await expect(page.locator('#root')).not.toBeEmpty();

    // Check for the Subtrate logo/branding (in the sidebar).
    await expect(page.getByRole('link', { name: 'S Subtrate' })).toBeVisible();
  });

  test('navigation links are functional', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Check that sidebar navigation links exist and are clickable.
    const sidebar = page.getByRole('complementary');
    await expect(sidebar.getByRole('link', { name: /inbox/i })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /starred/i })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /agents/i })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /sessions/i })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /settings/i })).toBeVisible();
  });

  test('inbox page loads without errors', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Check for page errors.
    const pageErrors = getPageErrors(page);
    expect(pageErrors).toHaveLength(0);

    // Verify inbox-related content is rendered.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('agents page loads without errors', async ({ page }) => {
    await page.goto('/agents');
    await page.waitForLoadState('networkidle');

    // Check for page errors.
    const pageErrors = getPageErrors(page);
    expect(pageErrors).toHaveLength(0);

    // Verify agents-related content is rendered.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('settings page loads without errors', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // Check for page errors.
    const pageErrors = getPageErrors(page);
    expect(pageErrors).toHaveLength(0);

    // Verify settings page is rendered.
    await expect(page.locator('#root')).not.toBeEmpty();
  });
});

test.describe('Route Navigation', () => {
  test('redirects from / to /inbox', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // Should be redirected to inbox.
    await expect(page).toHaveURL(/\/inbox/);
  });

  test('handles unknown routes gracefully', async ({ page }) => {
    await page.goto('/unknown-route-that-does-not-exist');
    await page.waitForLoadState('networkidle');

    // Check for page errors (should not crash).
    const pageErrors = getPageErrors(page);
    expect(pageErrors).toHaveLength(0);

    // Should either show 404 or redirect.
    await expect(page.locator('#root')).not.toBeEmpty();
  });
});

test.describe('Error Boundary', () => {
  test('error boundary catches runtime errors', async ({ page }) => {
    // Navigate to app.
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // The app should be functional and not show error boundary initially.
    const errorBoundary = page.getByText(/something went wrong/i);

    // If error boundary is visible, the app has crashed.
    const isErrorBoundaryVisible = await errorBoundary.isVisible().catch(() => false);

    if (isErrorBoundaryVisible) {
      // This is a failure - app should not show error boundary on load.
      expect(isErrorBoundaryVisible, 'App should not show error boundary on initial load').toBe(false);
    }
  });
});
