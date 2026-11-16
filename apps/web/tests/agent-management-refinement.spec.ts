import {expect, test, type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

const tools = [
  {name:'browser_observe',description:'Inspect the active browser page.',source:'builtin'},
  {name:'computer_observe',description:'Inspect a permitted application.',source:'builtin'},
  {name:'calculator',description:'Calculate an expression.',source:'builtin'},
  {name:'workspace_process_start',description:'Run a command inside the permitted folder.',source:'builtin'},
  {name:'document_search',description:'Find text in an approved document.',source:'builtin'},
  {name:'mcp__records__search',description:'Search departmental records through this connection.',source:'mcp:records'},
].map(tool => ({...tool, enabled:true}));

async function fixture(page:Page, locale='en', theme='light') {
  const state = await craftFixture(page, locale, theme);
  await page.route('**/v1/agents/tools*', r => r.fulfill({json:{tools,enabled_count:tools.length}}));
  await page.route('**/v1/agents/mcp', r => r.fulfill({json:{servers:[{name:'Records',url:'https://records.example.test/mcp',transport:'http',status:'connected',tools:4}]}}));
  return state;
}

for (const [locale,theme,width] of [['en','light',1440],['en','dark',1440],['ar','dark',390]] as const) {
  test(`tool and connection hierarchy ${locale} ${theme} ${width}`, async({page},info) => {
    const state=await fixture(page,locale,theme);
    await page.setViewportSize({width,height:960});
    await page.goto('/ui/#/agents/tools');
    await expect(page.locator('.tool-list article')).toHaveCount(6);
    const icons=await page.locator('.tool-glyph svg').evaluateAll(nodes=>nodes.map(n=>n.innerHTML));
    expect(new Set(icons).size).toBe(6);
    await expect(page.locator('.connector-list')).toContainText('Records');
    await expect(page.locator('.tool-list article').last()).toHaveCSS('border-bottom-width','1px');
    const connection=page.locator('.connector-list article');
    const icon=await connection.locator('.connector-glyph').boundingBox();
    const remove=await connection.getByRole('button').boundingBox();
    expect(Math.abs(icon!.y-remove!.y)).toBeLessThan(16);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:info.outputPath('tools.png'),fullPage:true});
    const search=page.locator('#agent-tools-panel input[type=search]');
    await search.fill('mcp:records');
    await expect(page.locator('.tool-list article')).toHaveCount(1);
    await expect(page.locator('.tool-list')).toContainText('mcp__records__search');
    await search.fill('no-matching-tool');
    await expect(page.locator('.tool-list article')).toHaveCount(0);
    expect(state.writes).toEqual([]);
  });
}

test('tool changes serialize and preserve the prior value after a rejected mutation',async({page})=>{
  await fixture(page);
  let finish: (()=>void)|undefined, writes=0;
  await page.route('**/v1/agents/tools*',async r=>{
    if(r.request().method()==='PATCH'){
      writes++;
      await new Promise<void>(resolve=>{finish=resolve;});
      return r.fulfill({status:503,json:{error:'Tool change was not saved.'}});
    }
    return r.fulfill({json:{tools,enabled_count:tools.length}});
  });
  await page.goto('/ui/#/agents/tools');
  await page.getByRole('checkbox',{name:'calculator',exact:true}).click();
  await expect.poll(()=>writes).toBe(1);
  for(const box of await page.locator('.tool-list input').all())await expect(box).toBeDisabled();
  finish!();
  await expect(page.getByRole('alert')).toContainText('Tool change was not saved');
  await expect(page.getByRole('checkbox',{name:'calculator',exact:true})).toBeChecked();
  await expect(page.getByRole('checkbox',{name:'calculator',exact:true})).toBeEnabled();
  expect(writes).toBe(1);
});

test('connection failure keeps its draft and explicit retry closes setup only after success',async({page})=>{
  await fixture(page);
  let writes=0, connected=false;
  await page.route('**/v1/agents/mcp',r=>{
    if(r.request().method()==='POST'){
      writes++;
      expect(r.request().postDataJSON()).toEqual({name:'My records',url:'https://records.example.test/mcp'});
      if(writes===1)return r.fulfill({status:503,json:{error:'Connection was not saved.'}});
      connected=true;return r.fulfill({json:{status:'connected',server:'My records',tools_added:2}});
    }
    return r.fulfill({json:{servers:connected?[{name:'My records',transport:'http',tools:2,status:'connected'}]:[]}});
  });
  await page.goto('/ui/#/agents/tools');
  const trigger=page.locator('.connector-panel .workspace-section-heading button');
  await trigger.click();
  await expect(page.getByLabel('Connection name',{exact:true})).toBeFocused();
  await page.getByLabel('Connection name',{exact:true}).fill('My records');
  await page.getByLabel('MCP server URL',{exact:true}).fill('https://records.example.test/mcp');
  const form=page.getByRole('form',{name:'Connect',exact:true});
  await form.getByRole('button',{name:'Connect',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('Connection was not saved');
  await expect(page.getByLabel('Connection name',{exact:true})).toHaveValue('My records');
  await form.getByRole('button',{name:'Connect',exact:true}).click();
  await expect(form).toHaveCount(0);
  await expect(page.locator('.connector-list')).toContainText('My records');
  await expect(trigger).toBeFocused();
  expect(writes).toBe(2);
});
