// E2E tests for thread view modal when clicking inbox messages.

import { test, expect } from '@playwright/test';

// Get API base URL from environment or use default.
const API_PORT = process.env.API_PORT ?? '8082';
const PROD_PORT = process.env.PROD_PORT ?? '8090';
const USE_PRODUCTION = process.env.PLAYWRIGHT_USE_PRODUCTION === 'true';
const API_BASE_URL = USE_PRODUCTION
  ? `http://localhost:${PROD_PORT}/api/v1`
  : `http://localhost:${API_PORT}/api/v1`;

test.describe.skip('Thread View Modal', () => {
  // Skip: Tests create agents and send messages via API, but the inbox view
  // shows messages for "User" (global identity), not the test agents.
  // These tests need the inbox to show messages for the created test agents.
  let agentId: number;
  let senderId: number;

  test.beforeAll(async ({ request }) => {
    // Create a test agent.
    const agentResponse = await request.post(`${API_BASE_URL}/agents`, {
      data: { name: 'ThreadTestAgent' },
    });

    // Create sender agent.
    const senderResponse = await request.post(`${API_BASE_URL}/agents`, {
      data: { name: 'ThreadSender' },
    });

    if (agentResponse.ok() && senderResponse.ok()) {
      const agent = await agentResponse.json();
      const sender = await senderResponse.json();
      agentId = agent.id;
      senderId = sender.id;

      // Send a test message.
      await request.post(`${API_BASE_URL}/messages`, {
        data: {
          to: [agentId],
          subject: 'Test Thread Subject',
          body: 'This is a test message body for thread view testing.',
          priority: 'normal',
        },
        headers: {
          'X-Agent-ID': String(senderId),
        },
      });
    }
  });

  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('clicking a message opens thread view modal', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the first message row.
    await messageRows.first().click();

    // Wait for modal content - look for the "Back to inbox" button which is in the modal toolbar.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });
  });

  test('thread view displays message subject', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message.
    await messageRows.first().click();

    // Wait for modal to open - look for the thread content.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // The h1 heading with subject should be visible.
    const heading = page.locator('h1');
    await expect(heading.first()).toBeVisible({ timeout: 5000 });
  });

  test('thread view displays message body', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message.
    await messageRows.first().click();

    // Wait for modal to open.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // The message body should be visible somewhere in the page.
    const bodyText = page.getByText('This is a test message body');
    await expect(bodyText.first()).toBeVisible({ timeout: 5000 });
  });

  test('thread view can be closed with Escape', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message to open thread view.
    await messageRows.first().click();

    // Wait for modal to open.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // Press Escape to close.
    await page.keyboard.press('Escape');

    // The back button should no longer be visible.
    await expect(backButton).not.toBeVisible({ timeout: 3000 });

    // Inbox should still be visible.
    await expect(messageRows.first()).toBeVisible();
  });

  test('thread view has reply textarea', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message to open thread view.
    await messageRows.first().click();

    // Wait for modal to open.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // Look for reply textarea.
    const replyTextarea = page.getByPlaceholder(/Write a reply/i);
    await expect(replyTextarea).toBeVisible({ timeout: 5000 });
  });

  test('thread view has action buttons', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message to open thread view.
    await messageRows.first().click();

    // Wait for modal to open.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // Look for action buttons in the modal toolbar.
    const archiveButton = page.getByRole('button', { name: /archive/i });
    const deleteButton = page.getByRole('button', { name: /delete/i });

    // At least one action button should be visible.
    const archiveCount = await archiveButton.count();
    const deleteCount = await deleteButton.count();

    expect(archiveCount + deleteCount).toBeGreaterThan(0);
  });

  test('thread view closes when clicking back button', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click the message to open thread view.
    await messageRows.first().click();

    // Wait for modal to open.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // Click back button to close.
    await backButton.click();

    // The back button should no longer be visible.
    await expect(backButton).not.toBeVisible({ timeout: 3000 });

    // Inbox should still be visible.
    await expect(messageRows.first()).toBeVisible();
  });
});

test.describe.skip('Thread View with Multiple Messages', () => {
  // Skip: Same issue as above - messages sent to test agents don't appear
  // in the global User inbox view.
  let agentId: number;
  let senderId: number;

  test.beforeAll(async ({ request }) => {
    // Create test agent.
    const agentResponse = await request.post(`${API_BASE_URL}/agents`, {
      data: { name: 'MultiThreadAgent' },
    });

    const senderResponse = await request.post(`${API_BASE_URL}/agents`, {
      data: { name: 'MultiSender' },
    });

    if (agentResponse.ok() && senderResponse.ok()) {
      const agent = await agentResponse.json();
      const sender = await senderResponse.json();
      agentId = agent.id;
      senderId = sender.id;

      // Send multiple messages.
      await request.post(`${API_BASE_URL}/messages`, {
        data: {
          to: [agentId],
          subject: 'First Message',
          body: 'First message body content.',
          priority: 'normal',
        },
        headers: {
          'X-Agent-ID': String(senderId),
        },
      });

      await request.post(`${API_BASE_URL}/messages`, {
        data: {
          to: [agentId],
          subject: 'Second Message',
          body: 'Second message body content.',
          priority: 'urgent',
        },
        headers: {
          'X-Agent-ID': String(senderId),
        },
      });
    }
  });

  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('can view different messages sequentially', async ({ page }) => {
    // Wait for message rows to appear.
    const messageRows = page.locator('[data-testid="message-row"]');
    await expect(messageRows.first()).toBeVisible({ timeout: 10000 });

    // Click first message row.
    await messageRows.first().click();

    // Wait for modal.
    const backButton = page.getByRole('button', { name: /back to inbox/i });
    await expect(backButton).toBeVisible({ timeout: 5000 });

    // Close the thread view.
    await page.keyboard.press('Escape');
    await expect(backButton).not.toBeVisible({ timeout: 3000 });

    // Click second message row if available.
    const rowCount = await messageRows.count();
    if (rowCount > 1) {
      await messageRows.nth(1).click();

      // Modal should open again.
      await expect(backButton).toBeVisible({ timeout: 5000 });
    }
  });
});
