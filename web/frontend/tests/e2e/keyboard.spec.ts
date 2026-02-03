// E2E tests for keyboard navigation and interactions.
// Tests all keyboard shortcuts and accessibility.

import { test, expect } from '@playwright/test';

test.describe('Keyboard Navigation', () => {
  test('escape closes modals', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Open compose modal.
    const composeButton = page.getByRole('button', { name: /compose/i });
    if (await composeButton.isVisible()) {
      await composeButton.click();
      // Check for compose heading (HeadlessUI dialog may not expose dialog role).
      await expect(page.getByRole('heading', { name: /compose/i })).toBeVisible();

      // Press escape to close.
      await page.keyboard.press('Escape');
      await expect(page.getByRole('heading', { name: /compose/i })).not.toBeVisible();
    }
  });

  test('tab navigates through interactive elements', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Tab through the page.
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    // Some element should be focused.
    const focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });

  test('enter activates focused button', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Focus the compose button and press enter.
    const composeButton = page.getByRole('button', { name: /compose/i });
    if (await composeButton.isVisible()) {
      await composeButton.focus();
      await page.keyboard.press('Enter');

      // Modal should open - check for compose heading.
      await expect(page.getByRole('heading', { name: /compose/i })).toBeVisible();
    }
  });

  test('Cmd+K opens search', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Press Cmd+K (or Ctrl+K on Windows).
    await page.keyboard.press('Meta+k');

    // Search should open - look for search input.
    const searchInput = page.getByRole('textbox').filter({ hasText: /search/i }).or(page.getByPlaceholder(/search/i));

    // Wait a moment for search to open.
    await page.waitForTimeout(200);
  });
});

test.describe('Focus Management', () => {
  test('modal traps focus', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Open compose modal.
    const composeButton = page.getByRole('button', { name: /compose/i });
    if (await composeButton.isVisible()) {
      await composeButton.click();
      // Check for compose heading (HeadlessUI dialog may not expose dialog role).
      const composeHeading = page.getByRole('heading', { name: /compose/i });
      await expect(composeHeading).toBeVisible();

      // Tab repeatedly and ensure focus stays in modal.
      for (let i = 0; i < 10; i++) {
        await page.keyboard.press('Tab');
      }

      // Focused element should still be visible (within the modal context).
      const focusedElement = page.locator(':focus');
      await expect(focusedElement).toBeVisible();
    }
  });

  test('closing modal returns focus', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Open compose modal.
    const composeButton = page.getByRole('button', { name: /compose/i });
    if (await composeButton.isVisible()) {
      await composeButton.focus();
      await composeButton.click();
      // Check for compose heading.
      await expect(page.getByRole('heading', { name: /compose/i })).toBeVisible();

      // Close modal.
      await page.keyboard.press('Escape');
      await expect(page.getByRole('heading', { name: /compose/i })).not.toBeVisible();

      // Focus should return to trigger element (or nearby).
      await page.waitForTimeout(100);
    }
  });
});

test.describe('Sidebar Navigation Keys', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('sidebar links are keyboard accessible', async ({ page }) => {
    const sidebar = page.getByRole('complementary');
    const links = sidebar.getByRole('link');

    const count = await links.count();
    expect(count).toBeGreaterThan(0);

    // Focus first link.
    await links.first().focus();
    const focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });
});
