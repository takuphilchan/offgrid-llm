// Hidden, disposable native host check. No installed workspace or OS input.
// Pass a packaged executable, or Electron's executable plus apps/desktop.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {mkdtemp,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve,join,dirname} from 'node:path';
import {createRequire} from 'node:module';
const root=resolve(import.meta.dirname,'../..');
const require=createRequire(join(root,'apps/web/package.json'));
const {_electron:electron,expect}=require('@playwright/test');
assert.ok(process.argv[2],'Pass an Electron or packaged executable');
const binary=resolve(process.argv[2]),args=process.argv[3]?[resolve(process.argv[3])]:[];
const profile=await mkdtemp(join(tmpdir(),'offgrid-caption-'));
await writeFile(join(profile,'desktop-presentation.json'),JSON.stringify({locale:'en',theme:'light'}));
const server=createServer((_req,res)=>res.writeHead(200,{'Content-Type':'application/json'}).end('{}'));
server.listen(0,'127.0.0.1');await once(server,'listening');
const env={...process.env,OFFGRID_DESKTOP_HOME:profile,OFFGRID_DESKTOP_TEST_HIDDEN:'1',OFFGRID_PORT:String(server.address().port)};
delete env.ELECTRON_RUN_AS_NODE;
let app;
try {
 app=await electron.launch({executablePath:binary,args,cwd:dirname(binary),env,timeout:30000});
 const page=await app.firstWindow();
 await expect(page.locator('#offgrid-native-chrome')).toBeAttached();
 assert.equal(await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].isVisible()),false);
 assert.match(await page.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute('content'),/style-src 'self'/);
 const cdp=await page.context().newCDPSession(page);
 const find=(node,predicate)=>predicate(node)?node:[...(node.children??[]),...(node.shadowRoots??[])].map(child=>find(child,predicate)).find(Boolean);
 for(const zoom of [1,1.5,2]) {
  await app.evaluate(({BrowserWindow},factor)=>BrowserWindow.getAllWindows()[0].webContents.setZoomFactor(factor),zoom);
  await expect.poll(async()=>Math.round((await page.locator('#offgrid-native-chrome').boundingBox()).height*zoom)).toBe(32);
  const bounds=await page.locator('#offgrid-native-chrome').boundingBox();
  assert.equal(bounds.y,0);assert.equal(bounds.x,0);
  assert.equal(await page.locator('#offgrid-native-chrome').evaluate(el=>getComputedStyle(el).position),'fixed');
  assert.equal(await page.locator('#offgrid-native-chrome').evaluate(el=>el.shadowRoot),null,'Window actions remain in a closed shadow');
  const {root:tree}=await cdp.send('DOM.getDocument',{depth:-1,pierce:true});
  const host=find(tree,node=>node.attributes?.includes('offgrid-native-chrome'));
  const buttons=host.shadowRoots[0].children.filter(node=>node.nodeName==='BUTTON');
  assert.equal(buttons.length,3);
  for (const button of buttons) {
   const {model}=await cdp.send('DOM.getBoxModel',{nodeId:button.nodeId});
   assert.ok(model.width>=45&&model.width<=47,'Caption buttons retain their styled width under the strict CSP');
   assert.ok(model.height>=15&&model.height<=33);
   assert.ok(model.border[0]>bounds.width-145,'Caption buttons align at the right edge');
   assert.ok(button.attributes.includes('aria-label'),'Every caption has an accessible name');
  }
  assert.ok(await page.locator('main').evaluate(el=>el.getBoundingClientRect().top>=parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--native-chrome-height'))));
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'Native startup must not create a horizontal scrollbar');
 }
 await app.evaluate(({BrowserWindow})=>{const wc=BrowserWindow.getAllWindows()[0].webContents;wc.setZoomFactor(1);wc.setBackgroundThrottling(false);wc.send('startup-state',{state:'starting',managedByDesktop:true});});
 await expect.poll(async()=>(await page.locator('#offgrid-native-chrome').boundingBox()).height).toBe(32);
 await expect(page.locator('html')).toHaveAttribute('data-startup-state','loading');
 await expect(page.locator('#startup-welcome')).toHaveText('Welcome to OffGrid.');
 await expect(page.locator('#indicator,.startup-spinner')).toHaveCount(0);
 await expect(page.locator('.startup-brand svg path').first()).toHaveCSS('animation-name','startup-layer-top');
 for(const theme of ['light','dark']) {
  await page.evaluate(theme=>window.electron.setPresentation({theme,locale:'en'}),theme);
  await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
  // Hidden windows may return the previously committed compositor frame. Let
  // layout paint, then prime capture before retaining visual evidence.
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await app.evaluate(async({BrowserWindow})=>{await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined,{stayHidden:true,stayAwake:true});});
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  const png=await app.evaluate(async({BrowserWindow})=>(await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined,{stayHidden:true,stayAwake:true})).toPNG().toString('base64'));
  await writeFile(join(profile,`captions-${theme}.png`),Buffer.from(png,'base64'));
 }
 console.log(JSON.stringify({passed:true,profile,strictStartupCSP:true,closedCaptionControls:true,zooms:[1,1.5,2],hidden:true}));
} finally {
 if(app)await app.close();
 server.closeAllConnections();server.close();
}
