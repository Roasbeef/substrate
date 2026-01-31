// Test setup file for Vitest with React Testing Library and MSW.

import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, vi } from 'vitest';
import { server } from './mocks/server.js';

// Start MSW server before all tests.
beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' });
});

// Reset handlers after each test to prevent test pollution.
afterEach(() => {
  server.resetHandlers();
});

// Stop MSW server after all tests.
afterAll(() => {
  server.close();
});

// Cleanup after each test to prevent memory leaks.
afterEach(() => {
  cleanup();
});

// Mock window.matchMedia for components that use media queries.
beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

// Mock ResizeObserver for components that use it.
class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
beforeAll(() => {
  global.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver;
});

// Mock IntersectionObserver for components that use it.
class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  root = null;
  rootMargin = '';
  thresholds: ReadonlyArray<number> = [];
  takeRecords = vi.fn().mockReturnValue([]);
  constructor(_callback: IntersectionObserverCallback, _options?: IntersectionObserverInit) {}
}
beforeAll(() => {
  global.IntersectionObserver = MockIntersectionObserver as unknown as typeof IntersectionObserver;
});

// Suppress console errors during tests unless needed for debugging.
// Uncomment the following to see all console output:
// beforeAll(() => {
//   vi.spyOn(console, 'error').mockImplementation(() => {});
//   vi.spyOn(console, 'warn').mockImplementation(() => {});
// });
