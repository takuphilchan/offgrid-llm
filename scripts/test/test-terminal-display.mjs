// Real bundled renderer/xterm with a synthetic bridge. No shell or installed
// workspace is opened; native process lifecycle has separate packaged tests.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile,mkdtemp} from 'node:fs/promises';
import {createRequire} from 'node:module';
import {join,resolve,sep,extname} from 'node:path';
import {tmpdir} from 'node:os';
const root=resolve(import.meta.dirname,'../..'),desktop=join(root,'apps/desktop');
const require=createRequire(join(root,'apps/web/package.json'));
const {chromium,expect}=require('@playwright/test');
const server=createServer(async(req,res)=>{try{const file=resolve(desktop,'.'+new URL(req.url,'http://localhost').pathname);assert.ok(file.startsWith(desktop+sep));res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css'})[extname(file)]||'application/octet-stream');res.end(await readFile(file));}catch{res.statusCode=404;res.end();}});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const browser=await chromium.launch({headless:true}),page=await browser.newPage({viewport:{width:1100,height:700}});
const errors=[];page.on('pageerror',error=>errors.push(error.message));
const evidence=await mkdtemp(join(tmpdir(),'offgrid-terminal-display-'));
await page.addInitScript(()=>{
 window.fixtureCalls=[];let sequence=0,callback=()=>{};
 window.terminalDesktop={onEvent(fn){callback=fn;window.fixtureEvent=fn;},async request(value){fixtureCalls.push(value);switch(value.action){case 'share':if(window.fixtureShareMode==='delay')return new Promise((resolve,reject)=>{window.finishShare=resolve;window.failShare=reject;});return {};case 'handshake':return {generation:'fixture',available:true,presentation:{visible:true,theme:'light'}};case 'create':return {id:String(++sequence),title:'OffGrid CLI',cwd:'C:\\fixture',state:'running'};case 'attach':setTimeout(()=>callback({type:'data',id:value.id,sequence:1,text:'OffGrid terminal fixture\r\nPS C:\\fixture> '}),0);return {};default:return {};}}};
});
try{
 await page.goto(`http://127.0.0.1:${server.address().port}/terminal.html`);
 await page.locator('#start').click();
 await expect.poll(()=>page.evaluate(()=>tabs.size)).toBe(1);
 await expect(page.locator('#commands')).toBeHidden();
 await page.evaluate(()=>tabs.get(current).term.select(0,0,7));
 await expect(page.getByRole('button',{name:'Ask about selection',exact:true})).toBeVisible();
 await page.locator('#commands').click();
 await expect(page.locator('#context-command')).toBeHidden();
 await expect(page.locator('#context-output')).toHaveValue('OffGrid');
 await page.keyboard.press('Escape');
 await page.evaluate(()=>tabs.get(current).term.clearSelection());
 await expect(page.locator('#commands')).toBeHidden();
 await page.getByRole('button',{name:'Terminal display',exact:true}).click();
 await expect(page.getByRole('dialog')).toBeVisible();
 await page.getByLabel('Text size',{exact:true}).selectOption('18');
 await page.getByLabel('Line spacing',{exact:true}).selectOption('1.6');
 await page.getByLabel('Cursor',{exact:true}).selectOption('bar');
 await page.getByLabel('Scrollback',{exact:true}).selectOption('10000');
 await page.getByLabel('Scrollback',{exact:true}).focus();
 await page.evaluate(()=>{fixtureEvent({type:'visibility',visible:false});fixtureEvent({type:'visibility',visible:true});fixtureEvent({type:'layout'});});
 await page.waitForTimeout(50);
 await expect(page.getByLabel('Scrollback',{exact:true})).toBeFocused();
 assert.deepEqual(await page.evaluate(()=>{const t=[...tabs.values()][0].term;return [t.options.fontSize,t.options.lineHeight,t.options.cursorStyle,t.options.scrollback,t.options.disableStdin];}),[18,1.6,'bar',10000,true]);
 assert.equal(await page.evaluate(()=>fixtureCalls.filter(c=>c.action==='input').length),0);
 await page.keyboard.press('Escape');
 await expect(page.getByRole('dialog')).toBeHidden();
 await expect(page.locator('#display-settings')).toBeFocused();
 await expect.poll(()=>page.evaluate(()=>[...tabs.values()][0].term.options.disableStdin)).toBe(false);
 await page.locator('#search').click();
 await page.locator('#query').fill('fixture');
 await page.evaluate(()=>{fixtureEvent({type:'visibility',visible:false});fixtureEvent({type:'visibility',visible:true});});
 await expect(page.locator('#query')).toBeFocused();
 await expect(page.locator('#find-result')).toHaveText('1 of 2');
 await page.locator('#query').fill('not-present');await expect(page.locator('#find-result')).toHaveText('No matches');
 await page.locator('#query').fill('fixture');await page.locator('#query').fill('');
 await expect(page.locator('#find-result')).toHaveText('');
 await expect.poll(()=>page.locator('.xterm-decoration').count()).toBe(0);
 await expect(page.locator('#next')).toBeDisabled();
 await page.locator('#query').fill('fixture');
 await page.evaluate(()=>fixtureEvent({type:'layout'}));await expect(page.locator('#query')).toBeFocused();
 await page.locator('#find-close').click();
 await expect.poll(()=>page.locator('.xterm-decoration').count()).toBe(0);
 await page.locator('#new').click();await page.locator('#cli').click();
 await expect.poll(()=>page.evaluate(()=>tabs.size)).toBe(2);
 assert.equal(await page.evaluate(()=>tabs.get(current).term.options.fontSize),18);
 await page.emulateMedia({reducedMotion:'reduce'});
 await expect.poll(()=>page.evaluate(()=>tabs.get(current).term.options.cursorBlink)).toBe(false);
 await page.evaluate(()=>fixtureEvent({type:'appearance',theme:'dark'}));
 await page.locator('#display-settings').click();
 await page.screenshot({path:join(evidence,'display-dark.png')});
 await page.setViewportSize({width:400,height:450});
 assert.equal(await page.evaluate(()=>{const r=document.querySelector('dialog').getBoundingClientRect();return r.x>=0&&r.right<=innerWidth&&r.y>=0&&r.bottom<=innerHeight;}),true);
 await page.screenshot({path:join(evidence,'display-narrow.png')});
 // A 200% desktop zoom leaves the equivalent CSS viewport of 550 x 350.
 await page.setViewportSize({width:550,height:350});
 await page.getByRole('button',{name:'Done',exact:true}).scrollIntoViewIfNeeded();
 assert.equal(await page.evaluate(()=>{const d=document.querySelector('dialog'),r=d.getBoundingClientRect();return r.x>=0&&r.right<=innerWidth&&r.y>=0&&r.bottom<=innerHeight&&d.scrollWidth<=d.clientWidth;}),true);
 await page.screenshot({path:join(evidence,'display-zoom-equivalent.png')});
 await page.emulateMedia({forcedColors:'active'});
 await expect.poll(()=>page.evaluate(()=>{const t=tabs.get(current).term.options.theme;return t.red===t.foreground&&t.blue===t.foreground;})).toBe(true);
 await page.keyboard.press('Escape');
 await page.reload();
 await page.locator('#start').click();
 await expect.poll(()=>page.evaluate(()=>tabs.size)).toBe(1);
 assert.equal(await page.evaluate(()=>tabs.get(current).term.options.fontSize),18);
 await page.locator('#display-settings').click();await page.getByRole('button',{name:'Reset defaults'}).click();
 assert.equal(await page.evaluate(()=>tabs.get(current).term.options.fontSize),14);
 await page.getByRole('button',{name:'Done',exact:true}).click();
 await page.emulateMedia({forcedColors:'none'});await page.setViewportSize({width:1100,height:700});
 await page.evaluate(()=>new Promise(resolve=>{
  const mark=value=>'\x1b]633;OffGrid;'+btoa(unescape(encodeURIComponent(JSON.stringify({v:1,...value}))))+'\x07';
  tabs.get(current).term.write('\r\n'+mark({event:'begin',command:'Write-Host sample',cwd:'C:\\changed',truncated:false})+'sample café 日本語'+mark({event:'end',exit:7})+mark({event:'prompt',cwd:'C:\\changed'})+'PS next> ',resolve);
 }));
 await expect(page.locator('#identity')).toContainText('changed');
 await page.locator('#commands').click();
 await expect(page.locator('#context-output')).toHaveValue('sample café 日本語');
 await expect(page.locator('#context-state')).toContainText('exit 7');
 await page.locator('#context-output').fill('redacted output');
 await page.locator('#context-question').fill('Explain this');
 await page.locator('#context-share').click();
 const shares=await page.evaluate(()=>fixtureCalls.filter(c=>c.action==='share'));
 assert.equal(shares.length,1);assert.equal(shares[0].context.output,'redacted output');assert.equal(shares[0].context.command,'Write-Host sample');
 assert.equal(await page.evaluate(()=>fixtureCalls.filter(c=>c.action==='input').length),0);
 await page.locator('#commands').click();await page.setViewportSize({width:400,height:450});
 assert.equal(await page.evaluate(()=>{const d=document.querySelector('#terminal-commands'),r=d.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&d.scrollWidth<=d.clientWidth;}),true);
 await page.screenshot({path:join(evidence,'command-context-narrow.png')});
 await page.keyboard.press('Escape');

 // Command completion must update an already-open preview, without replaying input.
 await page.setViewportSize({width:1100,height:700});
 await page.evaluate(()=>new Promise(resolve=>{
  const mark=value=>'\x1b]633;OffGrid;'+btoa(JSON.stringify({v:1,...value}))+'\x07';
  tabs.get(current).term.write('\r\n'+mark({event:'begin',command:'slow command',cwd:'C:\\fixture',truncated:false}),resolve);
 }));
 await page.locator('#commands').click();
 await expect(page.getByRole('dialog',{name:'Command context',exact:true})).toBeVisible();
 await expect(page.locator('#context-state')).toHaveText('Command still running');
 await expect(page.locator('#context-share')).toBeDisabled();
 await page.locator('#context-question').fill('Explain this result');
 await page.evaluate(()=>new Promise(resolve=>{
  const mark=value=>'\x1b]633;OffGrid;'+btoa(JSON.stringify({v:1,...value}))+'\x07';
  tabs.get(current).term.write('finished output'+mark({event:'end',exit:0}),resolve);
 }));
 await expect(page.locator('#context-output')).toHaveValue('finished output');
 await expect(page.locator('#context-share')).toBeEnabled();
 await page.locator('#context-output').fill('private material removed');
 await page.evaluate(()=>new Promise(resolve=>tabs.get(current).term.write('\x1b]633;OffGrid;'+btoa(JSON.stringify({v:1,event:'prompt',cwd:'C:\\fixture'}))+'\x07',resolve)));
 await expect(page.locator('#context-output')).toHaveValue('private material removed');
 // Pending sends cannot be duplicated by editing the form. Failure retains edits.
 await page.evaluate(()=>window.fixtureShareMode='delay');
 await page.locator('#context-share').click();
 await page.locator('#context-question').fill('Updated question');
 await expect(page.locator('#context-share')).toBeDisabled();
 assert.equal(await page.evaluate(()=>fixtureCalls.filter(c=>c.action==='share').length),2);
 await page.evaluate(()=>window.failShare(new Error('Chat unavailable; try again')));
 await expect(page.locator('#context-error')).toContainText('Chat unavailable');
 await expect(page.locator('#context-output')).toHaveValue('private material removed');
 await expect(page.locator('#context-question')).toHaveValue('Updated question');
 await expect(page.locator('#context-share')).toBeEnabled();
 // A late reply to a dismissed preview must not close a newly opened preview.
 await page.locator('#context-share').click();await page.keyboard.press('Escape');
 await page.locator('#commands').click();await page.locator('#context-question').fill('A new preview');
 await page.evaluate(()=>window.finishShare({}));
 await expect(page.getByRole('dialog',{name:'Command context',exact:true})).toBeVisible();
 await expect(page.locator('#context-question')).toHaveValue('A new preview');
 await page.keyboard.press('Escape');
 await page.evaluate(()=>window.fixtureShareMode='normal');
 // Empty completed commands have a truthful state, with sharing disabled.
 await page.evaluate(()=>new Promise(resolve=>{
  const mark=value=>'\x1b]633;OffGrid;'+btoa(JSON.stringify({v:1,...value}))+'\x07';
  tabs.get(current).term.write('\r\n'+mark({event:'begin',command:'no output',cwd:'C:\\fixture',truncated:false})+mark({event:'end',exit:0}),resolve);
 }));
 await page.locator('#commands').click();
 await expect(page.locator('#context-limits')).toHaveText('This command produced no text output.');
 await expect(page.locator('#context-copy')).toBeDisabled();await page.keyboard.press('Escape');
 // Exercise the actual xterm renderer with every new locally bundled palette.
 const catalog=require(join(desktop,'theme-palettes.json'));
 await page.emulateMedia({forcedColors:'none',reducedMotion:'no-preference'});
 await page.setViewportSize({width:1000,height:650});
 for(const [id,modes] of Object.entries(catalog.terminal))for(const [mode,ansi] of Object.entries(modes)){
  const palette={...catalog.themes[id][mode],ansi};
  await page.evaluate(({mode,palette})=>fixtureEvent({type:'appearance',theme:mode,palette,reduceMotion:true}),{mode,palette});
  const actual=await page.evaluate(()=>({theme:tabs.get(current).term.options.theme,blink:tabs.get(current).term.options.cursorBlink}));
  assert.equal(actual.theme.background,palette['surface-0']);assert.equal(actual.theme.cursorAccent,palette['surface-0']);
  for(const [key,color] of Object.entries(ansi))assert.equal(actual.theme[key],color);
  assert.equal(actual.blink,false);
 }
 await page.evaluate(()=>new Promise(resolve=>tabs.get(current).term.write('\r\n\x1b[31mError\x1b[0m  \x1b[32mSuccess\x1b[0m  \x1b[33mWarning\x1b[0m  \x1b[34mLink\x1b[0m\r\n',resolve)));
 await page.screenshot({path:join(evidence,'established-palette-terminal.png')});
 await page.emulateMedia({forcedColors:'active'});
 await expect.poll(()=>page.evaluate(()=>{const t=tabs.get(current).term.options.theme;return t.red===t.foreground;})).toBe(true);
 if(process.env.OFFGRID_TERMINAL_CAPTURE){
  const capture=await readFile(process.env.OFFGRID_TERMINAL_CAPTURE,'utf8');
  await page.emulateMedia({forcedColors:'none'});
  await page.setViewportSize({width:900,height:550});
  await page.evaluate(()=>fixtureEvent({type:'appearance',theme:'dark'}));
  await page.evaluate(text=>new Promise(resolve=>{const term=tabs.get(current).term;term.reset();term.resize(72,20);term.write(text,resolve);}),capture);
  await page.screenshot({path:join(evidence,'live-agent.png')});
 }
 // Language updates localize authored controls without touching shell output,
 // selected text, command edits or the terminal process.
 await page.keyboard.press('Escape');
 const terminalCopy=require(join(desktop,'terminal-translations.js'));
 const terminalOverrides=require(join(root,'apps/web/src/i18n/feature-packs.json'));
 const registry=require(join(desktop,'language-registry.json'));
 for(const locale of Object.keys(registry))for(const key of Object.keys(terminalCopy.catalogs.en))assert.ok(Object.hasOwn(terminalOverrides[locale]?.['desktop/terminal']??{},key)||Object.hasOwn(terminalCopy.catalogs[locale]??{},key),`${locale}: missing terminal message ${key}`);
 const beforeLanguage=await page.evaluate(()=>({output:tabs.get(current).term.buffer.active.getLine(0)?.translateToString(),inputs:fixtureCalls.filter(c=>c.action==='input').length,tabs:tabs.size}));
 for(const locale of Object.keys(terminalCopy.catalogs)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'dark'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('html')).toHaveAttribute('lang',locale);
  await expect(page.locator('#display-settings')).toHaveAccessibleName(terminalCopy.text(locale,'Terminal display'));
  await expect(page.locator('.xterm-helper-textarea').first()).toHaveAttribute('aria-label',terminalCopy.text(locale,'Terminal input'));
  await page.locator('#display-settings').click();
  await expect(page.locator('#terminal-display-title')).toHaveText(terminalCopy.text(locale,'Terminal display'));
  await expect(page.locator('option[value="underline"]')).toHaveText(terminalCopy.text(locale,'Underline'));
  await expect(page.locator('#terminal-display button[value="close"]').last()).toBeInViewport();
  if(['ar','de','sn'].includes(locale))await page.screenshot({path:join(evidence,`terminal-language-${locale}.png`)});
  await page.keyboard.press('Escape');
 }
 assert.deepEqual(await page.evaluate(()=>({output:tabs.get(current).term.buffer.active.getLine(0)?.translateToString(),inputs:fixtureCalls.filter(c=>c.action==='input').length,tabs:tabs.size})),beforeLanguage);
 // Switch language while a real command preview is already open. Redacted
 // output and the user's question must survive all 29 language changes.
 await page.evaluate(()=>new Promise(resolve=>{
  const mark=value=>'\x1b]633;OffGrid;'+btoa(unescape(encodeURIComponent(JSON.stringify({v:1,...value}))))+'\x07';
  tabs.get(current).term.clearSelection();
  tabs.get(current).term.write('\r\n'+mark({event:'begin',command:'echo audit',cwd:'C:\\fixture',truncated:false})+'audit output'+mark({event:'end',exit:4}),resolve);
 }));
 await page.locator('#commands').click();await page.locator('#context-output').fill('Keep this redaction — café 日本語');await page.locator('#context-question').fill('Keep my question');
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('#context-output')).toHaveValue('Keep this redaction — café 日本語');await expect(page.locator('#context-question')).toHaveValue('Keep my question');
  await expect(page.locator('#context-state')).toContainText(terminalCopy.text(locale,'Shell reported exit {code} · {seconds}s',{code:4,seconds:''}).split('4')[0]);
  await expect(page.locator('#context-share')).toHaveText(terminalCopy.text(locale,'Continue in Chat'));
  const markers=await page.evaluate(()=>{
   const result=[];
   for(const element of document.querySelectorAll('[data-terminal-text]'))result.push({key:element.dataset.terminalText,actual:element.textContent});
   for(const [marker,attribute] of [['data-terminal-label','aria-label'],['data-terminal-title','title'],['data-terminal-placeholder','placeholder']])for(const element of document.querySelectorAll('['+marker+']'))result.push({key:element.getAttribute(marker),actual:element.getAttribute(attribute)});
   return result;
  });
  for(const {key,actual} of markers)assert.equal(actual,terminalCopy.text(locale,key),`${locale}: ${key}`);
 }
 await page.keyboard.press('Escape');await page.locator('#search').click();await page.locator('#query').fill('audit');
 const searchBefore=await page.evaluate(()=>tabs.get(current).term.getSelectionPosition());
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('#query')).toHaveValue('audit');
  assert.deepEqual(await page.evaluate(()=>tabs.get(current).term.getSelectionPosition()),searchBefore);
 }
 await page.locator('#find-close').click();
 await page.locator('#new').click();await page.locator('#agent').click();
 await expect.poll(()=>page.evaluate(()=>tabs.get(current)?.kind)).toBe('agent');
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('[role="tab"][aria-selected="true"]')).toHaveAccessibleName(terminalCopy.text(locale,'Agent'));
 }
 await page.locator('#display-settings').click();
 await page.evaluate(()=>{window.fixtureStorageSet=Storage.prototype.setItem;Storage.prototype.setItem=()=>{throw new DOMException('Fixture storage denied','SecurityError');};});
 await page.locator('select[name="fontSize"]').selectOption('16');
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('#display-save-status')).toHaveText(terminalCopy.text(locale,'Applied for this window; preferences could not be saved.'));
  await expect(page.locator('select[name="fontSize"]')).toHaveValue('16');
 }
 await page.evaluate(()=>{Storage.prototype.setItem=window.fixtureStorageSet;});await page.keyboard.press('Escape');
 await page.evaluate(()=>tabs.get(current).term.selectAll());await page.locator('#commands').click();
 await page.locator('#context-output').fill('Private selection redacted');await page.locator('#context-question').fill('Keep selected-text question');
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('#context-output')).toHaveValue('Private selection redacted');
  await expect(page.locator('#context-question')).toHaveValue('Keep selected-text question');
  await expect(page.locator('#context-state')).toHaveText(terminalCopy.text(locale,'Manually selected output · command and outcome not verified'));
 }
 await page.keyboard.press('Escape');
 await page.evaluate(()=>fixtureEvent({type:'error',message:'fixture_backend_error: original diagnostic <untrusted>'}));
 for(const locale of Object.keys(registry)){
  await page.evaluate(({locale,translations})=>fixtureEvent({type:'appearance',locale,translations,theme:'light'}),{locale,translations:terminalOverrides[locale]?.['desktop/terminal']??{}});
  await expect(page.locator('#notice')).toHaveText(terminalCopy.text(locale,'The terminal request could not be confirmed. Check its state before trying again.'));
  await expect(page.locator('#terminal-error-details summary')).toHaveText(terminalCopy.text(locale,'Technical details'));
  await expect(page.locator('#terminal-error-text')).toHaveText('fixture_backend_error: original diagnostic <untrusted>');
  await expect(page.locator('#terminal-error-text untrusted')).toHaveCount(0);
 }
 assert.deepEqual(errors,[]);
 console.log(JSON.stringify({passed:true,evidence,checks:['existing/new tabs','no settings keystrokes to PTY','focus restoration','persistence/reload','reset','reduced motion','forced colors','narrow dialog','command output snapshot','edited context sharing','search counts and empty-query decoration cleanup','running command completion','private edits retained on shell events','failed handoff retention','duplicate handoff guard','late acknowledgement isolation','empty command output'],nativePTY:false}));
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
