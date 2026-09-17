'use strict';
const byId = id => document.getElementById(id);
const bridge = window.electron;
let actionPending = false;

function render(state) {
  const busy = ['checking', 'starting'].includes(state.state);
  const ready = state.state === 'ready';
  byId('title').textContent = busy ? 'Opening your workspace' : ready ? 'Your workspace is ready' : 'Choose how to continue';
  byId('status-title').textContent = ({ checking: 'Checking your service', starting: 'Starting the desktop service', ready: 'Connected', incompatible: 'A different service is already running', unavailable: 'The service is not ready', error: 'Startup needs attention', offline: 'Service is offline' })[state.state] || 'Checking your service';
  byId('status-text').textContent = state.reason || (ready ? 'Opening the verified workspace…' : state.state === 'starting' ? 'Preparing local storage and the bundled runtime. You can still move or close this window.' : 'Verifying the service version and interface.');
  byId('indicator').classList.toggle('idle', !busy);
  byId('desktop-version').textContent = state.desktopVersion || '—';
  byId('service-version').textContent = state.version || (busy ? 'Checking…' : 'Not available');
  byId('address').textContent = state.url || '—';
  byId('timing').textContent = state.elapsedMs ? `${(state.elapsedMs / 1000).toFixed(1)}s · ${busy ? 'Connecting' : 'Connection check'}` : '';
  byId('recovery').hidden = busy || ready;
  byId('browser').hidden = !state.canOpenBrowser;
  byId('separate').hidden = Boolean(state.managedByDesktop);
  byId('guidance').textContent = state.managedByDesktop
    ? 'Retry reconnects to the desktop service without starting another process. Your saved workspace is preserved.'
    : 'Your existing service and saved work are untouched. Update that service to match this desktop, or choose a separate desktop workspace.';
  byId('footer-state').textContent = state.workspaceMode === 'isolated' ? 'Separate desktop workspace' : state.managedByDesktop ? 'Desktop-managed service' : 'Local service connection';
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
    byId('action-error').textContent = 'This action could not complete. Retry, or close and reopen OffGrid.';
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
