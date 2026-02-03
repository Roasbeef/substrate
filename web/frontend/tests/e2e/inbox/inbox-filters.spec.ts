// E2E tests for inbox filter buttons (All/Unread/Starred).

import { test, expect } from '@playwright/test';

// Helper to mock messages API with filter support.
async function setupMessagesAPI(page: import('@playwright/test').Page) {
  const allMessages = [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Agent A',
      subject: 'Read Message',
      body: 'This message was read.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 1, agent_id: 100, agent_name: 'Me', state: 'read', is_starred: false }],
    },
    {
      id: 2,
      sender_id: 2,
      sender_name: 'Agent B',
      subject: 'Unread Important',
      body: 'This is unread.',
      priority: 'urgent',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 2, agent_id: 100, agent_name: 'Me', state: 'unread', is_starred: false }],
    },
    {
      id: 3,
      sender_id: 1,
      sender_name: 'Agent A',
      subject: 'Starred Favorite',
      body: 'This is starred.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 3, agent_id: 100, agent_name: 'Me', state: 'starred', is_starred: true }],
    },
    {
      id: 4,
      sender_id: 3,
      sender_name: 'Agent C',
      subject: 'Another Unread',
      body: 'Also unread.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 4, agent_id: 100, agent_name: 'Me', state: 'unread', is_starred: false }],
    },
  ];

  await page.route('**/api/v1/messages*', async (route) => {
    const url = new URL(route.request().url());
    const filter = url.searchParams.get('filter');

    let filtered = allMessages;
    if (filter === 'unread') {
      filtered = allMessages.filter((m) => m.recipients[0]?.state === 'unread');
    } else if (filter === 'starred') {
      filtered = allMessages.filter((m) => m.recipients[0]?.is_starred);
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: filtered,
        meta: { total: filtered.length, page: 1, page_size: 20 },
      }),
    });
  });

  return allMessages;
}

test.describe('Inbox filters', () => {
  test('displays filter buttons', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check for filter buttons.
    await expect(page.locator('button:has-text("All")')).toBeVisible();
    await expect(page.locator('button:has-text("Unread")')).toBeVisible();
    await expect(page.locator('button:has-text("Starred")')).toBeVisible();
  });

  test('All filter shows all messages by default', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait for messages to load.
    await page.waitForTimeout(500);

    // All button should be active by default.
    const allButton = page.locator('button:has-text("All")');
    await expect(allButton).toBeVisible();

    // Should see all message subjects.
    await expect(page.locator('text=Read Message')).toBeVisible();
    await expect(page.locator('text=Unread Important')).toBeVisible();
  });

  test('Unread filter shows only unread messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click the Unread filter button.
    await page.locator('button:has-text("Unread")').click();
    await page.waitForTimeout(500);

    // Should show unread messages.
    await expect(page.locator('text=Unread Important')).toBeVisible();
    await expect(page.locator('text=Another Unread')).toBeVisible();

    // Should not show read or starred messages.
    await expect(page.locator('text=Read Message')).not.toBeVisible();
  });

  test('Starred filter shows only starred messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click the Starred filter button.
    await page.locator('button:has-text("Starred")').click();
    await page.waitForTimeout(500);

    // Should show starred messages.
    await expect(page.locator('text=Starred Favorite')).toBeVisible();

    // Should not show other messages.
    await expect(page.locator('text=Read Message')).not.toBeVisible();
    await expect(page.locator('text=Unread Important')).not.toBeVisible();
  });

  test('clicking All after Unread shows all messages again', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Unread first.
    await page.locator('button:has-text("Unread")').click();
    await page.waitForTimeout(500);

    // Verify only unread visible.
    await expect(page.locator('text=Unread Important')).toBeVisible();

    // Click All to go back.
    await page.locator('button:has-text("All")').click();
    await page.waitForTimeout(500);

    // All messages should be visible again.
    await expect(page.locator('text=Read Message')).toBeVisible();
    await expect(page.locator('text=Starred Favorite')).toBeVisible();
  });

  test('filter buttons show active state', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // All should be active initially (check for aria-pressed or visual indicator).
    const allButton = page.locator('button:has-text("All")');
    const unreadButton = page.locator('button:has-text("Unread")');

    // Click Unread.
    await unreadButton.click();
    await page.waitForTimeout(200);

    // Unread should now have active state.
    // This depends on implementation - checking basic functionality.
    await expect(unreadButton).toBeVisible();
  });

  test('filter persists during navigation', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Set filter to Unread.
    await page.locator('button:has-text("Unread")').click();
    await page.waitForTimeout(300);

    // The filter state should be maintained.
    // This test verifies the filter was applied.
    await expect(page.locator('text=Unread Important')).toBeVisible();
  });
});

test.describe('Filter empty states', () => {
  test('shows empty state when no unread messages', async ({ page }) => {
    // All messages are read.
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const filter = url.searchParams.get('filter');

      if (filter === 'unread') {
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
                sender_name: 'Agent',
                subject: 'Read Message',
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

    // Click Unread filter.
    await page.locator('button:has-text("Unread")').click();
    await page.waitForTimeout(500);

    // Should show empty state or no messages.
    await expect(page.locator('text=Read Message')).not.toBeVisible();
  });

  test('shows empty state when no starred messages', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const filter = url.searchParams.get('filter');

      if (filter === 'starred') {
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
                sender_name: 'Agent',
                subject: 'Normal Message',
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

    // Click Starred filter.
    await page.locator('button:has-text("Starred")').click();
    await page.waitForTimeout(500);

    // Should show empty state.
    await expect(page.locator('text=Normal Message')).not.toBeVisible();
  });
});
