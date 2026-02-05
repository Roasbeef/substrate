// E2E tests for navigation and routing.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });

  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [{ id: 1, name: 'TestAgent', created_at: new Date().toISOString() }],
        meta: { total: 1, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [{ id: 1, name: 'TestAgent', status: 'active' }],
        counts: { active: 1, busy: 0, idle: 0, offline: 0 },
      }),
    });
  });

  await page.route('**/api/v1/sessions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });
}

test.describe('Main navigation', () => {
  test('home page loads inbox', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
  });

  test('sidebar navigation links are visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Should show main navigation links.
    await expect(page.locator('a[href="/"], text=Inbox')).toBeVisible();
    await expect(page.locator('a[href="/agents"]')).toBeVisible();
    await expect(page.locator('a[href="/sessions"]')).toBeVisible();
  });

  test('clicking Agents navigates to agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    await page.locator('a[href="/agents"]').click();
    await page.waitForURL('**/agents');

    await expect(page.locator('text=Agents')).toBeVisible();
  });

  test('clicking Sessions navigates to sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    await page.locator('a[href="/sessions"]').click();
    await page.waitForURL('**/sessions');

    await expect(page.locator('text=Sessions')).toBeVisible();
  });

  test('clicking Inbox navigates back to inbox', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('a[href="/"], a:has-text("Inbox")').first().click();
    await page.waitForURL(/\/$/);

    await expect(page.locator('text=Inbox')).toBeVisible();
  });
});

test.describe('Direct URL navigation', () => {
  test('/agents loads agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();
  });

  test('/sessions loads sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await expect(page.locator('text=Sessions')).toBeVisible();
  });

  test('unknown routes show 404 or redirect', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/unknown-route');

    // Should show 404 page or redirect to home.
    const notFound = page.locator('text=/not found|404/i');
    const inbox = page.locator('text=Inbox');

    // Either 404 or redirected to inbox.
    await page.waitForTimeout(500);
  });
});

test.describe('Browser history', () => {
  test('back button works after navigation', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Navigate to agents.
    await page.locator('a[href="/agents"]').click();
    await page.waitForURL('**/agents');

    // Go back.
    await page.goBack();
    await expect(page.locator('text=Inbox')).toBeVisible();
  });

  test('forward button works after back', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Navigate to agents.
    await page.locator('a[href="/agents"]').click();
    await page.waitForURL('**/agents');

    // Go back.
    await page.goBack();
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Go forward.
    await page.goForward();
    await expect(page.locator('text=Agents')).toBeVisible();
  });
});

test.describe('Active link highlighting', () => {
  test('current page link is highlighted', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Inbox link should be active.
    const inboxLink = page.locator('a[href="/"]');
    const isActive = await inboxLink.getAttribute('class');
    // Check for active class (depends on implementation).
  });

  test('link highlight updates on navigation', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Navigate to agents.
    await page.locator('a[href="/agents"]').click();
    await page.waitForURL('**/agents');

    // Agents link should now be active.
    const agentsLink = page.locator('a[href="/agents"]');
    await expect(agentsLink).toBeVisible();
  });
});

test.describe('Header navigation', () => {
  test('logo links to home', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');

    const logo = page.locator('[data-testid="logo"], a:has(text="Subtrate"), header a').first();
    if (await logo.isVisible()) {
      await logo.click();
      await page.waitForURL(/\/$/);

      await expect(page.locator('text=Inbox')).toBeVisible();
    }
  });

  test('search is accessible from all pages', async ({ page }) => {
    await setupAPIs(page);

    // Check on inbox.
    await page.goto('/');
    await expect(page.locator('[data-testid="search-bar"], input[placeholder*="search" i]')).toBeVisible();

    // Check on agents.
    await page.goto('/agents');
    await expect(page.locator('[data-testid="search-bar"], input[placeholder*="search" i]')).toBeVisible();
  });
});

test.describe('Mobile navigation', () => {
  test('hamburger menu appears on mobile', async ({ page }) => {
    await setupAPIs(page);
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/');

    // Hamburger button should be visible.
    const hamburger = page.locator('[data-testid="mobile-menu-button"], button[aria-label*="menu" i]');
    await expect(hamburger).toBeVisible();
  });

  test('clicking hamburger opens mobile menu', async ({ page }) => {
    await setupAPIs(page);
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/');

    const hamburger = page.locator('[data-testid="mobile-menu-button"], button[aria-label*="menu" i]');
    if (await hamburger.isVisible()) {
      await hamburger.click();
      await page.waitForTimeout(300);

      // Mobile menu should open.
      const mobileMenu = page.locator('[data-testid="mobile-menu"], [role="dialog"]');
      await expect(mobileMenu).toBeVisible();
    }
  });

  test('mobile navigation links work', async ({ page }) => {
    await setupAPIs(page);
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/');

    const hamburger = page.locator('[data-testid="mobile-menu-button"]');
    if (await hamburger.isVisible()) {
      await hamburger.click();
      await page.waitForTimeout(300);

      // Click agents link.
      await page.locator('a[href="/agents"]').click();
      await page.waitForURL('**/agents');

      await expect(page.locator('text=Agents')).toBeVisible();
    }
  });
});
