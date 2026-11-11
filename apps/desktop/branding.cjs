// Keep the established Electron profile and single-instance scope across rebranding.
function applyBranding(app) {
  const userData = app.getPath('userData');
  app.setName('OffGrid');
  app.setPath('userData', userData);
  return 'OffGrid';
}
function chromeColors(theme, palette) {
  if (/^#[0-9a-f]{6}$/i.test(palette?.['surface-0']) && /^#[0-9a-f]{6}$/i.test(palette?.text)) return {color:palette['surface-0'],symbolColor:palette.text};
  return theme === 'dark' ? { color: '#101011', symbolColor: '#efefed' } : { color: '#f7f7f6', symbolColor: '#191919' };
}
function windowChrome(platform, theme, palette) {
  return { backgroundColor: chromeColors(theme, palette).color, ...(platform === 'win32' ? {
    titleBarStyle: 'hidden', titleBarOverlay: { ...chromeColors(theme, palette), height: 36 }
  } : {}) };
}
function updateChrome(window, platform, theme, palette) {
  if (!window || window.isDestroyed()) return;
  window.setBackgroundColor(chromeColors(theme, palette).color);
  if (platform === 'win32') window.setTitleBarOverlay({ ...chromeColors(theme, palette), height: 36 });
}
module.exports = { applyBranding, windowChrome, updateChrome };
