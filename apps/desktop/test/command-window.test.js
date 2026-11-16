const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createCommandWindow, saveShortcut } = require('../command-window');
function fixture() {
  let bounds = { x: 80, y: 60, width: 1200, height: 800 }, maximized = false, visible = true, focused = true;
  const messages = [], calls = [], callbacks = new Map();
  const window = { isDestroyed: () => false, isVisible: () => visible, isFocused: () => focused, isMinimized: () => false, isFullScreen: () => false,
    isMaximized: () => maximized, getNormalBounds: () => ({ ...bounds }), setBounds: value => { bounds = value; }, setMinimumSize: (...value) => calls.push(['minimum', ...value]),
    setAlwaysOnTop: value => calls.push(['top', value]), unmaximize: () => { maximized = false; }, maximize: () => { maximized = true; },
    setResizable: value=>calls.push(['resizable',value]),setMaximizable:value=>calls.push(['maximizable',value]),setFullScreenable:value=>calls.push(['fullscreenable',value]),setMaximumSize:(...value)=>calls.push(['maximum',...value]),
    show: () => { visible = true; }, hide: () => { visible = false; }, focus: () => { focused = true; }, webContents: { send: (...value) => messages.push(value) } };
  const area = { x: 0, y: 0, width: 1920, height: 1080 };
  let currentWindow = window;
  const shortcuts = { register: (key, callback) => { if (key === 'Alt+Space') return false; callbacks.set(key, callback); return true; }, unregister: key => callbacks.delete(key) };
  const control = createCommandWindow({ window: () => currentWindow, screen: { getCursorScreenPoint: () => ({}), getDisplayNearestPoint: () => ({workArea: area}), getDisplayMatching: () => ({workArea: area}) }, globalShortcut: shortcuts, ready: () => true, changed: value => messages.push(value), hideTerminal: () => calls.push(['terminal']) });
  return { control, window, calls, messages, callbacks, area, bounds: () => bounds, replaceWindow: () => {currentWindow = {...window};} };
}
test('compact and expanded modes preserve one window and full geometry', async () => {
  const f = fixture(), original = f.bounds();
  await f.control.open(); assert.equal(f.control.snapshot().mode, 'compact'); assert.equal(f.bounds().width, 680);
  assert.deepEqual(f.control.fullState().bounds, original);
  f.control.dismiss(); assert.equal(f.window.isVisible(), false);
  assert.ok(f.messages.some(value => Array.isArray(value) && value[0] === 'command-window-dismiss'));
  await f.control.open(); await f.control.expand(); assert.deepEqual(f.bounds(), original); assert.equal(f.control.fullState(), null);
  assert.ok(f.calls.some(value => value[0] === 'top' && value[1] === false));
});
test('shortcut collision is reported and registration is released on quit', async () => {
  const f = fixture();
  assert.equal(f.control.configure('CommandOrControl+Shift+Space').registered, true);
  f.callbacks.get('CommandOrControl+Shift+Space')(); assert.equal(f.control.snapshot().mode, 'compact');
  f.callbacks.get('CommandOrControl+Shift+Space')(); assert.equal(f.window.isVisible(), false);
  assert.equal(f.control.configure('Alt+Space').registered, false); assert.equal(f.callbacks.size, 0);
  assert.throws(() => f.control.configure('path/to/program'), /invalid/);
  f.control.configure('CommandOrControl+Alt+Space'); f.control.dispose(); assert.equal(f.callbacks.size, 0);
});
test('display removal restores full geometry within remaining work area', async () => {
  const f = fixture(); await f.control.open(); f.area.width = 1024; f.area.height = 768; await f.control.expand();
  assert.deepEqual(f.bounds(), {x: 0, y: 0, width: 1024, height: 768});
});

test('compact window grows for work and voice without overwriting full-window bounds', async () => {
 const f=fixture(), original=f.bounds();
 await f.control.open();assert.equal(f.bounds().height,164);
 await f.control.setLayout('voice');assert.equal(f.bounds().height,540);assert.equal(f.bounds().width,420);
 await f.control.setLayout('work');assert.equal(f.bounds().height,480);
 assert.throws(()=>f.control.setLayout({width:4000}),/invalid/);
 await f.control.expand();assert.deepEqual(f.bounds(),original);
 await f.control.setLayout('voice');assert.deepEqual(f.bounds(),original);
});

test('failed preference writes preserve the active shortcut and invalid choices never reach disk', async () => {
 const f = fixture(); f.control.configure('CommandOrControl+Shift+Space');
 await assert.rejects(saveShortcut('Alt+Space', async () => {throw Error('disk full');}, f.control.configure), /disk full/);
 assert.equal(f.control.snapshot().shortcut, 'CommandOrControl+Shift+Space');
 assert.equal(f.callbacks.size, 1);
 await assert.rejects(saveShortcut('arbitrary command', async () => {assert.fail('must not persist');}, f.control.configure), /invalid/);
 let stored;
 const status = await saveShortcut('Alt+Space', async value => {stored=value;}, f.control.configure);
 assert.equal(stored, 'Alt+Space'); assert.equal(status.registered, false);
});

test('full-screen state and small display constraints survive compact mode', async () => {
 const f=fixture(); let fullScreen=true;
 f.window.isFullScreen=()=>fullScreen; f.window.setFullScreen=value=>{fullScreen=value;};
 await f.control.open(); assert.equal(fullScreen,false);
 f.area.width=600;f.area.height=400;await f.control.expand();
 assert.equal(fullScreen,true);assert.deepEqual(f.bounds(),{x:0,y:0,width:600,height:400});
 assert.deepEqual(f.calls.filter(value=>value[0]==='minimum').at(-1),['minimum',600,400]);
});

test('repeated compact toggles do not accumulate native DPI rounding', async () => {
 const f=fixture(),original=f.bounds(),setBounds=f.window.setBounds;
 f.window.setBounds=value=>setBounds({...value,height:value.height+1});
 for(let i=0;i<5;i++){await f.control.open();await f.control.expand();assert.equal(f.bounds().height,original.height+1);}
 setBounds({...original,width:1000,height:700});
 await f.control.open();await f.control.expand();assert.equal(f.bounds().width,1000);assert.equal(f.bounds().height,701);
});

test('a recreated native window starts with fresh presentation geometry', async () => {
 const f=fixture();await f.control.open();await f.control.setLayout('voice');
 f.replaceWindow();assert.equal(f.control.snapshot().mode,'full');assert.equal(f.control.fullState(),null);
 await f.control.open();assert.equal(f.bounds().height,164);
});

test('compact presentation disables native maximize and resizing, then restores both',async()=>{
 const f=fixture();f.window.maximize();await f.control.open();
 assert.equal(f.window.isMaximized(),false);
 for(const name of ['resizable','maximizable','fullscreenable'])assert.deepEqual(f.calls.filter(call=>call[0]===name).at(-1),[name,false]);
 await f.control.expand();assert.equal(f.window.isMaximized(),true);
 for(const name of ['resizable','maximizable','fullscreenable'])assert.deepEqual(f.calls.filter(call=>call[0]===name).at(-1),[name,true]);
});
