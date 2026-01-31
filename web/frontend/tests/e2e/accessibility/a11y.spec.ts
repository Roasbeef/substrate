// Accessibility tests using axe-core via Playwright.

import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Helper to setup API mocks.
async function setupAPIMocks(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Test Agent',
            subject: 'Test Message',
            body: 'Test body content',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          },
        ],
        meta: { total: 1, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [],
        meta: { total: 0, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [],
        meta: { total: 0, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [],
        meta: { total: 0, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/sessions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [],
        meta: { total: 0, page: 1, page_size: 20 },
      }),
    });
  });
}

test.describe('Accessibility', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('inbox page has no accessibility violations', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('agents page has no accessibility violations', async ({ page }) => {
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('sessions page has no accessibility violations', async ({ page }) => {
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('settings page has no accessibility violations', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForTimeout(500);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });

  test('compose modal has no accessibility violations', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Open compose modal.
    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .include('[role="dialog"]')
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();

    expect(accessibilityScanResults.violations).toEqual([]);
  });
});

test.describe('Keyboard accessibility', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('all interactive elements are keyboard accessible', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Tab through the page and check focus visibility.
    let focusableCount = 0;
    const maxTabs = 50;

    for (let i = 0; i < maxTabs; i++) {
      await page.keyboard.press('Tab');
      const focusedElement = page.locator(':focus');

      if (await focusedElement.count() > 0) {
        focusableCount++;

        // Check that focused element is visible.
        await expect(focusedElement).toBeVisible();
      }
    }

    // Should have multiple focusable elements.
    expect(focusableCount).toBeGreaterThan(5);
  });

  test('escape key closes modals', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Open compose modal.
    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();

    // Press escape.
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);

    await expect(modal).not.toBeVisible();
  });
});

test.describe('Color contrast', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('text has sufficient color contrast', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    const accessibilityScanResults = await new AxeBuilder({ page })
      .withTags(['wcag2aa'])
      .options({ runOnly: ['color-contrast'] })
      .analyze();

    // Log any violations for debugging.
    if (accessibilityScanResults.violations.length > 0) {
      console.log('Color contrast violations:', JSON.stringify(accessibilityScanResults.violations, null, 2));
    }

    expect(accessibilityScanResults.violations).toEqual([]);
  });
});

test.describe('ARIA attributes', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('interactive elements have proper ARIA labels', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Check that buttons have accessible names.
    const buttons = page.locator('button');
    const buttonCount = await buttons.count();

    for (let i = 0; i < Math.min(buttonCount, 10); i++) {
      const button = buttons.nth(i);
      const accessibleName = await button.evaluate((el) => {
        return el.getAttribute('aria-label') ||
               el.textContent?.trim() ||
               el.getAttribute('title') ||
               '';
      });

      // Button should have some accessible name.
      expect(accessibleName.length).toBeGreaterThan(0);
    }
  });

  test('modals have proper ARIA attributes', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Open compose modal.
    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();

    // Check for aria-modal or aria-label.
    const hasAriaModal = await modal.evaluate((el) =>
      el.getAttribute('aria-modal') === 'true' || el.hasAttribute('aria-labelledby')
    );

    expect(hasAriaModal).toBe(true);
  });
});
