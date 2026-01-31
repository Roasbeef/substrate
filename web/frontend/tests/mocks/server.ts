// MSW server setup for testing.

import { setupServer } from 'msw/node';
import { handlers } from './handlers.js';

// Create the mock server with default handlers.
export const server = setupServer(...handlers);
