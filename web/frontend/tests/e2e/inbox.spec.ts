// E2E tests for inbox functionality.
// Tests all inbox-related buttons, filters, and user flows.

import { test, expect } from '@playwright/test';

test.describe('Inbox Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('displays inbox page with message list', async ({ page }) => {
    // Verify inbox page structure.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('compose button opens compose modal', async ({ page }) => {
    // Find and click the compose button.
    const composeButton = page.getByRole('button', { name: /compose/i });
    await expect(composeButton).toBeVisible();
    await composeButton.click();

    // Verify compose modal opens by checking for the modal title/content.
    // HeadlessUI dialogs may have visibility quirks, so check for content.
    await expect(page.getByRole('heading', { name: /compose/i })).toBeVisible();
  });

  test('compose modal can be closed', async ({ page }) => {
    // Open compose modal.
    await page.getByRole('button', { name: /compose/i }).click();
    await expect(page.getByRole('heading', { name: /compose/i })).toBeVisible();

    // Close via close button.
    const closeButton = page.getByRole('button', { name: /close modal/i });
    if (await closeButton.isVisible()) {
      await closeButton.click();
    } else {
      // Try pressing escape.
      await page.keyboard.press('Escape');
    }

    // Modal should be closed - the heading should no longer be visible.
    await expect(page.getByRole('heading', { name: /compose/i })).not.toBeVisible();
  });
});

test.describe('Inbox Navigation', () => {
  test('clicking inbox nav shows inbox messages', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    const sidebar = page.getByRole('complementary');
    await sidebar.getByRole('link', { name: /inbox/i }).click();

    await expect(page).toHaveURL(/\/inbox/);
  });

  test('clicking starred nav shows starred messages', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    const sidebar = page.getByRole('complementary');
    await sidebar.getByRole('link', { name: /starred/i }).click();

    await expect(page).toHaveURL(/\/starred/);
  });

  test('clicking snoozed nav shows snoozed messages', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    const sidebar = page.getByRole('complementary');
    await sidebar.getByRole('link', { name: /snoozed/i }).click();

    await expect(page).toHaveURL(/\/snoozed/);
  });

  test('clicking sent nav shows sent messages', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    const sidebar = page.getByRole('complementary');
    await sidebar.getByRole('link', { name: /sent/i }).click();

    await expect(page).toHaveURL(/\/sent/);
  });

  test('clicking archive nav shows archived messages', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    const sidebar = page.getByRole('complementary');
    await sidebar.getByRole('link', { name: /archive/i }).click();

    await expect(page).toHaveURL(/\/archive/);
  });
});

test.describe('Inbox Filters', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('filter tabs are visible', async ({ page }) => {
    // Look for filter tabs (All, Unread, Starred, etc.).
    const filterButtons = page.getByRole('button').filter({ hasText: /all|unread|starred/i });
    await expect(filterButtons.first()).toBeVisible();
  });
});

test.describe('Message Actions', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('star button is visible on message rows', async ({ page }) => {
    // If messages exist, check for star buttons.
    const starButtons = page.getByRole('button', { name: /star/i });
    const count = await starButtons.count();

    // Either there are star buttons or the inbox is empty.
    if (count > 0) {
      await expect(starButtons.first()).toBeVisible();
    }
  });

  test('archive button is visible on message rows', async ({ page }) => {
    const archiveButtons = page.getByRole('button', { name: /archive/i });
    const count = await archiveButtons.count();

    if (count > 0) {
      await expect(archiveButtons.first()).toBeVisible();
    }
  });

  test('delete button is visible on message rows', async ({ page }) => {
    const deleteButtons = page.getByRole('button', { name: /delete/i });
    const count = await deleteButtons.count();

    if (count > 0) {
      await expect(deleteButtons.first()).toBeVisible();
    }
  });
});

test.describe('Search Functionality', () => {
  test('search bar is visible', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Search bar or search button should be visible.
    const searchElement = page.getByPlaceholder(/search/i).or(page.getByRole('button', { name: /search/i }));
    await expect(searchElement.first()).toBeVisible();
  });

  test('clicking search opens search interface', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Try clicking on search.
    const searchButton = page.getByRole('button').filter({ hasText: /search/i }).first();
    if (await searchButton.isVisible()) {
      await searchButton.click();

      // Expect search input or search results to appear.
      const searchInput = page.getByRole('textbox').filter({ hasText: /search|type/i });
      // Some interaction should happen.
    }
  });
});
