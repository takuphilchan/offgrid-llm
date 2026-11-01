import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

test('native tools do not depend on optional external integration or connection reads',async({page})=>{
 const state=await craftFixture(page,'en','light'), unrelated:string[]=[];
 await page.route('**/v1/agents/tools?all=true',r=>r.fulfill({json:{tools:[{name:'workspace_read',description:'Read an approved file',enabled:true,source:'native'}],enabled_count:1}}));
 await page.route(/\/(?:v1\/integrations|v1\/agents\/mcp)(?:\?|$)/,r=>{unrelated.push(r.request().url());return r.fulfill({status:503,json:{error:'Optional provider unavailable'}});});
 await page.goto('/ui/#/agents/tools');
 await expect(page.getByRole('heading',{name:'workspace_read',exact:true})).toBeVisible();
 await expect(page.getByRole('alert')).toHaveCount(0);expect(unrelated).toEqual([]);expect(state.writes).toEqual([]);
});

test('external provider failure has scoped retry and does not hide connected MCP servers',async({page})=>{
 const state=await craftFixture(page,'en','light');let failed=true,reads=0;
 await page.route('**/v1/agents/mcp',r=>r.fulfill({json:{servers:[{name:'Research source',transport:'http',tools:3,status:'connected'}]}}));
 await page.route('**/v1/integrations*',r=>{reads++;return r.fulfill(failed?{status:503,json:{error:'Provider readiness unavailable'}}:{json:{integrations:[]}});});
 await page.goto('/ui/#/agents/connections');
 await expect(page.getByRole('heading',{name:'Research source',exact:true})).toBeVisible();
 const providers=page.locator('.external-providers');
 await expect(providers.getByRole('alert')).toContainText('Provider readiness unavailable');
 await expect(providers).not.toContainText('No external-agent providers are available.');
 failed=false;await providers.getByRole('button',{name:'Try again',exact:true}).click();
 await expect(providers.getByRole('alert')).toHaveCount(0);expect(reads).toBeGreaterThan(1);
 await expect(page.getByRole('heading',{name:'Research source',exact:true})).toBeVisible();expect(state.writes).toEqual([]);
});

test('connection test result cannot silently describe an edited endpoint',async({page})=>{
 const state=await craftFixture(page,'en','light');let release:(()=>void)|undefined;const tests:unknown[]=[];
 await page.route('**/v1/agents/mcp/test',async r=>{tests.push(r.request().postDataJSON());await new Promise<void>(resolve=>release=resolve);await r.fulfill({json:{tools_count:3,status:'ok'}});});
 await page.goto('/ui/#/agents/connections');
 const url=page.getByLabel('MCP server URL', {exact:true}),name=page.getByLabel('Connection name',{exact:true});
 await name.fill('Test source');await url.fill('https://first.example.test/mcp');
 await page.getByRole('button',{name:'Test connection',exact:true}).click();
 await expect(url).toBeDisabled();await expect(name).toBeDisabled();await expect.poll(()=>tests.length).toBe(1);
 release!();await expect(page.locator('.connection-success')).toContainText('3');
 await url.fill('https://second.example.test/mcp');await expect(page.locator('.connection-success')).toHaveCount(0);
 expect(tests).toEqual([{url:'https://first.example.test/mcp'}]);expect(state.writes).toEqual([]);
});

test('restricted accounts receive a permission explanation without connection administration requests',async({page})=>{
 await craftFixture(page,'en','light');const requests:string[]=[];
 await page.route('**/v1/users/me',r=>r.fulfill({json:{authenticated:true,auth_required:true,user:{id:'viewer',username:'viewer',role:'viewer'}}}));
 await page.route(/\/v1\/(?:integrations|agents\/mcp|agents\/tools)/,r=>{requests.push(r.request().url());return r.fulfill({status:403,json:{error:'Forbidden'}});});
 await page.goto('/ui/#/agents/connections');
 await expect(page.locator('.permission-notice')).toBeVisible();await expect(page.getByRole('form',{name:'Connect',exact:true})).toHaveCount(0);expect(requests).toEqual([]);
});
