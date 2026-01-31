// Playwright configuration for E2E testing.

import { defineConfig, devices } from '@playwright/test';

// Read from environment variables with test defaults.
const VITE_DEV_PORT = process.env.VITE_DEV_PORT ?? '5175';
const API_PORT = process.env.API_PORT ?? '8082';

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
    // Base URL for navigation.
    baseURL: `http://localhost:${VITE_DEV_PORT}`,

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

  // Configure projects for major browsers.
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },

    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },

    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },

    // Mobile viewports.
    {
      name: 'Mobile Chrome',
      use: { ...devices['Pixel 5'] },
    },

    {
      name: 'Mobile Safari',
      use: { ...devices['iPhone 12'] },
    },
  ],

  // Local dev server configuration.
  webServer: [
    // Start the Go API server.
    {
      command: `cd ../.. && go run ./cmd/substrated -web-only -addr :${API_PORT} -data-dir .test-data`,
      url: `http://localhost:${API_PORT}/api/v1/health`,
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
    // Start the Vite dev server.
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
