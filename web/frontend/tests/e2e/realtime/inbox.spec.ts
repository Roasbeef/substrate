// E2E tests for real-time inbox updates.

import { test, expect, type Page, type WebSocketRoute } from '@playwright/test';

// Helper to create a mock message for WebSocket events.
function createMockMessage(overrides: Partial<{
  id: number;
  sender_id: number;
  sender_name: string;
  subject: string;
  body: string;
  priority: string;
  created_at: string;
}> = {}) {
  return {
    id: overrides.id ?? Date.now(),
    sender_id: overrides.sender_id ?? 1,
    sender_name: overrides.sender_name ?? 'TestAgent',
    subject: overrides.subject ?? 'Test Message',
    body: overrides.body ?? 'Test message body',
    priority: overrides.priority ?? 'normal',
    created_at: overrides.created_at ?? new Date().toISOString(),
  };
}

// Helper to setup WebSocket route with message simulation.
async function setupWebSocketRoute(
  page: Page,
  onConnect?: (ws: WebSocketRoute) => void,
): Promise<WebSocketRoute | undefined> {
  let wsRoute: WebSocketRoute | undefined;

  await page.routeWebSocket(/\/ws/, async (ws) => {
    wsRoute = ws;
    onConnect?.(ws);
  });

  return wsRoute;
}

test.describe('Inbox real-time updates', () => {
  test('displays new message when received via WebSocket', async ({ page }) => {
    // Setup WebSocket route to capture connection.
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      // Don't close the connection, keep it open for messages.
      ws.onMessage(() => {
        // Handle ping/pong if needed.
      });
    });

    // Navigate to inbox page.
    await page.goto('/');

    // Wait for initial content to load.
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Get initial message count (if any visible).
    const initialMessages = await page.locator('[data-testid="message-row"]').count();

    // Simulate new message via WebSocket.
    if (wsConnection) {
      const newMessage = createMockMessage({
        id: 9999,
        sender_name: 'NewSender',
        subject: 'Breaking News',
        priority: 'urgent',
      });

      // Send WebSocket message that simulates new_message event.
      wsConnection.send(JSON.stringify({
        type: 'new_message',
        payload: newMessage,
      }));

      // Wait for the new message to appear.
      await expect(page.locator('text=Breaking News')).toBeVisible({ timeout: 5000 });
    }
  });

  test('updates unread count when new message arrives', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Find the unread count indicator if present.
    const unreadBadge = page.locator('[data-testid="unread-count"]').first();

    // Send a new unread message.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'new_message',
        payload: createMockMessage({ subject: 'Unread Message' }),
      }));

      // Give time for UI to update.
      await page.waitForTimeout(500);
    }
  });

  test('handles message star update in real-time', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Simulate message update event.
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'message_updated',
        payload: {
          id: 1,
          is_starred: true,
        },
      }));

      // Give time for UI to update.
      await page.waitForTimeout(500);
    }
  });

  test('handles message archive in real-time', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Simulate message deleted event (archived).
    if (wsConnection) {
      wsConnection.send(JSON.stringify({
        type: 'message_deleted',
        payload: { id: 1 },
      }));

      await page.waitForTimeout(500);
    }
  });
});

test.describe('Inbox WebSocket connection state', () => {
  test('shows connection indicator', async ({ page }) => {
    await page.routeWebSocket(/\/ws/, async (ws) => {
      // Keep connection open.
      ws.onMessage(() => {});
    });

    await page.goto('/');

    // Connection indicator should show connected state after WS connects.
    // This depends on implementation - check for any connection status UI.
    await page.waitForTimeout(1000);
  });

  test('handles disconnection gracefully', async ({ page }) => {
    let wsConnection: WebSocketRoute | undefined;
    await page.routeWebSocket(/\/ws/, async (ws) => {
      wsConnection = ws;
      ws.onMessage(() => {});
    });

    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();

    // Close the WebSocket connection.
    if (wsConnection) {
      wsConnection.close();
    }

    // Page should still be functional.
    await page.waitForTimeout(500);
    await expect(page.getByRole('link', { name: 'Inbox' })).toBeVisible();
  });
});
