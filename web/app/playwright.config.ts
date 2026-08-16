import { defineConfig, devices, type BrowserChannel } from '@playwright/test';

const channel = process.env.PLAYWRIGHT_CHANNEL as BrowserChannel | undefined;

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: true,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: process.env.OFFGRID_E2E_URL ?? 'http://127.0.0.1:11611',
    ...devices['Desktop Edge'],
    ...(channel ? { channel } : {}),
    trace: 'retain-on-failure'
  }
});
