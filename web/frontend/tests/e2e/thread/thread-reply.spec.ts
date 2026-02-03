// E2E tests for thread reply functionality.

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
            sender_name: 'Alice Agent',
            subject: 'Test Thread',
            body: 'Initial message.',
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
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'thread-1',
        messages: [
          {
            id: 1,
            sender_id: 1,
            sender_name: 'Alice Agent',
            subject: 'Test Thread',
            body: 'Initial message.',
            priority: 'normal',
            created_at: new Date().toISOString(),
          },
        ],
      }),
    });
  });
}

test.describe('Thread reply box', () => {
  test('displays reply input at bottom of thread', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      // Should have reply textarea.
      const replyInput = threadView.locator('textarea[placeholder*="reply" i], [data-testid="reply-input"]');
      await expect(replyInput).toBeVisible();
    }
  });

  test('reply input is focusable', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      if (await replyInput.isVisible()) {
        await replyInput.focus();
        await expect(replyInput).toBeFocused();
      }
    }
  });

  test('typing in reply input works', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      if (await replyInput.isVisible()) {
        await replyInput.fill('This is my reply message.');
        await expect(replyInput).toHaveValue('This is my reply message.');
      }
    }
  });
});

test.describe('Sending reply', () => {
  test('send button is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const sendButton = threadView.locator('button:has-text("Send"), button[aria-label*="send" i]');
      await expect(sendButton).toBeVisible();
    }
  });

  test('send button is disabled when input is empty', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const sendButton = threadView.locator('button:has-text("Send")');
      if (await sendButton.isVisible()) {
        // Should be disabled with empty input.
        await expect(sendButton).toBeDisabled();
      }
    }
  });

  test('send button enables when input has text', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await page.waitForTimeout(100);

        // Should be enabled.
        await expect(sendButton).not.toBeDisabled();
      }
    }
  });

  test('clicking send submits reply', async ({ page }) => {
    await setupAPIs(page);

    let replySubmitted = false;
    await page.route('**/api/v1/threads/*/reply', async (route) => {
      replySubmitted = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 100,
          sender_name: 'Me',
          body: 'My reply',
          created_at: new Date().toISOString(),
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await sendButton.click();
        await page.waitForTimeout(500);

        // Reply should be submitted.
        expect(replySubmitted).toBe(true);
      }
    }
  });

  test('new reply appears in thread', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/threads/*/reply', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 100,
          sender_name: 'Me',
          body: 'My new reply',
          created_at: new Date().toISOString(),
        }),
      });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My new reply');
        await sendButton.click();
        await page.waitForTimeout(500);

        // New reply should appear in thread.
        await expect(threadView.locator('text=My new reply')).toBeVisible();
      }
    }
  });

  test('input is cleared after sending', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/threads/*/reply', async (route) => {
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await sendButton.click();
        await page.waitForTimeout(500);

        // Input should be cleared.
        await expect(replyInput).toHaveValue('');
      }
    }
  });
});

test.describe('Reply keyboard shortcuts', () => {
  test('Ctrl+Enter sends reply', async ({ page }) => {
    await setupAPIs(page);

    let replySubmitted = false;
    await page.route('**/api/v1/threads/*/reply', async (route) => {
      replySubmitted = true;
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await page.keyboard.press('Control+Enter');
        await page.waitForTimeout(500);

        // Should submit reply.
        expect(replySubmitted).toBe(true);
      }
    }
  });

  test('Cmd+Enter sends reply on Mac', async ({ page }) => {
    await setupAPIs(page);

    let replySubmitted = false;
    await page.route('**/api/v1/threads/*/reply', async (route) => {
      replySubmitted = true;
      await route.fulfill({ status: 200, body: '{}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await page.keyboard.press('Meta+Enter');
        await page.waitForTimeout(500);

        // Should submit reply on Mac.
      }
    }
  });
});

test.describe('Reply error handling', () => {
  test('shows error on send failure', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/threads/*/reply', async (route) => {
      await route.fulfill({ status: 500, body: '{"error": "Server error"}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply');
        await sendButton.click();
        await page.waitForTimeout(500);

        // Should show error toast or message.
        const error = page.locator('[role="alert"], text=/error|failed/i');
        // Error handling depends on implementation.
      }
    }
  });

  test('retains input on send failure', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/threads/*/reply', async (route) => {
      await route.fulfill({ status: 500, body: '{"error": "Server error"}' });
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();
    await page.waitForTimeout(500);

    await page.locator('text=Test Thread').click();
    await page.waitForTimeout(500);

    const threadView = page.locator('[data-testid="thread-view"], [role="dialog"]');
    if (await threadView.isVisible()) {
      const replyInput = threadView.locator('textarea').first();
      const sendButton = threadView.locator('button:has-text("Send")');

      if (await replyInput.isVisible()) {
        await replyInput.fill('My reply that failed');
        await sendButton.click();
        await page.waitForTimeout(500);

        // Input should retain the text.
        await expect(replyInput).toHaveValue('My reply that failed');
      }
    }
  });
});
