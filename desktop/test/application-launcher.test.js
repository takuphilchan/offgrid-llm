const test = require('node:test');
const assert = require('node:assert/strict');
const { createApplicationLauncher, parseDesktopFile, titleFrom } = require('../application-launcher');
const path = require('node:path');

function fakeFs(files, dirs = {}) {
  const key = value => value.replaceAll('\\', '/');
  return {
    readdirSync(root, options) {
      const normalized = key(root);
      if (dirs[normalized]) return dirs[normalized].map(name => ({ name, isDirectory: () => dirs[`${normalized}/${name}`] !== undefined }));
      throw new Error('missing');
    },
    readFileSync(file) { if (files[key(file)] !== undefined) return files[key(file)]; throw new Error('missing'); }
  };
}

test('desktop entries are parsed without exposing arbitrary commands', () => {
  const fs = { readFileSync: () => '[Desktop Entry]\nType=Application\nName=Writer\nExec=writer %F\n' };
  assert.deepEqual(parseDesktopFile('/writer.desktop', fs), { name: 'Writer', exec: 'writer %F' });
  assert.equal(parseDesktopFile('/hidden.desktop', { readFileSync: () => '[Desktop Entry]\nType=Application\nName=X\nHidden=true\nExec=x' }), null);
  assert.equal(titleFrom('/Applications/Text.app'), 'Text');
});

test('Windows catalog launches only a selected opaque shortcut id', async () => {
  const root = 'C:/Start';
  const fs = fakeFs({}, { [root]: ['Writer.lnk', 'nested'], [`${root}/nested`]: [] });
  let opened = '';
  const launcher = createApplicationLauncher({ platform: 'win32', env: {}, applicationRoots: { windows: [root] }, fs, shell: { openPath: async value => { opened = value; return ''; } } });
  const apps = launcher.discover();
  assert.equal(apps.length, 1); assert.equal(apps[0].title, 'Writer');
  await launcher.launch(apps[0].id); assert.equal(opened, path.join(root, 'Writer.lnk'));
  await assert.rejects(launcher.launch('not-from-catalog'), /application_not_selected/);
});

test('Linux launch uses gtk-launch with a catalog desktop id and no shell', async () => {
  const root = '/apps'; let called;
  const fs = fakeFs({ [`${root}/writer.desktop`]: '[Desktop Entry]\nType=Application\nName=Writer\nExec=writer %F\n' }, { [root]: ['writer.desktop'] });
  const launcher = createApplicationLauncher({ platform: 'linux', env: {}, applicationRoots: { linux: [root] }, fs, spawn: (file, args, options) => { called = { file, args, options }; return { once: (event, cb) => event === 'spawn' && cb(), unref() {} }; } });
  const apps = launcher.discover(); assert.equal(apps[0].driver, 'linux-atspi');
  await launcher.launch(apps[0].id); assert.deepEqual(called, { file: 'gtk-launch', args: ['writer.desktop'], options: { detached: true, stdio: 'ignore' } });
});
