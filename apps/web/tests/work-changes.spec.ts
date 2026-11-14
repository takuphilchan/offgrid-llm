import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import {expect, test, type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

async function fixture(page: Page, locale = 'en', integrationState = 'partial') {
  const craft = await craftFixture(page, locale, 'dark');
  const data = Buffer.from('Complete proposed file with final citations.\n');
  const hash = createHash('sha256').update(data).digest('hex');
  const retained = Buffer.from('User edits retained during interrupted integration.\n');
  const retainedHash = createHash('sha256').update(retained).digest('hex');
  const change = {id:'change-a',work_id:'work-root',run_id:'run-root',worker_run_id:'run-child',kind:'candidate',state:integrationState,instruction_version:1,control_epoch:1,current:true,operation_count:2,validation_state:'passed',validation_count:1,integration_id:'integration-a',integrated_count:integrationState === 'abandoned' ? 0 : 1,available:true};
  const state = {revoked:false,corrupt:false,wrongWork:false,requests:[] as string[],mutations:[] as string[]};
  const root = {run_id:'run-root',task_id:'run-root',work_id:'work-root',prompt:'Edit and test',model:'fixture',status:'completed',output:'Partial result',steps:[],event_cursor:'1'};
  const session = {name:'Demo briefing',model_id:'fixture',messages:[{role:'user',content:root.prompt}],turn:{id:'turn-root',job_id:root.run_id,status:root.status,prompt:root.prompt,job_admission:'linked'}};
  await page.route('**/v1/sessions', route => route.fulfill({json:{sessions:[session]}}));
  await page.route('**/v1/sessions/Demo%20briefing', route => route.fulfill({json:session}));
  await page.route('**/v1/sessions/Demo%20briefing/turn', route => route.fulfill({json:{turn:session.turn,job:root}}));
  await page.route('**/api/v2/jobs/**', route => route.fulfill({json:root}));
  await page.route('**/api/v2/work-items/**', async route => {
    const request = route.request(), url = new URL(request.url());
    state.requests.push(url.pathname + url.search);
    if (request.method() !== 'GET') {state.mutations.push(request.method()); return route.fulfill({status:405});}
    if (url.pathname.endsWith('/changes')) {
      const after = url.searchParams.get('after');
      return route.fulfill({json:{items:[{...change,id:after ? 'change-b' : change.id,available:!state.revoked}],next_cursor:after ? 'change-b' : change.id,has_more:!after}});
    }
    if (state.revoked) return route.fulfill({status:403,json:{error:{message:'Source access revoked'}}});
    const view = url.searchParams.get('view'), offset = Number(url.searchParams.get('offset'));
    const detail = {change:{...change,id:url.pathname.split('/').at(-1),work_id:state.wrongWork ? 'foreign' : change.work_id},view};
    if (view === 'files') return route.fulfill({json:{...detail,files:[{path:offset ? 'later.txt' : 'src/proposed.txt',kind:'update',result:{kind:'file',bytes:data.length,mode:420,sha256:hash}}],...(offset ? {} : {next_offset:1})}});
    if (view === 'validation') return route.fulfill({json:{...detail,checks:[{id:'check',state:'exited',exit_code:0,request_digest:'request',receipt_digest:'receipt'}]}});
    if (view === 'integration') return route.fulfill({json:integrationState === 'abandoned' ? {...detail,error:'execution_unsupported',receipts:[]} : {...detail,conflict_path:'later.txt',receipts:[{ordinal:1,path:'src/proposed.txt',operation:'update',state:'integrated',dispatched:true,original_retained:true},{ordinal:2,path:'later.txt',operation:'update',state:'conflict',dispatched:false,original_retained:true}]}});
    if (view === 'recovery') return route.fulfill({json:{...detail,recovery:[{id:'1:displaced',path:'src/proposed.txt',role:'unconfirmed',bytes:retained.length,sha256:retainedHash}]}});
    if (view === 'content' || view === 'preserved') {
      if (view === 'preserved') expect(url.searchParams.get('artifact')).toBe('1:displaced');
      const source = view === 'preserved' ? retained : data, sourceHash = view === 'preserved' ? retainedHash : hash;
      expect(url.searchParams.get('hash')).toBe(sourceHash);
      const chunk = Buffer.from(source.subarray(offset,offset+8));
      if (state.corrupt) chunk[0] ^= 1;
      return route.fulfill({json:{...detail,content:{sha256:sourceHash,bytes:source.length,offset,data:chunk.toString('base64')},...(offset+chunk.length < source.length ? {next_offset:offset+chunk.length} : {})}});
    }
    throw new Error(`Unexpected change view ${view}`);
  });
  await page.goto('/ui/#/chat');
  const panel = page.locator('.work-changes');
  await expect(panel).toBeVisible();
  expect(state.requests).toHaveLength(0);
  return {state,craft,data,retained,panel,detail:panel.locator('.work-change').first()};
}

test('saved change pages, validation and partial integration stay read-only', async ({page}) => {
  const f = await fixture(page);
  await f.panel.locator(':scope > summary').click();
  await f.panel.locator('.work-changes-body > button').filter({hasText:'Load more'}).click();
  await expect(f.panel.locator('.work-change')).toHaveCount(2);
  const second = f.panel.locator('.work-change').last();
  await second.locator('summary').click();
  await expect(second).toContainText('src/proposed.txt');
  await f.panel.locator('.work-changes-body > button').filter({hasText:'Refresh'}).click();
  await expect(f.panel.locator('.work-changes-body')).toHaveAttribute('aria-busy','false');
  await expect(f.panel.locator('.work-change')).toHaveCount(2);
  await expect(second).toHaveAttribute('open','');
  await f.detail.locator('summary').click();
  await expect(f.detail).toContainText('src/proposed.txt');
  await f.detail.getByRole('button',{name:'Load more',exact:true}).click();
  await expect(f.detail).toContainText('later.txt');
  await f.detail.getByRole('button',{name:'Validation',exact:true}).click();
  await expect(f.detail.locator('li')).toContainText('Passed');
  await f.detail.getByRole('button',{name:'Integration receipts',exact:true}).click();
  await expect(f.detail).toContainText('Conflict: later.txt');
  await expect(f.detail.locator('li').first()).toContainText('Integrated');
  await expect(f.detail.locator('li').first()).toContainText('Original retained');
  await expect(f.detail.locator('li').last()).toContainText('Conflict');
  expect(f.state.mutations).toEqual([]);
  expect(f.craft.writes).toEqual([]);
});

for (const state of ['partial','abandoned']) test(`proposed-file downloads retain sealed bytes after ${state} integration`, async ({page}) => {
  const f = await fixture(page, 'en', state);
  await f.panel.locator(':scope > summary').click();
  await f.detail.locator('summary').click();
  const pending = page.waitForEvent('download');
  await f.detail.getByRole('button',{name:'Download proposed file'}).click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe('proposed.txt');
  expect(await readFile((await download.path())!)).toEqual(f.data);
  expect(f.state.requests.filter(url => url.includes('view=content')).length).toBeGreaterThan(1);
  expect(f.state.mutations).toEqual([]);
  if (state === 'abandoned') {
    await expect(f.detail.locator('summary')).toContainText('Failed');
    await f.detail.getByRole('button',{name:'Integration receipts',exact:true}).click();
    await expect(f.detail).toContainText('execution_unsupported');
  }
});

test('retained-file downloads preserve uncertainty and never restore over current files', async ({page}) => {
  const f = await fixture(page);
  await f.panel.locator(':scope > summary').click(); await f.detail.locator('summary').click();
  await f.detail.getByRole('button',{name:'Preserved files',exact:true}).click();
  await expect(f.detail.locator('li')).toContainText('Uncertain');
  const pending = page.waitForEvent('download');
  await f.detail.getByRole('button',{name:'Download retained file',exact:true}).click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe('retained-proposed.txt');
  expect(await readFile((await download.path())!)).toEqual(f.retained);
  await expect(f.detail.locator('li')).toContainText('Uncertain');
  expect(f.state.requests.filter(url => url.includes('view=preserved')).length).toBeGreaterThan(1);
  expect(f.state.mutations).toEqual([]); expect(f.craft.writes).toEqual([]);
});

for (const failure of ['corrupt','revoked'] as const) test(`retained-file ${failure} prevents download and clears cached recovery`, async ({page}) => {
  const f = await fixture(page), downloads:string[] = [];
  page.on('download', download => downloads.push(download.suggestedFilename()));
  await f.panel.locator(':scope > summary').click(); await f.detail.locator('summary').click();
  await f.detail.getByRole('button',{name:'Preserved files',exact:true}).click();
  await expect(f.detail.getByRole('button',{name:'Download retained file',exact:true})).toBeEnabled();
  f.state[failure] = true;
  await f.detail.getByRole('button',{name:'Download retained file',exact:true}).click();
  await expect(f.detail.getByRole('alert')).toBeVisible();
  await expect(f.detail.getByRole('button',{name:'Download retained file',exact:true})).toHaveCount(0);
  expect(downloads).toEqual([]); expect(f.state.mutations).toEqual([]);
});

for (const failure of ['corrupt','wrongWork'] as const) test(`rejects ${failure} evidence without publishing a download`, async ({page}) => {
  const f = await fixture(page), downloads:string[] = [];
  page.on('download', download => downloads.push(download.suggestedFilename()));
  await f.panel.locator(':scope > summary').click();
  await f.detail.locator('summary').click();
  await expect(f.detail.getByRole('button',{name:'Download proposed file'})).toBeEnabled();
  f.state[failure] = true;
  await f.detail.getByRole('button',{name:'Download proposed file'}).click();
  await expect(f.detail.getByRole('alert')).toBeVisible();
  await expect(f.detail.getByRole('button',{name:'Download proposed file'})).toHaveCount(0);
  expect(downloads).toEqual([]);
});

test('revocation removes cached file details and retains unavailable status', async ({page}) => {
  const f = await fixture(page);
  await f.panel.locator(':scope > summary').click();
  await f.detail.locator('summary').click();
  await expect(f.detail).toContainText('src/proposed.txt');
  f.state.revoked = true;
  await f.detail.getByRole('button',{name:'Refresh',exact:true}).click();
  await expect(f.detail.getByRole('alert')).toContainText('Source access revoked');
  await expect(f.detail).not.toContainText('src/proposed.txt');
  await f.panel.locator('.work-changes-body > button').filter({hasText:'Refresh'}).click();
  await expect(f.detail.getByRole('status')).toContainText('Source access is unavailable');
  await expect(f.detail.getByRole('button')).toHaveCount(0);
  expect(f.state.mutations).toEqual([]);
});

test('change inspection supports keyboard use and narrow RTL layout', async ({page}) => {
  await page.setViewportSize({width:390,height:844});
  const f = await fixture(page,'ar');
  const summary = f.panel.locator(':scope > summary');
  await summary.focus(); await page.keyboard.press('Enter');
  const candidate = f.detail.locator('summary');
  await candidate.focus(); await page.keyboard.press('Enter');
  await expect(f.detail).toContainText('src/proposed.txt');
  expect(await page.locator('html').getAttribute('dir')).toBe('rtl');
  expect((await f.panel.boundingBox())!.width).toBeLessThanOrEqual(390);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(f.state.mutations).toEqual([]);
});
