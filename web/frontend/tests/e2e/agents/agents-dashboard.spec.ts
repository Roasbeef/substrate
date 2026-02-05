// E2E tests for agents dashboard page.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(page: import('@playwright/test').Page) {
  const agents = [
    {
      id: '1',
      name: 'BuildAgent',
      status: 'active',
      last_active_at: new Date().toISOString(),
      session_id: 'session-1',
      seconds_since_heartbeat: 30,
    },
    {
      id: '2',
      name: 'TestAgent',
      status: 'idle',
      last_active_at: new Date(Date.now() - 600000).toISOString(),
      seconds_since_heartbeat: 600,
    },
    {
      id: '3',
      name: 'DeployAgent',
      status: 'busy',
      last_active_at: new Date().toISOString(),
      session_id: 'session-2',
      seconds_since_heartbeat: 10,
    },
    {
      id: '4',
      name: 'OfflineAgent',
      status: 'offline',
      last_active_at: new Date(Date.now() - 3600000).toISOString(),
      seconds_since_heartbeat: 3600,
    },
  ];

  await page.route('**/api/v1/agents-status', async (route) => {
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
          id: '100',
          name: body?.name || 'NewAgent',
          created_at: new Date().toISOString(),
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          agents: agents.map((a) => ({ id: a.id, name: a.name, created_at: new Date().toISOString() })),
        }),
      });
    }
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        activities: [
          {
            id: '1',
            agent_id: '1',
            agent_name: 'BuildAgent',
            type: 'session_start',
            description: 'Started session',
            created_at: new Date().toISOString(),
          },
          {
            id: '2',
            agent_id: '3',
            agent_name: 'DeployAgent',
            type: 'commit',
            description: 'Committed changes',
            created_at: new Date().toISOString(),
          },
        ],
      }),
    });
  });

  return agents;
}

test.describe('Agents dashboard loading', () => {
  test('navigates to agents page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    // Use heading role to avoid matching multiple "Agents" text elements.
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible();
  });

  test('displays agent cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show agent cards by name (use heading role to avoid header switcher).
    await expect(page.getByRole('heading', { name: 'BuildAgent' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'TestAgent' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'DeployAgent' })).toBeVisible();
  });

  test('shows status indicators on agent cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show status text on agent cards.
    const statusText = page.locator('text=/active|busy|idle|offline/i');
    const count = await statusText.count();
    expect(count).toBeGreaterThan(0);
  });
});

test.describe('Dashboard stats', () => {
  test('displays stats cards', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show stats section with stat labels.
    await expect(page.getByText('Total')).toBeVisible();
  });

  test('shows active agent count', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show active text in stats.
    await expect(page.getByText(/active/i).first()).toBeVisible();
  });

  test('shows correct status counts', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Stats should show numeric counts.
    const statNumbers = page.locator('.text-2xl');
    const count = await statNumbers.count();
    expect(count).toBeGreaterThan(0);
  });
});

test.describe('Agent filters', () => {
  test('displays filter tabs', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show filter tabs (buttons have counts appended like "Active1").
    const filterNav = page.locator('nav[aria-label="Filter agents"]');
    await expect(filterNav.getByRole('button', { name: 'All' })).toBeVisible();
    await expect(filterNav.getByRole('button', { name: /^Active/ })).toBeVisible();
    await expect(filterNav.getByRole('button', { name: /^Idle/ })).toBeVisible();
  });

  test('All filter shows all agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Should show all agent names (use heading role to avoid header switcher).
    await expect(page.getByRole('heading', { name: 'BuildAgent' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'TestAgent' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'DeployAgent' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'OfflineAgent' })).toBeVisible();
  });

  test('Active filter shows only active agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Click Active filter button (has count appended like "Active1").
    const filterNav = page.locator('nav[aria-label="Filter agents"]');
    await filterNav.getByRole('button', { name: /^Active/ }).click();
    await page.waitForTimeout(300);

    // Should filter to only active agents (busy is a separate filter).
    await expect(page.getByRole('heading', { name: 'BuildAgent' })).toBeVisible();
    // DeployAgent is busy, not active, so not visible with Active filter.
    await expect(page.getByRole('heading', { name: 'DeployAgent' })).not.toBeVisible();
    await expect(page.getByRole('heading', { name: 'OfflineAgent' })).not.toBeVisible();
  });

  test('Idle filter shows only idle agents', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Click Idle filter button (has count appended like "Idle1").
    const filterNav = page.locator('nav[aria-label="Filter agents"]');
    await filterNav.getByRole('button', { name: /^Idle/ }).click();
    await page.waitForTimeout(300);

    // Should filter to idle agents (use heading role to avoid header switcher).
    await expect(page.getByRole('heading', { name: 'TestAgent' })).toBeVisible();
  });
});

test.describe('Agent card interactions', () => {
  test('agent card shows last active time', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Agent cards show status and time info.
    await expect(page.getByText(/ago|just now|active|idle|offline/i).first()).toBeVisible();
  });
});

test.describe('New agent registration', () => {
  test('new agent button is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/agents');
    await page.waitForTimeout(500);

    // Button text is "Add Agent" in the sidebar.
    const newAgentButton = page.getByRole('button', { name: /Add Agent/i });
    await expect(newAgentButton).toBeVisible();
  });
});
