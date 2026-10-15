import { expect, type Page } from '@playwright/test';

/** Exercise the public conversation API while retaining a fixture's canonical job. */
export async function installChatJob(page: Page, options: {
  workspace: string;
  model: string;
  snapshot: () => any;
  submit: (payload: any) => any | 'lost';
  onWrite?: (path: string, payload: any) => void;
}) {
  const profile = { id: 'task-agent', revision: 1, actor: 'local', workspace: options.workspace,
    archived: false, digest: 'fixture', settings: { name: 'Task agent', description: '', model: options.model,
      instructions: '', tools: [], knowledge: [], computer: true, max_iterations: 10 } };
  let session: any = null, accepted: any = null;
  const turn = () => accepted ? { id: accepted.request_id, prompt: accepted.content, model: options.model,
    status: options.snapshot().status, output: options.snapshot().output ?? '', job_id: options.snapshot().run_id,
    job_admission: 'linked', agent: { id: profile.id, revision: 1 } } : null;
  const saved = () => {
    if (session && accepted && options.snapshot().status === 'completed') session.messages = [
      {role:'user', content:accepted.content, turn_id:accepted.request_id, job_id:options.snapshot().run_id},
      {role:'assistant', content:options.snapshot().output ?? '', turn_id:accepted.request_id, job_id:options.snapshot().run_id},
    ];
    return session;
  };
  await page.route('**/api/v2/system', route => route.fulfill({json:{product:'offgrid',api_version:2,version:'fixture',workspace_id:options.workspace,
    capabilities:['agent-profiles-v1','agent-conversations-v1','task-first-agents-v2','durable-agent-events-v2']}}));
  await page.route('**/api/v2/agent-profiles', route => route.fulfill({json:{profiles:[profile]}}));
  await page.route(/\/v1\/sessions(?:\/|$)/, async route => {
    const request = route.request(), path = decodeURIComponent(new URL(request.url()).pathname);
    if(request.method()!=='GET') options.onWrite?.(path,request.postDataJSON());
    if (path === '/v1/sessions') {
      if (request.method() === 'POST') {
        session = {name:request.postDataJSON().name,model_id:options.model,messages:[],jobs:[],created_at:'2026-10-15T00:00:00Z',updated_at:'2026-10-15T00:00:00Z'};
        accepted = null;
        return route.fulfill({json:session});
      }
      return route.fulfill({json:{sessions:session ? [saved()] : []}});
    }
    if (path.endsWith('/generate')) {
      const payload = request.postDataJSON(), result = options.submit(payload);
      if (result === 'lost') return route.abort('failed');
      accepted = payload;
      session.jobs = [{turn_id:accepted.request_id,job_id:options.snapshot().run_id}];
      return route.fulfill({status:202,json:{turn:turn(),job:result ?? options.snapshot()}});
    }
    if (path.endsWith('/turn')) return route.fulfill({json:{turn:turn()}});
    if (path.endsWith('/turn/recover')) { saved(); return route.fulfill({json:{turn:turn(),job:options.snapshot()}}); }
    return route.fulfill({json:saved()});
  });
}

export async function openAgentChat(page: Page) {
  await page.goto('/ui/#/chat');
  await selectTaskAgent(page);
}

export async function selectTaskAgent(page: Page) {
  const selector = page.locator('.chat-agent-selector > button');
  await selector.click();
  await page.getByRole('radio', {name:/^Task agent /}).check();
  await page.keyboard.press('Escape');
  await expect(selector).toBeFocused();
}

export async function sendAgentMessage(page: Page, text: string) {
  await page.locator('.composer textarea').fill(text);
  await page.locator('.composer-actions').getByRole('button', {name:'Send',exact:true}).click();
}
