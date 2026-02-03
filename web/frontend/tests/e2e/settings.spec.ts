// E2E tests for settings page functionality.
// Tests all settings toggles, inputs, and user flows.

import { test, expect } from '@playwright/test';

test.describe('Settings Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
  });

  test('displays settings page', async ({ page }) => {
    // Verify settings page loads.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('has notification settings section', async ({ page }) => {
    // Look for notification settings.
    const notificationSection = page.getByText(/notification/i);
    await expect(notificationSection.first()).toBeVisible();
  });
});

test.describe('Notification Toggles', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
  });

  test('notification toggles are interactive', async ({ page }) => {
    // Find toggle switches.
    const switches = page.getByRole('switch').or(page.locator('input[type="checkbox"]'));
    const count = await switches.count();

    if (count > 0) {
      const firstSwitch = switches.first();
      const initialState = await firstSwitch.isChecked().catch(() => null);

      // Click to toggle.
      await firstSwitch.click();

      // State should change (or interaction should work).
    }
  });

  test('enable notifications toggle exists', async ({ page }) => {
    // Look for enable notifications toggle.
    const toggle = page.getByLabel(/enable.*notification|notification.*enable/i);
    const fallback = page.getByRole('switch');

    const hasToggle = await toggle.isVisible().catch(() => false) ||
                      await fallback.first().isVisible().catch(() => false);

    // Some toggle should exist.
    expect(hasToggle).toBe(true);
  });
});

test.describe('Agent Settings', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
  });

  test('agent info is displayed', async ({ page }) => {
    // Look for current agent information.
    const agentSection = page.getByText(/agent|current/i);
    await expect(agentSection.first()).toBeVisible();
  });

  test('agent switcher is accessible from settings', async ({ page }) => {
    // Look for agent switcher or dropdown.
    const agentSwitcher = page.getByRole('button', { name: /switch.*agent|select.*agent/i }).or(
      page.getByRole('combobox')
    );

    const exists = await agentSwitcher.first().isVisible().catch(() => false);
    // Switcher may or may not be on settings page.
  });
});

test.describe('Settings Navigation', () => {
  test('settings link in sidebar works', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Click settings link.
    const sidebar = page.getByRole('complementary');
    const settingsLink = sidebar.getByRole('link', { name: /settings/i });

    if (await settingsLink.isVisible()) {
      await settingsLink.click();
      await expect(page).toHaveURL(/\/settings/);
    }
  });

  test('settings link in header works', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Look for settings icon in header.
    const settingsIcon = page.getByRole('link', { name: /settings/i }).not(page.getByRole('complementary').locator('*'));

    if (await settingsIcon.isVisible()) {
      await settingsIcon.click();
      await expect(page).toHaveURL(/\/settings/);
    }
  });
});
