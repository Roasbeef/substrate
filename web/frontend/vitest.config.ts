// Vitest configuration for unit and integration testing.

import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    // Use happy-dom for DOM testing (faster and better ESM compat than jsdom).
    environment: 'happy-dom',

    // Setup files run before each test file.
    setupFiles: ['./tests/setup.ts'],

    // Include patterns for test files.
    include: [
      'tests/unit/**/*.test.{ts,tsx}',
      'tests/integration/**/*.test.{ts,tsx}',
      'tests/hooks/**/*.test.{ts,tsx}',
      'src/**/*.test.{ts,tsx}',
    ],

    // Exclude patterns.
    exclude: ['node_modules', 'dist', 'tests/e2e'],

    // Enable globals for describe, it, expect, etc.
    globals: true,

    // Coverage configuration.
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/main.tsx',
        'src/vite-env.d.ts',
        '**/*.d.ts',
        'src/**/*.test.{ts,tsx}',
      ],
      thresholds: {
        statements: 80,
        branches: 80,
        functions: 80,
        lines: 80,
      },
    },

    // Reporter configuration.
    reporters: ['default'],

    // Watch mode settings.
    watch: false,

    // Use threads pool which handles ESM better.
    pool: 'threads',

    // CSS handling.
    css: {
      modules: {
        classNameStrategy: 'non-scoped',
      },
    },
  },
});
