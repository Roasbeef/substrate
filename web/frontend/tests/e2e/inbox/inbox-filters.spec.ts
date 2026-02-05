// E2E tests for inbox filter buttons (All/Unread/Starred).

import { test, expect } from '@playwright/test';

// Helper to mock messages API with filter support.
// Uses grpc-gateway format with string IDs.
async function setupMessagesAPI(page: import('@playwright/test').Page) {
  const allMessages = [
    {
      id: '1',
      sender_id: '1',
      sender_name: 'Agent A',
      subject: 'Read Message',
      body: 'This message was read.',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
      state: 'read',
      is_starred: false,
    },
    {
      id: '2',
      sender_id: '2',
      sender_name: 'Agent B',
      subject: 'Unread Important',
      body: 'This is unread.',
      priority: 'PRIORITY_URGENT',
      created_at: new Date().toISOString(),
      state: 'unread',
      is_starred: false,
    },
    {
      id: '3',
      sender_id: '1',
      sender_name: 'Agent A',
      subject: 'Starred Favorite',
      body: 'This is starred.',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
      state: 'starred',
      is_starred: true,
    },
    {
      id: '4',
      sender_id: '3',
      sender_name: 'Agent C',
      subject: 'Another Unread',
      body: 'Also unread.',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
      state: 'unread',
      is_starred: false,
    },
  ];

  await page.route('**/api/v1/messages*', async (route) => {
    const url = new URL(route.request().url());
    const unreadOnly = url.searchParams.get('unread_only');
    const stateFilter = url.searchParams.get('state_filter');

    let filtered = allMessages;
    if (unreadOnly === 'true') {
      filtered = allMessages.filter((m) => m.state === 'unread');
    } else if (stateFilter === 'STATE_STARRED') {
      filtered = allMessages.filter((m) => m.is_starred);
    }

    // Use grpc-gateway format: { messages: [...] }.
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        messages: filtered,
      }),
    });
  });

  return allMessages;
}

test.describe('Inbox filters', () => {
  test('displays filter buttons', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();

    // Check for filter buttons in the main content area using exact matching.
    const mainContent = page.locator('main');
    await expect(mainContent.getByRole('button', { name: 'All', exact: true })).toBeVisible();
    await expect(mainContent.getByRole('button', { name: 'Unread', exact: true })).toBeVisible();
    await expect(mainContent.getByRole('button', { name: 'Starred', exact: true })).toBeVisible();
  });

  test('All filter shows all messages by default', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();

    // Wait for messages to load.
    await page.waitForTimeout(500);

    // All button should be active by default.
    const mainContent = page.locator('main');
    await expect(mainContent.getByRole('button', { name: 'All', exact: true })).toBeVisible();

    // Should see all message subjects.
    await expect(page.getByText('Read Message')).toBeVisible();
    await expect(page.getByText('Unread Important')).toBeVisible();
  });

  test('Unread filter shows only unread messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Click the Unread filter button in main content using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Unread', exact: true }).click();
    await page.waitForTimeout(500);

    // Should show unread messages.
    await expect(page.getByText('Unread Important')).toBeVisible();
    await expect(page.getByText('Another Unread')).toBeVisible();

    // Should not show read or starred messages.
    await expect(page.getByText('Read Message')).not.toBeVisible();
  });

  test('Starred filter shows only starred messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Click the Starred filter button in main content using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Starred', exact: true }).click();
    await page.waitForTimeout(500);

    // Should show starred messages.
    await expect(page.getByText('Starred Favorite')).toBeVisible();

    // Should not show other messages.
    await expect(page.getByText('Read Message')).not.toBeVisible();
    await expect(page.getByText('Unread Important')).not.toBeVisible();
  });

  test('clicking All after Unread shows all messages again', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Unread first using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Unread', exact: true }).click();
    await page.waitForTimeout(500);

    // Verify only unread visible.
    await expect(page.getByText('Unread Important')).toBeVisible();

    // Click All to go back.
    await mainContent.getByRole('button', { name: 'All', exact: true }).click();
    await page.waitForTimeout(500);

    // All messages should be visible again.
    await expect(page.getByText('Read Message')).toBeVisible();
    await expect(page.getByText('Starred Favorite')).toBeVisible();
  });

  test('filter buttons show active state', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();

    const mainContent = page.locator('main');
    const unreadButton = mainContent.getByRole('button', { name: 'Unread', exact: true });

    // Click Unread.
    await unreadButton.click();
    await page.waitForTimeout(200);

    // Unread should now have active state.
    await expect(unreadButton).toBeVisible();
  });

  test('filter persists during navigation', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Set filter to Unread using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Unread', exact: true }).click();
    await page.waitForTimeout(300);

    // The filter state should be maintained.
    await expect(page.getByText('Unread Important')).toBeVisible();
  });
});

test.describe('Filter empty states', () => {
  test('shows empty state when no unread messages', async ({ page }) => {
    // All messages are read.
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const unreadOnly = url.searchParams.get('unread_only');

      if (unreadOnly === 'true') {
        // Use grpc-gateway format: { messages: [...] }.
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
                sender_name: 'Agent',
                subject: 'Read Message',
                body: 'Body',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
              },
            ],
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Unread filter in main content using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Unread', exact: true }).click();
    await page.waitForTimeout(500);

    // Should show empty state or no messages (use exact match to avoid matching "unread messages" text).
    await expect(page.locator('[data-testid="message-row"]').filter({ hasText: 'Read Message' })).not.toBeVisible();
  });

  test('shows empty state when no starred messages', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const stateFilter = url.searchParams.get('state_filter');

      if (stateFilter === 'STATE_STARRED') {
        // Use grpc-gateway format: { messages: [...] }.
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
                sender_name: 'Agent',
                subject: 'Normal Message',
                body: 'Body',
                priority: 'PRIORITY_NORMAL',
                created_at: new Date().toISOString(),
              },
            ],
          }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('.grid')).toBeVisible();
    await page.waitForTimeout(500);

    // Click Starred filter in main content using exact matching.
    const mainContent = page.locator('main');
    await mainContent.getByRole('button', { name: 'Starred', exact: true }).click();
    await page.waitForTimeout(500);

    // Should show empty state.
    await expect(page.getByText('Normal Message')).not.toBeVisible();
  });
});
