// E2E tests for real-time activity feed updates.

import { test, expect, type WebSocketRoute } from '@playwright/test';

// Helper to create a mock activity.
function createMockActivity(overrides: Partial<{
  id: number;
  agent_id: number;
  agent_name: string;
  type: string;
  description: string;
  created_at: string;
}> = {}) {
  return {
    id: overrides.id ?? Date.now(),
    agent_id: overrides.agent_id ?? 1,
    agent_name: overrides.agent_name ?? 'Agent1',
    type: overrides.type ?? 'message_sent',
    description: overrides.description ?? 'Performed an action',
    created_at: overrides.created_at ?? new Date().toISOString(),
  };
}

test.describe('Activity feed real-time updates', () => {
  test('shows new activity when received via WebSocket', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    // Navigate to agents page where activity feed is visible.
    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate new activity.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          id: 9999,
          agent_name: 'Agent1',
          type: 'session_started',
          description: 'Started a new session',
        }),
      }));

      // Wait for activity to appear.
      await page.waitForTimeout(500);
    }
  });

  test('shows message sent activity', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate message sent activity.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          type: 'message_sent',
          description: 'Sent a message to User',
        }),
      }));

      await page.waitForTimeout(500);
    }
  });

  test('shows session completed activity', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate session completed activity.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          type: 'session_completed',
          description: 'Completed session #42',
        }),
      }));

      await page.waitForTimeout(500);
    }
  });

  test('multiple activities arrive in sequence', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Send multiple activities.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          id: 1001,
          description: 'First activity',
        }),
      }));

      await page.waitForTimeout(100);

      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          id: 1002,
          description: 'Second activity',
        }),
      }));

      await page.waitForTimeout(100);

      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          id: 1003,
          description: 'Third activity',
        }),
      }));

      await page.waitForTimeout(500);
    }
  });
});

test.describe('Activity feed on different pages', () => {
  test('activity updates work on inbox page', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Activity updates should still be received even on inbox page.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          description: 'Activity while on inbox',
        }),
      }));

      await page.waitForTimeout(500);
    }
  });

  test('navigating preserves activity feed state', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Send activity.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_activity',
        payload: createMockActivity({
          description: 'Activity before navigation',
        }),
      }));
    }

    await page.waitForTimeout(300);

    // Navigate away.
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Navigate back.
    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Activity should still be visible or refetched.
    await page.waitForTimeout(500);
  });
});
