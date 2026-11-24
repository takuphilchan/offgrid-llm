'use strict';
// Presentation only: no requests, workspace mutation, or privileged IPC.
(() => {
  const root = document.documentElement;
  root.dataset.workspaceBooting = '';
  const app = document.getElementById('root');
  if (app) app.inert = true;
  const surface = document.createElement('div');
  surface.id = 'workspace-startup';
  surface.dataset.startupState = 'loading';
  surface.innerHTML = '<section class="startup-content" tabindex="-1"><header class="startup-brand"><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m12 2-10 5 10 5 10-5-10-5Z"/><path d="m2 12 10 5 10-5"/><path d="m2 17 10 5 10-5"/></svg><span>OffGrid</span></header><div role="status" aria-live="polite"><div class="startup-stage"></div><p class="startup-message" hidden></p></div><div class="startup-actions" hidden><button type="button"></button></div></section>';
  const stage = surface.querySelector('.startup-stage');
  const message = surface.querySelector('.startup-message');
  const actions = surface.querySelector('.startup-actions');
  const retry = surface.querySelector('button');
  const preferences = window.electron?.presentation;
  let copy = {opening:'Opening your workspace',attention:'Startup needs attention',reconnecting:'Waiting for your workspace',retry:'Retry connection',...preferences?.copy};
  let failed = '', slow = false, retryAction, watchdog;
  const render = () => {
    surface.dataset.startupState = failed ? 'recovery' : 'loading';
    stage.textContent = failed ? copy.attention : slow ? copy.reconnecting : copy.opening;
    message.textContent = failed;
    message.hidden = !failed;
    actions.hidden = !failed && !slow;
    retry.textContent = copy.retry;
  };
  const watch = () => { clearTimeout(watchdog); watchdog = setTimeout(() => { slow = true; render(); },30000); };
  surface.dataset.theme = preferences?.effectiveTheme || (matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light');
  if (preferences) {
    root.lang = preferences.locale;
    root.dir = preferences.direction === 'rtl' ? 'rtl' : 'ltr';
    root.dataset.reduceMotion = String(preferences.appearance?.reduceMotion === true);
    for (const [token,name] of Object.entries({'surface-0':'bg',text:'text',muted:'muted',line:'line'})) {
      const color = preferences.palette?.[token];
      if (typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)) surface.style.setProperty('--'+name,color);
    }
  }
  render(); watch(); document.body.append(surface);
  retry.addEventListener('click',() => {
    if (!retryAction || !failed) { location.reload(); return; }
    failed = ''; slow = false; render(); watch();
    surface.querySelector('section').focus({preventScroll:true});
    retryAction();
  });
  const dispose = () => { clearTimeout(watchdog); window.removeEventListener('offgrid-workspace-startup',update); window.removeEventListener('pagehide',dispose); retryAction = undefined; };
  const update = event => {
    const state = event.detail;
    if (state?.ready) {
      dispose(); delete root.dataset.workspaceBooting;
      if (app) app.inert = false;
      surface.remove(); return;
    }
    if (state?.copy) copy = {...copy,...state.copy};
    if (state?.locale) root.lang = state.locale;
    if (typeof state?.error === 'string') failed = state.error;
    if (state?.onRetry) retryAction = state.onRetry;
    render();
    // The initialized web theme takes precedence once available.
    for (const [token,name] of Object.entries({'surface-0':'bg',text:'text',muted:'muted',line:'line'})) {
      const value = getComputedStyle(root).getPropertyValue('--'+token).trim();
      if (value) surface.style.setProperty('--'+name,value);
    }
  };
  window.addEventListener('offgrid-workspace-startup',update);
  window.addEventListener('pagehide',dispose,{once:true});
})();
