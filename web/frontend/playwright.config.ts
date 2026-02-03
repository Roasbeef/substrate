// Playwright configuration for E2E testing.

import { defineConfig, devices } from '@playwright/test';

// Read from environment variables with test defaults.
const VITE_DEV_PORT = process.env.VITE_DEV_PORT ?? '5175';
const API_PORT = process.env.API_PORT ?? '8082';
const GRPC_PORT = process.env.GRPC_PORT ?? '10012';
const PROD_PORT = process.env.PROD_PORT ?? '8090';

// Check if we're testing against production build.
const USE_PRODUCTION = process.env.PLAYWRIGHT_USE_PRODUCTION === 'true';

export default defineConfig({
  // Test directory.
  testDir: './tests/e2e',

  // Run tests in parallel.
  fullyParallel: true,

  // Fail the build on CI if you accidentally left test.only in the source code.
  forbidOnly: !!process.env.CI,

  // Retry on CI only.
  retries: process.env.CI ? 2 : 0,

  // Opt out of parallel tests on CI.
  workers: process.env.CI ? 1 : undefined,

  // Reporter to use.
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
  ],

  // Shared settings for all tests.
  use: {
    // Base URL for navigation (use production server if PLAYWRIGHT_USE_PRODUCTION is set).
    baseURL: USE_PRODUCTION ? `http://localhost:${PROD_PORT}` : `http://localhost:${VITE_DEV_PORT}`,

    // Collect trace when retrying failed tests.
    trace: 'on-first-retry',

    // Take screenshot on failure.
    screenshot: 'only-on-failure',

    // Video on first retry.
    video: 'on-first-retry',

    // Timeout for each action.
    actionTimeout: 10000,
  },

  // Timeout for each test.
  timeout: 30000,

  // Expect timeout.
  expect: {
    timeout: 5000,
  },

  // Configure projects for browsers.
  // Only Chromium for now to keep CI fast.
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Server configuration - use production build or dev servers.
  // Tests use a separate database (.test-data/test.db) to avoid modifying your normal data.
  webServer: USE_PRODUCTION
    ? [
        // Production: Start the Go server with embedded frontend.
        // gRPC must be enabled for grpc-gateway REST API to work.
        {
          command: `cd ../.. && ./substrated -web-only -web :${PROD_PORT} -grpc localhost:${GRPC_PORT} -db .test-data/test.db`,
          url: `http://localhost:${PROD_PORT}/api/v1/health`,
          reuseExistingServer: !process.env.CI,
          timeout: 120000,
        },
      ]
    : [
        // Development: Start Go API server + Vite dev server.
        // Note: gRPC must be enabled for grpc-gateway REST API to work.
        {
          command: `cd ../.. && CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" CGO_LDFLAGS="-lm" go run ./cmd/substrated -web-only -web :${API_PORT} -grpc localhost:${GRPC_PORT} -db .test-data/test.db`,
          url: `http://localhost:${API_PORT}/api/v1/health`,
          reuseExistingServer: !process.env.CI,
          timeout: 120000,
        },
        {
          command: `bun run dev --port ${VITE_DEV_PORT}`,
          url: `http://localhost:${VITE_DEV_PORT}`,
          reuseExistingServer: !process.env.CI,
          timeout: 120000,
        },
      ],

  // Output directory for test artifacts.
  outputDir: './test-results',
});
