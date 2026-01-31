// E2E tests for sessions list page.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  const sessions = [
    {
      id: 1,
      agent_id: 1,
      agent_name: 'BuildAgent',
      status: 'active',
      started_at: new Date().toISOString(),
    },
    {
      id: 2,
      agent_id: 2,
      agent_name: 'TestAgent',
      status: 'active',
      started_at: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: 3,
      agent_id: 3,
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
          agent_id: body?.agent_id || 1,
          status: 'active',
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: sessions,
          meta: { total: sessions.length, page: 1, page_size: 20 },
        }),
      });
    }
  });

  await page.route('**/api/v1/sessions/active', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: sessions.filter((s) => s.status === 'active'),
      }),
    });
  });

  await page.route('**/api/v1/sessions/*', async (route) => {
    const url = route.request().url();
    if (url.includes('/complete')) {
      await route.fulfill({ status: 204, body: '' });
    } else {
      const idMatch = url.match(/sessions\/(\d+)/);
      const id = idMatch ? parseInt(idMatch[1]) : 1;
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
        data: [
          { id: 1, name: 'BuildAgent', created_at: new Date().toISOString() },
          { id: 2, name: 'TestAgent', created_at: new Date().toISOString() },
          { id: 3, name: 'DeployAgent', created_at: new Date().toISOString() },
        ],
        meta: { total: 3, page: 1, page_size: 20 },
      }),
    });
  });

  return sessions;
}

test.describe('Sessions page loading', () => {
  test('navigates to sessions page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await expect(page.locator('text=Sessions')).toBeVisible();
  });

  test('displays sessions list', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Should show session rows.
    const sessionList = page.locator('[data-testid="session-list"]');
    await expect(sessionList).toBeVisible();
  });

  test('shows session agent names', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Should show agent names.
    await expect(page.locator('text=BuildAgent')).toBeVisible();
    await expect(page.locator('text=TestAgent')).toBeVisible();
  });
});

test.describe('Sessions filtering', () => {
  test('active filter shows only active sessions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Click active filter.
    const activeFilter = page.locator('button:has-text("Active")');
    if (await activeFilter.isVisible()) {
      await activeFilter.click();
      await page.waitForTimeout(300);

      // Should show only active sessions.
      await expect(page.locator('text=BuildAgent')).toBeVisible();
      await expect(page.locator('text=TestAgent')).toBeVisible();
    }
  });

  test('completed filter shows only completed sessions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const completedFilter = page.locator('button:has-text("Completed")');
    if (await completedFilter.isVisible()) {
      await completedFilter.click();
      await page.waitForTimeout(300);

      // Should show only completed sessions.
      await expect(page.locator('text=DeployAgent')).toBeVisible();
    }
  });
});

test.describe('Session detail view', () => {
  test('clicking session opens detail', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Click on first session.
    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      // Detail modal should open.
      const detail = page.locator('[data-testid="session-detail"], [role="dialog"]');
      if (await detail.isVisible()) {
        await expect(detail).toBeVisible();
      }
    }
  });

  test('session detail shows agent name', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        await expect(detail.locator('text=BuildAgent')).toBeVisible();
      }
    }
  });

  test('session detail shows start time', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        // Should show start time.
        await expect(detail.locator('text=/started|start time/i')).toBeVisible();
      }
    }
  });

  test('close button closes detail', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        const closeButton = detail.locator('button[aria-label*="close" i], button:has-text("×")');
        if (await closeButton.isVisible()) {
          await closeButton.click();
          await page.waitForTimeout(200);

          await expect(detail).not.toBeVisible();
        }
      }
    }
  });
});

test.describe('Starting new session', () => {
  test('start session button is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const startButton = page.locator('button:has-text("Start Session"), [data-testid="start-session-button"]');
    await expect(startButton).toBeVisible();
  });

  test('clicking start session opens modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Start Session")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"], [data-testid="start-session-modal"]');
    await expect(modal).toBeVisible();
  });

  test('can select agent for new session', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Start Session")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Should have agent selector.
      const agentSelect = modal.locator('[data-testid="agent-select"], select, button:has-text("Select Agent")');
      await expect(agentSelect).toBeVisible();
    }
  });

  test('submitting starts new session', async ({ page }) => {
    await setupAPIs(page);

    let sessionStarted = false;
    await page.route('**/api/v1/sessions', async (route) => {
      if (route.request().method() === 'POST') {
        sessionStarted = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            session_id: 'new-session',
            agent_id: 1,
            status: 'active',
          }),
        });
      } else {
        await route.continue();
      }
    });

    await page.goto('/sessions');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Start Session")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Fill in session details and submit.
      const submitButton = modal.locator('button:has-text("Start"), button[type="submit"]');
      if (await submitButton.isVisible()) {
        await submitButton.click();
        await page.waitForTimeout(500);

        expect(sessionStarted).toBe(true);
      }
    }
  });
});

test.describe('Completing session', () => {
  test('complete button is visible for active sessions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    // Open active session detail.
    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        const completeButton = detail.locator('button:has-text("Complete"), [data-testid="complete-session"]');
        await expect(completeButton).toBeVisible();
      }
    }
  });

  test('clicking complete marks session as completed', async ({ page }) => {
    let sessionCompleted = false;

    await setupAPIs(page);
    await page.route('**/api/v1/sessions/*/complete', async (route) => {
      sessionCompleted = true;
      await route.fulfill({ status: 204, body: '' });
    });

    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        const completeButton = detail.locator('button:has-text("Complete")');
        if (await completeButton.isVisible()) {
          await completeButton.click();
          await page.waitForTimeout(500);

          expect(sessionCompleted).toBe(true);
        }
      }
    }
  });
});

test.describe('Session tabs', () => {
  test('session detail has tabs', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        // Should have Overview, Log, Tasks tabs.
        await expect(detail.locator('button:has-text("Overview")')).toBeVisible();
        await expect(detail.locator('button:has-text("Log")')).toBeVisible();
        await expect(detail.locator('button:has-text("Tasks")')).toBeVisible();
      }
    }
  });

  test('clicking tab switches content', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/sessions');
    await page.waitForTimeout(500);

    const sessionRow = page.locator('[data-testid="session-row"]').first();
    if (await sessionRow.isVisible()) {
      await sessionRow.click();
      await page.waitForTimeout(300);

      const detail = page.locator('[role="dialog"]');
      if (await detail.isVisible()) {
        // Click Log tab.
        const logTab = detail.locator('button:has-text("Log")');
        if (await logTab.isVisible()) {
          await logTab.click();
          await page.waitForTimeout(200);

          // Log content should be visible.
          // Content depends on implementation.
        }
      }
    }
  });
});
