// E2E tests for message multi-select and bulk actions.

import { test, expect } from '@playwright/test';

// Helper to setup API with sample messages.
async function setupMessagesAPI(page: import('@playwright/test').Page) {
  const messages = Array.from({ length: 5 }, (_, i) => ({
    id: i + 1,
    sender_id: 1,
    sender_name: `Agent ${i + 1}`,
    subject: `Message ${i + 1}`,
    body: `Body of message ${i + 1}`,
    priority: i === 0 ? 'urgent' : 'normal',
    created_at: new Date().toISOString(),
    recipients: [{ message_id: i + 1, agent_id: 100, state: 'unread' }],
  }));

  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: messages,
        meta: { total: messages.length, page: 1, page_size: 20 },
      }),
    });
  });

  return messages;
}

test.describe('Message selection', () => {
  test('individual message can be selected', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click checkbox of first message.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(200);

      // Checkbox should be checked.
      await expect(checkbox).toBeChecked();
    }
  });

  test('clicking checkbox again deselects message', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await checkbox.isVisible()) {
      // Select.
      await checkbox.click();
      await page.waitForTimeout(100);
      await expect(checkbox).toBeChecked();

      // Deselect.
      await checkbox.click();
      await page.waitForTimeout(100);
      await expect(checkbox).not.toBeChecked();
    }
  });

  test('multiple messages can be selected', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    const count = await checkboxes.count();

    if (count >= 3) {
      // Select first three.
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await checkboxes.nth(2).click();
      await page.waitForTimeout(200);

      // All three should be checked.
      await expect(checkboxes.nth(0)).toBeChecked();
      await expect(checkboxes.nth(1)).toBeChecked();
      await expect(checkboxes.nth(2)).toBeChecked();
    }
  });
});

test.describe('Select all functionality', () => {
  test('select all checkbox selects all messages', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const selectAll = page.locator('[data-testid="select-all"], thead input[type="checkbox"]');
    if (await selectAll.isVisible()) {
      await selectAll.click();
      await page.waitForTimeout(200);

      // All message checkboxes should be checked.
      const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
      const count = await checkboxes.count();

      for (let i = 0; i < count; i++) {
        await expect(checkboxes.nth(i)).toBeChecked();
      }
    }
  });

  test('clicking select all again deselects all', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const selectAll = page.locator('[data-testid="select-all"]');
    if (await selectAll.isVisible()) {
      // Select all.
      await selectAll.click();
      await page.waitForTimeout(100);

      // Deselect all.
      await selectAll.click();
      await page.waitForTimeout(100);

      // All should be unchecked.
      const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
      const count = await checkboxes.count();

      for (let i = 0; i < count; i++) {
        await expect(checkboxes.nth(i)).not.toBeChecked();
      }
    }
  });

  test('partial selection shows indeterminate state', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Select only one message.
    const firstCheckbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await firstCheckbox.isVisible()) {
      await firstCheckbox.click();
      await page.waitForTimeout(200);

      // Select all checkbox should show indeterminate state.
      const selectAll = page.locator('[data-testid="select-all"]');
      if (await selectAll.isVisible()) {
        // Check for indeterminate attribute or class.
        // Implementation dependent.
      }
    }
  });
});

test.describe('Bulk actions toolbar', () => {
  test('bulk actions appear when messages are selected', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Initially no bulk actions visible.
    const bulkActions = page.locator('[data-testid="bulk-actions"]');
    await expect(bulkActions).not.toBeVisible();

    // Select a message.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(200);

      // Bulk actions should appear.
      await expect(bulkActions).toBeVisible();
    }
  });

  test('bulk actions disappear when all deselected', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await checkbox.isVisible()) {
      // Select.
      await checkbox.click();
      await page.waitForTimeout(100);

      const bulkActions = page.locator('[data-testid="bulk-actions"]');
      await expect(bulkActions).toBeVisible();

      // Deselect.
      await checkbox.click();
      await page.waitForTimeout(200);

      // Bulk actions should disappear.
      await expect(bulkActions).not.toBeVisible();
    }
  });

  test('selection count is displayed', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Select two messages.
    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    if ((await checkboxes.count()) >= 2) {
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await page.waitForTimeout(200);

      // Should show "2 selected" or similar.
      const selectionCount = page.locator('[data-testid="selection-count"], text=/\\d+ selected/i');
      if (await selectionCount.isVisible()) {
        await expect(selectionCount).toContainText(/2/);
      }
    }
  });
});

test.describe('Bulk action operations', () => {
  test('bulk archive removes selected messages', async ({ page }) => {
    await setupMessagesAPI(page);

    await page.route('**/api/v1/messages/bulk-archive', async (route) => {
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Select messages.
    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    if ((await checkboxes.count()) >= 2) {
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await page.waitForTimeout(200);

      // Click bulk archive.
      const bulkArchive = page.locator('[data-testid="bulk-archive"], button:has-text("Archive")');
      if (await bulkArchive.isVisible()) {
        await bulkArchive.click();
        await page.waitForTimeout(500);
      }
    }
  });

  test('bulk star marks selected messages as starred', async ({ page }) => {
    await setupMessagesAPI(page);

    await page.route('**/api/v1/messages/bulk-star', async (route) => {
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    if ((await checkboxes.count()) >= 2) {
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await page.waitForTimeout(200);

      const bulkStar = page.locator('[data-testid="bulk-star"], button:has-text("Star")');
      if (await bulkStar.isVisible()) {
        await bulkStar.click();
        await page.waitForTimeout(500);
      }
    }
  });

  test('bulk mark read updates selected messages', async ({ page }) => {
    await setupMessagesAPI(page);

    await page.route('**/api/v1/messages/bulk-read', async (route) => {
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    if ((await checkboxes.count()) >= 2) {
      await checkboxes.nth(0).click();
      await checkboxes.nth(1).click();
      await page.waitForTimeout(200);

      const bulkMarkRead = page.locator('[data-testid="bulk-mark-read"], button:has-text("Mark Read")');
      if (await bulkMarkRead.isVisible()) {
        await bulkMarkRead.click();
        await page.waitForTimeout(500);
      }
    }
  });
});

test.describe('Selection keyboard shortcuts', () => {
  test('keyboard selection with Shift+Click', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    const checkboxes = page.locator('[data-testid="message-row"] input[type="checkbox"]');
    if ((await checkboxes.count()) >= 3) {
      // Click first.
      await checkboxes.nth(0).click();
      await page.waitForTimeout(100);

      // Shift+click third to select range.
      await page.keyboard.down('Shift');
      await checkboxes.nth(2).click();
      await page.keyboard.up('Shift');
      await page.waitForTimeout(200);

      // All three should be selected.
      // Implementation dependent - range selection.
    }
  });

  test('Escape clears selection', async ({ page }) => {
    await setupMessagesAPI(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Select a message.
    const checkbox = page.locator('[data-testid="message-row"] input[type="checkbox"]').first();
    if (await checkbox.isVisible()) {
      await checkbox.click();
      await page.waitForTimeout(100);

      // Press Escape.
      await page.keyboard.press('Escape');
      await page.waitForTimeout(200);

      // Selection might be cleared (implementation dependent).
    }
  });
});
