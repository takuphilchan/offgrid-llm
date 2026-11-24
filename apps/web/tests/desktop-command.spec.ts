import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {desktopCommand} from '../src/i18n/desktop-command';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';

async function desktop(page:Page,locale:'en'|'ar'|'zh-Hans'='en') {
 const fixture=await craftFixture(page,locale,'dark');
 await page.addInitScript(()=>{
  let state={mode:'full',shortcut:'CommandOrControl+Shift+Space',registered:true};
  const listeners=new Set<(value:typeof state)=>void>();
  const publish=()=>listeners.forEach(listener=>listener(state));
  Object.defineProperty(window,'outerWidth',{configurable:true,get:()=>innerWidth*((window as any).nativeZoom??1)});
  const change=(mode:string)=>{state={...state,mode};publish();return Promise.resolve(state);};
  Object.assign(window,{electron:{isDesktop:true,platform:'win32',commandWindowProtocol:1,
   getSystemTheme:async()=>'dark',onThemeChange:()=>()=>{},getBackendInfo:async()=>({state:'ready',managedByDesktop:false}),
   getCommandWindow:async()=>state,onCommandWindow:(callback:(value:typeof state)=>void)=>{listeners.add(callback);return()=>{listeners.delete(callback);};},
   openCommandWindow:()=>change('compact'),expandCommandWindow:()=>change('full'),
   dismissCommandWindow:async()=>{window.dispatchEvent(new Event('offgrid-desktop-dismiss'));return state;},
   setCommandShortcut:async(shortcut:string)=>{state={...state,shortcut,registered:shortcut!==''&&shortcut!=='Alt+Space'};publish();return state;},
   setCommandLayout:async(layout:string)=>{(window as any).commandLayout=layout;return state;}
  }});
 });
 return fixture;
}

test('summon and expand preserve the draft without submitting or duplicating work',async({page})=>{
 const fixture=await desktop(page);
 await page.goto('/ui/#/chat');
 const input=page.locator('.composer textarea');
 await input.fill('Open the application I choose');
 const before=fixture.writes.length;
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 await page.setViewportSize({width:720,height:480});
 await expect(page.locator('.app-shell')).toHaveClass(/command-window/);
 await expect(input).toHaveValue('Open the application I choose');
 await expect(page.locator('#workspace-sidebar')).toBeHidden();
 await page.getByRole('button',{name:desktopCommand('en').expand,exact:true}).click();
 await expect(page.locator('.app-shell')).not.toHaveClass(/command-window/);
 await expect(input).toHaveValue('Open the application I choose');
 expect(fixture.writes.length).toBe(before);
});

test('settings is the configuration destination and shortcut conflict is recoverable',async({page})=>{
 await desktop(page);await page.goto('/ui/#/settings/shortcuts');
 await expect(page.locator('.desktop-command-settings')).toBeVisible();
 await page.getByLabel(desktopCommand('en').shortcut,{exact:true}).selectOption('Alt+Space');
 await expect(page.getByText(desktopCommand('en').conflict,{exact:true})).toBeVisible();
 await page.getByLabel(desktopCommand('en').shortcut,{exact:true}).selectOption('CommandOrControl+Alt+Space');
 await expect(page.getByText(desktopCommand('en').conflict,{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:desktopCommand('en').open,exact:true}).click();
 await expect(page.locator('.composer textarea')).toBeVisible();
 await expect(page.locator('.app-shell')).toHaveClass(/command-window/);
});

for(const locale of ['en','ar','zh-Hans'] as const) test(`compact rendering ${locale}`,async({page})=>{
 await desktop(page,locale);await page.setViewportSize({width:720,height:480});await page.goto('/ui/#/chat');
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 await expect(page.locator('.command-window-header')).toBeVisible();
 await expect(page.locator('.composer textarea')).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 const directory=resolve('../../build/ui-craft/desktop-agent');mkdirSync(directory,{recursive:true});
 await page.screenshot({path:resolve(directory,`quick-${locale}.png`)});
});

test('idle command input stays compact and typing does not submit a task',async({page})=>{
 const fixture=await desktop(page);await page.goto('/ui/#/chat/new');
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 await page.setViewportSize({width:720,height:144});
 const input=page.locator('.composer textarea');
 await expect(input).toBeInViewport();
 const before=fixture.writes.length;
 await input.fill('Find the browser tab I was working on');
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeInViewport();
 const directory=resolve('../../build/ui-craft/desktop-agent');mkdirSync(directory,{recursive:true});
 await page.screenshot({path:resolve(directory,'quick-input.png')});
 expect(fixture.writes.length).toBe(before);
});

test('saved reduced motion applies on first Chat load before visiting Settings',async({page})=>{
 await desktop(page);
 await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'offgrid',dark:'offgrid',textSize:100,reduceMotion:true})));
 await page.goto('/ui/#/chat');
 await expect(page.locator('html')).toHaveAttribute('data-reduce-motion','true');
 await page.getByRole('button',{name:'Response settings',exact:true}).click();
 await expect(page.getByRole('region',{name:'Response settings',exact:true})).toHaveCSS('animation-duration','1e-05s');
 await page.keyboard.press('Escape');
 await expect(page.getByRole('button',{name:'Response settings',exact:true})).toBeFocused();
});

test('compact input grows when native zoom leaves too little room and settles after resizing',async({page})=>{
 const fixture=await desktop(page);await page.goto('/ui/#/chat/new');
 const input=page.locator('.composer textarea');await input.fill('Keep this unsent request');
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 // The viewport sizes match a 680x164 native window at 200% zoom and its
 // existing 680x480 work layout. This fixture checks the resize protocol;
 // the packaged native check separately exercises actual Chromium zoom.
 await page.setViewportSize({width:340,height:82});
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('work');
 await page.setViewportSize({width:340,height:240});
 const picker=page.locator('.composer .model-select-trigger');
 await expect(input).toBeInViewport();await expect(picker).toBeInViewport();
 await picker.click();await page.keyboard.press('Escape');
 await expect(picker).toBeFocused();
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('work');
 await page.setViewportSize({width:680,height:164});
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('input');
 await expect(input).toHaveValue('Keep this unsent request');
 await expect(page.locator('.composer .model-select-trigger')).toHaveCount(1);
 expect(fixture.writes).toEqual([]);
});

test('an intermediate native resize does not retain a larger input layout',async({page})=>{
 const fixture=await desktop(page);await page.goto('/ui/#/chat/new');
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 await page.setViewportSize({width:680,height:164});
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('input');
 await page.evaluate(()=>document.documentElement.dataset.windowTransition='resizing');
 await page.setViewportSize({width:680,height:82});
 await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
 await page.setViewportSize({width:680,height:164});
 await page.evaluate(()=>delete document.documentElement.dataset.windowTransition);
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('input');
 // Chromium can expose the resized innerHeight before CSS viewport units
 // settle. The old workspace geometry must not become a retained overflow.
 await page.locator('.workspace').evaluate(node=>(node as HTMLElement).style.height='480px');
 await page.setViewportSize({width:680,height:82});
 await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
 await page.setViewportSize({width:680,height:164});
 await page.locator('.workspace').evaluate(node=>(node as HTMLElement).style.removeProperty('height'));
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('input');
 expect(fixture.writes).toEqual([]);
});

test('zooming while the model menu closes keeps enough room for input',async({page})=>{
 const fixture=await desktop(page);await page.goto('/ui/#/chat/new');
 await page.evaluate(()=>window.electron!.openCommandWindow!());
 const input=page.locator('.composer textarea');await input.fill('Retain the request');
 await page.setViewportSize({width:680,height:164});
 const picker=page.locator('.composer .model-select-trigger');await picker.click();
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('work');
 await page.evaluate(()=>(window as any).nativeZoom=2);
 await page.setViewportSize({width:340,height:240});
 await page.keyboard.press('Escape');await expect(picker).toBeFocused();
 await expect(page.locator('html')).not.toHaveAttribute('data-window-transition',/./);
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('work');
 await expect(input).toHaveValue('Retain the request');expect(fixture.writes).toEqual([]);
});

for(const locale of ['en','ar','zh-Hans'] as const) for(const width of [320,680]) test(`quick model menu grows and preserves input ${locale} ${width}`,async({page})=>{
 const fixture=await desktop(page,locale);await page.goto('/ui/#/chat/new');
 await page.evaluate(()=>window.electron!.openCommandWindow!());await page.setViewportSize({width,height:164});
 const input=page.locator('.composer textarea');await input.fill('Keep this request');
 const model=page.locator('.composer-actions .model-select-trigger');
 await expect(model).toBeInViewport();await expect(input).toBeInViewport();
 await model.click();await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('work');
 await page.setViewportSize({width,height:480});
 await expect(page.locator('.model-select-options [role=option]').first()).toBeInViewport();
 await page.keyboard.press('Escape');await expect(model).toBeFocused();
 await expect.poll(()=>page.evaluate(()=>(window as any).commandLayout)).toBe('input');
 await expect(input).toHaveValue('Keep this request');expect(fixture.writes).toEqual([]);
});
