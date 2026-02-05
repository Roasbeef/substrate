// Accessibility tests using axe-core via Playwright.

import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

// Helper to setup API mocks with grpc-gateway format.
async function setupAPIMocks(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        messages: [
          {
            id: '1',
            sender_id: '1',
            sender_name: 'Test Agent',
            subject: 'Test Message',
            body: 'Test body content',
            priority: 'PRIORITY_NORMAL',
            created_at: new Date().toISOString(),
          },
        ],
      }),
    });
  });

  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [],
      }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        topics: [],
      }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        activities: [],
      }),
    });
  });

  await page.route('**/api/v1/sessions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        sessions: [],
      }),
    });
  });
}


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
});
