// E2E tests for thread view modal when clicking inbox messages.

import { test, expect } from '@playwright/test';

// Get API base URL from environment or use default.
const API_PORT = process.env.API_PORT ?? '8082';
const PROD_PORT = process.env.PROD_PORT ?? '8090';
const USE_PRODUCTION = process.env.PLAYWRIGHT_USE_PRODUCTION === 'true';
const API_BASE_URL = USE_PRODUCTION
  ? `http://localhost:${PROD_PORT}/api/v1`
  : `http://localhost:${API_PORT}/api/v1`;
