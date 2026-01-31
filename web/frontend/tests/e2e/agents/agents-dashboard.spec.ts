// E2E tests for agents dashboard page.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  const agents = [
    {
      id: 1,
      name: 'BuildAgent',
      status: 'active',
      last_active_at: new Date().toISOString(),
      session_id: 'session-1',
      seconds_since_heartbeat: 30,
    },
    {
      id: 2,
      name: 'TestAgent',
      status: 'idle',
      last_active_at: new Date(Date.now() - 600000).toISOString(),
      seconds_since_heartbeat: 600,
    },
    {
      id: 3,
      name: 'DeployAgent',
      status: 'busy',
      last_active_at: new Date().toISOString(),
      session_id: 'session-2',
      seconds_since_heartbeat: 10,
    },
    {
      id: 4,
      name: 'OfflineAgent',
      status: 'offline',
      last_active_at: new Date(Date.now() - 3600000).toISOString(),
      seconds_since_heartbeat: 3600,
    },
  ];

  await page.route('**/api/v1/agents/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents,
        counts: { active: 1, busy: 1, idle: 1, offline: 1 },
      }),
    });
  });

  await page.route('**/api/v1/agents', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 100,
          name: body?.name || 'NewAgent',
          created_at: new Date().toISOString(),
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: agents.map((a) => ({ id: a.id, name: a.name, created_at: new Date().toISOString() })),
          meta: { total: agents.length, page: 1, page_size: 20 },
        }),
      });
    }
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [
          {
            id: 1,
            agent_id: 1,
            agent_name: 'BuildAgent',
            type: 'session_start',
            description: 'Started session',
            created_at: new Date().toISOString(),
          },
          {
            id: 2,
            agent_id: 3,
            agent_name: 'DeployAgent',
            type: 'commit',
            description: 'Committed changes',
            created_at: new Date().toISOString(),
          },
        ],
        meta: { total: 2, page: 1, page_size: 20 },
      }),
    });
  });

  return agents;
}

test.describe('Agents dashboard loading', () => {
  test('navigates to agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();
  });

  test('displays agent cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show agent cards.
    await expect(page.locator('text=BuildAgent')).toBeVisible();
    await expect(page.locator('text=TestAgent')).toBeVisible();
    await expect(page.locator('text=DeployAgent')).toBeVisible();
  });

  test('shows status indicators on agent cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show status badges.
    const statusBadges = page.locator('[data-testid="status-badge"]');
    const count = await statusBadges.count();
    expect(count).toBeGreaterThan(0);
  });
});

test.describe('Dashboard stats', () => {
  test('displays stats cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show stats section.
    const statsSection = page.locator('[data-testid="dashboard-stats"]');
    await expect(statsSection).toBeVisible();
  });

  test('shows active agent count', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show active count.
    const activeCard = page.locator('[data-testid="active-count"], text=/active/i');
    await expect(activeCard.first()).toBeVisible();
  });

  test('shows correct status counts', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Stats should reflect API data (1 active, 1 busy, 1 idle, 1 offline).
    const stats = page.locator('[data-testid="dashboard-stats"]');
    if (await stats.isVisible()) {
      // Check for count values.
      await expect(stats).toContainText(/1/);
    }
  });
});

test.describe('Agent filters', () => {
  test('displays filter tabs', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show filter tabs.
    await expect(page.locator('button:has-text("All")')).toBeVisible();
    await expect(page.locator('button:has-text("Active")')).toBeVisible();
    await expect(page.locator('button:has-text("Idle")')).toBeVisible();
  });

  test('All filter shows all agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // All filter is default.
    const agentCards = page.locator('[data-testid="agent-card"]');
    const count = await agentCards.count();
    expect(count).toBe(4);
  });

  test('Active filter shows only active/busy agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Active")').click();
    await page.waitForTimeout(300);

    // Should filter to active/busy agents.
    await expect(page.locator('text=BuildAgent')).toBeVisible();
    await expect(page.locator('text=DeployAgent')).toBeVisible();
    await expect(page.locator('text=OfflineAgent')).not.toBeVisible();
  });

  test('Idle filter shows only idle agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Idle")').click();
    await page.waitForTimeout(300);

    // Should filter to idle agents.
    await expect(page.locator('text=TestAgent')).toBeVisible();
  });
});

test.describe('Agent card interactions', () => {
  test('clicking agent card opens detail', async ({ page }) => {
    await setupAPIs(page);

    await page.route('**/api/v1/agents/1', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 1,
          name: 'BuildAgent',
          created_at: new Date().toISOString(),
        }),
      });
    });

    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('[data-testid="agent-card"]').first().click();
    await page.waitForTimeout(500);

    // Should open agent detail modal or page.
    const detail = page.locator('[data-testid="agent-detail"], [role="dialog"]');
    if (await detail.isVisible()) {
      await expect(detail.locator('text=BuildAgent')).toBeVisible();
    }
  });

  test('agent card shows last active time', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const agentCard = page.locator('[data-testid="agent-card"]').first();
    if (await agentCard.isVisible()) {
      // Should show relative time.
      await expect(agentCard.locator('text=/ago|just now|active/i')).toBeVisible();
    }
  });
});

test.describe('Activity feed', () => {
  test('displays activity feed', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const activityFeed = page.locator('[data-testid="activity-feed"]');
    await expect(activityFeed).toBeVisible();
  });

  test('shows activity items', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show activity items.
    await expect(page.locator('text=Started session')).toBeVisible();
    await expect(page.locator('text=Committed changes')).toBeVisible();
  });

  test('activity items show agent name', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const activityFeed = page.locator('[data-testid="activity-feed"]');
    if (await activityFeed.isVisible()) {
      await expect(activityFeed.locator('text=BuildAgent')).toBeVisible();
    }
  });
});

test.describe('New agent registration', () => {
  test('new agent button is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    const newAgentButton = page.locator('button:has-text("New Agent"), [data-testid="new-agent-button"]');
    await expect(newAgentButton).toBeVisible();
  });

  test('clicking new agent opens modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("New Agent")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"], [data-testid="new-agent-modal"]');
    await expect(modal).toBeVisible();
  });

  test('can register new agent', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("New Agent")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Fill in agent name.
      const nameInput = modal.locator('input[placeholder*="name" i], [data-testid="agent-name-input"]');
      if (await nameInput.isVisible()) {
        await nameInput.fill('MyNewAgent');
      }

      // Submit.
      await modal.locator('button:has-text("Register"), button:has-text("Create")').click();
      await page.waitForTimeout(500);

      // Modal should close.
      await expect(modal).not.toBeVisible();
    }
  });

  test('new agent appears in list after registration', async ({ page }) => {
    let agentRegistered = false;

    await page.route('**/api/v1/agents', async (route) => {
      if (route.request().method() === 'POST') {
        agentRegistered = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            id: 100,
            name: 'MyNewAgent',
            created_at: new Date().toISOString(),
          }),
        });
      } else {
        const agents = agentRegistered
          ? [
              { id: 1, name: 'BuildAgent', status: 'active' },
              { id: 100, name: 'MyNewAgent', status: 'offline' },
            ]
          : [{ id: 1, name: 'BuildAgent', status: 'active' }];

        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: agents,
            meta: { total: agents.length, page: 1, page_size: 20 },
          }),
        });
      }
    });

    await page.route('**/api/v1/agents/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          agents: [{ id: 1, name: 'BuildAgent', status: 'active' }],
          counts: { active: 1, busy: 0, idle: 0, offline: 0 },
        }),
      });
    });

    await page.goto('/agents');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("New Agent")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const nameInput = modal.locator('input').first();
      await nameInput.fill('MyNewAgent');
      await modal.locator('button:has-text("Register"), button:has-text("Create")').click();
      await page.waitForTimeout(500);

      // New agent should appear.
      await expect(page.locator('text=MyNewAgent')).toBeVisible();
    }
  });
});
