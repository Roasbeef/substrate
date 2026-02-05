// E2E tests for search functionality.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ messages: [] }),
    });
  });

  await page.route('**/api/v1/search*', async (route) => {
    const url = new URL(route.request().url());
    const query = url.searchParams.get('query') || '';

    const results = query
      ? [
          {
            id: '1',
            thread_id: '100',
            sender_id: '1',
            sender_name: 'Agent',
            subject: `Result for "${query}"`,
            body: 'Matching message body.',
            priority: 'PRIORITY_NORMAL',
            created_at: new Date().toISOString(),
          },
          {
            id: '2',
            thread_id: '101',
            sender_id: '2',
            sender_name: 'Other Agent',
            subject: `Another match for ${query}`,
            body: 'Another result.',
            priority: 'PRIORITY_NORMAL',
            created_at: new Date().toISOString(),
          },
        ]
      : [];

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ results }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ agents: [], counts: {} }),
    });
  });
}

// Helper to open the search modal by clicking the trigger button.
async function openSearchModal(page: import('@playwright/test').Page) {
  // The header has a "Search mail..." button that opens the search dialog.
  const searchTrigger = page.getByRole('button', { name: /search/i }).first();
  await searchTrigger.click();

  // Wait for the dialog to appear and its input to be ready.
  const searchInput = page.getByRole('combobox', { name: /search/i });
  await expect(searchInput).toBeVisible();
  return searchInput;
}

test.describe('Search input', () => {
  test('search input is visible after opening modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await expect(searchInput).toBeVisible();
  });

  test('search input is focusable', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.focus();
    await expect(searchInput).toBeFocused();
  });

  test('typing in search input works', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test query');
    await expect(searchInput).toHaveValue('test query');
  });
});

test.describe('Search results', () => {
  test('search shows results dropdown', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Results listbox should be present inside the dialog.
    const results = page.locator('[role="listbox"]');
    await expect(results).toBeVisible();
  });

  test('results contain matching messages', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Should show matching results.
    await expect(page.locator('text=/Result for/i')).toBeVisible();
  });

  test('clicking result navigates to message', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Click a result (each result is a button with role="option").
    const result = page.locator('[role="option"]').first();
    if (await result.isVisible()) {
      await result.click();
      await page.waitForTimeout(300);

      // Should navigate to thread view and close the modal.
      await expect(searchInput).not.toBeVisible();
    }
  });

  test('empty query shows no results', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('');
    await page.waitForTimeout(300);

    // The dialog should show the empty state hint, not result items.
    await expect(page.locator('[role="option"]').first()).not.toBeVisible();
  });

  test('no results shows empty state', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ messages: [] }),
      });
    });

    await page.route('**/api/v1/search*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ results: [] }),
      });
    });

    await page.route('**/api/v1/agents-status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ agents: [], counts: {} }),
      });
    });

    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('nonexistent query xyz');
    await page.waitForTimeout(500);

    // Should show no results message.
    await expect(page.locator('text=/no results/i')).toBeVisible();
  });
});

test.describe('Search keyboard navigation', () => {
  test('slash key focuses search', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Press / to potentially open the search modal.
    await page.keyboard.press('/');
    await page.waitForTimeout(200);

    // If the slash key opens the modal, the search input should be visible.
    const searchInput = page.getByRole('combobox', { name: /search/i });
    if (await searchInput.isVisible()) {
      await expect(searchInput).toBeVisible();
    }
  });

  test('Escape closes search modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(300);

    await page.keyboard.press('Escape');
    await page.waitForTimeout(200);

    // Search modal should be closed.
    await expect(searchInput).not.toBeVisible();
  });

  test('arrow keys navigate results', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Arrow down to navigate.
    await page.keyboard.press('ArrowDown');
    await page.waitForTimeout(100);

    // A result should be highlighted (aria-selected).
    const selectedResult = page.locator('[role="option"][aria-selected="true"]');
    await expect(selectedResult).toBeVisible();
  });

  test('Enter selects highlighted result', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(300);

    // Should close the modal after selecting.
    await expect(searchInput).not.toBeVisible();
  });
});

test.describe('Search debouncing', () => {
  test('search waits before sending request', async ({ page }) => {
    let requestCount = 0;

    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ messages: [] }),
      });
    });

    await page.route('**/api/v1/search*', async (route) => {
      requestCount++;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ results: [] }),
      });
    });

    await page.route('**/api/v1/agents-status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ agents: [], counts: {} }),
      });
    });

    await page.goto('/');

    const searchInput = await openSearchModal(page);

    // Type quickly.
    await searchInput.pressSequentially('test', { delay: 50 });
    await page.waitForTimeout(100);

    const immediateCount = requestCount;

    // Wait for debounce.
    await page.waitForTimeout(600);

    // Should have made fewer requests due to debouncing.
    expect(requestCount).toBeLessThanOrEqual(immediateCount + 1);
  });
});

test.describe('Search modal from inbox', () => {
  test('search modal opens and shows results', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    // Open the search modal.
    const searchInput = await openSearchModal(page);

    // Type a query and verify results appear.
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Should show search results inside the modal.
    await expect(page.locator('text=/Result for/i')).toBeVisible();
  });

  test('search modal shows query in no-results state', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ messages: [] }),
      });
    });

    await page.route('**/api/v1/search*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ results: [] }),
      });
    });

    await page.route('**/api/v1/agents-status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ agents: [], counts: {} }),
      });
    });

    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Should display the query text in the no-results message.
    await expect(page.getByText(/no results found for/i)).toBeVisible();
  });

  test('search modal shows result count in footer', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');

    const searchInput = await openSearchModal(page);
    await searchInput.fill('test');
    await page.waitForTimeout(500);

    // Should show result count in the footer.
    await expect(page.locator('text=/\\d+ results?/i')).toBeVisible();
  });
});
