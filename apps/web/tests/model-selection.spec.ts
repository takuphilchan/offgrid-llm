import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const host of ['browser', 'full-desktop'] as const) test(`opening the model picker and editing never blank the ${host} workspace`, async ({page}) => {
  const fixture = await craftFixture(page, 'en', 'light');
  if (host === 'full-desktop') await page.addInitScript(() => {
    const state = {mode:'full',shortcut:'',registered:false};
    Object.assign(window,{electron:{isDesktop:true,getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},getCommandWindow:async()=>state,onCommandWindow:()=>()=>{},setCommandLayout:async()=>state}});
  });
  await page.goto('/ui/#/chat/new');
  const editor=page.locator('.composer textarea'),trigger=page.locator('.model-select-trigger');
  await expect(editor).toBeVisible();
  await page.evaluate(() => {
    const phases:string[]=[];
    Object.assign(window,{observedPresentationPhases:phases});
    new MutationObserver(records=>{for (const record of records) phases.push(record.oldValue ?? document.documentElement.dataset.windowTransition ?? '');})
      .observe(document.documentElement,{attributes:true,attributeFilter:['data-window-transition'],attributeOldValue:true});
  });
  await trigger.click();await expect(page.locator('.model-select-panel')).toBeVisible();
  await expect(page.locator('.app-shell')).toHaveCSS('opacity','1');
  await page.keyboard.press('Escape');await expect(trigger).toBeFocused();
  const draft='Keep this question and the workspace visible while I edit it. '.repeat(4);
  await editor.fill(draft);await expect(editor).toHaveValue(draft);
  await editor.fill('Short draft');await expect(editor).toHaveValue('Short draft');
  await expect(page.locator('.app-shell')).toHaveCSS('opacity','1');
  expect(await page.evaluate(()=>(window as unknown as {observedPresentationPhases:string[]}).observedPresentationPhases)).toEqual([]);
  expect(fixture.writes).toEqual([]);
});

for (const locale of ['en', 'ar']) for (const width of [320, 390, 768, 1440]) test(`long model trigger stays inside composer without overlapping controls: ${locale} ${width}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  await page.setViewportSize({ width, height: 900 });
  await page.goto('/ui/#/chat');
  const trigger = page.locator('.model-select-trigger');
  await expect(trigger).toBeVisible();
  const errors = await page.locator('.composer').evaluate(composer => {
    const box = composer.getBoundingClientRect();
    const controls = [...composer.querySelectorAll<HTMLButtonElement>('.composer-actions > button, .composer-actions .model-select-trigger')].filter(node => node.getClientRects().length);
    const problems: string[] = [];
    for (const [index, control] of controls.entries()) {
      const rect = control.getBoundingClientRect();
      if (rect.left < box.left - 1 || rect.right > box.right + 1) problems.push(`outside: ${control.getAttribute('aria-label') ?? control.textContent}`);
      for (const other of controls.slice(index + 1)) {
        const next = other.getBoundingClientRect();
        if (Math.min(rect.right, next.right) - Math.max(rect.left, next.left) > 1 && Math.min(rect.bottom, next.bottom) - Math.max(rect.top, next.top) > 1) problems.push(`overlap: ${control.textContent} / ${other.textContent}`);
      }
    }
    return problems;
  });
  expect(errors).toEqual([]);
  const label=trigger.locator('.model-label-regular:visible,.model-label-narrow:visible');
  expect(await label.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
  await trigger.click();
  await expect(page.locator('.model-select-panel')).toBeVisible();
  expect(state.writes).toEqual([]);
});

test('model picker searches exact identifiers, shows actual readiness and preserves selection authority', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  const models = [
    { id: 'Qwen2.5-7B-Instruct-Q4_K_M', type: 'chat', loaded: true, context_window: 32768 },
    { id: 'Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589', type: 'chat', runtime_ready: false },
    { id: 'voice-only', type: 'tts' },
  ];
  await page.route('**/v1/models', r => r.fulfill({ json: { data: models } }));
  await page.goto('/ui/#/chat');
  const picker = page.getByRole('button', { name: 'Model', exact: true });
  await picker.click();
  await expect(page.getByRole('searchbox', { name: 'Filter installed models' })).toBeFocused();
  await expect(page.locator('.model-select-panel').getByRole('option')).toHaveCount(2);
  await expect(page.locator('.model-select-panel').getByRole('option').first()).toContainText('Loaded');
  await expect(page.locator('.model-select-panel').getByRole('option').last()).toContainText('Runtime ready: No');
  await page.getByRole('searchbox', { name: 'Filter installed models' }).fill('f1e3673a9589');
  await expect(page.locator('.model-select-panel').getByRole('option')).toHaveCount(1);
  await page.keyboard.press('ArrowDown');
  await expect(page.locator('.model-select-panel').getByRole('option')).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(picker).toContainText('Llama 3.1 8B');
  await expect(picker).toBeFocused();
  await expect(picker).toHaveAttribute('aria-expanded', 'false');
  expect(state.writes).toEqual([]);
  await picker.click();
  await expect(page.locator('.model-select-panel').getByRole('option', { selected: true })).toContainText(models[1].id);
  await page.keyboard.press('Escape');
  await expect(picker).toBeFocused();
});

for(const locale of ['en','ar']) for(const theme of ['light','dark']) test(`model picker has one scrolling list with fixed search: ${locale} ${theme}`,async({page},info)=>{
  const state=await craftFixture(page,locale,theme);
  const models=Array.from({length:30},(_,i)=>({id:`Research-Model-${i}-Q4_K_M`,type:'chat',context_window:32768}));
  await page.route('**/v1/models',r=>r.fulfill({json:{data:models}}));
  await page.setViewportSize({width:locale==='ar'?390:1280,height:640});
  await page.goto('/ui/#/chat');
  const trigger=page.locator('.model-select-trigger');await trigger.click();
  const panel=page.locator('.model-select-panel'),search=panel.getByRole('searchbox'),list=panel.getByRole('listbox');
  await expect(search).toBeFocused();
  await expect(panel).toHaveCSS('opacity','1');
  // Measure the fixed search after the entrance translation has finished.
  // Opacity stays at one throughout this animation and is not a readiness check.
  await panel.evaluate(async el=>{await Promise.all(el.getAnimations().map(animation=>animation.finished));});
  const top=(await search.boundingBox())!.y;
  const overflow=await panel.evaluate(el=>[el,...Array.from(el.querySelectorAll('*'))].filter(n=>n.scrollHeight>n.clientHeight+1&&['auto','scroll'].includes(getComputedStyle(n).overflowY)).map(n=>n.className));
  expect(overflow).toEqual(['model-select-options']);
  await search.press('ArrowDown'); await page.keyboard.press('End');
  await expect(list.getByRole('option').last()).toBeFocused();
  await expect.poll(()=>list.evaluate(el=>el.scrollTop)).toBeGreaterThan(0);
  expect(Math.abs((await search.boundingBox())!.y-top)).toBeLessThan(1);
  await expect(panel.getByRole('link')).toBeInViewport();
  const row = list.getByRole('option').last();
  expect((await row.boundingBox())!.height).toBeLessThan(105);
  expect((await list.boundingBox())!.height).toBeGreaterThan(150);
  expect(await row.evaluate(el=>{
    const outer=el.getBoundingClientRect();
    return Array.from(el.children).every(child=>{const box=child.getBoundingClientRect();return box.top>=outer.top&&box.bottom<=outer.bottom;});
  })).toBe(true);
  await expect(row).toBeInViewport({ratio:1});
  await page.screenshot({path:info.outputPath('model-picker.png')});
  await page.keyboard.press('Escape');await expect(trigger).toBeFocused();
  expect(state.writes).toEqual([]);
});

for (const locale of ['en', 'ar']) test(`model picker fits narrow ${locale} and distinguishes same-family variants`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'dark');
  await page.route('**/v1/models', r => r.fulfill({ json: { data: ['Q4_K_M', 'Q8_0'].map(quant => ({ id: `Qwen2.5-7B-Instruct-${quant}`, type: 'chat' })) } }));
  await page.setViewportSize({ width: 390, height: 640 });
  await page.goto('/ui/#/chat');
  await page.locator('.model-select-trigger').click();
  await expect(page.locator('.model-select-panel').getByRole('option')).toHaveCount(2);
  await expect(page.locator('.model-select-panel').getByRole('option').first()).toContainText('Q4_K_M');
  await expect(page.locator('.model-select-panel').getByRole('option').last()).toContainText('Q8_0');
  await expect(page.locator('.model-select-panel').getByRole('option').first().locator('strong')).toBeVisible();
  await expect(page.locator('.model-select-trigger > span')).toBeVisible();
  expect(await page.locator('.model-select-panel').getByRole('option').first().evaluate(node => getComputedStyle(node).display)).toBe('grid');
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath(`model-picker-${locale}.png`), fullPage: true, animations: 'disabled' });
  expect(state.writes).toEqual([]);
});

for(const locale of ['en','ar']) test('enlarged text keeps model and input actions separate: '+locale,async({page})=>{
 await craftFixture(page,locale,'light');
 await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'offgrid',dark:'offgrid',textSize:150,reduceMotion:true})));
 await page.route('**/v1/models',r=>r.fulfill({json:{data:[{id:'MistralSmall-24B-Instruct-Q4_K_M',type:'chat'}]}}));
 await page.setViewportSize({width:320,height:800});await page.goto('/ui/#/chat');
 await expect(page.locator('.model-select-trigger')).toBeVisible();
 const controls=await page.locator('.composer-footer').evaluate(el=>{
 const buttons=Array.from(el.querySelectorAll('button')).filter(b=>b.getClientRects().length);const parent=el.closest('.composer')!.getBoundingClientRect();
 return buttons.flatMap((button,i)=>{const r=button.getBoundingClientRect();const errors=[];if(r.left<parent.left-1||r.right>parent.right+1)errors.push({outside:button.getAttribute('aria-label'),r:r.toJSON(),parent:parent.toJSON()});for(const other of buttons.slice(i+1)){const n=other.getBoundingClientRect();if(Math.min(r.right,n.right)-Math.max(r.left,n.left)>1&&Math.min(r.bottom,n.bottom)-Math.max(r.top,n.top)>1)errors.push({overlap:[button.getAttribute('aria-label'),other.getAttribute('aria-label')]});}return errors;});
 });await page.screenshot({path:test.info().outputPath('enlarged-model.png')});expect(controls).toEqual([]);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
