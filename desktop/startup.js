'use strict';
const byId = id => document.getElementById(id);
const bridge = window.electron;
let actionPending = false;
let preferences = bridge?.presentation;
let lastState;
const t = key => preferences?.copy?.[key] ?? key;
function applyPresentation(value) {
  preferences = value;
  document.documentElement.lang = value.locale;
  document.documentElement.dir = value.locale === 'ar' ? 'rtl' : 'ltr';
  document.documentElement.dataset.theme = value.effectiveTheme;
  for (const node of document.querySelectorAll('[data-copy]')) node.textContent = t(node.dataset.copy);
  document.title = 'OffGrid — ' + t('opening');
  if (lastState) render(lastState);
}
if (preferences) applyPresentation(preferences);
if (bridge?.getPresentation) void bridge.getPresentation().then(applyPresentation).catch(() => {});

function render(state) {
  lastState = state;
  const busy = ['checking', 'starting'].includes(state.state);
  const ready = state.state === 'ready';
  byId('title').textContent = t(busy ? 'opening' : ready ? 'ready' : 'attention');
  byId('status-title').textContent = t(({ checking: 'checking', starting: 'starting', ready: 'ready', incompatible: 'incompatible', unavailable: 'unavailable', error: 'attention', offline: 'unavailable' })[state.state] || 'checking');
  byId('status-text').textContent = t(busy ? 'intro' : ready ? 'ready' : 'guidance');
  byId('technical').hidden = !state.reason;
  byId('technical-reason').textContent = state.reason || '';
  byId('indicator').classList.toggle('idle', !busy);
  byId('desktop-version').textContent = state.desktopVersion || '—';
  byId('service-version').textContent = state.version || t(busy ? 'checking' : 'unknown');
  byId('address').textContent = state.url || '—';
  byId('timing').textContent = state.elapsedMs ? `${(state.elapsedMs / 1000).toFixed(1)}s` : '';
  byId('recovery').hidden = busy || ready;
  byId('browser').hidden = !state.canOpenBrowser;
  byId('separate').hidden = Boolean(state.managedByDesktop);
  byId('guidance').textContent = t('guidance');
  byId('footer-state').textContent = t(state.workspaceMode === 'isolated' ? 'separate' : 'configured');
}

async function perform(action) {
  if (actionPending) return;
  actionPending = true;
  byId('action-error').hidden = true;
  for (const button of document.querySelectorAll('button')) button.disabled = true;
  try {
    const state = await action();
    if (state) render(state);
  } catch {
    byId('action-error').textContent = t('actionError');
    byId('action-error').hidden = false;
  } finally {
    actionPending = false;
    for (const button of document.querySelectorAll('button')) button.disabled = false;
  }
}

byId('retry').addEventListener('click', () => void perform(() => bridge.retryStartup()));
byId('local').addEventListener('click', () => void perform(() => bridge.startDesktopWorkspace()));
byId('browser').addEventListener('click', () => void perform(() => bridge.openExistingWorkspace()));
byId('help').addEventListener('click', () => void perform(() => bridge.openStartupHelp()));
if (bridge?.getBackendInfo && bridge?.onStartupState) {
  const unsubscribe = bridge.onStartupState(render);
  window.addEventListener('pagehide', unsubscribe, { once: true });
  // Subscribe first; this read is a cached snapshot, not another network probe.
  void bridge.getBackendInfo().then(render).catch(() => render({ state: 'error', reason: 'The desktop bridge is unavailable. Close and reopen OffGrid.' }));
} else render({ state: 'error', reason: 'The desktop bridge is unavailable. Close and reopen OffGrid.' });
