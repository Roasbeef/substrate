// Example E2E test to verify Playwright setup.

import { test, expect } from '@playwright/test';

test.describe('App smoke tests', () => {
  test('homepage loads', async ({ page }) => {
    await page.goto('/');

    // Check that the page has a heading.
    await expect(page.locator('h1')).toBeVisible();
  });

  test('has correct title', async ({ page }) => {
    await page.goto('/');

    // Check page title.
    await expect(page).toHaveTitle(/Subtrate/);
  });
});
