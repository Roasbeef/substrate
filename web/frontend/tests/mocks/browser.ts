// MSW browser setup for development.

import { setupWorker } from 'msw/browser';
import { handlers } from './handlers.js';

// Create the service worker with default handlers.
export const worker = setupWorker(...handlers);
