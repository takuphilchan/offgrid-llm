import { expect, test, type Page } from '@playwright/test';
import type { InferenceEndpointStatus } from '../src/api/client';
import { languageRegistry } from '../src/i18n/language';

async function fixture(page:Page,role='admin',locale='en') {
 const item:InferenceEndpointStatus={id:'lab',revision:'r1',model_id:'endpoint:lab@r1',provider:'ollama',url:'https://inference.example.test',model:'fixture',actors:['fixture-user'],config_pin:'a'.repeat(64),installed:true,current:true,enabled:false,review_required:true,credential_ready:true,managed:false,probe_allowed:false};
 const state={item,revision:'b'.repeat(64),requests:[] as string[],stale:false};
 await page.addInitScript(language=>{localStorage.setItem('offgrid.onboarding.complete','true');localStorage.setItem('offgrid.locale',language);},locale);
 await page.route('**/health',route=>route.fulfill({json:{status:'healthy'}}));
 await page.route(/\/(?:v1|api\/v2)\//,route=>{
  const req=route.request(),url=new URL(req.url()),path=url.pathname;state.requests.push(`${req.method()} ${path}`);
  if(path.endsWith('/inference-endpoints/review')){
   const body=req.postDataJSON();
   expect(Object.keys(body).sort()).toEqual(['config_pin','enabled','id','policy_revision','revision']);
   expect(body.config_pin).toBe(item.config_pin);
   if(state.stale)return route.fulfill({status:409,json:{error:{message:'Endpoint policy changed; refresh before reviewing'}}});
   expect(body.policy_revision).toBe(state.revision);state.revision='c'.repeat(64);item.enabled=body.enabled;item.review_required=false;item.probe_allowed=item.enabled;
  }
  if(path.endsWith('/inference-endpoints/probe'))return route.fulfill({json:{model_id:item.model_id,connection_verified:true,qualified:false}});
  if(path.startsWith('/api/v2/inference-endpoints'))return route.fulfill({json:{configured:true,policy_state:'ready',policy_revision:state.revision,items:[item]}});
  const bodies:Record<string,unknown>={
   '/api/v2/system':{product:'offgrid',api_version:2,workspace_id:'endpoint-library-fixture',capabilities:[]},
   '/v1/system/config':{require_auth:true},'/v1/users/me':{authenticated:true,auth_required:true,user:{id:'fixture-user',username:'fixture-user',role}},
   '/v1/models':{data:[{id:'fixture-chat',type:'llm'},...(role!=='admin'||item.enabled?[{id:item.model_id,type:'llm',engine:'ollama',owned_by:'enrolled-endpoint'}]:[])]},
   '/v1/catalog':{models:[]},'/v1/models/download/progress':{},'/api/v2/jobs':[], '/v1/sessions':{sessions:[]},
   '/api/v2/models':{models:[]},'/api/v2/models/operations':{operations:[]},'/api/v2/models/packages':{packages:[]}
  };
  return route.fulfill({json:bodies[path]??{}});
 });
 return state;
}

test('administrator reviews exact endpoint, probes without qualification and disables without legacy mutations',async({page},testInfo)=>{
 const state=await fixture(page);await page.goto('/ui/#/models');
 const card=page.locator('.endpoint-model');await expect(card).toContainText('Review required');
 await card.getByRole('button',{name:'Review and enable',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText(state.item.url);
 await page.getByRole('dialog').getByRole('button',{name:'Review and enable',exact:true}).click();
 await expect(card).toContainText('Access enabled');
 await card.getByRole('button',{name:'Check connection',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('without sending a prompt');
 await page.getByRole('dialog').getByRole('button',{name:'Check connection',exact:true}).click();
 await expect(page.getByText('Pinned connection metadata verified. Model quality remains unqualified.')).toBeVisible();
 await page.screenshot({path:testInfo.outputPath('endpoint-management-en.png'),fullPage:true});
 await expect(page.getByRole('button',{name:'Model actions: endpoint:lab@r1',exact:true})).toHaveCount(0);
 await card.getByRole('button',{name:'Disable access',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('does not undo');
 await page.getByRole('dialog').getByRole('button',{name:'Disable access',exact:true}).click();
 await expect(card).toContainText('Access disabled');
 expect(state.requests.filter(value=>value.startsWith('POST'))).toEqual(['POST /api/v2/inference-endpoints/review','POST /api/v2/inference-endpoints/probe','POST /api/v2/inference-endpoints/review']);
});

test('stale review stays visible for refresh without automatic retry',async({page})=>{
 const state=await fixture(page);await page.goto('/ui/#/models');
 await page.locator('.endpoint-model').getByRole('button',{name:'Review and enable',exact:true}).click();state.stale=true;
 await page.getByRole('dialog').getByRole('button',{name:'Review and enable',exact:true}).click();
 await expect(page.getByRole('dialog').getByRole('alert')).toContainText('changed');
 expect(state.requests.filter(value=>value==='POST /api/v2/inference-endpoints/review')).toHaveLength(1);
 expect(state.item.enabled).toBe(false);
});

test('ordinary user sees endpoint selection without admin configuration or file maintenance',async({page})=>{
 const state=await fixture(page,'user');await page.goto('/ui/#/models');
 const card=page.locator('.endpoint-model');await expect(card).toContainText('endpoint:lab@r1');
 await expect(card.getByRole('button',{name:'Select model',exact:true})).toBeVisible();
 await expect(card.getByRole('button',{name:'Review and enable',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Model actions: endpoint:lab@r1',exact:true})).toHaveCount(0);
 expect(state.requests.some(value=>value.includes('/inference-endpoints'))).toBe(false);
});

test('Arabic endpoint review has translated controls and fits the viewport',async({page},testInfo)=>{
 await fixture(page,'admin','ar');await page.goto('/ui/#/models');
 await expect(page.locator('.endpoint-model').getByRole('button',{name:'مراجعة وتفعيل',exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await page.screenshot({path:testInfo.outputPath('endpoint-management-ar.png'),fullPage:true});
});

test('Chinese preview endpoint copy stays in the selected language',async({page},testInfo)=>{
 await fixture(page,'admin','zh-Hans');await page.goto('/ui/#/models');
 await expect(page.locator('.endpoint-model').getByRole('button',{name:'审核并启用',exact:true})).toBeVisible();
 await expect(page.locator('.inference-endpoint-section')).not.toContainText('Review and enable');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await page.screenshot({path:testInfo.outputPath('endpoint-management-zh.png'),fullPage:true});
});

for(const locale of Object.keys(languageRegistry)) test(`older service explains missing endpoint management without false empty or failure: ${locale}`,async({page})=>{
 const state=await fixture(page,'admin',locale);
 await page.route('**/api/v2/inference-endpoints',route=>route.fulfill({status:404,body:'not found'}));
 await page.goto('/ui/#/models');
 const section=page.locator('.inference-endpoint-section');
 const status=section.getByRole('status');await expect(status).toBeVisible();
 if(locale==='en')await expect(status).toHaveText('This service does not support inference endpoint management. Installed local models remain available.');
 else {expect((await status.innerText()).length).toBeGreaterThan(20);await expect(status).not.toContainText('This service does not support');}
 await expect(section.getByRole('alert')).toHaveCount(0);
 await expect(section.getByText('No endpoints are provisioned.',{exact:true})).toHaveCount(0);
 await expect(section.getByRole('button')).toHaveCount(0);
 await expect(page.locator('.installed-model button[aria-pressed]')).toBeEnabled();
 expect(state.requests.filter(value=>value.startsWith('POST'))).toEqual([]);
});

test('endpoint read outages remain retryable and are not reported as unsupported',async({page})=>{
 const state=await fixture(page);
 await page.route('**/api/v2/inference-endpoints',route=>route.fulfill({status:503,json:{error:'Endpoint status unavailable'}}));
 await page.goto('/ui/#/models');
 const section=page.locator('.inference-endpoint-section');
 await expect(section.getByRole('alert')).toContainText('Endpoint status unavailable');
 await expect(section.getByText('This service does not support inference endpoint management.',{exact:false})).toHaveCount(0);
 await page.unroute('**/api/v2/inference-endpoints');
 await section.getByRole('button',{name:'Refresh',exact:true}).click();
 await expect(section.getByRole('button',{name:'Review and enable',exact:true})).toBeVisible();
 expect(state.requests.filter(value=>value.startsWith('POST'))).toEqual([]);
});

test('unsupported management preserves endpoint models advertised by the service',async({page})=>{
 const state=await fixture(page);state.item.enabled=true;
 await page.route('**/api/v2/inference-endpoints',route=>route.fulfill({status:404,body:'not found'}));
 await page.goto('/ui/#/models');
 const section=page.locator('.inference-endpoint-section');
 await expect(section.getByRole('status')).toContainText('does not support');
 const card=section.locator('.endpoint-model');await expect(card).toContainText(state.item.model_id);
 await expect(card.getByRole('button',{name:'Select model',exact:true})).toBeEnabled();
 await expect(card.getByRole('button',{name:/enable|connection|disable/i})).toHaveCount(0);
 expect(state.requests.filter(value=>value.startsWith('POST'))).toEqual([]);
});
