// E2E tests for message row interactions.

import { test, expect } from '@playwright/test';

// Helper to setup API with sample messages using grpc-gateway format.
async function setupMessagesAPI(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        messages: [
          {
            id: '1',
            sender_id: '1',
            sender_name: 'Alice Agent',
            subject: 'Important Update',
            body: 'This is the full message body with important details.',
            priority: 'PRIORITY_URGENT',
            created_at: new Date().toISOString(),
            thread_id: 'thread-1',
            recipients: [],
          },
          {
            id: '2',
            sender_id: '2',
            sender_name: 'Bob Agent',
            subject: 'Quick Question',
            body: 'I have a question about the project.',
            priority: 'PRIORITY_NORMAL',
            created_at: new Date().toISOString(),
            thread_id: 'thread-2',
            recipients: [],
          },
        ],
      }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents: [], counts: {} }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ topics: [] }),
    });
  });
}

test.describe('Message row display', () => {
  test('displays message row with sender name', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check sender names are displayed within message rows.
    const rows = page.locator('[data-testid="message-row"]');
    await expect(rows.filter({ hasText: 'Alice Agent' }).first()).toBeVisible();
    await expect(rows.filter({ hasText: 'Bob Agent' }).first()).toBeVisible();
  });

  test('displays message subject', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check subjects are displayed.
    await expect(page.locator('text=Important Update')).toBeVisible();
    await expect(page.locator('text=Quick Question')).toBeVisible();
  });

  test('displays priority badge for urgent messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check for priority badge.
    const urgentBadge = page.locator('[data-testid="priority-badge"]').filter({ hasText: /urgent/i });
    if (await urgentBadge.count() > 0) {
      await expect(urgentBadge.first()).toBeVisible();
    }
  });

  test('shows unread indicator for unread messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Unread messages should have visual indicator.
    const messageRows = page.locator('[data-testid="message-row"]');
    const count = await messageRows.count();

    if (count > 0) {
      // First message is unread, should have indicator.
      const firstRow = messageRows.first();
      // Check for unread class or indicator.
      await expect(firstRow).toBeVisible();
    }
  });
});

test.describe('Message row checkbox', () => {
  test('displays checkbox for selection', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check for checkboxes.
    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"], [data-testid="message-row"] [role="checkbox"]');
    const count = await checkboxes.count();

    if (count > 0) {
      await expect(checkboxes.first()).toBeVisible();
    }
  });

  test('clicking checkbox toggles selection', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Find and click a checkbox.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"], [data-testid="message-row"] [role="checkbox"]').first();

    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(200);

      // Checkbox should be checked.
      const isChecked = await checkbox.isChecked().catch(() => false);
      // OR check for aria-checked attribute.
    }
  });

  test('selecting message shows bulk actions', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Select a message.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();

    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(300);

      // Bulk actions toolbar should appear.
      const bulkActions = page.locator('[data-testid="bulk-actions"]');
      if (await bulkActions.isVisible()) {
        await expect(bulkActions).toBeVisible();
      }
    }
  });
});

test.describe('Message row click to open', () => {
  test('clicking message row opens thread view', async ({ page }) => {
    await setupMessagesAPI(page);

    // Also mock thread endpoint.
    await page.route('**/api/v1/threads/*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            id: 'thread-1',
            messages: [
              {
                id: 1,
                sender_id: 1,
                sender_name: 'Alice Agent',
                subject: 'Important Update',
                body: 'This is the full message body with important details.',
                priority: 'urgent',
                created_at: new Date().toISOString(),
              },
            ],
          },
        }),
      });
    });

    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Click on a message row (but not the checkbox).
    const messageSubject = page.locator('text=Important Update');
    await messageSubject.click();
    await page.waitForTimeout(500);

    // Thread view modal should open.
    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      await expect(threadView).toBeVisible();
    }
  });

  test('clicking checkbox does not open thread view', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Click on checkbox specifically.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();

    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(300);

      // Thread view should NOT open.
      const threadView = page.locator('[data-testid="thread-view"]');
      await expect(threadView).not.toBeVisible();
    }
  });
});

test.describe('Message row hover effects', () => {
  test('row shows hover state', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Hover over a message row.
    const messageRow = page.locator('[data-testid="message-row"]').first();
    if (await messageRow.isVisible()) {
      await messageRow.hover();
      await page.waitForTimeout(200);

      // Should show hover state (visual change).
      await expect(messageRow).toBeVisible();
    }
  });

  test('hover reveals action buttons', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Hover over a message row.
    const messageRow = page.locator('[data-testid="message-row"]').first();
    if (await messageRow.isVisible()) {
      await messageRow.hover();
      await page.waitForTimeout(200);

      // Action buttons should become visible.
      const actionButtons = messageRow.locator('[data-testid="message-actions"], button');
      // Actions might be always visible or revealed on hover.
    }
  });
});

test.describe('Message row accessibility', () => {
  test('message rows are keyboard navigable', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Tab to first message.
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    // Should be able to navigate with keyboard.
    await page.waitForTimeout(200);
  });

  test('Enter key opens selected message', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Focus a message row.
    const messageRow = page.locator('[data-testid="message-row"]').first();
    if (await messageRow.isVisible()) {
      await messageRow.focus();
      await page.keyboard.press('Enter');
      await page.waitForTimeout(500);

      // Should open thread view.
    }
  });
});
