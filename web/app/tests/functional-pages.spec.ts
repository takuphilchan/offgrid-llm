import { expect, test } from '@playwright/test';

const catalogEmbedding = {
  id: 'bge-m3', name: 'BGE M3', description: 'Multilingual embedding model', category: 'embedding',
  parameters: '567M', size: '417 MB', size_bytes: 437800000, repo: 'smarttasks/bge-m3-GGUF',
  file: 'bge-m3-Q4_K_M.gguf', quant: 'Q4_K_M', min_ram_gb: 2, recommended: true,
  provider: 'BAAI', type: 'embedding', license: 'MIT'
};

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
});

test('knowledge setup downloads one stable model and enables RAG', async ({ page }) => {
  let enabled = false;
  let installed = false;
  let downloadRequests = 0;
  let enableRequests = 0;
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy', version: 'test' } }));
  await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: false, user: null } }));
  await page.route('**/v1/models', route => route.fulfill({ json: { object: 'list', data: installed ? [{ id: 'bge-m3', type: 'embedding', size: 437800000 }] : [{ id: 'chat-model', type: 'llm', size: 1000 }] } }));
  await page.route('**/v1/catalog', route => route.fulfill({ json: { total: 1, models: [catalogEmbedding] } }));
  await page.route('**/v1/documents', route => route.fulfill({ json: { count: 0, documents: [] } }));
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled, embedding_model: enabled ? 'bge-m3' : '', stats: {} } }));
  await page.route('**/v1/models/download', async route => {
    downloadRequests++;
    const body = route.request().postDataJSON();
    expect(body.model_id).toBe('bge-m3');
    expect(body.file_name).toBe('bge-m3-Q4_K_M.gguf');
    await route.fulfill({ json: { success: true, status: 'downloading', file_name: 'bge-m3.gguf' } });
  });
  await page.route('**/v1/models/download/progress', route => {
    installed = true;
    return route.fulfill({ json: { 'bge-m3.gguf': { file_name: 'bge-m3.gguf', bytes_total: 437800000, bytes_done: 437800000, percent: 100, speed: 1, started_at: 1, status: 'complete' } } });
  });
  await page.route('**/v1/rag/enable', async route => { enableRequests++; enabled = true; await route.fulfill({ json: { success: true, message: 'enabled' } }); });

  await page.goto('/ui/#/knowledge');
  await expect(page.getByRole('heading', { name: 'Knowledge retrieval is not enabled.' })).toBeVisible();
  await page.getByRole('button', { name: 'Install and enable' }).dblclick();
  await expect.poll(() => downloadRequests).toBe(1);
  await expect.poll(() => enableRequests).toBe(1);
  await expect(page.getByText(/Active embedding model: bge-m3/)).toBeVisible();
});

test('models page starts only one download for a rapid repeated click', async ({ page }) => {
  let downloadRequests = 0;
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy', version: 'test' } }));
  await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: false, user: null } }));
  await page.route('**/v1/models', route => route.fulfill({ json: { object: 'list', data: [] } }));
  await page.route('**/v1/catalog', route => route.fulfill({ json: { total: 1, models: [catalogEmbedding] } }));
  await page.route('**/v1/models/download/progress', route => route.fulfill({ json: {} }));
  await page.route('**/v1/models/download', async route => {
    downloadRequests++;
    await new Promise(resolve => setTimeout(resolve, 50));
    await route.fulfill({ json: { success: true, status: 'downloading', file_name: 'bge-m3.gguf' } });
  });

  await page.goto('/ui/#/models');
  await expect(page.getByRole('heading', { name: 'BGE M3' })).toBeVisible();
  await page.getByRole('button', { name: 'Download' }).dblclick();
  await expect.poll(() => downloadRequests).toBe(1);
});

test('agents page exposes runtime tools, durable tasks, MCP, and computer status', async ({ page }) => {
  let toolEnabled = true;
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy', version: 'test' } }));
  await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: false, user: null } }));
  await page.route('**/v1/models', route => route.fulfill({ json: { object: 'list', data: [{ id: 'chat-model', type: 'llm', size: 1000 }] } }));
  await page.route('**/v1/agents/tools?all=true', async route => {
    if (route.request().method() === 'PATCH') toolEnabled = !toolEnabled;
    await route.fulfill({ json: { total: 1, enabled_count: toolEnabled ? 1 : 0, tools: [{ name: 'calculator', description: 'Calculate an expression', source: 'builtin', enabled: toolEnabled, capability: { name: 'calculator', namespace: 'tool', source: 'builtin', kind: 'execute', risk: 'low' } }] } });
  });
  await page.route('**/v1/agents/tasks', route => route.fulfill({ json: [{ id: 'run-1', prompt: 'Summarize the report', status: 'completed', created_at: new Date().toISOString() }] }));
  await page.route('**/v1/agents/mcp', route => route.fulfill({ json: { servers: [{ name: 'local-docs', url: 'http://127.0.0.1:3000/mcp', transport: 'http', tools: 2, status: 'connected' }] } }));
  await page.route('**/v1/computer/status', route => route.fulfill({ json: { available: false, emergency_stop: false, active_sessions: 0 } }));

  await page.goto('/ui/#/agents');
  await expect(page.getByText('1/1 enabled').first()).toBeVisible();
  await expect(page.getByText('Summarize the report')).toBeVisible();
  await expect(page.getByText('local-docs')).toBeVisible();
  await expect(page.getByText('Driver unavailable')).toBeVisible();
  await expect(page.getByText('Calculate an expression')).toBeVisible();
});
