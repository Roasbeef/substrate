import { test, expect } from '@playwright/test';

test('check console for errors on click', async ({ page }) => {
  const errors: string[] = [];
  const logs: string[] = [];

  page.on('console', msg => {
    if (msg.type() === 'error') {
      errors.push(msg.text());
    }
    logs.push('[' + msg.type() + '] ' + msg.text());
  });

  page.on('response', response => {
    if (response.status() >= 400) {
      logs.push('HTTP ' + response.status() + ': ' + response.url());
    }
  });

  await page.goto('/inbox');
  await page.waitForLoadState('networkidle');

  // Find a message row and click it
  const messageRow = page.locator('[data-testid="message-row"]').first();
  if (await messageRow.isVisible()) {
    console.log('Found message row, clicking...');
    await messageRow.click();
    await page.waitForTimeout(2000);
  } else {
    console.log('No message rows found - inbox might be empty');
  }

  console.log('=== Console Logs ===');
  logs.forEach(log => console.log(log));

  console.log('=== Errors ===');
  errors.forEach(err => console.log(err));

  // Report but don't fail - we want to see the output
  if (errors.length > 0) {
    console.log('Found ' + errors.length + ' console errors');
  }
});
