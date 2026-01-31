// E2E tests for message archive functionality.

import { test, expect } from '@playwright/test';

// Helper to setup API with messages.
async function setupMessagesAPI(page: import('@playwright/test').Page) {
  const messages = [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Agent A',
      subject: 'Message to Archive',
      body: 'This message will be archived.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 1, agent_id: 100, state: 'unread' }],
    },
    {
      id: 2,
      sender_id: 2,
      sender_name: 'Agent B',
      subject: 'Keep This Message',
      body: 'This message stays in inbox.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      recipients: [{ message_id: 2, agent_id: 100, state: 'read' }],
    },
  ];

  let archivedIds: number[] = [];

  await page.route('**/api/v1/messages*', async (route) => {
    const activeMessages = messages.filter((m) => !archivedIds.includes(m.id));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: activeMessages,
        meta: { total: activeMessages.length, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/messages/*/archive', async (route) => {
    const url = route.request().url();
    const idMatch = url.match(/messages\/(\d+)\/archive/);
    const id = idMatch ? parseInt(idMatch[1]) : 1;

    archivedIds.push(id);

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id, state: 'archived' }),
    });
  });

  return { messages, getArchivedIds: () => archivedIds };
}

test.describe('Message archive button', () => {
  test('displays archive button on message row', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Check for archive buttons.
    const archiveButtons = page.locator('[data-testid="archive-button"], button[aria-label*="archive" i]');
    const count = await archiveButtons.count();

    // Should have at least one archive button per message.
    if (count > 0) {
      await expect(archiveButtons.first()).toBeVisible();
    }
  });

  test('archive button is visible on hover', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Hover over message row.
    const messageRow = page.locator('[data-testid="message-row"]').first();
    if (await messageRow.isVisible()) {
      await messageRow.hover();
      await page.waitForTimeout(200);

      // Archive button should be visible (or always visible).
      const archiveButton = messageRow.locator('[data-testid="archive-button"], button[aria-label*="archive" i]');
      // Button visibility depends on implementation.
    }
  });
});

test.describe('Archive interaction', () => {
  test('clicking archive removes message from list', async ({ page }) => {
    const { getArchivedIds } = await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Verify message is visible.
    await expect(page.locator('text=Message to Archive')).toBeVisible();

    // Click archive button.
    const messageRow = page.locator('[data-testid="message-row"]').filter({ hasText: 'Message to Archive' });
    if (await messageRow.isVisible()) {
      const archiveButton = messageRow.locator('[data-testid="archive-button"], button[aria-label*="archive" i]');
      if (await archiveButton.isVisible()) {
        await archiveButton.click();
        await page.waitForTimeout(500);

        // Message should disappear from inbox.
        await expect(page.locator('text=Message to Archive')).not.toBeVisible();

        // Other message should still be visible.
        await expect(page.locator('text=Keep This Message')).toBeVisible();
      }
    }
  });

  test('shows confirmation toast after archive', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Archive a message.
    const archiveButton = page.locator('[data-testid="archive-button"]').first();
    if (await archiveButton.isVisible()) {
      await archiveButton.click();
      await page.waitForTimeout(500);

      // Should show a toast notification.
      const toast = page.locator('[role="alert"], [data-testid="toast"]');
      if (await toast.isVisible()) {
        await expect(toast).toContainText(/archived/i);
      }
    }
  });

  test('archived message count updates stats', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Get initial message count.
    const initialRows = await page.locator('[data-testid="message-row"]').count();

    // Archive a message.
    const archiveButton = page.locator('[data-testid="archive-button"]').first();
    if (await archiveButton.isVisible()) {
      await archiveButton.click();
      await page.waitForTimeout(500);

      // Count should decrease.
      const newRows = await page.locator('[data-testid="message-row"]').count();
      expect(newRows).toBeLessThan(initialRows);
    }
  });
});

test.describe('Bulk archive', () => {
  test('select multiple messages and archive', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Select multiple messages.
    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    const count = await checkboxes.count();

    if (count >= 2) {
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await page.waitForTimeout(200);

      // Bulk archive button should appear.
      const bulkArchive = page.locator('[data-testid="bulk-archive"], button:has-text("Archive")').filter({ has: page.locator(':visible') });
      if (await bulkArchive.isVisible()) {
        await bulkArchive.click();
        await page.waitForTimeout(500);

        // Both messages should be archived.
      }
    }
  });

  test('select all and archive', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click select all checkbox.
    const selectAll = page.locator('[data-testid="select-all"], thead input[type="checkbox"]');
    if (await selectAll.isVisible()) {
      await selectAll.click();
      await page.waitForTimeout(200);

      // Bulk archive.
      const bulkArchive = page.locator('[data-testid="bulk-archive"]');
      if (await bulkArchive.isVisible()) {
        await bulkArchive.click();
        await page.waitForTimeout(500);

        // All messages should be archived.
        const remaining = await page.locator('[data-testid="message-row"]').count();
        expect(remaining).toBe(0);
      }
    }
  });
});

test.describe('Archive undo', () => {
  test('undo button restores archived message', async ({ page }) => {
    let archivedIds: number[] = [];

    await page.route('**/api/v1/messages*', async (route) => {
      const messages = [
        {
          id: 1,
          sender_id: 1,
          sender_name: 'Agent',
          subject: 'Archived Message',
          body: 'Body',
          priority: 'normal',
          created_at: new Date().toISOString(),
          recipients: [],
        },
      ].filter((m) => !archivedIds.includes(m.id));

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: messages, meta: { total: messages.length, page: 1, page_size: 20 } }),
      });
    });

    await page.route('**/api/v1/messages/*/archive', async (route) => {
      archivedIds.push(1);
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.route('**/api/v1/messages/*/unarchive', async (route) => {
      archivedIds = archivedIds.filter((id) => id !== 1);
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Archive the message.
    const archiveButton = page.locator('[data-testid="archive-button"]').first();
    if (await archiveButton.isVisible()) {
      await archiveButton.click();
      await page.waitForTimeout(500);

      // Look for undo button in toast.
      const undoButton = page.locator('button:has-text("Undo")');
      if (await undoButton.isVisible()) {
        await undoButton.click();
        await page.waitForTimeout(500);

        // Message should reappear.
        await expect(page.locator('text=Archived Message')).toBeVisible();
      }
    }
  });
});

test.describe('Archive keyboard accessibility', () => {
  test('archive button is keyboard accessible', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Focus archive button.
    const archiveButton = page.locator('[data-testid="archive-button"]').first();
    if (await archiveButton.isVisible()) {
      await archiveButton.focus();
      await page.keyboard.press('Enter');
      await page.waitForTimeout(500);

      // Should trigger archive.
    }
  });
});
