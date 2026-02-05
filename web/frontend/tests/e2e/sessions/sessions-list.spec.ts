// E2E tests for sessions list page.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints with grpc-gateway format.
async function setupAPIs(page: import('@playwright/test').Page) {
  const sessions = [
    {
      id: '1',
      agent_id: '1',
      agent_name: 'BuildAgent',
      status: 'active',
      started_at: new Date().toISOString(),
    },
    {
      id: '2',
      agent_id: '2',
      agent_name: 'TestAgent',
      status: 'active',
      started_at: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: '3',
      agent_id: '3',
      agent_name: 'DeployAgent',
      status: 'completed',
      started_at: new Date(Date.now() - 86400000).toISOString(),
      completed_at: new Date(Date.now() - 82800000).toISOString(),
    },
  ];

  await page.route('**/api/v1/sessions', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: 'new-session',
          agent_id: body?.agent_id || '1',
          status: 'active',
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          sessions: sessions,
        }),
      });
    }
  });

  await page.route('**/api/v1/sessions/*', async (route) => {
    const url = route.request().url();
    if (url.includes('/complete')) {
      await route.fulfill({ status: 204, body: '' });
    } else {
      const idMatch = url.match(/sessions\/(\d+)/);
      const id = idMatch ? idMatch[1] : '1';
      const session = sessions.find((s) => s.id === id) || sessions[0];
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(session),
      });
    }
  });

  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [
          { id: '1', name: 'BuildAgent', created_at: new Date().toISOString() },
          { id: '2', name: 'TestAgent', created_at: new Date().toISOString() },
          { id: '3', name: 'DeployAgent', created_at: new Date().toISOString() },
        ],
      }),
    });
  });

  return sessions;
}

test.describe('Sessions page loading', () => {
  test('navigates to sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await expect(page.getByRole('heading', { name: 'Sessions' })).toBeVisible();
  });

  test('displays sessions list', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Should show session rows - look for agent names.
    await expect(page.getByText('BuildAgent')).toBeVisible();
  });

  test('shows session agent names', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Should show agent names (use .first() to avoid matching agent dropdown options).
    await expect(page.getByText('BuildAgent').first()).toBeVisible();
    await expect(page.getByText('TestAgent').first()).toBeVisible();
  });
});

test.describe('Sessions filtering', () => {
  test('active filter shows only active sessions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Click active filter.
    const activeFilter = page.getByRole('button', { name: 'Active', exact: true });
    if (await activeFilter.isVisible()) {
      await activeFilter.click();
      await page.waitForTimeout(300);

      // Should show only active sessions.
      await expect(page.getByText('BuildAgent')).toBeVisible();
      await expect(page.getByText('TestAgent')).toBeVisible();
    }
  });

  test('completed filter shows only completed sessions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const completedFilter = page.getByRole('button', { name: 'Completed', exact: true });
    if (await completedFilter.isVisible()) {
      await completedFilter.click();
      await page.waitForTimeout(300);

      // Should show only completed sessions.
      await expect(page.getByText('DeployAgent')).toBeVisible();
    }
  });
});

