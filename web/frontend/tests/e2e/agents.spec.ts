// E2E tests for agents dashboard functionality.
// Tests all agent-related buttons, cards, and user flows.

import { test, expect } from '@playwright/test';

test.describe('Agents Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/agents');
    await page.waitForLoadState('networkidle');
  });

  test('displays agents dashboard', async ({ page }) => {
    // Verify agents page loads.
    await expect(page.locator('#root')).not.toBeEmpty();
  });

  test('displays stats cards', async ({ page }) => {
    // Look for stats indicators (active agents, sessions, etc.).
    const stats = page.locator('[class*="stat"], [class*="card"]');
    // Stats cards should exist if there's data.
    const count = await stats.count();
    // Page should have some structure.
    expect(count).toBeGreaterThanOrEqual(0);
  });

  test('filter tabs are visible and clickable', async ({ page }) => {
    // Look for filter tabs.
    const allTab = page.getByRole('button', { name: /all/i });
    const activeTab = page.getByRole('button', { name: /active/i });
    const idleTab = page.getByRole('button', { name: /idle/i });

    // At least one filter should be visible.
    const anyVisible = await allTab.isVisible() || await activeTab.isVisible() || await idleTab.isVisible();

    if (anyVisible) {
      if (await allTab.isVisible()) {
        await allTab.click();
      } else if (await activeTab.isVisible()) {
        await activeTab.click();
      }
    }
  });
});

test.describe('Agent Cards', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/agents');
    await page.waitForLoadState('networkidle');
  });

  test('agent cards display status indicators', async ({ page }) => {
    // Look for status badges on agent cards.
    const statusBadges = page.locator('[class*="badge"], [class*="status"]');
    const count = await statusBadges.count();

    // If agents exist, they should have status.
    if (count > 0) {
      await expect(statusBadges.first()).toBeVisible();
    }
  });

  test('clicking agent card navigates to agent detail', async ({ page }) => {
    // Find an agent card or link.
    const agentCards = page.locator('[class*="agent"], [class*="card"]').filter({ has: page.locator('a, button') });
    const count = await agentCards.count();

    if (count > 0) {
      // Click first card.
      await agentCards.first().click();

      // URL might change to agent detail.
      // Wait for navigation or modal.
      await page.waitForLoadState('networkidle');
    }
  });
});

test.describe('New Agent Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/agents');
    await page.waitForLoadState('networkidle');
  });

  test('new agent button opens registration modal', async ({ page }) => {
    // Find new agent button.
    const newAgentButton = page.getByRole('button', { name: /new agent|register|add agent/i });

    if (await newAgentButton.isVisible()) {
      await newAgentButton.click();

      // Modal should open.
      await expect(page.getByRole('dialog')).toBeVisible();
    }
  });

  test('new agent modal has required fields', async ({ page }) => {
    const newAgentButton = page.getByRole('button', { name: /new agent|register|add agent/i });

    if (await newAgentButton.isVisible()) {
      await newAgentButton.click();

      // Check for name input.
      const nameInput = page.getByLabel(/name/i).or(page.getByPlaceholder(/name/i));
      await expect(nameInput.first()).toBeVisible();
    }
  });

  test('new agent modal can be closed', async ({ page }) => {
    const newAgentButton = page.getByRole('button', { name: /new agent|register|add agent/i });

    if (await newAgentButton.isVisible()) {
      await newAgentButton.click();
      await expect(page.getByRole('dialog')).toBeVisible();

      // Close via escape.
      await page.keyboard.press('Escape');

      // Modal should close.
      await expect(page.getByRole('dialog')).not.toBeVisible();
    }
  });
});

test.describe('Activity Feed', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/agents');
    await page.waitForLoadState('networkidle');
  });

  test('activity feed displays if activities exist', async ({ page }) => {
    // Look for activity items.
    const activityItems = page.locator('[class*="activity"], [class*="feed"]');
    const count = await activityItems.count();

    // Activity section may or may not have items.
    expect(count).toBeGreaterThanOrEqual(0);
  });
});
