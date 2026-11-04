import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e-live',
  workers: 1,
  fullyParallel: false,
  timeout: 90000,
  expect: { timeout: 15000 },
  use: {
    baseURL: process.env.BASE_URL || process.env.PUBLIC_BASE_URL || 'http://127.0.0.1:8080',
    browserName: 'chromium',
    headless: true,
    viewport: { width: 1440, height: 1000 },
    screenshot: 'only-on-failure',
  },
})
