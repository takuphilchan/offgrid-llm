import { expect, test, type Page, type Route } from '@playwright/test';

const embedding = {id:'bge-m3',name:'BGE M3',type:'embedding',description:'Fixture',parameters:'567M',size_bytes:1000,min_ram_gb:2,repo:'fixture/bge',file:'bge.gguf',quant:'Q4_K_M'};
async function workspace(page:Page, override:(path:string,route:Route)=>Promise<unknown>|unknown=()=>undefined, onboarding=false){
 await page.addInitScript(({onboarding})=>{localStorage.setItem('offgrid.locale','en');if(!onboarding)localStorage.setItem('offgrid.onboarding.complete','true');},{onboarding});
 await page.route('**/health',r=>r.fulfill({json:{status:'healthy'}}));
 await page.route('**/v1/**',async r=>{const path=new URL(r.request().url()).pathname;const custom=await override(path,r);if(custom!==undefined)return r.fulfill({json:custom});return r.fulfill({json: path==='/v1/users/me'?{authenticated:false,user:null}:path==='/v1/models'?{data:[{id:'chat-model',type:'chat'}]}:path==='/v1/catalog'?{models:[embedding]}:path==='/v1/documents'?{documents:[],count:0}:path==='/v1/rag/status'?{enabled:false,stats:{}}:path==='/v1/sessions'?{sessions:[]}:path.endsWith('/turn')?{turn:null}:{} });});
}

test('onboarding contains keyboard focus and Escape restores the workspace',async({page})=>{
 await workspace(page,undefined,true);await page.goto('/ui/');const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();
 for(let i=0;i<8;i++){await page.keyboard.press('Tab');expect(await page.evaluate(()=>!!document.activeElement?.closest('dialog'))).toBe(true);}
 await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await page.locator('.primary-nav a[href="#/models"]').click();await expect(page.locator('.models-workspace')).toBeVisible();
});

test('Activity ignores a late response for the previous selected task',async({page})=>{
 await workspace(page,async path=>{
  if(path==='/v1/runs')return{runs:['A','B'].map(id=>({id,status:'completed',updated_at:new Date().toISOString(),event_count:1,data:{prompt:'Task '+id}}))};
  if(path.endsWith('/events')){const id=path.includes('/A/')?'A':'B';await new Promise(r=>setTimeout(r,id==='A'?700:50));return{events:[{id,type:id+'-result',sequence:1,time:new Date().toISOString()}]};}
 });
 await page.goto('/ui/#/activity');await page.getByRole('button',{name:/Task A/}).click();await page.getByRole('button',{name:/Task B/}).click();
 await expect(page.locator('.event-list')).toContainText('B-result');await page.waitForTimeout(850);
 await expect(page.locator('.run-row.selected')).toContainText('Task B');await expect(page.locator('.event-list')).not.toContainText('A-result');
});

test('a failed external download remains discoverable and resumes its exact source',async({page})=>{
 let status='downloading',submissions=0;
 await workspace(page,(path,route)=>{
  if(path==='/v1/models/download/progress')return{'external.gguf':{file_name:'external.gguf',model_id:'external',repository:'fixture/repo',source_file:'nested/model.gguf',quantization:'Q4_K_M',status,bytes_done:500,bytes_total:1000,percent:50,speed:0,started_at:1,error:status==='failed'?'Network disconnected':''}};
  if(path==='/v1/models/download'){expect(route.request().postDataJSON()).toMatchObject({model_id:'external',repository:'fixture/repo',file_name:'nested/model.gguf'});submissions++;status='downloading';return{success:true,file_name:'external.gguf'};}
 });
 await page.goto('/ui/#/models');const card=page.locator('.catalog-card').filter({has:page.getByText('external.gguf',{exact:true})});await expect(card).toContainText('50.0%');status='failed';
 await expect(card.getByRole('button',{name:'Resume',exact:true})).toBeVisible();await page.reload();await card.getByRole('button',{name:'Resume',exact:true}).click();await expect.poll(()=>submissions).toBe(1);
});

test('knowledge activation survives leaving its setup page',async({page})=>{
 let submitted=false,complete=false;
 await workspace(page,(path,route)=>{
  if(path==='/v1/models')return{data:[{id:'chat-model',type:'chat'},...(complete?[{id:'bge-m3',type:'embedding'}]:[])]};
  if(path==='/v1/rag/status')return{enabled:complete,embedding_model:complete?'bge-m3':'',stats:{}};
  if(path==='/v1/models/download'){expect(route.request().postDataJSON().enable_knowledge).toBe(true);submitted=true;return{success:true,file_name:'bge-m3.gguf'};}
  if(path==='/v1/models/download/progress')return submitted?{'bge-m3.gguf':{file_name:'bge-m3.gguf',enable_knowledge:true,status:complete?'complete':'downloading',bytes_done:100,bytes_total:1000,percent:10,speed:0,started_at:1}}:{};
 });
 await page.goto('/ui/#/knowledge');await page.getByRole('button',{name:'Install and enable'}).click();await expect(page.locator('.knowledge-setup')).toContainText('10.0%');
 await page.locator('.primary-nav a[href="#/models"]').click();await page.locator('.primary-nav a[href="#/knowledge"]').click();await expect(page.locator('.knowledge-setup-controls .primary-button')).toBeDisabled();
 complete=true;await expect(page.getByText(/Active embedding model: bge-m3/)).toBeVisible();
});

test('disabled knowledge keeps source inspection and confirmed deletion available',async({page})=>{
 const doc={id:'doc',name:'Guide 世界',size:30,source_retained:true,chunk_count:2};let removed=false,deletions=0;
 await workspace(page,(path)=>{
  if(path==='/v1/documents')return{documents:removed?[]:[doc],count:removed?0:1};
  if(path==='/v1/documents/source')return{document:doc,content:'Retained source 世界 <script>not executed</script>',truncated:false};
  if(path==='/v1/documents/delete'){removed=true;deletions++;return{success:true};}
 });
 await page.goto('/ui/#/knowledge');await page.getByRole('button',{name:'View source'}).click();await expect(page.getByRole('dialog')).toContainText('Retained source 世界');await page.getByRole('dialog').getByRole('button').click();
 await page.getByRole('button',{name:'Delete',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'Cancel'}).click();expect(deletions).toBe(0);
 await page.getByRole('button',{name:'Delete',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'Confirm delete'}).click();await expect.poll(()=>deletions).toBe(1);await expect(page.getByRole('heading',{name:doc.name})).toHaveCount(0);
});

test('model deletion has a safe Cancel and one explicit confirmation',async({page})=>{
 let deletions=0;await workspace(page,path=>{if(path==='/v1/models/delete'){deletions++;return{success:true};}});
 await page.goto('/ui/#/models');await page.locator('.model-actions').getByRole('button',{name:'Delete',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('Saved chats remain');await page.keyboard.press('Escape');expect(deletions).toBe(0);
 await page.locator('.model-actions').getByRole('button',{name:'Delete',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'Confirm delete'}).click();await expect.poll(()=>deletions).toBe(1);
});

test('a stalled startup request has a bounded recovery path',async({page})=>{
 await workspace(page);await page.clock.install();
 await page.addInitScript(()=>{const original=window.fetch.bind(window);window.fetch=(input,init)=>String(input)==='/v1/users/me'?new Promise((_resolve,reject)=>init?.signal?.addEventListener('abort',()=>reject(new DOMException('aborted','AbortError')),{once:true})):original(input,init);});
 await page.goto('/ui/');await expect(page.locator('.boot-screen')).toBeVisible();await page.clock.fastForward(31_000);await expect(page.locator('.boot-screen')).toContainText('too long');await expect(page.getByRole('button',{name:'Try again'})).toBeVisible();
});

test('an active chat reconnects after navigation without submitting new work',async({page})=>{
 const turn={id:'turn-one',prompt:'hello',output:'Retained partial',status:'running',phase:'generating',model:'chat-model',profile:'interactive',knowledge:false,max_tokens:1024};
 const session={name:'recoverable-chat',model_id:'chat-model',messages:[],updated_at:new Date().toISOString(),created_at:new Date().toISOString(),turn};let cancellations=0;
 await workspace(page,(path,route)=>{
  if(path==='/v1/sessions')return{sessions:[session]};
  if(path.endsWith('/turn'))return{turn};
  if(path.endsWith('/turn/cancel')){expect(route.request().postDataJSON()).toEqual({id:turn.id});cancellations++;return{success:true};}
 });
 await page.addInitScript(()=>{const original=window.fetch.bind(window);(window as any).attachments=0;window.fetch=async(input,init)=>{if(!String(input).includes('/turn/events'))return original(input,init);(window as any).attachments++;return new Response(new ReadableStream({start(c){c.enqueue(new TextEncoder().encode('data: {"type":"delta","delta":"Retained partial"}\n\n'));init?.signal?.addEventListener('abort',()=>c.error(new DOMException('aborted','AbortError')),{once:true});}}),{headers:{'content-type':'text/event-stream'}});};});
 await page.goto('/ui/#/chat');await expect(page.locator('.streaming-response')).toContainText('Retained partial');await page.locator('.primary-nav a[href="#/models"]').click();expect(cancellations).toBe(0);
 await page.locator('.primary-nav a[href="#/chat"]').click();await expect(page.locator('.streaming-response')).toContainText('Retained partial');expect(await page.evaluate(()=>(window as any).attachments)).toBe(2);await page.locator('.composer button').click();await expect.poll(()=>cancellations).toBe(1);
});
