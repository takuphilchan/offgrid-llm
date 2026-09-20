// Never publish this package. A separate Windows identity keeps installer smoke
// tests away from a developer's real OffGrid registration and shortcuts.
const { build } = require('./package.json');
module.exports = {
  ...build,
  protocols: [],
  appId: 'com.offgrid.llm.desktop.installtest',
  productName: 'OffGrid Desktop Install Test',
  extraMetadata: { name: 'offgrid-desktop-install-test' },
  directories: { ...build.directories, output: '../build/windows-installer-smoke' },
  nsis: {
    ...build.nsis,
    guid: '37f1826c-126f-448e-8e4c-ce1b846307b9',
    shortcutName: 'OffGrid Desktop Install Test',
    createDesktopShortcut: false,
    createStartMenuShortcut: false,
    runAfterFinish: true,
    allowElevation: false
  },
  publish: null
};
