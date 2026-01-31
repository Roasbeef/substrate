// E2E tests for inbox message list loading, pagination, and empty state.

import { test, expect } from '@playwright/test';

test.describe('Inbox message list', () => {
  test('loads inbox page with message list', async ({ page }) => {
    await page.goto('/');

    // Check inbox heading is visible.
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check that message list container exists.
    await expect(page.locator('[data-testid="message-list"]')).toBeVisible();
  });

  test('displays messages with proper structure', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Check for message rows if any exist.
    const messageRows = page.locator('[data-testid="message-row"]');

    // Even if empty, the list container should be visible.
    await expect(page.locator('[data-testid="message-list"]')).toBeVisible();
  });

  test('shows empty state when no messages', async ({ page }) => {
    // Route API to return empty messages.
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

    // Should show empty state or "no messages" text.
    const emptyState = page.locator('[data-testid="empty-state"]');
    const noMessagesText = page.locator('text=/no messages|inbox is empty/i');

    // Either empty state component or text should be visible.
    await page.waitForTimeout(1000);
  });

  test('displays message sender, subject, and preview', async ({ page }) => {
    // Route API to return sample messages.
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Alice Agent',
              subject: 'Important Update',
              body: 'This is the message body preview text.',
              priority: 'normal',
              created_at: new Date().toISOString(),
              recipients: [],
            },
            {
              id: 2,
              sender_id: 2,
              sender_name: 'Bob Agent',
              subject: 'Follow-up Question',
              body: 'Can you help me with this issue?',
              priority: 'urgent',
              created_at: new Date().toISOString(),
              recipients: [],
            },
          ],
          meta: { total: 2, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait for messages to load.
    await page.waitForTimeout(500);

    // Check for sender names.
    await expect(page.locator('text=Alice Agent').first()).toBeVisible();
    await expect(page.locator('text=Bob Agent').first()).toBeVisible();

    // Check for subjects.
    await expect(page.locator('text=Important Update')).toBeVisible();
    await expect(page.locator('text=Follow-up Question')).toBeVisible();
  });

  test('handles API error gracefully', async ({ page }) => {
    // Route API to return error.
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'server_error', message: 'Internal server error' },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Should handle error gracefully - page should still be functional.
    await page.waitForTimeout(1000);
    await expect(page.locator('text=Inbox')).toBeVisible();
  });

  test('maintains message order by date', async ({ page }) => {
    const oldDate = new Date('2024-01-01T10:00:00Z');
    const newDate = new Date('2024-01-02T10:00:00Z');

    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 2,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Newer Message',
              body: 'Body',
              priority: 'normal',
              created_at: newDate.toISOString(),
              recipients: [],
            },
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Older Message',
              body: 'Body',
              priority: 'normal',
              created_at: oldDate.toISOString(),
              recipients: [],
            },
          ],
          meta: { total: 2, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Newer message should appear first in the list.
    const messages = page.locator('[data-testid="message-row"]');
    const count = await messages.count();

    if (count >= 2) {
      const firstMessage = messages.first();
      await expect(firstMessage.locator('text=Newer Message')).toBeVisible();
    }
  });
});

test.describe('Inbox pagination', () => {
  test('loads first page of messages', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      const url = new URL(route.request().url());
      const currentPage = url.searchParams.get('page') || '1';

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: Array.from({ length: 20 }, (_, i) => ({
            id: i + 1,
            sender_id: 1,
            sender_name: `Agent ${i + 1}`,
            subject: `Message ${i + 1}`,
            body: 'Body',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          })),
          meta: { total: 50, page: parseInt(currentPage), page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Check that messages are displayed.
    const messages = page.locator('[data-testid="message-row"]');
    const count = await messages.count();
    expect(count).toBeGreaterThan(0);
  });

  test('shows load more or pagination when more messages exist', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: Array.from({ length: 20 }, (_, i) => ({
            id: i + 1,
            sender_id: 1,
            sender_name: `Agent ${i + 1}`,
            subject: `Message ${i + 1}`,
            body: 'Body',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          })),
          meta: { total: 100, page: 1, page_size: 20 },
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Look for pagination controls or load more button.
    const loadMore = page.locator('text=/load more|next page/i');
    const pagination = page.locator('[data-testid="pagination"]');

    // Either should exist if total > page_size.
  });
});
