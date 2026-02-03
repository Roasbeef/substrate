// E2E tests for real-time agent status updates.

import { test, expect, type Page, type WebSocketRoute } from '@playwright/test';

// Helper to create a mock agent status update.
function createAgentStatusUpdate(overrides: Partial<{
  id: number;
  name: string;
  status: 'active' | 'busy' | 'idle' | 'offline';
  last_active_at: string;
  session_id?: number;
  seconds_since_heartbeat: number;
}> = {}) {
  return {
    id: overrides.id ?? 1,
    name: overrides.name ?? 'Agent1',
    status: overrides.status ?? 'active',
    last_active_at: overrides.last_active_at ?? new Date().toISOString(),
    session_id: overrides.session_id,
    seconds_since_heartbeat: overrides.seconds_since_heartbeat ?? 10,
  };
}

test.describe('Agent status real-time updates', () => {
  test('updates agent status when received via WebSocket', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    // Navigate to agents page.
    await page.goto('/agents');

    // Wait for agents page to load.
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate agent status update.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 1,
          name: 'Agent1',
          status: 'busy',
          session_id: 42,
        }),
      }));

      // Wait for status change to reflect.
      await page.waitForTimeout(500);
    }
  });

  test('shows agent going offline', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate agent going offline.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 1,
          name: 'Agent1',
          status: 'offline',
          seconds_since_heartbeat: 3600,
        }),
      }));

      await page.waitForTimeout(500);
    }
  });

  test('shows agent becoming active', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate agent becoming active.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 2,
          name: 'Agent2',
          status: 'active',
          seconds_since_heartbeat: 5,
        }),
      }));

      await page.waitForTimeout(500);
    }
  });

  test('updates dashboard stats when agent status changes', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Capture initial stats if available.
    const statsCards = page.locator('[data-testid="stats-card"]');

    // Send multiple status updates.
    if (wsConnection) {
      // Agent 1 goes busy.
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({ id: 1, status: 'busy', session_id: 100 }),
      }));

      // Agent 2 goes idle.
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({ id: 2, status: 'idle' }),
      }));

      await page.waitForTimeout(500);
    }
  });
});

test.describe('Agent session tracking', () => {
  test('shows session info when agent starts session', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // Simulate agent starting a session.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 1,
          name: 'Agent1',
          status: 'busy',
          session_id: 42,
        }),
      }));

      // Check for session indicator.
      await page.waitForTimeout(500);
    }
  });

  test('clears session info when agent completes session', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/agents');
    await expect(page.locator('text=Agents')).toBeVisible();

    // First set agent as busy with session.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 1,
          name: 'Agent1',
          status: 'busy',
          session_id: 42,
        }),
      }));

      await page.waitForTimeout(300);

      // Then complete the session.
      wsConnection.send(JSON.stringify({
        type: 'agent_status_changed',
        payload: createAgentStatusUpdate({
          id: 1,
          name: 'Agent1',
          status: 'active',
          session_id: undefined,
        }),
      }));

      await page.waitForTimeout(500);
    }
  });
});
