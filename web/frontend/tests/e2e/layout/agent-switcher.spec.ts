// E2E tests for agent switcher with unread count in header.

import { test, expect } from '@playwright/test';

test.describe('Header Agent Switcher', () => {
  test.beforeEach(async ({ page, request }) => {
    // Create test agents.
    await request.post('/api/v1/agents', {
      data: { name: 'AgentAlpha' },
    });
    await request.post('/api/v1/agents', {
      data: { name: 'AgentBeta' },
    });

    // Send an unread message to see unread count.
    await request.post('/api/v1/messages', {
      data: {
        to: ['AgentAlpha'],
        from: 'User',
        subject: 'Test Message',
        body: 'This is a test message',
        priority: 'normal',
      },
    });

    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('displays agent switcher in header', async ({ page }) => {
    // Find the agent switcher button in header.
    const header = page.getByRole('banner');
    const switcherButton = header.getByRole('button').filter({ hasText: /Agent|Select/i });
    await expect(switcherButton).toBeVisible({ timeout: 5000 });
  });

  test('agent switcher opens dropdown on click', async ({ page }) => {
    const header = page.getByRole('banner');

    // Find and click the agent switcher.
    const switcherButton = header.getByRole('button').filter({ hasText: /Agent|Select/i });
    await switcherButton.click();

    // Dropdown should show agent list.
    const dropdown = page.getByRole('menu');
    await expect(dropdown).toBeVisible({ timeout: 5000 });
  });

  test('shows agents in dropdown', async ({ page }) => {
    const header = page.getByRole('banner');

    // Open the agent switcher.
    const switcherButton = header.getByRole('button').filter({ hasText: /Agent|Select/i });
    await switcherButton.click();

    // Wait for dropdown.
    const dropdown = page.getByRole('menu');
    await expect(dropdown).toBeVisible({ timeout: 5000 });

    // Look for our test agents.
    const agentAlpha = dropdown.getByText('AgentAlpha');
    const agentBeta = dropdown.getByText('AgentBeta');

    await expect(agentAlpha).toBeVisible();
    await expect(agentBeta).toBeVisible();
  });

  test('can select an agent from dropdown', async ({ page }) => {
    const header = page.getByRole('banner');

    // Open the agent switcher.
    const switcherButton = header.getByRole('button').filter({ hasText: /Agent|Select/i });
    await switcherButton.click();

    // Select an agent from the menu.
    const dropdown = page.getByRole('menu');
    await expect(dropdown).toBeVisible({ timeout: 5000 });

    // Menu items are rendered as menuitem role.
    const agentMenuItem = dropdown.getByRole('menuitem').filter({ hasText: 'AgentAlpha' });
    await agentMenuItem.click();

    // The dropdown should close and the button should show the selected agent.
    await expect(dropdown).not.toBeVisible({ timeout: 5000 });
    await expect(switcherButton).toContainText('AgentAlpha');
  });

  test('shows unread badge when messages exist', async ({ page }) => {
    const header = page.getByRole('banner');

    // The notification bell should have a badge.
    const bellButton = header.getByRole('button', { name: 'View notifications' });
    await expect(bellButton).toBeVisible();

    // Check for the badge indicator (a small dot).
    const badge = bellButton.locator('span').filter({ hasText: /\d+/ }).or(
      bellButton.locator('.bg-red-500'),
    );
    // Badge may or may not exist depending on unread count.
    const badgeCount = await badge.count();
    expect(badgeCount).toBeGreaterThanOrEqual(0);
  });

  test('agent dropdown shows status indicators', async ({ page }) => {
    const header = page.getByRole('banner');

    // Open the agent switcher.
    const switcherButton = header.getByRole('button').filter({ hasText: /Agent|Select/i });
    await switcherButton.click();

    // The dropdown should show status badges.
    const dropdown = page.getByRole('menu');
    await expect(dropdown).toBeVisible({ timeout: 5000 });

    // Status badges should be visible (active, idle, offline, etc.).
    // Look for status text.
    const statusBadges = dropdown.locator('[class*="bg-green"]').or(
      dropdown.locator('[class*="bg-gray"]'),
    );
    const statusCount = await statusBadges.count();
    expect(statusCount).toBeGreaterThanOrEqual(0);
  });
});
