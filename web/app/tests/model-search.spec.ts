import { expect, test } from '@playwright/test';

test('model discovery is explicit, offers large quantizations, and downloads only the chosen file', async ({ page }) => {
  let searches = 0, fileQueries = 0, downloads = 0;
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: false, user: null } }));
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [] } }));
  await page.route('**/v1/catalog', route => route.fulfill({ json: { models: [] } }));
  await page.route('**/v1/models/download/progress', route => route.fulfill({ json: {} }));
  await page.route('**/v1/search?*', route => {
    searches++;
    expect(new URL(route.request().url()).searchParams.get('query')).toBe('large model');
    return route.fulfill({ json: { total: 1, results: [{ id: 'owner/large-GGUF', author: 'owner', name: 'large-GGUF', downloads: 5, likes: 1 }] } });
  });
  await page.route('**/v1/search/files?*', route => {
    fileQueries++;
    expect(new URL(route.request().url()).searchParams.get('repo')).toBe('owner/large-GGUF');
    return route.fulfill({ json: { repo: 'owner/large-GGUF', files: [
      { id: 'model-q4-aaa', file: 'nested/model.Q4_K_M.gguf', quant: 'Q4_K_M', size_bytes: 40 * 1024 ** 3, supported: true },
      { id: 'model-q8-bbb', file: 'nested/model.Q8_0.gguf', quant: 'Q8_0', size_bytes: 80 * 1024 ** 3, supported: true },
      { id: 'model-split-ccc', file: 'model-00001-of-00002.gguf', quant: 'Q8_0', size_bytes: 1024, supported: false, reason: 'split_weights' }
    ] } });
  });
  await page.route('**/v1/models/download', async route => {
    downloads++;
    expect(route.request().postDataJSON()).toMatchObject({ model_id: 'model-q8-bbb', repository: 'owner/large-GGUF', file_name: 'nested/model.Q8_0.gguf' });
    await new Promise(resolve => setTimeout(resolve, 300));
    await route.fulfill({ json: { success: true, file_name: 'model-q8-bbb.gguf' } });
  });
  await page.goto('/ui/#/models');
  const search = page.getByRole('region', { name: 'Find more models' });
  await expect(search).toBeVisible();
  expect(searches).toBe(0);
  await search.getByRole('searchbox').fill('large model');
  expect(searches).toBe(0);
  await search.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(search.getByRole('heading', { name: 'owner/large-GGUF' })).toBeVisible();
  expect(fileQueries).toBe(0);
  await search.getByRole('button', { name: 'Choose files' }).click();
  await expect(search.getByRole('combobox')).toHaveValue('model-q4-aaa');
  await expect(search.getByRole('option', { name: /80.0 GB/ })).toHaveCount(1);
  await expect(search.getByRole('option', { name: /model-00001/ })).toBeDisabled();
  await search.getByRole('combobox').selectOption('model-q8-bbb');
  await search.getByRole('button', { name: 'Download', exact: true }).dblclick();
  await expect.poll(() => downloads).toBe(1);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  // Recover from an upstream error: no false "no results" success state.
  await page.route('**/v1/search?*', route => route.fulfill({ status: 502, json: { error: 'Unavailable' } }));
  await search.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(search.getByRole('alert')).toContainText('unavailable');
  await expect(search.getByText('No matching public GGUF repositories. Try a different name.')).toHaveCount(0);
});
