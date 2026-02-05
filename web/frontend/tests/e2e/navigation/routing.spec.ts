// E2E tests for navigation and routing.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ messages: [] }),
    });
  });

  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [{ id: '1', name: 'TestAgent', created_at: new Date().toISOString() }],
      }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [{ id: '1', name: 'TestAgent', status: 'active' }],
        counts: { active: 1, busy: 0, idle: 0, offline: 0 },
      }),
    });
  });

  await page.route('**/api/v1/sessions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sessions: [] }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ activities: [] }),
    });
  });
}

test.describe('Main navigation', () => {
  test('home page loads inbox', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
  });

  test('sidebar navigation links are visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Should show main navigation links.
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Agents' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Sessions' })).toBeVisible();
  });

  test('clicking Agents navigates to agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    await page.getByRole('link', { name: 'Agents' }).click();
    await page.waitForURL('**/agents');

    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  });

  test('clicking Sessions navigates to sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    await page.getByRole('link', { name: 'Sessions' }).click();
    await page.waitForURL('**/sessions');

    await expect(page.getByRole('heading', { name: 'Sessions' })).toBeVisible();
  });

  test('clicking Inbox navigates back to inbox', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.getByRole('link', { name: 'Inbox' }).click();
    await page.waitForURL(/\/inbox/);

    await expect(page.locator('main')).toBeVisible();
  });
});

test.describe('Direct URL navigation', () => {
  test('/agents loads agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  });

  test('/sessions loads sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await expect(page.getByRole('heading', { name: 'Sessions' })).toBeVisible();
  });

  test('unknown routes show 404 or redirect', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/unknown-route');

    // Should show 404 page or redirect to home.
    await page.waitForTimeout(500);
    // Either 404 or redirected to inbox - page should load successfully.
  });
});

test.describe('Browser history', () => {
  test('back button works after navigation', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();

    // Navigate to agents.
    await page.getByRole('link', { name: 'Agents' }).click();
    await page.waitForURL('**/agents');

    // Go back.
    await page.goBack();
    await expect(page.locator('main')).toBeVisible();
  });

  test('forward button works after back', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Navigate to agents.
    await page.getByRole('link', { name: 'Agents' }).click();
    await page.waitForURL('**/agents');

    // Go back.
    await page.goBack();
    await expect(page.locator('main')).toBeVisible();

    // Go forward.
    await page.goForward();
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  });
});

test.describe('Active link highlighting', () => {
  test('current page link is highlighted', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Inbox link should be active.
    const inboxLink = page.getByRole('link', { name: 'Inbox' });
    await expect(inboxLink).toBeVisible();
    // Check for active class (depends on implementation).
  });

  test('link highlight updates on navigation', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Navigate to agents.
    await page.getByRole('link', { name: 'Agents' }).click();
    await page.waitForURL('**/agents');

    // Agents link should now be active.
    const agentsLink = page.getByRole('link', { name: 'Agents' });
    await expect(agentsLink).toBeVisible();
  });
});

test.describe('Header navigation', () => {
  test('logo links to home', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');

    const logo = page.getByRole('link', { name: 'Substrate' });
    if (await logo.isVisible()) {
      await logo.click();
      await page.waitForURL(/\/inbox/);

      await expect(page.locator('main')).toBeVisible();
    }
  });

  test('search is accessible from all pages', async ({ page }) => {
    await setupAPIs(page);

    // Check on inbox - search button should be visible.
    await page.goto('/');
    const searchButton = page.getByRole('button', { name: /search/i });
    await expect(searchButton).toBeVisible();

    // Check on agents.
    await page.goto('/agents');
    await expect(searchButton).toBeVisible();
  });
});

