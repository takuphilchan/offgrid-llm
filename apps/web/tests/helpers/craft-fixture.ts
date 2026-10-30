import type { Page } from '@playwright/test';
const model = 'Local-Research-7B-Instruct-Q4_K_M-long-model-name';
const answer = '## A quieter workspace\n\nKeep the work readable and the next action clear. This is a saved answer with **emphasis**, a [source](https://example.com), and multilingual text: café 日本語 مرحباً.\n\n### Next steps\n\n1. Review the evidence.\n2. Prepare a short briefing.\n\n```typescript\nconst summary = { ready: true, title: "OffGrid" };\nconsole.log(summary); // A deliberately long line to exercise horizontal code scrolling without widening the conversation.\n```\n\n| Item | Status |\n| --- | --- |\n| Notes | Ready |\n| Sources | Reviewed |';
export async function craftFixture(page: Page, locale: string, theme: string) {
  const state = { mode: 'ready', writes: [] as string[], download: false, percent: 0 };
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
        (window as unknown as { emitCraftDelta: (delta: string) => void }).emitCraftDelta = delta => send({type:'delta',delta});
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
      '/v1/models/download/progress':state.download ? {'demo-model.gguf':{model_id:'demo-model',file_name:'demo-model.gguf',status:'downloading',percent:state.percent=(state.percent+10)%90,bytes_done:1024,bytes_total:4096,speed:1024,started_at:1}} : {},'/api/v2/models':{models:[]},'/api/v2/models/catalog':{models:[]},'/api/v2/models/operations':{operations:[]},
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

