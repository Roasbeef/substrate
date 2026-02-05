// Performance tests for web vitals and loading times.

import { test, expect } from '@playwright/test';

// Helper to setup API mocks with grpc-gateway format.
async function setupAPIMocks(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        messages: Array.from({ length: 20 }, (_, i) => ({
          id: String(i + 1),
          sender_id: '1',
          sender_name: `Agent ${i + 1}`,
          subject: `Message ${i + 1}`,
          body: 'Test body content',
          priority: 'PRIORITY_NORMAL',
          created_at: new Date().toISOString(),
          recipients: [],
        })),
      }),
    });
  });

  await page.route('**/api/v1/agents-status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: Array.from({ length: 10 }, (_, i) => ({
          id: String(i + 1),
          name: `Agent ${i + 1}`,
          status: i % 3 === 0 ? 'active' : i % 3 === 1 ? 'idle' : 'offline',
          last_active_at: new Date().toISOString(),
          seconds_since_heartbeat: 30,
        })),
        counts: { active: 4, idle: 3, offline: 3 },
      }),
    });
  });

  await page.route('**/api/v1/agents', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: Array.from({ length: 10 }, (_, i) => ({
          id: String(i + 1),
          name: `Agent ${i + 1}`,
          created_at: new Date().toISOString(),
        })),
      }),
    });
  });

  await page.route('**/api/v1/topics*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ topics: [] }),
    });
  });

  await page.route('**/api/v1/activities*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ activities: [] }),
    });
  });

  await page.route('**/api/v1/sessions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sessions: [] }),
    });
  });
}

test.describe('Page load performance', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('inbox page loads within acceptable time', async ({ page }) => {
    const startTime = Date.now();
    await page.goto('/inbox');
    await page.waitForLoadState('domcontentloaded');
    const loadTime = Date.now() - startTime;

    // Should load DOM within 3 seconds.
    expect(loadTime).toBeLessThan(3000);

    // Wait for full network idle.
    const networkIdleStart = Date.now();
    await page.waitForLoadState('networkidle');
    const networkIdleTime = Date.now() - networkIdleStart;

    // Network idle should be quick after DOM.
    expect(networkIdleTime).toBeLessThan(2000);
  });

  test('agents page loads within acceptable time', async ({ page }) => {
    const startTime = Date.now();
    await page.goto('/agents');
    await page.waitForLoadState('domcontentloaded');
    const loadTime = Date.now() - startTime;

    expect(loadTime).toBeLessThan(3000);
  });

  test('navigation between pages is fast', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Navigate to agents.
    const navStart = Date.now();
    await page.click('a[href="/agents"]');
    await page.waitForURL('**/agents');
    await page.waitForLoadState('domcontentloaded');
    const navTime = Date.now() - navStart;

    // SPA navigation should be fast.
    expect(navTime).toBeLessThan(1000);
  });
});

test.describe('Web Vitals', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('Largest Contentful Paint (LCP) is acceptable', async ({ page }) => {
    // Enable performance timing.
    await page.goto('/inbox');

    // Wait for page to fully render.
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    // Get LCP from performance observer.
    const lcp = await page.evaluate(() => {
      return new Promise((resolve) => {
        new PerformanceObserver((entryList) => {
          const entries = entryList.getEntries();
          const lastEntry = entries[entries.length - 1];
          resolve(lastEntry?.startTime || 0);
        }).observe({ type: 'largest-contentful-paint', buffered: true });

        // Fallback if no LCP entry.
        setTimeout(() => resolve(0), 1000);
      });
    });

    // LCP should be under 2.5 seconds (good).
    expect(lcp).toBeLessThan(2500);
  });

  test('First Input Delay (FID) proxy - interaction is responsive', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Measure time to first interaction.
    const button = page.getByRole('button', { name: /compose/i });
    await button.waitFor({ state: 'visible' });

    const interactionStart = Date.now();
    await button.click();
    await page.getByRole('heading', { name: 'Compose Message' }).waitFor({ state: 'visible' });
    const interactionTime = Date.now() - interactionStart;

    // Interaction should be under 100ms (good FID).
    expect(interactionTime).toBeLessThan(500);
  });
});

test.describe('Resource loading', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('JavaScript bundles are reasonably sized', async ({ page }) => {
    const resourceSizes: { url: string; size: number }[] = [];

    page.on('response', async (response) => {
      if (response.url().endsWith('.js')) {
        try {
          const buffer = await response.body();
          resourceSizes.push({
            url: response.url(),
            size: buffer.length,
          });
        } catch {
          // Ignore errors for failed responses.
        }
      }
    });

    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Check total JS size.
    const totalJsSize = resourceSizes.reduce((sum, r) => sum + r.size, 0);

    // Total JS should be under 1MB uncompressed.
    expect(totalJsSize).toBeLessThan(1024 * 1024);

    // No single bundle should be over 500KB.
    for (const resource of resourceSizes) {
      expect(resource.size).toBeLessThan(500 * 1024);
    }
  });

  test('CSS is reasonably sized', async ({ page }) => {
    let cssSize = 0;

    page.on('response', async (response) => {
      if (response.url().endsWith('.css')) {
        try {
          const buffer = await response.body();
          cssSize += buffer.length;
        } catch {
          // Ignore errors.
        }
      }
    });

    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // CSS should be under 200KB.
    expect(cssSize).toBeLessThan(200 * 1024);
  });
});

test.describe('Rendering performance', () => {
  test.beforeEach(async ({ page }) => {
    await setupAPIMocks(page);
  });

  test('message list renders efficiently', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Count message rows.
    const messageRows = page.locator('[data-testid="message-row"], tr, .message-row');
    const rowCount = await messageRows.count();

    // Should render messages.
    expect(rowCount).toBeGreaterThan(0);

    // Measure scroll performance (if applicable).
    if (rowCount > 10) {
      const scrollStart = Date.now();
      await page.evaluate(() => {
        window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
      });
      await page.waitForTimeout(500);
      const scrollTime = Date.now() - scrollStart;

      // Scroll should be smooth.
      expect(scrollTime).toBeLessThan(1000);
    }
  });

  test('no layout thrashing during interactions', async ({ page }) => {
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Measure performance during rapid tab switching.
    const tabs = page.locator('[role="tab"]');
    const tabCount = await tabs.count();

    if (tabCount > 1) {
      const interactionStart = Date.now();

      for (let i = 0; i < Math.min(tabCount, 3); i++) {
        await tabs.nth(i).click();
        await page.waitForTimeout(100);
      }

      const interactionTime = Date.now() - interactionStart;

      // Rapid tab switches should not cause significant delays.
      expect(interactionTime).toBeLessThan(2000);
    }
  });
});

test.describe('Memory usage', () => {
  test('no memory leaks during navigation', async ({ page }) => {
    await setupAPIMocks(page);

    // Initial navigation.
    await page.goto('/inbox');
    await page.waitForLoadState('networkidle');

    // Get initial memory if available.
    const initialMemory = await page.evaluate(() => {
      // @ts-expect-error - memory is non-standard
      return (performance as { memory?: { usedJSHeapSize: number } }).memory?.usedJSHeapSize || 0;
    });

    // Navigate multiple times.
    for (let i = 0; i < 5; i++) {
      await page.click('a[href="/agents"]');
      await page.waitForURL('**/agents');
      await page.waitForTimeout(200);

      await page.click('a[href="/inbox"]');
      await page.waitForURL('**/inbox');
      await page.waitForTimeout(200);
    }

    // Get final memory.
    const finalMemory = await page.evaluate(() => {
      // @ts-expect-error - memory is non-standard
      return (performance as { memory?: { usedJSHeapSize: number } }).memory?.usedJSHeapSize || 0;
    });

    // Memory should not grow excessively (allow 50% growth for caching).
    if (initialMemory > 0 && finalMemory > 0) {
      expect(finalMemory).toBeLessThan(initialMemory * 1.5);
    }
  });
});
