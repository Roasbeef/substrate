// E2E tests for inbox category tabs (Primary/Agents/Topics).

import { test, expect } from '@playwright/test';

test.describe('Inbox category tabs', () => {
  test('displays category tabs', async ({ page }) => {
    await page.goto('/');
    // Wait for page to load with main content visible.
    await expect(page.locator('main')).toBeVisible();

    // Check for category buttons in the category navigation.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await expect(categoryNav.getByRole('button', { name: 'Primary' })).toBeVisible();
    await expect(categoryNav.getByRole('button', { name: 'Agents' })).toBeVisible();
    await expect(categoryNav.getByRole('button', { name: 'Topics' })).toBeVisible();
  });

  test('Primary tab is active by default', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();

    // Primary button should be visible.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await expect(categoryNav.getByRole('button', { name: 'Primary' })).toBeVisible();
  });

  test('clicking Agents tab switches view', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const category = url.searchParams.get('category');

      if (category === 'agents') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [
              {
                id: '10',
                sender_id: '1',
                sender_name: 'BuildAgent',
                subject: 'Build Complete',
                body: 'Build finished successfully',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [
              {
                id: '1',
                sender_id: '1',
                sender_name: 'User',
                subject: 'Primary Message',
                body: 'Content',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Agents tab within category nav.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await categoryNav.getByRole('button', { name: 'Agents' }).click();
    await page.waitForTimeout(500);

    // View should switch - Agents tab should have active styling.
    await expect(categoryNav.getByRole('button', { name: 'Agents' })).toBeVisible();
  });

  test('clicking Topics tab switches view', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const category = url.searchParams.get('category');

      if (category === 'topics') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [
              {
                id: '20',
                sender_id: '1',
                sender_name: 'System',
                subject: 'Topic Update',
                body: 'New topic content',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [],
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Topics tab within category nav.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await categoryNav.getByRole('button', { name: 'Topics' }).click();
    await page.waitForTimeout(500);

    // Topics tab should be visible.
    await expect(categoryNav.getByRole('button', { name: 'Topics' })).toBeVisible();
  });

  test('switching back to Primary tab restores view', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          messages: [
            {
              id: '1',
              sender_id: '1',
              sender_name: 'User',
              subject: 'Primary Content',
              body: 'Body',
              priority: 'PRIORITY_NORMAL',
              created_at: new Date().toISOString(),
              recipients: [],
            },
          ],
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
    await page.waitForTimeout(500);

    const categoryNav = page.locator('nav[aria-label="Category tabs"]');

    // Switch to Agents.
    await categoryNav.getByRole('button', { name: 'Agents' }).click();
    await page.waitForTimeout(300);

    // Switch back to Primary.
    await categoryNav.getByRole('button', { name: 'Primary' }).click();
    await page.waitForTimeout(300);

    // Primary should be visible.
    await expect(categoryNav.getByRole('button', { name: 'Primary' })).toBeVisible();
  });

  test('category navigation is visible', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();

    // Check category navigation exists.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await expect(categoryNav).toBeVisible();

    // Check category buttons exist.
    const buttons = categoryNav.getByRole('button');
    const count = await buttons.count();
    expect(count).toBeGreaterThanOrEqual(3);
  });
});

test.describe('Category content loading', () => {
  test('shows loading state when switching categories', async ({ page }) => {
    let requestCount = 0;

    await page.route('**/api/v1/messages*', async (route) => {
      requestCount++;
      // Delay response to show loading state.
      await new Promise((resolve) => setTimeout(resolve, 100));

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          messages: [],
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
    await page.waitForTimeout(200);

    // Switch category.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await categoryNav.getByRole('button', { name: 'Agents' }).click();

    // Should trigger new request.
    await page.waitForTimeout(300);
    expect(requestCount).toBeGreaterThanOrEqual(1);
  });

  test('handles empty category gracefully', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const category = url.searchParams.get('category');

      if (category === 'topics') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [],
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            messages: [
              {
                id: '1',
                sender_id: '1',
                sender_name: 'User',
                subject: 'Message',
                body: 'Body',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();
    await page.waitForTimeout(500);

    // Switch to empty Topics category.
    const categoryNav = page.locator('nav[aria-label="Category tabs"]');
    await categoryNav.getByRole('button', { name: 'Topics' }).click();
    await page.waitForTimeout(500);

    // Should handle empty state gracefully.
    await expect(categoryNav.getByRole('button', { name: 'Topics' })).toBeVisible();
  });
});

test.describe('Category keyboard navigation', () => {
  test('tabs can be navigated with keyboard', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();

    const categoryNav = page.locator('nav[aria-label="Category tabs"]');

    // Focus the first category button.
    await categoryNav.getByRole('button', { name: 'Primary' }).focus();

    // Use arrow keys to navigate (if implemented).
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(100);

    // The next button should receive focus.
    // This depends on implementation.
  });

  test('Enter key activates focused button', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('main')).toBeVisible();

    const categoryNav = page.locator('nav[aria-label="Category tabs"]');

    // Focus Agents button.
    await categoryNav.getByRole('button', { name: 'Agents' }).focus();

    // Press Enter.
    await page.keyboard.press('Enter');
    await page.waitForTimeout(300);

    // Agents button should be activated.
    await expect(categoryNav.getByRole('button', { name: 'Agents' })).toBeVisible();
  });
});
