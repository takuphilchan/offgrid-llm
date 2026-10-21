import { expect, test } from '@playwright/test';

test.describe('complete package acquisition through a real isolated service', () => {
  test.skip(process.env.OFFGRID_E2E_MODEL_FIXTURES !== '1', 'Requires the explicitly fixture-tagged service, never a live installation.');
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en'); });
    await page.goto('/ui/#/models');
    await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  });
  test('multiple supported variants require an explicit inline choice', async ({ page, request }) => {
    const discovered = await (await request.get('/api/v2/models/discover?repository=fixture/whisper')).json();
    const choice = discovered.choices[0];
    let resolutions = 0, installs = 0;
    await page.route('**/api/v2/models/discover?*', route => route.fulfill({ json: { ...discovered, choices: [choice, { ...choice, id: 'alternate', name: 'Alternate profile' }] } }));
    page.on('request', request => {
      if (request.method() === 'POST' && request.url().endsWith('/api/v2/models/resolve')) resolutions++;
      if (request.method() === 'POST' && request.url().endsWith('/api/v2/models/operations')) installs++;
    });
    await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
    await page.getByRole('searchbox').fill('whisper');
    await page.getByRole('button', { name: 'Search', exact: true }).click();
    const repo = page.locator('article.model-search-result').filter({ hasText: 'fixture/whisper' });
    await repo.getByRole('button', { name: 'Review download', exact: true }).click();
    const variants = repo.getByRole('combobox');
    await expect(variants).toHaveValue('');
    expect(resolutions).toBe(0); expect(installs).toBe(0);
    await expect(repo.getByRole('button', { name: 'Download', exact: true })).toHaveCount(0);
    await variants.selectOption(choice.id);
    await expect(repo.getByRole('button', { name: 'Download', exact: true })).toBeEnabled();
    expect(resolutions).toBe(1); expect(installs).toBe(0);
    await expect(repo.getByText('Required free space', { exact: true })).toBeVisible();
  });
  for (const architecture of ['whisper', 'piper', 'zipformer-streaming', 'kokoro']) {
    test(`${architecture}: discover, preview, download, verify, repair and remove`, async ({ page, request }, info) => {
      test.setTimeout(90000);
      const category = ['whisper', 'zipformer-streaming'].includes(architecture) ? 'Speech recognition' : 'Speech generation';
      await page.getByRole('button', { name: category, exact: true }).click();
      await page.getByRole('searchbox').fill(architecture);
      await page.getByRole('button', { name: 'Search', exact: true }).click();
      const repo = page.locator('article.model-search-result').filter({ hasText: `fixture/${architecture}` });
      await repo.getByRole('button').click();
      await expect(page.getByLabel('Model variant')).toHaveCount(0);
      const preview = repo.locator('.package-download-details');
      await expect(preview.getByText('Required free space', { exact: true })).toBeVisible();
      await expect(preview.getByText(/cannot run here yet/)).toBeVisible();
      await preview.getByRole('button', { name: 'Download', exact: true }).click();
      // Navigating away and back must restore operation status, not re-submit.
      await page.getByRole('button', { name: 'Language', exact: true }).click();
      await page.getByRole('button', { name: category, exact: true }).click();
      const card = page.getByRole('region', { name: 'Installed', exact: true }).locator('article').filter({ hasText: `fixture/${architecture}` });
      await page.getByRole('tab', { name: 'Installed', exact: true }).click();
      await expect(card.getByRole('button', { name: 'Verify', exact: true })).toBeVisible({ timeout: 30000 });
      await card.getByRole('button', { name: 'Verify', exact: true }).click();
      await expect(card.getByText('Integrity checked', { exact: true })).toBeVisible();
      const work = (await (await request.get('/api/v2/models/operations')).json()).operations;
      const matches = work.filter((o: any) => o.target.package.architecture === architecture && !o.discarded);
      expect(matches).toHaveLength(1);
      expect(matches[0].state).toBe('complete');
      expect((await (await request.get('/v1/models')).json()).data.some((m: any) => m.id === matches[0].target.package.id)).toBe(false);
      // Exercise the shared repair endpoint on this synthetic package. Healthy
      // cards intentionally omit Repair.
      expect((await request.post('/api/v2/models/operations', { data: { action: 'repair', source_operation_id: matches[0].id, request_id: crypto.randomUUID() } })).status()).toBe(202);
      await expect.poll(async () => (await (await request.get('/api/v2/models/operations')).json()).operations.find((o: any) => o.action === 'repair' && o.target.package.architecture === architecture)?.state, { timeout: 30000 }).toBe('complete');
      await expect(card.getByRole('button', { name: 'Delete', exact: true })).toBeEnabled({ timeout: 10000 });
      await page.screenshot({ path: info.outputPath(`${architecture}-installed.png`), fullPage: true });
      await card.getByRole('button', { name: 'Delete', exact: true }).click();
      await page.getByRole('dialog').locator('.danger-button').click();
      await expect(card).toHaveCount(0);
    });
  }

  test('cancel, explicit resume, partial discard and all locale/theme layouts', async ({ page, request }, info) => {
    test.setTimeout(90000);
    const discovery = await (await request.get('/api/v2/models/discover?repository=fixture/whisper')).json();
    const resolution = await (await request.post('/api/v2/models/resolve', { data: { repository: discovery.repository, revision: discovery.revision, variant: discovery.choices[0].id, architecture: 'whisper' } })).json();
    const response = await request.post('/api/v2/models/operations', { data: { request_id: crypto.randomUUID(), action: 'install', resolution_id: resolution.id } });
    expect(response.status()).toBe(202);
    const operation = await response.json();
    await request.post(`/api/v2/models/operations/${operation.id}/cancel`);
    await expect.poll(async () => (await (await request.get(`/api/v2/models/operations/${operation.id}`)).json()).state).toBe('cancelled');
    await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
    const transfer = page.locator('article').filter({ has: page.getByRole('button', { name: 'Resume', exact: true }) });
    await expect(transfer).toHaveCount(1);
    await transfer.getByRole('button', { name: 'Resume', exact: true }).click();
    await expect.poll(async () => (await (await request.get(`/api/v2/models/operations/${operation.id}`)).json()).state, { timeout: 30000 }).toBe('complete');
    expect((await request.post(`/api/v2/models/packages/${operation.target.package.id}/${operation.target.package.revision}/remove`)).ok()).toBe(true);
    // A second cancelled request demonstrates explicit partial cleanup.
    const retry = await (await request.post('/api/v2/models/operations', { data: { request_id: crypto.randomUUID(), action: 'install', resolution_id: resolution.id } })).json();
    await request.post(`/api/v2/models/operations/${retry.id}/cancel`);
    await expect.poll(async () => (await (await request.get(`/api/v2/models/operations/${retry.id}`)).json()).state).toBe('cancelled');
    await expect(transfer).toHaveCount(1);
    await transfer.getByRole('button', { name: 'Discard partial data', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Discard partial data', exact: true }).click();
    await expect(transfer).toHaveCount(0);
    expect((await request.post(`/api/v2/models/operations/${retry.id}/resume`)).status()).toBe(409);
    for (const locale of ['en', 'fr', 'es', 'ar', 'sw', 'sn', 'nd', 'zu', 'de']) {
      for (const theme of ['light', 'dark']) {
        await page.evaluate(({ locale, theme }) => { localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.theme', theme); }, { locale, theme });
        await page.reload();
        await page.setViewportSize({ width: 390, height: 844 });
        const group = page.locator('.model-category-picker');
        await group.getByRole('button').nth(2).click();
        await expect(group.getByRole('button').nth(2)).toHaveAttribute('aria-pressed', 'true');
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        if (locale === 'ar') await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
        expect(await group.getByRole('button').allTextContents()).not.toContain('');
        if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`${locale}-${theme}-mobile.png`), fullPage: true });
      }
    }
  });
});
