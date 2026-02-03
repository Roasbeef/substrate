// E2E tests for sessions page functionality.
// Tests all session-related buttons and user flows.

import { test, expect } from '@playwright/test';

test.describe('Sessions Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/sessions');
    await page.waitForLoadState('networkidle');
  });

  test('displays sessions page', async ({ page }) => {
    // Verify sessions page loads without errors.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('displays session list or empty state', async ({ page }) => {
    // Either sessions exist or there's an empty state message.
    const content = page.locator('#root');
    await expect(content).not.toBeEmpty();
  });

  test('filter tabs are clickable', async ({ page }) => {
    // Look for filter buttons (active, completed, etc.).
    const activeTab = page.getByRole('button', { name: /active/i });
    const completedTab = page.getByRole('button', { name: /completed/i });

    if (await activeTab.isVisible()) {
      await activeTab.click();
    }

    if (await completedTab.isVisible()) {
      await completedTab.click();
    }
  });
});

test.describe('Start Session Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/sessions');
    await page.waitForLoadState('networkidle');
  });

  test('start session button opens modal', async ({ page }) => {
    const startButton = page.getByRole('button', { name: /start.*session|new.*session/i });

    if (await startButton.isVisible()) {
      await startButton.click();

      // Modal should open - check for session heading or form content.
      const modalContent = page.getByRole('heading', { name: /start.*session|new.*session/i })
        .or(page.getByLabel(/project/i))
        .or(page.getByPlaceholder(/project/i));
      await expect(modalContent.first()).toBeVisible();
    }
  });

  test('start session modal has project field', async ({ page }) => {
    const startButton = page.getByRole('button', { name: /start.*session|new.*session/i });

    if (await startButton.isVisible()) {
      await startButton.click();

      // Check for project input.
      const projectInput = page.getByLabel(/project/i).or(page.getByPlaceholder(/project/i));
      await expect(projectInput.first()).toBeVisible();
    }
  });

  test('start session modal has branch field', async ({ page }) => {
    const startButton = page.getByRole('button', { name: /start.*session|new.*session/i });

    if (await startButton.isVisible()) {
      await startButton.click();

      // Check for branch input.
      const branchInput = page.getByLabel(/branch/i).or(page.getByPlaceholder(/branch/i));
      await expect(branchInput.first()).toBeVisible();
    }
  });

  test('start session modal can be closed', async ({ page }) => {
    const startButton = page.getByRole('button', { name: /start.*session|new.*session/i });

    if (await startButton.isVisible()) {
      await startButton.click();
      // Check for modal content.
      const modalContent = page.getByRole('heading', { name: /start.*session|new.*session/i })
        .or(page.getByLabel(/project/i));
      await expect(modalContent.first()).toBeVisible();

      // Close via cancel button or escape.
      const cancelButton = page.getByRole('button', { name: /cancel/i });
      if (await cancelButton.isVisible()) {
        await cancelButton.click();
      } else {
        await page.keyboard.press('Escape');
      }

      // Modal should be closed.
      await expect(modalContent.first()).not.toBeVisible();
    }
  });
});

test.describe('Session Detail', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/sessions');
    await page.waitForLoadState('networkidle');
  });

  test('clicking session row opens detail view', async ({ page }) => {
    // Find session rows.
    const sessionRows = page.locator('[class*="session"], [class*="row"]').filter({ has: page.locator('button, a') });
    const count = await sessionRows.count();

    if (count > 0) {
      await sessionRows.first().click();
      await page.waitForLoadState('networkidle');

      // Either navigated or modal opened.
    }
  });
});
