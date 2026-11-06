// Keep the established Electron profile and single-instance scope across rebranding.
function applyBranding(app) {
  const userData = app.getPath('userData');
  app.setName('OffGrid');
  app.setPath('userData', userData);
  return 'OffGrid';
}
function chromeColors(theme) {
  return theme === 'dark' ? { color: '#101011', symbolColor: '#efefed' } : { color: '#f7f7f6', symbolColor: '#191919' };
}
function windowChrome(platform, theme) {
  return { backgroundColor: chromeColors(theme).color, ...(platform === 'win32' ? {
    titleBarStyle: 'hidden', titleBarOverlay: { ...chromeColors(theme), height: 36 }
  } : {}) };
}
function updateChrome(window, platform, theme) {
  if (!window || window.isDestroyed()) return;
  window.setBackgroundColor(chromeColors(theme).color);
  if (platform === 'win32') window.setTitleBarOverlay({ ...chromeColors(theme), height: 36 });
}
module.exports = { applyBranding, windowChrome, updateChrome };
