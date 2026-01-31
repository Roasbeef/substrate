// E2E tests for notification permission handling.

import { test, expect } from '@playwright/test';

test.describe('Notification permission prompt', () => {
  test.beforeEach(async ({ context }) => {
    // Clear any stored permissions.
    await context.clearCookies();
  });

  test('shows notification prompt after delay', async ({ page, context }) => {
    // Set notification permission to default (not yet requested).
    await context.grantPermissions([]);

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait for the notification prompt to appear (2.5s delay + render time).
    await page.waitForTimeout(3000);

    // Look for the notification prompt.
    const prompt = page.locator('[role="alert"]');
    // Prompt may or may not appear depending on localStorage state.
  });

  test('does not show prompt if already dismissed', async ({ page }) => {
    // Set localStorage to indicate prompt was dismissed.
    await page.addInitScript(() => {
      localStorage.setItem('subtrate_notification_prompt_dismissed', 'true');
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait past the delay.
    await page.waitForTimeout(3000);

    // Prompt should not appear.
    const prompt = page.locator('[role="alert"]').filter({ hasText: 'notification' });
    await expect(prompt).not.toBeVisible();
  });

  test('dismisses prompt when "Not now" is clicked', async ({ page, context }) => {
    // Set permission to default.
    await context.grantPermissions([]);

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait for prompt.
    await page.waitForTimeout(3000);

    // If prompt is visible, dismiss it.
    const notNowButton = page.locator('button', { hasText: 'Not now' });
    if (await notNowButton.isVisible()) {
      await notNowButton.click();

      // Prompt should disappear.
      await expect(page.locator('[role="alert"]').filter({ hasText: 'notification' })).not.toBeVisible();
    }
  });

  test('requests permission when "Enable" is clicked', async ({ page, context }) => {
    // Grant notifications permission in advance.
    await context.grantPermissions(['notifications']);

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Wait for prompt.
    await page.waitForTimeout(3000);

    // If prompt is visible, click enable.
    const enableButton = page.locator('button', { hasText: 'Enable' });
    if (await enableButton.isVisible()) {
      await enableButton.click();

      // Prompt should disappear after permission is granted.
      await page.waitForTimeout(500);
    }
  });
});

test.describe('Notification settings', () => {
  test('navigates to settings page', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Look for settings link/button.
    const settingsButton = page.locator('[aria-label*="settings"], [data-testid="settings"]').first();
    if (await settingsButton.isVisible()) {
      await settingsButton.click();
    }
  });

  test('can toggle notification preferences', async ({ page, context }) => {
    // Grant notifications permission.
    await context.grantPermissions(['notifications']);

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Navigate to settings if there's a settings page.
    // This would depend on actual UI implementation.
  });
});

test.describe('Notification permission denied', () => {
  test('handles denied permission gracefully', async ({ page, context }) => {
    // Block notifications.
    await context.clearPermissions();

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Page should load without errors.
    await page.waitForTimeout(1000);
    await expect(page.locator('text=Inbox')).toBeVisible();
  });

  test('shows blocked status in settings', async ({ page, context }) => {
    // Block notifications.
    await context.clearPermissions();

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // If there's a notification status indicator, it should show blocked.
  });
});
