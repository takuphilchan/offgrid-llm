import { expect, test, type Page } from '@playwright/test';
import { locales, type LocaleCode } from '../src/i18n';
import { voiceSettingsText } from '../src/i18n/voice-settings';
import { speechFeedback } from '../src/i18n/speech-feedback';
import { workspaceManagement } from '../src/i18n/workspace-management';
import { modelLibrary } from '../src/i18n/model-library';

// Renderer fixtures: no microphone, native input, downloads or model inference.
async function fixture(page: Page, locale: LocaleCode = 'en', theme = 'light', history = 0) {
  await page.addInitScript(({ locale, theme }) => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.theme', theme);
  }, { locale, theme });
  const writes: string[] = [];
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:api\/v2|v1)\//, r => {
    const path = new URL(r.request().url()).pathname;
    if (r.request().method() !== 'GET') writes.push(path);
    const bodies: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', version: 'quality', workspace_id: 'quality', api_version: 2, capabilities: ['task-first-agents-v2'] },
      '/v1/users/me': { authenticated: false, user: null }, '/v1/system/config': { version: 'quality', inference_slots: 1 },
      '/v1/models': { data: [{ id: 'quality-model.Q4_K_M', type: 'chat', size: 1024 ** 3 }] },
      '/v1/catalog': { models: [] }, '/v1/models/download/progress': {},
      '/api/v2/models': { models: [] }, '/api/v2/models/operations': { operations: [] },
      '/api/v2/models/catalog': { models: [], repositories: [] },
      '/v1/sessions': { sessions: Array.from({ length: history }, (_, i) => ({ name: `Conversation ${i}`, model_id: 'quality-model.Q4_K_M', messages: [], updated_at: '2026-10-08T00:00:00Z' })) },
      '/v1/rag/status': { enabled: true, embedding_model: 'fixture-embedding', stats: {} },
      '/v1/documents': { documents: [{ id: 'guide', name: 'Guide — دليل — 日本語.txt', size: 1256, index_status: 'ready', source_retained: true, chunk_count: 2 }] },
      '/api/v2/jobs': [], '/v1/agents/tasks': [], '/v1/runs': { runs: [] }, '/v1/stats': { server: { uptime: '1m', version: 'quality' } },
      '/api/v2/computer/status': { available: false }, '/v1/integrations': { integrations: [] },
      '/v1/agents/tools': { tools: [{ name: 'read_document', description: 'Fixture tool', source: 'builtin', enabled: false }], total: 1, enabled_count: 0 },
      '/v1/agents/mcp': { servers: [{ name: 'Documentation connection with a long name', tools: 2, transport: 'http', status: 'disconnected' }] },
      '/v1/audio/status': { profiles: [{ id: 'asr', revision: 'r1', name: 'Whisper fixture', capabilities: ['transcription'], available: true }, { id: 'tts', revision: 'r1', name: 'Piper fixture', capabilities: ['speech_synthesis'], available: true, voices: [{ id: 'voice-one', language: 'en' }] }] },
    };
    if (path.endsWith('/turn')) return r.fulfill({ json: { turn: null } });
    return r.fulfill({ json: bodies[path] ?? {} });
  });
  return writes;
}

test('all new presentation resources have nine complete typed dictionaries', () => {
  for (const resource of [voiceSettingsText, speechFeedback, workspaceManagement, modelLibrary]) {
    const expected = Object.keys(resource('en')).sort();
    for (const locale of Object.keys(locales) as LocaleCode[]) {
      const actual = resource(locale);
      expect(Object.keys(actual).sort()).toEqual(expected);
      for (const value of Object.values(actual)) expect(typeof value === 'string' && value.trim().length > 0).toBe(true);
    }
  }
});

test('one header refresh reloads page data without resetting drafts or dispatching work', async ({ page }) => {
  const writes = await fixture(page);
  const reads: string[] = [];
  page.on('request', request => { if (request.method() === 'GET') reads.push(new URL(request.url()).pathname); });
  const refresh = async (paths: string[]) => {
    const button = page.getByRole('button', { name: 'Refresh', exact: true });
    await expect(button).toHaveCount(1);
    await expect(button).toBeEnabled();
    const start = reads.length;
    await button.click();
    await expect.poll(() => paths.every(path => reads.slice(start).includes(path))).toBe(true);
    await expect(button).toBeEnabled();
    await expect(page.locator('.page-failure')).toHaveCount(0);
  };
  for (const [route, paths] of [
    ['knowledge', ['/v1/documents', '/v1/rag/status']],
    ['activity', ['/v1/runs']],
    ['agents/tools', ['/v1/agents/tools']],
    ['agents/connections', ['/v1/agents/mcp']],
  ] as const) {
    await page.goto(`/ui/#/${route}`);
    await refresh([...paths]);
  }
  await page.goto('/ui/#/models');
  for (const category of ['Language', 'Embeddings', 'Speech recognition', 'Speech generation']) {
    await page.getByRole('button', { name: category, exact: true }).click();
    for (const view of ['Installed', 'Discover models']) {
      await page.getByRole('tab', { name: view, exact: true }).click();
      await refresh(category.startsWith('Speech') ? ['/api/v2/models', '/api/v2/models/catalog'] : ['/v1/models', '/v1/catalog']);
    }
  }
  for (const [route, selector] of [['chat', '.composer textarea'], ['agents', '.task-prompt textarea']]) {
    await page.goto(`/ui/#/${route}`);
    const draft = page.locator(selector);
    await draft.fill('Keep this draft while refreshing');
    await refresh(['/v1/models']);
    await expect(draft).toHaveValue('Keep this draft while refreshing');
  }
  expect(writes).toEqual([]);
});

test('workspace proportions prioritize the editor and keep service navigation in Settings', async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const writes = await fixture(page);
  await page.goto('/ui/#/chat');
  await expect(page.locator('.topbar')).toHaveCSS('min-height', '64px');
  await expect(page.locator('.composer')).toHaveCSS('max-width', '800px');
  await expect(page.getByRole('link', { name: 'Service details', exact: true })).toHaveCount(0);
  await page.goto('/ui/#/agents');
  await expect(page.getByText('No agent tasks have run yet.', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'New task', exact: true })).toHaveCount(0);
  await expect(page.locator('.task-entry')).toHaveCSS('border-top-width', '0px');
  await expect(page.locator('.task-prompt')).toHaveCSS('border-top-width', '1px');
  await page.screenshot({ path: info.outputPath('agent-editor-desktop.png'), fullPage: true });
  await page.goto('/ui/#/models');
  const row = page.locator('.installed-model').first();
  await expect(row).toBeVisible();
  await expect(row.getByRole('button', { name: 'quality-model.Q4_K_M', exact: true })).toHaveCSS('border-top-width', '1px');
  expect((await row.boundingBox())!.height).toBeLessThanOrEqual(150);
  await page.screenshot({ path: info.outputPath('model-list-desktop.png'), fullPage: true });
  await page.goto('/ui/#/settings');
  await expect(page.locator('.settings-page')).toHaveCSS('max-width', '960px');
  expect(writes).toEqual([]);
});

for (const locale of Object.keys(locales) as LocaleCode[]) test(`voice preferences and workflow labels follow ${locale} without English fallback`, async ({ page }) => {
  const writes = await fixture(page, locale), copy = voiceSettingsText(locale);
  for (const route of ['chat', 'agents']) {
    await page.goto(`/ui/#/${route}`);
    const trigger = page.getByRole('button', { name: copy.title, exact: true });
    await trigger.click();
    await expect(page.getByLabel(copy.recognition, { exact: true })).toBeEnabled();
    await expect(page.getByLabel(copy.synthesis, { exact: true })).toBeEnabled();
    await expect(page.getByLabel(copy.voice, { exact: true })).toBeEnabled();
    await expect(page.getByRole('link', { name: copy.manage, exact: true })).toBeVisible();
    await expect(page.getByText(copy.retention, { exact: true })).toBeVisible();
    if (locale !== 'en') await expect(page.getByText(voiceSettingsText('en').retention, { exact: true })).toHaveCount(0);
    await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
  }
  expect(writes).toEqual([]);
});

for (const theme of ['light', 'dark']) for (const profile of [
  { width: 320, height: 480 }, { width: 390, height: 844 }, { width: 1280, height: 400 },
  // Browser zoom also reduces the CSS viewport. This is a reflow fixture;
  // actual Electron setZoomFactor coverage lives in the packaged theme check.
  { width: 640, height: 450, zoom: 2 },
]) test(`all pages remain reachable: ${theme} ${profile.width}x${profile.height} zoom ${profile.zoom ?? 1}`, async ({ page }, info) => {
  await page.setViewportSize(profile); await page.emulateMedia({ reducedMotion: 'reduce' });
  const writes = await fixture(page, 'en', theme);
  for (const route of ['chat', 'agents', 'models', 'knowledge', 'activity', 'settings', 'agents/tools', 'agents/connections']) {
    await page.goto(`/ui/#/${route}`);
    await expect(page.locator('.page-content')).toBeVisible();
    await expect(page.locator('.page-failure')).toHaveCount(0);
    await page.evaluate(() => document.fonts.ready);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
    const last = page.locator('.page-content button:visible:not(:disabled), .page-content a:visible').last();
    if (await last.count()) {
      await last.focus(); await last.scrollIntoViewIfNeeded(); await expect(last).toBeFocused();
      const obscured = await last.evaluate(element => {
        const box = element.getBoundingClientRect();
        const at = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
        return !at || !(element === at || element.contains(at));
      });
      expect(obscured, `${route}: keyboard action is covered`).toBe(false);
    }
    await page.screenshot({ path: info.outputPath(`${route.replace('/', '-')}.png`), fullPage: true });
  }
  expect(writes).toEqual([]);
});

for (const theme of ['light', 'dark']) test(`essential normal text meets 4.5:1 contrast in ${theme}`, async ({ page }) => {
  await fixture(page, 'en', theme);
  for (const route of ['chat', 'agents', 'models', 'knowledge', 'activity', 'settings', 'agents/tools', 'agents/connections']) {
    await page.goto(`/ui/#/${route}`);
    await expect(page.locator('.page-content')).toBeVisible();
    const failures = await page.locator('.page-content').evaluate(root => {
      const rgb = (color: string) => (color.match(/[\d.]+/g) ?? []).map(Number);
      const lum = (c: number[]) => c.slice(0, 3).map(n => n / 255).map(n => n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4).reduce((s, n, i) => s + n * [.2126, .7152, .0722][i], 0);
      return [...root.querySelectorAll('p, small, h2, h3, dt, dd, label > span, button, a')].filter(el => {
        return el.getClientRects().length && getComputedStyle(el).visibility !== 'hidden' && !el.closest(':disabled, [popover]:not(:popover-open)') && el.textContent?.trim();
      }).flatMap(el => {
        const style = getComputedStyle(el), fg = rgb(style.color);
        let parent: Element | null = el, bg = [255, 255, 255];
        while (parent) { const value = rgb(getComputedStyle(parent).backgroundColor); if (value.length === 3 || value[3] === 1) { bg = value; break; } parent = parent.parentElement; }
        const a = lum(fg), b = lum(bg), ratio = (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
        return ratio < 4.5 ? [{ text: el.textContent!.trim().slice(0, 70), ratio: Number(ratio.toFixed(2)), fg, bg }] : [];
      });
    });
    expect(failures, route).toEqual([]);
  }
});

test('IME input, reduced motion, focus restoration and status announcements remain separate from submission', async ({ page }) => {
  const writes = await fixture(page); await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/ui/#/chat');
  const input = page.locator('.composer textarea');
  await input.fill('日本語の入力 — مرحباً');
  await input.dispatchEvent('keydown', { key: 'Enter', code: 'Enter', isComposing: true });
  expect(writes).toEqual([]);
  const trigger = page.getByRole('button', { name: 'Voice settings', exact: true });
  await trigger.focus(); await trigger.press('Enter');
  await expect(page.getByLabel('Recognition model', { exact: true })).toBeEnabled();
  await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
  await expect(input).toHaveValue('日本語の入力 — مرحباً');
  const transition = await trigger.evaluate(el => getComputedStyle(el).transitionDuration);
  expect(transition.split(',').every(value => parseFloat(value) <= .001)).toBe(true);
  await page.route('**/v1/audio/status', r => r.fulfill({ status: 503, json: { error: 'Fixture unavailable' } }));
  await trigger.click(); await expect(page.getByRole('alert')).toContainText('Fixture unavailable');
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
  expect(writes).toEqual([]);
});

test('bounded history filtering and warm controls meet the fixture feedback budget', async ({ page }, info) => {
  const writes = await fixture(page, 'en', 'light', 1000);
  const routes: Record<string, number> = {};
  for (const route of ['chat', 'models', 'settings', 'agents', 'knowledge', 'activity']) {
    const start = Date.now(); await page.goto(`/ui/#/${route}`); await expect(page.locator('.topbar h1')).toBeVisible();
    routes[route] = Date.now() - start;
  }
  await page.goto('/ui/#/chat'); await expect(page.getByRole('searchbox', { name: 'Search conversations' })).toBeVisible();
  const samples = await page.getByRole('searchbox', { name: 'Search conversations' }).evaluate(async input => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    const durations: number[] = [];
    for (let i = 0; i < 30; i++) {
      const start = performance.now(); setter.call(input, i % 2 ? '' : 'Conversation 99');
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
      durations.push(performance.now() - start);
    }
    return durations.sort((a, b) => a - b);
  });
  const p95 = samples[Math.ceil(samples.length * .95) - 1];
  expect(p95).toBeLessThanOrEqual(250);
  const rows = await page.locator('.history-list .history-row').count();
  console.log(JSON.stringify({ fixture: 'history-feedback', p95, routes, rows, history: 1000 }));
  await info.attach('ui-performance.json', { body: JSON.stringify({ samples, p95, routes, rows, history: 1000, kind: 'API fixtures; route navigation includes local transport; no inference' }, null, 2), contentType: 'application/json' });
  expect(rows).toBe(20);
  await page.getByRole('button', { name: 'Show more (980)', exact: true }).click();
  await expect(page.locator('.history-list .history-row')).toHaveCount(40);
  expect(writes).toEqual([]);
});
