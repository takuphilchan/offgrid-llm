'use strict';
// Runs before the application module. This is presentation only: no requests,
// workspace mutation, or privileged IPC. Readiness comes from actual UI reads.
(() => {
  const root = document.documentElement;
  root.dataset.workspaceBooting = '';
  const app = document.getElementById('root');
  if (app) app.inert = true;
  const surface = document.createElement('div');
  surface.id = 'workspace-startup';
  surface.innerHTML = '<section><header class="startup-brand"><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m12 2-10 5 10 5 10-5-10-5Z"/><path d="m2 12 10 5 10-5"/><path d="m2 17 10 5 10-5"/></svg><span>OffGrid</span></header><p class="startup-welcome"></p><div class="startup-stage" role="status"><span></span></div><span class="startup-elapsed" aria-hidden="true"></span><button type="button" hidden></button></section>';
  const welcome = surface.querySelector('.startup-welcome');
  const stage = surface.querySelector('.startup-stage span');
  const elapsed = surface.querySelector('.startup-elapsed');
  const retry = surface.querySelector('button');
  const preferences = window.electron?.presentation;
  const copy = preferences?.copy ?? {};
  welcome.textContent = copy.intro || 'Welcome to OffGrid.';
  const greeting = window.OffGridStartupWelcome?.create(welcome,preferences?.locale,preferences?.startupOverrides);
  stage.textContent = copy.opening || 'Opening your workspace';
  retry.textContent = copy.retry || 'Retry connection';
  const theme = preferences?.effectiveTheme || (matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light');
  surface.dataset.theme = theme;
  if (preferences) {
    root.lang = preferences.locale;
    root.dir = preferences.direction === 'rtl' ? 'rtl' : 'ltr';
    root.dataset.reduceMotion = String(preferences.appearance?.reduceMotion === true);
    for (const [token, name] of Object.entries({'surface-0':'bg',text:'text',muted:'muted',line:'line'})) {
      const color = preferences.palette?.[token];
      if (typeof color === 'string' && /^#[0-9a-f]{6}$/i.test(color)) surface.style.setProperty('--'+name,color);
    }
  }
  document.body.append(surface);
  const started = performance.now();
  const seconds = () => { elapsed.textContent = new Intl.NumberFormat(root.lang || 'en', {style:'unit',unit:'second',unitDisplay:'short'}).format(Math.floor((performance.now()-started)/1000)); };
  seconds();
  const clock = setInterval(seconds,1000);
  // A failed bundle or a stuck dependency must leave a way to recover. Do not
  // turn a timeout into a false "ready" state or reveal disabled Chat controls.
  const watchdog = setTimeout(() => { retry.hidden = false; },30000);
  retry.addEventListener('click',() => location.reload());
  const update = event => {
    const state = event.detail;
    if (state?.ready) {
      clearInterval(clock); clearTimeout(watchdog); greeting?.dispose();
      delete root.dataset.workspaceBooting;
      if (app) app.inert = false;
      surface.remove();
      window.removeEventListener('offgrid-workspace-startup',update);
      return;
    }
    if (state?.stage) stage.textContent = state.stage;
    if (state?.locale) greeting?.setLocale(state.locale,state.startupOverrides);
    if (state?.retry) retry.textContent = state.retry;
    // The initialized web theme takes precedence once available.
    for (const [token,name] of Object.entries({'surface-0':'bg',text:'text',muted:'muted',line:'line'})) {
      const value = getComputedStyle(root).getPropertyValue('--'+token).trim();
      if (value) surface.style.setProperty('--'+name,value);
    }
  };
  window.addEventListener('offgrid-workspace-startup',update);
  window.addEventListener('pagehide',() => { clearInterval(clock);clearTimeout(watchdog);greeting?.dispose();window.removeEventListener('offgrid-workspace-startup',update); },{once:true});
})();
