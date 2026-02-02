import { test, expect } from '@playwright/test';

test('inspect inbox state', async ({ page }) => {
  // Navigate to inbox
  await page.goto('/inbox');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(3000);

  // Take screenshot
  await page.screenshot({ path: 'test-results/inbox-state.png', fullPage: true });

  // Log the page structure
  const content = await page.content();
  console.log('Page title:', await page.title());

  // Check for various message selectors
  const selectors = [
    '[data-testid="message-row"]',
    '[data-testid="message-list"]',
    'button[data-testid]',
    '[role="button"]',
    '.message-row',
    '[class*="message"]',
  ];

  for (const selector of selectors) {
    const count = await page.locator(selector).count();
    console.log(`Selector "${selector}": ${count} elements`);
  }

  // Try to find any clickable rows
  const buttons = await page.getByRole('button').all();
  console.log('Total buttons:', buttons.length);

  // Check if there's any message-like content
  const messageCount = await page.locator('text=/messages|20 messages/i').count();
  console.log('Message count text found:', messageCount);
});
