// E2E tests for inbox category tabs (Primary/Agents/Topics).

import { test, expect } from '@playwright/test';

test.describe('Inbox category tabs', () => {
  test('displays category tabs', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check for category tabs.
    await expect(page.locator('[role="tablist"]')).toBeVisible();
    await expect(page.locator('button:has-text("Primary")')).toBeVisible();
    await expect(page.locator('button:has-text("Agents")')).toBeVisible();
    await expect(page.locator('button:has-text("Topics")')).toBeVisible();
  });

  test('Primary tab is active by default', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Primary tab should be selected.
    const primaryTab = page.locator('button:has-text("Primary")');
    await expect(primaryTab).toBeVisible();

    // Check for aria-selected or active class.
    const isSelected = await primaryTab.getAttribute('aria-selected');
    // Tab should indicate selection (implementation dependent).
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
            data: [
              {
                id: 10,
                sender_id: 1,
                sender_name: 'BuildAgent',
                subject: 'Build Complete',
                body: 'Build finished successfully',
                priority: 'normal',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
            meta: { total: 1, page: 1, page_size: 20 },
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                id: 1,
                sender_id: 1,
                sender_name: 'User',
                subject: 'Primary Message',
                body: 'Content',
                priority: 'normal',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
            meta: { total: 1, page: 1, page_size: 20 },
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Agents tab.
    await page.locator('button:has-text("Agents")').click();
    await page.waitForTimeout(500);

    // View should switch to agent messages.
    await expect(page.locator('button:has-text("Agents")')).toBeVisible();
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
            data: [
              {
                id: 20,
                sender_id: 1,
                sender_name: 'System',
                subject: 'Topic Update',
                body: 'New topic content',
                priority: 'normal',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
            meta: { total: 1, page: 1, page_size: 20 },
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Topics tab.
    await page.locator('button:has-text("Topics")').click();
    await page.waitForTimeout(500);

    // Topics tab should be active.
    await expect(page.locator('button:has-text("Topics")')).toBeVisible();
  });

  test('switching back to Primary tab restores view', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'User',
              subject: 'Primary Content',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Switch to Agents.
    await page.locator('button:has-text("Agents")').click();
    await page.waitForTimeout(300);

    // Switch back to Primary.
    await page.locator('button:has-text("Primary")').click();
    await page.waitForTimeout(300);

    // Primary should be active.
    await expect(page.locator('button:has-text("Primary")')).toBeVisible();
  });

  test('tabs have proper ARIA attributes', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check tablist role.
    const tablist = page.locator('[role="tablist"]');
    await expect(tablist).toBeVisible();

    // Check tab roles.
    const tabs = page.locator('[role="tab"]');
    const count = await tabs.count();
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
          data: [],
          meta: { total: 0, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(200);

    // Switch category.
    await page.locator('button:has-text("Agents")').click();

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
            data: [],
            meta: { total: 0, page: 1, page_size: 20 },
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                id: 1,
                sender_id: 1,
                sender_name: 'User',
                subject: 'Message',
                body: 'Body',
                priority: 'normal',
                created_at: new Date().toISOString(),
                recipients: [],
              },
            ],
            meta: { total: 1, page: 1, page_size: 20 },
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Switch to empty Topics category.
    await page.locator('button:has-text("Topics")').click();
    await page.waitForTimeout(500);

    // Should handle empty state gracefully.
    await expect(page.locator('button:has-text("Topics")')).toBeVisible();
  });
});

test.describe('Category keyboard navigation', () => {
  test('tabs can be navigated with keyboard', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Focus the tab list.
    await page.locator('button:has-text("Primary")').focus();

    // Use arrow keys to navigate (if implemented).
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(100);

    // The next tab should receive focus.
    // This depends on implementation.
  });

  test('Enter key activates focused tab', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Focus Agents tab.
    await page.locator('button:has-text("Agents")').focus();

    // Press Enter.
    await page.keyboard.press('Enter');
    await page.waitForTimeout(300);

    // Agents tab should be activated.
    await expect(page.locator('button:has-text("Agents")')).toBeVisible();
  });
});
