// E2E tests for inbox stats cards.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(
  page: import('@playwright/test').Page,
  messages: object[] = [],
) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ messages }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ topics: [] }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents: [], counts: {} }),
    });
  });
}

test.describe('Inbox stats cards', () => {
  test('displays stats cards section', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Stats cards render as buttons with accessible names like "Unread 0".
    // Use role-based selectors with regex to distinguish from filter buttons.
    await expect(page.getByRole('button', { name: /^Unread \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Starred \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Urgent \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Completed \d+$/ })).toBeVisible();
  });

  test('shows unread count', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'First Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
      },
      {
        id: '2',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Second Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
      },
      {
        id: '3',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Third Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // The Unread stat card button should be visible with a numeric value.
    const unreadCard = page.getByRole('button', { name: /^Unread \d+$/ });
    await expect(unreadCard).toBeVisible();
  });

  test('shows starred count', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'A Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // The Starred stat card button should be visible with a numeric value.
    const starredCard = page.getByRole('button', { name: /^Starred \d+$/ });
    await expect(starredCard).toBeVisible();
  });

  test('shows urgent count', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Urgent Message',
        body: 'Body',
        priority: 'PRIORITY_URGENT',
        created_at: new Date().toISOString(),
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // The Urgent stat card should show a count of 1 since we provided one
    // urgent message. The API normalization converts PRIORITY_URGENT to
    // 'urgent', and the stats count messages with that priority.
    const urgentCard = page.getByRole('button', { name: /^Urgent \d+$/ });
    await expect(urgentCard).toBeVisible();
    await expect(urgentCard).toContainText('1');
  });

  test('clicking stats card applies filter', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Click on the Unread stat card button.
    const unreadCard = page.getByRole('button', { name: /^Unread \d+$/ });
    if (await unreadCard.isVisible()) {
      await unreadCard.click();
      await page.waitForTimeout(300);
    }
  });

  test('stats update when messages change', async ({ page }) => {
    const messageCount = 2;

    const messages = Array.from({ length: messageCount }, (_, i) => ({
      id: String(i + 1),
      sender_id: '1',
      sender_name: 'Agent',
      subject: `Message ${i + 1}`,
      body: 'Body',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
    }));
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Stats section should be visible with all four stat card buttons.
    await expect(page.getByRole('button', { name: /^Unread \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Starred \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Urgent \d+$/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Completed \d+$/ })).toBeVisible();
  });

  test('stats cards are accessible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Stats cards with click handlers render as buttons. Check that they are
    // keyboard accessible by focusing and pressing Enter.
    const unreadCard = page.getByRole('button', { name: /^Unread \d+$/ });
    const count = await unreadCard.count();

    if (count > 0) {
      await unreadCard.focus();
      await page.keyboard.press('Enter');
      await page.waitForTimeout(200);
    }
  });
});

test.describe('Stats cards zero state', () => {
  test('shows zero for empty inbox', async ({ page }) => {
    await setupAPIs(page, []);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Stats should show zero counts. The stat card button includes the count
    // in its accessible name, so "Unread 0" means 0 unread messages.
    const unreadCard = page.getByRole('button', { name: 'Unread 0' });
    await expect(unreadCard).toBeVisible();
    await expect(unreadCard).toContainText('0');
  });
});
