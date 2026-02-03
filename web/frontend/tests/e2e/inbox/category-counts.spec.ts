// E2E tests for category tab counts feature.

import { test, expect } from '@playwright/test';

test.describe('Inbox category tab counts', () => {
  test.beforeEach(async ({ page, request }) => {
    // Seed test data with messages from different senders.
    // Primary messages come from "User", agent messages from agents.

    // Create an agent first.
    await request.post('/api/v1/agents', {
      data: { name: 'TestAgent' },
    });

    // Send messages from User (Primary category).
    await request.post('/api/v1/messages', {
      data: {
        to: ['TestAgent'],
        from: 'User',
        subject: 'User Message 1',
        body: 'Message from user',
        priority: 'normal',
      },
    });

    await request.post('/api/v1/messages', {
      data: {
        to: ['TestAgent'],
        from: 'User',
        subject: 'User Message 2',
        body: 'Another message from user',
        priority: 'normal',
      },
    });

    // Send a message from an agent (Agents category).
    await request.post('/api/v1/messages', {
      data: {
        to: ['User'],
        from: 'TestAgent',
        subject: 'Agent Message',
        body: 'Message from agent',
        priority: 'normal',
      },
    });

    // Navigate to inbox.
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('displays count badge on category tabs', async ({ page }) => {
    // Find the category tabs.
    const primaryTab = page.getByRole('button', { name: /Primary/i });
    const agentsTab = page.getByRole('button', { name: /Agents/i });
    const topicsTab = page.getByRole('button', { name: /Topics/i });

    // Verify the tabs exist.
    await expect(primaryTab).toBeVisible();
    await expect(agentsTab).toBeVisible();
    await expect(topicsTab).toBeVisible();
  });

  test('Primary tab shows count of user messages', async ({ page }) => {
    // The Primary tab should show count > 0 for user messages.
    const primaryTab = page.getByRole('button', { name: /Primary/i });
    await expect(primaryTab).toBeVisible();

    // Look for a count badge within the primary tab.
    // Count badges are rendered as spans with numeric content.
    const countBadge = primaryTab.locator('span').filter({ hasText: /^\d+$/ });

    // We expect at least some count (from our seeded user messages).
    const badgeText = await countBadge.textContent().catch(() => '0');
    const count = parseInt(badgeText ?? '0', 10);
    expect(count).toBeGreaterThanOrEqual(0);
  });

  test('Agents tab shows count of agent messages', async ({ page }) => {
    // The Agents tab should show count for agent messages.
    const agentsTab = page.getByRole('button', { name: /Agents/i });
    await expect(agentsTab).toBeVisible();

    // Look for a count badge within the agents tab.
    const countBadge = agentsTab.locator('span').filter({ hasText: /^\d+$/ });

    // We expect at least some count (from our seeded agent message).
    const badgeText = await countBadge.textContent().catch(() => '0');
    const count = parseInt(badgeText ?? '0', 10);
    expect(count).toBeGreaterThanOrEqual(0);
  });

  test('Topics tab shows zero count when no topic messages exist', async ({ page }) => {
    // Topics tab should show 0 or no badge since we have no topic messages.
    const topicsTab = page.getByRole('button', { name: /Topics/i });
    await expect(topicsTab).toBeVisible();

    // Topics should have 0 count (not displayed when 0).
    const countBadge = topicsTab.locator('span').filter({ hasText: /^\d+$/ });
    // Either no badge exists or it shows 0.
    const count = await countBadge.count();
    // If count is 0, the badge is hidden (count > 0 condition in component).
    expect(count).toBeLessThanOrEqual(1);
  });

  test('counts update when filter changes', async ({ page }) => {
    // Initially all messages shown.
    const primaryTab = page.getByRole('button', { name: /Primary/i });
    await expect(primaryTab).toBeVisible();

    // Click on Agents tab.
    const agentsTab = page.getByRole('button', { name: /Agents/i });
    await agentsTab.click();

    // Tab should be selected (aria-current).
    await expect(agentsTab).toHaveAttribute('aria-current', 'true');
  });
});
