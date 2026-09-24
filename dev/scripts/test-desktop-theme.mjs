// Installed Electron main/preload/renderer check, with an isolated profile and
// local HTTP fixture. This never changes OS appearance or the user's workspace.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve, sep, extname} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
const root = resolve(import.meta.dirname, '../..');
const require = createRequire(join(root, 'web/app/package.json'));
const {_electron: electron} = require('playwright');
const {expect} = require('@playwright/test');
assert.ok(process.argv[2], 'Pass the packaged desktop executable');
const binary = resolve(process.argv[2]);
const ui = join(root, 'web/dist');
const version = JSON.parse(await readFile(join(root, 'desktop/package.json'), 'utf8')).version;
const uiBuild = createHash('sha256').update((await readFile(join(ui, 'index.html'), 'utf8')).replace(/\r\n/g, '\n')).digest('hex');
const evidence = await mkdtemp(join(tmpdir(), 'offgrid-desktop-theme-'));
await writeFile(join(evidence, 'desktop-presentation.json'), JSON.stringify({locale:'en',theme:'dark'}));
const server = createServer(async (request, response) => {
  try {
    const pathname = new URL(request.url, 'http://127.0.0.1').pathname;
    if (pathname.startsWith('/ui/')) {
      const file = resolve(ui, pathname.slice(4) || 'index.html');
      assert.ok(file.startsWith(ui + sep));
      const types = {'.html':'text/html','.js':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2'};
      response.setHeader('Content-Type', types[extname(file)] || 'application/octet-stream');
      response.end(await readFile(file)); return;
    }
    const data = {
      '/api/v2/system': {product:'offgrid',version,api_version:2,ui_build_id:uiBuild,capabilities:['sessions-v1','chat-streaming-v1','durable-agent-runs-v1','task-first-agents-v2']},
      '/health': {status:'healthy'}, '/v1/users/me': {authenticated:false,auth_required:false,user:null},
      '/v1/models': {data:[{id:'theme-fixture',type:'chat'}]}, '/v1/sessions': {sessions:[]},
      '/v1/agents/tasks': [], '/v1/agents/tools': {tools:[],enabled_count:0},
      '/api/v2/jobs': [],
      '/v1/agents/mcp': {servers:[]}, '/v1/integrations': {integrations:[]},
      '/api/v2/computer/sessions': {sessions:[]}
    };
    response.setHeader('Content-Type','application/json');
    response.end(JSON.stringify(data[pathname] || {}));
  } catch { response.statusCode = 500; response.end('{}'); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const env = {...process.env,OFFGRID_DESKTOP_HOME:evidence,OFFGRID_PORT:String(server.address().port)};
delete env.ELECTRON_RUN_AS_NODE;
let app;
async function launch() {
  app = await electron.launch({executablePath:binary,env,timeout:30000});
  const page = await app.firstWindow();
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete','true'));
  await page.waitForURL(/\/ui\//);
  await page.reload();
  return page;
}
try {
  let page = await launch();
  assert.equal(await app.evaluate(({nativeTheme}) => nativeTheme.themeSource), 'dark', 'Saved preference must reach native controls on startup');
  await page.goto(`http://127.0.0.1:${server.address().port}/ui/#/settings`);
  for (const theme of ['light','dark','system','dark']) {
    await page.getByRole('button',{name:{light:'Light',dark:'Dark',system:'System'}[theme],exact:true}).click();
    await expect.poll(() => app.evaluate(({nativeTheme}) => nativeTheme.themeSource)).toBe(theme);
    const effective = await app.evaluate(({nativeTheme}) => nativeTheme.shouldUseDarkColors ? 'dark' : 'light');
    await expect(page.locator('html')).toHaveAttribute('data-theme', effective);
    const language = page.locator('.locale-picker select');
    await expect(language.locator('option')).toHaveCount(9);
    for (const option of await language.locator('option').all()) {
      await expect(option).toHaveCSS('color', effective === 'dark' ? 'rgb(245, 245, 244)' : 'rgb(23, 23, 23)');
      await expect(option).toHaveCSS('background-color', effective === 'dark' ? 'rgb(24, 24, 25)' : 'rgb(255, 255, 255)');
    }
    await language.focus();
    await language.press('ArrowDown');
    await language.press('Enter');
    await expect(language).toHaveValue('fr');
    await language.selectOption('en');
  }
  await page.screenshot({path:join(evidence,'dark-controls.png')});
  await app.close(); app = null;
  page = await launch();
  assert.equal(await app.evaluate(({nativeTheme}) => nativeTheme.themeSource), 'dark');
  await expect(page.locator('html')).toHaveAttribute('data-theme','dark');
  console.log(JSON.stringify({passed:true,platform:process.platform,savedAndLiveNativeTheme:true,readableOptions:9,evidence}));
} finally {
  await app?.close(); server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
}
