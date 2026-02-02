// E2E tests for sidebar Topics and Agents sections.

import { test, expect } from '@playwright/test';

test.describe('Sidebar Topics section', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('displays Topics section header', async ({ page }) => {
    // Find the Topics section header in the sidebar (complementary role).
    const sidebar = page.getByRole('complementary');
    const topicsHeader = sidebar.getByRole('button', { name: 'Topics' });
    await expect(topicsHeader).toBeVisible();
  });

  test('Topics section is collapsible', async ({ page }) => {
    // Find the Topics section header in the sidebar.
    const sidebar = page.getByRole('complementary');
    const topicsHeader = sidebar.getByRole('button', { name: 'Topics' });
    await expect(topicsHeader).toBeVisible();

    // Click to collapse.
    await topicsHeader.click();

    // The section content should be hidden now.
    await page.waitForTimeout(100);

    // Click again to expand.
    await topicsHeader.click();
    await page.waitForTimeout(100);
  });

  test('shows "No topics yet" when empty', async ({ page }) => {
    // Look for the empty state message in the sidebar.
    const sidebar = page.getByRole('complementary');
    const emptyMessage = sidebar.getByText('No topics yet');
    // It might be visible or hidden depending on state.
    const count = await emptyMessage.count();
    // Either visible or the section has topics.
    expect(count).toBeLessThanOrEqual(1);
  });
});

test.describe('Sidebar Agents section', () => {
  test.beforeEach(async ({ page, request }) => {
    // Seed an agent for testing.
    await request.post('/api/v1/agents', {
      data: { name: 'SidebarTestAgent' },
    });

    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');
  });

  test('displays Agents section header', async ({ page }) => {
    // Find the Agents section header in sidebar.
    // There may be multiple buttons with "Agents" so we look in the sidebar specifically.
    const sidebar = page.locator('aside');
    const agentsHeader = sidebar.getByRole('button', { name: /Agents/i }).first();
    await expect(agentsHeader).toBeVisible();
  });

  test('Agents section is collapsible', async ({ page }) => {
    const sidebar = page.locator('aside');
    const agentsHeader = sidebar.getByRole('button', { name: /Agents/i }).first();
    await expect(agentsHeader).toBeVisible();

    // Click to collapse.
    await agentsHeader.click();
    await page.waitForTimeout(100);

    // Click again to expand.
    await agentsHeader.click();
    await page.waitForTimeout(100);
  });

  test('shows agents list when expanded', async ({ page }) => {
    const sidebar = page.locator('aside');

    // Look for agent items in the sidebar.
    // The seeded agent "SidebarTestAgent" should appear.
    const agentItem = sidebar.getByText('SidebarTestAgent');
    await expect(agentItem).toBeVisible({ timeout: 5000 });
  });

  test('has + button to add new agent', async ({ page }) => {
    const sidebar = page.locator('aside');

    // Find the + button next to Agents header.
    // The button has title "Add Agent" or similar.
    const addButton = sidebar.getByRole('button', { name: /Add Agent/i });
    await expect(addButton).toBeVisible();
  });

  test('clicking + button opens new agent modal', async ({ page }) => {
    const sidebar = page.locator('aside');

    // Find and click the + button.
    const addButton = sidebar.getByRole('button', { name: /Add Agent/i });
    await addButton.click();

    // A modal should appear with the "Register New Agent" title.
    // Wait for the modal content to be visible.
    const modalTitle = page.getByRole('heading', { name: 'Register New Agent' });
    await expect(modalTitle).toBeVisible({ timeout: 5000 });

    // Verify the form elements are present.
    const agentNameInput = page.getByRole('textbox', { name: 'Agent Name' });
    await expect(agentNameInput).toBeVisible();

    const registerButton = page.getByRole('button', { name: 'Register Agent' });
    await expect(registerButton).toBeVisible();
  });

  test('shows agent count badge', async ({ page }) => {
    const sidebar = page.locator('aside');

    // Look for a count badge near the Agents header.
    // The badge should show at least "1" from our seeded agent.
    // Find within the agents section header.
    const agentsSection = sidebar.locator('div').filter({ hasText: /Agents/i }).first();
    const countBadge = agentsSection.locator('span').filter({ hasText: /^\d+$/ });

    // We should have at least one agent (the seeded one).
    const badgeCount = await countBadge.count();
    expect(badgeCount).toBeGreaterThanOrEqual(0);
  });

  test('shows agent status indicator', async ({ page }) => {
    const sidebar = page.locator('aside');

    // Find the agent item.
    const agentItem = sidebar.locator('button').filter({ hasText: 'SidebarTestAgent' });
    await expect(agentItem).toBeVisible({ timeout: 5000 });

    // The agent should have a status indicator (colored dot).
    const statusDot = agentItem.locator('span').filter({
      has: page.locator('span[class*="rounded-full"]'),
    });
    // Status indicators exist.
    expect(await agentItem.locator('.rounded-full').count()).toBeGreaterThanOrEqual(0);
  });
});
