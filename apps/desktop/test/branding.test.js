const test = require('node:test');
const assert = require('node:assert/strict');
const { applyBranding } = require('../branding.cjs');
const pkg = require('../package.json');
const { windowChrome, updateChrome } = require('../branding.cjs');

test('Windows keeps native caption controls on the sidebar surface without a duplicate title', () => {
  for (const theme of ['light', 'dark']) {
    const options = windowChrome('win32', theme);
    assert.equal(options.titleBarStyle, 'hidden');
    assert.equal(options.titleBarOverlay.height, 36);
    assert.equal(options.backgroundColor, options.titleBarOverlay.color);
    let background, overlay;
    updateChrome({ isDestroyed: () => false, setBackgroundColor: v => background = v, setTitleBarOverlay: v => overlay = v }, 'win32', theme);
    assert.equal(background, options.backgroundColor);
    assert.deepEqual(overlay, options.titleBarOverlay);
  }
  for (const platform of ['darwin', 'linux']) assert.equal(windowChrome(platform, 'light').titleBarStyle, undefined);
  updateChrome({ isDestroyed: () => true }, 'win32', 'dark');
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
