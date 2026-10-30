import { expect, test, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

// Opt-in evidence matrix. All API traffic is intercepted; no installed state,
// model inference, native input or external network is used.
const phase = process.env.OFFGRID_VISUAL_PHASE;
test.describe.configure({ mode: 'parallel' });
const model = 'Local-Research-7B-Instruct-Q4_K_M-long-model-name';
const answer = '## A quieter workspace\n\nKeep the work readable and the next action clear. This is a saved answer with **emphasis**, a [source](https://example.com), and multilingual text: café 日本語 مرحباً.\n\n### Next steps\n\n1. Review the evidence.\n2. Prepare a short briefing.\n\n```typescript\nconst summary = { ready: true, title: "OffGrid" };\nconsole.log(summary); // A deliberately long line to exercise horizontal code scrolling without widening the conversation.\n```\n\n| Item | Status |\n| --- | --- |\n| Notes | Ready |\n| Sources | Reviewed |';
async function fixture(page: Page, locale: string, theme: string) {
  const state = { mode: 'ready', writes: [] as string[] };
  await page.addInitScript(({locale, theme}) => {
    localStorage.setItem('offgrid.onboarding.complete','true');
    localStorage.setItem('offgrid.locale',locale); localStorage.setItem('offgrid.theme',theme);
    const fetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (!String(input).endsWith('/generate')) return fetch(input,init);
      return new Response(new ReadableStream({ start(controller) {
        const send = (value: unknown) => controller.enqueue(new TextEncoder().encode(`data: ${JSON.stringify(value)}\n\n`));
        send({type:'phase',phase:'processing'});
        send({type:'delta',delta:'A calm answer arrives as it is generated. café 日本語 مرحباً'});
        init?.signal?.addEventListener('abort',()=>controller.close(),{once:true});
      }}), {headers:{'Content-Type':'text/event-stream'}});
    };
  }, {locale,theme});
  const session = {name:'Demo briefing',model_id:model,updated_at:'2026-10-30T00:00:00Z',messages:[{role:'user',content:'Prepare a concise briefing with an example.'},{role:'assistant',content:answer}]};
  const profiles = [{id:'researcher',actor:'local',workspace:'craft',revision:3,digest:'fixture',settings:{name:'Researcher',model,instructions:'Read approved sources and explain uncertainty.',max_iterations:20}}];
  const job = {id:'run-'+'a'.repeat(32),run_id:'run-'+'a'.repeat(32),prompt:'Prepare a demo briefing',model,status:'completed',created_at:'2026-10-30T00:00:00Z',updated_at:'2026-10-30T00:00:00Z',steps:[],result:'A concise briefing is ready.'};
  await page.route('**/health',r=>r.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:v1|api\/v2)\//, async r => {
    const path=decodeURIComponent(new URL(r.request().url()).pathname);
    if(r.request().method()!=='GET') state.writes.push(path);
    if(path==='/v1/documents' && state.mode==='loading') return;
    if(path==='/v1/documents' && state.mode==='error') return r.fulfill({status:503,json:{error:{message:'The document library is temporarily unavailable.'}}});
    const bodies:Record<string,unknown>={
      '/v1/users/me':{authenticated:false,auth_required:false},
      '/api/v2/system':{product:'offgrid',version:'visual-fixture',workspace_id:'craft',api_version:2,capabilities:['task-first-agents-v2','agent-profiles-v1']},
      '/v1/system/config':{version:'visual-fixture',require_auth:false},
      '/v1/models':{data:[{id:model,type:'chat',size:4.4*1024**3}]},
      '/v1/catalog':{models:[{id:'demo-model',name:'Local Research',description:'A local model for everyday writing and research.',repo:'fixture/local',file:'model.gguf',quant:'Q4_K_M',parameters:'7B',size_bytes:4.4*1024**3,min_ram_gb:8}]},
      '/v1/models/download/progress':{},'/api/v2/models':{models:[]},'/api/v2/models/catalog':{models:[]},'/api/v2/models/operations':{operations:[]},
      '/v1/sessions':{sessions:[session]},'/v1/sessions/Demo briefing':session,
      '/v1/rag/status':{enabled:true,embedding_model:'local-embedding',stats:{}},
      '/v1/documents':{documents:state.mode==='empty'?[]:[{id:'notes',name:'Demo notes — café 日本語 مرحباً.txt',size:4096,index_status:'ready',source_retained:true,chunk_count:4}]},
      '/api/v2/jobs':[job], [`/api/v2/jobs/${job.id}`]:job,
      '/api/v2/agent-profiles':{profiles},'/api/v2/agent-profiles/resources':{tools:{}},
      '/api/v2/computer/status':{available:false},'/api/v2/computer/sessions':{sessions:[]},
      '/v1/agents/tasks':[],'/v1/runs':{runs:[]},'/v1/stats':{server:{}},'/v1/agents/mcp':{servers:[]},'/v1/agents/tools':{tools:[]},
      '/v1/audio/status':{profiles:[]},'/v1/integrations':{integrations:[]},
    };
    if(path==='/v1/sessions' && r.request().method()==='POST') return r.fulfill({json:{...r.request().postDataJSON(),messages:[]}});
    return r.fulfill({json:path.endsWith('/turn')?{turn:null}:bodies[path]??{}});
  });
  return state;
}

for(const theme of ['light','dark']) for(const locale of ['en','ar']) for(const width of [390,768,1024,1440]) {
  test(`visual craft ${theme} ${locale} ${width}`,async({page})=>{
    test.skip(!phase,'Set OFFGRID_VISUAL_PHASE=before or after for the evidence matrix.');
    test.setTimeout(120_000);
    await page.setViewportSize({width,height:900});
    const state=await fixture(page,locale,theme);
    const dir=resolve('../../build/ui-craft',phase!,`${theme}-${locale}-${width}`); mkdirSync(dir,{recursive:true});
    const capture=async(name:string)=>{
      await page.evaluate(()=>document.fonts.ready);
      await expect(page.locator('.page-failure')).toHaveCount(0);
      await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
      await page.screenshot({path:resolve(dir,`${name}.png`),fullPage:true,animations:'disabled'});
    };
    for(const route of ['agents','activity','knowledge','models','settings']) {
      await page.goto(`/ui/#/${route}`);
      await expect(page.locator('.page-content')).toBeVisible();
      if(route==='agents') await expect(page.locator('.profile-list-item')).toHaveCount(1);
      if(route==='activity') await expect(page.locator('.task-list-item')).toHaveCount(1);
      if(route==='knowledge') await expect(page.locator('.knowledge-documents article')).toHaveCount(1);
      if(route==='models') await expect(page.locator('.installed-model')).toHaveCount(1);
      await capture(route);
      if(route==='models') { await page.locator('#model-view-discover').click(); await expect(page.locator('.catalog-card')).toHaveCount(1); await capture('models-discover'); }
    }
    for(const mode of ['empty','error','loading']) {
      state.mode=mode; await page.goto('/ui/#/knowledge'); await page.reload();
      await expect(page.locator('.page-content')).toBeVisible();
      await expect(page.locator(mode==='empty'?'.workspace-empty':mode==='error'?'[role="alert"]':'.skeleton-card').first()).toBeVisible();
      await capture(mode);
    }
    state.mode='ready'; await page.goto('/ui/#/chat');
    if(width<1024) await page.locator('.navigation-trigger').click();
    await page.locator('.history-open').first().click();
    await expect(page.locator('.code-block')).toHaveCount(1); await capture('chat-code');
    await page.locator('.composer textarea').fill('Continue the briefing.'); await page.locator('.composer-actions > button').click();
    await expect(page.locator('.streaming-response')).toBeVisible(); await capture('chat-stream');
    // Reload ends only the synthetic stream, never an installed job.
    await page.goto('/ui/#/settings'); await page.reload();
    await page.locator('.command-trigger').click(); await expect(page.locator('dialog.command-palette')).toBeVisible(); await capture('palette');
    await page.keyboard.press('Escape');
    if(width<1024) await page.locator('.navigation-trigger').click();
    await page.locator('.workspace-options > button').click(); await expect(page.locator('.workspace-options-panel')).toBeVisible(); await capture('popover');
    await page.keyboard.press('Escape');
    if(width<1024) await page.keyboard.press('Escape');
    await page.locator('.settings-group .settings-panel button.secondary-button').first().click();
    await expect(page.locator('.onboarding')).toBeVisible(); await capture('onboarding');
    expect(state.writes.every(path=>path.startsWith('/v1/sessions'))).toBe(true);
  });
}
