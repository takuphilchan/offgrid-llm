import { test, expect, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { availablePalettes, type PaletteMode } from '../src/appearance';

async function fixture(page: Page, palette: string, mode: PaletteMode, locale = 'sw', textSize = 100) {
  const state = await craftFixture(page, locale, mode);
  await page.addInitScript(({palette,textSize}) => localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:palette,dark:palette,textSize,reduceMotion:false})),{palette,textSize});
  await page.route(/\/api\/v2\/models(?:\?|$)/, r => r.fulfill({json:{models:[1,2].map(n=>({
    id:`fixture-speech-${n}`,name:`Speech fixture ${n}`,revision:'1',kind:'package',category:'speech_recognition',
    provenance:{repository:'fixture/speech'},engine:'whisper',format:'ggml',backend:'cpu',
    readiness:{integrity:'checked',runtime_compatible:true},
    package:{integrity:'checked',runtime_compatible:true,manifest:{license:'CC0',runtime:{adapter:'whisper',revision:'1'},artifacts:[]}}
  }))}}));
  return state;
}

// Detect two rules around an empty strip of content, rather than merely checking
// that a particular CSS selector has been edited. Exclude interactive controls.
async function emptyDoubleRules(page: Page) {
  return page.locator('main').evaluate(main => {
    const lines: {el: Element;x:number;right:number;y:number;label:string}[] = [];
    for(const el of main.querySelectorAll('*')) {
      if(el.matches('a,button,input,select,textarea,[role=tab]') || el.closest('[hidden]')) continue;
      const r=el.getBoundingClientRect(),s=getComputedStyle(el);
      if(r.width<100 || !r.height || s.visibility==='hidden') continue;
      // Enclosed cards, code blocks and table cells are functional boundaries.
      if(parseFloat(s.borderLeftWidth)>0 && parseFloat(s.borderRightWidth)>0) continue;
      for(const side of ['Top','Bottom'] as const) {
        const width=parseFloat(s[`border${side}Width`]);
        if(width>0 && width<=2 && s[`border${side}Style`]!=='none' && !['transparent','rgba(0, 0, 0, 0)'].includes(s[`border${side}Color`]))
          lines.push({el,x:r.left,right:r.right,y:side==='Top'?r.top:r.bottom,label:el.tagName+'.'+el.className+':'+side});
      }
    }
    const textRects: DOMRect[]=[];
    const walker=document.createTreeWalker(main,NodeFilter.SHOW_TEXT);
    while(walker.nextNode())if(walker.currentNode.textContent?.trim()){
      const range=document.createRange();range.selectNodeContents(walker.currentNode);textRects.push(...range.getClientRects());
    }
    const duplicates:string[]=[];
    for(const a of lines)for(const b of lines){
      const left=Math.max(a.x,b.x),right=Math.min(a.right,b.right);
      if(a.el===b.el || b.y-a.y<0.5 || b.y-a.y>32 || right-left<Math.min(a.right-a.x,b.right-b.x)*.85)continue;
      if(!textRects.some(r=>r.width && r.height && r.top<b.y && r.bottom>a.y && r.left<right && r.right>left))duplicates.push(`${a.label} / ${b.label}`);
    }
    return duplicates;
  });
}

for(const mode of ['light','dark'] as const)for(const palette of availablePalettes(mode)){
  test(`Models boundary and settings separators: ${palette} ${mode}`,async({page})=>{
    const state=await fixture(page,palette,mode);
    await page.goto('/ui/#/models');
    await page.locator('.model-category-picker button').nth(2).click();
    const rows=page.locator('.installed-package-list > article');
    await expect(rows).toHaveCount(2);
    await expect(rows.last().locator(':scope > details > summary')).toBeVisible();
    expect(await emptyDoubleRules(page)).toEqual([]);
    const advanced=page.locator('#model-library-panel > details.model-section');
    await expect(advanced).toBeVisible();
    await expect(advanced).toHaveCSS('border-top-width','0px');
    await expect(rows.first()).toHaveCSS('border-bottom-width','1px');
    await expect(rows.last()).toHaveCSS('border-bottom-width','0px');
    await advanced.locator('summary').focus();await page.keyboard.press('Enter');
    const bounds=await advanced.locator('summary').boundingBox();
    expect(bounds!.width).toBeLessThan((await advanced.boundingBox())!.width*.9);
    await expect(advanced).toHaveAttribute('open','');
    expect(await emptyDoubleRules(page)).toEqual([]);
    await page.keyboard.press('Enter');
    if(['dracula','warm','solarized','contrast'].includes(palette))await page.screenshot({path:test.info().outputPath('models.png'),fullPage:true});
    await page.goto('/ui/#/settings');
    await expect(page.locator('#settings-language')).toBeVisible();
    for(const panel of await page.locator('.settings-panel').all())await expect(panel).toHaveCSS('border-bottom-width','0px');
    for(const row of await page.locator('.appearance-preference-row').all())await expect(row).toHaveCSS('border-bottom-width','0px');
    expect(await emptyDoubleRules(page)).toEqual([]);
    await page.locator('.theme-summary').first().click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.press('Escape');
    expect(state.writes).toEqual([]);
  });
}

for(const [locale,width,textSize] of [['ar',390,150],['de',640,150],['sw',1280,100]] as const){
  test(`main-route divider audit ${locale} ${width} ${textSize}`,async({page})=>{
    const state=await fixture(page,'dracula','light',locale,textSize);
    await page.setViewportSize({width,height:900});
    for(const route of ['chat','knowledge','agents','activity','settings','models']){
      await page.goto('/ui/#/'+route);
      await expect(page.locator('main')).toBeVisible();
      await expect(page.locator('.deferred-page')).toHaveCount(0);
      expect(await emptyDoubleRules(page),route).toEqual([]);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),route).toBe(true);
      await page.screenshot({path:test.info().outputPath(route+'.png'),fullPage:true});
    }
    expect(state.writes).toEqual([]);
  });
}

test('Markdown thematic break and forced colors preserve semantics and keyboard focus',async({page})=>{
  await fixture(page,'dracula','light','en');
  const session={name:'Demo briefing',model_id:'fixture',messages:[{role:'assistant',content:'Before the break\n\n---\n\nAfter the break'}]};
  await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
  await page.route(/\/v1\/sessions\/Demo(?:%20| )briefing$/,r=>r.fulfill({json:session}));
  await page.goto('/ui/#/chat');
  await page.getByRole('button',{name:'Demo briefing',exact:true}).click();
  const rule=page.locator('.message-body hr');
  await expect(rule).toBeVisible();
  await expect(rule).toHaveCSS('border-top-width','1px');
  await expect(rule).toHaveCSS('border-bottom-width','0px');
  await page.emulateMedia({forcedColors:'active'});
  const colors=await rule.evaluate(el=>({rule:getComputedStyle(el).borderTopColor,text:getComputedStyle(document.body).color}));
  expect(colors.rule).toBe(colors.text);
  await page.goto('/ui/#/models');
  await page.keyboard.press('Tab');
  await page.locator('#model-view-installed').focus();
  await expect(page.locator('#model-view-installed')).toHaveCSS('outline-style','solid');
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('#model-view-discover')).toBeFocused();
  await expect(page.locator('#model-view-discover')).toHaveCSS('border-bottom-width','2px');
  await page.screenshot({path:test.info().outputPath('forced-colors.png')});
});
