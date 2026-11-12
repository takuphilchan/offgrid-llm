import { expect, test } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { craftFixture } from './helpers/craft-fixture';

// Deliberately opt-in: real UI, synthetic public demonstration data, no inference.
for (const theme of ['light', 'dark']) test(`capture documentation workspace (${theme})`, async ({ page }) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Set OFFGRID_DOC_CAPTURES=1 to refresh documentation images.');
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, 'en', theme);
  const model = 'Qwen2.5-7B-Instruct-Q4_K_M';
  await page.route('**/v1/models', r => r.fulfill({ json: { data: [{ id: model, type: 'chat', size: 4.4 * 1024 ** 3 }] } }));
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
      { role: 'assistant', content: '## A practical one-hour workshop\n\nStart with something participants can try themselves. Keep the first session focused on asking questions, working with a document, and understanding where their data is stored.\n\n| Time | Activity |\n| --- | --- |\n| 0–10 min | Introduce local AI and its limits |\n| 10–25 min | Try a question and a follow-up |\n| 25–45 min | Explore a shared sample document |\n| 45–60 min | Compare answers and discuss uncertainty |\n\n### Before the session\n\n- Install and test a model on the computers you will use.\n- Prepare a short, public sample document.\n- Pair participants so they can help each other.\n\n**A useful closing exercise:** ask everyone to identify one answer they would verify before relying on it.' },
    ], updated_at: '2026-11-07T00:00:00Z',
  };
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/Community%20workshop', r => r.fulfill({ json: session }));
  await page.route('**/v1/documents', r => r.fulfill({ json: { documents: [
    { id: 'workshop', name: 'Community workshop notes.txt', size: 4096, index_status: 'ready', source_retained: true, chunk_count: 4 },
    { id: 'guide', name: 'Local AI introduction.pdf', size: 204800, index_status: 'ready', source_retained: true, chunk_count: 12 },
    { id: 'checklist', name: 'Facilitator checklist.docx', size: 16384, index_status: 'ready', source_retained: true, chunk_count: 3 },
  ], count: 3 } }));
  await page.goto('/ui/#/chat');
  await page.locator('.history-open').first().click();
  await expect(page.getByRole('heading', { name: 'A practical one-hour workshop' })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: resolve(output, theme === 'dark' ? 'chat-dark.png' : 'chat-workspace.png'), animations: 'disabled' });
  if (theme === 'dark') return;
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-documents article')).toHaveCount(3);
  await page.screenshot({ path: resolve(output, 'knowledge-library.png'), animations: 'disabled' });
  await page.goto('/ui/#/agents');
  await expect(page.locator('.profile-list-item')).toHaveCount(1);
  await page.locator('.profile-list-item').click();
  await expect(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Workshop assistant');
  await page.screenshot({ path: resolve(output, 'agent-profiles.png'), animations: 'disabled' });
});
