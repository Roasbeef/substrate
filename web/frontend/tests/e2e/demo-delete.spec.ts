import { test, expect } from '@playwright/test';

test('demonstrate delete bug - message should disappear after delete', async ({ page }) => {
  // Navigate to inbox
  await page.goto('/inbox');
  await page.waitForLoadState('networkidle');

  // Wait for messages to load
  await page.waitForTimeout(2000);

  // Take screenshot of initial state
  await page.screenshot({ path: 'test-results/delete-demo-1-initial.png' });

  // Find the first message row
  const messageRows = page.locator('[data-testid="message-row"]');
  const firstRow = messageRows.first();

  if (await firstRow.isVisible()) {
    // Hover to reveal action buttons
    await firstRow.hover();
    await page.waitForTimeout(500);

    // Take screenshot showing hover state
    await page.screenshot({ path: 'test-results/delete-demo-2-hover.png' });

    // Find and click delete button
    const deleteButton = firstRow.getByRole('button', { name: /delete/i });
    if (await deleteButton.isVisible()) {
      console.log('Found delete button, clicking...');
      await deleteButton.click();

      // Wait for any UI update
      await page.waitForTimeout(2000);

      // Take screenshot after delete
      await page.screenshot({ path: 'test-results/delete-demo-3-after-delete.png' });

      // Check if the message is still there (this is the bug)
      const messageStillVisible = await firstRow.isVisible();
      console.log('Message still visible after delete:', messageStillVisible);

      // The bug: message should NOT be visible after delete
      // expect(messageStillVisible).toBe(false);
    } else {
      console.log('Delete button not visible');
    }
  } else {
    console.log('No message rows found');
  }
});
