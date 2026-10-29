import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/testSetup.ts'],
    globals: false,
    // e2e/*.spec.ts are Playwright browser tests (npm run e2e), not vitest.
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
