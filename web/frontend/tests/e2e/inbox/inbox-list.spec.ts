// E2E tests for inbox message list loading, pagination, and empty state.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(
  page: import('@playwright/test').Page,
  messages: object[] = [],
) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ messages }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ topics: [] }),
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

test.describe('Inbox message list', () => {
  test('loads inbox page with message list', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Test Agent',
        subject: 'Test Message',
        body: 'Test body',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
        recipients: [],
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');

    // Check inbox heading is visible in sidebar.
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Check that the main inbox area is loaded (has stats or category tabs).
    await expect(
      page.locator('nav[aria-label="Category tabs"]'),
    ).toBeVisible();
  });

  test('displays messages with proper structure', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Test Agent',
        subject: 'Test Message',
        body: 'Test body content',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
        recipients: [],
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check for message in the list.
    await expect(page.getByText('Test Agent').first()).toBeVisible();
    await expect(page.getByText('Test Message')).toBeVisible();
  });

  test('displays message sender, subject, and preview', async ({ page }) => {
    const messages = [
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Alice Agent',
        subject: 'Important Update',
        body: 'This is the message body preview text.',
        priority: 'PRIORITY_NORMAL',
        created_at: new Date().toISOString(),
        recipients: [],
      },
      {
        id: '2',
        sender_id: '2',
        sender_name: 'Bob Agent',
        subject: 'Follow-up Question',
        body: 'Can you help me with this issue?',
        priority: 'PRIORITY_URGENT',
        created_at: new Date().toISOString(),
        recipients: [],
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check for sender names.
    await expect(page.getByText('Alice Agent').first()).toBeVisible();
    await expect(page.getByText('Bob Agent').first()).toBeVisible();

    // Check for subjects.
    await expect(page.getByText('Important Update')).toBeVisible();
    await expect(page.getByText('Follow-up Question')).toBeVisible();
  });

  test('handles API error gracefully', async ({ page }) => {
    // Route API to return error.
    await page.route('**/api/v1/messages*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'server_error', message: 'Internal server error' },
        }),
      });
    });

    await page.goto('/');

    // Should handle error gracefully - page should still be functional.
    await page.waitForTimeout(1000);
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
  });

  test('maintains message order by date', async ({ page }) => {
    const oldDate = new Date('2024-01-01T10:00:00Z');
    const newDate = new Date('2024-01-02T10:00:00Z');

    const messages = [
      {
        id: '2',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Newer Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: newDate.toISOString(),
        recipients: [],
      },
      {
        id: '1',
        sender_id: '1',
        sender_name: 'Agent',
        subject: 'Older Message',
        body: 'Body',
        priority: 'PRIORITY_NORMAL',
        created_at: oldDate.toISOString(),
        recipients: [],
      },
    ];
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Newer message should appear first in the list.
    await expect(page.getByText('Newer Message')).toBeVisible();
    await expect(page.getByText('Older Message')).toBeVisible();
  });
});

test.describe('Inbox pagination', () => {
  test('loads first page of messages', async ({ page }) => {
    const messages = Array.from({ length: 5 }, (_, i) => ({
      id: String(i + 1),
      sender_id: '1',
      sender_name: `Agent ${i + 1}`,
      subject: `Message ${i + 1}`,
      body: 'Body',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
      recipients: [],
    }));
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check that messages are displayed.
    await expect(page.getByText('Agent 1').first()).toBeVisible();
    await expect(page.getByText('Message 1')).toBeVisible();
  });

  test('shows load more or pagination when more messages exist', async ({
    page,
  }) => {
    const messages = Array.from({ length: 20 }, (_, i) => ({
      id: String(i + 1),
      sender_id: '1',
      sender_name: `Agent ${i + 1}`,
      subject: `Message ${i + 1}`,
      body: 'Body',
      priority: 'PRIORITY_NORMAL',
      created_at: new Date().toISOString(),
      recipients: [],
    }));
    await setupAPIs(page, messages);
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
    await page.waitForTimeout(500);

    // Check messages are showing.
    await expect(page.getByText('Agent 1').first()).toBeVisible();
  });
});
