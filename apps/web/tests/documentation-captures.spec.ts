import { expect, test } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { craftFixture } from './helpers/craft-fixture';

// Deliberately opt-in: real UI, synthetic public demonstration data, no inference.
for (const theme of ['light', 'dark']) test(`capture documentation workspace (${theme})`, async ({ page }) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Set OFFGRID_DOC_CAPTURES=1 to refresh documentation images.');
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, 'en', theme);
  await page.route('**/api/v2/system', r => r.fulfill({ json: {
    product: 'offgrid', version: 'documentation-preview', workspace_id: 'craft', api_version: 2,
    capabilities: ['task-first-agents-v2', 'agent-profiles-v1', 'session-files-v1',
      'knowledge-collections-v1', 'scoped-knowledge-chat-v1', 'document-comparison-jobs-v1',
      'document-analysis-history-v1', 'document-synthesis-candidates-v1'],
  } }));
  const model = 'Qwen2.5-7B-Instruct-Q4_K_M';
  await page.route('**/v1/models', r => r.fulfill({ json: { data: [
    { id: model, type: 'chat', size: Math.round(4.4 * 1024 ** 3) },
    { id: 'Qwen3.5-2B-Q4_K_M', type: 'chat', size: 1280835840 },
    { id: 'SmolLM3-Q4_K_M', type: 'chat', size: 1915305312 },
  ] } }));
  await page.route('**/v1/catalog', r => r.fulfill({ json: { models: [
    { id: 'qwen3.5-2b', name: 'Qwen3.5 2B', description: 'Small multilingual chat model. Bundle download includes its vision projector; image and agent readiness are checked separately.', parameters: '2B', repo: 'unsloth/Qwen3.5-2B-GGUF', file: 'Qwen3.5-2B-Q4_K_M.gguf', size_bytes: 1280835840, min_ram_gb: 6 },
    { id: 'smollm3-3b', name: 'SmolLM3 3B', description: 'Text-only multilingual model from Hugging Face. A compact alternative for chat and writing; does not accept images.', parameters: '3B', repo: 'ggml-org/SmolLM3-3B-GGUF', file: 'SmolLM3-Q4_K_M.gguf', size_bytes: 1915305312, min_ram_gb: 6 },
    { id: 'granite-4.1-3b', name: 'Granite 4.1 3B', description: 'IBM\'s compact text model for conversation and document tasks. Text only; tool use requires separate runtime checks.', parameters: '3B', repo: 'ibm-granite/granite-4.1-3b-GGUF', file: 'granite-4.1-3b-Q4_K_M.gguf', size_bytes: 2099501664, min_ram_gb: 6 },
  ].map(item => ({ ...item, type: 'chat', quant: 'Q4_K_M', license: 'Apache 2.0' })) } }));
  const profile = {
    id: 'researcher', actor: 'local', workspace: 'craft', revision: 1, digest: 'documentation-demo',
    settings: { name: 'Workshop assistant', model, instructions: 'Help prepare community workshops. Read approved sources, explain uncertainty, and keep activities accessible to beginners.', max_iterations: 20 },
  };
  await page.route('**/api/v2/agent-profiles', r => r.fulfill({ json: { profiles: [profile] } }));
  await page.route('**/api/v2/agent-profiles/researcher/effective**', r => r.fulfill({ json: { profile, ready: true, capabilities: {}, issues: [] } }));
  const output = resolve('../../build/documentation-captures');
  mkdirSync(output, { recursive: true });
  const session = {
    name: 'Community workshop', model_id: 'Qwen2.5-7B-Instruct-Q4_K_M',
    messages: [
      { role: 'user', content: 'Help me plan a one-hour introduction to local AI for our community workshop.' },
      { role: 'assistant', content: '## A practical one-hour workshop\n\nStart with a question, then try a sample document together.\n\n| Time | Activity |\n| --- | --- |\n| 0–10 min | Introduce local AI and its limits |\n| 10–25 min | Try a question and a follow-up |\n| 25–45 min | Explore a shared sample document |\n| 45–60 min | Compare answers and discuss uncertainty |\n\n**Before the session:** test your model and prepare a short, public sample document.' },
    ], updated_at: '2026-11-07T00:00:00Z',
  };
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/Community%20workshop', r => r.fulfill({ json: session }));
  await page.route('**/v1/documents', r => r.fulfill({ json: { documents: [
    { id: 'workshop', name: 'Community workshop notes.txt', content_type: 'text/plain', collection_id: 'workspace', size: 4096, index_status: 'ready', source_retained: true, chunk_count: 4 },
    { id: 'guide', name: 'Local AI introduction.pdf', content_type: 'application/pdf', collection_id: 'workspace', size: 204800, index_status: 'ready', source_retained: true, chunk_count: 12 },
    { id: 'checklist', name: 'Facilitator checklist.docx', content_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', collection_id: 'workspace', size: 16384, index_status: 'ready', source_retained: true, chunk_count: 3 },
  ], count: 3 } }));
  await page.goto('/ui/#/chat');
  await page.locator('.history-open').first().click();
  await expect(page.getByRole('heading', { name: 'A practical one-hour workshop' })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: resolve(output, theme === 'dark' ? 'chat-dark.png' : 'chat-workspace.png'), animations: 'disabled' });
  if (theme === 'dark') return;
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-documents article')).toHaveCount(3);
  await expect(page.getByRole('button', { name: /Compare documents/ }).first()).toBeVisible();
  await page.screenshot({ path: resolve(output, 'knowledge-library.png'), animations: 'disabled' });
  await page.goto('/ui/#/agents');
  await expect(page.locator('.profile-list-item')).toHaveCount(1);
  await page.locator('.profile-list-item').click();
  await expect(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Workshop assistant');
  await page.screenshot({ path: resolve(output, 'agent-profiles.png'), animations: 'disabled' });
  await page.goto('/ui/#/models');
  await expect(page.getByRole('region', { name: 'Installed', exact: true })).toContainText('Qwen');
  await page.screenshot({ path: resolve(output, 'models-installed.png'), animations: 'disabled' });
  await page.locator('.model-library-navigation').getByRole('button', { name: 'Discover models', exact: true }).click();
  await expect(page.locator('.catalog-card')).toHaveCount(3);
  await page.screenshot({ path: resolve(output, 'models-discover.png'), animations: 'disabled' });
  await page.goto('/ui/#/settings/appearance');
  await expect(page.getByRole('heading', { name: 'Appearance', exact: true })).toBeVisible();
  await page.screenshot({ path: resolve(output, 'settings-appearance.png'), animations: 'disabled' });
  await page.goto('/ui/#/settings/models');
  await expect(page.getByRole('heading', { name: 'Models & responses', exact: true })).toBeVisible();
  await page.screenshot({ path: resolve(output, 'settings-responses.png'), animations: 'disabled' });
});

test('capture documentation document reader', async ({ page }) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Set OFFGRID_DOC_CAPTURES=1 to refresh documentation images.');
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, 'en', 'light');
  await page.route('**/api/v2/system', r => r.fulfill({ json: {
    product: 'offgrid', workspace_id: 'craft', capabilities: ['session-files-v1'],
  } }));
  await page.route('**/v1/sessions', r => r.request().method() === 'GET'
    ? r.fulfill({ json: { sessions: [] } }) : r.fallback());
  const file = { id: 'file-' + 'a'.repeat(32), name: 'Workshop guide.pdf',
    sha256: 'b'.repeat(64), revision: 'c'.repeat(64), media_type: 'application/pdf',
    bytes: 204800, status: 'ready', error_code: '', created_at: '2026-11-16T00:00:00Z' };
  await page.route('**/v1/sessions/*/attachments', r => r.fulfill({ json:
    r.request().method() === 'POST' ? { attachment: file } : { attachments: [file] },
  }));
  await page.goto('/ui/#/chat');
  await page.evaluate(() => document.fonts.ready);
  // Authored demonstration page, served through the reader's normal raster API.
  // This capture exercises presentation, not PDF extraction or model inference.
  const raster = await page.evaluate(() => {
    const canvas = document.createElement('canvas'); canvas.width = 720; canvas.height = 960;
    const x = canvas.getContext('2d')!;
    x.fillStyle = '#ffffff'; x.fillRect(0, 0, 720, 960);
    x.fillStyle = '#667078'; x.font = '13px sans-serif';
    x.fillText('OFFGRID / DEMONSTRATION DOCUMENT', 64, 64);
    x.fillStyle = '#20252a'; x.font = 'bold 38px sans-serif';
    x.fillText('Local AI, together', 64, 148);
    x.fillStyle = '#5b636b'; x.font = '19px sans-serif';
    x.fillText('A one-hour community workshop', 64, 186);
    x.fillStyle = '#20252a'; x.font = 'bold 22px sans-serif';
    x.fillText('Learn by doing', 64, 266);
    x.font = '17px sans-serif';
    ['Ask a question. Explore a sample document. Compare',
      'the answer with its sources and discuss what is missing.'].forEach((line, i) => x.fillText(line, 64, 304 + i * 28));
    x.font = 'bold 22px sans-serif'; x.fillText('Your session at a glance', 64, 402);
    const rows = [['10 minutes', 'Local AI and its limits'], ['15 minutes', 'Questions and follow-ups'],
      ['20 minutes', 'A document and its sources'], ['15 minutes', 'Review and discussion']];
    rows.forEach(([time, task], i) => {
      const y = 440 + i * 58;
      x.fillStyle = i % 2 === 0 ? '#f1f4f6' : '#fafbfc'; x.fillRect(64, y, 592, 56);
      x.fillStyle = '#5b636b'; x.font = '16px sans-serif'; x.fillText(time, 80, y + 35);
      x.fillStyle = '#20252a'; x.fillText(task, 245, y + 35);
    });
    x.font = 'bold 22px sans-serif'; x.fillText('Before you arrive', 64, 738);
    x.font = '17px sans-serif';
    ['Test your model and prepare a short public document.',
      'Bring one question whose answer you can verify.'].forEach((line, i) => x.fillText(line, 64, 778 + i * 28));
    x.fillStyle = '#78828b'; x.font = '12px sans-serif';
    x.fillText('Sample content for the OffGrid interface tour · 1', 64, 904);
    return canvas.toDataURL().split(',')[1];
  });
  await page.route('**/attachments/*/preview*', r => new URL(r.request().url()).searchParams.has('page')
    ? r.fulfill({ contentType: 'image/png', body: Buffer.from(raster, 'base64') })
    : r.fulfill({ json: { name: file.name, sha256: file.sha256, format: 'pdf', pages: 1,
      state: 'complete', units: [{ kind: 'page', index: 1, state: 'ready', source_sha256: file.sha256,
        extractor_revision: 'documentation-fixture', text: 'Local AI, together. A one-hour community workshop.' }] } }));
  await expect(page.locator('.composer textarea')).toBeEnabled();
  await page.locator('.composer textarea').fill('Help me prepare for this workshop.');
  await page.locator('input[data-attachment=files]').setInputFiles({ name: file.name,
    mimeType: 'application/pdf', buffer: Buffer.from('%PDF-documentation-fixture') });
  await page.getByRole('button', { name: 'Open document', exact: true }).click();
  await expect(page.locator('.document-reader-surface img')).toBeVisible();
  const output = resolve('../../build/documentation-captures');
  mkdirSync(output, { recursive: true });
  await page.screenshot({ path: resolve(output, 'document-reader.png'), animations: 'disabled' });
});
