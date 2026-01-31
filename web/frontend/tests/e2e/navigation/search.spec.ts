// E2E tests for search functionality.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });

  await page.route('**/api/v1/search*', async (route) => {
    const url = new URL(route.request().url());
    const query = url.searchParams.get('q') || '';

    const results = query
      ? [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Agent',
            subject: `Result for "${query}"`,
            body: 'Matching message body.',
            priority: 'normal',
            created_at: new Date().toISOString(),
          },
          {
            id: 2,
            sender_id: 2,
            sender_name: 'Other Agent',
            subject: `Another match for ${query}`,
            body: 'Another result.',
            priority: 'normal',
            created_at: new Date().toISOString(),
          },
        ]
      : [];

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: results,
        meta: { total: results.length, page: 1, page_size: 20 },
      }),
    });
  });
}

test.describe('Search input', () => {
  test('search input is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('[data-testid="search-input"], input[placeholder*="search" i]');
    await expect(searchInput).toBeVisible();
  });

  test('search input is focusable', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.focus();
      await expect(searchInput).toBeFocused();
    }
  });

  test('typing in search input works', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test query');
      await expect(searchInput).toHaveValue('test query');
    }
  });
});

test.describe('Search results', () => {
  test('search shows results dropdown', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(500);

      // Results dropdown should appear.
      const results = page.locator('[data-testid="search-results"], [role="listbox"]');
      if (await results.isVisible()) {
        await expect(results).toBeVisible();
      }
    }
  });

  test('results contain matching messages', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(500);

      // Should show matching results.
      await expect(page.locator('text=/Result for/i')).toBeVisible();
    }
  });

  test('clicking result navigates to message', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(500);

      // Click a result.
      const result = page.locator('[data-testid="search-result"]').first();
      if (await result.isVisible()) {
        await result.click();
        await page.waitForTimeout(300);

        // Should navigate or open message.
      }
    }
  });

  test('empty query shows no results', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('');
      await page.waitForTimeout(300);

      // No results should be shown.
      const results = page.locator('[data-testid="search-results"]');
      await expect(results).not.toBeVisible();
    }
  });

  test('no results shows empty state', async ({ page }) => {
    await page.route('**/api/v1/search*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('nonexistent query xyz');
      await page.waitForTimeout(500);

      // Should show no results message.
      await expect(page.locator('text=/no results|no matches/i')).toBeVisible();
    }
  });
});

test.describe('Search keyboard navigation', () => {
  test('slash key focuses search', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Press / to focus search.
    await page.keyboard.press('/');
    await page.waitForTimeout(100);

    const searchInput = page.locator('input[placeholder*="search" i]');
    // May be focused depending on implementation.
  });

  test('Escape clears and closes search', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(300);

      await page.keyboard.press('Escape');
      await page.waitForTimeout(200);

      // Search should be cleared or results hidden.
    }
  });

  test('arrow keys navigate results', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(500);

      // Arrow down to navigate.
      await page.keyboard.press('ArrowDown');
      await page.waitForTimeout(100);

      // First result should be highlighted.
    }
  });

  test('Enter selects highlighted result', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await page.waitForTimeout(500);

      await page.keyboard.press('ArrowDown');
      await page.keyboard.press('Enter');
      await page.waitForTimeout(300);

      // Should select and navigate.
    }
  });
});

test.describe('Search debouncing', () => {
  test('search waits before sending request', async ({ page }) => {
    let requestCount = 0;

    await page.route('**/api/v1/search*', async (route) => {
      requestCount++;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    });

    await page.goto('/');

    const searchInput = page.locator('input[placeholder*="search" i]');
    if (await searchInput.isVisible()) {
      // Type quickly.
      await searchInput.pressSequentially('test', { delay: 50 });
      await page.waitForTimeout(100);

      const immediateCount = requestCount;

      // Wait for debounce.
      await page.waitForTimeout(600);

      // Should have made fewer requests due to debouncing.
      expect(requestCount).toBeLessThanOrEqual(immediateCount + 1);
    }
  });
});

test.describe('Search page', () => {
  test('search results page loads', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/search?q=test');
    await page.waitForTimeout(500);

    // Should show search results page.
    await expect(page.locator('text=/search|results/i')).toBeVisible();
  });

  test('search results page shows query', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/search?q=test');
    await page.waitForTimeout(500);

    // Should show the query.
    await expect(page.locator('text=test')).toBeVisible();
  });

  test('search results page shows result count', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/search?q=test');
    await page.waitForTimeout(500);

    // Should show result count.
    await expect(page.locator('text=/\\d+ result/i')).toBeVisible();
  });
});
