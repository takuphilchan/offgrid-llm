import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const date = new Date().toISOString();
  let chats = ['Keep this', 'Delete 日本語', 'Delete second'].map(name => ({ name, model_id: 'model', messages: [{role:'user',content:'Saved text'}], created_at: date, updated_at: date }));
  let tasks = Array.from({ length: 25 }, (_, i) => ({ id: `task-${i}`, prompt: `Task ${String(i).padStart(2,'0')}`, status: i === 0 ? 'running' : i === 1 ? 'uncertain' : 'completed', deletable: i > 1, created_at: date }));
  const calls: string[] = [];
  let failing = '';
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale','en'); localStorage.setItem('offgrid.onboarding.complete','true'); });
  await page.route('**/health', r => r.fulfill({ json: {status:'healthy'} }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: {product:'offgrid',version:'test',api_version:2,capabilities:[]} }));
  await page.route('**/v1/**', async r => {
    const path = decodeURIComponent(new URL(r.request().url()).pathname);
    let json: unknown = {};
    if (path === '/v1/users/me') json = {authenticated:false,user:null};
    else if (path === '/v1/models') json = {data:[{id:'model',type:'chat'}]};
    else if (path === '/v1/sessions') json = {sessions:chats};
    else if (path.startsWith('/v1/sessions/')) {
      const name = path.slice('/v1/sessions/'.length);
      if (r.request().method() === 'DELETE') {
        calls.push(name);
        if (name === failing) return r.fulfill({status:409,json:{error:'Conversation is busy'}});
        chats = chats.filter(item => item.name !== name); json = {success:true};
      } else json = chats.find(item => item.name === name);
    } else if (path === '/v1/agents/tasks') json = tasks;
    else if (path.startsWith('/v1/agents/tasks/')) {
      const id = path.slice('/v1/agents/tasks/'.length);
      if (r.request().method() === 'DELETE') {
        calls.push(id); tasks = tasks.filter(item => item.id !== id); json = {success:true};
      } else json = {run_id:id,status:tasks.find(item => item.id === id)?.status ?? 'completed', output:`Result for ${id}`,steps:[]};
    } else if (path === '/v1/agents/tools') json = {tools:[],enabled_count:0};
    else if (path === '/v1/agents/mcp') json = {servers:[]};
    else if (path === '/v1/integrations') json = {integrations:[]};
    await r.fulfill({json});
  });
  return { calls, fail:(name: string) => {failing=name;}, chats:()=>chats, tasks:()=>tasks };
}

test('chat deletion is visible, confirmed, searchable and survives reload', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/chat');
  const row = page.locator('.history-row', {hasText:'Delete 日本語'});
  await row.locator('.history-open').click();
  await page.locator('.composer textarea').fill('draft to remove with the chat');
  const button = row.getByRole('button',{name:'Delete conversation: Delete 日本語',exact:true});
  expect(await button.evaluate(e => getComputedStyle(e).color)).not.toBe('rgba(0, 0, 0, 0)');
  await button.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('button',{name:'Cancel',exact:true})).toBeFocused();
  await page.keyboard.press('Escape');
  expect(state.calls).toEqual([]);
  await button.click();
  await dialog.getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(row).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await page.reload();
  await expect(row).toHaveCount(0);
  await page.getByRole('searchbox',{name:'Search conversations'}).fill('Keep');
  await expect(page.locator('.history-row')).toHaveCount(1);
  expect(state.calls).toEqual(['Delete 日本語']);
});

test('bulk chat deletion only removes confirmed matches and retries failures, not successes', async ({page}) => {
  const state = await fixture(page);
  state.fail('Delete second');
  await page.goto('/ui/#/chat');
  await page.getByRole('searchbox',{name:'Search conversations'}).fill('Delete');
  await page.getByRole('button',{name:'Delete listed chats',exact:true}).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Delete 2 listed conversations');
  await dialog.getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('1 items could not be deleted');
  expect(state.chats().map(item=>item.name)).toEqual(['Keep this','Delete second']);
  state.fail('');
  await dialog.getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  expect(state.calls).toEqual(['Delete 日本語','Delete second','Delete second']);
  expect(state.chats().map(item=>item.name)).toEqual(['Keep this']);
});

test('agent history browses older tasks, reuses safely, deletes selected output and preserves active work', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/agents');
  const history = page.locator('.agent-history-panel');
  await expect(history.locator('.task-history article')).toHaveCount(20);
  await history.getByRole('button',{name:'Show more',exact:false}).click();
  await expect(history.locator('.task-history article')).toHaveCount(25);
  const row = history.locator('article',{hasText:'Task 24'});
  await row.getByRole('button',{name:'Task 24',exact:true}).click();
  await expect(page.locator('.result-card')).toContainText('Result for task-24');
  await row.getByRole('button',{name:'Reuse task',exact:true}).click();
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Task 24');
  await expect(row.getByRole('button',{name:'Reuse task',exact:true})).toBeDisabled();
  await row.getByRole('button',{name:'Delete task',exact:true}).click();
  await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(page.locator('.result-card')).not.toContainText('Result for task-24');
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Task 24');
  await expect(history.locator('article',{hasText:'Task 00'}).getByRole('button',{name:'Delete task',exact:true})).toBeDisabled();
  await history.getByRole('button',{name:'Clear removable tasks',exact:true}).click();
  await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(history.locator('.task-history article')).toHaveCount(2);
  expect(state.tasks().map(item=>item.id)).toEqual(['task-0','task-1']);
  expect(state.calls).not.toContain('task-0'); expect(state.calls).not.toContain('task-1');
  await page.reload();
  await expect(history.locator('.task-history article')).toHaveCount(2);
});

test('late history refresh cannot replace a newer conversation or draft', async ({page}) => {
 const state=await fixture(page);let requested=false,returned=false;let release!:()=>void;
 const delayed=new Promise<void>(resolve=>{release=resolve;});
 await page.goto('/ui/#/chat');
 await expect(page.locator('.composer textarea')).toBeEnabled();
 await page.getByRole('button',{name:'New chat',exact:true}).click();
 await page.locator('.composer textarea').fill('Keep my current draft');
 await page.locator('.history-row',{hasText:'Delete second'}).locator('.history-open').click();
 await expect(page.locator('.composer textarea')).toBeEnabled();
 // Trigger the second request explicitly. StrictMode mounts twice only in the
 // development server; CI serves the production build with one initial request.
 await page.route('**/v1/sessions',async r=>{
  requested=true;await delayed;
  await r.fulfill({json:{sessions:state.chats()}});returned=true;
 });
 await page.locator('#conversation-history').getByRole('button',{name:'Refresh',exact:true}).click();
 await expect.poll(()=>requested).toBe(true);
 await page.getByRole('button',{name:'New chat',exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep my current draft');
 release();await expect.poll(()=>returned).toBe(true);
 await expect(page.locator('.composer textarea')).toBeEnabled();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep my current draft');
 await expect(page.locator('.history-row.active')).toHaveCount(0);
});

test('mobile history confirmation fits and search does not erase the composer draft', async ({page}, testInfo) => {
  await page.setViewportSize({width:390,height:844});
  await fixture(page);
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep my unsent text');
  await page.getByRole('button',{name:'Show conversations',exact:true}).click();
  await page.getByRole('searchbox',{name:'Search conversations'}).fill('Delete');
  await page.getByRole('button',{name:'Delete listed chats',exact:true}).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeInViewport();
  expect(await dialog.evaluate(e => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: testInfo.outputPath('mobile-history-dialog.png') });
  await dialog.getByRole('button',{name:'Cancel',exact:true}).click();
  await page.locator('#conversation-history').getByRole('button',{name:'Close conversations',exact:true}).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my unsent text');
});

test('unrecoverable interrupted history can be deleted while active and uncertain tasks remain protected', async ({ page }) => {
  await fixture(page);
  let legacy = true;
  const deleted: string[] = [];
  await page.route('**/v1/agents/tasks', r => r.fulfill({ json: [
    ...(legacy ? [{ id: 'legacy', prompt: 'Legacy task without checkpoint', status: 'interrupted', deletable: true, created_at: new Date().toISOString() }] : []),
    { id: 'active', prompt: 'Active task', status: 'running', deletable: false, created_at: new Date().toISOString() },
    { id: 'uncertain', prompt: 'Outcome unknown', status: 'uncertain', deletable: false, created_at: new Date().toISOString() }
  ] }));
  await page.route('**/v1/agents/tasks/*', r => {
    if (r.request().method() !== 'DELETE') return r.fulfill({ json: {} });
    deleted.push(new URL(r.request().url()).pathname.split('/').at(-1)!);
    legacy = false; return r.fulfill({ json: { success: true } });
  });
  await page.goto('/ui/#/agents');
  const history = page.locator('.agent-history-panel');
  await expect(history.locator('article', { hasText: 'Legacy task without checkpoint' }).getByRole('button', { name: 'Delete task', exact: true })).toBeEnabled();
  await expect(history.locator('article', { hasText: 'Active task' }).getByRole('button', { name: 'Delete task', exact: true })).toBeDisabled();
  await expect(history.locator('article', { hasText: 'Outcome unknown' }).getByRole('button', { name: 'Delete task', exact: true })).toBeDisabled();
  await history.getByRole('button', { name: 'Clear removable tasks' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Remove 1 selected tasks');
  await dialog.getByRole('button', { name: 'Confirm delete' }).click();
  expect(deleted).toEqual(['legacy']);
  await page.reload();
  await expect(history.locator('article')).toHaveCount(2);
  await expect(history).not.toContainText('Legacy task without checkpoint');
});
