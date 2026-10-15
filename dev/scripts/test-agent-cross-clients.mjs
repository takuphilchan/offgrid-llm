// Companion fixture for TestAgentCrossClientIntegration: actual browser network
// requests to disposable Go handlers. UI layout is covered by agent-chat.spec.ts.
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
const require = createRequire(resolve(import.meta.dirname, '../../web/app/package.json'));
const { chromium } = require('@playwright/test');
const [origin, job, otherKey] = process.argv.slice(2);
const url = new URL(origin);
if (url.hostname !== '127.0.0.1' || !url.port || url.port === '11611') throw new Error('Disposable loopback fixture required');
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ extraHTTPHeaders: { Authorization: `Bearer ${process.env.OFFGRID_API_KEY}` } });
  const page = await context.newPage();
  await page.goto(origin);
  const result = await page.evaluate(async ({ job }) => {
    const get = await fetch(`/api/v2/jobs/${job}`);
    if (!get.ok) throw new Error(`Inspect: ${get.status}`);
    const saved = await get.json();
    if (saved.run_id !== job) throw new Error('Wrong canonical identity');
    const body = JSON.stringify({ request_id: 'browser-correction', instruction: 'Keep the final explanation concise; do not use tools.' });
    for (let retry = 0; retry < 2; retry++) {
      const response = await fetch(`/api/v2/jobs/${job}/steer`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body });
      if (!response.ok) throw new Error(`Steer: ${response.status}`);
      const current = await response.json();
      if (current.run_id !== job || current.pending_instructions.length !== 1) throw new Error('Correction duplicated or lost');
    }
    return { job_id: job, steered: true };
  }, { job });
  // Use a separate browser identity: context-level Authorization overrides
  // fetch headers, so changing a header inside Alice's page is not a Bob test.
  const other = await browser.newContext({ extraHTTPHeaders: { Authorization: `Bearer ${otherKey}` } });
  const otherPage = await other.newPage();
  await otherPage.goto(origin);
  await otherPage.evaluate(async job => {
    const body = JSON.stringify({ request_id: 'foreign-correction', instruction: 'Foreign instruction' });
    for (const [method, suffix] of [['GET', ''], ['POST', '/cancel'], ['POST', '/steer']]) {
      const denied = await fetch(`/api/v2/jobs/${job}${suffix}`, { method, headers: { 'Content-Type': 'application/json' }, ...(method === 'POST' ? { body } : {}) });
      if (denied.status !== 404) throw new Error(`Cross-actor access: ${denied.status}`);
    }
  }, job);
  console.log(JSON.stringify(result));
} finally { await browser.close(); }
