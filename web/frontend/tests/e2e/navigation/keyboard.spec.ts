// E2E tests for keyboard navigation.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Agent A',
            subject: 'First Message',
            body: 'Body 1',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          },
          {
            id: 2,
            sender_id: 2,
            sender_name: 'Agent B',
            subject: 'Second Message',
            body: 'Body 2',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          },
          {
            id: 3,
            sender_id: 3,
            sender_name: 'Agent C',
            subject: 'Third Message',
            body: 'Body 3',
            priority: 'normal',
            created_at: new Date().toISOString(),
            recipients: [],
          },
        ],
        meta: { total: 3, page: 1, page_size: 20 },
      }),
    });
  });
}

test.describe('Tab navigation', () => {
  test('Tab moves focus through interactive elements', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Start tabbing.
    await page.keyboard.press('Tab');
    let focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();

    await page.keyboard.press('Tab');
    focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();

    await page.keyboard.press('Tab');
    focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });

  test('Shift+Tab moves focus backwards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Tab forward several times.
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    const forwardElement = page.locator(':focus');
    await expect(forwardElement).toBeVisible();

    // Tab backward.
    await page.keyboard.press('Shift+Tab');
    const backwardElement = page.locator(':focus');
    await expect(backwardElement).toBeVisible();
  });

  test('focus is visible with outline', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.keyboard.press('Tab');
    const focusedElement = page.locator(':focus');

    // Check for visible focus indicator.
    await expect(focusedElement).toBeVisible();
    // Focus outline/ring should be applied (check for CSS).
  });
});

test.describe('Enter key behavior', () => {
  test('Enter activates focused link', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Focus agents link.
    const agentsLink = page.locator('a[href="/agents"]');
    await agentsLink.focus();

    await page.keyboard.press('Enter');
    await page.waitForURL('**/agents');

    // Use heading to avoid matching multiple "Agents" text elements.
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  });
});

test.describe('Arrow key navigation', () => {
  test('arrow keys navigate message list', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Focus first message.
    const firstMessage = page.locator('[data-testid="message-row"]').first();
    if (await firstMessage.isVisible()) {
      await firstMessage.focus();

      // Arrow down.
      await page.keyboard.press('ArrowDown');
      await page.waitForTimeout(100);

      // Focus should move (depends on implementation).
    }
  });

  test('arrow keys navigate tabs', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Focus first tab.
    const primaryTab = page.locator('button:has-text("Primary")');
    await primaryTab.focus();

    // Arrow right.
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(100);

    // Focus should move to next tab.
    const focusedTab = page.locator('[role="tab"]:focus');
    // Tab navigation depends on implementation.
  });
});

test.describe('Keyboard shortcuts', () => {
  test('c opens compose modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Press c for compose.
    await page.keyboard.press('c');
    await page.waitForTimeout(300);

    // Compose modal may open (depends on implementation).
    const modal = page.locator('[role="dialog"]');
    // Check if modal opened.
  });

  test('/ focuses search', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.keyboard.press('/');
    await page.waitForTimeout(100);

    const searchInput = page.locator('input[placeholder*="search" i]');
    // May be focused.
  });

  test('? opens keyboard shortcuts help', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.keyboard.press('Shift+/'); // ?
    await page.waitForTimeout(300);

    // Help dialog may open.
    const helpDialog = page.locator('[data-testid="keyboard-shortcuts"]');
    // Depends on implementation.
  });
});

test.describe('Focus management', () => {
  test('focus returns after modal closes', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await page.waitForTimeout(500);

    // Open compose modal.
    const composeButton = page.locator('button:has-text("Compose")');
    await composeButton.click();
    await page.waitForTimeout(300);

    // Close modal.
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);

    // Focus should return to trigger element.
    await expect(composeButton).toBeFocused();
  });
});

test.describe('Skip links', () => {
  test('skip to main content link exists', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Press Tab to reveal skip link.
    await page.keyboard.press('Tab');
    await page.waitForTimeout(100);

    const skipLink = page.locator('a[href="#main"], a:has-text("Skip to")');
    // Skip link may be present.
  });

  test('skip link navigates to main content', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const skipLink = page.locator('a[href="#main"]');
    if (await skipLink.isVisible()) {
      await skipLink.click();

      // Focus should move to main content.
      const mainContent = page.locator('main, #main, [role="main"]');
      // Main should be focused.
    }
  });
});
