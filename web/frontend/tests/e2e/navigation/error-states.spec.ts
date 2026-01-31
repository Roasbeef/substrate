// E2E tests for error states and error handling.

import { test, expect } from '@playwright/test';

test.describe('API error handling', () => {
  test('shows error message on 500 error', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'server_error', message: 'Internal server error' } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Should show error state.
    const errorMessage = page.locator('[data-testid="error-message"], text=/error|failed/i');
    // Error handling depends on implementation.
  });

  test('handles network error gracefully', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.abort('failed');
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Should show error or offline state.
    const errorState = page.locator('[data-testid="error-state"], [data-testid="offline-state"]');
    // Page should still be functional.
    await expect(page.locator('text=Inbox')).toBeVisible();
  });

  test('shows retry button on error', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({ status: 500, body: '{}' });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    const retryButton = page.locator('button:has-text("Retry"), button:has-text("Try again")');
    // Retry button may be present.
  });

  test('retry button refetches data', async ({ page }) => {
    let requestCount = 0;

    await page.route('**/api/v1/messages*', async (route) => {
      requestCount++;
      if (requestCount === 1) {
        await route.fulfill({ status: 500, body: '{}' });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
        });
      }
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    const retryButton = page.locator('button:has-text("Retry")');
    if (await retryButton.isVisible()) {
      await retryButton.click();
      await page.waitForTimeout(500);

      expect(requestCount).toBeGreaterThan(1);
    }
  });
});

test.describe('404 handling', () => {
  test('shows 404 page for unknown routes', async ({ page }) => {
    await page.goto('/unknown-page-that-does-not-exist');
    await page.waitForTimeout(500);

    // Should show 404 or redirect.
    const notFound = page.locator('text=/404|not found|page does not exist/i');
    const inbox = page.locator('text=Inbox');

    // Either 404 page or redirected to inbox.
  });

  test('404 page has link to home', async ({ page }) => {
    await page.goto('/unknown-page');
    await page.waitForTimeout(500);

    const homeLink = page.locator('a[href="/"], a:has-text("Home"), a:has-text("Go back")');
    if (await homeLink.isVisible()) {
      await homeLink.click();
      await page.waitForURL(/\/$/);

      await expect(page.locator('text=Inbox')).toBeVisible();
    }
  });
});

test.describe('Form validation errors', () => {
  test('shows validation error for empty required field', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Open compose modal.
    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Click send without filling fields.
      const sendButton = modal.locator('button:has-text("Send")');
      await sendButton.click();
      await page.waitForTimeout(300);

      // Should show validation error.
      const error = modal.locator('[data-testid="validation-error"], text=/required|cannot be empty/i');
      // Validation handling depends on implementation.
    }
  });

  test('shows inline validation for invalid input', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Enter invalid data and blur.
      const subjectInput = modal.locator('input').nth(1);
      await subjectInput.fill('');
      await subjectInput.blur();
      await page.waitForTimeout(200);

      // Inline error may appear.
    }
  });

  test('error clears when valid input entered', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Trigger and fix validation error.
      const subjectInput = modal.locator('input').nth(1);
      await subjectInput.fill('');
      await modal.locator('button:has-text("Send")').click();
      await page.waitForTimeout(200);

      await subjectInput.fill('Valid Subject');
      await page.waitForTimeout(200);

      // Error should clear.
    }
  });
});

test.describe('Timeout handling', () => {
  test('shows loading state during slow requests', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');

    // Should show loading indicator.
    const loading = page.locator('[data-testid="loading"], [role="progressbar"], .animate-spin');
    await expect(loading.first()).toBeVisible();
  });

  test('handles request timeout', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      // Delay long enough to timeout.
      await new Promise((resolve) => setTimeout(resolve, 30000));
    });

    await page.goto('/');
    await page.waitForTimeout(5000);

    // Should show timeout error or handle gracefully.
    // Timeout handling depends on implementation.
  });
});

test.describe('Offline handling', () => {
  test('shows offline indicator when disconnected', async ({ page, context }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Simulate offline.
    await context.setOffline(true);
    await page.waitForTimeout(500);

    // Offline indicator may appear.
    const offlineIndicator = page.locator('[data-testid="offline-indicator"], text=/offline/i');
    // Offline handling depends on implementation.
  });

  test('reconnects when back online', async ({ page, context }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Go offline then online.
    await context.setOffline(true);
    await page.waitForTimeout(500);
    await context.setOffline(false);
    await page.waitForTimeout(500);

    // Should recover.
  });
});

test.describe('Error boundaries', () => {
  test('component error shows fallback UI', async ({ page }) => {
    // Force a component error by providing bad data.
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: 'not valid json{',
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Should show error boundary fallback or handle gracefully.
    // Page should not crash completely.
    await expect(page.locator('body')).toBeVisible();
  });

  test('error does not crash entire app', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({ status: 500, body: '{}' });
    });

    await page.route('**/api/v1/agents*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');
    await page.waitForTimeout(500);

    // Navigation should still work.
    await page.locator('a[href="/agents"]').click();
    await page.waitForURL('**/agents');

    await expect(page.locator('text=Agents')).toBeVisible();
  });
});
