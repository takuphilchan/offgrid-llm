import { expect, test } from '@playwright/test';

test('a completed transfer stays cancellable while finalizing, then a failed promotion resumes', async ({ page }) => {
  let state = 'finalizing';
  let installed = false;
  let downloads = 0;
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy', version: 'test' } }));
  await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: false, user: null } }));
  await page.route('**/v1/models', route => route.fulfill({ json: { data: installed ? [{ id: 'tinyllama', type: 'llm', size: 1000 }] : [] } }));
  await page.route('**/v1/catalog', route => route.fulfill({ json: { models: [
    { id: 'tinyllama', name: 'TinyLlama', description: 'Fixture', parameters: '1.1B', size_bytes: 1000, min_ram_gb: 2, repo: 'fixture/model', file: 'tinyllama.gguf', quant: 'Q4_K_M' },
    { id: 'other', name: 'Other model', description: 'Fixture', parameters: '1B', size_bytes: 1000, min_ram_gb: 2, repo: 'fixture/other', file: 'other.gguf', quant: 'Q4_K_M' }
  ] } }));
  await page.route('**/v1/models/download/progress', route => route.fulfill({ json: {
    'tinyllama.gguf': { file_name: 'tinyllama.gguf', status: state, bytes_done: 1000, bytes_total: 1000, percent: 100, speed: 0, started_at: 1, error: state === 'failed' ? 'File temporarily in use' : '' }
  } }));
  await page.route('**/v1/models/download', async route => {
    expect(route.request().postDataJSON().model_id).toBe('tinyllama');
    downloads++;
    state = 'finalizing';
    await route.fulfill({ json: { success: true, status: state, file_name: 'tinyllama.gguf' } });
  });
  await page.goto('/ui/#/models');
  const card = page.locator('.catalog-card').filter({ has: page.getByRole('heading', { name: 'TinyLlama', exact: true }) });
  await expect(card.getByRole('status')).toHaveText('Preparing model · 100.0%');
  await expect(card.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
  await expect(card.getByRole('button', { name: 'Download', exact: true })).toHaveCount(0);
  state = 'failed';
  await expect(card.getByText(/Downloaded data is kept/)).toBeVisible();
  await card.getByRole('button', { name: 'Resume', exact: true }).dblclick();
  await expect.poll(() => downloads).toBe(1);
  await expect(card.getByRole('status')).toHaveText('Preparing model · 100.0%');
  installed = true; state = 'complete';
  await expect(card.getByRole('button', { name: 'Installed', exact: true })).toBeDisabled();
  await expect(page.locator('.installed-model')).toContainText('tinyllama');
  await expect(page.locator('.catalog-card').filter({ hasText: 'Other model' }).getByRole('button', { name: 'Download', exact: true })).toBeEnabled();
});
