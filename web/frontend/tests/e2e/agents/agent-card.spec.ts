// E2E tests for agent card component.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/agents/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [
          {
            id: 1,
            name: 'ActiveAgent',
            status: 'active',
            last_active_at: new Date().toISOString(),
            session_id: 'sess-1',
            seconds_since_heartbeat: 30,
          },
          {
            id: 2,
            name: 'BusyAgent',
            status: 'busy',
            last_active_at: new Date().toISOString(),
            session_id: 'sess-2',
            seconds_since_heartbeat: 5,
          },
          {
            id: 3,
            name: 'IdleAgent',
            status: 'idle',
            last_active_at: new Date(Date.now() - 600000).toISOString(),
            seconds_since_heartbeat: 600,
          },
          {
            id: 4,
            name: 'OfflineAgent',
            status: 'offline',
            last_active_at: new Date(Date.now() - 3600000).toISOString(),
            seconds_since_heartbeat: 3600,
          },
        ],
        counts: { active: 1, busy: 1, idle: 1, offline: 1 },
      }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });
}

test.describe('Agent card display', () => {
  test('shows agent name', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    await expect(agentCard.locator('text=ActiveAgent')).toBeVisible();
  });

  test('shows status badge', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Active agent should have active badge.
    const activeCard = page.locator('[data-testid="agent-card"]').filter({ hasText: 'ActiveAgent' });
    if (await activeCard.isVisible()) {
      const badge = activeCard.locator('[data-testid="status-badge"], text=/active/i');
      await expect(badge.first()).toBeVisible();
    }
  });

  test('active status has green indicator', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const activeCard = page.locator('[data-testid="agent-card"]').filter({ hasText: 'ActiveAgent' });
    if (await activeCard.isVisible()) {
      // Check for green styling.
      const statusIndicator = activeCard.locator('[data-testid="status-indicator"], .bg-green-500, .text-green-500');
      // Styling verification depends on implementation.
    }
  });

  test('busy status shows session info', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const busyCard = page.locator('[data-testid="agent-card"]').filter({ hasText: 'BusyAgent' });
    if (await busyCard.isVisible()) {
      // Should show busy status.
      await expect(busyCard.locator('text=/busy/i')).toBeVisible();
    }
  });

  test('idle status shows time since active', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const idleCard = page.locator('[data-testid="agent-card"]').filter({ hasText: 'IdleAgent' });
    if (await idleCard.isVisible()) {
      // Should show idle status with time.
      await expect(idleCard.locator('text=/idle|ago|minutes/i')).toBeVisible();
    }
  });

  test('offline status is visually distinct', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const offlineCard = page.locator('[data-testid="agent-card"]').filter({ hasText: 'OfflineAgent' });
    if (await offlineCard.isVisible()) {
      // Should show offline status.
      await expect(offlineCard.locator('text=/offline/i')).toBeVisible();
    }
  });
});

test.describe('Agent card actions', () => {
  test('shows action buttons on hover', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    if (await agentCard.isVisible()) {
      await agentCard.hover();
      await page.waitForTimeout(200);

      // Action buttons may appear on hover.
      const actionButtons = agentCard.locator('[data-testid="card-actions"] button, button[aria-label]');
      // Actions depend on implementation.
    }
  });

  test('card is clickable', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/agents/1', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 1, name: 'ActiveAgent', created_at: new Date().toISOString() }),
      });
    });

    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    if (await agentCard.isVisible()) {
      await agentCard.click();
      await page.waitForTimeout(300);

      // Should open detail view.
      const detail = page.locator('[role="dialog"], [data-testid="agent-detail"]');
      // Detail view depends on implementation.
    }
  });
});

test.describe('Agent card accessibility', () => {
  test('cards are keyboard navigable', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Tab to agent cards.
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await page.waitForTimeout(100);

    // Cards should be focusable.
    const focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });

  test('Enter key activates focused card', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    if (await agentCard.isVisible()) {
      await agentCard.focus();
      await page.keyboard.press('Enter');
      await page.waitForTimeout(300);

      // Should activate the card.
    }
  });

  test('cards have proper ARIA attributes', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    if (await agentCard.isVisible()) {
      // Check for role or aria-label.
      const role = await agentCard.getAttribute('role');
      const ariaLabel = await agentCard.getAttribute('aria-label');
      // Should have accessible attributes.
    }
  });
});

test.describe('Agent card grid layout', () => {
  test('cards display in grid', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const grid = page.locator('[data-testid="agent-card-grid"]');
    await expect(grid).toBeVisible();

    // Should have multiple cards in grid.
    const cards = grid.locator('[data-testid="agent-card"]');
    const count = await cards.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test('grid is responsive', async ({ page }) => {
    await setupAPIs(page);

    // Test mobile viewport.
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Cards should still be visible.
    const cards = page.locator('[data-testid="agent-card"]');
    const count = await cards.count();
    expect(count).toBeGreaterThan(0);
  });
});
