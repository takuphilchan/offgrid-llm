// One lifecycle owner. Renderer navigation and status reads never start probes
// or spawn additional services. Only explicit recovery actions retry startup.
const { EventEmitter } = require('node:events');
const fs = require('node:fs/promises');
const net = require('node:net');
const { spawn } = require('node:child_process');
const { setTimeout: delay } = require('node:timers/promises');
const { inspectBackend } = require('./backend');

function availablePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port;
      server.close(error => error ? reject(error) : resolve(port));
    });
  });
}

class DesktopRuntime extends EventEmitter {
  constructor(options) {
    super();
    this.options = { inspect: inspectBackend, spawn, fs, availablePort, timeoutMs: 30000, retryMs: 250, ...options };
    this.url = options.url;
    this.workspace = options.workspace;
    this.child = null;
    this.pending = null;
    this.closing = false;
    this.state = { state: 'checking', url: this.url, desktopVersion: options.version, managedByDesktop: false, workspaceMode: 'default', elapsedMs: 0 };
  }

  snapshot() { return { ...this.state, url: this.url, managedByDesktop: Boolean(this.child) }; }

  publish(update) {
    this.state = { ...this.state, ...update, elapsedMs: this.started ? Date.now() - this.started : 0 };
    this.emit('status', this.snapshot());
    return this.snapshot();
  }

  connect(isolated = false) {
    if (this.pending) return this.pending;
    if (this.closing || this.state.state === 'ready') return Promise.resolve(this.snapshot());
    // No renderer-supplied paths, commands, or addresses. This explicit action
    // starts a different workspace; it never upgrades/stops the existing one.
    if (isolated && (this.child || !['checking', 'incompatible', 'unavailable', 'error'].includes(this.state.state))) {
      return Promise.resolve(this.snapshot());
    }
    this.started = Date.now();
    this.abort = new AbortController();
    this.pending = this.run(isolated).catch(() => {
      if (!this.closing) this.publish({ state: 'error', reason: 'The desktop service could not start. Check that its files and workspace are accessible, then retry.' });
      return this.snapshot();
    }).finally(() => { this.pending = null; });
    return this.pending;
  }

  async run(isolated) {
    this.publish({ state: 'checking', reason: undefined, version: undefined, canOpenBrowser: false });
    if (isolated) {
      this.url = `http://127.0.0.1:${await this.options.availablePort()}`;
      this.workspace = this.options.isolatedWorkspace;
      this.state.workspaceMode = 'isolated';
    }
    const probe = () => this.options.inspect(this.url, this.options.version, this.options.uiBuildID,
      Math.max(1, Math.min(2000, this.options.timeoutMs - (Date.now() - this.started))), this.abort.signal);
    let result = await probe();
    if (this.closing) return this.snapshot();
    if (this.state.workspaceMode === 'isolated' && !this.child && result.state !== 'offline') {
      return this.publish({ state: 'unavailable', reason: 'The selected local port is already occupied. Choose Start desktop workspace again to try another port; no existing service was changed.' });
    }
    if (result.state === 'ready') return this.publish(result);
    if (!this.child && result.state !== 'offline') return this.publish(result);
    if (!this.child) {
      this.publish({ state: 'starting', reason: undefined });
      try { await this.options.fs.access(this.options.binary); }
      catch { return this.publish({ state: 'error', reason: 'The bundled OffGrid service is missing or inaccessible. Repair or reinstall the desktop application, then retry.' }); }
      for (const directory of [this.workspace.models, this.workspace.data]) {
        await this.options.fs.mkdir(directory, { recursive: true });
      }
      if (this.closing) return this.snapshot();
      // A parent container/CLI environment must not expose this local child.
      const child = this.options.spawn(this.options.binary, ['serve'], {
        windowsHide: true, detached: false, stdio: ['ignore', 'pipe', 'pipe'],
        env: { ...process.env, OFFGRID_HOST: '127.0.0.1', OFFGRID_PORT: new URL(this.url).port,
          OFFGRID_DATA_DIR: this.workspace.data, OFFGRID_MODELS_DIR: this.workspace.models,
          OFFGRID_UI_DIR: this.options.uiDir }
      });
      this.child = child;
      // Drain pipes without synchronously logging every token/request to the
      // Electron main process or exposing raw model output in startup errors.
      child.stdout?.resume();
      child.stderr?.resume();
      const stopped = (code, signal) => {
        if (this.child !== child) return;
        this.child = null;
        const detail = Number.isInteger(code) ? ` (exit ${code})` : typeof signal === 'string' ? ` (${signal})` : '';
        if (!this.closing) this.publish({ state: 'error', reason: `The desktop service stopped${detail}. Another process may own this workspace, or startup failed. Retry after checking the workspace.` });
      };
      child.once('error', stopped);
      child.once('exit', stopped);
    }
    while (!this.closing && this.child && Date.now() - this.started < this.options.timeoutMs) {
      result = await probe();
      if (this.closing || !this.child) break;
      if (result.state === 'ready' || result.state === 'incompatible') return this.publish(result);
      this.publish({ state: 'starting' });
      await delay(this.options.retryMs, undefined, { signal: this.abort.signal }).catch(() => {});
    }
    if (!this.closing && this.child) this.publish({ state: 'unavailable', reason: 'Startup is taking longer than expected. You can retry the connection; the existing startup process will not be duplicated.' });
    return this.snapshot();
  }

  async stop() {
    this.closing = true;
    this.abort?.abort();
    await this.pending;
    const child = this.child;
    if (!child) return;
    await new Promise(resolve => {
      let timer;
      const done = () => { clearTimeout(timer); if (this.child === child) this.child = null; resolve(); };
      child.once('exit', done);
      timer = setTimeout(() => { try { child.kill('SIGKILL'); } catch {} done(); }, 5000);
      try { child.kill('SIGTERM'); } catch { done(); }
    });
  }
}

module.exports = { DesktopRuntime, availablePort };
