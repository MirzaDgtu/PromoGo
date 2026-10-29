import { defineConfig, devices } from '@playwright/test'

// Browser E2E for Admin Web (docs/admin-web-plan.md 8): the critical M1
// scenarios against the real local stack — the Go app from
// deployments/docker-compose.yml, the mock OIDC issuer (profile "dev-oidc")
// and this app's Vite dev server. Nothing is mocked in the browser: login
// is the real Authorization Code + PKCE flow, every screen talks to the
// real API. See e2e/README.md for how to bring the stack up.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:5173'

export default defineConfig({
  testDir: './e2e',
  // The scenarios build on each other (an organization, then its store,
  // its program, its key, its staff), so they run in order, one at a time.
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  globalSetup: './e2e/global-setup.ts',
  use: {
    baseURL,
    locale: 'ru-RU',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: 'npm run dev -- --port 5173 --strictPort',
        url: baseURL,
        reuseExistingServer: true,
        timeout: 60_000,
      },
})
