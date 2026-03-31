import { defineConfig } from '@playwright/test';

/**
 * Playwright E2E configuration for AgentLens.
 *
 * The backend is built and started automatically via `webServer`.
 * It uses SQLite so no external dependencies are needed.
 *
 * Environment variables used at runtime:
 *   AGENTLENS_PORT          – HTTP port (default 18080)
 *   AGENTLENS_JWT_SECRET    – JWT signing key (default test-secret)
 *   AGENTLENS_DATA_DIR      – directory for SQLite file
 *   AGENTLENS_ADMIN_PASSWORD – captured from stdout on first run
 */
export default defineConfig({
  testDir: './tests',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,          // tests share state (admin user)
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,                    // single backend instance
  reporter: process.env.CI ? 'github' : 'list',

  use: {
    baseURL: `http://localhost:${process.env.AGENTLENS_PORT ?? 18080}`,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
