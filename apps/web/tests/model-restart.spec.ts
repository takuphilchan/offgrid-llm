import { expect, test } from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

test('interrupted package resumes its pinned source after a real service restart', async ({ request, page }) => {
  const control = process.env.OFFGRID_E2E_RESTART_DIR;
  test.skip(!control, 'Run separately with the disposable restart harness.');
  test.setTimeout(60000);
  const discovery = await (await request.get('/api/v2/models/discover?repository=fixture/zipformer-streaming')).json();
  const resolution = await (await request.post('/api/v2/models/resolve', { data: { repository: discovery.repository, revision: discovery.revision, variant: discovery.choices[0].id, architecture: 'zipformer-streaming' } })).json();
  const payload = { action: 'install', request_id: crypto.randomUUID(), resolution_id: resolution.id };
  const response = await request.post('/api/v2/models/operations', { data: payload });
  expect(response.status()).toBe(202);
  const accepted = await response.json();
  await expect.poll(async () => (await (await request.get(`/api/v2/models/operations/${accepted.id}`)).json()).bytes_done).toBeGreaterThan(0);
  const token = crypto.randomUUID();
  await writeFile(join(control!, 'restart-request'), token);
  await expect.poll(async () => readFile(join(control!, 'restart-ack'), 'utf8').catch(() => ''), { timeout: 20000 }).toBe(token);
  const recovered = await (await request.get(`/api/v2/models/operations/${accepted.id}`)).json();
  expect(recovered.state).toBe('interrupted');
  expect(recovered.target).toEqual(accepted.target);
  // A lost acceptance acknowledgment deduplicates even though previews expired on restart.
  const again = await (await request.post('/api/v2/models/operations', { data: payload })).json();
  expect(again.id).toBe(accepted.id);
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.goto('/ui/#/models');
  await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Resume', exact: true })).toBeVisible();
  await page.waitForTimeout(2200);
  expect((await (await request.get(`/api/v2/models/operations/${accepted.id}`)).json()).state).toBe('interrupted');
  await page.getByRole('button', { name: 'Resume', exact: true }).click();
  await expect.poll(async () => (await (await request.get(`/api/v2/models/operations/${accepted.id}`)).json()).state, { timeout: 30000 }).toBe('complete');
  expect((await request.post(`/api/v2/models/packages/${accepted.target.package.id}/${accepted.target.package.revision}/remove`)).ok()).toBe(true);
});
