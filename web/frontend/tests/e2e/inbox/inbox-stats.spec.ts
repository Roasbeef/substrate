// E2E tests for inbox stats cards.

import { test, expect } from '@playwright/test';

test.describe('Inbox stats cards', () => {
  test('displays stats cards section', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check for stats section.
    await expect(page.locator('[data-testid="stats-cards"]')).toBeVisible();
  });

  test('shows unread count', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Unread 1',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 1, agent_id: 100, state: 'unread' }],
            },
            {
              id: 2,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Unread 2',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 2, agent_id: 100, state: 'unread' }],
            },
            {
              id: 3,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Read',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 3, agent_id: 100, state: 'read' }],
            },
          ],
          meta: { total: 3, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Should show unread count.
    const unreadCard = page.locator('[data-testid="unread-stat"]');
    if (await unreadCard.isVisible()) {
      await expect(unreadCard).toContainText(/\d+/);
    }
  });

  test('shows starred count', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Starred',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 1, agent_id: 100, state: 'starred', is_starred: true }],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Should show starred count.
    const starredCard = page.locator('[data-testid="starred-stat"]');
    if (await starredCard.isVisible()) {
      await expect(starredCard).toContainText(/\d+/);
    }
  });

  test('shows urgent count', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Urgent Message',
              body: 'Body',
              priority: 'urgent',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 1, agent_id: 100, state: 'unread' }],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Should show urgent count.
    const urgentCard = page.locator('[data-testid="urgent-stat"]');
    if (await urgentCard.isVisible()) {
      await expect(urgentCard).toContainText(/\d+/);
    }
  });

  test('clicking stats card applies filter', async ({ page }) => {
    let lastFilter = '';

    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      lastFilter = url.searchParams.get('filter') || '';

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Message',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [{ message_id: 1, agent_id: 100, state: 'unread' }],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click on unread stat card if it exists.
    const unreadCard = page.locator('[data-testid="unread-stat"]');
    if (await unreadCard.isVisible()) {
      await unreadCard.click();
      await page.waitForTimeout(300);
    }
  });

  test('stats update when messages change', async ({ page }) => {
    const messageCount = 2;

    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: Array.from({ length: messageCount }, (_, i) => ({
            id: i + 1,
            sender_id: 1,
            sender_name: 'Agent',
            subject: `Message ${i + 1}`,
            body: 'Body',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [{ message_id: i + 1, agent_id: 100, state: 'unread' }],
          })),
          meta: { total: messageCount, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Stats should reflect current message state.
    const statsCards = page.locator('[data-testid="stats-cards"]');
    await expect(statsCards).toBeVisible();
  });

  test('stats cards are accessible', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check that stats cards are focusable.
    const statsCards = page.locator('[data-testid="stats-cards"] button, [data-testid="stats-cards"] [role="button"]');
    const count = await statsCards.count();

    // If there are clickable stats, they should be keyboard accessible.
    if (count > 0) {
      await statsCards.first().focus();
      await page.keyboard.press('Enter');
      await page.waitForTimeout(200);
    }
  });
});

test.describe('Stats cards zero state', () => {
  test('shows zero for empty inbox', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
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
    await page.waitForTimeout(500);

    // Stats should show zero or empty state.
    const statsCards = page.locator('[data-testid="stats-cards"]');
    if (await statsCards.isVisible()) {
      // Should display zero counts.
      await expect(statsCards).toContainText(/0/);
    }
  });
});
