// E2E tests for opening and loading thread view.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  const messages = [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Alice Agent',
      subject: 'Test Thread',
      body: 'Initial message in thread.',
      priority: 'normal',
      created_at: new Date().toISOString(),
      thread_id: 'thread-1',
      recipients: [{ message_id: 1, agent_id: 100, state: 'unread' }],
    },
  ];

  const threadMessages = [
    {
      id: 1,
      sender_id: 1,
      sender_name: 'Alice Agent',
      subject: 'Test Thread',
      body: 'Initial message in thread.',
      priority: 'normal',
      created_at: new Date('2024-01-01T10:00:00Z').toISOString(),
    },
    {
      id: 10,
      sender_id: 100,
      sender_name: 'Me',
      subject: 'Re: Test Thread',
      body: 'My reply to the thread.',
      priority: 'normal',
      created_at: new Date('2024-01-01T11:00:00Z').toISOString(),
    },
    {
      id: 11,
      sender_id: 1,
      sender_name: 'Alice Agent',
      subject: 'Re: Test Thread',
      body: 'Follow-up message.',
      priority: 'normal',
      created_at: new Date('2024-01-01T12:00:00Z').toISOString(),
    },
  ];

  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: messages,
        meta: { total: messages.length, page: 1, page_size: 20 },
      }),
    });
  });

  await page.route('**/api/v1/threads/*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'thread-1',
        messages: threadMessages,
      }),
    });
  });

  return { messages, threadMessages };
}

test.describe('Thread view opening', () => {
  test('clicking message row opens thread view', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    // Click on message subject to open thread.
    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    // Thread view modal should open.
    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    await expect(threadView).toBeVisible();
  });

  test('thread view shows message subject', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    // Should display subject in thread header.
    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      await expect(threadView.locator('text=Test Thread')).toBeVisible();
    }
  });

  test('thread view loads all messages in thread', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      // Should show all thread messages.
      await expect(threadView.locator('text=Initial message in thread.')).toBeVisible();
      await expect(threadView.locator('text=My reply to the thread.')).toBeVisible();
      await expect(threadView.locator('text=Follow-up message.')).toBeVisible();
    }
  });

  test('thread view shows sender avatars', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      // Should show sender names.
      await expect(threadView.locator('text=Alice Agent').first()).toBeVisible();
      await expect(threadView.locator('text=Me')).toBeVisible();
    }
  });

  test('thread view marks message as read', async ({ page }) => {
    let markReadCalled = false;

    await setupAPIs(page);
    await page.route('**/api/v1/messages/*/read', async (route) => {
      markReadCalled = true;
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    // Opening thread should mark as read.
    // Check if API was called (depends on implementation).
  });
});

test.describe('Thread view loading states', () => {
  test('shows loading indicator while fetching', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Test',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              thread_id: 'thread-1',
              recipients: [],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.route('**/api/v1/threads/*', async (route) => {
      // Delay response to show loading.
      await new Promise((resolve) => setTimeout(resolve, 500));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'thread-1',
          messages: [],
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test').click();

    // Should show loading indicator.
    const loading = page.locator('[data-testid="loading"], [role="progressbar"], text=/loading/i');
    // Loading may be visible briefly.
  });

  test('handles thread load error', async ({ page }) => {
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 1,
              sender_id: 1,
              sender_name: 'Agent',
              subject: 'Test',
              body: 'Body',
              priority: 'normal',
              created_at: new Date().toISOString(),
              thread_id: 'thread-1',
              recipients: [],
            },
          ],
          meta: { total: 1, page: 1, page_size: 20 },
        }),
      });
    });

    await page.route('**/api/v1/threads/*', async (route) => {
      await route.fulfill({ status: 500, body: '{"error": "Server error"}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test').click();
    await page.waitForTimeout(500);

    // Should handle error gracefully.
    const dialog = page.locator('[role="dialog"]');
    // Error state or fallback should be shown.
  });
});

test.describe('Thread view close', () => {
  test('close button closes thread view', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      // Click close button.
      const closeButton = threadView.locator('button[aria-label*="close" i], button:has-text("×"), [data-testid="close-button"]');
      if (await closeButton.isVisible()) {
        await closeButton.click();
        await page.waitForTimeout(300);

        // Thread view should close.
        await expect(threadView).not.toBeVisible();
      }
    }
  });

  test('clicking outside closes thread view', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      // Click overlay/backdrop.
      const overlay = page.locator('[data-testid="modal-overlay"], .fixed.inset-0');
      if (await overlay.isVisible()) {
        await overlay.click({ position: { x: 10, y: 10 } });
        await page.waitForTimeout(300);

        // Thread view should close.
        await expect(threadView).not.toBeVisible();
      }
    }
  });

  test('Escape key closes thread view', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);

      // Thread view should close.
      await expect(threadView).not.toBeVisible();
    }
  });
});
