import { expect, test } from '@playwright/test';

// Presentation fixtures, never inference or OS-control qualification. The same
// workspace states let us compare each route before/after the redesign.
for (const state of ['empty', 'populated', 'busy', 'blocked', 'error', 'completed']) {
  test(`workspace comparison: ${state}`, async ({ page }, info) => {
    const id = `run-${'a'.repeat(32)}`, time = '2026-10-08T10:00:00Z';
    const populated = !['empty', 'error'].includes(state);
    const status = state === 'busy' ? 'running' : state === 'blocked' ? 'waiting_for_input' : 'completed';
    const task = { id, run_id: id, prompt: 'Summarize the permitted document', model: 'fixture-model', status, steps: [], created_at: time, updated_at: time, deletable: status === 'completed', output: status === 'completed' ? 'A saved fixture answer. This is not an independent verification claim.' : '', pending_input: state === 'blocked' ? { id: 'input-fixture', kind: 'computer', mode: 'app', target: 'Text editor' } : null };
    const session = { name: 'Document discussion', model_id: 'fixture-model', messages: [{ role: 'user', content: 'What are the next steps?' }, { role: 'assistant', content: '## Next steps\n\nReview the document and confirm the owner.\n\n- Review\n- Confirm' }], created_at: time, updated_at: time };
    await page.addInitScript(({ populated }) => {
      localStorage.setItem('offgrid.onboarding.complete', 'true');
      localStorage.setItem('offgrid.locale', 'en');
      localStorage.setItem('offgrid.theme', 'light');
      if (populated) localStorage.setItem('offgrid.active-session:local', 'Document discussion');
    }, { populated });
    await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
    await page.route(/\/(v1|api\/v2)\//, r => {
      const path = new URL(r.request().url()).pathname;
      if (r.request().method() !== 'GET') throw new Error(`Baseline must not mutate: ${path}`);
      if (state === 'error' && ['/v1/rag/status', '/v1/catalog', '/v1/documents', '/v1/runs', '/api/v2/jobs'].includes(path)) return r.fulfill({ status: 503, json: { error: { message: 'Fixture: status temporarily unavailable' } } });
      if (path.endsWith('/events') && path.includes('/jobs/')) return r.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify({ ...task, type: 'snapshot' })}\n\n` });
      const data: Record<string, unknown> = {
        '/api/v2/system': { product: 'offgrid', version: 'fixture', workspace_id: 'comparison', api_version: 2, ui_build_id: 'a'.repeat(64), capabilities: ['task-first-agents-v2', 'durable-agent-events-v2'] },
        '/v1/users/me': { authenticated: false, user: null },
        '/v1/models': { data: state === 'blocked' ? [] : [{ id: 'fixture-model', type: 'chat', size: 1024 ** 3 }] },
        '/v1/system/config': { version: 'fixture', inference_slots: 1 },
        '/v1/sessions': { sessions: populated ? [session] : [] },
        '/v1/sessions/Document%20discussion': session,
        '/v1/sessions/Document%20discussion/turn': { turn: null },
        '/v1/catalog': { models: [] },
        '/v1/documents': { documents: populated ? [{ id: 'source', name: 'Project notes.txt', size: 300, chunk_count: 1, source_retained: true }] : [], count: populated ? 1 : 0 },
        '/v1/rag/status': { enabled: state !== 'blocked', embedding_model: 'fixture-embedding', stats: {} },
        '/api/v2/jobs': populated ? [task] : [], [`/api/v2/jobs/${id}`]: task,
        '/v1/runs': { runs: populated ? [{ ...task, event_count: 1, data: { prompt: task.prompt } }] : [] },
        '/v1/stats': { server: { version: 'fixture', uptime: '5m' }, inference: { aggregate: { total_requests: 2 } } },
        '/api/v2/computer/status': { available: false, active_sessions: 0, emergency_stop: false },
      };
      return r.fulfill({ json: data[path] ?? {} });
    });
    const profiles = state === 'empty' ? [
      { width: 1440, height: 900 }, { width: 1280, height: 720 },
      { width: 390, height: 844 }, { width: 320, height: 480 },
      { width: 1280, height: 400 }, { width: 720, height: 450 },
    ] : [{ width: 1440, height: 900 }];
    for (const viewport of profiles) {
      await page.setViewportSize(viewport);
      for (const route of ['chat', 'agents', 'models', 'knowledge', 'activity', 'settings']) {
        await page.goto(`/ui/#/${route === 'agents' && populated ? `agents/task/${id}` : route}`);
        await expect(page.locator('.page-content')).toBeVisible();
        await expect(page.locator('.page-failure')).toHaveCount(0);
        await page.evaluate(() => document.fonts.ready);
        await page.screenshot({ path: info.outputPath(`${route}-${viewport.width}x${viewport.height}.png`), fullPage: true });
      }
    }
  });
}
