import { defineConfig, devices, type BrowserChannel } from '@playwright/test';

const channel = process.env.PLAYWRIGHT_CHANNEL as BrowserChannel | undefined;
const externalURL = process.env.OFFGRID_E2E_URL;
const localURL = 'http://127.0.0.1:5173';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: true,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: externalURL ?? localURL,
    ...devices['Desktop Edge'],
    ...(channel ? { channel } : {}),
    trace: 'retain-on-failure'
  },
  ...(externalURL ? {} : {
    webServer: {
      command: 'npm run dev -- --host 127.0.0.1',
      url: `${localURL}/ui/`,
      reuseExistingServer: false,
      timeout: 120_000
    }
  })
});
