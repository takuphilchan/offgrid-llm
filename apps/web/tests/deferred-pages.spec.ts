import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

const modelsModule=/\/assets\/ModelsPage-[^/]+\.js$/;

test('Chat defers secondary screens, retains its draft, and reuses loaded code',async({page})=>{
 const fixture=await craftFixture(page,'en','light');
 const chunks:string[]=[];
 page.on('request',request=>{if(/\/assets\/(?:Models|Settings|Knowledge|Activity|Agent)Page-[^/]+\.js$/.test(request.url())) chunks.push(request.url());});
 await page.goto('/ui/#/chat/new');
 await page.locator('.composer textarea').fill('Retain this question café 日本語');
 expect(chunks).toEqual([]);
 const startup=await page.evaluate(()=>performance.getEntriesByType('resource').filter(entry=>/\/assets\/.*\.js$/.test(entry.name)).map(entry=>{const resource=entry as PerformanceResourceTiming;return {asset:new URL(resource.name).pathname,bytes:resource.decodedBodySize,duration:resource.duration};}));
 await test.info().attach('chat-startup-code.json',{body:JSON.stringify(startup,null,2),contentType:'application/json'});
 console.log(JSON.stringify({fixture:'chat-startup-code',bytes:startup.reduce((sum,resource)=>sum+resource.bytes,0),assets:startup.length}));
 await page.locator('.sidebar a[href="#/models"]').click();
 await expect(page.locator('.installed-model').first()).toBeVisible();
 expect(chunks.filter(url=>modelsModule.test(url))).toHaveLength(1);
 await page.locator('.sidebar a[href="#/chat"]').click();
 await expect(page.locator('.composer textarea')).toHaveValue('Retain this question café 日本語');
 await page.locator('.sidebar a[href="#/models"]').click();
 await expect(page.locator('.installed-model').first()).toBeVisible();
 expect(chunks.filter(url=>modelsModule.test(url))).toHaveLength(1);
 expect(fixture.writes).toEqual([]);
});

test('delayed route loading is visible, cancellable by navigation, and respects reduced motion',async({page})=>{
 await craftFixture(page,'en','dark');
 await page.emulateMedia({reducedMotion:'reduce'});
 let release!:()=>void;
 const gate=new Promise<void>(resolve=>{release=resolve;});
 await page.route(modelsModule,async route=>{await gate;await route.continue();});
 await page.goto('/ui/#/chat/new');
 await page.locator('.composer textarea').fill('Draft before slow route');
 await page.locator('.sidebar a[href="#/models"]').click();
 await expect(page.locator('.deferred-page')).toBeVisible();
 expect(await page.locator('.deferred-page-indicator').evaluate(element=>getComputedStyle(element).animationName)).toBe('none');
 await page.locator('.sidebar a[href="#/chat"]').click();
 await expect(page.locator('.composer textarea')).toHaveValue('Draft before slow route');
 release();
 await expect(page.locator('.installed-model')).toHaveCount(0);
 await page.locator('.sidebar a[href="#/models"]').click();
 await expect(page.locator('.installed-model').first()).toBeVisible();
});

test('failed page load recovers by explicit reload and retains the saved draft',async({page})=>{
 await craftFixture(page,'en','light');
 let failed=false;
 await page.route(modelsModule,route=>{if(!failed){failed=true;return route.abort('failed');}return route.continue();});
 await page.goto('/ui/#/chat/new');
 await page.locator('.composer textarea').fill('Keep this during recovery');
 await page.locator('.sidebar a[href="#/models"]').click();
 await expect(page.locator('.page-failure')).toBeVisible();
 await page.getByRole('button',{name:'Reload app',exact:true}).click();
 await expect(page.locator('.installed-model').first()).toBeVisible();
 await page.locator('.sidebar a[href="#/chat"]').click();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this during recovery');
});
