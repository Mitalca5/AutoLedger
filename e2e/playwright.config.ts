import { defineConfig } from '@playwright/test'

// Runs against a server already started on BASE_URL and seeded with cmd/demoseed (see CLAUDE.md).
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  projects: [
    { name: 'setup', testMatch: /auth\.setup\.ts/ },
    { name: 'flows', testMatch: /(flows|a11y)\.spec\.ts/, dependencies: ['setup'], use: { storageState: '.auth/state.json' } },
  ],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:8080',
    locale: 'en-GB',
    trace: 'retain-on-failure',
  },
})
