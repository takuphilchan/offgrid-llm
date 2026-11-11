import { defineConfig, devices, type BrowserChannel } from '@playwright/test';

const channel = process.env.PLAYWRIGHT_CHANNEL as BrowserChannel | undefined;
const externalURL = process.env.OFFGRID_E2E_URL;
const fixturePort = Number(process.env.OFFGRID_FIXTURE_PORT ?? 5173);
if (!Number.isInteger(fixturePort) || fixturePort < 1024 || fixturePort > 65535) throw new Error('Invalid isolated fixture port');
const localURL = `http://127.0.0.1:${fixturePort}`;

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
      command: `npm run dev -- --host 127.0.0.1 --port ${fixturePort} --strictPort`,
      // Fixture tests must never fall through to an installed user service.
      env: { OFFGRID_FIXTURE_TEST: '1' },
      url: `${localURL}/ui/`,
      reuseExistingServer: false,
      timeout: 120_000
    }
  })
});
