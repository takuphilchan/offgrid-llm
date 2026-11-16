const test = require('node:test');
const assert = require('node:assert/strict');
const { applyBranding } = require('../branding.cjs');
const pkg = require('../package.json');
const { windowChrome, updateChrome } = require('../branding.cjs');

test('Windows uses one frameless host with preload captions and a theme-matched background', () => {
  for (const theme of ['light', 'dark']) {
    const options = windowChrome('win32', theme);
    assert.equal(options.frame, false);
    assert.equal(options.titleBarOverlay, undefined);
    let background, overlay;
    updateChrome({ isDestroyed: () => false, setBackgroundColor: v => background = v, setTitleBarOverlay: v => overlay = v }, 'win32', theme);
    assert.equal(background, options.backgroundColor);
    assert.equal(overlay, undefined);
  }
  for (const platform of ['darwin', 'linux']) assert.equal(windowChrome(platform, 'light').titleBarStyle, undefined);
  updateChrome({ isDestroyed: () => true }, 'win32', 'dark');
});

test('caption actions are bounded and compact mode cannot maximize',()=>{
 const {controlWindow}=require('../branding.cjs'),calls=[];
 const window={isDestroyed:()=>false,isMaximized:()=>false,maximize:()=>calls.push('maximize'),unmaximize:()=>calls.push('restore'),minimize:()=>calls.push('minimize'),close:()=>calls.push('close')};
 controlWindow(window,'maximize',true);assert.deepEqual(calls,[]);
 controlWindow(window,'maximize');controlWindow(window,'minimize');controlWindow(window,'close');
 assert.deepEqual(calls,['maximize','minimize','close']);
 assert.throws(()=>controlWindow(window,'arbitrary'),/invalid_window_control/);
 const captions=require('../window-controls.json'),locales=require('../language-registry.json');
 assert.deepEqual(Object.keys(captions).sort(),Object.keys(locales).sort());
 for(const row of Object.values(captions))assert.ok(row.length===3&&row.every(value=>typeof value==='string'&&value.trim()));
 const {localizedCopy}=require('../locale-messages');
 assert.deepEqual(Object.values(localizedCopy(captions,'en','desktop/window-controls',{en:{'desktop/window-controls':{'0':'Reviewed minimize'}}})),['Reviewed minimize',captions.en[1],captions.en[2]]);
 assert.ok(pkg.build.files.includes('window-controls.json'));
 const preload=require('node:fs').readFileSync(require('node:path').join(__dirname,'../preload.js'),'utf8');
 assert.ok(preload.includes("attachShadow({mode:'closed'})"));
 assert.ok(preload.includes("if(event.isTrusted)void ipcRenderer.invoke('window-chrome-action',action)"));
 assert.ok(!preload.slice(preload.indexOf("contextBridge.exposeInMainWorld")).includes('window-chrome-action'));
});

test('public brand preserves the existing Electron profile before changing the name', () => {
  let name = pkg.name;
  let pinned;
  const app = {
    getPath(key) { assert.equal(key, 'userData'); return pinned || `profiles/${name}`; },
    setName(value) { name = value; },
    setPath(key, value) { assert.equal(key, 'userData'); pinned = value; }
  };
  assert.equal(applyBranding(app), 'OffGrid');
  assert.equal(name, 'OffGrid');
  assert.equal(app.getPath('userData'), 'profiles/offgrid-llm-desktop');
  assert.equal(pkg.build.appId, 'com.offgrid.llm.desktop');
  assert.equal(pkg.build.win.executableName, 'OffGrid LLM Desktop');
  assert.equal(pkg.build.linux.executableName, 'offgrid-llm-desktop');
  assert.equal(pkg.build.nsis.shortcutName, 'OffGrid');
  assert.ok(pkg.build.files.includes('branding.cjs'));
});
