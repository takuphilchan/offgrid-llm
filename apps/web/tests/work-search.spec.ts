import { expect, test, type Page } from '@playwright/test';

const run = 'run-' + 'd'.repeat(32);
async function fixture(page: Page, locale = 'en') {
  const state = { workspace: 'search-one', actor: 'alice', available: true, partial: false, fail: false, queries: [] as string[], writes: [] as string[] };
  await page.addInitScript(locale => { localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.onboarding.complete', 'true'); }, locale);
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(api\/v2|v1)\//, async r => {
    const url = new URL(r.request().url()), path = url.pathname;
    if (r.request().method() !== 'GET') state.writes.push(path);
    const sessions = Array.from({length: 500}, (_, i) => ({ name: i === 499 ? 'Archived demo café' : `Conversation ${i}`, model_id: 'model', messages: [{role:'assistant',content:`Saved answer ${i}`}],created_at:'2026-10-01T00:00:00Z',updated_at:'2026-10-01T00:00:00Z' }));
    if (path === '/api/v2/work/search') {
      const query = url.searchParams.get('query')!; state.queries.push(query);
      if (state.fail) return r.fulfill({ status:503,json:{error:'unavailable'} });
      const title = query === 'slow' ? 'Stale private result' : 'Archived demo café';
      if (query === 'slow') await new Promise(resolve => setTimeout(resolve, 1000));
      return r.fulfill({json:{ scopes:['conversation_names','task_requests'], partial:state.partial, results:[{kind:'chat',id:title,title,updated_at:'2026-10-01T00:00:00Z'},{kind:'task',id:run,title:'Demo report still running',updated_at:'2026-10-01T00:00:00Z'}]}});
    }
    const data: Record<string, unknown> = {
      '/api/v2/system': { product:'offgrid',api_version:2,workspace_id:state.workspace,capabilities:['task-first-agents-v2',...(state.available?['work-search-v1']:[])] },
      '/v1/users/me': {authenticated:true,auth_required:true,user:{id:state.actor,username:state.actor,role:'admin'}},
      '/v1/models': {data:[{id:'model',type:'chat'}]}, '/v1/catalog':{models:[]}, '/v1/rag/status':{enabled:false},
      '/v1/sessions': {sessions}, '/api/v2/jobs':[{id:run,prompt:'Demo report still running',status:'running',created_at:'2026-10-01T00:00:00Z'}],
      [`/api/v2/jobs/${run}`]:{run_id:run,id:run,prompt:'Demo report still running',status:'running',steps:[],created_at:'2026-10-01T00:00:00Z'},
    };
    if(path.endsWith('/turn')) return r.fulfill({json:{turn:null}});
    const session = sessions.find(item => path === `/v1/sessions/${encodeURIComponent(item.name)}`);
    return r.fulfill({json:session ?? data[path] ?? {}});
  });
  return state;
}
async function search(page: Page, query: string) {
  await page.getByRole('button', {name:/Quick actions/}).click();
  await page.getByRole('dialog').getByRole('combobox').fill(query);
}

test('find an old conversation among 500 and return to the current draft without execution', async ({page}) => {
  const state=await fixture(page); await page.goto('/ui/#/chat');
  await expect(page.locator('.message')).toContainText(['Saved answer 0']);
  await page.locator('.composer textarea').fill('Preserve my 日本語 draft');
  await search(page,'demo');
  await expect(page.getByRole('dialog')).toContainText('not messages or file contents');
  await page.getByRole('option',{name:'Archived demo café',exact:true}).click();
  await expect(page.locator('.message')).toContainText(['Saved answer 499']);
  await page.getByRole('button',{name:'Back to previous view'}).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Preserve my 日本語 draft');
  await expect(page.locator('.message')).toContainText(['Saved answer 0']);
  expect(state.writes).toEqual([]);
});

test('opening an active task is navigation only and retains a return path', async ({page}) => {
  const state=await fixture(page); await page.goto('/ui/#/models');await search(page,'demo');
  await page.getByRole('option',{name:'Demo report still running'}).click();
  await expect(page).toHaveURL(new RegExp(`/agents/task/${run}$`));
  await page.getByRole('button',{name:'Back to previous view'}).click();
  await expect(page).toHaveURL(/#\/models$/);expect(state.writes).toEqual([]);
});

test('bounded search debounces, rejects stale responses, and discloses partial results and failures', async ({page}) => {
  const state=await fixture(page);await page.goto('/ui/#/chat');await search(page,'d');
  await expect(page.getByRole('dialog')).toContainText('at least 2 characters');expect(state.queries).toEqual([]);
  await page.getByRole('combobox').fill('slow');await expect.poll(()=>state.queries).toContain('slow');
  state.partial=true;await page.getByRole('combobox').fill('demo');
  await expect(page.getByRole('option',{name:'Archived demo café'})).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText('Some work could not be searched');
  await page.waitForTimeout(1100);await expect(page.getByText('Stale private result')).toHaveCount(0);
  state.fail=true;await page.getByRole('combobox').fill('failure');
  await expect(page.getByRole('dialog')).toContainText('Saved work could not be searched');
  await expect(page.getByRole('option',{name:'Archived demo café'})).toHaveCount(0);
  await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:/Quick actions/})).toBeFocused();
});

test('older services keep command navigation and do not pretend to search saved content', async ({page})=>{
 const state=await fixture(page);state.available=false;await page.goto('/ui/#/chat');await search(page,'Models');
 await expect(page.getByRole('dialog')).toContainText('action search only');
 await page.getByRole('option',{name:'Models',exact:true}).click();await expect(page).toHaveURL(/#\/models$/);expect(state.queries).toEqual([]);
});

test('workspace and actor changes dispose search results and return state', async ({page})=>{
 const state=await fixture(page);await page.goto('/ui/#/models');await search(page,'demo');
 await page.getByRole('option',{name:'Archived demo café'}).click();
 await expect(page.getByRole('button',{name:'Back to previous view'})).toBeVisible();
 state.workspace='search-two';state.actor='bob';
 await page.getByRole('button',{name:'Workspace options',exact:true}).click();
 await page.getByRole('button',{name:'Refresh',exact:true}).click();
 await expect(page.getByRole('button',{name:'Back to previous view'})).toHaveCount(0);
 await expect(page.locator('dialog.onboarding')).toBeVisible();
 await page.locator('dialog.onboarding .onboarding-close').click();
 await expect(page.locator('dialog.onboarding')).toHaveCount(0);
 await search(page,'slow');await expect.poll(()=>state.queries).toContain('slow');await page.keyboard.press('Escape');
 await page.getByRole('button',{name:'Workspace options',exact:true}).click();
 await page.getByRole('button',{name:'Sign out',exact:true}).click();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 await page.waitForTimeout(1100);await expect(page.getByText('Stale private result')).toHaveCount(0);
 expect(state.writes).toEqual(['/v1/auth/logout']);
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu']) test(`search results fit narrow ${locale} layout and respect keyboard composition`,async({page}, testInfo)=>{
 await fixture(page,locale);await page.setViewportSize({width:320,height:640});await page.goto('/ui/#/models');
 await page.keyboard.press('Control+k');await page.getByRole('combobox').fill('demo');
 await expect(page.getByRole('option',{name:'Archived demo café'})).toBeVisible();
 await page.getByRole('combobox').dispatchEvent('keydown',{key:'Enter',isComposing:true});await expect(page.getByRole('dialog')).toBeVisible();
 const overflow=await page.getByRole('dialog').evaluate(node=>node.scrollWidth>node.clientWidth+1);expect(overflow).toBe(false);
 if(locale==='en'||locale==='ar') await page.screenshot({path:testInfo.outputPath(`search-${locale}.png`)});
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);
});
