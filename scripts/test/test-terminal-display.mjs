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
 if(process.env.OFFGRID_TERMINAL_CAPTURE){
  const capture=await readFile(process.env.OFFGRID_TERMINAL_CAPTURE,'utf8');
  await page.emulateMedia({forcedColors:'none'});
  await page.setViewportSize({width:900,height:550});
  await page.evaluate(()=>fixtureEvent({type:'appearance',theme:'dark'}));
  await page.evaluate(text=>new Promise(resolve=>{const term=tabs.get(current).term;term.reset();term.resize(72,20);term.write(text,resolve);}),capture);
  await page.screenshot({path:join(evidence,'live-agent.png')});
 }
 assert.deepEqual(errors,[]);
 console.log(JSON.stringify({passed:true,evidence,checks:['existing/new tabs','no settings keystrokes to PTY','focus restoration','persistence/reload','reset','reduced motion','forced colors','narrow dialog','command output snapshot','edited context sharing','search counts and empty-query decoration cleanup','running command completion','private edits retained on shell events','failed handoff retention','duplicate handoff guard','late acknowledgement isolation','empty command output'],nativePTY:false}));
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
